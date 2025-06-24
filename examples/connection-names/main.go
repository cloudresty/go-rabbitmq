package main

import (
	"context"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

func main() {
	emit.Info.Msg("Starting simplified connection example")

	// Create publisher using environment configuration
	publisher, err := rabbitmq.NewPublisher()
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher",
			emit.ZString("error", err.Error()),
			emit.ZString("hint", "Set RABBITMQ_* environment variables"))
		return
	}
	defer func() {
		_ = publisher.Close()
	}()

	emit.Info.Msg("Publisher created successfully")

	// Create consumer using environment configuration
	consumer, err := rabbitmq.NewConsumer()
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer",
			emit.ZString("error", err.Error()))
		return
	}
	defer func() {
		_ = consumer.Close()
	}()

	emit.Info.Msg("Consumer created successfully")

	// Publish a test message
	ctx := context.Background()
	err = publisher.Publish(ctx, rabbitmq.PublishConfig{
		Exchange:    "orders",
		RoutingKey:  "order.created",
		Message:     []byte(`{"order_id": "123", "status": "created"}`),
		ContentType: "application/json",
	})

	if err != nil {
		emit.Error.StructuredFields("Failed to publish message",
			emit.ZString("error", err.Error()))
	} else {
		emit.Info.Msg("Message published successfully")
	}

	emit.Info.Msg("Connection example completed!")
}
