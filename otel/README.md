# OpenTelemetry Package

[Home](../README.md) &nbsp;/&nbsp; OpenTelemetry Package

&nbsp;

The `otel` package provides OpenTelemetry integration for the go-rabbitmq library. It implements the `MetricsCollector` and `Tracer` interfaces using OpenTelemetry semantic conventions for messaging systems, enabling production-grade observability for your RabbitMQ applications.

&nbsp;

## Features

- **OpenTelemetry Metrics**: Full metrics collection using OpenTelemetry SDK
- **Distributed Tracing**: Span creation and context propagation for publish/consume operations
- **Semantic Conventions**: Follows [OpenTelemetry Messaging Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/messaging/)
- **Connection Metrics**: Track connections, reconnections, and connection attempts
- **Publishing Metrics**: Monitor publish operations, confirmations, and message sizes
- **Consumption Metrics**: Track message consumption, processing, and requeue events
- **Delivery Assurance Metrics**: Monitor delivery outcomes and timeouts
- **Health Check Metrics**: Track health check operations and durations
- **Error Tracking**: Comprehensive error counting by operation type

&nbsp;

🔝 [back to top](#opentelemetry-package)

&nbsp;

## Installation

```bash
go get github.com/cloudresty/go-rabbitmq/otel
```

&nbsp;

🔝 [back to top](#opentelemetry-package)

&nbsp;

## Quick Start

### Basic OpenTelemetry Integration

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
    rabbitotel "github.com/cloudresty/go-rabbitmq/otel"
    "go.opentelemetry.io/otel"
)

func main() {
    // Get OpenTelemetry meter and tracer from your provider
    meter := otel.Meter("rabbitmq-client")
    tracer := otel.Tracer("rabbitmq-client")

    // Create OpenTelemetry metrics collector
    metrics, err := rabbitotel.NewMetricsCollector(meter)
    if err != nil {
        log.Fatal("Failed to create metrics collector:", err)
    }

    // Create OpenTelemetry tracer wrapper
    otelTracer := rabbitotel.NewTracer(tracer)

    // Create RabbitMQ client with OpenTelemetry observability
    client, err := rabbitmq.NewClient(
        rabbitmq.FromEnv(),
        rabbitmq.WithMetrics(metrics),
        rabbitmq.WithTracer(otelTracer),
    )
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // All publish/consume operations are now traced and metered
    publisher, _ := client.NewPublisher()
    _ = publisher.Publish(context.Background(), "events", "user.created",
        rabbitmq.NewMessage([]byte(`{"user_id": "123"}`)))
}
```

&nbsp;

🔝 [back to top](#opentelemetry-package)

&nbsp;

## Metrics Reference

### Connection Metrics

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `rabbitmq.connections` | Counter | Number of RabbitMQ connections established |
| `rabbitmq.connection.attempts` | Counter | Number of connection attempts |
| `rabbitmq.connection.duration` | Histogram | Duration of connection attempts (seconds) |
| `rabbitmq.reconnections` | Counter | Number of reconnection attempts |

### Publishing Metrics

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `rabbitmq.publish.count` | Counter | Number of messages published |
| `rabbitmq.publish.duration` | Histogram | Duration of publish operations (seconds) |
| `rabbitmq.publish.message.size` | Histogram | Size of published messages (bytes) |
| `rabbitmq.publish.confirmations` | Counter | Number of publish confirmations |
| `rabbitmq.publish.confirmation.duration` | Histogram | Duration waiting for confirmations (seconds) |

### Delivery Assurance Metrics

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `rabbitmq.delivery.outcomes` | Counter | Number of delivery outcomes by type |
| `rabbitmq.delivery.timeouts` | Counter | Number of delivery timeouts |

### Consumption Metrics

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `rabbitmq.consume.count` | Counter | Number of messages consumed |
| `rabbitmq.consume.duration` | Histogram | Duration of consume operations (seconds) |
| `rabbitmq.consume.message.size` | Histogram | Size of consumed messages (bytes) |
| `rabbitmq.messages.received` | Counter | Number of messages received |
| `rabbitmq.messages.processed` | Counter | Number of messages processed |
| `rabbitmq.messages.requeued` | Counter | Number of messages requeued |

### Health Metrics

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `rabbitmq.health.checks` | Counter | Number of health checks |
| `rabbitmq.health.check.duration` | Histogram | Duration of health checks (seconds) |
| `rabbitmq.errors` | Counter | Number of errors by operation |

&nbsp;

🔝 [back to top](#opentelemetry-package)

&nbsp;

## Semantic Conventions

This package follows the [OpenTelemetry Semantic Conventions for Messaging](https://opentelemetry.io/docs/specs/semconv/messaging/). The following attributes are automatically added to spans and metrics:

| Attribute | Description |
| :--- | :--- |
| `messaging.system` | Always set to `"rabbitmq"` |
| `messaging.operation` | Operation type: `publish`, `receive`, `process` |
| `messaging.destination.name` | Exchange or queue name |
| `messaging.rabbitmq.routing_key` | RabbitMQ routing key |
| `messaging.message.id` | Message ID |
| `messaging.message.body.size` | Message body size in bytes |
| `messaging.consumer.id` | Consumer identifier |

&nbsp;

🔝 [back to top](#opentelemetry-package)

&nbsp;

## API Reference

For detailed API documentation, see [API Reference](api-reference.md).

&nbsp;

🔝 [back to top](#opentelemetry-package)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

<sub>&copy; Cloudresty - All rights reserved</sub>

&nbsp;
