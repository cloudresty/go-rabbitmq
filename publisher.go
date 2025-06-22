package rabbitmq

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudresty/emit"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher handles publishing messages to RabbitMQ
type Publisher struct {
	conn     *Connection
	config   PublisherConfig
	inFlight *InFlightTracker
}

// PublisherConfig holds configuration for the publisher
type PublisherConfig struct {
	ConnectionConfig
	DefaultExchange     string
	DefaultRoutingKey   string
	Persistent          bool
	Mandatory           bool
	Immediate           bool
	ConfirmationTimeout time.Duration // Timeout for publisher confirmations
	ShutdownTimeout     time.Duration // Timeout for graceful publisher shutdown
}

// PublishConfig holds configuration for a single publish operation
type PublishConfig struct {
	Exchange    string
	RoutingKey  string
	Message     []byte
	ContentType string
	Headers     map[string]interface{}
	Persistent  *bool // Optional override for message persistence
	// Message identification and tracing (for backward compatibility)
	MessageID     string
	CorrelationID string
	ReplyTo       string
	Type          string
	AppID         string
	UserID        string
	Expiration    string
	Priority      uint8
}

// PublishMessageConfig holds configuration for publishing Message objects
type PublishMessageConfig struct {
	Exchange   string
	RoutingKey string
	Message    *Message
}

// NewPublisher creates a new publisher with default configuration
func NewPublisher(url string) (*Publisher, error) {
	config := PublisherConfig{
		ConnectionConfig:    DefaultConnectionConfig(url),
		Persistent:          true,
		Mandatory:           false,
		Immediate:           false,
		ConfirmationTimeout: time.Second * 5,
		ShutdownTimeout:     time.Second * 15, // 15 second timeout for graceful shutdown
	}
	config.ConnectionName = "go-rabbitmq-publisher"

	return NewPublisherWithConfig(config)
}

// NewPublisherWithConfig creates a new publisher with custom configuration
func NewPublisherWithConfig(config PublisherConfig) (*Publisher, error) {
	emit.Info.StructuredFields("Creating RabbitMQ publisher",
		emit.ZString("connection_name", config.ConnectionName),
		emit.ZString("default_exchange", config.DefaultExchange),
		emit.ZBool("persistent", config.Persistent))

	conn, err := NewConnection(config.ConnectionConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher connection",
			emit.ZString("error", err.Error()),
			emit.ZString("connection_name", config.ConnectionName))
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	publisher := &Publisher{
		conn:     conn,
		config:   config,
		inFlight: NewInFlightTracker(),
	}

	emit.Info.StructuredFields("RabbitMQ publisher created successfully",
		emit.ZString("connection_name", config.ConnectionName))

	return publisher, nil
}

// DeclareExchange declares an exchange with the given parameters
func (p *Publisher) DeclareExchange(name, kind string, durable, autoDelete, internal, noWait bool, args map[string]interface{}) error {
	if !p.conn.IsConnected() {
		emit.Error.StructuredFields("Publisher not connected when declaring exchange",
			emit.ZString("exchange_name", name),
			emit.ZString("exchange_type", kind))
		return fmt.Errorf("publisher is not connected")
	}

	emit.Debug.StructuredFields("Declaring RabbitMQ exchange",
		emit.ZString("exchange_name", name),
		emit.ZString("exchange_type", kind),
		emit.ZBool("durable", durable),
		emit.ZBool("auto_delete", autoDelete))

	err := p.conn.Channel().ExchangeDeclare(
		name,       // name
		kind,       // type
		durable,    // durable
		autoDelete, // auto-deleted
		internal,   // internal
		noWait,     // no-wait
		args,       // arguments
	)

	if err != nil {
		emit.Error.StructuredFields("Failed to declare exchange",
			emit.ZString("exchange_name", name),
			emit.ZString("exchange_type", kind),
			emit.ZString("error", err.Error()))
	} else {
		emit.Info.StructuredFields("Exchange declared successfully",
			emit.ZString("exchange_name", name),
			emit.ZString("exchange_type", kind))
	}

	return err
}

// DeclareQueue declares a queue with the given parameters
func (p *Publisher) DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args map[string]interface{}) error {
	if !p.conn.IsConnected() {
		emit.Error.StructuredFields("Publisher not connected when declaring queue",
			emit.ZString("queue_name", name))
		return fmt.Errorf("publisher is not connected")
	}

	emit.Debug.StructuredFields("Declaring RabbitMQ queue",
		emit.ZString("queue_name", name),
		emit.ZBool("durable", durable),
		emit.ZBool("auto_delete", autoDelete),
		emit.ZBool("exclusive", exclusive))

	_, err := p.conn.Channel().QueueDeclare(
		name,       // name
		durable,    // durable
		autoDelete, // delete when unused
		exclusive,  // exclusive
		noWait,     // no-wait
		args,       // arguments
	)

	if err != nil {
		emit.Error.StructuredFields("Failed to declare queue",
			emit.ZString("queue_name", name),
			emit.ZString("error", err.Error()))
	} else {
		emit.Info.StructuredFields("Queue declared successfully",
			emit.ZString("queue_name", name))
	}

	return err
}

// DeclareQuorumQueue declares a production-ready quorum queue
func (p *Publisher) DeclareQuorumQueue(name string) error {
	config := DefaultQuorumQueueConfig(name)
	return p.DeclareQueueWithConfig(config)
}

// DeclareHAQueue declares a production-ready HA classic queue
func (p *Publisher) DeclareHAQueue(name string) error {
	config := DefaultHAQueueConfig(name)
	return p.DeclareQueueWithConfig(config)
}

// DeclareQueueWithConfig declares a queue using QueueConfig
func (p *Publisher) DeclareQueueWithConfig(config QueueConfig) error {
	if !p.conn.IsConnected() {
		emit.Error.StructuredFields("Publisher not connected when declaring queue",
			emit.ZString("queue_name", config.Name),
			emit.ZString("queue_type", string(config.QueueType)))
		return fmt.Errorf("publisher is not connected")
	}

	args := config.ToArguments()

	emit.Info.StructuredFields("Declaring queue with enhanced config",
		emit.ZString("queue_name", config.Name),
		emit.ZString("queue_type", string(config.QueueType)),
		emit.ZBool("durable", config.Durable),
		emit.ZBool("high_availability", config.HighAvailability),
		emit.ZInt("replication_factor", config.ReplicationFactor))

	_, err := p.conn.Channel().QueueDeclare(
		config.Name,
		config.Durable,
		config.AutoDelete,
		config.Exclusive,
		config.NoWait,
		args,
	)

	if err != nil {
		emit.Error.StructuredFields("Failed to declare queue",
			emit.ZString("queue_name", config.Name),
			emit.ZString("queue_type", string(config.QueueType)),
			emit.ZString("error", err.Error()))
	} else {
		emit.Info.StructuredFields("Queue declared successfully",
			emit.ZString("queue_name", config.Name),
			emit.ZString("queue_type", string(config.QueueType)))
	}

	return err
}

// Publish publishes a message to RabbitMQ
func (p *Publisher) Publish(ctx context.Context, config PublishConfig) error {
	// Check if shutdown is in progress
	if !p.inFlight.Start() {
		return fmt.Errorf("publisher is shutting down, rejecting new publish operations")
	}
	defer p.inFlight.Done()

	if !p.conn.IsConnected() {
		emit.Error.StructuredFields("Publisher not connected when publishing message",
			emit.ZString("exchange", config.Exchange),
			emit.ZString("routing_key", config.RoutingKey))
		return fmt.Errorf("publisher is not connected")
	}

	// Use defaults if not specified
	exchange := config.Exchange
	if exchange == "" {
		exchange = p.config.DefaultExchange
	}

	routingKey := config.RoutingKey
	if routingKey == "" {
		routingKey = p.config.DefaultRoutingKey
	}

	contentType := config.ContentType
	if contentType == "" {
		contentType = ContentTypeJSON
	}

	// Determine message persistence
	persistent := p.config.Persistent
	if config.Persistent != nil {
		persistent = *config.Persistent
	}

	emit.Debug.StructuredFields("Publishing message to RabbitMQ",
		emit.ZString("exchange", exchange),
		emit.ZString("routing_key", routingKey),
		emit.ZString("content_type", contentType),
		emit.ZBool("persistent", persistent),
		emit.ZInt("message_size_bytes", len(config.Message)),
		emit.ZString("message_id", config.MessageID))

	// Prepare message
	deliveryMode := uint8(1) // Non-persistent
	if persistent {
		deliveryMode = uint8(2) // Persistent
	}

	// Auto-generate MessageID if not provided
	messageID := config.MessageID
	if messageID == "" {
		messageID = generateMessageID()
	}

	msg := amqp.Publishing{
		ContentType:   contentType,
		Body:          config.Message,
		DeliveryMode:  deliveryMode,
		Timestamp:     time.Now(),
		Headers:       config.Headers,
		MessageId:     messageID,
		CorrelationId: config.CorrelationID,
		ReplyTo:       config.ReplyTo,
		Type:          config.Type,
		AppId:         config.AppID,
		UserId:        config.UserID,
		Expiration:    config.Expiration,
		Priority:      config.Priority,
	}

	// Publish the message
	err := p.conn.Channel().PublishWithContext(
		ctx,
		exchange,           // exchange
		routingKey,         // routing key
		p.config.Mandatory, // mandatory
		p.config.Immediate, // immediate
		msg,                // message
	)

	if err != nil {
		emit.Error.StructuredFields("Failed to publish message",
			emit.ZString("exchange", exchange),
			emit.ZString("routing_key", routingKey),
			emit.ZString("error", err.Error()))
	} else {
		emit.Debug.StructuredFields("Message published successfully",
			emit.ZString("exchange", exchange),
			emit.ZString("routing_key", routingKey),
			emit.ZInt("message_size_bytes", len(config.Message)))
	}

	return err
}

// PublishMessage publishes a Message object to RabbitMQ
func (p *Publisher) PublishMessage(ctx context.Context, config PublishMessageConfig) error {
	// Check if shutdown is in progress
	if !p.inFlight.Start() {
		return fmt.Errorf("publisher is shutting down, rejecting new publish operations")
	}
	defer p.inFlight.Done()

	if !p.conn.IsConnected() {
		emit.Error.StructuredFields("Publisher not connected when publishing message",
			emit.ZString("exchange", config.Exchange),
			emit.ZString("routing_key", config.RoutingKey))
		return fmt.Errorf("publisher is not connected")
	}

	// Use provided values or defaults
	exchange := config.Exchange
	if exchange == "" {
		exchange = p.config.DefaultExchange
	}

	routingKey := config.RoutingKey
	if routingKey == "" {
		routingKey = p.config.DefaultRoutingKey
	}

	emit.Debug.StructuredFields("Publishing Message object to RabbitMQ",
		emit.ZString("exchange", exchange),
		emit.ZString("routing_key", routingKey),
		emit.ZString("message_id", config.Message.MessageID),
		emit.ZString("message_type", config.Message.Type),
		emit.ZString("correlation_id", config.Message.CorrelationID),
		emit.ZInt("message_size_bytes", len(config.Message.Body)))

	// Convert Message to amqp.Publishing
	msg := config.Message.ToPublishing()

	// Override exchange and routing key if needed
	config.Message.Exchange = exchange
	config.Message.RoutingKey = routingKey

	return p.conn.Channel().Publish(exchange, routingKey, p.config.Mandatory, p.config.Immediate, msg)
}

// PublishWithConfirmation publishes a message with confirmation
func (p *Publisher) PublishWithConfirmation(ctx context.Context, config PublishConfig) error {
	// Check if shutdown is in progress
	if !p.inFlight.Start() {
		return fmt.Errorf("publisher is shutting down, rejecting new publish operations")
	}
	defer p.inFlight.Done()

	if !p.conn.IsConnected() {
		emit.Error.StructuredFields("Publisher not connected when publishing with confirmation",
			emit.ZString("exchange", config.Exchange),
			emit.ZString("routing_key", config.RoutingKey))
		return fmt.Errorf("publisher is not connected")
	}

	emit.Debug.StructuredFields("Publishing message with confirmation",
		emit.ZString("exchange", config.Exchange),
		emit.ZString("routing_key", config.RoutingKey))

	// Enable confirm mode
	if err := p.conn.Channel().Confirm(false); err != nil {
		emit.Error.StructuredFields("Failed to enable confirm mode",
			emit.ZString("error", err.Error()))
		return fmt.Errorf("failed to enable confirm mode: %w", err)
	}

	// Create confirmation channel
	confirms := p.conn.Channel().NotifyPublish(make(chan amqp.Confirmation, 1))

	// Publish the message
	if err := p.Publish(ctx, config); err != nil {
		return err
	}

	// Wait for confirmation
	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			emit.Error.StructuredFields("Message not acknowledged by broker",
				emit.ZString("exchange", config.Exchange),
				emit.ZString("routing_key", config.RoutingKey),
				emit.ZString("delivery_tag", fmt.Sprintf("%d", confirm.DeliveryTag)))
			return fmt.Errorf("message was not acknowledged by broker")
		}
		emit.Debug.StructuredFields("Message confirmed by broker",
			emit.ZString("exchange", config.Exchange),
			emit.ZString("routing_key", config.RoutingKey),
			emit.ZString("delivery_tag", fmt.Sprintf("%d", confirm.DeliveryTag)))
		return nil
	case <-ctx.Done():
		emit.Warn.StructuredFields("Confirmation canceled by context",
			emit.ZString("exchange", config.Exchange),
			emit.ZString("routing_key", config.RoutingKey))
		return ctx.Err()
	case <-time.After(p.config.ConfirmationTimeout):
		emit.Error.StructuredFields("Timeout waiting for confirmation",
			emit.ZString("exchange", config.Exchange),
			emit.ZString("routing_key", config.RoutingKey),
			emit.ZDuration("timeout", p.config.ConfirmationTimeout))
		return fmt.Errorf("timeout waiting for confirmation after %v", p.config.ConfirmationTimeout)
	}
}

// Close closes the publisher connection with graceful shutdown timeout
func (p *Publisher) Close() error {
	if p.conn != nil {
		// Wait for in-flight operations to complete first
		emit.Info.Msg("Waiting for in-flight publish operations to complete")
		if p.config.ShutdownTimeout > 0 {
			if err := p.inFlight.CloseWithTimeout(p.config.ShutdownTimeout / 2); err != nil {
				emit.Warn.StructuredFields("Timeout waiting for in-flight operations",
					emit.ZDuration("timeout", p.config.ShutdownTimeout/2))
			}
		} else {
			_ = p.inFlight.Close() // Ignore error during shutdown
		}

		// If shutdown timeout is configured, wait for graceful shutdown
		if p.config.ShutdownTimeout > 0 {
			// Create a timeout context for shutdown
			shutdownCtx, cancel := context.WithTimeout(context.Background(), p.config.ShutdownTimeout)
			defer cancel()

			// Signal shutdown and wait for completion or timeout
			done := make(chan error, 1)
			go func() {
				done <- p.conn.Close()
			}()

			select {
			case err := <-done:
				if err != nil {
					emit.Error.StructuredFields("Error during publisher connection close",
						emit.ZString("error", err.Error()))
				} else {
					emit.Info.StructuredFields("Publisher closed gracefully",
						emit.ZDuration("shutdown_timeout", p.config.ShutdownTimeout))
				}
				return err
			case <-shutdownCtx.Done():
				emit.Warn.StructuredFields("Publisher shutdown timeout exceeded, forcing close",
					emit.ZDuration("timeout", p.config.ShutdownTimeout))
				// Force close the connection
				return p.conn.Close()
			}
		} else {
			// No timeout configured, close immediately
			return p.conn.Close()
		}
	}
	return nil
}

// IsConnected checks if the publisher is connected
func (p *Publisher) IsConnected() bool {
	return p.conn != nil && p.conn.IsConnected()
}

// GetConnection returns the underlying connection for advanced operations
func (p *Publisher) GetConnection() *Connection {
	return p.conn
}
