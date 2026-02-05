// Package streams provides RabbitMQ streams functionality for high-throughput scenarios.
//
// RabbitMQ streams are a data structure introduced in RabbitMQ 3.9+ that provides
// persistent, replicated, and high-throughput messaging. Streams are ideal for:
//   - Event sourcing applications
//   - Time-series data
//   - High-throughput messaging scenarios (100k+ messages/second)
//   - Cases where message order and durability are critical
//
// This package uses the native RabbitMQ stream protocol (port 5552) which provides
// 5-10x better performance compared to AMQP 0.9.1.
//
// Example usage:
//
//	// Create a stream handler
//	handler, err := streams.NewHandler(streams.Options{
//	    Host:     "localhost",
//	    Port:     5552,
//	    Username: "guest",
//	    Password: "guest",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer handler.Close()
//
//	// Create a stream
//	err = handler.CreateStream(ctx, "my-stream", rabbitmq.StreamConfig{
//	    MaxAge: 24 * time.Hour,
//	})
//
//	// Publish to stream (high-throughput)
//	err = handler.PublishToStream(ctx, "my-stream", rabbitmq.NewMessage([]byte("hello")))
//
//	// Consume from stream
//	err = handler.ConsumeFromStream(ctx, "my-stream", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
//	    fmt.Printf("Received: %s\\n", delivery.Body)
//	    return nil
//	})
package streams

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudresty/go-rabbitmq"
	"github.com/cloudresty/go-rabbitmq/streams/native"
)

// Options configures the stream handler connection.
type Options struct {
	Host               string
	Port               int
	Username           string
	Password           string
	VHost              string
	MaxProducers       int
	MaxConsumers       int
	RequestedHeartbeat time.Duration
}

// Handler implements rabbitmq.StreamHandler using the native stream protocol.
// This provides high-throughput messaging with sub-millisecond latency.
type Handler struct {
	env       *native.Environment
	producers map[string]*native.Producer
	consumers map[string]*native.Consumer
	mu        sync.RWMutex
	closed    bool
}

// NewHandler creates a new stream handler using the RabbitMQ native stream protocol.
//
// The native protocol (port 5552) provides:
//   - Sub-millisecond publish latency
//   - 100k+ messages/second throughput
//   - Efficient binary protocol
//   - Built-in offset tracking
func NewHandler(opts Options) (*Handler, error) {
	env, err := native.NewEnvironment(native.EnvironmentOptions{
		Host:               opts.Host,
		Port:               opts.Port,
		Username:           opts.Username,
		Password:           opts.Password,
		VHost:              opts.VHost,
		MaxProducers:       opts.MaxProducers,
		MaxConsumers:       opts.MaxConsumers,
		RequestedHeartbeat: opts.RequestedHeartbeat,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stream environment: %w", err)
	}

	return &Handler{
		env:       env,
		producers: make(map[string]*native.Producer),
		consumers: make(map[string]*native.Consumer),
	}, nil
}

// Close closes the handler and all associated producers/consumers.
func (h *Handler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}
	h.closed = true

	// Close all producers
	for _, p := range h.producers {
		_ = p.Close()
	}

	// Close all consumers
	for _, c := range h.consumers {
		_ = c.Close()
	}

	return h.env.Close()
}

// getOrCreateProducer gets an existing producer or creates a new one for the stream.
func (h *Handler) getOrCreateProducer(streamName string) (*native.Producer, error) {
	h.mu.RLock()
	if p, exists := h.producers[streamName]; exists {
		h.mu.RUnlock()
		return p, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Double-check after acquiring write lock
	if p, exists := h.producers[streamName]; exists {
		return p, nil
	}

	p, err := h.env.NewProducer(streamName, native.ProducerOptions{})
	if err != nil {
		return nil, err
	}

	h.producers[streamName] = p
	return p, nil
}

// PublishToStream publishes a message to the specified stream.
func (h *Handler) PublishToStream(ctx context.Context, streamName string, message *rabbitmq.Message) error {
	h.mu.RLock()
	if h.closed {
		h.mu.RUnlock()
		return fmt.Errorf("handler is closed")
	}
	h.mu.RUnlock()

	producer, err := h.getOrCreateProducer(streamName)
	if err != nil {
		return fmt.Errorf("failed to get producer for stream %s: %w", streamName, err)
	}

	// Convert rabbitmq.Message to native.Message
	nativeMsg := native.Message{
		Body: message.Body,
		Properties: native.MessageProperties{
			MessageID:     message.MessageID,
			CorrelationID: message.CorrelationID,
			ContentType:   message.ContentType,
		},
	}

	return producer.SendMessage(nativeMsg)
}

// ConsumeFromStream consumes messages from the specified stream.
func (h *Handler) ConsumeFromStream(ctx context.Context, streamName string, handler rabbitmq.StreamMessageHandler) error {
	h.mu.RLock()
	if h.closed {
		h.mu.RUnlock()
		return fmt.Errorf("handler is closed")
	}
	h.mu.RUnlock()

	// Create a consumer with a wrapper that converts native messages to rabbitmq.Delivery
	consumer, err := h.env.NewConsumer(streamName, func(msg native.Message) {
		// Convert native.Message to rabbitmq.Delivery
		delivery := &rabbitmq.Delivery{
			ReceivedAt: msg.Timestamp,
		}
		// Set the body directly on the underlying amqp.Delivery
		delivery.Body = msg.Body
		delivery.MessageId = msg.Properties.MessageID
		delivery.CorrelationId = msg.Properties.CorrelationID
		delivery.ContentType = msg.Properties.ContentType

		// Call the handler - errors don't stop consumption
		// The native protocol handles offset tracking automatically
		_ = handler(ctx, delivery)
	}, native.ConsumerOptions{
		OffsetType: native.OffsetFirst,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer for stream %s: %w", streamName, err)
	}

	// Store the consumer
	h.mu.Lock()
	h.consumers[streamName] = consumer
	h.mu.Unlock()

	// Wait for context cancellation
	<-ctx.Done()

	// Clean up consumer
	h.mu.Lock()
	delete(h.consumers, streamName)
	h.mu.Unlock()

	return consumer.Close()
}

// CreateStream creates a new stream with the specified configuration.
func (h *Handler) CreateStream(ctx context.Context, streamName string, config rabbitmq.StreamConfig) error {
	h.mu.RLock()
	if h.closed {
		h.mu.RUnlock()
		return fmt.Errorf("handler is closed")
	}
	h.mu.RUnlock()

	opts := native.StreamOptions{
		MaxLengthBytes:      int64(config.MaxLengthBytes),
		MaxSegmentSizeBytes: int64(config.MaxSegmentSizeBytes),
	}

	// Convert MaxAge to appropriate format
	if config.MaxAge > 0 {
		opts.MaxAge = config.MaxAge
	}

	return h.env.CreateStream(streamName, opts)
}

// DeleteStream deletes the specified stream.
func (h *Handler) DeleteStream(ctx context.Context, streamName string) error {
	h.mu.RLock()
	if h.closed {
		h.mu.RUnlock()
		return fmt.Errorf("handler is closed")
	}
	h.mu.RUnlock()

	// Close any existing producer/consumer for this stream
	h.mu.Lock()
	if p, exists := h.producers[streamName]; exists {
		_ = p.Close()
		delete(h.producers, streamName)
	}
	if c, exists := h.consumers[streamName]; exists {
		_ = c.Close()
		delete(h.consumers, streamName)
	}
	h.mu.Unlock()

	return h.env.DeleteStream(streamName)
}

// Ensure Handler implements the interface at compile time
var _ rabbitmq.StreamHandler = (*Handler)(nil)
