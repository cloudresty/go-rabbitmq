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
	// For streams, we also store the message ID in headers as a backup
	// to ensure reliable message identification across different stream configurations
	headers := message.Headers
	if headers == nil {
		headers = make(map[string]any)
	}
	headers["x-message-id"] = message.MessageID

	publishing := amqp.Publishing{
		ContentType:   message.ContentType,
		Body:          message.Body,
		Headers:       headers,
		MessageId:     message.MessageID,
		CorrelationId: message.CorrelationID,
		Timestamp:     time.Now(),
	}

	err = ch.PublishWithContext(ctx, "", streamName, false, false, publishing)
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

	// Set prefetch count for stream consumers (required for streams)
	err = ch.Qos(10, 0, false)
	if err != nil {
		return fmt.Errorf("failed to set prefetch count: %w", err)
	}

	// Create a consumer with stream-specific arguments
	args := amqp.Table{
		"x-stream-offset": "first", // Start reading from the beginning of the stream
	}
	msgs, err := ch.Consume(
		streamName, // queue
		"",         // consumer
		false,      // auto-ack (must be false for streams)
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		args,       // args - include stream offset
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

			// For streams, provide fallback message ID recovery from headers
			// This ensures reliable message identification even if the primary MessageId field fails
			if delivery.MessageId == "" && msg.Headers != nil {
				if headerMessageId, ok := msg.Headers["x-message-id"]; ok {
					if idStr, ok := headerMessageId.(string); ok {
						delivery.MessageId = idStr
					}
				}
			}

			// Call the handler
			if err := handler(ctx, delivery); err != nil {
				// On error, nack the message
				if nackErr := msg.Nack(false, false); nackErr != nil {
					return fmt.Errorf("handler error: %w, nack error: %w", err, nackErr)
				}
				return fmt.Errorf("handler error: %w", err)
			}

			// Acknowledge the message after successful processing
			if ackErr := msg.Ack(false); ackErr != nil {
				return fmt.Errorf("failed to acknowledge message: %w", ackErr)
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
		// RabbitMQ expects max-age as a simple string (e.g., "24h" for 24 hours, "7D" for 7 days)
		// Convert duration to the most appropriate unit
		if config.MaxAge >= 24*time.Hour && config.MaxAge%(24*time.Hour) == 0 {
			// Use days if it's a multiple of 24 hours
			days := int(config.MaxAge.Hours() / 24)
			args["x-max-age"] = fmt.Sprintf("%dD", days)
		} else if config.MaxAge >= time.Hour && config.MaxAge%time.Hour == 0 {
			// Use hours if it's a multiple of hours
			hours := int(config.MaxAge.Hours())
			args["x-max-age"] = fmt.Sprintf("%dh", hours)
		} else if config.MaxAge >= time.Minute && config.MaxAge%time.Minute == 0 {
			// Use minutes if it's a multiple of minutes
			minutes := int(config.MaxAge.Minutes())
			args["x-max-age"] = fmt.Sprintf("%dm", minutes)
		} else {
			// Use seconds for anything else
			seconds := int(config.MaxAge.Seconds())
			args["x-max-age"] = fmt.Sprintf("%ds", seconds)
		}
	}
	// Note: RabbitMQ streams don't support x-max-length (MaxLengthMessages)
	// They only support x-max-length-bytes for size-based retention
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
