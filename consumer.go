package rabbitmq

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudresty/emit"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Consumer handles consuming messages from RabbitMQ
type Consumer struct {
	conn   *Connection
	config ConsumerConfig
	mu     sync.RWMutex
	cancel context.CancelFunc
}

// ConsumerConfig holds configuration for the consumer
type ConsumerConfig struct {
	ConnectionConfig
	AutoAck        bool
	Exclusive      bool
	NoLocal        bool
	NoWait         bool
	PrefetchCount  int
	PrefetchSize   int
	PrefetchGlobal bool
}

// ConsumeConfig holds configuration for a single consume operation
type ConsumeConfig struct {
	Queue     string
	Consumer  string
	Handler   MessageHandler
	AutoAck   *bool // Optional override for auto-acknowledgment
	Exclusive *bool // Optional override for exclusive consumption
	Arguments map[string]interface{}
}

// MessageHandler is a function type for handling consumed messages
type MessageHandler func(ctx context.Context, message []byte) error

// DeliveryHandler is a function type for handling raw AMQP deliveries
type DeliveryHandler func(ctx context.Context, delivery amqp.Delivery) error

// NewConsumer creates a new consumer with default configuration
func NewConsumer(url string) (*Consumer, error) {
	config := ConsumerConfig{
		ConnectionConfig: DefaultConnectionConfig(url),
		AutoAck:          false,
		Exclusive:        false,
		NoLocal:          false,
		NoWait:           false,
		PrefetchCount:    1,
		PrefetchSize:     0,
		PrefetchGlobal:   false,
	}
	config.ConnectionConfig.ConnectionName = "go-rabbitmq-consumer"

	return NewConsumerWithConfig(config)
}

// NewConsumerWithConfig creates a new consumer with custom configuration
func NewConsumerWithConfig(config ConsumerConfig) (*Consumer, error) {
	emit.Info.StructuredFields("Creating RabbitMQ consumer",
		emit.ZString("connection_name", config.ConnectionConfig.ConnectionName),
		emit.ZBool("auto_ack", config.AutoAck),
		emit.ZInt("prefetch_count", config.PrefetchCount))

	conn, err := NewConnection(config.ConnectionConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer connection",
			emit.ZString("error", err.Error()),
			emit.ZString("connection_name", config.ConnectionConfig.ConnectionName))
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	consumer := &Consumer{
		conn:   conn,
		config: config,
	}

	// Set QoS
	if err := consumer.setQoS(); err != nil {
		emit.Error.StructuredFields("Failed to set QoS for consumer",
			emit.ZString("error", err.Error()),
			emit.ZInt("prefetch_count", config.PrefetchCount))
		conn.Close()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	emit.Info.StructuredFields("RabbitMQ consumer created successfully",
		emit.ZString("connection_name", config.ConnectionConfig.ConnectionName))

	return consumer, nil
}

// setQoS sets the Quality of Service for the consumer
func (c *Consumer) setQoS() error {
	return c.conn.Channel().Qos(
		c.config.PrefetchCount,  // prefetch count
		c.config.PrefetchSize,   // prefetch size
		c.config.PrefetchGlobal, // global
	)
}

// DeclareQueue declares a queue with the given parameters
func (c *Consumer) DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args map[string]any) (amqp.Queue, error) {
	if !c.conn.IsConnected() {
		emit.Error.StructuredFields("Consumer not connected when declaring queue",
			emit.ZString("queue_name", name))
		return amqp.Queue{}, fmt.Errorf("consumer is not connected")
	}

	emit.Debug.StructuredFields("Declaring RabbitMQ queue",
		emit.ZString("queue_name", name),
		emit.ZBool("durable", durable),
		emit.ZBool("auto_delete", autoDelete),
		emit.ZBool("exclusive", exclusive))

	queue, err := c.conn.Channel().QueueDeclare(
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
			emit.ZString("queue_name", name),
			emit.ZInt("message_count", int(queue.Messages)),
			emit.ZInt("consumer_count", int(queue.Consumers)))
	}

	return queue, err
}

// DeclareQuorumQueue declares a production-ready quorum queue
func (c *Consumer) DeclareQuorumQueue(name string) (amqp.Queue, error) {
	config := DefaultQuorumQueueConfig(name)
	return c.DeclareQueueWithConfig(config)
}

// DeclareHAQueue declares a production-ready HA classic queue
func (c *Consumer) DeclareHAQueue(name string) (amqp.Queue, error) {
	config := DefaultHAQueueConfig(name)
	return c.DeclareQueueWithConfig(config)
}

// DeclareQueueWithConfig declares a queue using QueueConfig
func (c *Consumer) DeclareQueueWithConfig(config QueueConfig) (amqp.Queue, error) {
	if !c.conn.IsConnected() {
		emit.Error.StructuredFields("Consumer not connected when declaring queue",
			emit.ZString("queue_name", config.Name),
			emit.ZString("queue_type", string(config.QueueType)))
		return amqp.Queue{}, fmt.Errorf("consumer is not connected")
	}

	args := config.ToArguments()

	emit.Info.StructuredFields("Declaring queue with enhanced config",
		emit.ZString("queue_name", config.Name),
		emit.ZString("queue_type", string(config.QueueType)),
		emit.ZBool("durable", config.Durable),
		emit.ZBool("high_availability", config.HighAvailability),
		emit.ZInt("replication_factor", config.ReplicationFactor))

	queue, err := c.conn.Channel().QueueDeclare(
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
			emit.ZString("queue_type", string(config.QueueType)),
			emit.ZInt("message_count", int(queue.Messages)),
			emit.ZInt("consumer_count", int(queue.Consumers)))
	}

	return queue, err
}

// BindQueue binds a queue to an exchange
func (c *Consumer) BindQueue(queueName, routingKey, exchangeName string, noWait bool, args map[string]any) error {
	if !c.conn.IsConnected() {
		emit.Error.StructuredFields("Consumer not connected when binding queue",
			emit.ZString("queue_name", queueName),
			emit.ZString("exchange_name", exchangeName),
			emit.ZString("routing_key", routingKey))
		return fmt.Errorf("consumer is not connected")
	}

	emit.Debug.StructuredFields("Binding queue to exchange",
		emit.ZString("queue_name", queueName),
		emit.ZString("exchange_name", exchangeName),
		emit.ZString("routing_key", routingKey))

	err := c.conn.Channel().QueueBind(
		queueName,    // queue name
		routingKey,   // routing key
		exchangeName, // exchange
		noWait,       // no-wait
		args,         // arguments
	)

	if err != nil {
		emit.Error.StructuredFields("Failed to bind queue to exchange",
			emit.ZString("queue_name", queueName),
			emit.ZString("exchange_name", exchangeName),
			emit.ZString("routing_key", routingKey),
			emit.ZString("error", err.Error()))
	} else {
		emit.Info.StructuredFields("Queue bound to exchange successfully",
			emit.ZString("queue_name", queueName),
			emit.ZString("exchange_name", exchangeName),
			emit.ZString("routing_key", routingKey))
	}

	return err
}

// Consume starts consuming messages from a queue
func (c *Consumer) Consume(ctx context.Context, config ConsumeConfig) error {
	if !c.conn.IsConnected() {
		return fmt.Errorf("consumer is not connected")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Create a cancellable context
	consumeCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	// Resolve configuration settings
	autoAck, exclusive := c.resolveConsumeSettings(config)

	// Start consuming
	deliveries, err := c.conn.Channel().Consume(
		config.Queue,     // queue
		config.Consumer,  // consumer
		autoAck,          // auto-ack
		exclusive,        // exclusive
		c.config.NoLocal, // no-local
		c.config.NoWait,  // no-wait
		config.Arguments, // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	// Handle connection close events
	closeChan := c.conn.NotifyClose()

	// Process messages in a separate goroutine
	go c.processMessages(consumeCtx, config, deliveries, closeChan, autoAck)

	// Wait for context cancellation
	<-consumeCtx.Done()
	return consumeCtx.Err()
}

// resolveConsumeSettings resolves configuration overrides for consume operation
func (c *Consumer) resolveConsumeSettings(config ConsumeConfig) (autoAck bool, exclusive bool) {
	autoAck = c.config.AutoAck
	if config.AutoAck != nil {
		autoAck = *config.AutoAck
	}

	exclusive = c.config.Exclusive
	if config.Exclusive != nil {
		exclusive = *config.Exclusive
	}

	return autoAck, exclusive
}

// processMessages handles the message processing loop
func (c *Consumer) processMessages(ctx context.Context, config ConsumeConfig, deliveries <-chan amqp.Delivery, closeChan <-chan *amqp.Error, autoAck bool) {
	for {
		select {
		case delivery, ok := <-deliveries:
			if !ok {
				emit.Info.Msg("Consumer delivery channel closed")
				return
			}
			c.handleDelivery(ctx, config, delivery, autoAck)

		case closeErr := <-closeChan:
			c.handleConnectionClose(closeErr, config.Queue)
			return

		case <-ctx.Done():
			emit.Info.StructuredFields("Consumer context canceled",
				emit.ZString("queue", config.Queue))
			return
		}
	}
}

// handleDelivery processes a single message delivery
func (c *Consumer) handleDelivery(ctx context.Context, config ConsumeConfig, delivery amqp.Delivery, autoAck bool) {
	err := config.Handler(ctx, delivery.Body)
	if err != nil {
		c.handleMessageError(err, config.Queue, delivery.RoutingKey, autoAck, delivery)
	} else {
		c.handleMessageSuccess(config.Queue, autoAck, delivery)
	}
}

// handleMessageError handles errors during message processing
func (c *Consumer) handleMessageError(err error, queue, routingKey string, autoAck bool, delivery amqp.Delivery) {
	emit.Error.StructuredFields("Error handling message",
		emit.ZString("error", err.Error()),
		emit.ZString("queue", queue),
		emit.ZString("routing_key", routingKey))

	if !autoAck {
		if nackErr := delivery.Nack(false, true); nackErr != nil { // Reject and requeue
			emit.Error.StructuredFields("Failed to nack message",
				emit.ZString("error", nackErr.Error()),
				emit.ZString("queue", queue))
		}
	}
}

// handleMessageSuccess handles successful message processing
func (c *Consumer) handleMessageSuccess(queue string, autoAck bool, delivery amqp.Delivery) {
	if !autoAck {
		if ackErr := delivery.Ack(false); ackErr != nil { // Acknowledge
			emit.Error.StructuredFields("Failed to ack message",
				emit.ZString("error", ackErr.Error()),
				emit.ZString("queue", queue))
		}
	}
}

// handleConnectionClose handles connection close events
func (c *Consumer) handleConnectionClose(closeErr *amqp.Error, queue string) {
	if closeErr != nil {
		emit.Warn.StructuredFields("Connection closed",
			emit.ZString("error", closeErr.Error()),
			emit.ZString("queue", queue))
	}
}

// ConsumeWithDeliveryHandler starts consuming with a raw delivery handler
func (c *Consumer) ConsumeWithDeliveryHandler(ctx context.Context, config ConsumeConfig, handler DeliveryHandler) error {
	if !c.conn.IsConnected() {
		return fmt.Errorf("consumer is not connected")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Create a cancellable context
	consumeCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	// Use defaults if not specified
	autoAck := c.config.AutoAck
	if config.AutoAck != nil {
		autoAck = *config.AutoAck
	}

	exclusive := c.config.Exclusive
	if config.Exclusive != nil {
		exclusive = *config.Exclusive
	}

	// Start consuming
	deliveries, err := c.conn.Channel().Consume(
		config.Queue,     // queue
		config.Consumer,  // consumer
		autoAck,          // auto-ack
		exclusive,        // exclusive
		c.config.NoLocal, // no-local
		c.config.NoWait,  // no-wait
		config.Arguments, // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	// Handle connection close events
	closeChan := c.conn.NotifyClose()

	// Process messages
	go func() {
		for {
			select {
			case delivery, ok := <-deliveries:
				if !ok {
					emit.Info.Msg("Consumer delivery channel closed")
					return
				}

				// Handle the raw delivery
				err := handler(consumeCtx, delivery)
				if err != nil {
					emit.Error.StructuredFields("Error handling delivery",
						emit.ZString("error", err.Error()),
						emit.ZString("queue", config.Queue),
						emit.ZString("routing_key", delivery.RoutingKey))
				}

			case closeErr := <-closeChan:
				if closeErr != nil {
					emit.Warn.StructuredFields("Connection closed",
						emit.ZString("error", closeErr.Error()),
						emit.ZString("queue", config.Queue))
				}
				return

			case <-consumeCtx.Done():
				emit.Info.StructuredFields("Consumer context canceled",
					emit.ZString("queue", config.Queue))
				return
			}
		}
	}()

	// Wait for context cancellation
	<-consumeCtx.Done()
	return consumeCtx.Err()
}

// Stop stops the consumer
func (c *Consumer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
}

// Close closes the consumer connection
func (c *Consumer) Close() error {
	c.Stop()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetConnection returns the underlying connection for advanced operations
func (c *Consumer) GetConnection() *Connection {
	return c.conn
}

// IsConnected checks if the consumer is connected
func (c *Consumer) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}
