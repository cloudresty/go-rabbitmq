# Examples

[Home](../README.md) &nbsp;/&nbsp; [Docs](README.md) &nbsp;/&nbsp; Examples

&nbsp;

This document provides comprehensive examples demonstrating different features of the go-rabbitmq package.

&nbsp;

## Available Examples

The package includes several examples in the `examples/` directory:

- **`publisher/`** - Basic message publishing using environment variables
- **`consumer/`** - Basic message consuming using environment variables
- **`advanced/`** - Advanced patterns with custom exchanges and queues
- **`connection-names/`** - Connection naming for better monitoring
- **`reconnection-test/`** - Auto-reconnection behavior demonstration
- **`production-queues/`** - Production-ready queue configurations (quorum, HA)
- **`dead-letter-queues/`** - Automatic dead letter infrastructure setup
- **`ulid-messages/`** - ULID-based message IDs with correlation patterns
- **`ulid-verification/`** - ULID format analysis and verification
- **`timeout-demo/`** - Timeout configuration and behavior demonstration
- **`graceful-shutdown/`** - Comprehensive graceful shutdown patterns
- **`env-config/`** - Environment variable configuration examples

🔝 [back to top](#examples)

&nbsp;

## Quick Start Examples

&nbsp;

### Basic Publisher

```go
package main

import (
    "context"
    "os"

    "github.com/cloudresty/emit"
    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create publisher from environment variables
    publisher, err := rabbitmq.NewPublisher()
    if err != nil {
        emit.Error.StructuredFields("Failed to create publisher",
            emit.ZString("error", err.Error()))
        os.Exit(1)
    }
    defer publisher.Close()

    // Publish a message
    err = publisher.Publish(context.Background(), rabbitmq.PublishConfig{
        Exchange:   "my-exchange",
        RoutingKey: "my-routing-key",
        Message:    []byte("Hello World!"),
    })
    if err != nil {
        emit.Error.StructuredFields("Failed to publish message",
            emit.ZString("error", err.Error()))
        os.Exit(1)
    }

    emit.Info.Msg("Message published successfully")
}
```

🔝 [back to top](#examples)

&nbsp;

### Basic Consumer

```go
package main

import (
    "context"
    "os"

    "github.com/cloudresty/emit"
    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create consumer from environment variables
    consumer, err := rabbitmq.NewConsumer()
    if err != nil {
        emit.Error.StructuredFields("Failed to create consumer",
            emit.ZString("error", err.Error()))
        os.Exit(1)
    }
    defer consumer.Close()

    // Start consuming
    err = consumer.Consume(context.Background(), rabbitmq.ConsumeConfig{
        Queue: "my-queue",
        Handler: func(ctx context.Context, message []byte) error {
            emit.Info.StructuredFields("Received message",
                emit.ZString("message", string(message)),
                emit.ZInt("size_bytes", len(message)))
            return nil
        },
    })
    if err != nil {
        emit.Error.StructuredFields("Consumer error",
            emit.ZString("error", err.Error()))
        os.Exit(1)
    }
}
```

🔝 [back to top](#examples)

&nbsp;

## Running Examples

All examples use environment variables by default:

```bash
# Set environment variables (optional, defaults will be used)
export RABBITMQ_HOST=localhost
export RABBITMQ_USERNAME=guest
export RABBITMQ_PASSWORD=guest

# Run examples
go run examples/publisher/main.go
go run examples/consumer/main.go
go run examples/env-config/main.go
```

🔝 [back to top](#examples)

&nbsp;

## Custom Connection Names

```go
// Using environment variables
export RABBITMQ_CONNECTION_NAME="order-service"

// Load from environment with custom name
publisher, err := rabbitmq.NewPublisher()
consumer, err := rabbitmq.NewConsumer()

// Or customize after loading from environment
envConfig, err := rabbitmq.LoadFromEnv()
envConfig.ConnectionName = "order-service-publisher"

publisherConfig := envConfig.ToPublisherConfig()
publisher, err := rabbitmq.NewPublisherWithConfig(publisherConfig)
```

🔝 [back to top](#examples)

&nbsp;

## Production Configuration

&nbsp;

### With Environment Variables

```go
// Set production environment
export RABBITMQ_HOST=prod-rabbitmq.company.com
export RABBITMQ_USERNAME=prod-user
export RABBITMQ_PASSWORD=secure-password
export RABBITMQ_VHOST=/production
export RABBITMQ_CONNECTION_NAME=payment-service
export RABBITMQ_HEARTBEAT=15s
export RABBITMQ_PUBLISHER_PERSISTENT=true
export RABBITMQ_CONSUMER_PREFETCH_COUNT=10

// Use in application
publisher, err := rabbitmq.NewPublisher()
consumer, err := rabbitmq.NewConsumer()
```

🔝 [back to top](#examples)

&nbsp;

### With Custom Configuration

```go
// Load from environment and customize
envConfig, err := rabbitmq.LoadFromEnv()
envConfig.ConnectionName = "payment-service-v2"
envConfig.HeartBeat = time.Second * 20
envConfig.ConsumerPrefetchCount = 5

publisherConfig := envConfig.ToPublisherConfig()
consumerConfig := envConfig.ToConsumerConfig()

publisher, err := rabbitmq.NewPublisherWithConfig(publisherConfig)
consumer, err := rabbitmq.NewConsumerWithConfig(consumerConfig)
```

🔝 [back to top](#examples)

&nbsp;

## Message Patterns

&nbsp;

### Request-Response

```go
// Publisher (Request)
publisher, _ := rabbitmq.NewPublisher()

correlationID, _ := ulid.New()
err := publisher.Publish(ctx, rabbitmq.PublishConfig{
    Exchange:      "requests",
    RoutingKey:    "payment.process",
    Message:       []byte(`{"amount": 100}`),
    CorrelationID: correlationID.String(),
    ReplyTo:       "payment.responses",
})

// Consumer (Response Handler)
consumer, _ := rabbitmq.NewConsumer()
err = consumer.Consume(ctx, rabbitmq.ConsumeConfig{
    Queue: "payment.responses",
    Handler: func(ctx context.Context, msg rabbitmq.Message) error {
        if msg.CorrelationID == correlationID.String() {
            emit.Info.StructuredFields("Received response",
                emit.ZString("correlation_id", msg.CorrelationID),
                emit.ZString("response", string(msg.Body)))
        }
        return nil
    },
})
```

🔝 [back to top](#examples)

&nbsp;

### Event Publishing

```go
publisher, _ := rabbitmq.NewPublisher()

events := []struct {
    Type string
    Data interface{}
}{
    {"user.created", map[string]string{"user_id": "123"}},
    {"order.placed", map[string]interface{}{"order_id": "456", "amount": 99.99}},
}

for _, event := range events {
    eventData, _ := json.Marshal(event.Data)

    err := publisher.Publish(ctx, rabbitmq.PublishConfig{
        Exchange:    "events",
        RoutingKey:  event.Type,
        Message:     eventData,
        ContentType: "application/json",
        Headers: map[string]interface{}{
            "event_type": event.Type,
            "timestamp":  time.Now().Unix(),
        },
    })
}
```

🔝 [back to top](#examples)

&nbsp;

### Dead Letter Handling

```go
// Setup queue with dead letter support
publisher, _ := rabbitmq.NewPublisher()

config := rabbitmq.DefaultQuorumQueueConfig("orders")
config.WithDeadLetter(".dlx", ".failed", 7) // 7-day TTL in DLQ

err := publisher.DeclareQueueWithConfig(config)

// Consumer with error handling
consumer, _ := rabbitmq.NewConsumer()
err = consumer.Consume(ctx, rabbitmq.ConsumeConfig{
    Queue: "orders",
    Handler: func(ctx context.Context, msg rabbitmq.Message) error {
        // Process message
        if err := processOrder(msg.Body); err != nil {
            emit.Error.StructuredFields("Failed to process order",
                emit.ZString("message_id", msg.MessageID),
                emit.ZString("error", err.Error()))
            // Return error to send to dead letter queue
            return err
        }
        return nil // Success
    },
})
```

🔝 [back to top](#examples)

&nbsp;

## Error Handling

&nbsp;

### Connection Errors

```go
publisher, err := rabbitmq.NewPublisher()
if err != nil {
    if connErr, ok := err.(*rabbitmq.ConnectionError); ok {
        emit.Error.StructuredFields("Connection failed",
            emit.ZString("error", connErr.Error()),
            emit.ZString("cause", connErr.Cause.Error()))
        // Handle connection-specific error
    }
}
```

🔝 [back to top](#examples)

&nbsp;

### Publish Errors

```go
err := publisher.Publish(ctx, config)
if err != nil {
    if pubErr, ok := err.(*rabbitmq.PublishError); ok {
        emit.Error.StructuredFields("Publish failed",
            emit.ZString("error", pubErr.Error()),
            emit.ZString("exchange", config.Exchange),
            emit.ZString("routing_key", config.RoutingKey))
        // Handle publish-specific error
    }
}
```

🔝 [back to top](#examples)

&nbsp;

### Consumer Errors

```go
err = consumer.Consume(ctx, rabbitmq.ConsumeConfig{
    Queue: "orders",
    Handler: func(ctx context.Context, msg rabbitmq.Message) error {
        // Return different error types for different handling
        if isTemporaryError(err) {
            return rabbitmq.NewConsumeError("temporary failure", err)
        }
        if isPermanentError(err) {
            // Log and acknowledge (don't retry)
            emit.Error.StructuredFields("Permanent error, discarding message",
                emit.ZString("message_id", msg.MessageID))
            return nil
        }
        return err // Will be retried
    },
})
```

🔝 [back to top](#examples)

&nbsp;

## Development and Testing

### Running Tests

```bash
# Unit tests only
make test

# Integration tests (requires RabbitMQ)
make test-integration

# Start RabbitMQ in Docker
make docker-rabbitmq

# Run specific example
make run-publisher
make run-consumer
make run-advanced
```

🔝 [back to top](#examples)

&nbsp;

### Building Examples

```bash
# Build all examples
make build

# Format and lint
make fmt
make lint

# Full CI pipeline
make ci
```

🔝 [back to top](#examples)

&nbsp;

### Testing Best Practices

- **Use Docker for integration tests** - Consistent test environment
- **Mock connections for unit tests** - Fast, isolated testing
- **Test error scenarios and edge cases** - Ensure robust error handling
- **Test with realistic message volumes** - Performance validation
- **Test reconnection scenarios** - Network resilience validation

🔝 [back to top](#examples)

&nbsp;

For more detailed examples, see the [`examples/`](../examples/) directory in the repository.

&nbsp;

---

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
