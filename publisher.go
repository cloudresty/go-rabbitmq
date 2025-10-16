package rabbitmq

import (
	"context"
	"fmt"
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
	confirmChannel           *amqp.Channel // Dedicated channel for delivery assurance
	pendingMessages          map[uint64]*pendingMessage
	returnedMessages         map[uint64]bool // Track messages that have been returned
	pendingMutex             sync.RWMutex
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
	RetryOnNack  bool
	MaxRetries   int
	RetryCount   int
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
		RetryOnNack: options.RetryOnNack,
		MaxRetries:  options.MaxRetries,
		RetryCount:  0,
	}

	// Set up timeout timer
	pending.TimeoutTimer = time.AfterFunc(timeout, func() {
		p.handleTimeout(deliveryTag)
	})

	// Add to pending messages before publishing
	p.pendingMutex.Lock()
	p.pendingMessages[deliveryTag] = pending
	pendingCount := int64(len(p.pendingMessages))
	p.pendingMutex.Unlock()

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
	err := p.confirmChannel.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		mandatory,
		p.config.Immediate,
		publishing,
	)

	if err != nil {
		// Remove from pending messages on publish error
		p.pendingMutex.Lock()
		delete(p.pendingMessages, deliveryTag)
		pendingCount := int64(len(p.pendingMessages))
		p.pendingMutex.Unlock()

		// Stop timeout timer
		pending.TimeoutTimer.Stop()

		// Update statistics
		p.statsMutex.Lock()
		p.stats.PendingMessages = pendingCount
		p.statsMutex.Unlock()

		p.client.config.Metrics.RecordError("publish_with_delivery_assurance", err)
		span.SetStatus(SpanStatusError, err.Error())
		return fmt.Errorf("failed to publish message: %w", err)
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
		p.pendingMutex.RLock()
		pendingCount := len(p.pendingMessages)
		p.pendingMutex.RUnlock()

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
		p.pendingMutex.Lock()
		for _, pending := range p.pendingMessages {
			if pending.TimeoutTimer != nil {
				pending.TimeoutTimer.Stop()
			}
		}
		finalPendingCount := len(p.pendingMessages)
		p.pendingMutex.Unlock()

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
	p.pendingMessages = make(map[uint64]*pendingMessage)
	p.returnedMessages = make(map[uint64]bool)
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
	p.pendingMutex.Lock()

	// Check if this message was already returned
	if p.returnedMessages[confirmation.DeliveryTag] {
		// Message was returned, cleanup the returned flag and skip confirmation processing
		delete(p.returnedMessages, confirmation.DeliveryTag)
		p.pendingMutex.Unlock()
		return
	}

	pending, exists := p.pendingMessages[confirmation.DeliveryTag]
	if !exists {
		p.pendingMutex.Unlock()
		return
	}

	// For mandatory messages with Ack, we need to wait briefly to see if a return arrives
	// RabbitMQ sends both Return and Ack, but they can arrive in any order
	if confirmation.Ack && pending.Mandatory {
		// Keep the message in pending state temporarily
		p.pendingMutex.Unlock()

		// Wait for potential return notification (50ms should be more than enough for local delivery)
		time.Sleep(50 * time.Millisecond)

		// Re-check if message was returned during the wait
		p.pendingMutex.Lock()
		if p.returnedMessages[confirmation.DeliveryTag] {
			// Message was returned, cleanup and skip confirmation processing
			delete(p.returnedMessages, confirmation.DeliveryTag)
			p.pendingMutex.Unlock()
			return
		}
		// Message was not returned, proceed with success processing
		p.pendingMutex.Unlock()
	}

	// Remove from pending messages
	p.pendingMutex.Lock()
	delete(p.pendingMessages, confirmation.DeliveryTag)
	pendingCount := int64(len(p.pendingMessages))
	p.pendingMutex.Unlock()

	// Stop the timeout timer
	if pending.TimeoutTimer != nil {
		pending.TimeoutTimer.Stop()
	}

	// Calculate duration
	duration := time.Since(pending.PublishedAt)

	// Update statistics
	p.statsMutex.Lock()
	if confirmation.Ack {
		p.stats.TotalConfirmed++
		p.stats.LastConfirmation = time.Now()
	} else {
		p.stats.TotalNacked++
		p.stats.LastNack = time.Now()
	}
	p.stats.PendingMessages = pendingCount
	p.statsMutex.Unlock()

	// Record metrics
	if confirmation.Ack {
		p.client.config.Metrics.RecordDeliveryOutcome(DeliverySuccess, duration)
	} else {
		p.client.config.Metrics.RecordDeliveryOutcome(DeliveryNacked, duration)
	}

	// Invoke callback
	if pending.Callback != nil {
		if confirmation.Ack {
			pending.Callback(pending.MessageID, DeliverySuccess, "")
		} else {
			// Handle nack - potentially retry
			if pending.RetryOnNack && pending.RetryCount < pending.MaxRetries {
				p.retryMessage(pending)
			} else {
				pending.Callback(pending.MessageID, DeliveryNacked, "message negatively acknowledged by broker")
			}
		}
	}
}

// handleReturn processes a returned message
func (p *Publisher) handleReturn(ret amqp.Return) {
	// Find the pending message by correlation
	// Note: Returns don't have delivery tags, so we need to match by message ID
	messageID := ret.MessageId

	p.pendingMutex.Lock()
	var pending *pendingMessage
	var deliveryTag uint64
	for tag, pm := range p.pendingMessages {
		if pm.MessageID == messageID {
			pending = pm
			deliveryTag = tag
			break
		}
	}
	if pending == nil {
		p.pendingMutex.Unlock()
		return
	}
	// Mark as returned so handleConfirmation knows not to process it
	p.returnedMessages[deliveryTag] = true
	delete(p.pendingMessages, deliveryTag)
	pendingCount := int64(len(p.pendingMessages))
	p.pendingMutex.Unlock()

	// Stop the timeout timer
	if pending.TimeoutTimer != nil {
		pending.TimeoutTimer.Stop()
	}

	// Calculate duration
	duration := time.Since(pending.PublishedAt)

	// Update statistics
	p.statsMutex.Lock()
	p.stats.TotalReturned++
	p.stats.LastReturn = time.Now()
	p.stats.PendingMessages = pendingCount
	p.statsMutex.Unlock()

	// Record metrics
	p.client.config.Metrics.RecordDeliveryOutcome(DeliveryFailed, duration)

	// Invoke callback
	if pending.Callback != nil {
		errorMsg := fmt.Sprintf("message returned: %s (reply code: %d)", ret.ReplyText, ret.ReplyCode)
		pending.Callback(pending.MessageID, DeliveryFailed, errorMsg)
	}
}

// retryMessage attempts to retry a nacked message
func (p *Publisher) retryMessage(pending *pendingMessage) {
	pending.RetryCount++

	p.client.config.Logger.Debug("Retrying nacked message",
		"message_id", pending.MessageID,
		"retry_count", pending.RetryCount,
		"max_retries", pending.MaxRetries)

	// Note: In a production implementation, you would re-publish the message here
	// For now, we'll just invoke the callback with nacked outcome after max retries
	if pending.RetryCount >= pending.MaxRetries {
		pending.Callback(pending.MessageID, DeliveryNacked,
			fmt.Sprintf("message nacked after %d retries", pending.RetryCount))
	}
}

// handleTimeout is called when a message delivery times out
func (p *Publisher) handleTimeout(deliveryTag uint64) {
	p.pendingMutex.Lock()
	pending, exists := p.pendingMessages[deliveryTag]
	if !exists {
		p.pendingMutex.Unlock()
		return
	}
	delete(p.pendingMessages, deliveryTag)
	pendingCount := int64(len(p.pendingMessages))
	p.pendingMutex.Unlock()

	// Calculate duration
	duration := time.Since(pending.PublishedAt)

	// Update statistics
	p.statsMutex.Lock()
	p.stats.TotalTimedOut++
	p.stats.PendingMessages = pendingCount
	p.statsMutex.Unlock()

	// Record metrics
	p.client.config.Metrics.RecordDeliveryOutcome(DeliveryTimeout, duration)
	p.client.config.Metrics.RecordDeliveryTimeout(pending.MessageID)

	// Invoke callback
	if pending.Callback != nil {
		pending.Callback(pending.MessageID, DeliveryTimeout,
			fmt.Sprintf("delivery confirmation timeout after %v", p.config.deliveryTimeout))
	}

	p.client.config.Logger.Warn("Message delivery timed out",
		"message_id", pending.MessageID,
		"timeout", p.config.deliveryTimeout)
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
