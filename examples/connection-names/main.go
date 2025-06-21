package main

import (
	"context"
	"os"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

func main() {
	// Example 1: Publisher with custom connection name and auto-reconnection
	publisherConfig := rabbitmq.PublisherConfig{
		ConnectionConfig: rabbitmq.ConnectionConfig{
			URL:                  "amqp://guest:guest@localhost:5672/",
			RetryAttempts:        3,
			RetryDelay:           time.Second * 2,
			Heartbeat:            time.Second * 10,
			ConnectionName:       "order-service-publisher", // Custom connection name
			AutoReconnect:        true,                      // Enable auto-reconnection
			ReconnectDelay:       time.Second * 5,           // Wait 5s between reconnection attempts
			MaxReconnectAttempts: 0,                         // Unlimited reconnection attempts
		},
		DefaultExchange: "orders",
		Persistent:      true,
	}

	publisher, err := rabbitmq.NewPublisherWithConfig(publisherConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer publisher.Close()

	// Example 2: Consumer with custom connection name and auto-reconnection
	consumerConfig := rabbitmq.ConsumerConfig{
		ConnectionConfig: rabbitmq.ConnectionConfig{
			URL:                  "amqp://guest:guest@localhost:5672/",
			RetryAttempts:        3,
			RetryDelay:           time.Second * 2,
			Heartbeat:            time.Second * 10,
			ConnectionName:       "order-service-consumer", // Custom connection name
			AutoReconnect:        true,                     // Enable auto-reconnection
			ReconnectDelay:       time.Second * 5,          // Wait 5s between reconnection attempts
			MaxReconnectAttempts: 10,                       // Max 10 reconnection attempts
		},
		AutoAck:       false,
		PrefetchCount: 5,
	}

	consumer, err := rabbitmq.NewConsumerWithConfig(consumerConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer consumer.Close()

	// Publish a test message
	err = publisher.Publish(context.Background(), rabbitmq.PublishConfig{
		Exchange:   "orders",
		RoutingKey: "order.created",
		Message:    []byte(`{"order_id": "123", "amount": 99.99}`),
	})
	if err != nil {
		emit.Error.StructuredFields("Failed to publish message",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	emit.Info.StructuredFields("Published message with custom connection name",
		emit.ZString("connection_name", publisherConfig.ConnectionName))

	// Start consuming with custom connection name
	err = consumer.Consume(context.Background(), rabbitmq.ConsumeConfig{
		Queue: "order-processing",
		Handler: func(ctx context.Context, message []byte) error {
			emit.Info.StructuredFields("Received order",
				emit.ZString("message", string(message)),
				emit.ZString("connection_name", consumerConfig.ConnectionName))
			return nil
		},
	})
	if err != nil {
		emit.Error.StructuredFields("Consumer error",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
}
