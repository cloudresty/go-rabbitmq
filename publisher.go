package rabbitmq

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher handles message publishing operations
type Publisher struct {
	client *Client
	config *publisherConfig
	ch     *amqp.Channel

	// Delivery assurance fields
	deliveryAssuranceEnabled bool
	confirmChannel           *amqp.Channel      // Dedicated channel for delivery assurance
	pendingMessages          *shardedPendingMap // Sharded map for pending messages (reduces contention by 32x)
	confirmChan              chan amqp.Confirmation
	returnChan               chan amqp.Return
	shutdownChan             chan struct{}
	shutdownWg               sync.WaitGroup
	nextDeliveryTag          uint64
	deliveryTagMutex         sync.Mutex

	// Delivery statistics
	stats      DeliveryStats
	statsMutex sync.RWMutex
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

	// Delivery assurance configuration
	enableDeliveryAssurance bool
	defaultDeliveryCallback DeliveryCallback
	deliveryTimeout         time.Duration
	defaultMandatory        bool
	mandatoryGracePeriod    time.Duration // Grace period to wait for Return after Confirmation on mandatory messages

	// Automatic retry configuration (opt-in feature)
	enableAutomaticRetries bool          // Enable true automatic re-publishing of nacked messages
	maxRetryAttempts       int           // Maximum number of retry attempts
	retryDelay             time.Duration // Delay between retry attempts

	// Connection error retry configuration
	maxConnectionRetries int // Max attempts to refresh channel and retry publish (0 = unlimited, default 3)
}

// pendingMessage tracks a message awaiting delivery confirmation
type pendingMessage struct {
	MessageID    string
	PublishedAt  time.Time
	Callback     DeliveryCallback
	TimeoutTimer *time.Timer
	DeliveryTag  uint64
	Exchange     string
	RoutingKey   string
	Mandatory    bool

	// Automatic retry support (only populated if automatic retries are enabled)
	OriginalMessage *Message        // Cloned message for retry (nil if retries disabled)
	OriginalContext context.Context // Original context for retry (preserves trace IDs, deadlines)
	RetryCount      int             // Current retry attempt number
	RetryOptions    DeliveryOptions // Original delivery options for retry

	// State tracking for delivery assurance
	// These fields are protected by the per-message mutex
	mu            sync.Mutex
	Confirmed     bool   // Has broker confirmed receipt?
	Returned      bool   // Has broker returned the message?
	ReturnReason  string // Reason for return (if returned)
	Nacked        bool   // Has broker nacked the message?
	CallbackFired bool   // Has the callback been invoked?
	FinalOutcome  DeliveryOutcome
	FinalError    string
}

// PublisherOption represents a functional option for publisher configuration
type PublisherOption func(*publisherConfig)

// PublishRequest represents a single publish operation
type PublishRequest struct {
	Exchange   string
	RoutingKey string
	Message    *Message
}

// PublishResult represents the result of a single publish operation in a batch
type PublishResult struct {
	Index   int   // Index of the message in the original batch
	Success bool  // Whether the publish was successful
	Error   error // Error if publish failed (nil if successful)
}

// BatchPublishResult represents the result of a batch publish operation
type BatchPublishResult struct {
	TotalMessages int             // Total number of messages in the batch
	SuccessCount  int             // Number of successfully published messages
	FailureCount  int             // Number of failed messages
	Results       []PublishResult // Individual results for each message
	Duration      time.Duration   // Total time taken for the batch operation
}

// HasFailures returns true if any messages in the batch failed to publish
func (r *BatchPublishResult) HasFailures() bool {
	return r.FailureCount > 0
}

// FailedMessages returns the indices of messages that failed to publish
func (r *BatchPublishResult) FailedMessages() []int {
	failed := make([]int, 0, r.FailureCount)
	for _, result := range r.Results {
		if !result.Success {
			failed = append(failed, result.Index)
		}
	}
	return failed
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

// Delivery Assurance Options

// WithDeliveryAssurance enables delivery assurance for the publisher.
// When enabled, the publisher will track message delivery outcomes (confirmations, returns, nacks)
// and invoke callbacks asynchronously. This provides reliable message delivery guarantees without
// blocking the publish operation.
//
// Delivery assurance uses a dedicated channel with publisher confirms and return notifications
// enabled. Messages are tracked until they are confirmed, returned, or timeout.
//
// Note: This is different from WithConfirmation which blocks on each publish.
// Delivery assurance is asynchronous and uses callbacks for notification.
func WithDeliveryAssurance() PublisherOption {
	return func(config *publisherConfig) {
		config.enableDeliveryAssurance = true
	}
}

// WithDefaultDeliveryCallback sets the default callback for delivery assurance outcomes.
// This callback will be invoked for all messages published with PublishWithDeliveryAssurance
// unless a message-specific callback is provided in DeliveryOptions.
//
// The callback is invoked asynchronously in a background goroutine when the delivery outcome
// is determined (confirmed, returned, nacked, or timeout).
//
// Example:
//
//	callback := func(messageID string, outcome DeliveryOutcome, errorMessage string) {
//	    switch outcome {
//	    case rabbitmq.DeliverySuccess:
//	        log.Printf("Message %s delivered successfully", messageID)
//	    case rabbitmq.DeliveryFailed:
//	        log.Printf("Message %s failed: %s", messageID, errorMessage)
//	    case rabbitmq.DeliveryNacked:
//	        log.Printf("Message %s nacked: %s", messageID, errorMessage)
//	    case rabbitmq.DeliveryTimeout:
//	        log.Printf("Message %s timed out", messageID)
//	    }
//	}
//	publisher, err := client.NewPublisher(
//	    rabbitmq.WithDeliveryAssurance(),
//	    rabbitmq.WithDefaultDeliveryCallback(callback),
//	)
func WithDefaultDeliveryCallback(callback DeliveryCallback) PublisherOption {
	return func(config *publisherConfig) {
		config.defaultDeliveryCallback = callback
	}
}

// WithDeliveryTimeout sets the default timeout for delivery confirmations.
// If a message is not confirmed, returned, or nacked within this timeout,
// the delivery callback will be invoked with DeliveryTimeout outcome.
//
// Default: 30 seconds if not specified
//
// Example:
//
//	publisher, err := client.NewPublisher(
//	    rabbitmq.WithDeliveryAssurance(),
//	    rabbitmq.WithDeliveryTimeout(10 * time.Second),
//	)
func WithDeliveryTimeout(timeout time.Duration) PublisherOption {
	return func(config *publisherConfig) {
		config.deliveryTimeout = timeout
	}
}

// WithMandatoryByDefault sets the default mandatory flag for delivery assurance.
// When true, all messages published with PublishWithDeliveryAssurance will be
// mandatory by default (unless overridden in DeliveryOptions).
//
// Mandatory messages are returned by the broker if they cannot be routed to any queue.
// This allows the application to detect routing failures.
//
// Example:
//
//	publisher, err := client.NewPublisher(
//	    rabbitmq.WithDeliveryAssurance(),
//	    rabbitmq.WithMandatoryByDefault(true),
//	)
func WithMandatoryByDefault(mandatory bool) PublisherOption {
	return func(config *publisherConfig) {
		config.defaultMandatory = mandatory
	}
}

// WithMandatoryGracePeriod sets the grace period to wait for Return notifications
// after receiving a Confirmation for mandatory messages.
//
// When a mandatory message is published, RabbitMQ sends both a Confirmation (ack/nack)
// and potentially a Return notification (if the message couldn't be routed). These can
// arrive in any order. The grace period ensures we wait long enough to receive both
// before finalizing the message as successful.
//
// Default: 200ms (suitable for most deployments)
// Increase this in high-latency environments or if you observe false positives
// (messages marked as success when they should have failed with NO_ROUTE).
//
// Example:
//
//	publisher, err := client.NewPublisher(
//	    rabbitmq.WithDeliveryAssurance(),
//	    rabbitmq.WithMandatoryByDefault(true),
//	    rabbitmq.WithMandatoryGracePeriod(500 * time.Millisecond), // For high-latency networks
//	)
func WithMandatoryGracePeriod(duration time.Duration) PublisherOption {
	return func(config *publisherConfig) {
		config.mandatoryGracePeriod = duration
	}
}

// WithPublisherRetry enables true, automatic re-publishing of nacked messages.
//
// When a message is nacked by the broker (typically due to transient issues like
// resource constraints), this feature will automatically re-publish the message
// up to maxAttempts times with the specified backoff between attempts.
//
// ⚠️ IMPORTANT: This feature stores the full message in memory until it is confirmed,
// which will increase the publisher's memory footprint. Use it for critical messages
// where at-least-once delivery is paramount.
//
// Parameters:
//   - maxAttempts: Maximum number of retry attempts (e.g., 3 means original + 3 retries = 4 total attempts)
//   - backoff: Time to wait between retry attempts (e.g., 1*time.Second)
//
// Example:
//
//	publisher, err := client.NewPublisher(
//	    rabbitmq.WithDeliveryAssurance(),
//	    rabbitmq.WithPublisherRetry(3, 1*time.Second), // Retry up to 3 times with 1s backoff
//	)
//
// Note: If you need more control over retry logic (e.g., exponential backoff, custom
// retry conditions), implement your own retry logic in the delivery callback instead.
func WithPublisherRetry(maxAttempts int, backoff time.Duration) PublisherOption {
	return func(config *publisherConfig) {
		config.enableAutomaticRetries = true
		config.maxRetryAttempts = maxAttempts
		config.retryDelay = backoff
	}
}

// WithPublisherConnectionRetry sets the maximum number of retry attempts for
// channel/connection errors during publish operations.
//
// When a publish fails with "channel/connection is not open", "channel already closed",
// or "connection closed" errors, the publisher will:
//  1. Wait using the ReconnectPolicy delay
//  2. Refresh the channel
//  3. Retry the publish
//
// Parameters:
//   - maxAttempts: Maximum retry attempts. Use 0 for unlimited retries.
//     Default is 3 if not specified.
//
// Example:
//
//	// Retry for up to 2 hours during RabbitMQ maintenance windows
//	// With exponential backoff (5s, 10s, 20s, 40s, ... capped at 5min)
//	// This allows ~24 retries over 2 hours
//	rabbitmq.WithPublisherConnectionRetry(0), // Unlimited - rely on context timeout
//
// Note: The actual timeout is controlled by the context passed to Publish().
// Set maxAttempts=0 and use context.WithTimeout() for time-based limits.
func WithPublisherConnectionRetry(maxAttempts int) PublisherOption {
	return func(config *publisherConfig) {
		config.maxConnectionRetries = maxAttempts
	}
}

// Core publishing methods

// Publish publishes a message to the specified exchange and routing key
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, message *Message) error {
	// Use default exchange if not specified
	if exchange == "" {
		exchange = p.config.DefaultExchange
	}

	// Validate topology if enabled
	if p.client.TopologyValidator() != nil && exchange != "" {
		if err := p.client.TopologyValidator().ValidateExchange(exchange); err != nil {
			return fmt.Errorf("topology validation failed for exchange '%s': %w", exchange, err)
		}
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

	// Publish message with auto-reconnect retry support
	var publishErr error
	// Use configured max retries (0 = unlimited)
	maxConnRetries := p.config.maxConnectionRetries

	for attempt := 0; ; attempt++ {
		// Check if we've exceeded max retries (skip check if unlimited)
		if maxConnRetries > 0 && attempt > maxConnRetries {
			break
		}

		publishErr = p.ch.PublishWithContext(
			ctx,
			exchange,
			routingKey,
			p.config.Mandatory,
			p.config.Immediate,
			publishing,
		)

		// Success - message published
		if publishErr == nil {
			break
		}

		// Check if this is a channel/connection error and AutoReconnect is enabled
		if isChannelError(publishErr) && p.client.config.AutoReconnect {
			// Check context cancellation FIRST (important for timeout control with unlimited retries)
			select {
			case <-ctx.Done():
				publishErr = ctx.Err()
				break
			default:
			}

			// Check if ReconnectPolicy allows retry
			if p.client.config.ReconnectPolicy != nil && !p.client.config.ReconnectPolicy.ShouldRetry(attempt+1, publishErr) {
				p.client.config.Logger.Warn("ReconnectPolicy indicates no more retries",
					"error", publishErr,
					"attempt", attempt+1,
					"message_id", message.MessageID)
				break
			}

			// Log max_attempts as "unlimited" when 0
			maxAttemptsLog := any(maxConnRetries)
			if maxConnRetries == 0 {
				maxAttemptsLog = "unlimited"
			}

			p.client.config.Logger.Warn("Publish failed due to channel/connection error, attempting to refresh channel",
				"error", publishErr,
				"attempt", attempt+1,
				"max_attempts", maxAttemptsLog,
				"message_id", message.MessageID)

			// Calculate delay based on ReconnectPolicy if available, otherwise use ReconnectDelay
			var reconnectDelay time.Duration
			if p.client.config.ReconnectPolicy != nil {
				reconnectDelay = p.client.config.ReconnectPolicy.NextDelay(attempt + 1)
			} else {
				reconnectDelay = p.client.config.ReconnectDelay
			}
			reconnectDelay = max(reconnectDelay, minReconnectDelay)

			select {
			case <-ctx.Done():
				publishErr = ctx.Err()
				break
			case <-time.After(reconnectDelay):
				// Attempt to refresh the channel
				if refreshErr := p.refreshChannel(); refreshErr != nil {
					p.client.config.Logger.Error("Failed to refresh channel",
						"error", refreshErr,
						"attempt", attempt+1,
						"message_id", message.MessageID)
					continue // Try again if we have retries left
				}

				p.client.config.Logger.Info("Channel refreshed, retrying publish",
					"message_id", message.MessageID,
					"attempt", attempt+2)
				continue // Retry the publish with the refreshed channel
			}
		}

		// Not a channel error or AutoReconnect disabled - don't retry
		break
	}

	// Record performance metrics
	duration := time.Since(start)
	success := publishErr == nil
	p.client.config.Metrics.RecordPublish(exchange, routingKey, len(message.Body), duration)
	if p.client.config.PerformanceMonitor != nil {
		p.client.config.PerformanceMonitor.RecordPublish(success, duration)
	}

	if publishErr != nil {
		p.client.config.Metrics.RecordError("publish", publishErr)
		span.SetStatus(SpanStatusError, publishErr.Error())
		return fmt.Errorf("failed to publish message: %w", publishErr)
	}

	// Wait for confirmation if publisher is configured for confirmations
	if p.config.ConfirmationEnabled {
		if err := p.waitForConfirmation(ctx); err != nil {
			span.SetStatus(SpanStatusError, err.Error())
			return fmt.Errorf("confirmation failed: %w", err)
		}
	}

	p.client.config.Logger.Debug("Message published successfully",
		"exchange", exchange,
		"routing_key", routingKey,
		"message_id", message.MessageID,
		"correlation_id", message.CorrelationID,
		"confirmed", p.config.ConfirmationEnabled)

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
	p.client.config.Logger.Info("Batch published successfully",
		"batch_size", len(messages))

	return nil
}

// PublishBatchParallel publishes multiple messages concurrently for higher throughput.
// Unlike PublishBatch which publishes sequentially and stops on first error,
// this method publishes all messages in parallel and returns detailed results for each.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - messages: Slice of publish requests to send
//   - maxConcurrency: Maximum number of concurrent publish operations (0 = unlimited)
//
// Returns BatchPublishResult with individual results for each message.
// This allows partial success handling where some messages succeed and others fail.
func (p *Publisher) PublishBatchParallel(ctx context.Context, messages []PublishRequest, maxConcurrency int) *BatchPublishResult {
	start := time.Now()
	result := &BatchPublishResult{
		TotalMessages: len(messages),
		Results:       make([]PublishResult, len(messages)),
	}

	if len(messages) == 0 {
		result.Duration = time.Since(start)
		return result
	}

	// Start tracing span
	ctx, span := p.client.config.Tracer.StartSpan(ctx, "rabbitmq.publish_batch_parallel")
	defer span.End()

	span.SetAttribute("batch_size", len(messages))
	span.SetAttribute("max_concurrency", maxConcurrency)

	// Use semaphore for concurrency limiting if maxConcurrency > 0
	var sem chan struct{}
	if maxConcurrency > 0 {
		sem = make(chan struct{}, maxConcurrency)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex // Protects result counters

	for i, req := range messages {
		wg.Add(1)

		go func(index int, request PublishRequest) {
			defer wg.Done()

			// Acquire semaphore slot if concurrency is limited
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					mu.Lock()
					result.Results[index] = PublishResult{
						Index:   index,
						Success: false,
						Error:   ctx.Err(),
					}
					result.FailureCount++
					mu.Unlock()
					return
				}
			}

			// Check context before publishing
			if ctx.Err() != nil {
				mu.Lock()
				result.Results[index] = PublishResult{
					Index:   index,
					Success: false,
					Error:   ctx.Err(),
				}
				result.FailureCount++
				mu.Unlock()
				return
			}

			// Publish the message
			err := p.Publish(ctx, request.Exchange, request.RoutingKey, request.Message)

			mu.Lock()
			if err != nil {
				result.Results[index] = PublishResult{
					Index:   index,
					Success: false,
					Error:   err,
				}
				result.FailureCount++
			} else {
				result.Results[index] = PublishResult{
					Index:   index,
					Success: true,
					Error:   nil,
				}
				result.SuccessCount++
			}
			mu.Unlock()
		}(i, req)
	}

	wg.Wait()
	result.Duration = time.Since(start)

	// Set span status based on results
	if result.FailureCount > 0 {
		span.SetStatus(SpanStatusError, fmt.Sprintf("%d/%d messages failed", result.FailureCount, result.TotalMessages))
	} else {
		span.SetStatus(SpanStatusOK, "")
	}

	p.client.config.Logger.Info("Parallel batch publish completed",
		"batch_size", len(messages),
		"success_count", result.SuccessCount,
		"failure_count", result.FailureCount,
		"duration", result.Duration)

	return result
}

// PublishWithDeliveryAssurance publishes a message with delivery assurance tracking.
// This method provides asynchronous delivery guarantees through callbacks without blocking.
//
// The message is published and tracked until one of the following outcomes occurs:
//   - DeliverySuccess: Message confirmed by broker and successfully routed
//   - DeliveryFailed: Message returned by broker (no queue bound to routing key)
//   - DeliveryNacked: Message negatively acknowledged by broker
//   - DeliveryTimeout: No confirmation received within the timeout period
//
// The delivery outcome is reported asynchronously via the callback specified in options
// or the publisher's default callback.
//
// Example:
//
//	err := publisher.PublishWithDeliveryAssurance(ctx, "events", "user.created", message,
//	    rabbitmq.DeliveryOptions{
//	        MessageID: "event-123",
//	        Mandatory: true,
//	        Callback: func(messageID string, outcome DeliveryOutcome, errorMessage string) {
//	            if outcome == rabbitmq.DeliverySuccess {
//	                log.Printf("Message %s delivered", messageID)
//	            } else {
//	                log.Printf("Message %s failed: %s", messageID, errorMessage)
//	            }
//	        },
//	    })
func (p *Publisher) PublishWithDeliveryAssurance(ctx context.Context, exchange, routingKey string, message *Message, options DeliveryOptions) error {
	if !p.deliveryAssuranceEnabled {
		return fmt.Errorf("delivery assurance is not enabled for this publisher")
	}

	// Use default exchange if not specified
	if exchange == "" {
		exchange = p.config.DefaultExchange
	}

	// Validate topology if enabled
	if p.client.TopologyValidator() != nil && exchange != "" {
		if err := p.client.TopologyValidator().ValidateExchange(exchange); err != nil {
			return fmt.Errorf("topology validation failed for exchange '%s': %w", exchange, err)
		}
	}

	// Determine message ID
	messageID := options.MessageID
	if messageID == "" {
		messageID = message.MessageID
	}
	if messageID == "" {
		return fmt.Errorf("message ID is required for delivery assurance")
	}

	// Determine callback
	callback := options.Callback
	if callback == nil {
		callback = p.config.defaultDeliveryCallback
	}
	if callback == nil {
		return fmt.Errorf("delivery callback is required (set via options or WithDefaultDeliveryCallback)")
	}

	// Determine timeout
	timeout := options.Timeout
	if timeout == 0 {
		timeout = p.config.deliveryTimeout
	}

	// Determine mandatory flag
	mandatory := options.Mandatory
	if !mandatory && p.config.defaultMandatory {
		mandatory = true
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

	// Ensure MessageId is set for correlation with returns
	publishing.MessageId = messageID

	// Get next delivery tag for tracking
	deliveryTag := p.getNextDeliveryTag()

	// Create pending message entry
	pending := &pendingMessage{
		MessageID:   messageID,
		PublishedAt: time.Now(),
		Callback:    callback,
		DeliveryTag: deliveryTag,
		Exchange:    exchange,
		RoutingKey:  routingKey,
		Mandatory:   mandatory,
		RetryCount:  0,
	}

	// If automatic retries are enabled, clone the message and store context for retry
	if p.config.enableAutomaticRetries {
		pending.OriginalMessage = message.Clone()
		pending.OriginalContext = ctx // Preserve context for trace propagation during retry
		pending.RetryOptions = options
	}

	// Set up timeout timer
	pending.TimeoutTimer = time.AfterFunc(timeout, func() {
		p.handleTimeout(deliveryTag)
	})

	// Add to pending messages before publishing
	// Check for duplicate MessageID to prevent race conditions
	if !p.pendingMessages.Store(messageID, deliveryTag, pending) {
		return fmt.Errorf("message with ID '%s' is already pending delivery - MessageID must be unique for in-flight messages", messageID)
	}
	pendingCount := p.pendingMessages.Count()

	// Update statistics
	p.statsMutex.Lock()
	p.stats.TotalPublished++
	p.stats.PendingMessages = pendingCount
	p.statsMutex.Unlock()

	// Start tracing span
	ctx, span := p.client.config.Tracer.StartSpan(ctx, "rabbitmq.publish_with_delivery_assurance")
	defer span.End()

	span.SetAttribute("exchange", exchange)
	span.SetAttribute("routing_key", routingKey)
	span.SetAttribute("message_id", messageID)
	span.SetAttribute("mandatory", mandatory)
	span.SetAttribute("delivery_assurance", true)

	// Publish message using the dedicated confirmation channel
	// With auto-reconnect retry support for connection/channel errors
	var publishErr error
	// Use configured max retries (0 = unlimited)
	maxConnRetries := p.config.maxConnectionRetries

	for attempt := 0; ; attempt++ {
		// Check if we've exceeded max retries (skip check if unlimited)
		if maxConnRetries > 0 && attempt > maxConnRetries {
			break
		}

		publishErr = p.confirmChannel.PublishWithContext(
			ctx,
			exchange,
			routingKey,
			mandatory,
			p.config.Immediate,
			publishing,
		)

		// Success - message published
		if publishErr == nil {
			break
		}

		// Check if this is a channel/connection error and AutoReconnect is enabled
		if isChannelError(publishErr) && p.client.config.AutoReconnect {
			// Check context cancellation FIRST (important for timeout control with unlimited retries)
			select {
			case <-ctx.Done():
				publishErr = ctx.Err()
				break
			default:
			}

			// Check if ReconnectPolicy allows retry
			if p.client.config.ReconnectPolicy != nil && !p.client.config.ReconnectPolicy.ShouldRetry(attempt+1, publishErr) {
				p.client.config.Logger.Warn("ReconnectPolicy indicates no more retries",
					"error", publishErr,
					"attempt", attempt+1,
					"message_id", messageID)
				break
			}

			// Log max_attempts as "unlimited" when 0
			maxAttemptsLog := any(maxConnRetries)
			if maxConnRetries == 0 {
				maxAttemptsLog = "unlimited"
			}

			p.client.config.Logger.Warn("Publish failed due to channel/connection error, attempting to refresh channel",
				"error", publishErr,
				"attempt", attempt+1,
				"max_attempts", maxAttemptsLog,
				"message_id", messageID)

			// Calculate delay based on ReconnectPolicy if available, otherwise use ReconnectDelay
			var reconnectDelay time.Duration
			if p.client.config.ReconnectPolicy != nil {
				reconnectDelay = p.client.config.ReconnectPolicy.NextDelay(attempt + 1)
			} else {
				reconnectDelay = p.client.config.ReconnectDelay
			}
			reconnectDelay = max(reconnectDelay, minReconnectDelay)

			select {
			case <-ctx.Done():
				publishErr = ctx.Err()
				break
			case <-time.After(reconnectDelay):
				// Attempt to refresh the channel
				if refreshErr := p.refreshConfirmChannel(); refreshErr != nil {
					p.client.config.Logger.Error("Failed to refresh confirm channel",
						"error", refreshErr,
						"attempt", attempt+1,
						"message_id", messageID)
					// Continue to next attempt if we have retries left
					continue
				}

				p.client.config.Logger.Info("Confirm channel refreshed, retrying publish",
					"message_id", messageID,
					"attempt", attempt+2)

				// Need to re-register the pending message with a new delivery tag
				// since we have a new channel
				p.pendingMessages.Delete(messageID, deliveryTag)
				pending.TimeoutTimer.Stop()

				deliveryTag = p.getNextDeliveryTag()
				pending.DeliveryTag = deliveryTag
				pending.TimeoutTimer = time.AfterFunc(timeout, func() {
					p.handleTimeout(deliveryTag)
				})

				if !p.pendingMessages.Store(messageID, deliveryTag, pending) {
					publishErr = fmt.Errorf("message with ID '%s' is already pending delivery after channel refresh", messageID)
					break
				}

				continue // Retry the publish with the refreshed channel
			}
		}

		// Not a channel error or AutoReconnect disabled - don't retry
		break
	}

	if publishErr != nil {
		// Remove from pending messages on publish error
		p.pendingMessages.Delete(messageID, deliveryTag)
		pendingCount := p.pendingMessages.Count()

		// Stop timeout timer
		pending.TimeoutTimer.Stop()

		// Update statistics
		p.statsMutex.Lock()
		p.stats.PendingMessages = pendingCount
		p.statsMutex.Unlock()

		p.client.config.Metrics.RecordError("publish_with_delivery_assurance", publishErr)
		span.SetStatus(SpanStatusError, publishErr.Error())
		return fmt.Errorf("failed to publish message: %w", publishErr)
	}

	span.SetStatus(SpanStatusOK, "")
	p.client.config.Logger.Debug("Message published with delivery assurance",
		"message_id", messageID,
		"exchange", exchange,
		"routing_key", routingKey,
		"mandatory", mandatory)

	return nil
}

// Publisher management

// Close closes the publisher and its channel.
// If delivery assurance is enabled, this method will:
//   - Signal shutdown to background goroutines
//   - Wait for pending confirmations (with timeout)
//   - Close the dedicated confirmation channel
//   - Clean up all resources
func (p *Publisher) Close() error {
	// Shutdown delivery assurance if enabled
	if p.deliveryAssuranceEnabled {
		pendingCount := p.pendingMessages.Count()

		p.client.config.Logger.Info("Shutting down delivery assurance",
			"pending_messages", pendingCount)

		// Signal shutdown to background goroutines
		close(p.shutdownChan)

		// Wait for background goroutines to finish (with timeout)
		done := make(chan struct{})
		go func() {
			p.shutdownWg.Wait()
			close(done)
		}()

		select {
		case <-done:
			p.client.config.Logger.Debug("Delivery assurance goroutines stopped")
		case <-time.After(5 * time.Second):
			p.client.config.Logger.Warn("Timeout waiting for delivery assurance goroutines to stop")
		}

		// Cancel all pending message timeouts
		var finalPendingCount int64
		p.pendingMessages.Range(func(msgID string, pending *pendingMessage) bool {
			if pending.TimeoutTimer != nil {
				pending.TimeoutTimer.Stop()
			}
			finalPendingCount++
			return true
		})

		if finalPendingCount > 0 {
			p.client.config.Logger.Warn("Publisher closed with pending messages",
				"pending_count", finalPendingCount)
		}

		// Close the dedicated confirmation channel
		if p.confirmChannel != nil && !p.confirmChannel.IsClosed() {
			if err := p.confirmChannel.Close(); err != nil {
				p.client.config.Logger.Error("Failed to close confirmation channel",
					"error", err.Error())
			}
		}
	}

	// Close the main publishing channel
	if p.ch != nil && !p.ch.IsClosed() {
		if err := p.ch.Close(); err != nil {
			p.client.config.Logger.Error("Failed to close publisher channel",
				"error", err.Error())
			return fmt.Errorf("failed to close channel: %w", err)
		}
	}

	p.client.config.Logger.Info("Publisher closed successfully")
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

// Delivery Assurance Infrastructure

// initDeliveryAssurance initializes the delivery assurance infrastructure
func (p *Publisher) initDeliveryAssurance() error {
	// Create a dedicated channel for delivery assurance
	confirmCh, err := p.client.getChannel()
	if err != nil {
		return fmt.Errorf("failed to create confirmation channel: %w", err)
	}

	// Enable publisher confirms on the dedicated channel
	if err := confirmCh.Confirm(false); err != nil {
		_ = confirmCh.Close()
		return fmt.Errorf("failed to enable confirm mode: %w", err)
	}

	// Initialize publisher fields
	p.deliveryAssuranceEnabled = true
	p.confirmChannel = confirmCh
	p.pendingMessages = newShardedPendingMap()
	p.confirmChan = make(chan amqp.Confirmation, 100)
	p.returnChan = make(chan amqp.Return, 100)
	p.shutdownChan = make(chan struct{})
	p.nextDeliveryTag = 1

	// Set up notification channels
	confirmCh.NotifyPublish(p.confirmChan)
	confirmCh.NotifyReturn(p.returnChan)

	// Start background goroutines for processing confirmations and returns
	p.shutdownWg.Add(2)
	go p.processConfirmations()
	go p.processReturns()

	p.client.config.Logger.Info("Delivery assurance initialized",
		"connection_name", p.client.config.ConnectionName)

	return nil
}

// refreshConfirmChannel attempts to get a new channel from the client and reinitialize
// the delivery assurance infrastructure. This is used when AutoReconnect is enabled
// and the current channel has closed due to a connection error.
//
// This method performs a "hot-swap" of the internal channel:
//  1. Gets a new channel from the client
//  2. Enables confirm mode on the new channel
//  3. Sets up new notification channels
//  4. Resets the delivery tag counter
//
// Note: Pending messages from the old channel will timeout normally.
// The background goroutines (processConfirmations/processReturns) continue running
// as they receive from channels that are replaced atomically.
func (p *Publisher) refreshConfirmChannel() error {
	// Get a new channel from the client
	newCh, err := p.client.getChannel()
	if err != nil {
		return fmt.Errorf("failed to get new channel: %w", err)
	}

	// Enable confirm mode on the new channel
	if err := newCh.Confirm(false); err != nil {
		_ = newCh.Close()
		return fmt.Errorf("failed to enable confirm mode on new channel: %w", err)
	}

	// Create new notification channels
	newConfirmChan := make(chan amqp.Confirmation, 100)
	newReturnChan := make(chan amqp.Return, 100)

	// Register notification handlers on the new channel
	newCh.NotifyPublish(newConfirmChan)
	newCh.NotifyReturn(newReturnChan)

	// Atomically swap the channels
	// Note: The old confirmChannel will be closed by RabbitMQ when connection drops
	p.confirmChannel = newCh
	p.confirmChan = newConfirmChan
	p.returnChan = newReturnChan

	// Reset delivery tag counter for new channel
	// Each channel has its own delivery tag sequence starting from 1
	p.deliveryTagMutex.Lock()
	p.nextDeliveryTag = 1
	p.deliveryTagMutex.Unlock()

	p.client.config.Logger.Info("Confirm channel refreshed successfully",
		"connection_name", p.client.config.ConnectionName)

	return nil
}

// isChannelError checks if an error indicates the channel or connection is closed
func isChannelError(err error) bool {
	if err == nil {
		return false
	}
	if err == amqp.ErrClosed {
		return true
	}
	errStr := err.Error()
	return strings.Contains(errStr, "channel/connection is not open") ||
		strings.Contains(errStr, "channel already closed") ||
		strings.Contains(errStr, "connection closed")
}

// refreshChannel attempts to get a new channel for regular (non-delivery-assurance) publishing.
// This is used when AutoReconnect is enabled and the current channel has closed.
func (p *Publisher) refreshChannel() error {
	// Get a new channel from the client
	newCh, err := p.client.getChannel()
	if err != nil {
		return fmt.Errorf("failed to get new channel: %w", err)
	}

	// Enable confirmation mode if it was enabled on the old channel
	if p.config.ConfirmationEnabled {
		if err := newCh.Confirm(false); err != nil {
			_ = newCh.Close()
			return fmt.Errorf("failed to enable confirm mode on new channel: %w", err)
		}
		// Set up new confirmation channel
		p.config.confirmations = newCh.NotifyPublish(make(chan amqp.Confirmation, 100))
	}

	// Swap the channel
	p.ch = newCh

	p.client.config.Logger.Info("Publisher channel refreshed successfully",
		"connection_name", p.client.config.ConnectionName)

	return nil
}

// processConfirmations processes publisher confirmations in the background
func (p *Publisher) processConfirmations() {
	defer p.shutdownWg.Done()

	for {
		select {
		case <-p.shutdownChan:
			return

		case confirmation, ok := <-p.confirmChan:
			if !ok {
				return
			}

			p.handleConfirmation(confirmation)
		}
	}
}

// processReturns processes returned messages in the background
func (p *Publisher) processReturns() {
	defer p.shutdownWg.Done()

	for {
		select {
		case <-p.shutdownChan:
			return

		case ret, ok := <-p.returnChan:
			if !ok {
				return
			}

			p.handleReturn(ret)
		}
	}
}

// handleConfirmation processes a single confirmation
func (p *Publisher) handleConfirmation(confirmation amqp.Confirmation) {
	// Get the pending message using sharded map
	pending, exists := p.pendingMessages.LoadByDeliveryTag(confirmation.DeliveryTag)

	if !exists {
		return
	}

	// Lock the message state (per-message lock)
	pending.mu.Lock()
	defer pending.mu.Unlock()

	// Log after acquiring lock to avoid race conditions
	p.client.config.Logger.Debug("Confirmation received",
		"message_id", pending.MessageID,
		"delivery_tag", confirmation.DeliveryTag,
		"ack", confirmation.Ack,
		"mandatory", pending.Mandatory,
		"returned", pending.Returned)

	// Mark as confirmed or nacked
	if confirmation.Ack {
		pending.Confirmed = true
	} else {
		pending.Nacked = true
	}

	// Try to finalize based on current state
	p.tryFinalizeMessage(pending)
}

// handleReturn processes a returned message
func (p *Publisher) handleReturn(ret amqp.Return) {
	// Find the pending message by correlation
	// Note: Returns don't have delivery tags, so we need to match by message ID
	messageID := ret.MessageId

	p.client.config.Logger.Debug("Return notification received",
		"message_id", messageID,
		"reply_code", ret.ReplyCode,
		"reply_text", ret.ReplyText,
		"exchange", ret.Exchange,
		"routing_key", ret.RoutingKey)

	// Find the message by MessageID using sharded map
	pending, exists := p.pendingMessages.LoadByMessageID(messageID)
	pendingCount := p.pendingMessages.Count()

	if !exists || pending == nil {
		// Message not found - may have already been finalized or timed out
		// This is OK - just log and return
		p.client.config.Logger.Warn("Return received for unknown message",
			"message_id", messageID,
			"reply_code", ret.ReplyCode,
			"reply_text", ret.ReplyText,
			"pending_count", pendingCount)
		return
	}

	// Lock the message state (per-message lock)
	pending.mu.Lock()
	defer pending.mu.Unlock()

	// Log after acquiring lock to avoid race conditions
	p.client.config.Logger.Debug("Return matched to pending message",
		"message_id", messageID,
		"delivery_tag", pending.DeliveryTag,
		"confirmed", pending.Confirmed,
		"callback_fired", pending.CallbackFired)

	// Check if callback already fired
	if pending.CallbackFired {
		p.client.config.Logger.Warn("Return received but callback already fired",
			"message_id", messageID,
			"delivery_tag", pending.DeliveryTag,
			"final_outcome", pending.FinalOutcome)
		return
	}

	// Mark as returned and store the reason
	pending.Returned = true
	pending.ReturnReason = fmt.Sprintf("message returned: %s (reply code: %d)", ret.ReplyText, ret.ReplyCode)

	p.client.config.Logger.Debug("Marked message as returned, trying to finalize",
		"message_id", messageID,
		"delivery_tag", pending.DeliveryTag,
		"confirmed", pending.Confirmed)

	// Try to finalize based on current state
	p.tryFinalizeMessage(pending)
}

// tryFinalizeMessage attempts to finalize a message based on its current state
// Must be called with pending.mu held
func (p *Publisher) tryFinalizeMessage(pending *pendingMessage) {
	// If callback already fired, do nothing
	if pending.CallbackFired {
		return
	}

	// Determine if we have enough information to finalize
	var canFinalize bool
	var outcome DeliveryOutcome
	var errorMessage string

	if pending.Nacked {
		// Nack is always final - no need to wait for anything else
		canFinalize = true
		outcome = DeliveryNacked
		errorMessage = "message negatively acknowledged by broker"
	} else if pending.Returned {
		// Return is always final for mandatory messages
		// Even if not confirmed yet, the return means it failed
		canFinalize = true
		outcome = DeliveryFailed
		errorMessage = pending.ReturnReason
	} else if pending.Confirmed && !pending.Mandatory {
		// Non-mandatory message: confirmation is enough
		canFinalize = true
		outcome = DeliverySuccess
		errorMessage = ""
	} else if pending.Confirmed && pending.Mandatory {
		// Mandatory message confirmed but not returned YET
		// We need to wait a grace period for potential return
		p.client.config.Logger.Debug("Mandatory message confirmed, starting grace period",
			"message_id", pending.MessageID,
			"delivery_tag", pending.DeliveryTag)

		// Reset the timeout timer to the configured grace period
		if pending.TimeoutTimer != nil {
			pending.TimeoutTimer.Stop()
		}
		pending.TimeoutTimer = time.AfterFunc(p.config.mandatoryGracePeriod, func() {
			p.handleMandatoryGracePeriodExpired(pending.DeliveryTag)
		})
		// Don't finalize yet - wait for return or grace period expiration
		return
	}

	if !canFinalize {
		// Not enough information yet - wait for more events
		p.client.config.Logger.Debug("Not enough information to finalize",
			"message_id", pending.MessageID,
			"delivery_tag", pending.DeliveryTag,
			"confirmed", pending.Confirmed,
			"returned", pending.Returned,
			"nacked", pending.Nacked,
			"mandatory", pending.Mandatory)
		return
	}

	p.client.config.Logger.Debug("Finalizing message",
		"message_id", pending.MessageID,
		"delivery_tag", pending.DeliveryTag,
		"outcome", outcome,
		"error", errorMessage)

	// Mark callback as fired
	pending.CallbackFired = true
	pending.FinalOutcome = outcome
	pending.FinalError = errorMessage

	// Stop the timeout timer if still running
	if pending.TimeoutTimer != nil {
		pending.TimeoutTimer.Stop()
	}

	// Calculate duration
	duration := time.Since(pending.PublishedAt)

	// Update statistics
	p.statsMutex.Lock()
	switch outcome {
	case DeliverySuccess:
		p.stats.TotalConfirmed++
		p.stats.LastConfirmation = time.Now()
	case DeliveryFailed:
		p.stats.TotalReturned++
		p.stats.LastReturn = time.Now()
	case DeliveryNacked:
		p.stats.TotalNacked++
		p.stats.LastNack = time.Now()
	case DeliveryTimeout:
		p.stats.TotalTimedOut++
	}
	p.statsMutex.Unlock()

	// Record metrics
	p.client.config.Metrics.RecordDeliveryOutcome(outcome, duration)

	// Copy all necessary data for the callback while still holding the per-message lock
	callback := pending.Callback
	messageID := pending.MessageID
	deliveryTag := pending.DeliveryTag

	// Check if we should attempt automatic retry
	shouldRetry := (outcome == DeliveryNacked &&
		p.config.enableAutomaticRetries &&
		pending.OriginalMessage != nil &&
		pending.RetryCount < p.config.maxRetryAttempts)

	// If we should retry, handle it differently
	if shouldRetry {
		// Don't invoke callback yet - we'll retry the message
		// The retryMessage function will handle cleanup and callback
		go p.retryMessage(pending)
		// The deferred pending.mu.Unlock() will execute when the function returns
		return
	}

	// Schedule the callback and cleanup to run in a separate goroutine
	// This allows us to release the lock immediately and prevents blocking the handler loop
	go func() {
		// Invoke the callback (if provided)
		if callback != nil {
			callback(messageID, outcome, errorMessage)
		}

		// Remove from pending messages after callback is done
		p.pendingMessages.Delete(messageID, deliveryTag)
		pendingCount := p.pendingMessages.Count()

		// Update pending count in stats
		p.statsMutex.Lock()
		p.stats.PendingMessages = pendingCount
		p.statsMutex.Unlock()
	}()

	// The deferred pending.mu.Unlock() will now execute correctly when the function returns
}

// retryMessage attempts to retry a nacked message by re-publishing it
// This function is called in a goroutine and does NOT hold any locks
func (p *Publisher) retryMessage(pending *pendingMessage) {
	// Acquire the per-message lock to safely read state
	pending.mu.Lock()

	// Increment retry count
	pending.RetryCount++
	retryCount := pending.RetryCount
	maxRetries := p.config.maxRetryAttempts

	p.client.config.Logger.Info("Automatic retry triggered for nacked message",
		"message_id", pending.MessageID,
		"retry_attempt", retryCount,
		"max_attempts", maxRetries)

	// Check if we've exhausted retries
	if retryCount > maxRetries {
		p.client.config.Logger.Warn("Message nacked after max retry attempts",
			"message_id", pending.MessageID,
			"total_attempts", retryCount)

		// Copy data needed for final callback
		callback := pending.Callback
		messageID := pending.MessageID
		deliveryTag := pending.DeliveryTag

		// Release the per-message lock
		pending.mu.Unlock()

		// Invoke final callback and cleanup
		if callback != nil {
			callback(messageID, DeliveryNacked,
				fmt.Sprintf("message nacked after %d retry attempts", retryCount))
		}

		// Remove from pending messages
		p.pendingMessages.Delete(messageID, deliveryTag)
		pendingCount := p.pendingMessages.Count()

		// Update stats
		p.statsMutex.Lock()
		p.stats.PendingMessages = pendingCount
		p.statsMutex.Unlock()

		return
	}

	// We have retries left - prepare to re-publish
	// Copy all data needed for re-publishing while holding the lock
	originalMessage := pending.OriginalMessage
	originalCtx := pending.OriginalContext
	exchange := pending.Exchange
	routingKey := pending.RoutingKey
	options := pending.RetryOptions
	messageID := pending.MessageID
	oldDeliveryTag := pending.DeliveryTag

	// Release the per-message lock before sleeping and re-publishing
	pending.mu.Unlock()

	// Wait for the configured retry delay
	p.client.config.Logger.Debug("Waiting before retry attempt",
		"message_id", messageID,
		"delay", p.config.retryDelay)
	time.Sleep(p.config.retryDelay)

	// Clean up the old pending entry before re-publishing
	// The re-publish will create a new entry with a new delivery tag
	p.pendingMessages.Delete(messageID, oldDeliveryTag)

	// Re-publish the message using the original context to preserve trace propagation
	// This creates a new pending entry with a new delivery tag
	ctx := originalCtx
	if ctx == nil {
		ctx = context.Background() // Fallback if no context was stored
	}
	err := p.PublishWithDeliveryAssurance(ctx, exchange, routingKey, originalMessage, options)

	if err != nil {
		p.client.config.Logger.Error("Failed to re-publish message for retry",
			"message_id", messageID,
			"error", err,
			"retry_attempt", retryCount)

		// Invoke callback with error
		if options.Callback != nil {
			options.Callback(messageID, DeliveryFailed,
				fmt.Sprintf("retry failed: %v", err))
		}
	} else {
		p.client.config.Logger.Info("Message re-published for retry",
			"message_id", messageID,
			"retry_attempt", retryCount)
	}
}

// handleMandatoryGracePeriodExpired is called when the grace period for a mandatory message expires
// This means the message was confirmed but no return was received, so it was successfully routed
func (p *Publisher) handleMandatoryGracePeriodExpired(deliveryTag uint64) {
	// Get the pending message using sharded map
	pending, exists := p.pendingMessages.LoadByDeliveryTag(deliveryTag)

	if !exists {
		return
	}

	p.client.config.Logger.Debug("Grace period expired for mandatory message",
		"message_id", pending.MessageID,
		"delivery_tag", deliveryTag)

	// Lock the message state (per-message lock)
	pending.mu.Lock()
	defer pending.mu.Unlock()

	// If callback already fired or message was returned, do nothing
	if pending.CallbackFired {
		p.client.config.Logger.Debug("Grace period expired but callback already fired",
			"message_id", pending.MessageID,
			"delivery_tag", deliveryTag,
			"final_outcome", pending.FinalOutcome)
		return
	}

	if pending.Returned {
		p.client.config.Logger.Debug("Grace period expired but message was returned",
			"message_id", pending.MessageID,
			"delivery_tag", deliveryTag)
		return
	}

	p.client.config.Logger.Info("Finalizing mandatory message as success (no return received)",
		"message_id", pending.MessageID,
		"delivery_tag", deliveryTag)

	// Finalize as success - message was confirmed and not returned
	pending.CallbackFired = true
	pending.FinalOutcome = DeliverySuccess
	pending.FinalError = ""

	// Calculate duration
	duration := time.Since(pending.PublishedAt)

	// Update statistics
	p.statsMutex.Lock()
	p.stats.TotalConfirmed++
	p.stats.LastConfirmation = time.Now()
	p.statsMutex.Unlock()

	// Record metrics
	p.client.config.Metrics.RecordDeliveryOutcome(DeliverySuccess, duration)

	// Copy all necessary data for the callback while still holding the per-message lock
	callback := pending.Callback
	messageID := pending.MessageID

	// Schedule the callback and cleanup to run in a separate goroutine
	go func() {
		// Invoke the callback (if provided)
		if callback != nil {
			callback(messageID, DeliverySuccess, "")
		}

		// Remove from pending messages after callback is done
		p.pendingMessages.Delete(messageID, deliveryTag)
		pendingCount := p.pendingMessages.Count()

		// Update pending count in stats
		p.statsMutex.Lock()
		p.stats.PendingMessages = pendingCount
		p.statsMutex.Unlock()
	}()

	// The deferred pending.mu.Unlock() will now execute correctly when the function returns
}

// handleTimeout is called when a message delivery times out
func (p *Publisher) handleTimeout(deliveryTag uint64) {
	// Get the pending message using sharded map
	pending, exists := p.pendingMessages.LoadByDeliveryTag(deliveryTag)

	if !exists {
		return
	}

	// Lock the message state (per-message lock)
	pending.mu.Lock()
	defer pending.mu.Unlock()

	// If callback already fired, do nothing
	if pending.CallbackFired {
		return
	}

	// Mark callback as fired
	pending.CallbackFired = true
	pending.FinalOutcome = DeliveryTimeout
	pending.FinalError = fmt.Sprintf("delivery confirmation timeout after %v", p.config.deliveryTimeout)

	// Calculate duration
	duration := time.Since(pending.PublishedAt)

	// Update statistics
	p.statsMutex.Lock()
	p.stats.TotalTimedOut++
	p.statsMutex.Unlock()

	// Record metrics
	p.client.config.Metrics.RecordDeliveryOutcome(DeliveryTimeout, duration)
	p.client.config.Metrics.RecordDeliveryTimeout(pending.MessageID)

	// Copy all necessary data for the callback while still holding the per-message lock
	callback := pending.Callback
	messageID := pending.MessageID
	errorMessage := pending.FinalError
	timeout := p.config.deliveryTimeout

	// Schedule the callback and cleanup to run in a separate goroutine
	go func() {
		// Invoke the callback (if provided)
		if callback != nil {
			callback(messageID, DeliveryTimeout, errorMessage)
		}

		p.client.config.Logger.Warn("Message delivery timed out",
			"message_id", messageID,
			"timeout", timeout)

		// Remove from pending messages after callback is done
		p.pendingMessages.Delete(messageID, deliveryTag)
		pendingCount := p.pendingMessages.Count()

		// Update pending count in stats
		p.statsMutex.Lock()
		p.stats.PendingMessages = pendingCount
		p.statsMutex.Unlock()
	}()

	// The deferred pending.mu.Unlock() will now execute correctly when the function returns
}

// getNextDeliveryTag returns the next delivery tag for tracking
func (p *Publisher) getNextDeliveryTag() uint64 {
	p.deliveryTagMutex.Lock()
	defer p.deliveryTagMutex.Unlock()
	tag := p.nextDeliveryTag
	p.nextDeliveryTag++
	return tag
}

// Delivery Assurance Public API

// GetDeliveryStats returns the current delivery assurance statistics.
// This method is thread-safe and can be called concurrently.
//
// Returns a copy of the current statistics including:
//   - Total messages published with delivery assurance
//   - Total confirmations, returns, nacks, and timeouts
//   - Current number of pending messages
//   - Timestamps of last confirmation, return, and nack
//
// Example:
//
//	stats := publisher.GetDeliveryStats()
//	fmt.Printf("Published: %d, Confirmed: %d, Pending: %d\n",
//	    stats.TotalPublished, stats.TotalConfirmed, stats.PendingMessages)
func (p *Publisher) GetDeliveryStats() DeliveryStats {
	p.statsMutex.RLock()
	defer p.statsMutex.RUnlock()

	// Return a copy of the stats
	return DeliveryStats{
		TotalPublished:   p.stats.TotalPublished,
		TotalConfirmed:   p.stats.TotalConfirmed,
		TotalReturned:    p.stats.TotalReturned,
		TotalNacked:      p.stats.TotalNacked,
		TotalTimedOut:    p.stats.TotalTimedOut,
		PendingMessages:  p.stats.PendingMessages,
		LastConfirmation: p.stats.LastConfirmation,
		LastReturn:       p.stats.LastReturn,
		LastNack:         p.stats.LastNack,
	}
}

// SetDefaultDeliveryCallback sets or updates the default delivery callback.
// This callback will be used for all messages published with PublishWithDeliveryAssurance
// that don't specify their own callback in DeliveryOptions.
//
// This method is thread-safe and can be called while the publisher is in use.
//
// Example:
//
//	publisher.SetDefaultDeliveryCallback(func(messageID string, outcome DeliveryOutcome, errorMessage string) {
//	    log.Printf("Message %s: %s - %s", messageID, outcome, errorMessage)
//	})
func (p *Publisher) SetDefaultDeliveryCallback(callback DeliveryCallback) {
	p.config.defaultDeliveryCallback = callback
}

// IsDeliveryAssuranceEnabled returns true if delivery assurance is enabled for this publisher.
// This can be useful for conditional logic, monitoring, or debugging purposes.
//
// Example:
//
//	if publisher.IsDeliveryAssuranceEnabled() {
//	    stats := publisher.GetDeliveryStats()
//	    log.Printf("Delivery stats: %+v", stats)
//	}
func (p *Publisher) IsDeliveryAssuranceEnabled() bool {
	return p.deliveryAssuranceEnabled
}
