# Streams Package

[Home](../README.md) &nbsp;/&nbsp; Streams Package

&nbsp;

The `streams` package provides RabbitMQ Streams functionality for high-throughput messaging scenarios. RabbitMQ Streams are a persistent, replicated data structure introduced in RabbitMQ 3.9+ that offers exceptional performance for event streaming, time-series data, and cases where message order and durability are critical.

&nbsp;

## Features

- **High-Throughput Publishing**: Optimized for high-volume message publishing scenarios
- **Durable Message Storage**: Persistent, replicated storage with configurable retention policies
- **Stream Configuration**: Full control over stream behavior (max age, size limits, clustering)
- **Consumer Offset Management**: Built-in support for consumer positioning and replay
- **Auto-Creation**: Automatic stream creation with sensible defaults when needed
- **Pluggable Interface**: Clean contract-implementation pattern for easy testing and extensibility

🔝 [back to top](#streams-package)

&nbsp;

## Installation

```bash
go get github.com/cloudresty/go-rabbitmq/streams
```

🔝 [back to top](#streams-package)

&nbsp;

## Quick Start

### Basic Stream Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/streams"
)

func main() {
    // Create RabbitMQ client
    client, err := rabbitmq.NewClient(
        rabbitmq.WithHosts("localhost:5672"),
        rabbitmq.WithCredentials("guest", "guest"),
    )
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Create streams handler
    streamsHandler := streams.NewHandler(client)

    // Create a stream with configuration
    streamConfig := rabbitmq.StreamConfig{
        MaxAge:            24 * time.Hour,     // Retain for 24 hours
        MaxLengthMessages: 1_000_000,          // Max 1M messages
        MaxLengthBytes:    1024 * 1024 * 1024, // Max 1GB storage
    }

    err = streamsHandler.CreateStream(context.Background(), "events.stream", streamConfig)
    if err != nil {
        log.Printf("Stream creation: %v", err)
    }

    // Publish messages
    for i := 0; i < 100; i++ {
        message := rabbitmq.NewMessage([]byte(fmt.Sprintf("Event %d", i)))
        message.MessageID = fmt.Sprintf("msg-%d", i)

        err = streamsHandler.PublishToStream(context.Background(), "events.stream", message)
        if err != nil {
            log.Printf("Failed to publish: %v", err)
        }
    }

    // Consume messages
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    err = streamsHandler.ConsumeFromStream(ctx, "events.stream", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        fmt.Printf("Received: %s\n", delivery.Body)
        return nil
    })
    if err != nil {
        log.Printf("Consumption ended: %v", err)
    }
}
```

🔝 [back to top](#streams-package)

&nbsp;

## Advanced Usage

### Stream Configuration Options

The `StreamConfig` struct provides comprehensive control over stream behavior:

```go
streamConfig := rabbitmq.StreamConfig{
    // Retention by time
    MaxAge: 7 * 24 * time.Hour, // Retain for 7 days

    // Retention by message count
    MaxLengthMessages: 10_000_000, // Max 10M messages

    // Retention by size
    MaxLengthBytes: 10 * 1024 * 1024 * 1024, // Max 10GB

    // Segment configuration (advanced)
    MaxSegmentSizeBytes: 500 * 1024 * 1024, // 500MB segments

    // Clustering
    InitialClusterSize: 3, // Replicate across 3 nodes
}

err := streamsHandler.CreateStream(ctx, "high-volume.stream", streamConfig)
```

🔝 [back to top](#streams-package)

&nbsp;

### Message Publishing with Metadata

```go
// Create message with comprehensive metadata
message := rabbitmq.NewMessage([]byte(`{"event": "user_signup", "user_id": 12345}`))
message.ContentType = "application/json"
message.MessageID = "signup-12345"
message.Headers = map[string]interface{}{
    "event_type":   "user_action",
    "source":       "web_app",
    "version":      "1.0",
    "correlation_id": "abc-123-def",
}

err := streamsHandler.PublishToStream(ctx, "user.events", message)
if err != nil {
    log.Printf("Failed to publish event: %v", err)
}
```

🔝 [back to top](#streams-package)

&nbsp;

### Consumer with Error Handling

```go
messageHandler := func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    // Extract message metadata
    eventType := delivery.Headers["event_type"]
    correlationID := delivery.Headers["correlation_id"]

    log.Printf("Processing event: %s (correlation: %v)", eventType, correlationID)

    // Process the message
    var event UserEvent
    if err := json.Unmarshal(delivery.Body, &event); err != nil {
        log.Printf("Failed to unmarshal event: %v", err)
        return err // This will cause consumption to stop
    }

    // Business logic
    if err := processUserEvent(event); err != nil {
        log.Printf("Failed to process event: %v", err)
        return err
    }

    log.Printf("Successfully processed event: %s", delivery.MessageId)
    return nil
}

// Start consuming with proper error handling
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

err := streamsHandler.ConsumeFromStream(ctx, "user.events", messageHandler)
if err != nil {
    if err == context.Canceled {
        log.Println("Consumption cancelled")
    } else {
        log.Printf("Consumption error: %v", err)
    }
}
```

🔝 [back to top](#streams-package)

&nbsp;

### Stream Management Operations

```go
// Create multiple streams with different configurations
streams := map[string]rabbitmq.StreamConfig{
    "events.critical": {
        MaxAge:            30 * 24 * time.Hour, // 30 days
        MaxLengthMessages: 50_000_000,
        InitialClusterSize: 5, // High availability
    },
    "events.logs": {
        MaxAge:         3 * 24 * time.Hour, // 3 days
        MaxLengthBytes: 1024 * 1024 * 1024, // 1GB
    },
    "events.metrics": {
        MaxLengthMessages: 1_000_000, // Rolling window
    },
}

for streamName, config := range streams {
    err := streamsHandler.CreateStream(ctx, streamName, config)
    if err != nil {
        log.Printf("Failed to create stream %s: %v", streamName, err)
    }
}

// Clean up streams when no longer needed
defer func() {
    for streamName := range streams {
        if err := streamsHandler.DeleteStream(ctx, streamName); err != nil {
            log.Printf("Failed to delete stream %s: %v", streamName, err)
        }
    }
}()
```

🔝 [back to top](#streams-package)

&nbsp;

## Integration with Client

### Using with Client Options

```go
// Create client with streams handler
client, err := rabbitmq.NewClient(
    rabbitmq.WithHosts("localhost:5672"),
    rabbitmq.WithCredentials("guest", "guest"),
    rabbitmq.WithStreamHandler(streams.NewHandler(nil)), // Note: pass nil, client will be set internally
)
if err != nil {
    log.Fatal("Failed to create client:", err)
}

// Access the streams handler through the client
// (if you need to access it directly from the client)
```

🔝 [back to top](#streams-package)

&nbsp;

### Manual Handler Creation

```go
// Create client first
client, err := rabbitmq.NewClient(
    rabbitmq.WithHosts("localhost:5672"),
    rabbitmq.WithCredentials("guest", "guest"),
)
if err != nil {
    log.Fatal("Failed to create client:", err)
}

// Create streams handler manually
streamsHandler := streams.NewHandler(client)

// Use the handler for all stream operations
err = streamsHandler.CreateStream(ctx, "my.stream", rabbitmq.StreamConfig{
    MaxAge: 24 * time.Hour,
})
```

🔝 [back to top](#streams-package)

&nbsp;

## Performance Considerations

### High-Throughput Publishing

```go
// For maximum throughput, consider batching
messages := make([]*rabbitmq.Message, 100)
for i := 0; i < 100; i++ {
    messages[i] = rabbitmq.NewMessage([]byte(fmt.Sprintf("batch-message-%d", i)))
}

// Publish in batch (conceptual - implement based on your needs)
for _, message := range messages {
    if err := streamsHandler.PublishToStream(ctx, "high-volume.stream", message); err != nil {
        log.Printf("Failed to publish: %v", err)
        break
    }
}
```

🔝 [back to top](#streams-package)

&nbsp;

### Resource Management

```go
// Always use context with timeouts for operations
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// For long-running consumers, use cancellable contexts
ctx, cancel := context.WithCancel(context.Background())

// Handle graceful shutdown
go func() {
    <-shutdownSignal
    cancel() // This will stop the consumer
}()

err := streamsHandler.ConsumeFromStream(ctx, "stream", messageHandler)
```

🔝 [back to top](#streams-package)

&nbsp;

## Error Handling

### Common Error Scenarios

```go
// Handle stream creation errors
err := streamsHandler.CreateStream(ctx, "my.stream", config)
if err != nil {
    if strings.Contains(err.Error(), "already exists") {
        log.Println("Stream already exists, continuing...")
    } else {
        log.Fatalf("Failed to create stream: %v", err)
    }
}

// Handle publishing errors
err = streamsHandler.PublishToStream(ctx, "my.stream", message)
if err != nil {
    if strings.Contains(err.Error(), "connection closed") {
        log.Println("Connection lost, implementing retry logic...")
        // Implement retry with backoff
    } else {
        log.Printf("Publish failed: %v", err)
    }
}

// Handle consumption errors
err = streamsHandler.ConsumeFromStream(ctx, "my.stream", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    // Return errors to stop consumption
    if criticalError := processMessage(delivery); criticalError != nil {
        return criticalError // This will stop the consumer
    }
    return nil // Continue consuming
})

if err != nil && err != context.Canceled {
    log.Printf("Consumption error: %v", err)
}
```

🔝 [back to top](#streams-package)

&nbsp;

## Best Practices

### Stream Design

1. **Naming Convention**: Use hierarchical names like `domain.entity.events`
2. **Configuration**: Set appropriate retention policies based on your use case
3. **Partitioning**: Consider using multiple streams for different event types
4. **Monitoring**: Implement proper logging and metrics collection

🔝 [back to top](#streams-package)

&nbsp;

### Message Design

1. **Content Type**: Always set appropriate content type (e.g., "application/json")
2. **Message ID**: Use unique, meaningful message IDs for tracking (automatically preserved through streams with backup header mechanism)
3. **Headers**: Include relevant metadata in headers for routing and filtering
4. **Size**: Keep messages reasonably sized; use external storage for large payloads

🔝 [back to top](#streams-package)

&nbsp;

### Consumer Design

1. **Error Handling**: Implement proper error handling and recovery
2. **Idempotency**: Design message handlers to be idempotent
3. **Timeouts**: Use appropriate context timeouts
4. **Graceful Shutdown**: Handle shutdown signals properly

🔝 [back to top](#streams-package)

&nbsp;

### Production Considerations

1. **High Availability**: Use `InitialClusterSize` > 1 for critical streams
2. **Retention**: Set appropriate `MaxAge`, `MaxLengthMessages`, and `MaxLengthBytes`
3. **Monitoring**: Monitor stream metrics and consumer lag
4. **Backpressure**: Implement backpressure handling for high-volume scenarios

🔝 [back to top](#streams-package)

&nbsp;

## Examples

For complete working examples, see:

- [streams-unified](../examples/streams-unified/main.go) - Comprehensive streams usage example
- [ulid-messages](../examples/ulid-messages/main.go) - Using streams with ULID message IDs
- [ulid-verification](../examples/ulid-verification/main.go) - Message verification with streams

🔝 [back to top](#streams-package)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
