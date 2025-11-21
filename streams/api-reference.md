# Streams Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [Streams Package](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides the complete API reference for the `streams` sub-package. The streams package provides RabbitMQ streams functionality for high-throughput scenarios, offering persistent, replicated, and high-performance messaging ideal for event sourcing, time-series data, and cases where message order and durability are critical.

&nbsp;

## Constructor Functions

| Function | Description |
|----------|-------------|
| `NewHandler(client *rabbitmq.Client)` | Creates a new stream handler with the provided RabbitMQ client |

🔝 [back to top](#streams-package-api-reference)

&nbsp;

## Handler Methods

| Function | Description |
|----------|-------------|
| `PublishToStream(ctx context.Context, streamName string, message *rabbitmq.Message)` | Publishes a message to the specified stream with automatic stream creation if needed |
| `ConsumeFromStream(ctx context.Context, streamName string, handler rabbitmq.StreamMessageHandler)` | Consumes messages from the specified stream using the provided message handler |
| `CreateStream(ctx context.Context, streamName string, config rabbitmq.StreamConfig)` | Creates a new stream with the specified configuration (max age, max size, etc.) |
| `DeleteStream(ctx context.Context, streamName string)` | Deletes an existing stream and all its messages |

🔝 [back to top](#streams-package-api-reference)

&nbsp;

## Internal Methods

| Function | Description |
|----------|-------------|
| `ensureStreamExists(ch *amqp.Channel, streamName string)` | Ensures a stream exists, creating it with default settings if needed (called internally) |

🔝 [back to top](#streams-package-api-reference)

&nbsp;

## Types and Structures

| Type | Description |
|------|-------------|
| `Handler` | Main stream handler that implements the rabbitmq.StreamHandler interface for stream operations |

🔝 [back to top](#streams-package-api-reference)

&nbsp;

## Usage Examples

&nbsp;

### Basic Stream Operations

```go
import (
    "github.com/cloudresty/go-rabbitmq/streams"
    "github.com/cloudresty/go-rabbitmq"
)

// Create RabbitMQ client
client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Create stream handler
handler := streams.NewHandler(client)

// Create a stream with custom configuration
config := rabbitmq.StreamConfig{
    MaxAge:            24 * time.Hour,   // Retain messages for 24 hours
    MaxLengthMessages: 1000000,          // Maximum 1M messages
    MaxLengthBytes:    1024 * 1024 * 1024, // Maximum 1GB
}

err = handler.CreateStream(ctx, "events-stream", config)
if err != nil {
    log.Fatal(err)
}
```

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

🔝 [back to top](#streams-package-api-reference)

&nbsp;

### Event Sourcing Pattern

```go
// Event sourcing with streams
type EventStore struct {
    handler *streams.Handler
}

func NewEventStore(client *rabbitmq.Client) *EventStore {
    return &EventStore{
        handler: streams.NewHandler(client),
    }
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
```

🔝 [back to top](#streams-package-api-reference)

&nbsp;

### Time-Series Data Streaming

```go
// Time-series data publishing
type MetricsCollector struct {
    handler    *streams.Handler
    streamName string
}

func NewMetricsCollector(client *rabbitmq.Client, streamName string) *MetricsCollector {
    handler := streams.NewHandler(client)

    // Create stream with time-based retention
    config := rabbitmq.StreamConfig{
        MaxAge: 7 * 24 * time.Hour, // Keep 7 days of metrics
    }
    handler.CreateStream(context.Background(), streamName, config)

    return &MetricsCollector{
        handler:    handler,
        streamName: streamName,
    }
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

🔝 [back to top](#streams-package-api-reference)

&nbsp;

### Stream Management

```go
// Create multiple streams with different configurations
streams := map[string]rabbitmq.StreamConfig{
    "user-events": {
        MaxAge:            30 * 24 * time.Hour, // 30 days
        MaxLengthMessages: 5000000,             // 5M messages
    },
    "order-events": {
        MaxAge:         7 * 24 * time.Hour, // 7 days
        MaxLengthBytes: 2 * 1024 * 1024 * 1024, // 2GB
    },
    "metrics": {
        MaxAge: 24 * time.Hour, // 1 day
    },
}

handler := streams.NewHandler(client)

// Create all streams
for streamName, config := range streams {
    err := handler.CreateStream(ctx, streamName, config)
    if err != nil {
        log.Printf("Failed to create stream %s: %v", streamName, err)
    } else {
        log.Printf("Created stream: %s", streamName)
    }
}

// Later, clean up streams if needed
for streamName := range streams {
    err := handler.DeleteStream(ctx, streamName)
    if err != nil {
        log.Printf("Failed to delete stream %s: %v", streamName, err)
    }
}
```

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

🔝 [back to top](#streams-package-api-reference)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
