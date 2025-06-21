# Go RabbitMQ Package

A reusable Go package for RabbitMQ operations including publishing and consuming messages.

## Features

- Simple Publisher for sending messages
- Flexible Consumer for receiving messages
- Connection management with retry logic
- Support for different exchangeThe package includes several comprehensive examples demonstrating different features:

- **`examples/publisher/`** - Basic message publishing
- **`examples/consumer/`** - Basic message consuming
- **`examples/advanced/`** - Advanced patterns with custom exchanges and queues
- **`examples/connection-names/`** - Connection naming for better monitoring
- **`examples/reconnection-test/`** - Auto-reconnection behavior demonstration
- **`examples/production-queues/`** - Production-ready queue configurations (quorum, HA)
- **`examples/dead-letter-queues/`** - Automatic dead letter infrastructure setup
- **`examples/ulid-messages/`** - ULID-based message IDs with correlation patterns
- **`examples/ulid-verification/`** - ULID format analysis and verification

Run any example:

```bash
go run examples/dead-letter-queues/main.go
go run examples/ulid-messages/main.go
go run examples/production-queues/main.go
go run examples/ulid-verification/main.go
```

## Acknowledgment handling

- **ULID-based Message IDs** - Universally Unique Lexicographically Sortable Identifiers for optimal database performance
- **High-performance structured logging** with automatic PII/sensitive data masking
- Zero-allocation logging using emit library
- Comprehensive error handling and monitoring

## ULID Message IDs

This package uses [ULID (Universally Unique Lexicographically Sortable Identifier)](https://github.com/cloudresty/ulid) for all message IDs, providing significant advantages over traditional UUIDs:

### Benefits

- **🚀 6x Faster Generation** - ~150ns per ULID vs ~800ns for UUID v4
- **📊 Database Optimized** - Sequential inserts reduce B-tree fragmentation
- **🔢 Lexicographically Sortable** - Natural time-based ordering
- **📦 Compact** - 26 characters vs UUID's 36 characters (28% smaller)
- **🌐 URL Safe** - No special characters, no encoding needed
- **🔒 Collision Resistant** - 1.21e+24 unique IDs per millisecond
- **📈 Better Cache Performance** - Time-ordered data improves locality

### ULID Format

```text
01ARZ3NDEKTSV4RRFFQ69G5FAV
|-----------|  |-------------|
  Timestamp      Randomness
   48bits         80bits
```

### Usage Examples

```go
// Auto-generated ULID message ID
message := rabbitmq.NewMessage([]byte(`{"order_id": "12345"}`))
// message.MessageID will be a ULID like: 06bs864k6ss3s12tsqaknhy6y8

// Custom message ID (still ULID format recommended)
customUlid, _ := ulid.New()
message := rabbitmq.NewMessageWithID([]byte(`{"data": "value"}`), customUlid)

// Rich message with ULID correlation
message := rabbitmq.NewMessage([]byte(`{"event": "payment"}`)).
    WithCorrelationID("06bs864k6vsf4z56brhryn2kyr"). // ULID correlation ID
    WithType("payment.processed").
    WithHeader("trace_id", "06bs864k6ss3s12tsqaknhy6y8")
```

All message IDs are automatically generated as ULIDs unless explicitly overridden. See `examples/ulid-messages/` and `examples/ulid-verification/` for complete demonstrations.

## Installation

```bash
go get github.com/cloudresty/go-rabbitmq
```

## Usage

### Publisher

```go
package main

import (
    "context"
    "os"

    "github.com/cloudresty/emit"
    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create publisher
    publisher, err := rabbitmq.NewPublisher("amqp://localhost:5672")
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

### Consumer

```go
package main

import (
    "context"
    "os"

    "github.com/cloudresty/emit"
    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create consumer
    consumer, err := rabbitmq.NewConsumer("amqp://localhost:5672")
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

### Custom Connection Names

You can set custom connection names to identify your applications in the RabbitMQ management console:

```go
// Publisher with custom connection name
publisherConfig := rabbitmq.PublisherConfig{
    ConnectionConfig: rabbitmq.ConnectionConfig{
        URL:            "amqp://localhost:5672",
        ConnectionName: "order-service-publisher", // Visible in RabbitMQ console
    },
}
publisher, err := rabbitmq.NewPublisherWithConfig(publisherConfig)

// Consumer with custom connection name
consumerConfig := rabbitmq.ConsumerConfig{
    ConnectionConfig: rabbitmq.ConnectionConfig{
        URL:            "amqp://localhost:5672",
        ConnectionName: "order-service-consumer", // Visible in RabbitMQ console
    },
}
consumer, err := rabbitmq.NewConsumerWithConfig(consumerConfig)
```

**Default Connection Names:**

- Publisher: `go-rabbitmq-publisher`
- Consumer: `go-rabbitmq-consumer`

### Auto-Reconnection

The package includes automatic reconnection functionality to handle network interruptions:

```go
config := rabbitmq.ConnectionConfig{
    URL:                  "amqp://localhost:5672",
    ConnectionName:       "my-service",
    AutoReconnect:        true,        // Enable auto-reconnection (default: true)
    ReconnectDelay:       time.Second * 5, // Wait between attempts (default: 5s)
    MaxReconnectAttempts: 10,          // Max attempts (0 = unlimited, default: 0)
    Heartbeat:           time.Second * 10, // Connection keepalive (default: 10s)
}
```

**Features:**

- **Automatic Detection**: Monitors connection health via heartbeats
- **Intelligent Retry**: Configurable delay and maximum attempts
- **Graceful Recovery**: Seamless reconnection without data loss
- **Production Ready**: Handles network issues, server restarts, etc.

### Production-Ready Queues

The package provides built-in support for production-ready queue configurations optimized for RabbitMQ clusters:

```go
// Quorum Queue (Recommended for HA clusters)
err := publisher.DeclareQuorumQueue("orders")

// HA Classic Queue (For backward compatibility)
err := consumer.DeclareHAQueue("notifications")

// Custom Quorum Queue with specific settings
config := rabbitmq.QueueConfig{
    Name:               "payments",
    QueueType:          rabbitmq.QueueTypeQuorum,
    ReplicationFactor:  5,                    // 5-node quorum
    MaxLength:          100000,               // Message limit
    MessageTTL:         int(time.Hour * 24),  // 24-hour TTL
    // Dead Letter Infrastructure (enabled by default)
    AutoCreateDLX:      true,                 // Auto-create DLX and DLQ
    DLXSuffix:          ".dlx",              // DLX naming: payments.dlx
    DLQSuffix:          ".dlq",              // DLQ naming: payments.dlq
    DLQMessageTTL:      7 * 24 * 60 * 60 * 1000, // 7-day TTL in DLQ
}
err := publisher.DeclareQueueWithConfig(config)
```

**Queue Types:**

- **Quorum Queues**: Raft-based replicated queues (recommended for new applications)
  - Built-in replication and leader election
  - Configurable replication factor (default: 3)
  - Better performance and safety than HA classic queues

- **HA Classic Queues**: Mirror-based replicated queues (for compatibility)
  - Automatic mirroring across cluster nodes
  - Compatible with older RabbitMQ features

**Production Features:**

- **Durability**: All queues are durable by default
- **High Availability**: Automatic replication across cluster nodes
- **Message Persistence**: Messages survive broker restarts
- **Automatic Dead Letter Infrastructure**: Built-in DLX and DLQ creation
- **Size Limits**: Configure max length and byte limits
- **TTL Support**: Automatic message expiration

### Dead Letter Infrastructure

The package automatically creates dead letter exchanges (DLX) and dead letter queues (DLQ) for production safety:

```go
// Default: Auto-creates payments.dlx and payments.dlq
config := rabbitmq.DefaultQuorumQueueConfig("payments")
// config.AutoCreateDLX = true (enabled by default)

// Disable dead letter infrastructure
config.WithoutDeadLetter()

// Custom dead letter settings
config.WithDeadLetter(".dlx", ".dead", 3) // 3-day TTL in DLQ

// Use existing DLX (manual setup)
config.WithCustomDeadLetter("my-dlx", "failed.routing")
```

**Dead Letter Benefits:**

- **Message Safety**: Failed messages are preserved for debugging
- **Automatic Setup**: No manual DLX/DLQ creation needed
- **Production Ready**: Proper replication and TTL configuration
- **Operational Visibility**: Easy identification of processing failures
- **Flexible Configuration**: Enable/disable per queue as needed

## Configuration

The package supports various configuration options for both publishers and consumers:

- Connection retry logic
- Exchange and queue declarations
- Message persistence
- Consumer acknowledgment modes
- Dead letter exchanges

## Examples

The package includes several comprehensive examples demonstrating different features:

- **`examples/publisher/`** - Basic message publishing
- **`examples/consumer/`** - Basic message consuming
- **`examples/advanced/`** - Advanced patterns with custom exchanges and queues
- **`examples/connection-names/`** - Connection naming for better monitoring
- **`examples/reconnection-test/`** - Auto-reconnection behavior demonstration
- **`examples/production-queues/`** - Production-ready queue configurations (quorum, HA)
- **`examples/ulid-messages/`** - ULID-based message IDs with correlation patterns
- **`examples/ulid-verification/`** - ULID format analysis and verification

Run any example:

```bash
go run examples/ulid-messages/main.go
go run examples/production-queues/main.go
go run examples/ulid-verification/main.go
```

## Logging

This package uses the [emit](https://github.com/cloudresty/emit) library for high-performance structured logging. Key features:

- **Zero-allocation logging**: Uses `emit.ZString`, `emit.ZInt`, etc. for optimal performance
- **Automatic PII protection**: Sensitive data like connection URLs are automatically sanitized
- **Structured fields**: All log entries include relevant context for better observability
- **Performance-first**: Designed for high-throughput production environments

### Logging Examples

```go
// Error logging with context
emit.Error.StructuredFields("Failed to publish message",
    emit.ZString("exchange", exchangeName),
    emit.ZString("routing_key", routingKey),
    emit.ZString("error", err.Error()))

// Info logging with metrics
emit.Info.StructuredFields("Message published successfully",
    emit.ZString("exchange", exchangeName),
    emit.ZInt("message_size", len(message)),
    emit.ZDuration("duration", time.Since(start)))
```

## Requirements

- Go 1.24+
- RabbitMQ server

## Contributing

We welcome contributions to improve this package! Please follow these guidelines:

### Getting Started

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add or update tests as needed
5. Ensure all tests pass (`make test`)
6. Run linting (`make lint`)
7. Commit your changes (`git commit -m 'Add amazing feature'`)
8. Push to the branch (`git push origin feature/amazing-feature`)
9. Open a Pull Request

### Development Setup

```bash
# Clone your fork
git clone https://github.com/yourusername/go-rabbitmq.git
cd go-rabbitmq

# Install dependencies
go mod download

# Start RabbitMQ for testing
make docker-rabbitmq

# Run tests
make test

# Run integration tests
make test-integration
```

### Code Guidelines

- Follow Go conventions and best practices
- Use `emit` for all logging (no `log` or `fmt` for logging)
- Include comprehensive tests for new features
- Update documentation as needed
- Maintain backwards compatibility where possible

### Reporting Issues

Please use GitHub Issues to report bugs or request features. Include:

- Go version
- RabbitMQ version
- Detailed description of the issue
- Steps to reproduce
- Expected vs actual behavior

## License

This project is licensed under the MIT License - see the [LICENSE.txt](LICENSE.txt) file for details.

---

Made with ❤️ by [Cloudresty](https://cloudresty.com)
