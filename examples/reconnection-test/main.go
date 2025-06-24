package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

func main() {
	emit.Info.Msg("Starting RabbitMQ reconnection test")

	// Create publisher and consumer with environment configuration
	publisher, err := rabbitmq.NewPublisher()
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer publisher.Close()

	consumer, err := rabbitmq.NewConsumer()
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer consumer.Close()

	// Test queue name
	testQueue := "reconnection-test-queue"

	// Declare the test queue
	_, err = consumer.DeclareQueue(testQueue, true, false, false, false, nil)
	if err != nil {
		emit.Error.StructuredFields("Failed to declare queue",
			emit.ZString("queue", testQueue),
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	emit.Info.StructuredFields("Test queue declared successfully",
		emit.ZString("queue", testQueue))

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Counter for messages
	var (
		messageCount   int
		publishedCount int
		consumedCount  int
		mu             sync.RWMutex
	)

	// Start consumer
	go func() {
		emit.Info.Msg("Starting message consumer...")

		err := consumer.Consume(ctx, rabbitmq.ConsumeConfig{
			Queue: testQueue,
			Handler: func(ctx context.Context, message []byte) error {
				mu.Lock()
				consumedCount++
				count := consumedCount
				mu.Unlock()

				emit.Info.StructuredFields("Message consumed",
					emit.ZString("message", string(message)),
					emit.ZInt("consumed_count", count),
					emit.ZInt("message_size", len(message)))

				return nil
			},
		})

		if err != nil {
			emit.Error.StructuredFields("Consumer error",
				emit.ZString("error", err.Error()))
		}
	}()

	// Start publisher
	go func() {
		emit.Info.Msg("Starting message publisher...")

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				emit.Info.Msg("Publisher context canceled")
				return
			case <-ticker.C:
				mu.Lock()
				messageCount++
				currentCount := messageCount
				mu.Unlock()

				message := fmt.Sprintf("Test message #%d at %s", currentCount, time.Now().Format(time.RFC3339))

				err := publisher.Publish(ctx, rabbitmq.PublishConfig{
					Exchange:   "",
					RoutingKey: testQueue,
					Message:    []byte(message),
				})

				if err != nil {
					emit.Error.StructuredFields("Failed to publish message",
						emit.ZInt("message_number", currentCount),
						emit.ZString("error", err.Error()))
				} else {
					mu.Lock()
					publishedCount++
					published := publishedCount
					mu.Unlock()

					emit.Info.StructuredFields("Message published successfully",
						emit.ZInt("message_number", currentCount),
						emit.ZInt("published_count", published),
						emit.ZString("message", message))
				}
			}
		}
	}()

	// Connection monitoring
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				publisherConnected := publisher.IsConnected()
				consumerConnected := consumer.IsConnected()

				mu.RLock()
				published := publishedCount
				consumed := consumedCount
				mu.RUnlock()

				emit.Info.StructuredFields("Connection status",
					emit.ZBool("publisher_connected", publisherConnected),
					emit.ZBool("consumer_connected", consumerConnected),
					emit.ZInt("published_count", published),
					emit.ZInt("consumed_count", consumed))

				if !publisherConnected || !consumerConnected {
					emit.Warn.StructuredFields("Connection issues detected",
						emit.ZBool("publisher_connected", publisherConnected),
						emit.ZBool("consumer_connected", consumerConnected))
				}
			}
		}
	}()

	// Instructions for testing reconnection
	emit.Info.Msg("=== Reconnection Test Instructions ===")
	emit.Info.Msg("1. The application is now running and publishing/consuming messages")
	emit.Info.Msg("2. To test reconnection, you can:")
	emit.Info.Msg("   a) Stop RabbitMQ server: sudo systemctl stop rabbitmq-server")
	emit.Info.Msg("   b) Or restart RabbitMQ: sudo systemctl restart rabbitmq-server")
	emit.Info.Msg("   c) Or disconnect network temporarily")
	emit.Info.Msg("3. Observe the logs - you should see:")
	emit.Info.Msg("   - Connection errors when RabbitMQ is unavailable")
	emit.Info.Msg("   - Automatic reconnection attempts")
	emit.Info.Msg("   - Successful reconnection when RabbitMQ is back")
	emit.Info.Msg("   - Continued message flow after reconnection")
	emit.Info.Msg("4. Press Ctrl+C to stop the test")
	emit.Info.Msg("=====================================")

	// Wait for shutdown signal
	<-sigChan
	emit.Info.Msg("Received shutdown signal, stopping reconnection test...")

	// Cancel context to stop all goroutines
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)

	// Final statistics
	mu.RLock()
	finalPublished := publishedCount
	finalConsumed := consumedCount
	mu.RUnlock()

	emit.Info.StructuredFields("Reconnection test completed",
		emit.ZInt("total_published", finalPublished),
		emit.ZInt("total_consumed", finalConsumed),
		emit.ZInt("message_loss", finalPublished-finalConsumed))

	if finalPublished == finalConsumed {
		emit.Info.Msg("No message loss detected during reconnection test")
	} else {
		emit.Warn.StructuredFields("Message loss detected",
			emit.ZInt("lost_messages", finalPublished-finalConsumed))
	}

	emit.Info.Msg("Reconnection test finished")
}
