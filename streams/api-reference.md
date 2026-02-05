# Streams Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [Streams Package](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides the complete API reference for the `streams` sub-package. The streams package provides RabbitMQ streams functionality using the **native stream protocol** (port 5552) for high-throughput scenarios, offering 5-10x better performance compared to AMQP 0.9.1.

&nbsp;

## Constructor Functions

| Function | Description |
| :--- | :--- |
| `NewHandler(opts Options) (*Handler, error)` | Creates a new stream handler using the native RabbitMQ stream protocol |

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

## Options Struct

| Field | Type | Description |
| :--- | :--- | :--- |
| `Host` | `string` | RabbitMQ host (default: "localhost") |
| `Port` | `int` | Stream protocol port (default: 5552) |
| `Username` | `string` | Authentication username (default: "guest") |
| `Password` | `string` | Authentication password (default: "guest") |
| `VHost` | `string` | Virtual host (default: "/") |
| `MaxProducers` | `int` | Maximum producers per connection |
| `MaxConsumers` | `int` | Maximum consumers per connection |
| `RequestedHeartbeat` | `time.Duration` | Heartbeat interval for connection health |

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

## Handler Methods

| Method | Description |
| :--- | :--- |
| `Close() error` | Closes the handler and all associated producers/consumers |
| `PublishToStream(ctx, streamName, message) error` | Publishes a message to the specified stream |
| `ConsumeFromStream(ctx, streamName, handler) error` | Consumes messages from the specified stream |
| `CreateStream(ctx, streamName, config) error` | Creates a new stream with the specified configuration |
| `DeleteStream(ctx, streamName) error` | Deletes an existing stream and all its messages |

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

## Types and Structures

| Type | Description |
| :--- | :--- |
| `Options` | Configuration options for creating a stream handler |
| `Handler` | Main stream handler that implements `rabbitmq.StreamHandler` interface |

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

## Usage Examples

&nbsp;

### Basic Stream Operations

```go
import (
    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/streams"
)

// Create stream handler using native protocol (port 5552)
handler, err := streams.NewHandler(streams.Options{
    Host:     "localhost",
    Port:     5552,
    Username: "guest",
    Password: "guest",
})
if err != nil {
    log.Fatal(err)
}
defer handler.Close()

// Create a stream with custom configuration
config := rabbitmq.StreamConfig{
    MaxAge:         24 * time.Hour,       // Retain messages for 24 hours
    MaxLengthBytes: 1024 * 1024 * 1024,   // Maximum 1GB
}

err = handler.CreateStream(ctx, "events-stream", config)
if err != nil {
    log.Fatal(err)
}
```

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

### Publishing to Streams

```go
// Create a message
message := rabbitmq.NewMessage([]byte(`{"event": "user_created", "user_id": "12345"}`)).
    WithContentType("application/json").
    WithHeader("event_type", "user_created").
    WithTimestamp(time.Now())

// Publish to stream
err := handler.PublishToStream(ctx, "events-stream", message)
if err != nil {
    log.Printf("Failed to publish message: %v", err)
} else {
    log.Println("Message published to stream successfully")
}
```

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

### Consuming from Streams

```go
// Define message handler
messageHandler := func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    log.Printf("Received message: %s", string(delivery.Body))

    // Process the message
    if err := processStreamMessage(delivery); err != nil {
        log.Printf("Failed to process message: %v", err)
        return err
    }

    // Acknowledge the message
    return delivery.Ack()
}

// Start consuming from stream
err := handler.ConsumeFromStream(ctx, "events-stream", messageHandler)
if err != nil {
    log.Fatal(err)
}

log.Println("Started consuming from stream...")
```

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

### Event Sourcing Pattern

```go
// Event sourcing with streams
type EventStore struct {
    handler *streams.Handler
}

func NewEventStore(opts streams.Options) (*EventStore, error) {
    handler, err := streams.NewHandler(opts)
    if err != nil {
        return nil, err
    }
    return &EventStore{handler: handler}, nil
}

func (es *EventStore) AppendEvent(ctx context.Context, streamName string, event interface{}) error {
    // Serialize event
    data, err := json.Marshal(event)
    if err != nil {
        return err
    }

    // Create message with event metadata
    message := rabbitmq.NewMessage(data).
        WithContentType("application/json").
        WithHeader("event_type", reflect.TypeOf(event).Name()).
        WithHeader("event_version", "1.0").
        WithTimestamp(time.Now())

    // Append to stream
    return es.handler.PublishToStream(ctx, streamName, message)
}

func (es *EventStore) ReplayEvents(ctx context.Context, streamName string, handler rabbitmq.StreamMessageHandler) error {
    return es.handler.ConsumeFromStream(ctx, streamName, handler)
}

func (es *EventStore) Close() error {
    return es.handler.Close()
}
```

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

### Time-Series Data Streaming

```go
// Time-series data publishing
type MetricsCollector struct {
    handler    *streams.Handler
    streamName string
}

func NewMetricsCollector(opts streams.Options, streamName string) (*MetricsCollector, error) {
    handler, err := streams.NewHandler(opts)
    if err != nil {
        return nil, err
    }

    // Create stream with time-based retention
    config := rabbitmq.StreamConfig{
        MaxAge: 7 * 24 * time.Hour, // Keep 7 days of metrics
    }
    handler.CreateStream(context.Background(), streamName, config)

    return &MetricsCollector{
        handler:    handler,
        streamName: streamName,
    }, nil
}

func (mc *MetricsCollector) PublishMetric(ctx context.Context, metric Metric) error {
    data, err := json.Marshal(metric)
    if err != nil {
        return err
    }

    message := rabbitmq.NewMessage(data).
        WithContentType("application/json").
        WithHeader("metric_type", metric.Type).
        WithHeader("timestamp", metric.Timestamp.Unix()).
        WithTimestamp(metric.Timestamp)

    return mc.handler.PublishToStream(ctx, mc.streamName, message)
}
```

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

### Stream Management

```go
// Create stream handler
handler, err := streams.NewHandler(streams.Options{
    Host:     "localhost",
    Port:     5552,
    Username: "guest",
    Password: "guest",
})
if err != nil {
    log.Fatal(err)
}
defer handler.Close()

// Create multiple streams with different configurations
streamConfigs := map[string]rabbitmq.StreamConfig{
    "user-events": {
        MaxAge:         30 * 24 * time.Hour,      // 30 days
        MaxLengthBytes: 5 * 1024 * 1024 * 1024,   // 5GB
    },
    "order-events": {
        MaxAge:         7 * 24 * time.Hour,       // 7 days
        MaxLengthBytes: 2 * 1024 * 1024 * 1024,   // 2GB
    },
    "metrics": {
        MaxAge: 24 * time.Hour, // 1 day
    },
}

// Create all streams
for streamName, config := range streamConfigs {
    err := handler.CreateStream(ctx, streamName, config)
    if err != nil {
        log.Printf("Failed to create stream %s: %v", streamName, err)
    } else {
        log.Printf("Created stream: %s", streamName)
    }
}

// Later, clean up streams if needed
for streamName := range streamConfigs {
    err := handler.DeleteStream(ctx, streamName)
    if err != nil {
        log.Printf("Failed to delete stream %s: %v", streamName, err)
    }
}
```

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

### High-Throughput Publishing

```go
// High-throughput publishing with batching
func publishBatchToStream(handler *streams.Handler, streamName string, events []Event) error {
    ctx := context.Background()

    for _, event := range events {
        data, err := json.Marshal(event)
        if err != nil {
            return err
        }

        message := rabbitmq.NewMessage(data).
            WithContentType("application/json").
            WithHeader("event_id", event.ID).
            WithTimestamp(event.Timestamp)

        // Publish each event (streams handle high throughput well)
        if err := handler.PublishToStream(ctx, streamName, message); err != nil {
            log.Printf("Failed to publish event %s: %v", event.ID, err)
            // Continue with other events or implement retry logic
        }
    }

    return nil
}

// Usage
events := generateEvents(1000) // Generate 1000 events
err := publishBatchToStream(handler, "high-volume-stream", events)
if err != nil {
    log.Printf("Batch publishing failed: %v", err)
}
```

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

### Consumer with Offset Management

```go
// Consumer with manual offset management
func consumeWithOffsetTracking(handler *streams.Handler, streamName string) {
    var lastProcessedOffset uint64

    messageHandler := func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        // Process message
        if err := processMessage(delivery); err != nil {
            log.Printf("Failed to process message: %v", err)
            return err
        }

        // Track offset (in real implementation, persist this)
        if offset := getMessageOffset(delivery); offset > lastProcessedOffset {
            lastProcessedOffset = offset
            log.Printf("Processed message at offset: %d", offset)
        }

        return delivery.Ack()
    }

    // Start consuming
    err := handler.ConsumeFromStream(context.Background(), streamName, messageHandler)
    if err != nil {
        log.Fatal(err)
    }
}
```

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

## Best Practices

1. **Stream Configuration**: Choose appropriate retention policies (max age, max size) based on your use case and storage constraints
2. **Message Ordering**: Leverage streams' natural ordering guarantees for event sourcing and time-series applications
3. **High Throughput**: Streams are designed for high throughput - take advantage of this for demanding applications
4. **Error Handling**: Implement robust error handling for both publishing and consuming operations
5. **Offset Management**: Consider implementing custom offset tracking for consumer resume capabilities
6. **Stream Lifecycle**: Plan stream creation and deletion carefully as streams store data persistently
7. **Message Size**: Be mindful of message sizes as they affect storage and throughput
8. **Consumer Groups**: Use multiple consumers for parallel processing while maintaining order per partition
9. **Monitoring**: Monitor stream size, throughput, and consumer lag for operational visibility
10. **Resource Management**: Properly close handlers and connections to prevent resource leaks

&nbsp;

🔝 [back to top](#streams-package-api-reference)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

<sub>&copy; Cloudresty - All rights reserved</sub>

&nbsp;
