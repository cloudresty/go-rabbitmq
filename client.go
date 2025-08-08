package rabbitmq

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client represents the main RabbitMQ client with unified architecture
type Client struct {
	// Connection management
	conn   *amqp.Connection
	connMu sync.RWMutex
	config *clientConfig

	// Connection state
	closed       bool
	reconnecting bool
	reconnectMu  sync.RWMutex
	// Services (lazy-initialized)
	admin     *AdminService
	adminOnce sync.Once

	// Topology management
	topologyRegistry  *TopologyRegistry
	topologyValidator *TopologyValidator
	topologyOnce      sync.Once

	// Shutdown handling
	closeCh chan struct{}
	closeWg sync.WaitGroup
}

// clientConfig holds all configuration for the client
type clientConfig struct {
	// Connection settings
	URL      string
	URLs     []string // Multiple URLs for failover support
	Hosts    []string // Multiple hosts for failover support
	Username string
	Password string
	VHost    string
	TLS      *tls.Config

	// Connection behavior
	ConnectionName string
	Heartbeat      time.Duration
	DialTimeout    time.Duration
	ChannelTimeout time.Duration

	// Reconnection policy
	AutoReconnect        bool
	ReconnectDelay       time.Duration
	MaxReconnectAttempts int
	ReconnectPolicy      ReconnectPolicy

	// Observability
	Logger             Logger
	Metrics            MetricsCollector
	Tracer             Tracer
	PerformanceMonitor PerformanceMonitor

	// Security
	AccessPolicy *AccessPolicy
	AuditLogger  AuditLogger

	// Advanced features (pluggable via contract-implementation pattern)
	ConnectionPooler ConnectionPooler
	StreamHandler    StreamHandler
	SagaOrchestrator SagaOrchestrator
	GracefulShutdown GracefulShutdown

	// Topology validation
	TopologyValidation           bool
	TopologyAutoRecreation       bool
	TopologyBackgroundValidation bool
	TopologyValidationInterval   time.Duration
}

// Option represents a functional option for configuring the Client
type Option func(*clientConfig) error

// NewClient creates a new RabbitMQ client with the specified options
func NewClient(opts ...Option) (*Client, error) {
	// Default configuration
	config := &clientConfig{
		Username:             "guest",
		Password:             "guest",
		VHost:                "/",
		ConnectionName:       "go-rabbitmq-client",
		Heartbeat:            10 * time.Second,
		DialTimeout:          30 * time.Second,
		ChannelTimeout:       10 * time.Second,
		AutoReconnect:        true,
		ReconnectDelay:       5 * time.Second,
		MaxReconnectAttempts: 0, // Unlimited
		Logger:               NewNopLogger(),
		Metrics:              NewNopMetrics(),
		Tracer:               NewNopTracer(),
		PerformanceMonitor:   NewNopPerformanceMonitor(),

		// Topology validation enabled by default for production reliability
		TopologyValidation:           true,
		TopologyAutoRecreation:       true,
		TopologyBackgroundValidation: true,
		TopologyValidationInterval:   30 * time.Second, // 30s default interval
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Build URL if not provided
	if config.URL == "" {
		scheme := "amqp"
		if config.TLS != nil {
			scheme = "amqps"
		}

		// Use first host from Hosts array or default to localhost:5672
		host := "localhost:5672"
		if len(config.Hosts) > 0 {
			host = config.Hosts[0]
		}

		config.URL = fmt.Sprintf("%s://%s:%s@%s%s",
			scheme, config.Username, config.Password, host, config.VHost)
	}

	client := &Client{
		config:  config,
		closeCh: make(chan struct{}),
	}

	// Initialize topology components if validation is enabled
	if config.TopologyValidation {
		client.initializeTopology()
	}

	// Establish initial connection
	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to establish connection: %w", err)
	}

	// Start connection monitoring if auto-reconnect is enabled
	if config.AutoReconnect {
		client.closeWg.Add(1)
		go client.connectionMonitor()
	}

	client.config.Logger.Info("RabbitMQ client created successfully",
		"connection_name", config.ConnectionName,
		"url", config.URL,
		"vhost", config.VHost)

	return client, nil
}

// initializeTopology initializes topology validation components
func (c *Client) initializeTopology() {
	c.topologyOnce.Do(func() {
		c.topologyRegistry = NewTopologyRegistry()
		// Note: We'll initialize the validator lazily when Admin() is first called
		// to avoid circular dependencies during client creation
	})
} // TopologyRegistry returns the topology registry for manual topology management
func (c *Client) TopologyRegistry() *TopologyRegistry {
	if c.topologyRegistry == nil {
		return nil
	}
	return c.topologyRegistry
}

// TopologyValidator returns the topology validator for manual validation operations
func (c *Client) TopologyValidator() *TopologyValidator {
	if c.topologyValidator == nil {
		return nil
	}
	return c.topologyValidator
}

// Health and connectivity methods
func (c *Client) Ping(ctx context.Context) error {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	if c.conn == nil || c.conn.IsClosed() {
		return fmt.Errorf("connection is not available")
	}

	// Open a temporary channel and perform a lightweight operation
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	// Perform a passive queue declare on a built-in exchange to verify broker responsiveness
	_, err = ch.QueueDeclarePassive("amq.direct", false, false, false, false, nil)
	if err != nil {
		// This is expected to fail, but if we get here, the broker is responsive
		if amqpErr, ok := err.(*amqp.Error); ok && amqpErr.Code == 404 {
			// Queue not found is expected for amq.direct, connection is good
			return nil
		}
		return fmt.Errorf("broker health check failed: %w", err)
	}

	return nil
}

func (c *Client) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.closeCh)

	c.config.Logger.Info("Closing RabbitMQ client",
		"connection_name", c.config.ConnectionName)

	// Close connection
	if c.conn != nil && !c.conn.IsClosed() {
		if err := c.conn.Close(); err != nil {
			c.config.Logger.Error("Failed to close connection gracefully",
				"error", err.Error())
		}
	}

	// Wait for background goroutines to finish
	c.closeWg.Wait()

	c.config.Logger.Info("RabbitMQ client closed successfully",
		"connection_name", c.config.ConnectionName)

	return nil
}

// Service accessors
func (c *Client) Admin() *AdminService {
	c.adminOnce.Do(func() {
		c.admin = &AdminService{client: c}

		// Initialize topology validator if validation is enabled
		if c.config.TopologyValidation && c.topologyValidator == nil {
			c.topologyValidator = NewTopologyValidator(c, c.admin, c.topologyRegistry)

			// Enable validation
			c.topologyValidator.Enable()

			// Enable auto-recreation if configured
			if c.config.TopologyAutoRecreation {
				c.topologyValidator.EnableAutoRecreate()
			}

			// Enable background validation if configured
			if c.config.TopologyBackgroundValidation {
				interval := c.config.TopologyValidationInterval
				if interval == 0 {
					interval = 30 * time.Second // Default interval
				}
				c.topologyValidator.EnableBackgroundValidation(interval)
			}
		}
	})
	return c.admin
}

// Connection information
func (c *Client) URL() string {
	return c.config.URL
}

func (c *Client) ConnectionName() string {
	return c.config.ConnectionName
}

// NewPublisher creates a new publisher from the client
func (c *Client) NewPublisher(opts ...PublisherOption) (*Publisher, error) {
	// Default configuration
	config := &publisherConfig{
		DefaultExchange:     "",
		Mandatory:           false,
		Immediate:           false,
		Persistent:          true,
		ConfirmationEnabled: false,
		ConfirmationTimeout: 5 * time.Second,
		RetryPolicy:         NoRetry,
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Get channel
	ch, err := c.getChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	// Enable confirmation mode if requested
	if config.ConfirmationEnabled {
		if err := ch.Confirm(false); err != nil {
			_ = ch.Close() // Clean up channel on error
			return nil, fmt.Errorf("failed to enable confirm mode: %w", err)
		}

		// Set up confirmation channel with buffering for concurrent publishes
		config.confirmations = ch.NotifyPublish(make(chan amqp.Confirmation, 100))
	}

	publisher := &Publisher{
		client: c,
		config: config,
		ch:     ch,
	}

	c.config.Logger.Info("Publisher created successfully",
		"connection_name", c.config.ConnectionName,
		"default_exchange", config.DefaultExchange,
		"mandatory", config.Mandatory,
		"persistent", config.Persistent,
		"confirmation_enabled", config.ConfirmationEnabled)

	return publisher, nil
}

// NewConsumer creates a new consumer from the client
func (c *Client) NewConsumer(opts ...ConsumerOption) (*Consumer, error) {
	// Default configuration
	config := &consumerConfig{
		AutoAck:        false,
		PrefetchCount:  1,
		PrefetchSize:   0,
		ConsumerTag:    "",
		Exclusive:      false,
		NoLocal:        false,
		NoWait:         false,
		MessageTimeout: 30 * time.Second,
		Concurrency:    1,
		RetryPolicy:    NoRetry,
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Get channel
	ch, err := c.getChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	consumer := &Consumer{
		client: c,
		config: config,
		ch:     ch,
		stopCh: make(chan struct{}),
	}

	c.config.Logger.Info("Consumer created successfully",
		"connection_name", c.config.ConnectionName,
		"prefetch_count", config.PrefetchCount,
		"prefetch_size", config.PrefetchSize,
		"auto_ack", config.AutoAck,
		"concurrency", config.Concurrency)

	return consumer, nil
}

// CreateChannel creates a new AMQP channel for advanced use cases
// This method is provided for implementing custom messaging patterns
// such as streams, sagas, or other experimental features
func (c *Client) CreateChannel() (*amqp.Channel, error) {
	return c.getChannel()
}

// connect establishes the connection to RabbitMQ
func (c *Client) connect() error {
	// Set up client properties with connection name
	properties := amqp.NewConnectionProperties()
	if c.config.ConnectionName != "" {
		properties.SetClientConnectionName(c.config.ConnectionName)
	}

	amqpConfig := amqp.Config{
		Heartbeat:       c.config.Heartbeat,
		TLSClientConfig: c.config.TLS,
		Dial:            amqp.DefaultDial(c.config.DialTimeout),
		Properties:      properties,
	}

	// Try multiple URLs if available (failover support)
	urls := c.getConnectionURLs()

	c.config.Logger.Info("Establishing RabbitMQ connection",
		"connection_name", c.config.ConnectionName,
		"url", c.config.URL,
		"failover_urls", len(urls))

	start := time.Now()
	var lastErr error
	for i, url := range urls {
		c.config.Logger.Debug("Attempting connection",
			"url_index", fmt.Sprintf("%d/%d", i+1, len(urls)),
			"connection_name", c.config.ConnectionName)

		attemptStart := time.Now()
		conn, err := amqp.DialConfig(url, amqpConfig)
		_ = time.Since(attemptStart) // Track attempt duration if needed for debugging

		if err != nil {
			lastErr = err
			c.config.Logger.Debug("Connection attempt failed",
				"url_index", fmt.Sprintf("%d/%d", i+1, len(urls)),
				"error", err.Error())
			continue
		}

		c.conn = conn
		totalDuration := time.Since(start)
		c.config.Metrics.RecordConnectionAttempt(true, totalDuration)

		c.config.Logger.Info("RabbitMQ connection established",
			"connection_name", c.config.ConnectionName,
			"connected_url_index", fmt.Sprintf("%d/%d", i+1, len(urls)))

		return nil
	}

	// All connection attempts failed
	totalDuration := time.Since(start)
	c.config.Metrics.RecordConnectionAttempt(false, totalDuration)
	return fmt.Errorf("failed to connect to any RabbitMQ host after %d attempts, last error: %w", len(urls), lastErr)
}

// getConnectionURLs returns the list of URLs to try for connection (failover support)
func (c *Client) getConnectionURLs() []string {
	// If we have multiple URLs configured, use them
	if len(c.config.URLs) > 0 {
		return c.config.URLs
	}

	// Fall back to single URL
	if c.config.URL != "" {
		return []string{c.config.URL}
	}

	// Last resort: build URL from individual components
	host := "localhost:5672"
	if len(c.config.Hosts) > 0 {
		host = c.config.Hosts[0]
	}
	return []string{fmt.Sprintf("amqp://%s:%s@%s%s",
		c.config.Username, c.config.Password, host, c.config.VHost)}
}

// connectionMonitor monitors the connection and handles reconnection
func (c *Client) connectionMonitor() {
	defer c.closeWg.Done()

	for {
		select {
		case <-c.closeCh:
			return
		default:
			// Check if connection is closed
			if c.conn != nil && c.conn.IsClosed() {
				c.config.Logger.Warn("Connection lost, attempting to reconnect",
					"connection_name", c.config.ConnectionName)

				c.handleReconnection()
			}

			time.Sleep(time.Second) // Check every second
		}
	}
}

// handleReconnection attempts to reconnect to RabbitMQ
func (c *Client) handleReconnection() {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	if c.reconnecting || c.closed {
		return
	}

	c.reconnecting = true
	defer func() { c.reconnecting = false }()

	attempt := 0
	for {
		if c.closed {
			return
		}

		if c.config.MaxReconnectAttempts > 0 && attempt >= c.config.MaxReconnectAttempts {
			c.config.Logger.Error("Max reconnection attempts reached",
				"connection_name", c.config.ConnectionName,
				"max_attempts", c.config.MaxReconnectAttempts)
			return
		}

		attempt++

		c.config.Logger.Info("Attempting to reconnect",
			"connection_name", c.config.ConnectionName,
			"attempt", attempt)

		// Calculate delay based on reconnection policy
		delay := c.config.ReconnectDelay
		if c.config.ReconnectPolicy != nil {
			delay = c.config.ReconnectPolicy.NextDelay(attempt)
		}

		// Wait before attempting reconnection
		select {
		case <-c.closeCh:
			return
		case <-time.After(delay):
		}

		// Attempt to reconnect
		c.connMu.Lock()
		if err := c.connect(); err != nil {
			c.connMu.Unlock()
			c.config.Logger.Warn("Reconnection attempt failed",
				"connection_name", c.config.ConnectionName,
				"attempt", attempt,
				"error", err.Error())
			continue
		}
		c.connMu.Unlock()

		c.config.Metrics.RecordReconnection(attempt)
		c.config.Logger.Info("Successfully reconnected to RabbitMQ",
			"connection_name", c.config.ConnectionName,
			"attempt", attempt)

		return
	}
}

// getChannel returns a new channel from the connection
func (c *Client) getChannel() (*amqp.Channel, error) {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	if c.conn == nil || c.conn.IsClosed() {
		return nil, fmt.Errorf("connection is not available")
	}

	return c.conn.Channel()
}
