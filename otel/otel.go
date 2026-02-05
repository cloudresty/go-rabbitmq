// Package otel provides OpenTelemetry integration for the go-rabbitmq library.
// It implements the MetricsCollector and Tracer interfaces using OpenTelemetry
// semantic conventions for messaging systems.
//
// Usage:
//
//	import (
//	    "github.com/cloudresty/go-rabbitmq"
//	    "github.com/cloudresty/go-rabbitmq/otel"
//	    "go.opentelemetry.io/otel"
//	    "go.opentelemetry.io/otel/metric"
//	)
//
//	// Create OpenTelemetry metrics collector
//	meter := otel.Meter("rabbitmq-client")
//	metrics, err := otel.NewMetricsCollector(meter)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create OpenTelemetry tracer
//	tracer := otel.NewTracer(otel.Tracer("rabbitmq-client"))
//
//	// Use with client
//	client, err := rabbitmq.NewClient(
//	    rabbitmq.WithMetrics(metrics),
//	    rabbitmq.WithTracer(tracer),
//	)
package otel

import (
	"context"
	"fmt"
	"time"

	rabbitmq "github.com/cloudresty/go-rabbitmq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Semantic convention attribute keys for messaging
// Based on OpenTelemetry Semantic Conventions for Messaging
// https://opentelemetry.io/docs/specs/semconv/messaging/
const (
	// Messaging system attributes
	MessagingSystem    = "messaging.system"
	MessagingOperation = "messaging.operation"

	// RabbitMQ specific attributes
	MessagingRabbitMQRoutingKey = "messaging.rabbitmq.routing_key"
	MessagingDestinationName    = "messaging.destination.name"

	// Message attributes
	MessagingMessageID             = "messaging.message.id"
	MessagingMessageBodySize       = "messaging.message.body.size"
	MessagingMessageConversationID = "messaging.message.conversation_id"

	// Consumer attributes
	MessagingConsumerID = "messaging.consumer.id"

	// Operation types
	OperationPublish = "publish"
	OperationReceive = "receive"
	OperationProcess = "process"
)

// MetricsCollector implements rabbitmq.MetricsCollector using OpenTelemetry metrics.
type MetricsCollector struct {
	meter metric.Meter

	// Connection metrics
	connectionCounter   metric.Int64Counter
	connectionAttempts  metric.Int64Counter
	connectionDuration  metric.Float64Histogram
	reconnectionCounter metric.Int64Counter

	// Publishing metrics
	publishCounter       metric.Int64Counter
	publishDuration      metric.Float64Histogram
	publishMessageSize   metric.Int64Histogram
	confirmationCounter  metric.Int64Counter
	confirmationDuration metric.Float64Histogram

	// Delivery assurance metrics
	deliveryOutcomeCounter metric.Int64Counter
	deliveryTimeoutCounter metric.Int64Counter

	// Consumption metrics
	consumeCounter          metric.Int64Counter
	consumeDuration         metric.Float64Histogram
	consumeMessageSize      metric.Int64Histogram
	messageReceivedCounter  metric.Int64Counter
	messageProcessedCounter metric.Int64Counter
	messageRequeuedCounter  metric.Int64Counter

	// Health metrics
	healthCheckCounter  metric.Int64Counter
	healthCheckDuration metric.Float64Histogram
	errorCounter        metric.Int64Counter
}

// NewMetricsCollector creates a new OpenTelemetry metrics collector.
// The meter should be obtained from an OpenTelemetry MeterProvider.
func NewMetricsCollector(meter metric.Meter) (*MetricsCollector, error) {
	m := &MetricsCollector{meter: meter}

	var err error

	// Connection metrics
	m.connectionCounter, err = meter.Int64Counter("rabbitmq.connections",
		metric.WithDescription("Number of RabbitMQ connections established"),
		metric.WithUnit("{connection}"))
	if err != nil {
		return nil, err
	}

	m.connectionAttempts, err = meter.Int64Counter("rabbitmq.connection.attempts",
		metric.WithDescription("Number of connection attempts"),
		metric.WithUnit("{attempt}"))
	if err != nil {
		return nil, err
	}

	m.connectionDuration, err = meter.Float64Histogram("rabbitmq.connection.duration",
		metric.WithDescription("Duration of connection attempts"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	m.reconnectionCounter, err = meter.Int64Counter("rabbitmq.reconnections",
		metric.WithDescription("Number of reconnection attempts"),
		metric.WithUnit("{reconnection}"))
	if err != nil {
		return nil, err
	}

	// Publishing metrics
	m.publishCounter, err = meter.Int64Counter("rabbitmq.publish.count",
		metric.WithDescription("Number of messages published"),
		metric.WithUnit("{message}"))
	if err != nil {
		return nil, err
	}

	m.publishDuration, err = meter.Float64Histogram("rabbitmq.publish.duration",
		metric.WithDescription("Duration of publish operations"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	m.publishMessageSize, err = meter.Int64Histogram("rabbitmq.publish.message.size",
		metric.WithDescription("Size of published messages"),
		metric.WithUnit("By"))
	if err != nil {
		return nil, err
	}

	m.confirmationCounter, err = meter.Int64Counter("rabbitmq.publish.confirmations",
		metric.WithDescription("Number of publish confirmations"),
		metric.WithUnit("{confirmation}"))
	if err != nil {
		return nil, err
	}

	m.confirmationDuration, err = meter.Float64Histogram("rabbitmq.publish.confirmation.duration",
		metric.WithDescription("Duration waiting for publish confirmations"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	// Delivery assurance metrics
	m.deliveryOutcomeCounter, err = meter.Int64Counter("rabbitmq.delivery.outcomes",
		metric.WithDescription("Number of delivery outcomes by type"),
		metric.WithUnit("{outcome}"))
	if err != nil {
		return nil, err
	}

	m.deliveryTimeoutCounter, err = meter.Int64Counter("rabbitmq.delivery.timeouts",
		metric.WithDescription("Number of delivery timeouts"),
		metric.WithUnit("{timeout}"))
	if err != nil {
		return nil, err
	}

	// Consumption metrics
	m.consumeCounter, err = meter.Int64Counter("rabbitmq.consume.count",
		metric.WithDescription("Number of messages consumed"),
		metric.WithUnit("{message}"))
	if err != nil {
		return nil, err
	}

	m.consumeDuration, err = meter.Float64Histogram("rabbitmq.consume.duration",
		metric.WithDescription("Duration of consume operations"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	m.consumeMessageSize, err = meter.Int64Histogram("rabbitmq.consume.message.size",
		metric.WithDescription("Size of consumed messages"),
		metric.WithUnit("By"))
	if err != nil {
		return nil, err
	}

	m.messageReceivedCounter, err = meter.Int64Counter("rabbitmq.messages.received",
		metric.WithDescription("Number of messages received"),
		metric.WithUnit("{message}"))
	if err != nil {
		return nil, err
	}

	m.messageProcessedCounter, err = meter.Int64Counter("rabbitmq.messages.processed",
		metric.WithDescription("Number of messages processed"),
		metric.WithUnit("{message}"))
	if err != nil {
		return nil, err
	}

	m.messageRequeuedCounter, err = meter.Int64Counter("rabbitmq.messages.requeued",
		metric.WithDescription("Number of messages requeued"),
		metric.WithUnit("{message}"))
	if err != nil {
		return nil, err
	}

	// Health metrics
	m.healthCheckCounter, err = meter.Int64Counter("rabbitmq.health.checks",
		metric.WithDescription("Number of health checks"),
		metric.WithUnit("{check}"))
	if err != nil {
		return nil, err
	}

	m.healthCheckDuration, err = meter.Float64Histogram("rabbitmq.health.check.duration",
		metric.WithDescription("Duration of health checks"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	m.errorCounter, err = meter.Int64Counter("rabbitmq.errors",
		metric.WithDescription("Number of errors"),
		metric.WithUnit("{error}"))
	if err != nil {
		return nil, err
	}

	return m, nil
}

// Ensure MetricsCollector implements rabbitmq.MetricsCollector
var _ rabbitmq.MetricsCollector = (*MetricsCollector)(nil)

// RecordConnection records a new connection
func (m *MetricsCollector) RecordConnection(connectionName string) {
	m.connectionCounter.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("connection.name", connectionName)))
}

// RecordConnectionAttempt records a connection attempt
func (m *MetricsCollector) RecordConnectionAttempt(success bool, duration time.Duration) {
	ctx := context.Background()
	m.connectionAttempts.Add(ctx, 1,
		metric.WithAttributes(attribute.Bool("success", success)))
	m.connectionDuration.Record(ctx, duration.Seconds(),
		metric.WithAttributes(attribute.Bool("success", success)))
}

// RecordReconnection records a reconnection attempt
func (m *MetricsCollector) RecordReconnection(attempt int) {
	m.reconnectionCounter.Add(context.Background(), 1,
		metric.WithAttributes(attribute.Int("attempt", attempt)))
}

// RecordPublish records a publish operation
func (m *MetricsCollector) RecordPublish(exchange, routingKey string, messageSize int, duration time.Duration) {
	ctx := context.Background()
	attrs := metric.WithAttributes(
		attribute.String(MessagingSystem, "rabbitmq"),
		attribute.String(MessagingOperation, OperationPublish),
		attribute.String(MessagingDestinationName, exchange),
		attribute.String(MessagingRabbitMQRoutingKey, routingKey),
	)
	m.publishCounter.Add(ctx, 1, attrs)
	m.publishDuration.Record(ctx, duration.Seconds(), attrs)
	m.publishMessageSize.Record(ctx, int64(messageSize), attrs)
}

// RecordPublishConfirmation records a publish confirmation
func (m *MetricsCollector) RecordPublishConfirmation(success bool, duration time.Duration) {
	ctx := context.Background()
	attrs := metric.WithAttributes(attribute.Bool("success", success))
	m.confirmationCounter.Add(ctx, 1, attrs)
	m.confirmationDuration.Record(ctx, duration.Seconds(), attrs)
}

// RecordDeliveryOutcome records a delivery assurance outcome
func (m *MetricsCollector) RecordDeliveryOutcome(outcome rabbitmq.DeliveryOutcome, duration time.Duration) {
	m.deliveryOutcomeCounter.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("outcome", string(outcome))))
}

// RecordDeliveryTimeout records a delivery timeout
func (m *MetricsCollector) RecordDeliveryTimeout(messageID string) {
	m.deliveryTimeoutCounter.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String(MessagingMessageID, messageID)))
}

// RecordConsume records a consume operation
func (m *MetricsCollector) RecordConsume(queue string, messageSize int, duration time.Duration) {
	ctx := context.Background()
	attrs := metric.WithAttributes(
		attribute.String(MessagingSystem, "rabbitmq"),
		attribute.String(MessagingOperation, OperationReceive),
		attribute.String(MessagingDestinationName, queue),
	)
	m.consumeCounter.Add(ctx, 1, attrs)
	m.consumeDuration.Record(ctx, duration.Seconds(), attrs)
	m.consumeMessageSize.Record(ctx, int64(messageSize), attrs)
}

// RecordMessageReceived records a message received event
func (m *MetricsCollector) RecordMessageReceived(queue string) {
	m.messageReceivedCounter.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String(MessagingDestinationName, queue)))
}

// RecordMessageProcessed records a message processed event
func (m *MetricsCollector) RecordMessageProcessed(queue string, success bool, duration time.Duration) {
	ctx := context.Background()
	attrs := metric.WithAttributes(
		attribute.String(MessagingSystem, "rabbitmq"),
		attribute.String(MessagingOperation, OperationProcess),
		attribute.String(MessagingDestinationName, queue),
		attribute.Bool("success", success),
	)
	m.messageProcessedCounter.Add(ctx, 1, attrs)
}

// RecordMessageRequeued records a message requeue event
func (m *MetricsCollector) RecordMessageRequeued(queue string) {
	m.messageRequeuedCounter.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String(MessagingDestinationName, queue)))
}

// RecordHealthCheck records a health check
func (m *MetricsCollector) RecordHealthCheck(success bool, duration time.Duration) {
	ctx := context.Background()
	attrs := metric.WithAttributes(attribute.Bool("success", success))
	m.healthCheckCounter.Add(ctx, 1, attrs)
	m.healthCheckDuration.Record(ctx, duration.Seconds(), attrs)
}

// RecordError records an error
func (m *MetricsCollector) RecordError(operation string, err error) {
	m.errorCounter.Add(context.Background(), 1,
		metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.String("error.type", err.Error()),
		))
}

// Tracer implements rabbitmq.Tracer using OpenTelemetry tracing.
type Tracer struct {
	tracer trace.Tracer
}

// NewTracer creates a new OpenTelemetry tracer wrapper.
// The tracer should be obtained from an OpenTelemetry TracerProvider.
func NewTracer(tracer trace.Tracer) *Tracer {
	return &Tracer{tracer: tracer}
}

// Ensure Tracer implements rabbitmq.Tracer
var _ rabbitmq.Tracer = (*Tracer)(nil)

// StartSpan starts a new span for the given operation
func (t *Tracer) StartSpan(ctx context.Context, operation string) (context.Context, rabbitmq.Span) {
	ctx, span := t.tracer.Start(ctx, operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String(MessagingSystem, "rabbitmq")),
	)
	return ctx, &Span{span: span}
}

// Span wraps an OpenTelemetry span to implement rabbitmq.Span
type Span struct {
	span trace.Span
}

// Ensure Span implements rabbitmq.Span
var _ rabbitmq.Span = (*Span)(nil)

// SetAttribute sets an attribute on the span
func (s *Span) SetAttribute(key string, value any) {
	switch v := value.(type) {
	case string:
		s.span.SetAttributes(attribute.String(key, v))
	case int:
		s.span.SetAttributes(attribute.Int(key, v))
	case int64:
		s.span.SetAttributes(attribute.Int64(key, v))
	case float64:
		s.span.SetAttributes(attribute.Float64(key, v))
	case bool:
		s.span.SetAttributes(attribute.Bool(key, v))
	default:
		s.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
	}
}

// SetStatus sets the status of the span
func (s *Span) SetStatus(code rabbitmq.SpanStatusCode, description string) {
	switch code {
	case rabbitmq.SpanStatusOK:
		s.span.SetStatus(codes.Ok, description)
	case rabbitmq.SpanStatusError:
		s.span.SetStatus(codes.Error, description)
	default:
		s.span.SetStatus(codes.Unset, description)
	}
}

// End ends the span
func (s *Span) End() {
	s.span.End()
}
