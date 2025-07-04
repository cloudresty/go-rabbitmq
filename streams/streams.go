// Package streams provides RabbitMQ streams functionality for high-throughput scenarios.
//
// RabbitMQ streams are a new data structure introduced in RabbitMQ 3.9+ that provides
// persistent, replicated, and high-throughput messaging. Streams are ideal for:
//   - Event sourcing applications
//   - Time-series data
//   - High-throughput messaging scenarios
//   - Cases where message order and durability are critical
//
// Features:
//   - High-throughput message publishing
//   - Durable message storage with configurable retention
//   - Consumer offset management
//   - Stream-specific configuration (max age, max size, etc.)
//
// Example usage:
//
//	// Create a stream handler
//	handler := streams.NewHandler(client)
//
//	// Create a stream
//	config := rabbitmq.StreamConfig{
//		MaxAge:            24 * time.Hour,
//		MaxLengthMessages: 1000000,
//	}
//	err := handler.CreateStream(ctx, "my-stream", config)
//
//	// Publish to stream
//	message := rabbitmq.NewMessage([]byte("hello"))
//	err = handler.PublishToStream(ctx, "my-stream", message)
//
//	// Consume from stream
//	err = handler.ConsumeFromStream(ctx, "my-stream", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
//		fmt.Printf("Received: %s\n", delivery.Body)
//		return nil
//	})
package streams

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudresty/go-rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler implements the rabbitmq.StreamHandler interface
type Handler struct {
	client *rabbitmq.Client
}

// NewHandler creates a new stream handler
func NewHandler(client *rabbitmq.Client) *Handler {
	return &Handler{
		client: client,
	}
}

// PublishToStream publishes a message to the specified stream
func (h *Handler) PublishToStream(ctx context.Context, streamName string, message *rabbitmq.Message) error {
	if h.client == nil {
		return fmt.Errorf("client is nil")
	}

	// Create a channel for publishing
	ch, err := h.client.CreateChannel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	// Declare the stream if it doesn't exist
	err = h.ensureStreamExists(ch, streamName)
	if err != nil {
		return fmt.Errorf("failed to ensure stream exists: %w", err)
	}

	// Publish the message
	err = ch.PublishWithContext(ctx, "", streamName, false, false, amqp.Publishing{
		ContentType: message.ContentType,
		Body:        message.Body,
		Headers:     message.Headers,
		Timestamp:   time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to publish message to stream: %w", err)
	}

	return nil
}

// ConsumeFromStream consumes messages from the specified stream
func (h *Handler) ConsumeFromStream(ctx context.Context, streamName string, handler rabbitmq.StreamMessageHandler) error {
	if h.client == nil {
		return fmt.Errorf("client is nil")
	}

	// Create a channel for consuming
	ch, err := h.client.CreateChannel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	// Declare the stream if it doesn't exist
	err = h.ensureStreamExists(ch, streamName)
	if err != nil {
		return fmt.Errorf("failed to ensure stream exists: %w", err)
	}

	// Create a consumer
	msgs, err := ch.Consume(
		streamName, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	// Process messages
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("message channel closed")
			}

			// Convert amqp.Delivery to rabbitmq.Delivery
			delivery := &rabbitmq.Delivery{
				Delivery:   msg,
				ReceivedAt: time.Now(),
			}

			// Call the handler
			if err := handler(ctx, delivery); err != nil {
				return fmt.Errorf("handler error: %w", err)
			}
		}
	}
}

// CreateStream creates a new stream with the specified configuration
func (h *Handler) CreateStream(ctx context.Context, streamName string, config rabbitmq.StreamConfig) error {
	if h.client == nil {
		return fmt.Errorf("client is nil")
	}

	// Create a channel for stream management
	ch, err := h.client.CreateChannel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	// Prepare stream arguments
	args := amqp.Table{
		"x-queue-type": "stream",
	}

	if config.MaxAge > 0 {
		args["x-max-age"] = config.MaxAge.String()
	}
	if config.MaxLengthMessages > 0 {
		args["x-max-length"] = config.MaxLengthMessages
	}
	if config.MaxLengthBytes > 0 {
		args["x-max-length-bytes"] = config.MaxLengthBytes
	}
	if config.MaxSegmentSizeBytes > 0 {
		args["x-stream-max-segment-size-bytes"] = config.MaxSegmentSizeBytes
	}
	if config.InitialClusterSize > 0 {
		args["x-initial-cluster-size"] = config.InitialClusterSize
	}

	// Declare the stream
	_, err = ch.QueueDeclare(
		streamName, // name
		true,       // durable
		false,      // delete when unused
		false,      // exclusive
		false,      // no-wait
		args,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare stream: %w", err)
	}

	return nil
}

// DeleteStream deletes the specified stream
func (h *Handler) DeleteStream(ctx context.Context, streamName string) error {
	if h.client == nil {
		return fmt.Errorf("client is nil")
	}

	// Create a channel for stream management
	ch, err := h.client.CreateChannel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	// Delete the stream
	_, err = ch.QueueDelete(streamName, false, false, false)
	if err != nil {
		return fmt.Errorf("failed to delete stream: %w", err)
	}

	return nil
}

// ensureStreamExists checks if a stream exists and creates it with default config if not
func (h *Handler) ensureStreamExists(ch *amqp.Channel, streamName string) error {
	// Try to passively declare the queue to check if it exists
	_, err := ch.QueueDeclarePassive(streamName, true, false, false, false, nil)
	if err != nil {
		// Stream doesn't exist, create it with default configuration
		args := amqp.Table{
			"x-queue-type": "stream",
		}
		_, err = ch.QueueDeclare(streamName, true, false, false, false, args)
		if err != nil {
			return fmt.Errorf("failed to create stream: %w", err)
		}
	}
	return nil
}

// Ensure Handler implements the interface at compile time
var _ rabbitmq.StreamHandler = (*Handler)(nil)
