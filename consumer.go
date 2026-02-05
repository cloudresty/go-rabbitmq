package rabbitmq

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/cloudresty/ulid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// minReconnectDelay is the minimum delay between reconnection attempts
// to prevent busy loops if ReconnectDelay is set to 0 or very low values
const minReconnectDelay = 100 * time.Millisecond

// GenerateConsumerTag creates a unique consumer tag using Hostname + ULID.
// The format is: <sanitized-hostname>-<ulid>
//
// This provides:
//   - Traceability: Hostname identifies which Pod/VM/machine the consumer is running on
//   - Uniqueness: ULID ensures global uniqueness
//   - Sortability: ULIDs are lexicographically sortable, so newer consumers sort after older ones
//
// Example: "web-server-01-01ARZ3NDEKTSV4RRFFQ69G5FAV"
func GenerateConsumerTag() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}
	// Sanitize hostname for RabbitMQ compatibility
	hostname = sanitizeHostname(hostname)

	// Generate ULID for uniqueness and sortability
	ulidStr, err := ulid.New()
	if err != nil {
		// Fallback to timestamp if ULID generation fails
		return fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())
	}

	return fmt.Sprintf("%s-%s", hostname, ulidStr)
}

// sanitizeHostname removes or replaces characters that might be invalid in AMQP consumer tags.
// RabbitMQ is generally permissive, but we ensure only alphanumeric, dash, underscore, and dot.
func sanitizeHostname(hostname string) string {
	result := make([]byte, 0, len(hostname))
	for _, r := range hostname {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			result = append(result, byte(r))
		} else {
			result = append(result, '-')
		}
	}
	// Truncate to reasonable length (keep first 50 chars to leave room for ULID)
	if len(result) > 50 {
		result = result[:50]
	}
	return string(result)
}

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

	// Retry configuration
	MaxRetries   int
	RetryBackoff time.Duration

	// Channel-per-worker mode: each worker gets its own channel
	// This provides better isolation - if one channel fails, only that worker is affected
	ChannelPerWorker bool
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

// BatchMessageHandler is the function signature for handling batches of messages
// It receives a slice of deliveries and should return a slice of errors (one per message)
// A nil error at index i means message i was processed successfully
type BatchMessageHandler func(ctx context.Context, deliveries []*Delivery) []error

// Delivery wraps amqp.Delivery with additional helper methods
type Delivery struct {
	amqp.Delivery

	// Additional metadata
	ReceivedAt time.Time
}

// BatchConsumeConfig holds configuration for batch consumption
type BatchConsumeConfig struct {
	BatchSize    int           // Maximum number of messages per batch
	BatchTimeout time.Duration // Maximum time to wait for a full batch
	AutoAck      bool          // Whether to auto-ack messages after successful batch processing
}

// DefaultBatchConsumeConfig returns sensible defaults for batch consumption
func DefaultBatchConsumeConfig() BatchConsumeConfig {
	return BatchConsumeConfig{
		BatchSize:    100,
		BatchTimeout: 5 * time.Second,
		AutoAck:      false,
	}
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

// WithChannelPerWorker enables channel-per-worker mode where each worker gets its own
// dedicated AMQP channel. This provides better isolation and fault tolerance:
//
//   - If one channel fails, only that worker is affected (not all workers)
//   - Each worker can independently reconnect its channel
//   - Better parallelism as channels are not shared
//
// Trade-offs:
//   - Uses more resources (one channel per worker instead of one shared channel)
//   - Slightly higher connection overhead
//
// Recommended for:
//   - High-throughput scenarios with many workers
//   - When fault isolation is critical
//   - Long-running consumers where channel stability matters
func WithChannelPerWorker() ConsumerOption {
	return func(config *consumerConfig) {
		config.ChannelPerWorker = true
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

// WithConsumerRetry enables retry logic with backoff
// maxAttempts: number of retry attempts before sending to DLX (requeue=false)
// backoff: duration to wait before requeuing (requeue=true)
func WithConsumerRetry(maxAttempts int, backoff time.Duration) ConsumerOption {
	return func(config *consumerConfig) {
		config.MaxRetries = maxAttempts
		config.RetryBackoff = backoff
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
	// Validate topology if enabled
	if c.client.TopologyValidator() != nil {
		if err := c.client.TopologyValidator().ValidateQueue(queue); err != nil {
			return fmt.Errorf("topology validation failed for queue '%s': %w", queue, err)
		}
	}

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
	// Use channel-per-worker mode if enabled
	if c.config.ChannelPerWorker {
		return c.consumeWithChannelPerWorker(ctx, queue, handler, config, span)
	}

	// Default: shared channel mode
	return c.consumeWithSharedChannel(ctx, queue, handler, config, span)
}

// consumeWithSharedChannel implements the traditional shared-channel consumption mode
func (c *Consumer) consumeWithSharedChannel(ctx context.Context, queue string, handler MessageHandler, config *consumeConfig, span Span) error {
	for {
		select {
		case <-ctx.Done():
			span.SetStatus(SpanStatusOK, "context cancelled")
			return ctx.Err()
		default:
		}

		// Ensure we have a valid channel
		if err := c.ensureValidChannel(); err != nil {
			c.client.config.Logger.Error("Failed to ensure valid channel, retrying...",
				"queue", queue,
				"error", err.Error())

			// Wait before retrying (enforce minimum to prevent busy loops)
			reconnectDelay := max(c.client.config.ReconnectDelay, minReconnectDelay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(reconnectDelay):
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
			c.client.config.Logger.Error("Failed to start consuming, retrying...",
				"queue", queue,
				"error", err.Error())

			// Wait before retrying (enforce minimum to prevent busy loops)
			reconnectDelay := max(c.client.config.ReconnectDelay, minReconnectDelay)
			select {
			case <-ctx.Done():
				span.SetStatus(SpanStatusError, err.Error())
				return ctx.Err()
			case <-time.After(reconnectDelay):
				continue
			}
		}

		c.client.config.Logger.Info("Started consuming messages",
			"queue", queue,
			"concurrency", c.config.Concurrency)

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
			c.client.config.Logger.Info("Consumption stopped due to context cancellation",
				"queue", queue)
			span.SetStatus(SpanStatusOK, "context cancelled")
			// Stop workers
			c.stopOnce.Do(func() {
				close(c.stopCh)
			})
			c.wg.Wait()
			return ctx.Err()
		case <-c.stopCh:
			c.client.config.Logger.Info("Consumption stopped due to stop signal",
				"queue", queue)
			span.SetStatus(SpanStatusOK, "stopped")
			c.wg.Wait()
			return nil
		case <-channelClosed:
			c.client.config.Logger.Warn("Consumer channel closed, reconnecting...",
				"queue", queue)
			// Stop current workers
			c.stopOnce.Do(func() {
				close(c.stopCh)
			})
			c.wg.Wait()
			// Continue the loop to reconnect
		}
	}
}

// consumeWithChannelPerWorker implements channel-per-worker mode where each worker
// gets its own dedicated AMQP channel for better isolation and fault tolerance.
func (c *Consumer) consumeWithChannelPerWorker(ctx context.Context, queue string, handler MessageHandler, config *consumeConfig, span Span) error {
	c.client.config.Logger.Info("Starting consumption with channel-per-worker mode",
		"queue", queue,
		"concurrency", c.config.Concurrency)

	// Reset stop channel
	c.stopCh = make(chan struct{})
	c.stopOnce = sync.Once{}

	// Start workers, each with its own channel
	for i := 0; i < c.config.Concurrency; i++ {
		c.wg.Add(1)
		workerID := i
		go c.workerWithOwnChannel(ctx, queue, handler, config, workerID)
	}

	// Wait for context cancellation or stop signal
	select {
	case <-ctx.Done():
		c.client.config.Logger.Info("Consumption stopped due to context cancellation",
			"queue", queue)
		span.SetStatus(SpanStatusOK, "context cancelled")
		c.stopOnce.Do(func() {
			close(c.stopCh)
		})
		c.wg.Wait()
		return ctx.Err()
	case <-c.stopCh:
		c.client.config.Logger.Info("Consumption stopped due to stop signal",
			"queue", queue)
		span.SetStatus(SpanStatusOK, "stopped")
		c.wg.Wait()
		return nil
	}
}

// ensureValidChannel ensures the consumer has a valid channel
func (c *Consumer) ensureValidChannel() error {
	// Check if current channel is still open
	if c.ch != nil && !c.ch.IsClosed() {
		return nil
	}

	c.client.config.Logger.Info("Recreating consumer channel",
		"connection_name", c.client.config.ConnectionName)

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
	c.client.config.Logger.Info("Consumer channel recreated successfully",
		"connection_name", c.client.config.ConnectionName)

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
				c.client.config.Logger.Warn("Delivery channel closed",
					"queue", queue)
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

// workerWithOwnChannel is a worker that manages its own dedicated AMQP channel.
// This provides better isolation - if this worker's channel fails, other workers continue.
// The worker handles its own reconnection independently.
func (c *Consumer) workerWithOwnChannel(ctx context.Context, queue string, handler MessageHandler, config *consumeConfig, workerID int) {
	defer c.wg.Done()

	// Generate unique consumer tag for this worker
	consumerTag := c.config.ConsumerTag
	if consumerTag != "" {
		consumerTag = fmt.Sprintf("%s-worker-%d", consumerTag, workerID)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		// Create a dedicated channel for this worker
		ch, err := c.client.getChannel()
		if err != nil {
			c.client.config.Logger.Error("Worker failed to get channel, retrying...",
				"queue", queue,
				"worker_id", workerID,
				"error", err.Error())

			reconnectDelay := max(c.client.config.ReconnectDelay, minReconnectDelay)
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-time.After(reconnectDelay):
				continue
			}
		}

		// Set QoS on this worker's channel
		if c.config.PrefetchCount > 0 || c.config.PrefetchSize > 0 {
			if err := ch.Qos(c.config.PrefetchCount, c.config.PrefetchSize, false); err != nil {
				c.client.config.Logger.Error("Worker failed to set QoS, retrying...",
					"queue", queue,
					"worker_id", workerID,
					"error", err.Error())
				_ = ch.Close()

				reconnectDelay := max(c.client.config.ReconnectDelay, minReconnectDelay)
				select {
				case <-ctx.Done():
					return
				case <-c.stopCh:
					return
				case <-time.After(reconnectDelay):
					continue
				}
			}
		}

		// Start consuming on this worker's channel
		deliveries, err := ch.Consume(
			queue,
			consumerTag,
			c.config.AutoAck,
			c.config.Exclusive,
			c.config.NoLocal,
			c.config.NoWait,
			nil,
		)
		if err != nil {
			c.client.config.Logger.Error("Worker failed to start consuming, retrying...",
				"queue", queue,
				"worker_id", workerID,
				"error", err.Error())
			_ = ch.Close()

			reconnectDelay := max(c.client.config.ReconnectDelay, minReconnectDelay)
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-time.After(reconnectDelay):
				continue
			}
		}

		c.client.config.Logger.Debug("Worker started with dedicated channel",
			"queue", queue,
			"worker_id", workerID,
			"consumer_tag", consumerTag)

		// Process messages until channel closes or stop signal
		channelOK := c.processDeliveriesWithOwnChannel(ctx, queue, handler, deliveries, config, workerID)

		// Clean up channel
		_ = ch.Close()

		if !channelOK {
			// Channel closed unexpectedly, reconnect
			c.client.config.Logger.Warn("Worker channel closed, reconnecting...",
				"queue", queue,
				"worker_id", workerID)

			reconnectDelay := max(c.client.config.ReconnectDelay, minReconnectDelay)
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-time.After(reconnectDelay):
				continue
			}
		}
	}
}

// processDeliveriesWithOwnChannel processes deliveries for a worker with its own channel.
// Returns true if stopped gracefully, false if channel closed unexpectedly.
func (c *Consumer) processDeliveriesWithOwnChannel(ctx context.Context, queue string, handler MessageHandler, deliveries <-chan amqp.Delivery, config *consumeConfig, workerID int) bool {
	for {
		select {
		case <-ctx.Done():
			return true
		case <-c.stopCh:
			return true
		case delivery, ok := <-deliveries:
			if !ok {
				// Channel closed
				return false
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
			c.client.config.Logger.Error("Failed to decrypt message",
				"queue", queue,
				"message_id", delivery.MessageId,
				"error", err.Error())
			// Handle as processing error
			c.handleProcessingError(queue, delivery, fmt.Errorf("decryption failed: %w", err), config)
			return
		}
	}

	// Apply decompression if configured
	if c.config.Compressor != nil {
		if err := c.applyDecompression(enhancedDelivery); err != nil {
			c.client.config.Logger.Error("Failed to decompress message",
				"queue", queue,
				"message_id", delivery.MessageId,
				"error", err.Error())
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
				c.client.config.Logger.Error("Message handler panicked",
					"queue", queue,
					"message_id", delivery.MessageId,
					"panic", fmt.Sprintf("%v", r))
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
				c.client.config.Logger.Error("Failed to acknowledge message",
					"queue", queue,
					"message_id", delivery.MessageId,
					"error", err.Error())
			}
		}

		c.client.config.Logger.Debug("Message processed successfully",
			"queue", queue,
			"message_id", delivery.MessageId,
			"duration", duration)
	} else {
		// Message processing failed
		span.SetStatus(SpanStatusError, handlerErr.Error())
		c.handleProcessingError(queue, delivery, handlerErr, config)
	}
}

// handleProcessingError handles message processing errors with retry logic
func (c *Consumer) handleProcessingError(queue string, delivery *amqp.Delivery, err error, config *consumeConfig) {
	c.client.config.Logger.Warn("Message processing failed",
		"queue", queue,
		"message_id", delivery.MessageId,
		"error", err.Error())

	if c.config.AutoAck {
		// Can't retry if auto-ack is enabled
		return
	}

	// Check if this is a reject error (user explicitly controls requeue)
	if rejectErr, ok := err.(*RejectError); ok {
		if err := delivery.Nack(false, rejectErr.Requeue); err != nil {
			c.client.config.Logger.Error("Failed to nack message",
				"queue", queue,
				"message_id", delivery.MessageId,
				"error", err.Error())
		}

		if rejectErr.Requeue {
			c.client.config.Metrics.RecordMessageRequeued(queue)
		}
		return
	}

	// Header-based retry logic (works across distributed consumers)
	if c.config.MaxRetries > 0 {
		retryCount := c.getRetryCount(delivery)

		if retryCount < c.config.MaxRetries {
			// Still have retries left
			c.client.config.Logger.Info("Retrying message",
				"queue", queue,
				"message_id", delivery.MessageId,
				"attempt", retryCount+1,
				"max_retries", c.config.MaxRetries)

			// Apply backoff if configured
			if c.config.RetryBackoff > 0 {
				time.Sleep(c.config.RetryBackoff)
			}

			// For classic queues: Ack and republish with incremented retry count
			// For quorum queues: Just Nack(requeue=true), broker tracks x-delivery-count
			if c.isQuorumQueue(delivery) {
				// Quorum queue: broker tracks delivery count automatically
				if err := delivery.Nack(false, true); err != nil {
					c.client.config.Logger.Error("Failed to nack (requeue) message",
						"queue", queue,
						"message_id", delivery.MessageId,
						"error", err.Error())
				}
			} else {
				// Classic queue: republish with incremented retry count
				if err := c.republishWithRetry(queue, delivery, retryCount+1); err != nil {
					c.client.config.Logger.Error("Failed to republish message for retry",
						"queue", queue,
						"message_id", delivery.MessageId,
						"error", err.Error())
					// Fall back to nack without requeue (send to DLX)
					_ = delivery.Nack(false, false)
					return
				}
				// Ack the original message since we republished it
				_ = delivery.Ack(false)
			}

			c.client.config.Metrics.RecordMessageRequeued(queue)
			return
		}

		// Max retries exceeded - send to DLX
		c.client.config.Logger.Warn("Max retries exceeded, sending to DLX",
			"queue", queue,
			"message_id", delivery.MessageId,
			"attempts", retryCount+1)

		if err := delivery.Nack(false, false); err != nil {
			c.client.config.Logger.Error("Failed to nack (no requeue) message",
				"queue", queue,
				"message_id", delivery.MessageId,
				"error", err.Error())
		}
		return
	}

	// Default behavior: requeue based on configuration
	requeue := config.RejectRequeue

	if err := delivery.Nack(false, requeue); err != nil {
		c.client.config.Logger.Error("Failed to nack message",
			"queue", queue,
			"message_id", delivery.MessageId,
			"error", err.Error())
	}

	if requeue {
		c.client.config.Metrics.RecordMessageRequeued(queue)
	}
}

// getRetryCount extracts the retry count from message headers
// For quorum queues: uses x-delivery-count (broker-tracked)
// For classic queues: uses x-retry-count (application-tracked)
func (c *Consumer) getRetryCount(delivery *amqp.Delivery) int {
	if delivery.Headers == nil {
		return 0
	}

	// Try quorum queue header first (x-delivery-count)
	if deliveryCount, ok := delivery.Headers["x-delivery-count"]; ok {
		switch v := deliveryCount.(type) {
		case int:
			return v - 1 // x-delivery-count starts at 1, we want 0-based
		case int32:
			return int(v) - 1
		case int64:
			return int(v) - 1
		}
	}

	// Fall back to classic queue header (x-retry-count)
	if retryCount, ok := delivery.Headers["x-retry-count"]; ok {
		switch v := retryCount.(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		}
	}

	return 0
}

// isQuorumQueue checks if the message came from a quorum queue
// Quorum queues have x-delivery-count header set by the broker
func (c *Consumer) isQuorumQueue(delivery *amqp.Delivery) bool {
	if delivery.Headers == nil {
		return false
	}
	_, hasDeliveryCount := delivery.Headers["x-delivery-count"]
	return hasDeliveryCount
}

// republishWithRetry republishes a message with an incremented retry count
// This is used for classic queues where we need to track retries manually
func (c *Consumer) republishWithRetry(queue string, delivery *amqp.Delivery, retryCount int) error {
	// Create a copy of the headers
	headers := make(amqp.Table)
	if delivery.Headers != nil {
		for k, v := range delivery.Headers {
			headers[k] = v
		}
	}

	// Set the retry count header
	headers["x-retry-count"] = retryCount

	// Preserve original message properties
	msg := amqp.Publishing{
		Headers:       headers,
		ContentType:   delivery.ContentType,
		Body:          delivery.Body,
		MessageId:     delivery.MessageId,
		CorrelationId: delivery.CorrelationId,
		ReplyTo:       delivery.ReplyTo,
		Expiration:    delivery.Expiration,
		Timestamp:     delivery.Timestamp,
		Type:          delivery.Type,
		UserId:        delivery.UserId,
		AppId:         delivery.AppId,
		Priority:      delivery.Priority,
		DeliveryMode:  delivery.DeliveryMode,
	}

	// Publish to the same queue (empty exchange, queue as routing key)
	return c.ch.Publish(
		"",    // default exchange
		queue, // routing key = queue name
		false, // mandatory
		false, // immediate
		msg,
	)
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
			c.client.config.Logger.Error("Failed to close consumer channel",
				"error", err.Error())
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}

	c.client.config.Logger.Info("Consumer closed successfully")
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

// ConsumeBatch consumes messages in batches from the specified queue.
// This is useful for batch processing scenarios where processing multiple messages
// together is more efficient than processing them one by one.
//
// The handler receives batches of messages and should return a slice of errors
// (one per message). A nil error at index i means message i was processed successfully.
//
// Messages are acknowledged individually based on the error returned for each:
//   - nil error: message is acknowledged
//   - non-nil error: message is rejected (requeued based on RejectRequeue config)
//
// The method blocks until the context is cancelled or Stop() is called.
func (c *Consumer) ConsumeBatch(ctx context.Context, queue string, handler BatchMessageHandler, config BatchConsumeConfig, opts ...ConsumeOption) error {
	// Validate configuration
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.BatchTimeout <= 0 {
		config.BatchTimeout = 5 * time.Second
	}

	// Apply consume options
	consumeConfig := &consumeConfig{
		RejectRequeue: true, // Default to requeue on failure
	}
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
	ctx, span := c.client.config.Tracer.StartSpan(ctx, "rabbitmq.consume_batch")
	defer span.End()

	span.SetAttribute("queue", queue)
	span.SetAttribute("batch_size", config.BatchSize)
	span.SetAttribute("batch_timeout", config.BatchTimeout.String())

	// Get channel
	ch, err := c.client.getChannel()
	if err != nil {
		span.SetStatus(SpanStatusError, err.Error())
		return fmt.Errorf("failed to get channel: %w", err)
	}

	// Set QoS - prefetch enough for batch processing
	prefetchCount := config.BatchSize
	if c.config.PrefetchCount > 0 && c.config.PrefetchCount > prefetchCount {
		prefetchCount = c.config.PrefetchCount
	}
	if err := ch.Qos(prefetchCount, c.config.PrefetchSize, false); err != nil {
		span.SetStatus(SpanStatusError, err.Error())
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Start consuming
	deliveries, err := ch.Consume(
		queue,
		c.config.ConsumerTag,
		config.AutoAck,
		c.config.Exclusive,
		c.config.NoLocal,
		c.config.NoWait,
		nil,
	)
	if err != nil {
		span.SetStatus(SpanStatusError, err.Error())
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	c.client.config.Logger.Info("Started batch consumption",
		"queue", queue,
		"batch_size", config.BatchSize,
		"batch_timeout", config.BatchTimeout)

	// Process messages in batches
	batch := make([]*Delivery, 0, config.BatchSize)
	batchTimer := time.NewTimer(config.BatchTimeout)
	defer batchTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			// Process remaining batch before exiting
			if len(batch) > 0 {
				c.processBatch(ctx, queue, handler, batch, consumeConfig, config.AutoAck)
			}
			span.SetStatus(SpanStatusOK, "context cancelled")
			return ctx.Err()

		case <-c.stopCh:
			// Process remaining batch before exiting
			if len(batch) > 0 {
				c.processBatch(ctx, queue, handler, batch, consumeConfig, config.AutoAck)
			}
			span.SetStatus(SpanStatusOK, "stopped")
			return nil

		case delivery, ok := <-deliveries:
			if !ok {
				// Channel closed, process remaining batch
				if len(batch) > 0 {
					c.processBatch(ctx, queue, handler, batch, consumeConfig, config.AutoAck)
				}
				span.SetStatus(SpanStatusError, "delivery channel closed")
				return fmt.Errorf("delivery channel closed")
			}

			// Create enhanced delivery
			enhancedDelivery := &Delivery{
				Delivery:   delivery,
				ReceivedAt: time.Now(),
			}

			// Apply decompression if configured
			if c.config.Compressor != nil {
				if err := c.applyDecompression(enhancedDelivery); err != nil {
					c.client.config.Logger.Error("Failed to decompress message in batch",
						"queue", queue,
						"message_id", delivery.MessageId,
						"error", err.Error())
					// Reject this message and continue
					if !config.AutoAck {
						_ = delivery.Reject(consumeConfig.RejectRequeue)
					}
					continue
				}
			}

			// Apply decryption if configured
			if c.config.Encryptor != nil {
				if err := c.applyDecryption(enhancedDelivery); err != nil {
					c.client.config.Logger.Error("Failed to decrypt message in batch",
						"queue", queue,
						"message_id", delivery.MessageId,
						"error", err.Error())
					// Reject this message and continue
					if !config.AutoAck {
						_ = delivery.Reject(consumeConfig.RejectRequeue)
					}
					continue
				}
			}

			batch = append(batch, enhancedDelivery)

			// Process batch if full
			if len(batch) >= config.BatchSize {
				c.processBatch(ctx, queue, handler, batch, consumeConfig, config.AutoAck)
				batch = make([]*Delivery, 0, config.BatchSize)
				batchTimer.Reset(config.BatchTimeout)
			}

		case <-batchTimer.C:
			// Timeout reached, process current batch even if not full
			if len(batch) > 0 {
				c.processBatch(ctx, queue, handler, batch, consumeConfig, config.AutoAck)
				batch = make([]*Delivery, 0, config.BatchSize)
			}
			batchTimer.Reset(config.BatchTimeout)
		}
	}
}

// processBatch processes a batch of messages and handles acknowledgments
func (c *Consumer) processBatch(ctx context.Context, queue string, handler BatchMessageHandler, batch []*Delivery, config *consumeConfig, autoAck bool) {
	if len(batch) == 0 {
		return
	}

	start := time.Now()

	// Start tracing span for batch processing
	ctx, span := c.client.config.Tracer.StartSpan(ctx, "rabbitmq.process_batch")
	defer span.End()

	span.SetAttribute("queue", queue)
	span.SetAttribute("batch_size", len(batch))

	// Call the batch handler with panic recovery
	var errors []error
	func() {
		defer func() {
			if r := recover(); r != nil {
				c.client.config.Logger.Error("Batch handler panicked",
					"queue", queue,
					"batch_size", len(batch),
					"panic", fmt.Sprintf("%v", r))
				// Mark all messages as failed
				errors = make([]error, len(batch))
				for i := range errors {
					errors[i] = fmt.Errorf("handler panicked: %v", r)
				}
			}
		}()
		errors = handler(ctx, batch)
	}()

	// Ensure errors slice has correct length
	if len(errors) != len(batch) {
		c.client.config.Logger.Warn("Batch handler returned wrong number of errors",
			"expected", len(batch),
			"got", len(errors))
		// Pad or truncate errors slice
		if len(errors) < len(batch) {
			padded := make([]error, len(batch))
			copy(padded, errors)
			errors = padded
		} else {
			errors = errors[:len(batch)]
		}
	}

	// Handle acknowledgments for each message
	successCount := 0
	failureCount := 0

	if !autoAck {
		for i, delivery := range batch {
			if errors[i] == nil {
				// Success - acknowledge
				if err := delivery.Ack(); err != nil {
					c.client.config.Logger.Error("Failed to acknowledge message in batch",
						"queue", queue,
						"message_id", delivery.MessageId,
						"error", err.Error())
				}
				successCount++
			} else {
				// Failure - reject
				if err := delivery.Reject(config.RejectRequeue); err != nil {
					c.client.config.Logger.Error("Failed to reject message in batch",
						"queue", queue,
						"message_id", delivery.MessageId,
						"error", err.Error())
				}
				failureCount++
			}
		}
	} else {
		// Count successes/failures for logging
		for _, err := range errors {
			if err == nil {
				successCount++
			} else {
				failureCount++
			}
		}
	}

	duration := time.Since(start)

	// Record metrics - use existing RecordMessageProcessed for each message in the batch
	for i, delivery := range batch {
		c.client.config.Metrics.RecordMessageProcessed(queue, errors[i] == nil, duration/time.Duration(len(batch)))
		if errors[i] == nil {
			c.client.config.Metrics.RecordConsume(queue, len(delivery.Body), duration/time.Duration(len(batch)))
		}
	}

	// Set span status
	if failureCount > 0 {
		span.SetStatus(SpanStatusError, fmt.Sprintf("%d/%d messages failed", failureCount, len(batch)))
	} else {
		span.SetStatus(SpanStatusOK, "")
	}

	c.client.config.Logger.Debug("Batch processed",
		"queue", queue,
		"batch_size", len(batch),
		"success_count", successCount,
		"failure_count", failureCount,
		"duration", duration)
}
