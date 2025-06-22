package rabbitmq

import (
	"fmt"
	"sync"
	"time"

	"github.com/cloudresty/emit"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Connection represents a RabbitMQ connection with retry logic
type Connection struct {
	conn           *amqp.Connection
	channel        *amqp.Channel
	url            string
	config         ConnectionConfig
	closed         bool
	reconnecting   bool
	reconnectMutex sync.RWMutex
}

// ConnectionConfig holds configuration for RabbitMQ connection
type ConnectionConfig struct {
	URL                  string
	RetryAttempts        int
	RetryDelay           time.Duration
	Heartbeat            time.Duration
	ConnectionName       string
	AutoReconnect        bool          // Enable automatic reconnection
	ReconnectDelay       time.Duration // Delay between reconnection attempts
	MaxReconnectAttempts int           // Max reconnection attempts (0 = unlimited)
}

// DefaultConnectionConfig returns a default connection configuration
func DefaultConnectionConfig(url string) ConnectionConfig {
	return ConnectionConfig{
		URL:                  url,
		RetryAttempts:        5,
		RetryDelay:           time.Second * 2,
		Heartbeat:            time.Second * 10,
		ConnectionName:       "go-rabbitmq",
		AutoReconnect:        true,
		ReconnectDelay:       time.Second * 5,
		MaxReconnectAttempts: 0, // Unlimited
	}
}

// NewConnection creates a new RabbitMQ connection
func NewConnection(config ConnectionConfig) (*Connection, error) {
	emit.Info.StructuredFields("Creating new RabbitMQ connection",
		emit.ZString("connection_name", config.ConnectionName),
		emit.ZInt("retry_attempts", config.RetryAttempts),
		emit.ZDuration("heartbeat", config.Heartbeat),
		emit.ZBool("auto_reconnect", config.AutoReconnect))

	conn := &Connection{
		url:    config.URL,
		config: config,
	}

	err := conn.connect(config)
	if err != nil {
		emit.Error.StructuredFields("Failed to establish RabbitMQ connection",
			emit.ZString("error", err.Error()),
			emit.ZString("connection_name", config.ConnectionName))
		return nil, fmt.Errorf("failed to establish connection: %w", err)
	}

	emit.Info.StructuredFields("RabbitMQ connection established successfully",
		emit.ZString("connection_name", config.ConnectionName))

	// Start connection monitoring if auto-reconnect is enabled
	if config.AutoReconnect {
		go conn.startConnectionMonitoring()
	}

	return conn, nil
}

// connect establishes connection to RabbitMQ with retry logic
func (c *Connection) connect(config ConnectionConfig) error {
	var err error

	emit.Info.StructuredFields("Attempting to connect to RabbitMQ",
		emit.ZString("connection_name", config.ConnectionName),
		emit.ZInt("retry_attempts", config.RetryAttempts),
		emit.ZDuration("retry_delay", config.RetryDelay))

	for attempt := 0; attempt < config.RetryAttempts; attempt++ {
		if attempt > 0 {
			emit.Warn.StructuredFields("Connection attempt failed, retrying",
				emit.ZInt("attempt", attempt+1),
				emit.ZInt("max_attempts", config.RetryAttempts),
				emit.ZString("connection_name", config.ConnectionName))
		}

		// Configure connection properties
		amqpConfig := amqp.Config{
			Heartbeat: config.Heartbeat,
			Properties: amqp.Table{
				"connection_name": config.ConnectionName,
			},
		}

		c.conn, err = amqp.DialConfig(config.URL, amqpConfig)
		if err != nil {
			if attempt == config.RetryAttempts-1 {
				emit.Error.StructuredFields("Failed to connect after all retry attempts",
					emit.ZString("error", err.Error()),
					emit.ZInt("attempts", config.RetryAttempts),
					emit.ZString("connection_name", config.ConnectionName))
				return fmt.Errorf("failed to connect after %d attempts: %w", config.RetryAttempts, err)
			}
			time.Sleep(config.RetryDelay)
			continue
		}

		c.channel, err = c.conn.Channel()
		if err != nil {
			c.conn.Close()
			if attempt == config.RetryAttempts-1 {
				emit.Error.StructuredFields("Failed to create channel after all retry attempts",
					emit.ZString("error", err.Error()),
					emit.ZInt("attempts", config.RetryAttempts),
					emit.ZString("connection_name", config.ConnectionName))
				return fmt.Errorf("failed to create channel after %d attempts: %w", config.RetryAttempts, err)
			}
			time.Sleep(config.RetryDelay)
			continue
		}

		emit.Info.StructuredFields("Successfully connected to RabbitMQ",
			emit.ZInt("attempt", attempt+1),
			emit.ZString("connection_name", config.ConnectionName))
		return nil
	}

	return err
}

// Channel returns the AMQP channel
func (c *Connection) Channel() *amqp.Channel {
	return c.channel
}

// IsConnected checks if the connection is still active
func (c *Connection) IsConnected() bool {
	return c.conn != nil && !c.conn.IsClosed() && c.channel != nil
}

// Close closes the connection and channel
func (c *Connection) Close() error {
	c.reconnectMutex.Lock()
	if c.closed {
		c.reconnectMutex.Unlock()
		return nil
	}
	c.closed = true
	c.reconnectMutex.Unlock()

	emit.Debug.Msg("Closing RabbitMQ connection")

	var err error
	if c.channel != nil {
		if channelErr := c.channel.Close(); channelErr != nil {
			emit.Error.StructuredFields("Error closing RabbitMQ channel",
				emit.ZString("error", channelErr.Error()))
			err = channelErr
		}
	}

	if c.conn != nil {
		if connErr := c.conn.Close(); connErr != nil {
			emit.Error.StructuredFields("Error closing RabbitMQ connection",
				emit.ZString("error", connErr.Error()))
			if err == nil {
				err = connErr
			}
		}
	}

	if err == nil {
		emit.Info.Msg("RabbitMQ connection closed successfully")
	}

	return err
}

// NotifyClose returns a channel that will receive close notifications
func (c *Connection) NotifyClose() <-chan *amqp.Error {
	if c.conn == nil {
		ch := make(chan *amqp.Error, 1)
		close(ch)
		return ch
	}
	return c.conn.NotifyClose(make(chan *amqp.Error, 1))
}

// startConnectionMonitoring monitors the connection and automatically reconnects if needed
func (c *Connection) startConnectionMonitoring() {
	for {
		c.reconnectMutex.RLock()
		isClosed := c.closed
		c.reconnectMutex.RUnlock()

		if isClosed {
			return
		}

		// Wait for connection close notification
		closeChan := c.NotifyClose()
		closeErr := <-closeChan

		c.reconnectMutex.RLock()
		isClosed = c.closed
		c.reconnectMutex.RUnlock()

		if closeErr != nil && !isClosed {
			emit.Warn.StructuredFields("Connection lost, attempting to reconnect",
				emit.ZString("error", closeErr.Error()),
				emit.ZString("connection_name", c.config.ConnectionName))

			c.attemptReconnection()
		}
	}
}

// attemptReconnection attempts to reconnect to RabbitMQ
func (c *Connection) attemptReconnection() {
	c.reconnectMutex.Lock()
	defer c.reconnectMutex.Unlock()

	if c.reconnecting || c.closed {
		return
	}

	c.reconnecting = true
	defer func() { c.reconnecting = false }()

	attempts := 0
	maxAttempts := c.config.MaxReconnectAttempts

	for {
		if c.closed {
			return
		}

		attempts++

		// Check if we've exceeded max attempts (0 means unlimited)
		if maxAttempts > 0 && attempts > maxAttempts {
			emit.Error.StructuredFields("Exceeded maximum reconnection attempts",
				emit.ZInt("attempts", attempts-1),
				emit.ZInt("max_attempts", maxAttempts),
				emit.ZString("connection_name", c.config.ConnectionName))
			return
		}

		emit.Info.StructuredFields("Attempting to reconnect",
			emit.ZInt("attempt", attempts),
			emit.ZString("connection_name", c.config.ConnectionName))

		// Close existing connections if they exist
		if c.channel != nil {
			c.channel.Close()
		}
		if c.conn != nil {
			c.conn.Close()
		}

		// Attempt to reconnect
		err := c.connect(c.config)
		if err == nil {
			emit.Info.StructuredFields("Successfully reconnected to RabbitMQ",
				emit.ZInt("attempts", attempts),
				emit.ZString("connection_name", c.config.ConnectionName))
			return
		}

		emit.Warn.StructuredFields("Reconnection attempt failed",
			emit.ZInt("attempt", attempts),
			emit.ZString("error", err.Error()),
			emit.ZString("connection_name", c.config.ConnectionName))

		// Wait before next attempt
		time.Sleep(c.config.ReconnectDelay)
	}
}

// IsReconnecting returns true if the connection is currently attempting to reconnect
func (c *Connection) IsReconnecting() bool {
	c.reconnectMutex.RLock()
	defer c.reconnectMutex.RUnlock()
	return c.reconnecting
}
