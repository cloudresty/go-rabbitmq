package main

import (
	"context"
	"os"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

func main() {
	emit.Info.Msg("Starting production queues example with environment configuration")

	// Create publisher and consumer from environment variables
	publisher, err := rabbitmq.NewPublisher()
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher from environment",
			emit.ZString("error", err.Error()),
			emit.ZString("hint", "Set RABBITMQ_* environment variables or use defaults"))
		os.Exit(1)
	}
	defer func() {
		_ = publisher.Close() // Ignore error during cleanup
	}()

	consumer, err := rabbitmq.NewConsumer()
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer from environment",
			emit.ZString("error", err.Error()),
			emit.ZString("hint", "Set RABBITMQ_* environment variables or use defaults"))
		os.Exit(1)
	}
	defer func() {
		_ = consumer.Close() // Ignore error during cleanup
	}()

	emit.Info.Msg("Publisher and consumer created successfully from environment configuration")

	// Example 1: Create a production-ready Quorum Queue
	emit.Info.Msg("Creating Quorum Queue for high availability...")
	err = publisher.DeclareQuorumQueue("orders-quorum")
	if err != nil {
		emit.Error.StructuredFields("Failed to declare quorum queue",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Example 2: Create a production-ready HA Classic Queue
	emit.Info.Msg("Creating HA Classic Queue for compatibility...")
	err = publisher.DeclareHAQueue("notifications-ha")
	if err != nil {
		emit.Error.StructuredFields("Failed to declare HA queue",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Example 3: Custom Quorum Queue with specific settings
	emit.Info.Msg("Creating custom Quorum Queue with specific settings...")
	customQuorumConfig := rabbitmq.QueueConfig{
		Name:               "payments-quorum",
		Durable:            true,
		QueueType:          rabbitmq.QueueTypeQuorum,
		ReplicationFactor:  5,                   // 5-node quorum for critical data
		MaxLength:          100000,              // Limit to 100k messages
		MessageTTL:         int(time.Hour * 24), // 24-hour TTL in milliseconds
		DeadLetterExchange: "payments-dlx",      // Dead letter exchange
		Arguments:          make(map[string]any),
	}
	err = publisher.DeclareQueueWithConfig(customQuorumConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to declare custom quorum queue",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Example 4: Custom HA Queue with size limits and DLX
	emit.Info.Msg("Creating custom HA Queue with size limits...")
	customHAConfig := rabbitmq.QueueConfig{
		Name:                 "events-ha",
		Durable:              true,
		QueueType:            rabbitmq.QueueTypeClassic,
		HighAvailability:     true,
		MaxLengthBytes:       1024 * 1024 * 100,  // 100MB limit
		MessageTTL:           int(time.Hour * 6), // 6-hour TTL in milliseconds
		DeadLetterExchange:   "events-dlx",       // Dead letter exchange
		DeadLetterRoutingKey: "failed.events",    // Dead letter routing key
		Arguments:            make(map[string]any),
	}
	err = publisher.DeclareQueueWithConfig(customHAConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to declare custom HA queue",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Example 5: Use individual queue declarations instead of topology setup
	emit.Info.Msg("Creating individual production-ready queues...")

	// Create exchanges first
	err = publisher.DeclareExchange("orders", "topic", true, false, false, false, nil)
	if err != nil {
		emit.Error.StructuredFields("Failed to declare orders exchange",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	err = publisher.DeclareExchange("orders-dlx", "direct", true, false, false, false, nil)
	if err != nil {
		emit.Error.StructuredFields("Failed to declare DLX exchange",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Create production-ready queues
	queueConfigs := []rabbitmq.QueueConfig{
		rabbitmq.DefaultQuorumQueueConfig("order-processing"),
		rabbitmq.DefaultQuorumQueueConfig("order-validation"),
		rabbitmq.DefaultHAQueueConfig("order-notifications"),
	}

	for _, config := range queueConfigs {
		err = publisher.DeclareQueueWithConfig(config)
		if err != nil {
			emit.Error.StructuredFields("Failed to declare queue",
				emit.ZString("queue_name", config.Name),
				emit.ZString("error", err.Error()))
			os.Exit(1)
		}
	}

	// Publish test messages to demonstrate the production-ready queues
	emit.Info.Msg("Publishing test messages to production-ready queues...")

	testMessages := []struct {
		exchange   string
		routingKey string
		message    string
	}{
		{"orders", "order.created", `{"order_id": "12345", "amount": 99.99}`},
		{"orders", "order.validation.required", `{"order_id": "12345", "status": "validation_required"}`},
		{"orders", "order.notification.email", `{"order_id": "12345", "type": "confirmation_email"}`},
	}

	for _, msg := range testMessages {
		err = publisher.Publish(context.Background(), rabbitmq.PublishConfig{
			Exchange:   msg.exchange,
			RoutingKey: msg.routingKey,
			Message:    []byte(msg.message),
		})
		if err != nil {
			emit.Error.StructuredFields("Failed to publish test message",
				emit.ZString("exchange", msg.exchange),
				emit.ZString("routing_key", msg.routingKey),
				emit.ZString("error", err.Error()))
		} else {
			emit.Info.StructuredFields("Published test message",
				emit.ZString("exchange", msg.exchange),
				emit.ZString("routing_key", msg.routingKey),
				emit.ZString("message", msg.message))
		}
	}

	emit.Info.Msg("Production-ready queue setup complete!")
	emit.Info.Msg("Check RabbitMQ Management UI to see:")
	emit.Info.Msg("- Quorum queues with replication")
	emit.Info.Msg("- HA classic queues with mirroring")
	emit.Info.Msg("- Message TTL and size limits")
	emit.Info.Msg("- Dead letter exchange configuration")
}
