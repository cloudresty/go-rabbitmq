# OpenTelemetry Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [OpenTelemetry Package](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides the complete API reference for the `otel` sub-package. The otel package provides OpenTelemetry integration implementing the `rabbitmq.MetricsCollector` and `rabbitmq.Tracer` interfaces using OpenTelemetry semantic conventions for messaging systems.

&nbsp;

## Constructor Functions

| Function | Description |
| :--- | :--- |
| `NewMetricsCollector(meter metric.Meter) (*MetricsCollector, error)` | Creates a new OpenTelemetry metrics collector |
| `NewTracer(tracer trace.Tracer) *Tracer` | Creates a new OpenTelemetry tracer wrapper |

&nbsp;

🔝 [back to top](#opentelemetry-package-api-reference)

&nbsp;

## Types and Structures

| Type | Description |
| :--- | :--- |
| `MetricsCollector` | Implements `rabbitmq.MetricsCollector` using OpenTelemetry metrics |
| `Tracer` | Implements `rabbitmq.Tracer` using OpenTelemetry tracing |
| `Span` | Implements `rabbitmq.Span` wrapping an OpenTelemetry span |

&nbsp;

🔝 [back to top](#opentelemetry-package-api-reference)

&nbsp;

## MetricsCollector Methods

| Method | Description |
| :--- | :--- |
| `RecordConnection(connectionName string)` | Records a new connection |
| `RecordConnectionAttempt(success bool, duration time.Duration)` | Records a connection attempt |
| `RecordReconnection(attempt int)` | Records a reconnection attempt |
| `RecordPublish(exchange, routingKey string, messageSize int, duration time.Duration)` | Records a publish operation |
| `RecordPublishConfirmation(success bool, duration time.Duration)` | Records a publish confirmation |
| `RecordDeliveryOutcome(outcome rabbitmq.DeliveryOutcome, duration time.Duration)` | Records a delivery assurance outcome |
| `RecordDeliveryTimeout(messageID string)` | Records a delivery timeout |
| `RecordConsume(queue string, messageSize int, duration time.Duration)` | Records a consume operation |
| `RecordMessageReceived(queue string)` | Records a message received event |
| `RecordMessageProcessed(queue string, success bool, duration time.Duration)` | Records a message processed event |
| `RecordMessageRequeued(queue string)` | Records a message requeue event |
| `RecordHealthCheck(success bool, duration time.Duration)` | Records a health check |
| `RecordError(operation string, err error)` | Records an error |

&nbsp;

🔝 [back to top](#opentelemetry-package-api-reference)

&nbsp;

## Tracer Methods

| Method | Description |
| :--- | :--- |
| `StartSpan(ctx context.Context, operation string) (context.Context, rabbitmq.Span)` | Starts a new span for the given operation |

&nbsp;

🔝 [back to top](#opentelemetry-package-api-reference)

&nbsp;

## Span Methods

| Method | Description |
| :--- | :--- |
| `SetAttribute(key string, value any)` | Sets an attribute on the span |
| `SetStatus(code rabbitmq.SpanStatusCode, description string)` | Sets the status of the span |
| `End()` | Ends the span |

&nbsp;

🔝 [back to top](#opentelemetry-package-api-reference)

&nbsp;

## Semantic Convention Constants

| Constant | Value | Description |
| :--- | :--- | :--- |
| `MessagingSystem` | `"messaging.system"` | Messaging system attribute key |
| `MessagingOperation` | `"messaging.operation"` | Operation type attribute key |
| `MessagingRabbitMQRoutingKey` | `"messaging.rabbitmq.routing_key"` | RabbitMQ routing key attribute |
| `MessagingDestinationName` | `"messaging.destination.name"` | Destination (exchange/queue) name |
| `MessagingMessageID` | `"messaging.message.id"` | Message ID attribute |
| `MessagingMessageBodySize` | `"messaging.message.body.size"` | Message body size attribute |
| `MessagingMessageConversationID` | `"messaging.message.conversation_id"` | Conversation ID attribute |
| `MessagingConsumerID` | `"messaging.consumer.id"` | Consumer ID attribute |
| `OperationPublish` | `"publish"` | Publish operation type |
| `OperationReceive` | `"receive"` | Receive operation type |
| `OperationProcess` | `"process"` | Process operation type |

&nbsp;

🔝 [back to top](#opentelemetry-package-api-reference)

&nbsp;

## Usage Examples

&nbsp;

### Basic Integration

```go
import (
    "github.com/cloudresty/go-rabbitmq"
    rabbitotel "github.com/cloudresty/go-rabbitmq/otel"
    "go.opentelemetry.io/otel"
)

// Get OpenTelemetry providers
meter := otel.Meter("rabbitmq-client")
tracer := otel.Tracer("rabbitmq-client")

// Create metrics collector
metrics, err := rabbitotel.NewMetricsCollector(meter)
if err != nil {
    log.Fatal(err)
}

// Create tracer wrapper
otelTracer := rabbitotel.NewTracer(tracer)

// Use with RabbitMQ client
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithMetrics(metrics),
    rabbitmq.WithTracer(otelTracer),
)
```

&nbsp;

🔝 [back to top](#opentelemetry-package-api-reference)

&nbsp;

### Production Setup with OTLP Exporter

```go
import (
    "context"

    "github.com/cloudresty/go-rabbitmq"
    rabbitotel "github.com/cloudresty/go-rabbitmq/otel"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func setupObservability(ctx context.Context) (*rabbitotel.MetricsCollector, *rabbitotel.Tracer, error) {
    // Setup OTLP metric exporter
    metricExporter, err := otlpmetricgrpc.New(ctx)
    if err != nil {
        return nil, nil, err
    }

    meterProvider := sdkmetric.NewMeterProvider(
        sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
    )
    otel.SetMeterProvider(meterProvider)

    // Setup OTLP trace exporter
    traceExporter, err := otlptracegrpc.New(ctx)
    if err != nil {
        return nil, nil, err
    }

    tracerProvider := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(traceExporter),
    )
    otel.SetTracerProvider(tracerProvider)

    // Create RabbitMQ observability components
    metrics, err := rabbitotel.NewMetricsCollector(otel.Meter("rabbitmq-client"))
    if err != nil {
        return nil, nil, err
    }

    tracer := rabbitotel.NewTracer(otel.Tracer("rabbitmq-client"))

    return metrics, tracer, nil
}
```

&nbsp;

🔝 [back to top](#opentelemetry-package-api-reference)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

<sub>&copy; Cloudresty - All rights reserved</sub>

&nbsp;
