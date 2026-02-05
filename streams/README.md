# Streams Package

[Home](../README.md) &nbsp;/&nbsp; Streams Package

&nbsp;

The `streams` package provides RabbitMQ Streams functionality for high-throughput messaging scenarios. RabbitMQ Streams are a persistent, replicated data structure introduced in RabbitMQ 3.9+ that offers exceptional performance for event streaming, time-series data, and cases where message order and durability are critical.

This package uses the **native RabbitMQ stream protocol** (port 5552) which provides **5-10x better performance** compared to AMQP 0.9.1 for stream operations.

&nbsp;

## Features

- **Native Stream Protocol**: Uses RabbitMQ's binary stream protocol (port 5552) for maximum performance
- **Sub-Millisecond Latency**: Optimized binary protocol for ultra-low latency publishing
- **High-Throughput**: Capable of 100k+ messages/second
- **Durable Message Storage**: Persistent, replicated storage with configurable retention policies
- **Stream Configuration**: Full control over stream behavior (max age, size limits, clustering)
- **Consumer Offset Management**: Built-in support for consumer positioning and replay
- **Pluggable Interface**: Implements `rabbitmq.StreamHandler` interface for easy testing and extensibility

&nbsp;

🔝 [back to top](#streams-package)

&nbsp;

## Installation

```bash
go get github.com/cloudresty/go-rabbitmq/streams
```

&nbsp;

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
    // Create streams handler using native stream protocol (port 5552)
    // This provides 5-10x better performance compared to AMQP 0.9.1
    handler, err := streams.NewHandler(streams.Options{
        Host:     "localhost",
        Port:     5552,  // Native stream protocol port
        Username: "guest",
        Password: "guest",
    })
    if err != nil {
        log.Fatal("Failed to create streams handler:", err)
    }
    defer handler.Close()

    // Create a stream with configuration
    streamConfig := rabbitmq.StreamConfig{
        MaxAge:            24 * time.Hour,     // Retain for 24 hours
        MaxLengthBytes:    1024 * 1024 * 1024, // Max 1GB storage
    }

    err = handler.CreateStream(context.Background(), "events.stream", streamConfig)
    if err != nil {
        log.Printf("Stream creation: %v", err)
    }

    // Publish messages (high-throughput)
    for i := 0; i < 100; i++ {
        message := rabbitmq.NewMessage([]byte(fmt.Sprintf("Event %d", i)))
        message.MessageID = fmt.Sprintf("msg-%d", i)

        err = handler.PublishToStream(context.Background(), "events.stream", message)
        if err != nil {
            log.Printf("Failed to publish: %v", err)
        }
    }

    // Consume messages
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    err = handler.ConsumeFromStream(ctx, "events.stream", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        fmt.Printf("Received: %s\n", delivery.Body)
        return nil
    })
    if err != nil {
        log.Printf("Consumption ended: %v", err)
    }
}
```

&nbsp;

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

&nbsp;

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

&nbsp;

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

&nbsp;

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

&nbsp;

🔝 [back to top](#streams-package)

&nbsp;

## Connection Options

### Basic Connection

```go
// Create streams handler with default options
handler, err := streams.NewHandler(streams.Options{
    Host:     "localhost",
    Port:     5552,  // Native stream protocol port
    Username: "guest",
    Password: "guest",
})
if err != nil {
    log.Fatal("Failed to create streams handler:", err)
}
defer handler.Close()
```

&nbsp;

🔝 [back to top](#streams-package)

&nbsp;

### Advanced Connection Options

```go
// Create streams handler with advanced options
handler, err := streams.NewHandler(streams.Options{
    Host:               "rabbitmq.example.com",
    Port:               5552,
    Username:           "myuser",
    Password:           "mypassword",
    VHost:              "/production",
    MaxProducers:       10,              // Max producers per connection
    MaxConsumers:       10,              // Max consumers per connection
    RequestedHeartbeat: 30 * time.Second,
})
if err != nil {
    log.Fatal("Failed to create streams handler:", err)
}
defer handler.Close()

// Use the handler for all stream operations
err = handler.CreateStream(ctx, "my.stream", rabbitmq.StreamConfig{
    MaxAge: 24 * time.Hour,
})
```

&nbsp;

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

&nbsp;

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

&nbsp;

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

&nbsp;

🔝 [back to top](#streams-package)

&nbsp;

## Best Practices

### Stream Design

1. **Naming Convention**: Use hierarchical names like `domain.entity.events`
2. **Configuration**: Set appropriate retention policies based on your use case
3. **Partitioning**: Consider using multiple streams for different event types
4. **Monitoring**: Implement proper logging and metrics collection

&nbsp;

🔝 [back to top](#streams-package)

&nbsp;

### Message Design

1. **Content Type**: Always set appropriate content type (e.g., "application/json")
2. **Message ID**: Use unique, meaningful message IDs for tracking (automatically preserved through streams with backup header mechanism)
3. **Headers**: Include relevant metadata in headers for routing and filtering
4. **Size**: Keep messages reasonably sized; use external storage for large payloads

&nbsp;

🔝 [back to top](#streams-package)

&nbsp;

### Consumer Design

1. **Error Handling**: Implement proper error handling and recovery
2. **Idempotency**: Design message handlers to be idempotent
3. **Timeouts**: Use appropriate context timeouts
4. **Graceful Shutdown**: Handle shutdown signals properly

&nbsp;

🔝 [back to top](#streams-package)

&nbsp;

### Production Considerations

1. **High Availability**: Use `InitialClusterSize` > 1 for critical streams
2. **Retention**: Set appropriate `MaxAge`, `MaxLengthMessages`, and `MaxLengthBytes`
3. **Monitoring**: Monitor stream metrics and consumer lag
4. **Backpressure**: Implement backpressure handling for high-volume scenarios

&nbsp;

🔝 [back to top](#streams-package)

&nbsp;

## Examples

For complete working examples, see:

- [streams-unified](../examples/streams-unified/main.go) - Comprehensive streams usage example
- [ulid-messages](../examples/ulid-messages/main.go) - Using streams with ULID message IDs
- [ulid-verification](../examples/ulid-verification/main.go) - Message verification with streams

&nbsp;

🔝 [back to top](#streams-package)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

<sub>&copy; Cloudresty - All rights reserved</sub>

&nbsp;
