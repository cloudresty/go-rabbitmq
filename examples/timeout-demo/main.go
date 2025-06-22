package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

// This example demonstrates timeout functionality across the RabbitMQ package
func main() {
	emit.Info.Msg("RabbitMQ Timeout Configuration Demo")

	// Connection with custom timeouts
	connConfig := rabbitmq.DefaultConnectionConfig("amqp://localhost:5672")
	connConfig.DialTimeout = time.Second * 15   // Custom dial timeout
	connConfig.ChannelTimeout = time.Second * 5 // Custom channel timeout
	connConfig.ConnectionName = "timeout-demo"

	// Publisher with custom confirmation timeout
	publisherConfig := rabbitmq.PublisherConfig{
		ConnectionConfig:    connConfig,
		ConfirmationTimeout: time.Second * 10, // Extended confirmation timeout
		Persistent:          true,
	}

	// Consumer with custom message and shutdown timeouts
	consumerConfig := rabbitmq.ConsumerConfig{
		ConnectionConfig: connConfig,
		MessageTimeout:   time.Second * 30, // 30-second message processing timeout
		ShutdownTimeout:  time.Second * 15, // 15-second graceful shutdown timeout
		PrefetchCount:    1,
		AutoAck:          false,
	}

	fmt.Printf("Configuration:\n")
	fmt.Printf("  Dial Timeout: %v\n", connConfig.DialTimeout)
	fmt.Printf("  Channel Timeout: %v\n", connConfig.ChannelTimeout)
	fmt.Printf("  Confirmation Timeout: %v\n", publisherConfig.ConfirmationTimeout)
	fmt.Printf("  Message Timeout: %v\n", consumerConfig.MessageTimeout)
	fmt.Printf("  Shutdown Timeout: %v\n", consumerConfig.ShutdownTimeout)

	// Demonstrate publisher timeout configuration
	emit.Info.Msg("Creating publisher with custom timeout configuration...")
	publisher, err := rabbitmq.NewPublisherWithConfig(publisherConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = publisher.Close() // Ignore error during cleanup
	}()

	// Demonstrate consumer timeout configuration
	emit.Info.Msg("Creating consumer with custom timeout configuration...")
	consumer, err := rabbitmq.NewConsumerWithConfig(consumerConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = consumer.Close() // Ignore error during cleanup
	}()

	emit.Info.Msg("Timeout configuration demo completed successfully!")
	emit.Info.Msg("All timeout configurations are now applied and ready for use.")

	// Demonstrate timeout behavior (conceptual - requires actual RabbitMQ connection)
	demonstrateTimeoutConcepts()
}

func demonstrateTimeoutConcepts() {
	fmt.Printf("\nTimeout Behavior Examples:\n")

	fmt.Printf("\n1. Connection Timeouts:\n")
	fmt.Printf("   - DialTimeout: Controls TCP connection establishment time\n")
	fmt.Printf("   - ChannelTimeout: Controls AMQP channel creation time\n")

	fmt.Printf("\n2. Publisher Timeouts:\n")
	fmt.Printf("   - ConfirmationTimeout: Controls how long to wait for publish confirmations\n")
	fmt.Printf("   - Prevents indefinite blocking on slow broker responses\n")

	fmt.Printf("\n3. Consumer Timeouts:\n")
	fmt.Printf("   - MessageTimeout: Automatically cancels long-running message handlers\n")
	fmt.Printf("   - ShutdownTimeout: Controls graceful shutdown duration\n")
	fmt.Printf("   - Prevents stuck consumers and ensures clean resource cleanup\n")

	fmt.Printf("\n4. Timeout Benefits:\n")
	fmt.Printf("   - ✅ Prevents indefinite blocking\n")
	fmt.Printf("   - ✅ Enables graceful degradation\n")
	fmt.Printf("   - ✅ Improves system resilience\n")
	fmt.Printf("   - ✅ Facilitates better error handling\n")
	fmt.Printf("   - ✅ Provides predictable performance characteristics\n")
}
