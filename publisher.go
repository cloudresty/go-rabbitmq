package rabbitmq

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudresty/emit"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher handles message publishing operations
type Publisher struct {
	client *Client
	config *publisherConfig
	ch     *amqp.Channel
}

// publisherConfig holds publisher-specific configuration
type publisherConfig struct {
	DefaultExchange      string
	Mandatory            bool
	Immediate            bool
	Persistent           bool
	ConfirmationEnabled  bool
	ConfirmationTimeout  time.Duration
	RetryPolicy          RetryPolicy
	Compressor           MessageCompressor
	CompressionThreshold int
	Encryptor            MessageEncryptor
	Serializer           MessageSerializer

	// Internal confirmation handling
	confirmations chan amqp.Confirmation
}

// PublisherOption represents a functional option for publisher configuration
type PublisherOption func(*publisherConfig)

// PublishRequest represents a single publish operation
type PublishRequest struct {
	Exchange   string
	RoutingKey string
	Message    *Message
}

// Publisher options

// WithDefaultExchange sets the default exchange for publishing
func WithDefaultExchange(exchange string) PublisherOption {
	return func(config *publisherConfig) {
		config.DefaultExchange = exchange
	}
}

// WithMandatory enables mandatory publishing
func WithMandatory() PublisherOption {
	return func(config *publisherConfig) {
		config.Mandatory = true
	}
}

// WithImmediate enables immediate publishing
func WithImmediate() PublisherOption {
	return func(config *publisherConfig) {
		config.Immediate = true
	}
}

// WithPersistent makes all messages persistent by default
func WithPersistent() PublisherOption {
	return func(config *publisherConfig) {
		config.Persistent = true
	}
}

// WithConfirmation enables publish confirmations for the publisher.
// The provided timeout is used to wait for each acknowledgement.
// This makes the publisher reliable but slower as each Publish call will block
// until the broker confirms the message or the timeout expires.
func WithConfirmation(timeout time.Duration) PublisherOption {
	return func(config *publisherConfig) {
		config.ConfirmationEnabled = true
		config.ConfirmationTimeout = timeout
	}
}

// WithRetryPolicy sets the retry policy for failed publishes
func WithRetryPolicy(policy RetryPolicy) PublisherOption {
	return func(config *publisherConfig) {
		config.RetryPolicy = policy
	}
}

// WithCompression sets the message compressor for the publisher
func WithCompression(compressor MessageCompressor) PublisherOption {
	return func(config *publisherConfig) {
		config.Compressor = compressor
	}
}

// WithCompressionThreshold sets the compression threshold for the publisher
func WithCompressionThreshold(threshold int) PublisherOption {
	return func(config *publisherConfig) {
		config.CompressionThreshold = threshold
	}
}

// WithSerializer sets the message serializer for the publisher
func WithSerializer(serializer MessageSerializer) PublisherOption {
	return func(config *publisherConfig) {
		config.Serializer = serializer
	}
}

// WithEncryption sets the message encryptor for the publisher
func WithEncryption(encryptor MessageEncryptor) PublisherOption {
	return func(config *publisherConfig) {
		config.Encryptor = encryptor
	}
}

// Core publishing methods

// Publish publishes a message to the specified exchange and routing key
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, message *Message) error {
	// Use default exchange if not specified
	if exchange == "" {
		exchange = p.config.DefaultExchange
	}

	// Apply publisher defaults to message if not set
	if p.config.Persistent && !message.Persistent {
		message = message.Clone()
		message.Persistent = p.config.Persistent
	}

	// Apply compression if configured
	if p.config.Compressor != nil {
		var err error
		message, err = p.applyCompression(message)
		if err != nil {
			return fmt.Errorf("failed to compress message: %w", err)
		}
	}

	// Apply encryption if configured
	if p.config.Encryptor != nil {
		var err error
		message, err = p.applyEncryption(message)
		if err != nil {
			return fmt.Errorf("failed to encrypt message: %w", err)
		}
	}

	// Convert to AMQP publishing
	publishing := message.ToAMQPPublishing()

	// Record metrics
	start := time.Now()

	// Start tracing span
	ctx, span := p.client.config.Tracer.StartSpan(ctx, "rabbitmq.publish")
	defer span.End()

	span.SetAttribute("exchange", exchange)
	span.SetAttribute("routing_key", routingKey)
	span.SetAttribute("message_id", message.MessageID)
	span.SetAttribute("confirmation_enabled", p.config.ConfirmationEnabled)

	// Publish message
	err := p.ch.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		p.config.Mandatory,
		p.config.Immediate,
		publishing,
	)

	// Record performance metrics
	duration := time.Since(start)
	success := err == nil
	p.client.config.Metrics.RecordPublish(exchange, routingKey, len(message.Body), duration)
	if p.client.config.PerformanceMonitor != nil {
		p.client.config.PerformanceMonitor.RecordPublish(success, duration)
	}

	if err != nil {
		p.client.config.Metrics.RecordError("publish", err)
		span.SetStatus(SpanStatusError, err.Error())
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Wait for confirmation if publisher is configured for confirmations
	if p.config.ConfirmationEnabled {
		if err := p.waitForConfirmation(ctx); err != nil {
			span.SetStatus(SpanStatusError, err.Error())
			return fmt.Errorf("confirmation failed: %w", err)
		}
	}

	emit.Debug.StructuredFields("Message published successfully",
		emit.ZString("exchange", exchange),
		emit.ZString("routing_key", routingKey),
		emit.ZString("message_id", message.MessageID),
		emit.ZString("correlation_id", message.CorrelationID),
		emit.ZBool("confirmed", p.config.ConfirmationEnabled))

	span.SetStatus(SpanStatusOK, "")
	return nil
}

// Batch publishing methods

// PublishBatch publishes multiple messages in a batch
func (p *Publisher) PublishBatch(ctx context.Context, messages []PublishRequest) error {
	if len(messages) == 0 {
		return nil
	}

	// Start tracing span
	ctx, span := p.client.config.Tracer.StartSpan(ctx, "rabbitmq.publish_batch")
	defer span.End()

	span.SetAttribute("batch_size", len(messages))

	// Publish each message
	for i, req := range messages {
		if err := p.Publish(ctx, req.Exchange, req.RoutingKey, req.Message); err != nil {
			span.SetStatus(SpanStatusError, fmt.Sprintf("failed at message %d: %v", i, err))
			return fmt.Errorf("failed to publish message %d: %w", i, err)
		}
	}

	span.SetStatus(SpanStatusOK, "")
	emit.Info.StructuredFields("Batch published successfully",
		emit.ZInt("batch_size", len(messages)))

	return nil
}

// Publisher management

// Close closes the publisher and its channel
func (p *Publisher) Close() error {
	if p.ch != nil && !p.ch.IsClosed() {
		if err := p.ch.Close(); err != nil {
			emit.Error.StructuredFields("Failed to close publisher channel",
				emit.ZString("error", err.Error()))
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}

	emit.Info.StructuredFields("Publisher closed successfully")
	return nil
}

// Helper methods for message processing

// applyCompression applies compression to the message if the message size exceeds the threshold
func (p *Publisher) applyCompression(message *Message) (*Message, error) {
	if len(message.Body) < p.config.CompressionThreshold {
		return message, nil
	}

	compressed, err := p.config.Compressor.Compress(message.Body)
	if err != nil {
		return nil, fmt.Errorf("compression failed: %w", err)
	}

	// Clone message and update body
	newMessage := message.Clone()
	newMessage.Body = compressed

	// Add compression header
	if newMessage.Headers == nil {
		newMessage.Headers = make(map[string]any)
	}
	newMessage.Headers["x-compression"] = p.config.Compressor.Algorithm()

	return newMessage, nil
}

// applyEncryption applies encryption to the message
func (p *Publisher) applyEncryption(message *Message) (*Message, error) {
	encrypted, err := p.config.Encryptor.Encrypt(message.Body)
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	// Clone message and update body
	newMessage := message.Clone()
	newMessage.Body = encrypted

	// Add encryption header
	if newMessage.Headers == nil {
		newMessage.Headers = make(map[string]any)
	}
	newMessage.Headers["x-encryption"] = p.config.Encryptor.Algorithm()

	return newMessage, nil
}

// waitForConfirmation waits for a single publish confirmation
func (p *Publisher) waitForConfirmation(ctx context.Context) error {
	if !p.config.ConfirmationEnabled || p.config.confirmations == nil {
		return nil // No confirmation required
	}

	// Wait for confirmation with timeout
	select {
	case confirmation := <-p.config.confirmations:
		start := time.Now()
		success := confirmation.Ack
		p.client.config.Metrics.RecordPublishConfirmation(success, time.Since(start))

		if !success {
			return fmt.Errorf("message not acknowledged by broker")
		}

		return nil

	case <-time.After(p.config.ConfirmationTimeout):
		p.client.config.Metrics.RecordPublishConfirmation(false, p.config.ConfirmationTimeout)
		return fmt.Errorf("confirmation timeout after %v", p.config.ConfirmationTimeout)

	case <-ctx.Done():
		return ctx.Err()
	}
}
