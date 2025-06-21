package rabbitmq

import (
	"fmt"
	"maps"
	"time"

	"github.com/cloudresty/emit"
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
	// Dead Letter Infrastructure (NEW)
	AutoCreateDLX bool   // Automatically create dead letter exchange and queue (default: true)
	DLXSuffix     string // Suffix for DLX name (default: ".dlx")
	DLQSuffix     string // Suffix for DLQ name (default: ".dlq")
	DLQMaxLength  int    // Max length for dead letter queue (0 = unlimited)
	DLQMessageTTL int    // TTL for messages in DLQ in milliseconds (0 = no TTL, default: 7 days)
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
		// Dead Letter Infrastructure (enabled by default)
		AutoCreateDLX: true,
		DLXSuffix:     ".dlx",
		DLQSuffix:     ".dlq",
		DLQMaxLength:  0,                       // Unlimited
		DLQMessageTTL: 7 * 24 * 60 * 60 * 1000, // 7 days in milliseconds
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
		// Dead Letter Infrastructure (enabled by default)
		AutoCreateDLX: true,
		DLXSuffix:     ".dlx",
		DLQSuffix:     ".dlq",
		DLQMaxLength:  0,                       // Unlimited
		DLQMessageTTL: 7 * 24 * 60 * 60 * 1000, // 7 days in milliseconds
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
		// Dead Letter Infrastructure (enabled by default)
		AutoCreateDLX: true,
		DLXSuffix:     ".dlx",
		DLQSuffix:     ".dlq",
		DLQMaxLength:  0,                       // Unlimited
		DLQMessageTTL: 7 * 24 * 60 * 60 * 1000, // 7 days in milliseconds
	}
}

// ToArguments converts the QueueConfig to RabbitMQ queue arguments
func (q *QueueConfig) ToArguments() map[string]any {
	args := make(map[string]any)

	// Copy existing arguments
	for k, v := range q.Arguments {
		args[k] = v
	}

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
	} else if q.AutoCreateDLX {
		// Auto-configure dead letter exchange if enabled
		dlxName := q.Name + q.DLXSuffix
		args["x-dead-letter-exchange"] = dlxName
		if q.DeadLetterRoutingKey != "" {
			args["x-dead-letter-routing-key"] = q.DeadLetterRoutingKey
		} else {
			// Use the DLQ name as routing key for direct exchange
			args["x-dead-letter-routing-key"] = q.Name + q.DLQSuffix
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
		emit.Warn.StructuredFields("ULID generation failed, using timestamp fallback",
			emit.ZString("error", err.Error()),
			emit.ZInt64("timestamp", timestamp))
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

// NewJSONMessage creates a new Message for JSON content
func NewJSONMessage(body []byte) *Message {
	msg := NewMessage(body)
	msg.ContentType = ContentTypeJSON
	return msg
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

// BindingConfig holds configuration for binding a queue to an exchange
type BindingConfig struct {
	QueueName    string
	ExchangeName string
	RoutingKey   string
	NoWait       bool
	Arguments    map[string]any
}

// SetupTopology creates exchanges, queues, and bindings
func SetupTopology(conn *Connection, exchanges []ExchangeConfig, queues []QueueConfig, bindings []BindingConfig) error {
	if !conn.IsConnected() {
		return fmt.Errorf("connection is not established")
	}

	channel := conn.Channel()

	// Step 1: Declare main exchanges
	if err := declareExchanges(channel, exchanges); err != nil {
		return err
	}

	// Step 2: Declare dead letter exchanges for queues that need them
	if err := declareDeadLetterExchanges(channel, queues); err != nil {
		return err
	}

	// Step 3: Declare dead letter queues
	if err := declareDeadLetterQueues(channel, queues); err != nil {
		return err
	}

	// Step 4: Declare main queues
	if err := declareMainQueues(channel, queues); err != nil {
		return err
	}

	// Step 5: Create bindings
	if err := createBindings(channel, bindings); err != nil {
		return err
	}

	return nil
}

// declareExchanges declares all the main exchanges
func declareExchanges(channel *amqp.Channel, exchanges []ExchangeConfig) error {
	for _, exchange := range exchanges {
		err := channel.ExchangeDeclare(
			exchange.Name,
			string(exchange.Type),
			exchange.Durable,
			exchange.AutoDelete,
			exchange.Internal,
			exchange.NoWait,
			exchange.Arguments,
		)
		if err != nil {
			return fmt.Errorf("failed to declare exchange %s: %w", exchange.Name, err)
		}

		emit.Info.StructuredFields("Exchange declared successfully",
			emit.ZString("exchange_name", exchange.Name),
			emit.ZString("exchange_type", string(exchange.Type)))
	}
	return nil
}

// declareDeadLetterExchanges declares dead letter exchanges for queues that need them
func declareDeadLetterExchanges(channel *amqp.Channel, queues []QueueConfig) error {
	dlxMap := make(map[string]bool) // Track created DLXs to avoid duplicates
	for _, queue := range queues {
		if queue.AutoCreateDLX {
			dlxConfig := queue.GetDLXConfig()
			dlxName := dlxConfig.Name

			if !dlxMap[dlxName] {
				err := channel.ExchangeDeclare(
					dlxConfig.Name,
					string(dlxConfig.Type),
					dlxConfig.Durable,
					dlxConfig.AutoDelete,
					dlxConfig.Internal,
					dlxConfig.NoWait,
					dlxConfig.Arguments,
				)
				if err != nil {
					return fmt.Errorf("failed to declare dead letter exchange %s: %w", dlxName, err)
				}

				emit.Info.StructuredFields("Dead letter exchange declared",
					emit.ZString("dlx_name", dlxName),
					emit.ZString("queue_name", queue.Name))

				dlxMap[dlxName] = true
			}
		}
	}
	return nil
}

// declareDeadLetterQueues declares dead letter queues
func declareDeadLetterQueues(channel *amqp.Channel, queues []QueueConfig) error {
	dlqMap := make(map[string]bool) // Track created DLQs to avoid duplicates
	for _, queue := range queues {
		if queue.AutoCreateDLX {
			dlqConfig := queue.GetDLQConfig()
			dlqName := dlqConfig.Name

			if !dlqMap[dlqName] {
				dlqArgs := dlqConfig.ToArguments()

				_, err := channel.QueueDeclare(
					dlqConfig.Name,
					dlqConfig.Durable,
					dlqConfig.AutoDelete,
					dlqConfig.Exclusive,
					dlqConfig.NoWait,
					dlqArgs,
				)
				if err != nil {
					return fmt.Errorf("failed to declare dead letter queue %s: %w", dlqName, err)
				}

				emit.Info.StructuredFields("Dead letter queue declared",
					emit.ZString("dlq_name", dlqName),
					emit.ZString("queue_name", queue.Name),
					emit.ZString("queue_type", string(dlqConfig.QueueType)))

				// Bind DLQ to DLX
				if err := bindDeadLetterQueue(channel, &queue, dlqName); err != nil {
					return err
				}

				dlqMap[dlqName] = true
			}
		}
	}
	return nil
}

// bindDeadLetterQueue binds a dead letter queue to its exchange
func bindDeadLetterQueue(channel *amqp.Channel, queue *QueueConfig, dlqName string) error {
	dlxName := queue.GetDLXName()
	routingKey := queue.Name + queue.DLQSuffix
	if queue.DeadLetterRoutingKey != "" {
		routingKey = queue.DeadLetterRoutingKey
	}

	err := channel.QueueBind(
		dlqName,    // queue name
		routingKey, // routing key
		dlxName,    // exchange name
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind dead letter queue %s to exchange %s: %w", dlqName, dlxName, err)
	}

	emit.Info.StructuredFields("Dead letter queue bound to exchange",
		emit.ZString("dlq_name", dlqName),
		emit.ZString("dlx_name", dlxName),
		emit.ZString("routing_key", routingKey))
	return nil
}

// declareMainQueues declares all the main queues
func declareMainQueues(channel *amqp.Channel, queues []QueueConfig) error {
	for _, queue := range queues {
		// Convert queue config to arguments (this will include DLX settings)
		args := queue.ToArguments()

		emit.Info.StructuredFields("Declaring queue",
			emit.ZString("queue_name", queue.Name),
			emit.ZString("queue_type", string(queue.QueueType)),
			emit.ZBool("durable", queue.Durable),
			emit.ZBool("high_availability", queue.HighAvailability),
			emit.ZBool("auto_create_dlx", queue.AutoCreateDLX),
			emit.ZInt("replication_factor", queue.ReplicationFactor))

		_, err := channel.QueueDeclare(
			queue.Name,
			queue.Durable,
			queue.AutoDelete,
			queue.Exclusive,
			queue.NoWait,
			args, // Use converted arguments (includes DLX settings)
		)
		if err != nil {
			emit.Error.StructuredFields("Failed to declare queue",
				emit.ZString("queue_name", queue.Name),
				emit.ZString("queue_type", string(queue.QueueType)),
				emit.ZString("error", err.Error()))
			return fmt.Errorf("failed to declare queue %s: %w", queue.Name, err)
		}

		emit.Info.StructuredFields("Queue declared successfully",
			emit.ZString("queue_name", queue.Name),
			emit.ZString("queue_type", string(queue.QueueType)),
			emit.ZString("dlx_name", queue.GetDLXName()))
	}
	return nil
}

// createBindings creates all the queue bindings
func createBindings(channel *amqp.Channel, bindings []BindingConfig) error {
	for _, binding := range bindings {
		err := channel.QueueBind(
			binding.QueueName,
			binding.RoutingKey,
			binding.ExchangeName,
			binding.NoWait,
			binding.Arguments,
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s: %w",
				binding.QueueName, binding.ExchangeName, err)
		}
	}
	return nil
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

// DeclareStandardTopology sets up a standard topology with work queues
func DeclareStandardTopology(conn *Connection, exchangeName, queueName string) error {
	exchanges := []ExchangeConfig{
		{
			Name:       exchangeName,
			Type:       ExchangeTypeDirect,
			Durable:    true,
			AutoDelete: false,
		},
	}

	queues := []QueueConfig{
		{
			Name:       queueName,
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
		},
	}

	bindings := []BindingConfig{
		{
			QueueName:    queueName,
			ExchangeName: exchangeName,
			RoutingKey:   queueName,
		},
	}

	return SetupTopology(conn, exchanges, queues, bindings)
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

// GetDLXName returns the dead letter exchange name for this queue config
func (q *QueueConfig) GetDLXName() string {
	if q.DeadLetterExchange != "" {
		return q.DeadLetterExchange
	}
	if q.AutoCreateDLX {
		return q.Name + q.DLXSuffix
	}
	return ""
}

// GetDLQName returns the dead letter queue name for this queue config
func (q *QueueConfig) GetDLQName() string {
	if q.AutoCreateDLX {
		return q.Name + q.DLQSuffix
	}
	return ""
}

// GetDLQConfig returns a QueueConfig for the dead letter queue
func (q *QueueConfig) GetDLQConfig() QueueConfig {
	if !q.AutoCreateDLX {
		return QueueConfig{} // Return empty config if DLX is disabled
	}

	dlqConfig := QueueConfig{
		Name:              q.GetDLQName(),
		Durable:           q.Durable, // Same durability as main queue
		AutoDelete:        false,     // Never auto-delete DLQs
		Exclusive:         false,     // DLQs should be accessible
		NoWait:            q.NoWait,
		QueueType:         q.QueueType, // Same type as main queue
		HighAvailability:  q.HighAvailability,
		ReplicationFactor: q.ReplicationFactor,
		MaxLength:         q.DLQMaxLength,
		MaxLengthBytes:    0, // No byte limit on DLQ
		MessageTTL:        q.DLQMessageTTL,
		Arguments:         make(map[string]any),
		// Disable auto-DLX for DLQ to prevent infinite loops
		AutoCreateDLX:      false,
		DeadLetterExchange: "", // No DLX for the DLQ itself
	}

	return dlqConfig
}

// GetDLXConfig returns an ExchangeConfig for the dead letter exchange
func (q *QueueConfig) GetDLXConfig() ExchangeConfig {
	if !q.AutoCreateDLX {
		return ExchangeConfig{} // Return empty config if DLX is disabled
	}

	return ExchangeConfig{
		Name:       q.GetDLXName(),
		Type:       ExchangeTypeDirect, // Use direct exchange for DLX
		Durable:    q.Durable,          // Same durability as main queue
		AutoDelete: false,              // Never auto-delete DLX
		Internal:   false,
		NoWait:     q.NoWait,
		Arguments:  make(map[string]any),
	}
}

// WithoutDeadLetter disables automatic dead letter infrastructure creation
func (q *QueueConfig) WithoutDeadLetter() *QueueConfig {
	q.AutoCreateDLX = false
	q.DeadLetterExchange = ""
	q.DeadLetterRoutingKey = ""
	return q
}

// WithDeadLetter enables and configures dead letter infrastructure
func (q *QueueConfig) WithDeadLetter(dlxSuffix, dlqSuffix string, dlqTTLDays int) *QueueConfig {
	q.AutoCreateDLX = true
	q.DLXSuffix = dlxSuffix
	q.DLQSuffix = dlqSuffix
	q.DLQMessageTTL = dlqTTLDays * 24 * 60 * 60 * 1000 // Convert days to milliseconds
	return q
}

// WithCustomDeadLetter configures a custom dead letter exchange (disables auto-creation)
func (q *QueueConfig) WithCustomDeadLetter(dlxName, routingKey string) *QueueConfig {
	q.AutoCreateDLX = false
	q.DeadLetterExchange = dlxName
	q.DeadLetterRoutingKey = routingKey
	return q
}
