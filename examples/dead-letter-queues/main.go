package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

func main() {

	fmt.Println("=== Dead Letter Queue Infrastructure Demo ===")

	// Create publisher
	publisher, err := rabbitmq.NewPublisher("amqp://guest:guest@localhost:5672/")
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer publisher.Close()

	// Create consumer
	consumer, err := rabbitmq.NewConsumer("amqp://guest:guest@localhost:5672/")
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer consumer.Close()

	fmt.Println("1. Setting up topology with automatic dead letter infrastructure...")

	// Setup topology with automatic dead letter queues
	exchanges := []rabbitmq.ExchangeConfig{
		{
			Name:    "orders",
			Type:    rabbitmq.ExchangeTypeDirect,
			Durable: true,
		},
	}

	// Default configuration with auto-DLX enabled
	queues := []rabbitmq.QueueConfig{
		// Default quorum queue with auto-DLX (creates: orders-processing.dlx and orders-processing.dlq)
		rabbitmq.DefaultQuorumQueueConfig("orders-processing"),

		// Custom configuration with different DLX settings
		func() rabbitmq.QueueConfig {
			config := rabbitmq.DefaultQuorumQueueConfig("payments")
			return *config.WithDeadLetter(".dlx", ".dead", 3) // 3-day TTL in DLQ
		}(),

		// Queue without dead letter infrastructure
		func() rabbitmq.QueueConfig {
			config := rabbitmq.DefaultQuorumQueueConfig("notifications")
			return *config.WithoutDeadLetter()
		}(),

		// Queue with custom dead letter exchange (manual setup)
		func() rabbitmq.QueueConfig {
			config := rabbitmq.DefaultQuorumQueueConfig("priority-orders")
			return *config.WithCustomDeadLetter("manual-dlx", "failed.priority")
		}(),
	}

	bindings := []rabbitmq.BindingConfig{
		{
			QueueName:    "orders-processing",
			ExchangeName: "orders",
			RoutingKey:   "order.new",
		},
		{
			QueueName:    "payments",
			ExchangeName: "orders",
			RoutingKey:   "payment.process",
		},
		{
			QueueName:    "notifications",
			ExchangeName: "orders",
			RoutingKey:   "notification.send",
		},
		{
			QueueName:    "priority-orders",
			ExchangeName: "orders",
			RoutingKey:   "order.priority",
		},
	}

	err = rabbitmq.SetupTopology(publisher.GetConnection(), exchanges, queues, bindings)
	if err != nil {
		emit.Error.StructuredFields("Failed to setup topology",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	fmt.Println("✅ Topology created successfully!")
	fmt.Println("\nInfrastructure created:")
	fmt.Println("📦 Main Queues:")
	fmt.Println("   • orders-processing (with auto-DLX)")
	fmt.Println("   • payments (with custom DLX config)")
	fmt.Println("   • notifications (no DLX)")
	fmt.Println("   • priority-orders (custom DLX)")
	fmt.Println("\n⚰️  Dead Letter Exchanges:")
	fmt.Println("   • orders-processing.dlx")
	fmt.Println("   • payments.dlx")
	fmt.Println("   • manual-dlx (custom)")
	fmt.Println("\n💀 Dead Letter Queues:")
	fmt.Println("   • orders-processing.dlq (7-day TTL)")
	fmt.Println("   • payments.dead (3-day TTL)")
	fmt.Println("   • Manual DLQ bound to manual-dlx")

	fmt.Println("\n2. Publishing test messages...")

	// Publish messages to demonstrate the setup
	messages := []struct {
		exchange    string
		routingKey  string
		content     string
		description string
	}{
		{"orders", "order.new", `{"order_id": "12345", "amount": 99.99}`, "Order processing message"},
		{"orders", "payment.process", `{"payment_id": "pay-67890", "amount": 99.99}`, "Payment processing message"},
		{"orders", "notification.send", `{"user_id": "user-123", "type": "email"}`, "Notification message (no DLX)"},
		{"orders", "order.priority", `{"priority": "high", "order_id": "vip-456"}`, "Priority order message"},
	}

	for _, msg := range messages {
		message := rabbitmq.NewMessage([]byte(msg.content)).
			WithType("order.event").
			WithHeader("description", msg.description)

		err := publisher.PublishMessage(context.Background(), rabbitmq.PublishMessageConfig{
			Exchange:   msg.exchange,
			RoutingKey: msg.routingKey,
			Message:    message,
		})

		if err != nil {
			emit.Error.StructuredFields("Failed to publish message",
				emit.ZString("routing_key", msg.routingKey),
				emit.ZString("error", err.Error()))
		} else {
			emit.Info.StructuredFields("Message published",
				emit.ZString("routing_key", msg.routingKey),
				emit.ZString("message_id", message.MessageID),
				emit.ZString("description", msg.description))
		}
	}

	fmt.Println("\n3. Dead Letter Infrastructure Benefits:")
	fmt.Println("✅ Automatic Setup: No manual DLX/DLQ creation needed")
	fmt.Println("✅ Production Ready: Proper replication and durability")
	fmt.Println("✅ Configurable TTL: Messages auto-expire from DLQ")
	fmt.Println("✅ Flexible Options: Enable/disable per queue")
	fmt.Println("✅ Best Practices: Follows RabbitMQ recommendations")
	fmt.Println("✅ Debugging Aid: Failed messages preserved for analysis")
	fmt.Println("✅ Operational Safety: Prevents message loss on processing failures")

	fmt.Println("\n4. Usage Patterns:")
	fmt.Println("// Enable auto-DLX (default)")
	fmt.Println("config := rabbitmq.DefaultQuorumQueueConfig(\"orders\")")
	fmt.Println("// config.AutoCreateDLX = true (default)")
	fmt.Println("")
	fmt.Println("// Disable auto-DLX")
	fmt.Println("config.WithoutDeadLetter()")
	fmt.Println("")
	fmt.Println("// Custom DLX settings")
	fmt.Println("config.WithDeadLetter(\".dlx\", \".dead\", 7) // 7-day TTL")
	fmt.Println("")
	fmt.Println("// Manual DLX (existing infrastructure)")
	fmt.Println("config.WithCustomDeadLetter(\"my-dlx\", \"failed.routing\")")

	time.Sleep(100 * time.Millisecond) // Brief pause for message processing

	fmt.Println("\n✅ Dead Letter Queue infrastructure demo completed!")
	fmt.Println("Check your RabbitMQ management console to see the created infrastructure.")
}
