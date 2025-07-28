package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudresty/emit"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Consumer handles message consumption operations
type Consumer struct {
	client *Client
	config *consumerConfig
	ch     *amqp.Channel

	// Internal state
	consuming bool
	stopCh    chan struct{}
	stopOnce  sync.Once
	wg        sync.WaitGroup
	mu        sync.RWMutex
}

// consumerConfig holds consumer-specific configuration
type consumerConfig struct {
	AutoAck        bool
	PrefetchCount  int
	PrefetchSize   int
	ConsumerTag    string
	Exclusive      bool
	NoLocal        bool
	NoWait         bool
	MessageTimeout time.Duration
	Concurrency    int
	RetryPolicy    RetryPolicy
	Compressor     MessageCompressor
	Encryptor      MessageEncryptor
	Serializer     MessageSerializer
}

// ConsumerOption represents a functional option for consumer configuration
type ConsumerOption func(*consumerConfig)

// ConsumeOption represents a functional option for consumption behavior
type ConsumeOption func(*consumeConfig)

// consumeConfig holds consumption-specific configuration
type consumeConfig struct {
	RejectRequeue    bool
	RetryPolicy      RetryPolicy
	DeadLetterPolicy DeadLetterPolicy
}

// MessageHandler is the function signature for handling consumed messages
type MessageHandler func(ctx context.Context, delivery *Delivery) error

// Delivery wraps amqp.Delivery with additional helper methods
type Delivery struct {
	amqp.Delivery

	// Additional metadata
	ReceivedAt time.Time
}

// DeadLetterPolicy defines what to do with messages after retries are exhausted
type DeadLetterPolicy interface {
	ShouldDeadLetter(delivery *Delivery, attempts int) bool
}

// DeadLetterAfterRetries sends messages to DLQ after max retry attempts
type DeadLetterAfterRetries struct{}

func (d *DeadLetterAfterRetries) ShouldDeadLetter(delivery *Delivery, attempts int) bool {
	return true
}

// Consumer options

// WithAutoAck enables automatic acknowledgment
func WithAutoAck() ConsumerOption {
	return func(config *consumerConfig) {
		config.AutoAck = true
	}
}

// WithPrefetchCount sets the prefetch count
func WithPrefetchCount(count int) ConsumerOption {
	return func(config *consumerConfig) {
		config.PrefetchCount = count
	}
}

// WithPrefetchSize sets the prefetch size in bytes
func WithPrefetchSize(size int) ConsumerOption {
	return func(config *consumerConfig) {
		config.PrefetchSize = size
	}
}

// WithConsumerTag sets the consumer tag
func WithConsumerTag(tag string) ConsumerOption {
	return func(config *consumerConfig) {
		config.ConsumerTag = tag
	}
}

// WithExclusiveConsumer makes the consumer exclusive
func WithExclusiveConsumer() ConsumerOption {
	return func(config *consumerConfig) {
		config.Exclusive = true
	}
}

// WithNoLocal prevents delivery of messages published on this connection
func WithNoLocal() ConsumerOption {
	return func(config *consumerConfig) {
		config.NoLocal = true
	}
}

// WithNoWait makes the consume operation not wait for server response
func WithNoWait() ConsumerOption {
	return func(config *consumerConfig) {
		config.NoWait = true
	}
}

// WithMessageTimeout sets the timeout for processing each message
func WithMessageTimeout(timeout time.Duration) ConsumerOption {
	return func(config *consumerConfig) {
		config.MessageTimeout = timeout
	}
}

// WithConcurrency sets the number of concurrent message processors
func WithConcurrency(workers int) ConsumerOption {
	return func(config *consumerConfig) {
		config.Concurrency = workers
	}
}

// WithConsumerRetryPolicy sets the retry policy for message processing failures
func WithConsumerRetryPolicy(policy RetryPolicy) ConsumerOption {
	return func(config *consumerConfig) {
		config.RetryPolicy = policy
	}
}

// WithConsumerCompression sets the compression for the consumer (for decompression)
func WithConsumerCompression(compressor MessageCompressor) ConsumerOption {
	return func(config *consumerConfig) {
		config.Compressor = compressor
	}
}

// WithConsumerEncryption sets the encryption for the consumer (for decryption)
func WithConsumerEncryption(encryptor MessageEncryptor) ConsumerOption {
	return func(config *consumerConfig) {
		config.Encryptor = encryptor
	}
}

// WithConsumerSerialization sets the serializer for the consumer (for deserialization)
func WithConsumerSerialization(serializer MessageSerializer) ConsumerOption {
	return func(config *consumerConfig) {
		config.Serializer = serializer
	}
}

// Consumption options

// WithRejectRequeue configures message rejection behavior
func WithRejectRequeue() ConsumeOption {
	return func(config *consumeConfig) {
		config.RejectRequeue = true
	}
}

// WithConsumeRetryPolicy sets the retry policy for consumption
func WithConsumeRetryPolicy(policy RetryPolicy) ConsumeOption {
	return func(config *consumeConfig) {
		config.RetryPolicy = policy
	}
}

// WithDeadLetterPolicy sets the dead letter policy
func WithDeadLetterPolicy(policy DeadLetterPolicy) ConsumeOption {
	return func(config *consumeConfig) {
		config.DeadLetterPolicy = policy
	}
}

// Consumer methods

// Consume starts consuming messages from the specified queue with optional settings
func (c *Consumer) Consume(ctx context.Context, queue string, handler MessageHandler, opts ...ConsumeOption) error {
	// Default consume configuration
	consumeConfig := &consumeConfig{
		RejectRequeue:    false,
		RetryPolicy:      c.config.RetryPolicy,
		DeadLetterPolicy: &DeadLetterAfterRetries{},
	}

	// Apply consume options
	for _, opt := range opts {
		opt(consumeConfig)
	}

	c.mu.Lock()
	if c.consuming {
		c.mu.Unlock()
		return fmt.Errorf("consumer is already consuming")
	}
	c.consuming = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.consuming = false
		c.mu.Unlock()
	}()

	// Start tracing span
	ctx, span := c.client.config.Tracer.StartSpan(ctx, "rabbitmq.consume")
	defer span.End()

	span.SetAttribute("queue", queue)
	span.SetAttribute("concurrency", c.config.Concurrency)

	// Consume with automatic reconnection
	return c.consumeWithReconnection(ctx, queue, handler, consumeConfig, span)
}

// consumeWithReconnection handles consuming with automatic reconnection
func (c *Consumer) consumeWithReconnection(ctx context.Context, queue string, handler MessageHandler, config *consumeConfig, span Span) error {
	for {
		select {
		case <-ctx.Done():
			span.SetStatus(SpanStatusOK, "context cancelled")
			return ctx.Err()
		default:
		}

		// Ensure we have a valid channel
		if err := c.ensureValidChannel(); err != nil {
			emit.Error.StructuredFields("Failed to ensure valid channel, retrying...",
				emit.ZString("queue", queue),
				emit.ZString("error", err.Error()))

			// Wait before retrying
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.client.config.ReconnectDelay):
				continue
			}
		}

		// Start consuming
		deliveries, err := c.ch.Consume(
			queue,
			c.config.ConsumerTag,
			c.config.AutoAck,
			c.config.Exclusive,
			c.config.NoLocal,
			c.config.NoWait,
			nil, // arguments
		)
		if err != nil {
			emit.Error.StructuredFields("Failed to start consuming, retrying...",
				emit.ZString("queue", queue),
				emit.ZString("error", err.Error()))

			// Wait before retrying
			select {
			case <-ctx.Done():
				span.SetStatus(SpanStatusError, err.Error())
				return ctx.Err()
			case <-time.After(c.client.config.ReconnectDelay):
				continue
			}
		}

		emit.Info.StructuredFields("Started consuming messages",
			emit.ZString("queue", queue),
			emit.ZInt("concurrency", c.config.Concurrency))

		// Reset stop channel for this consumption cycle
		c.stopCh = make(chan struct{})
		c.stopOnce = sync.Once{}

		// Create worker pool
		channelClosed := make(chan struct{})
		for i := 0; i < c.config.Concurrency; i++ {
			c.wg.Add(1)
			go c.workerWithReconnection(ctx, queue, handler, deliveries, config, channelClosed)
		}

		// Wait for context cancellation, stop signal, or channel closure
		select {
		case <-ctx.Done():
			emit.Info.StructuredFields("Consumption stopped due to context cancellation",
				emit.ZString("queue", queue))
			span.SetStatus(SpanStatusOK, "context cancelled")
			// Stop workers
			c.stopOnce.Do(func() {
				close(c.stopCh)
			})
			c.wg.Wait()
			return ctx.Err()
		case <-c.stopCh:
			emit.Info.StructuredFields("Consumption stopped due to stop signal",
				emit.ZString("queue", queue))
			span.SetStatus(SpanStatusOK, "stopped")
			c.wg.Wait()
			return nil
		case <-channelClosed:
			emit.Warn.StructuredFields("Consumer channel closed, reconnecting...",
				emit.ZString("queue", queue))
			// Stop current workers
			c.stopOnce.Do(func() {
				close(c.stopCh)
			})
			c.wg.Wait()
			// Continue the loop to reconnect
		}
	}
}

// ensureValidChannel ensures the consumer has a valid channel
func (c *Consumer) ensureValidChannel() error {
	// Check if current channel is still open
	if c.ch != nil && !c.ch.IsClosed() {
		return nil
	}

	emit.Info.StructuredFields("Recreating consumer channel",
		emit.ZString("connection_name", c.client.config.ConnectionName))

	// Get a new channel from the client
	newCh, err := c.client.getChannel()
	if err != nil {
		return fmt.Errorf("failed to get new channel: %w", err)
	}

	// Set QoS if needed
	if c.config.PrefetchCount > 0 || c.config.PrefetchSize > 0 {
		if err := newCh.Qos(c.config.PrefetchCount, c.config.PrefetchSize, false); err != nil {
			_ = newCh.Close() // Ignore close error in cleanup
			return fmt.Errorf("failed to set QoS: %w", err)
		}
	}

	c.ch = newCh
	emit.Info.StructuredFields("Consumer channel recreated successfully",
		emit.ZString("connection_name", c.client.config.ConnectionName))

	return nil
}

// workerWithReconnection processes messages from the delivery channel with reconnection signaling
func (c *Consumer) workerWithReconnection(ctx context.Context, queue string, handler MessageHandler, deliveries <-chan amqp.Delivery, config *consumeConfig, channelClosed chan<- struct{}) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case delivery, ok := <-deliveries:
			if !ok {
				emit.Warn.StructuredFields("Delivery channel closed",
					emit.ZString("queue", queue))
				// Signal that the channel is closed (only once)
				select {
				case channelClosed <- struct{}{}:
				default:
				}
				return
			}

			c.processMessage(ctx, queue, handler, &delivery, config)
		}
	}
}

// processMessage processes a single message with retry logic
func (c *Consumer) processMessage(ctx context.Context, queue string, handler MessageHandler, delivery *amqp.Delivery, config *consumeConfig) {
	// Record that we received a message
	c.client.config.Metrics.RecordMessageReceived(queue)

	// Create our enhanced delivery wrapper
	enhancedDelivery := &Delivery{
		Delivery:   *delivery,
		ReceivedAt: time.Now(),
	}

	// Apply decryption if configured
	if c.config.Encryptor != nil {
		if err := c.applyDecryption(enhancedDelivery); err != nil {
			emit.Error.StructuredFields("Failed to decrypt message",
				emit.ZString("queue", queue),
				emit.ZString("message_id", delivery.MessageId),
				emit.ZString("error", err.Error()))
			// Handle as processing error
			c.handleProcessingError(queue, delivery, fmt.Errorf("decryption failed: %w", err), config)
			return
		}
	}

	// Apply decompression if configured
	if c.config.Compressor != nil {
		if err := c.applyDecompression(enhancedDelivery); err != nil {
			emit.Error.StructuredFields("Failed to decompress message",
				emit.ZString("queue", queue),
				emit.ZString("message_id", delivery.MessageId),
				emit.ZString("error", err.Error()))
			// Handle as processing error
			c.handleProcessingError(queue, delivery, fmt.Errorf("decompression failed: %w", err), config)
			return
		}
	}

	// Add timeout to context if configured
	msgCtx := ctx
	if c.config.MessageTimeout > 0 {
		var cancel context.CancelFunc
		msgCtx, cancel = context.WithTimeout(ctx, c.config.MessageTimeout)
		defer cancel()
	}

	// Start tracing span for message processing
	msgCtx, span := c.client.config.Tracer.StartSpan(msgCtx, "rabbitmq.process_message")
	defer span.End()

	span.SetAttribute("queue", queue)
	span.SetAttribute("message_id", delivery.MessageId)
	span.SetAttribute("correlation_id", delivery.CorrelationId)
	span.SetAttribute("routing_key", delivery.RoutingKey)

	start := time.Now()
	var handlerErr error

	// Process message with panic recovery
	func() {
		defer func() {
			if r := recover(); r != nil {
				handlerErr = fmt.Errorf("handler panicked: %v", r)
				emit.Error.StructuredFields("Message handler panicked",
					emit.ZString("queue", queue),
					emit.ZString("message_id", delivery.MessageId),
					emit.ZString("panic", fmt.Sprintf("%v", r)))
			}
		}()

		handlerErr = handler(msgCtx, enhancedDelivery)
	}()

	duration := time.Since(start)
	success := handlerErr == nil

	// Record processing metrics
	c.client.config.Metrics.RecordMessageProcessed(queue, success, duration)
	if c.client.config.PerformanceMonitor != nil {
		c.client.config.PerformanceMonitor.RecordConsume(success, duration)
	}

	if success {
		// Message processed successfully
		span.SetStatus(SpanStatusOK, "")

		if !c.config.AutoAck {
			if err := delivery.Ack(false); err != nil {
				emit.Error.StructuredFields("Failed to acknowledge message",
					emit.ZString("queue", queue),
					emit.ZString("message_id", delivery.MessageId),
					emit.ZString("error", err.Error()))
			}
		}

		emit.Debug.StructuredFields("Message processed successfully",
			emit.ZString("queue", queue),
			emit.ZString("message_id", delivery.MessageId),
			emit.ZDuration("duration", duration))
	} else {
		// Message processing failed
		span.SetStatus(SpanStatusError, handlerErr.Error())
		c.handleProcessingError(queue, delivery, handlerErr, config)
	}
}

// handleProcessingError handles message processing errors with retry logic
func (c *Consumer) handleProcessingError(queue string, delivery *amqp.Delivery, err error, config *consumeConfig) {
	emit.Warn.StructuredFields("Message processing failed",
		emit.ZString("queue", queue),
		emit.ZString("message_id", delivery.MessageId),
		emit.ZString("error", err.Error()))

	if c.config.AutoAck {
		// Can't retry if auto-ack is enabled
		return
	}

	// Check if this is a reject error
	if rejectErr, ok := err.(*RejectError); ok {
		if err := delivery.Nack(false, rejectErr.Requeue); err != nil {
			emit.Error.StructuredFields("Failed to nack message",
				emit.ZString("queue", queue),
				emit.ZString("message_id", delivery.MessageId),
				emit.ZString("error", err.Error()))
		}

		if rejectErr.Requeue {
			c.client.config.Metrics.RecordMessageRequeued(queue)
		}
		return
	}

	// Default behavior: requeue based on configuration
	requeue := config.RejectRequeue

	if err := delivery.Nack(false, requeue); err != nil {
		emit.Error.StructuredFields("Failed to nack message",
			emit.ZString("queue", queue),
			emit.ZString("message_id", delivery.MessageId),
			emit.ZString("error", err.Error()))
	}

	if requeue {
		c.client.config.Metrics.RecordMessageRequeued(queue)
	}
}

// Close stops the consumer and closes its channel
func (c *Consumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.consuming {
		c.stopOnce.Do(func() {
			close(c.stopCh)
		})
		c.wg.Wait()
		c.consuming = false
	}

	if c.ch != nil && !c.ch.IsClosed() {
		if err := c.ch.Close(); err != nil {
			emit.Error.StructuredFields("Failed to close consumer channel",
				emit.ZString("error", err.Error()))
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}

	emit.Info.StructuredFields("Consumer closed successfully")
	return nil
}

// Delivery helper methods

// MessageID returns the message ID
func (d *Delivery) MessageID() string {
	return d.MessageId
}

// Timestamp returns the message timestamp
func (d *Delivery) Timestamp() time.Time {
	return d.Delivery.Timestamp
}

// IsRedelivered returns true if the message was redelivered
func (d *Delivery) IsRedelivered() bool {
	return d.Redelivered
}

// Ack acknowledges the message
func (d *Delivery) Ack() error {
	return d.Delivery.Ack(false)
}

// Nack negatively acknowledges the message with requeue option
func (d *Delivery) Nack(requeue bool) error {
	return d.Delivery.Nack(false, requeue)
}

// Reject rejects the message with requeue option
func (d *Delivery) Reject(requeue bool) error {
	return d.Delivery.Reject(requeue)
}

// GetMessage converts the delivery to a Message struct
func (d *Delivery) GetMessage() *Message {
	return &Message{
		Body:          d.Body,
		ContentType:   d.ContentType,
		Headers:       map[string]any(d.Headers),
		Persistent:    d.DeliveryMode == 2,
		MessageID:     d.MessageId,
		CorrelationID: d.CorrelationId,
		ReplyTo:       d.ReplyTo,
		Type:          d.Type,
		AppID:         d.AppId,
		UserID:        d.UserId,
		Timestamp:     d.Delivery.Timestamp.Unix(),
		Expiration:    d.Expiration,
		Priority:      d.Priority,
	}
}

// Helper methods for message processing

// applyDecryption applies decryption to the delivery if an encryption header is present
func (c *Consumer) applyDecryption(delivery *Delivery) error {
	// Check if the message has encryption header
	if delivery.Headers == nil {
		return nil
	}

	encryptionAlg, ok := delivery.Headers["x-encryption"]
	if !ok {
		return nil
	}

	// Verify the encryption algorithm matches
	if encryptionAlg != c.config.Encryptor.Algorithm() {
		return fmt.Errorf("encryption algorithm mismatch: expected %s, got %s",
			c.config.Encryptor.Algorithm(), encryptionAlg)
	}

	// Decrypt the message body
	decrypted, err := c.config.Encryptor.Decrypt(delivery.Body)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}

	// Update the delivery body
	delivery.Body = decrypted

	// Remove the encryption header
	delete(delivery.Headers, "x-encryption")

	return nil
}

// applyDecompression applies decompression to the delivery if a compression header is present
func (c *Consumer) applyDecompression(delivery *Delivery) error {
	// Check if the message has compression header
	if delivery.Headers == nil {
		return nil
	}

	compressionAlg, ok := delivery.Headers["x-compression"]
	if !ok {
		return nil
	}

	// Verify the compression algorithm matches
	if compressionAlg != c.config.Compressor.Algorithm() {
		return fmt.Errorf("compression algorithm mismatch: expected %s, got %s",
			c.config.Compressor.Algorithm(), compressionAlg)
	}

	// Decompress the message body
	decompressed, err := c.config.Compressor.Decompress(delivery.Body)
	if err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}

	// Update the delivery body
	delivery.Body = decompressed

	// Remove the compression header
	delete(delivery.Headers, "x-compression")

	return nil
}
