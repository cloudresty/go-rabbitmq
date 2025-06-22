package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

// This example demonstrates comprehensive graceful shutdown functionality
func main() {
	emit.Info.Msg("RabbitMQ Graceful Shutdown Demo")

	// Create shutdown manager
	shutdownConfig := rabbitmq.DefaultShutdownConfig()
	shutdownManager := rabbitmq.NewShutdownManager(shutdownConfig)

	// Setup signal handling for graceful shutdown
	shutdownManager.SetupSignalHandler()

	// Create publisher with custom shutdown timeout
	publisherConfig := rabbitmq.PublisherConfig{
		ConnectionConfig: rabbitmq.DefaultConnectionConfig("amqp://localhost:5672"),
		ShutdownTimeout:  time.Second * 10, // 10-second graceful shutdown
		Persistent:       true,
	}
	publisherConfig.ConnectionName = "graceful-shutdown-publisher"

	publisher, err := rabbitmq.NewPublisherWithConfig(publisherConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Create consumer with custom shutdown timeout
	consumerConfig := rabbitmq.ConsumerConfig{
		ConnectionConfig: rabbitmq.DefaultConnectionConfig("amqp://localhost:5672"),
		MessageTimeout:   time.Second * 30, // 30-second message processing timeout
		ShutdownTimeout:  time.Second * 15, // 15-second graceful shutdown
		PrefetchCount:    1,
		AutoAck:          false,
	}
	consumerConfig.ConnectionName = "graceful-shutdown-consumer"

	consumer, err := rabbitmq.NewConsumerWithConfig(consumerConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Register components with shutdown manager
	shutdownManager.Register(publisher)
	shutdownManager.Register(consumer)

	// Declare exchange and queue
	err = publisher.DeclareExchange("graceful-demo", "direct", true, false, false, false, nil)
	if err != nil {
		emit.Error.StructuredFields("Failed to declare exchange",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	queue, err := consumer.DeclareQueue("graceful-demo-queue", true, false, false, false, nil)
	if err != nil {
		emit.Error.StructuredFields("Failed to declare queue",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	err = consumer.BindQueue(queue.Name, "demo", "graceful-demo", false, nil)
	if err != nil {
		emit.Error.StructuredFields("Failed to bind queue",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Start publishing messages in background
	go func() {
		ticker := time.NewTicker(time.Second * 2)
		defer ticker.Stop()

		messageCount := 0
		for range ticker.C {
			// Check if shutdown is in progress
			if shutdownManager.IsShutdown() {
				emit.Info.Msg("Publisher stopping due to graceful shutdown")
				return
			}

			messageCount++
			message := fmt.Sprintf(`{"message": "Hello from graceful shutdown demo", "count": %d, "timestamp": "%s"}`,
				messageCount, time.Now().Format(time.RFC3339))

			err := publisher.Publish(context.Background(), rabbitmq.PublishConfig{
				Exchange:    "graceful-demo",
				RoutingKey:  "demo",
				Message:     []byte(message),
				ContentType: "application/json",
			})

			if err != nil {
				emit.Error.StructuredFields("Failed to publish message",
					emit.ZString("error", err.Error()),
					emit.ZInt("message_count", messageCount))
				return
			}

			emit.Info.StructuredFields("Published message",
				emit.ZInt("message_count", messageCount))
		}
	}()

	// Start consuming messages
	consumeCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := consumer.Consume(consumeCtx, rabbitmq.ConsumeConfig{
			Queue:    queue.Name,
			Consumer: "graceful-demo-consumer",
			Handler: func(ctx context.Context, message []byte) error {
				emit.Info.StructuredFields("Processing message",
					emit.ZString("message", string(message)))

				// Simulate some processing time
				time.Sleep(time.Second * 3)

				emit.Info.StructuredFields("Message processed successfully",
					emit.ZString("message", string(message)))
				return nil
			},
		})

		if err != nil && err != context.Canceled {
			emit.Error.StructuredFields("Consumer error",
				emit.ZString("error", err.Error()))
		}
	}()

	emit.Info.Msg("Graceful shutdown demo running...")
	emit.Info.Msg("Send SIGINT (Ctrl+C) or SIGTERM to initiate graceful shutdown")

	// Wait for shutdown signal
	shutdownManager.Wait()

	emit.Info.Msg("Graceful shutdown completed!")

	// Demonstrate the benefits
	demonstrateGracefulShutdownBenefits()
}

func demonstrateGracefulShutdownBenefits() {
	fmt.Printf("\nGraceful Shutdown Benefits Demonstrated:\n")
	fmt.Printf("\n1. ✅ Signal Handling:\n")
	fmt.Printf("   - Automatic SIGINT/SIGTERM signal capture\n")
	fmt.Printf("   - Coordinated shutdown across multiple components\n")
	fmt.Printf("   - Clean resource cleanup\n")

	fmt.Printf("\n2. ✅ In-Flight Operation Tracking:\n")
	fmt.Printf("   - Publisher waits for pending publish confirmations\n")
	fmt.Printf("   - Consumer waits for current message processing to complete\n")
	fmt.Printf("   - No abrupt interruption of ongoing operations\n")

	fmt.Printf("\n3. ✅ Timeout Controls:\n")
	fmt.Printf("   - Configurable shutdown timeouts per component\n")
	fmt.Printf("   - Graceful degradation when timeouts are exceeded\n")
	fmt.Printf("   - Forced shutdown as fallback\n")

	fmt.Printf("\n4. ✅ Structured Logging:\n")
	fmt.Printf("   - Comprehensive shutdown event logging\n")
	fmt.Printf("   - Clear visibility into shutdown progress\n")
	fmt.Printf("   - Error tracking during shutdown\n")

	fmt.Printf("\n5. ✅ Production Safety:\n")
	fmt.Printf("   - Prevents data loss during shutdown\n")
	fmt.Printf("   - Ensures message acknowledgments are sent\n")
	fmt.Printf("   - Maintains system consistency\n")

	fmt.Printf("\n6. ✅ Operational Excellence:\n")
	fmt.Printf("   - Predictable shutdown behavior\n")
	fmt.Printf("   - Monitoring-friendly shutdown events\n")
	fmt.Printf("   - Zero-downtime deployment support\n")
}
