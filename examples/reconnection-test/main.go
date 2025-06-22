package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

func main() {
	// Create a publisher with auto-reconnection enabled
	publisherConfig := rabbitmq.PublisherConfig{
		ConnectionConfig: rabbitmq.ConnectionConfig{
			URL:                  "amqp://guest:guest@localhost:5672/",
			ConnectionName:       "reconnection-test-publisher",
			AutoReconnect:        true,
			ReconnectDelay:       time.Second * 3,
			MaxReconnectAttempts: 0, // Unlimited
			Heartbeat:            time.Second * 5,
		},
	}

	publisher, err := rabbitmq.NewPublisherWithConfig(publisherConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = publisher.Close() // Ignore error during cleanup
	}()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start publishing messages periodically
	go func() {
		ticker := time.NewTicker(time.Second * 2)
		defer ticker.Stop()

		messageCount := 0
		for {
			select {
			case <-ticker.C:
				messageCount++
				message := fmt.Sprintf("Test message #%d at %s", messageCount, time.Now().Format(time.RFC3339))

				err := publisher.Publish(context.Background(), rabbitmq.PublishConfig{
					Exchange:   "",
					RoutingKey: "test-queue",
					Message:    []byte(message),
				})

				if err != nil {
					emit.Error.StructuredFields("Failed to publish message",
						emit.ZString("error", err.Error()),
						emit.ZInt("message_count", messageCount))
				} else {
					emit.Info.StructuredFields("Published message successfully",
						emit.ZInt("message_count", messageCount),
						emit.ZString("message", message))
				}

			case <-sigChan:
				emit.Info.Msg("Received shutdown signal, stopping publisher...")
				return
			}
		}
	}()

	emit.Info.Msg("Auto-reconnection test started. Try stopping/starting RabbitMQ to test reconnection...")
	emit.Info.Msg("Press Ctrl+C to stop")

	// Wait for shutdown signal
	<-sigChan
	emit.Info.Msg("Shutting down...")
}
