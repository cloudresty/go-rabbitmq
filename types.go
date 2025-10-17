package rabbitmq

import (
	"encoding/json"
	"fmt"
	"maps"
	"time"

	"github.com/cloudresty/ulid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// ContentType constants
const (
	ContentTypeJSON = "application/json"
	ContentTypeText = "text/plain"
)

// ExchangeType represents different types of exchanges
type ExchangeType string

const (
	ExchangeTypeDirect  ExchangeType = "direct"
	ExchangeTypeFanout  ExchangeType = "fanout"
	ExchangeTypeTopic   ExchangeType = "topic"
	ExchangeTypeHeaders ExchangeType = "headers"
)

// ExchangeConfig holds configuration for declaring an exchange
type ExchangeConfig struct {
	Name       string
	Type       ExchangeType
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Arguments  map[string]any
}

// QueueType represents the type of queue
type QueueType string

const (
	QueueTypeClassic QueueType = "classic"
	QueueTypeQuorum  QueueType = "quorum"
	QueueTypeStream  QueueType = "stream"
)

// QueueConfig holds configuration for declaring a queue
type QueueConfig struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Arguments  map[string]any
	// Cluster-aware settings
	QueueType            QueueType // Queue type: classic, quorum, stream
	HighAvailability     bool      // Enable HA for classic queues
	ReplicationFactor    int       // Replication factor for quorum queues (default: 3)
	MaxLength            int       // Maximum queue length (0 = unlimited)
	MaxLengthBytes       int       // Maximum queue size in bytes (0 = unlimited)
	MessageTTL           int       // Message TTL in milliseconds (0 = no TTL)
	DeadLetterExchange   string    // Dead letter exchange name
	DeadLetterRoutingKey string    // Dead letter routing key
}

// DefaultQuorumQueueConfig returns a production-ready quorum queue configuration
func DefaultQuorumQueueConfig(name string) QueueConfig {

	return QueueConfig{
		Name:              name,
		Durable:           true,
		AutoDelete:        false,
		Exclusive:         false,
		NoWait:            false,
		QueueType:         QueueTypeQuorum,
		HighAvailability:  false, // Not needed for quorum queues
		ReplicationFactor: 3,     // Default quorum size
		MaxLength:         0,     // Unlimited
		MaxLengthBytes:    0,     // Unlimited
		MessageTTL:        0,     // No TTL
		Arguments:         make(map[string]any),
	}

}

// DefaultHAQueueConfig returns a production-ready HA classic queue configuration
func DefaultHAQueueConfig(name string) QueueConfig {

	return QueueConfig{
		Name:              name,
		Durable:           true,
		AutoDelete:        false,
		Exclusive:         false,
		NoWait:            false,
		QueueType:         QueueTypeClassic,
		HighAvailability:  true,
		ReplicationFactor: 0, // Not applicable for classic HA queues
		MaxLength:         0, // Unlimited
		MaxLengthBytes:    0, // Unlimited
		MessageTTL:        0, // No TTL
		Arguments:         make(map[string]any),
	}

}

// DefaultClassicQueueConfig returns a basic durable classic queue configuration
func DefaultClassicQueueConfig(name string) QueueConfig {

	return QueueConfig{
		Name:              name,
		Durable:           true,
		AutoDelete:        false,
		Exclusive:         false,
		NoWait:            false,
		QueueType:         QueueTypeClassic,
		HighAvailability:  false,
		ReplicationFactor: 0,
		MaxLength:         0,
		MaxLengthBytes:    0,
		MessageTTL:        0,
		Arguments:         make(map[string]any),
	}

}

// ToArguments converts the QueueConfig to RabbitMQ queue arguments
func (q *QueueConfig) ToArguments() map[string]any {

	args := make(map[string]any)

	// Copy existing arguments
	maps.Copy(args, q.Arguments)

	// Set queue type
	switch q.QueueType {
	case QueueTypeQuorum:
		args["x-queue-type"] = "quorum"
		if q.ReplicationFactor > 0 {
			args["x-quorum-initial-group-size"] = q.ReplicationFactor
		}
	case QueueTypeStream:
		args["x-queue-type"] = "stream"
	case QueueTypeClassic:
		if q.HighAvailability {
			args["x-ha-policy"] = "all"
			args["x-ha-sync-mode"] = "automatic"
		}
	}

	// Set length limits
	if q.MaxLength > 0 {
		args["x-max-length"] = q.MaxLength
	}
	if q.MaxLengthBytes > 0 {
		args["x-max-length-bytes"] = q.MaxLengthBytes
	}

	// Set message TTL
	if q.MessageTTL > 0 {
		args["x-message-ttl"] = q.MessageTTL
	}

	// Set dead letter exchange
	if q.DeadLetterExchange != "" {
		args["x-dead-letter-exchange"] = q.DeadLetterExchange
		if q.DeadLetterRoutingKey != "" {
			args["x-dead-letter-routing-key"] = q.DeadLetterRoutingKey
		}
	}

	return args

}

// generateMessageID creates a unique message ID using ULID
func generateMessageID() string {

	// Generate a new ULID which provides:
	// - Temporal ordering (time-sortable)
	// - Global uniqueness
	// - Compact representation (26 characters)
	// - URL-safe characters
	ulidStr, err := ulid.New()
	if err != nil {
		// Fallback to timestamp-based ID if ULID generation fails
		timestamp := time.Now().UnixNano()
		return fmt.Sprintf("msg-%d", timestamp)
	}
	return ulidStr

}

// Message represents a message with metadata
type Message struct {
	Body        []byte
	ContentType string
	Headers     map[string]any
	Exchange    string
	RoutingKey  string
	Persistent  bool
	// Message identification and tracing
	MessageID     string // Unique message identifier (auto-generated if empty)
	CorrelationID string // Correlation ID for request-response patterns
	ReplyTo       string // Reply queue for RPC patterns
	// Message metadata
	Type   string // Message type/schema identifier
	AppID  string // Application ID that originated the message
	UserID string // User ID (if authenticated)
	// Timing and expiration
	Timestamp  int64  // Unix timestamp when message was created
	Expiration string // Message expiration (in milliseconds as string)
	// Message priority (0-255, higher = more priority)
	Priority uint8
}

// NewMessage creates a new Message with auto-generated ID and timestamp
func NewMessage(body []byte) *Message {

	return &Message{
		Body:        body,
		ContentType: ContentTypeJSON,
		Persistent:  true,
		MessageID:   generateMessageID(),
		Timestamp:   time.Now().Unix(),
		Headers:     make(map[string]any),
	}

}

// NewMessageWithID creates a new Message with a specific ID
func NewMessageWithID(body []byte, messageID string) *Message {

	return &Message{
		Body:        body,
		ContentType: ContentTypeJSON,
		Persistent:  true,
		MessageID:   messageID,
		Timestamp:   time.Now().Unix(),
		Headers:     make(map[string]any),
	}

}

// NewJSONMessage creates a new Message for JSON content by marshaling the provided value
func NewJSONMessage(v interface{}) (*Message, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	msg := NewMessage(body)
	msg.ContentType = ContentTypeJSON
	return msg, nil
}

// NewTextMessage creates a new Message for plain text content
func NewTextMessage(body []byte) *Message {

	msg := NewMessage(body)
	msg.ContentType = ContentTypeText
	return msg

}

// WithCorrelationID sets the correlation ID for request-response patterns
func (m *Message) WithCorrelationID(correlationID string) *Message {

	m.CorrelationID = correlationID
	return m

}

// WithReplyTo sets the reply queue for RPC patterns
func (m *Message) WithReplyTo(replyTo string) *Message {

	m.ReplyTo = replyTo
	return m

}

// WithType sets the message type/schema identifier
func (m *Message) WithType(messageType string) *Message {

	m.Type = messageType
	return m

}

// WithAppID sets the application ID
func (m *Message) WithAppID(appID string) *Message {

	m.AppID = appID
	return m

}

// WithUserID sets the user ID (if authenticated)
func (m *Message) WithUserID(userID string) *Message {

	m.UserID = userID
	return m

}

// WithExpiration sets message expiration in duration
func (m *Message) WithExpiration(expiration time.Duration) *Message {

	m.Expiration = fmt.Sprintf("%.0f", expiration.Seconds()*1000)
	return m

}

// WithPriority sets message priority (0-255, higher = more priority)
func (m *Message) WithPriority(priority uint8) *Message {

	m.Priority = priority
	return m

}

// WithHeader adds a custom header to the message
func (m *Message) WithHeader(key string, value any) *Message {

	if m.Headers == nil {
		m.Headers = make(map[string]any)
	}
	m.Headers[key] = value
	return m

}

// WithHeaders adds multiple custom headers to the message
func (m *Message) WithHeaders(headers map[string]any) *Message {

	if m.Headers == nil {
		m.Headers = make(map[string]any)
	}
	maps.Copy(m.Headers, headers)
	return m

}

// WithContentType sets the content type for the message
func (m *Message) WithContentType(contentType string) *Message {
	m.ContentType = contentType
	return m
}

// WithMessageID sets the message ID
func (m *Message) WithMessageID(id string) *Message {
	m.MessageID = id
	return m
}

// WithTimestamp sets the message timestamp
func (m *Message) WithTimestamp(t time.Time) *Message {
	m.Timestamp = t.Unix()
	return m
}

// WithPersistent sets the message to be persistent
func (m *Message) WithPersistent() *Message {
	m.Persistent = true
	return m
}

// WithTransient sets the message to be transient (non-persistent)
func (m *Message) WithTransient() *Message {
	m.Persistent = false
	return m
}

// Additional helper methods for the new unified API

// ToAMQPPublishing converts the message to amqp.Publishing for the new API
func (m *Message) ToAMQPPublishing() amqp.Publishing {
	deliveryMode := uint8(1) // Transient
	if m.Persistent {
		deliveryMode = 2 // Persistent
	}

	return amqp.Publishing{
		Headers:       amqp.Table(m.Headers),
		ContentType:   m.ContentType,
		Body:          m.Body,
		MessageId:     m.MessageID,
		CorrelationId: m.CorrelationID,
		ReplyTo:       m.ReplyTo,
		Expiration:    m.Expiration,
		Timestamp:     time.Unix(m.Timestamp, 0),
		Type:          m.Type,
		UserId:        m.UserID,
		AppId:         m.AppID,
		Priority:      m.Priority,
		DeliveryMode:  deliveryMode,
	}
}

// FromAMQPDelivery creates a Message from amqp.Delivery (for consumer API)
func FromAMQPDelivery(delivery amqp.Delivery) *Message {
	return &Message{
		Body:          delivery.Body,
		Headers:       map[string]any(delivery.Headers),
		ContentType:   delivery.ContentType,
		MessageID:     delivery.MessageId,
		CorrelationID: delivery.CorrelationId,
		ReplyTo:       delivery.ReplyTo,
		Expiration:    delivery.Expiration,
		Timestamp:     delivery.Timestamp.Unix(),
		Type:          delivery.Type,
		UserID:        delivery.UserId,
		AppID:         delivery.AppId,
		Priority:      delivery.Priority,
		Persistent:    delivery.DeliveryMode == 2,
	}
}

// BindingConfig holds configuration for binding a queue to an exchange
type BindingConfig struct {
	QueueName    string
	ExchangeName string
	RoutingKey   string
	NoWait       bool
	Arguments    map[string]any
}

// ToPublishing converts a Message to amqp.Publishing
func (m *Message) ToPublishing() amqp.Publishing {

	deliveryMode := uint8(1) // Non-persistent
	if m.Persistent {
		deliveryMode = uint8(2) // Persistent
	}

	contentType := m.ContentType
	if contentType == "" {
		contentType = ContentTypeJSON
	}

	// Auto-generate MessageID if not provided
	messageID := m.MessageID
	if messageID == "" {
		messageID = generateMessageID()
	}

	return amqp.Publishing{
		ContentType:   contentType,
		Body:          m.Body,
		DeliveryMode:  deliveryMode,
		Headers:       m.Headers,
		MessageId:     messageID,
		CorrelationId: m.CorrelationID,
		ReplyTo:       m.ReplyTo,
		Type:          m.Type,
		AppId:         m.AppID,
		UserId:        m.UserID,
		Timestamp:     time.Unix(m.Timestamp, 0),
		Expiration:    m.Expiration,
		Priority:      m.Priority,
	}

}

// DeliveryInfo contains information about a delivered message
type DeliveryInfo struct {
	MessageCount uint32
	Exchange     string
	RoutingKey   string
	Redelivered  bool
	DeliveryTag  uint64
	// Message metadata from AMQP properties
	MessageID     string
	CorrelationID string
	ReplyTo       string
	Type          string
	AppID         string
	UserID        string
	Timestamp     time.Time
	ContentType   string
	Priority      uint8
	Headers       map[string]any
}

// ExtractDeliveryInfo extracts delivery information from an AMQP delivery
func ExtractDeliveryInfo(delivery *amqp.Delivery) DeliveryInfo {
	return DeliveryInfo{
		MessageCount:  delivery.MessageCount,
		Exchange:      delivery.Exchange,
		RoutingKey:    delivery.RoutingKey,
		Redelivered:   delivery.Redelivered,
		DeliveryTag:   delivery.DeliveryTag,
		MessageID:     delivery.MessageId,
		CorrelationID: delivery.CorrelationId,
		ReplyTo:       delivery.ReplyTo,
		Type:          delivery.Type,
		AppID:         delivery.AppId,
		UserID:        delivery.UserId,
		Timestamp:     delivery.Timestamp,
		ContentType:   delivery.ContentType,
		Priority:      delivery.Priority,
		Headers:       delivery.Headers,
	}
}

// Error types for better error handling
type Error struct {
	Type    string
	Message string
	Cause   error
}

func (e *Error) Error() string {

	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)

}

func (e *Error) Unwrap() error {

	return e.Cause

}

// NewConnectionError creates a new connection error
func NewConnectionError(message string, cause error) *Error {

	return &Error{
		Type:    "ConnectionError",
		Message: message,
		Cause:   cause,
	}

}

// NewPublishError creates a new publish error
func NewPublishError(message string, cause error) *Error {

	return &Error{
		Type:    "PublishError",
		Message: message,
		Cause:   cause,
	}

}

// NewConsumeError creates a new consume error
func NewConsumeError(message string, cause error) *Error {

	return &Error{
		Type:    "ConsumeError",
		Message: message,
		Cause:   cause,
	}

}

// WithoutDeadLetter clears any dead letter configuration
func (q *QueueConfig) WithoutDeadLetter() *QueueConfig {
	q.DeadLetterExchange = ""
	q.DeadLetterRoutingKey = ""
	return q
}

// WithCustomDeadLetter configures a custom dead letter exchange
func (q *QueueConfig) WithCustomDeadLetter(dlxName, routingKey string) *QueueConfig {
	q.DeadLetterExchange = dlxName
	q.DeadLetterRoutingKey = routingKey
	return q
}

// Validate checks if the message is valid for publishing
func (m *Message) Validate() error {
	if len(m.Body) == 0 {
		return fmt.Errorf("message body cannot be empty")
	}

	if m.ContentType == "" {
		return fmt.Errorf("message content type cannot be empty")
	}

	// Note: Priority validation is not needed since uint8 enforces the range 0-255

	// Validate expiration format if set
	if m.Expiration != "" {
		// Expiration should be a numeric string representing milliseconds
		if _, err := time.ParseDuration(m.Expiration + "ms"); err != nil {
			return fmt.Errorf("invalid expiration format: %s", m.Expiration)
		}
	}

	return nil
}

// Clone creates a deep copy of the message
func (m *Message) Clone() *Message {
	clone := &Message{
		Body:          make([]byte, len(m.Body)),
		ContentType:   m.ContentType,
		Exchange:      m.Exchange,
		RoutingKey:    m.RoutingKey,
		Persistent:    m.Persistent,
		MessageID:     m.MessageID,
		CorrelationID: m.CorrelationID,
		ReplyTo:       m.ReplyTo,
		Type:          m.Type,
		AppID:         m.AppID,
		UserID:        m.UserID,
		Timestamp:     m.Timestamp,
		Expiration:    m.Expiration,
		Priority:      m.Priority,
		Headers:       make(map[string]any),
	}

	// Copy body
	copy(clone.Body, m.Body)

	// Deep copy headers
	if m.Headers != nil {
		maps.Copy(clone.Headers, m.Headers)
	}

	return clone
}

// Delivery Assurance Types

// DeliveryOutcome represents the possible outcomes of message delivery
type DeliveryOutcome string

const (
	// DeliverySuccess indicates the message was confirmed by the broker and successfully routed
	DeliverySuccess DeliveryOutcome = "success"

	// DeliveryFailed indicates the message was returned by the broker (no queue bound to routing key)
	DeliveryFailed DeliveryOutcome = "failed"

	// DeliveryNacked indicates the message was negatively acknowledged by the broker
	DeliveryNacked DeliveryOutcome = "nacked"

	// DeliveryTimeout indicates the delivery confirmation timed out
	DeliveryTimeout DeliveryOutcome = "timeout"
)

// DeliveryCallback defines the callback function signature for delivery assurance outcomes.
// The callback is invoked asynchronously when a delivery outcome is determined.
//
// Parameters:
//   - messageID: The unique identifier of the message (from Message.MessageID)
//   - outcome: The delivery outcome (success, failed, nacked, or timeout)
//   - errorMessage: Additional error details (empty for successful deliveries)
type DeliveryCallback func(messageID string, outcome DeliveryOutcome, errorMessage string)

// DeliveryOptions configures delivery assurance behavior for individual messages
type DeliveryOptions struct {
	// MessageID is the unique identifier for tracking this message.
	// If empty, the Message.MessageID will be used.
	MessageID string

	// Mandatory indicates whether the message should be returned if it cannot be routed.
	// When true, the broker will return the message if no queue is bound to the routing key.
	Mandatory bool

	// Callback is the delivery outcome callback for this specific message.
	// If nil, the publisher's default callback will be used (if configured).
	Callback DeliveryCallback

	// Timeout specifies how long to wait for delivery confirmation.
	// If zero, the publisher's default delivery timeout will be used.
	Timeout time.Duration

	// RetryOnNack indicates whether to automatically retry when the broker nacks the message.
	// This is typically used for transient broker issues.
	//
	// Note: Current implementation does not store the message body, so retries are not
	// fully functional. The callback will be invoked with a nack outcome. This will be
	// improved in a future version to support actual re-publishing.
	RetryOnNack bool

	// MaxRetries specifies the maximum number of retry attempts when RetryOnNack is true.
	// If zero, no retries will be attempted.
	MaxRetries int
}

// DeliveryStats provides metrics and statistics about delivery assurance operations
type DeliveryStats struct {
	// TotalPublished is the total number of messages published with delivery assurance
	TotalPublished int64

	// TotalConfirmed is the total number of messages confirmed by the broker
	TotalConfirmed int64

	// TotalReturned is the total number of messages returned by the broker (routing failures)
	TotalReturned int64

	// TotalNacked is the total number of messages negatively acknowledged by the broker
	TotalNacked int64

	// TotalTimedOut is the total number of messages that timed out waiting for confirmation
	TotalTimedOut int64

	// PendingMessages is the current number of messages awaiting confirmation
	PendingMessages int64

	// LastConfirmation is the timestamp of the most recent confirmation received
	LastConfirmation time.Time

	// LastReturn is the timestamp of the most recent message return
	LastReturn time.Time

	// LastNack is the timestamp of the most recent nack
	LastNack time.Time
}
