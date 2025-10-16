package rabbitmq

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// ReconnectPolicy defines the interface for reconnection strategies
type ReconnectPolicy interface {
	ShouldRetry(attempt int, err error) bool
	NextDelay(attempt int) time.Duration
}

// ExponentialBackoff implements exponential backoff reconnection policy
type ExponentialBackoff struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	MaxAttempts  int
}

func (e *ExponentialBackoff) ShouldRetry(attempt int, err error) bool {
	if e.MaxAttempts > 0 && attempt >= e.MaxAttempts {
		return false
	}
	return true
}

func (e *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	delay := e.InitialDelay
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * e.Multiplier)
		if delay > e.MaxDelay {
			return e.MaxDelay
		}
	}
	return delay
}

// LinearBackoff implements linear backoff reconnection policy
type LinearBackoff struct {
	Delay       time.Duration
	MaxAttempts int
}

func (l *LinearBackoff) ShouldRetry(attempt int, err error) bool {
	if l.MaxAttempts > 0 && attempt >= l.MaxAttempts {
		return false
	}
	return true
}

func (l *LinearBackoff) NextDelay(attempt int) time.Duration {
	return l.Delay
}

// FixedDelay implements fixed delay reconnection policy
type FixedDelay struct {
	Delay       time.Duration
	MaxAttempts int
}

func (f *FixedDelay) ShouldRetry(attempt int, err error) bool {
	if f.MaxAttempts > 0 && attempt >= f.MaxAttempts {
		return false
	}
	return true
}

func (f *FixedDelay) NextDelay(attempt int) time.Duration {
	return f.Delay
}

// Logger interface for structured logging
// Users can implement this interface to integrate their preferred logging solution.
type Logger interface {
	// Debug logs a debug message with optional structured fields
	Debug(msg string, fields ...any)

	// Info logs an informational message with optional structured fields
	Info(msg string, fields ...any)

	// Warn logs a warning message with optional structured fields
	Warn(msg string, fields ...any)

	// Error logs an error message with optional structured fields
	Error(msg string, fields ...any)
}

// NopLogger is a no-operation logger that produces no output.
// This is used as the default logger when no logger is provided.
type NopLogger struct{}

// Debug implements Logger.Debug with no operation
func (n *NopLogger) Debug(msg string, fields ...any) {}

// Info implements Logger.Info with no operation
func (n *NopLogger) Info(msg string, fields ...any) {}

// Warn implements Logger.Warn with no operation
func (n *NopLogger) Warn(msg string, fields ...any) {}

// Error implements Logger.Error with no operation
func (n *NopLogger) Error(msg string, fields ...any) {}

// NewNopLogger creates a new no-operation logger
func NewNopLogger() Logger {
	return &NopLogger{}
}

// MetricsCollector interface for collecting metrics
type MetricsCollector interface {
	// Connection metrics
	RecordConnection(connectionName string)
	RecordConnectionAttempt(success bool, duration time.Duration)
	RecordReconnection(attempt int)

	// Publishing metrics
	RecordPublish(exchange, routingKey string, messageSize int, duration time.Duration)
	RecordPublishConfirmation(success bool, duration time.Duration)

	// Delivery assurance metrics
	RecordDeliveryOutcome(outcome DeliveryOutcome, duration time.Duration)
	RecordDeliveryTimeout(messageID string)

	// Consumption metrics
	RecordConsume(queue string, messageSize int, duration time.Duration)
	RecordMessageReceived(queue string)
	RecordMessageProcessed(queue string, success bool, duration time.Duration)
	RecordMessageRequeued(queue string)

	// Health and error metrics
	RecordHealthCheck(success bool, duration time.Duration)
	RecordError(operation string, err error)
}

// NopMetrics is a no-operation metrics collector
type NopMetrics struct{}

func (n *NopMetrics) RecordConnection(connectionName string)                       {}
func (n *NopMetrics) RecordConnectionAttempt(success bool, duration time.Duration) {}
func (n *NopMetrics) RecordReconnection(attempt int)                               {}
func (n *NopMetrics) RecordPublish(exchange, routingKey string, messageSize int, duration time.Duration) {
}
func (n *NopMetrics) RecordPublishConfirmation(success bool, duration time.Duration)        {}
func (n *NopMetrics) RecordDeliveryOutcome(outcome DeliveryOutcome, duration time.Duration) {}
func (n *NopMetrics) RecordDeliveryTimeout(messageID string)                                {}
func (n *NopMetrics) RecordConsume(queue string, messageSize int, duration time.Duration)   {}
func (n *NopMetrics) RecordMessageReceived(queue string)                                    {}
func (n *NopMetrics) RecordMessageProcessed(queue string, success bool, duration time.Duration) {
}
func (n *NopMetrics) RecordMessageRequeued(queue string)                     {}
func (n *NopMetrics) RecordHealthCheck(success bool, duration time.Duration) {}
func (n *NopMetrics) RecordError(operation string, err error)                {}

func NewNopMetrics() MetricsCollector {
	return &NopMetrics{}
}

// Tracer interface for distributed tracing
type Tracer interface {
	StartSpan(ctx context.Context, operation string) (context.Context, Span)
}

// Span interface for tracing spans
type Span interface {
	SetAttribute(key string, value any)
	SetStatus(code SpanStatusCode, description string)
	End()
}

// SpanStatusCode represents the status of a span
type SpanStatusCode int

const (
	SpanStatusUnset SpanStatusCode = iota
	SpanStatusOK
	SpanStatusError
)

// NopTracer is a no-operation tracer
type NopTracer struct{}

func (n *NopTracer) StartSpan(ctx context.Context, operation string) (context.Context, Span) {
	return ctx, &NopSpan{}
}

// NopSpan is a no-operation span
type NopSpan struct{}

func (n *NopSpan) SetAttribute(key string, value any)                {}
func (n *NopSpan) SetStatus(code SpanStatusCode, description string) {}
func (n *NopSpan) End()                                              {}

func NewNopTracer() Tracer {
	return &NopTracer{}
}

// AccessPolicy defines role-based access control
type AccessPolicy struct {
	AllowedExchanges []string
	AllowedQueues    []string
	AllowedRoutes    []string
	ReadOnly         bool
}

// AuditLogger interface for audit logging
type AuditLogger interface {
	LogConnection(clientID, username, host string)
	LogPublish(clientID, exchange, routingKey, messageID string)
	LogConsume(clientID, queue, messageID string)
	LogTopologyChange(clientID, operation, resource string)
}

// NopAuditLogger is a no-operation audit logger
type NopAuditLogger struct{}

func (n *NopAuditLogger) LogConnection(clientID, username, host string)               {}
func (n *NopAuditLogger) LogPublish(clientID, exchange, routingKey, messageID string) {}
func (n *NopAuditLogger) LogConsume(clientID, queue, messageID string)                {}
func (n *NopAuditLogger) LogTopologyChange(clientID, operation, resource string)      {}

func NewNopAuditLogger() AuditLogger {
	return &NopAuditLogger{}
}

// RetryPolicy defines retry behavior for operations
type RetryPolicy interface {
	ShouldRetry(attempt int, err error) bool
	NextDelay(attempt int) time.Duration
}

// RejectError allows controlling message rejection behavior
type RejectError struct {
	Requeue bool
	Cause   error
}

func (r *RejectError) Error() string {
	return r.Cause.Error()
}

func (r *RejectError) Unwrap() error {
	return r.Cause
}

// Concrete retry policy implementations

// NoRetryPolicy never retries operations
type NoRetryPolicy struct{}

func (n NoRetryPolicy) ShouldRetry(attempt int, err error) bool {
	return false
}

func (n NoRetryPolicy) NextDelay(attempt int) time.Duration {
	return 0
}

// NoRetry is a global instance of NoRetryPolicy
var NoRetry = NoRetryPolicy{}

// ExponentialBackoffPolicy implements exponential backoff retry
type ExponentialBackoffPolicy struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	MaxAttempts  int
	Multiplier   float64
}

func (e ExponentialBackoffPolicy) ShouldRetry(attempt int, err error) bool {
	if e.MaxAttempts > 0 && attempt >= e.MaxAttempts {
		return false
	}
	return true
}

func (e ExponentialBackoffPolicy) NextDelay(attempt int) time.Duration {
	if e.InitialDelay == 0 {
		e.InitialDelay = 1 * time.Second
	}
	if e.Multiplier == 0 {
		e.Multiplier = 2.0
	}

	delay := time.Duration(float64(e.InitialDelay) * math.Pow(e.Multiplier, float64(attempt)))

	if e.MaxDelay > 0 && delay > e.MaxDelay {
		delay = e.MaxDelay
	}

	return delay
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if rejectErr, ok := err.(*RejectError); ok {
		return rejectErr.Requeue
	}

	// Check for specific error types that should not be retried
	errStr := err.Error()

	// Authentication errors - should not retry
	if strings.Contains(errStr, "access refused") ||
		strings.Contains(errStr, "authentication failed") ||
		strings.Contains(errStr, "invalid credentials") {
		return false
	}

	// Authorization errors - should not retry
	if strings.Contains(errStr, "access denied") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "not allowed") {
		return false
	}

	// Resource not found errors - should not retry for missing exchanges/queues
	if strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "does not exist") {
		return false
	}

	// Channel errors that indicate misuse - should not retry
	if strings.Contains(errStr, "channel/connection is not open") ||
		strings.Contains(errStr, "channel already closed") ||
		strings.Contains(errStr, "precondition failed") {
		return false
	}

	// Message too large errors - should not retry
	if strings.Contains(errStr, "message too large") ||
		strings.Contains(errStr, "frame too large") ||
		strings.Contains(errStr, "content too large") {
		return false
	}

	// Connection errors are generally retryable (handled by IsConnectionError)
	if IsConnectionError(err) {
		return true
	}

	// For unknown errors, default to retryable to maintain backward compatibility
	return true
}

// IsConnectionError checks if an error is connection-related
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network-level connection errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "connection lost") ||
		strings.Contains(errStr, "connection timeout") ||
		strings.Contains(errStr, "connection failed") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "host is unreachable") ||
		strings.Contains(errStr, "no route to host") {
		return true
	}

	// DNS resolution errors
	if strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "name resolution failed") ||
		strings.Contains(errStr, "dns lookup failed") {
		return true
	}

	// Socket-level errors
	if strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection aborted") ||
		strings.Contains(errStr, "socket closed") ||
		strings.Contains(errStr, "socket shutdown") {
		return true
	}

	// TLS/SSL errors (often connection-related)
	if strings.Contains(errStr, "tls handshake") ||
		strings.Contains(errStr, "ssl handshake") ||
		strings.Contains(errStr, "certificate") ||
		strings.Contains(errStr, "x509") {
		return true
	}

	// AMQP connection-specific errors
	if strings.Contains(errStr, "connection blocked") ||
		strings.Contains(errStr, "connection.close") ||
		strings.Contains(errStr, "heartbeat timeout") ||
		strings.Contains(errStr, "unexpected frame") ||
		strings.Contains(errStr, "protocol error") {
		return true
	}

	// Timeout errors (often connection-related)
	if strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") {
		return true
	}

	// Check for common Go error types that indicate connection issues
	if strings.Contains(errStr, "EOF") {
		return true
	}

	return false
}

// MessageEncryptor defines the interface for message encryption/decryption
type MessageEncryptor interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
	Algorithm() string
}

// nopEncryptor is a no-operation encryptor implementation.
type nopEncryptor struct{}

func (n *nopEncryptor) Encrypt(data []byte) ([]byte, error) { return data, nil }
func (n *nopEncryptor) Decrypt(data []byte) ([]byte, error) { return data, nil }
func (n *nopEncryptor) Algorithm() string                   { return "none" }

// NewNopEncryptor creates a new no-op encryptor.
// For advanced encryption capabilities, use the encryption sub-package.
func NewNopEncryptor() MessageEncryptor {
	return &nopEncryptor{}
}

// MessageCompressor defines the interface for message compression/decompression.
// This is the core contract that all compression implementations must satisfy.
// For concrete implementations, use the compression sub-package.
type MessageCompressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	Algorithm() string
	Threshold() int // Minimum size to trigger compression
}

// PerformanceMonitor provides performance monitoring capabilities for RabbitMQ operations.
// This is the core contract that all performance monitoring implementations must satisfy.
// For concrete implementations, use the performance sub-package.
type PerformanceMonitor interface {
	// RecordPublish records a publish operation with its success status and duration
	RecordPublish(success bool, duration time.Duration)

	// RecordConsume records a consume operation with its success status and duration
	RecordConsume(success bool, duration time.Duration)
}

// MessageSerializer defines the interface for message serialization/deserialization.
// This is the core contract that all serialization implementations must satisfy.
// For concrete implementations, use the protobuf sub-package or other serialization packages.
type MessageSerializer interface {
	Serialize(msg any) ([]byte, error)
	Deserialize(data []byte, target any) error
	ContentType() string
}

// ConnectionPooler defines the interface for connection pool management.
// This is the core contract that all connection pool implementations must satisfy.
// For concrete implementations, use the pool sub-package.
type ConnectionPooler interface {
	Get() (*Client, error)
	Close() error
	Stats() PoolStats
	Size() int
	HealthyCount() int
}

// PoolStats provides statistics about a connection pool.
type PoolStats struct {
	TotalConnections   int64
	HealthyConnections int64
	FailedConnections  int64
	RepairAttempts     int64
	LastHealthCheck    time.Time
}

// StreamHandler defines the interface for RabbitMQ streams operations.
// This is the core contract that all stream implementations must satisfy.
// For concrete implementations, use the streams sub-package.
type StreamHandler interface {
	PublishToStream(ctx context.Context, streamName string, message *Message) error
	ConsumeFromStream(ctx context.Context, streamName string, handler StreamMessageHandler) error
	CreateStream(ctx context.Context, streamName string, config StreamConfig) error
	DeleteStream(ctx context.Context, streamName string) error
}

// StreamMessageHandler defines the function signature for processing stream messages
type StreamMessageHandler func(ctx context.Context, delivery *Delivery) error

// StreamConfig holds configuration for stream creation.
type StreamConfig struct {
	MaxAge              time.Duration
	MaxLengthMessages   int
	MaxLengthBytes      int
	MaxSegmentSizeBytes int
	InitialClusterSize  int
}

// SagaOrchestrator defines the interface for saga pattern orchestration.
// This is the core contract that all saga implementations must satisfy.
// For concrete implementations, use the saga sub-package.
type SagaOrchestrator interface {
	StartSaga(ctx context.Context, sagaName string, steps []SagaStep, context map[string]any) (string, error)
	CompensateSaga(ctx context.Context, sagaID string) error
	GetSagaStatus(ctx context.Context, sagaID string) (SagaStatus, error)
}

// SagaStep represents a step in a saga workflow.
type SagaStep struct {
	Name         string
	Action       string
	Compensation string
}

// SagaStatus represents the current status of a saga.
type SagaStatus struct {
	ID    string
	State string
	Steps []SagaStepStatus
}

// SagaStepStatus represents the status of a saga step.
type SagaStepStatus struct {
	Name      string
	State     string
	Error     string
	Output    map[string]any
	Timestamp time.Time
}

// GracefulShutdown defines the interface for graceful shutdown coordination.
// This is the core contract that all shutdown implementations must satisfy.
// For concrete implementations, use the shutdown sub-package.
type GracefulShutdown interface {
	RegisterComponent(component Closable) error
	ShutdownGracefully(ctx context.Context) error
	SetupSignalHandling() <-chan struct{}
	IsShutdownComplete() bool
}

// Closable interface for components that can be gracefully closed.
type Closable interface {
	Close() error
}

// No-op implementations for the new pluggable feature interfaces

// nopSerializer is a no-op implementation of MessageSerializer
type nopSerializer struct{}

func (n *nopSerializer) Serialize(msg any) ([]byte, error) {
	if data, ok := msg.([]byte); ok {
		return data, nil
	}
	return nil, fmt.Errorf("nop serializer can only handle []byte, got %T", msg)
}

func (n *nopSerializer) Deserialize(data []byte, target any) error {
	if ptr, ok := target.(*[]byte); ok {
		*ptr = data
		return nil
	}
	return fmt.Errorf("nop serializer can only deserialize to *[]byte, got %T", target)
}

func (n *nopSerializer) ContentType() string { return "application/octet-stream" }

// NewNopSerializer creates a new no-op serializer.
// For advanced serialization capabilities, use the protobuf or other serialization sub-packages.
func NewNopSerializer() MessageSerializer {
	return &nopSerializer{}
}

// nopCompressor is a no-op implementation of MessageCompressor
type nopCompressor struct{}

func (n *nopCompressor) Compress(data []byte) ([]byte, error)   { return data, nil }
func (n *nopCompressor) Decompress(data []byte) ([]byte, error) { return data, nil }
func (n *nopCompressor) Algorithm() string                      { return "none" }
func (n *nopCompressor) Threshold() int                         { return 0 }

// NewNopCompressor creates a new no-op compressor.
// For advanced compression capabilities, use the compression sub-package.
func NewNopCompressor() MessageCompressor {
	return &nopCompressor{}
}

// nopPerformanceMonitor is a no-op implementation of PerformanceMonitor
type nopPerformanceMonitor struct{}

func (n *nopPerformanceMonitor) RecordPublish(success bool, duration time.Duration) {}
func (n *nopPerformanceMonitor) RecordConsume(success bool, duration time.Duration) {}

// NewNopPerformanceMonitor creates a new no-op performance monitor.
// For advanced monitoring capabilities, use the performance sub-package.
func NewNopPerformanceMonitor() PerformanceMonitor {
	return &nopPerformanceMonitor{}
}

// nopConnectionPooler is a no-op implementation of ConnectionPooler
type nopConnectionPooler struct{}

func (n *nopConnectionPooler) Get() (*Client, error) {
	return nil, fmt.Errorf("no-op connection pooler: no connections available")
}
func (n *nopConnectionPooler) Close() error      { return nil }
func (n *nopConnectionPooler) Stats() PoolStats  { return PoolStats{} }
func (n *nopConnectionPooler) Size() int         { return 0 }
func (n *nopConnectionPooler) HealthyCount() int { return 0 }

// NewNopConnectionPooler creates a new no-op connection pooler.
// For advanced connection pooling capabilities, use the pool sub-package.
func NewNopConnectionPooler() ConnectionPooler {
	return &nopConnectionPooler{}
}

// nopStreamHandler is a no-op implementation of StreamHandler
type nopStreamHandler struct{}

func (n *nopStreamHandler) PublishToStream(ctx context.Context, streamName string, message *Message) error {
	return fmt.Errorf("no-op stream handler: streams not enabled")
}
func (n *nopStreamHandler) ConsumeFromStream(ctx context.Context, streamName string, handler StreamMessageHandler) error {
	return fmt.Errorf("no-op stream handler: streams not enabled")
}
func (n *nopStreamHandler) CreateStream(ctx context.Context, streamName string, config StreamConfig) error {
	return fmt.Errorf("no-op stream handler: streams not enabled")
}
func (n *nopStreamHandler) DeleteStream(ctx context.Context, streamName string) error {
	return fmt.Errorf("no-op stream handler: streams not enabled")
}

// NewNopStreamHandler creates a new no-op stream handler.
// For advanced streaming capabilities, use the streams sub-package.
func NewNopStreamHandler() StreamHandler {
	return &nopStreamHandler{}
}

// nopSagaOrchestrator is a no-op implementation of SagaOrchestrator
type nopSagaOrchestrator struct{}

func (n *nopSagaOrchestrator) StartSaga(ctx context.Context, sagaName string, steps []SagaStep, context map[string]any) (string, error) {
	return "", fmt.Errorf("no-op saga orchestrator: saga pattern not enabled")
}
func (n *nopSagaOrchestrator) CompensateSaga(ctx context.Context, sagaID string) error {
	return fmt.Errorf("no-op saga orchestrator: saga pattern not enabled")
}
func (n *nopSagaOrchestrator) GetSagaStatus(ctx context.Context, sagaID string) (SagaStatus, error) {
	return SagaStatus{}, fmt.Errorf("no-op saga orchestrator: saga pattern not enabled")
}

// NewNopSagaOrchestrator creates a new no-op saga orchestrator.
// For advanced saga orchestration capabilities, use the saga sub-package.
func NewNopSagaOrchestrator() SagaOrchestrator {
	return &nopSagaOrchestrator{}
}

// nopGracefulShutdown is a no-op implementation of GracefulShutdown
type nopGracefulShutdown struct{}

func (n *nopGracefulShutdown) RegisterComponent(component Closable) error   { return nil }
func (n *nopGracefulShutdown) ShutdownGracefully(ctx context.Context) error { return nil }
func (n *nopGracefulShutdown) SetupSignalHandling() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (n *nopGracefulShutdown) IsShutdownComplete() bool { return false }

// NewNopGracefulShutdown creates a new no-op graceful shutdown handler.
// For advanced shutdown coordination capabilities, use the shutdown sub-package.
func NewNopGracefulShutdown() GracefulShutdown {
	return &nopGracefulShutdown{}
}
