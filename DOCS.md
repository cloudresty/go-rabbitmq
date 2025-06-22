# Go RabbitMQ Package Documentation

## Overview

This package provides a comprehensive, production-ready RabbitMQ client library for Go applications. It offers simplified interfaces for both publishing and consuming messages while maintaining full control over RabbitMQ features.

## Features

### Core Features

- ✅ **Publisher**: Simple and reliable message publishing with confirmation support
- ✅ **Consumer**: Flexible message consumption with acknowledgment handling
- ✅ **Connection Management**: Automatic connection retry and health monitoring
- ✅ **Topology Management**: Easy exchange, queue, and binding setup
- ✅ **Error Handling**: Comprehensive error types and handling
- ✅ **Context Support**: Full context.Context integration for cancellation

### Advanced Features

- ✅ **Publisher Confirmations**: Reliable message delivery with acknowledgments
- ✅ **Consumer QoS**: Prefetch count and size control
- ✅ **Multiple Exchange Types**: Direct, Fanout, Topic, and Headers exchanges
- ✅ **Message Persistence**: Configurable message durability
- ✅ **Graceful Shutdown**: Proper resource cleanup and connection closing
- ✅ **Thread Safety**: Safe for concurrent use

## Quick Start

### 1. Installation

```bash
go get github.com/cloudresty/go-rabbitmq
```

### 2. Basic Publisher

```go
package main

import (
    "context"
    "os"

    "github.com/cloudresty/emit"
    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    publisher, err := rabbitmq.NewPublisher("amqp://localhost:5672")
    if err != nil {
        emit.Error.StructuredFields("Failed to create publisher",
            emit.ZString("error", err.Error()))
        os.Exit(1)
    }
    defer publisher.Close()

    err = publisher.Publish(context.Background(), rabbitmq.PublishConfig{
        Exchange:   "my-exchange",
        RoutingKey: "my-key",
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

### 3. Basic Consumer

```go
package main

import (
    "context"
    "os"

    "github.com/cloudresty/emit"
    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    consumer, err := rabbitmq.NewConsumer("amqp://localhost:5672")
    if err != nil {
        emit.Error.StructuredFields("Failed to create consumer",
            emit.ZString("error", err.Error()))
        os.Exit(1)
    }
    defer consumer.Close()

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

## Architecture

### Package Structure

```text
go-rabbitmq/
├── connection.go     # Connection management with retry logic
├── publisher.go      # Publisher implementation
├── consumer.go       # Consumer implementation
├── types.go          # Common types and utilities
├── rabbitmq_test.go  # Test suite
└── examples/         # Usage examples
    ├── publisher/    # Publisher example
    ├── consumer/     # Consumer example
    └── advanced/     # Advanced topology example
```

### Key Types

- **`Publisher`**: Handles message publishing
- **`Consumer`**: Handles message consumption
- **`Connection`**: Manages RabbitMQ connections
- **`PublishConfig`**: Configuration for publishing messages
- **`ConsumeConfig`**: Configuration for consuming messages
- **`MessageHandler`**: Function type for handling messages
- **`Message`**: Rich message structure with ULID-based IDs and metadata
- **`QueueConfig`**: Advanced queue configuration with automatic dead letter infrastructure
- **`ExchangeConfig`**: Exchange configuration for topology setup
- **`BindingConfig`**: Queue-to-exchange binding configuration

## ULID Message IDs

This package uses [ULID (Universally Unique Lexicographically Sortable Identifier)](https://github.com/cloudresty/ulid) for all message identification, providing significant performance and operational benefits over traditional UUID v4.

### Why ULID?

**Performance Benefits:**

- **6x faster generation** than UUID v4 (~150ns vs ~800ns)
- **Better database performance** due to sequential inserts
- **Reduced B-tree fragmentation** in database indexes
- **Improved cache locality** for time-related queries

**Format Benefits:**

- **26 characters** vs UUID's 36 characters (28% smaller)
- **URL-safe** - no special characters, no encoding needed
- **Case-insensitive** parsing for backward compatibility
- **Lexicographically sortable** - natural time-based ordering

**Operational Benefits:**

- **Time-based correlation** - easy to correlate events across services
- **Collision resistant** - 1.21e+24 unique IDs per millisecond
- **No coordination required** - safe in distributed systems
- **Human readable** - time prefix makes debugging easier

### ULID Structure

```text
01ARZ3NDEKTSV4RRFFQ69G5FAV
|-----------|  |-------------|
  Timestamp      Randomness
  48 bits        80 bits
```

- **Timestamp (48 bits)**: UNIX timestamp in milliseconds, provides time-based sorting
- **Randomness (80 bits)**: Cryptographically secure random value for uniqueness

### Usage Patterns

#### Auto-Generated Message IDs

All messages automatically receive ULID-based IDs:

```go
// Auto-generated ULID message ID
message := rabbitmq.NewMessage([]byte(`{"order_id": "12345"}`))
// message.MessageID = "06bs864k6ss3s12tsqaknhy6y8"

// Publish with automatic ULID
err := publisher.PublishMessage(ctx, rabbitmq.PublishMessageConfig{
    Exchange:   "orders",
    RoutingKey: "order.created",
    Message:    message,
})
```

#### Custom ULID Message IDs

For specific use cases requiring custom IDs:

```go
// Generate custom ULID
customUlid, err := ulid.New()
if err != nil {
    // Handle error
}

message := rabbitmq.NewMessageWithID([]byte(`{"data": "value"}`), customUlid)
```

#### Correlation with ULIDs

ULIDs are excellent for message correlation and tracing:

```go
// Parent message with ULID
parentId, _ := ulid.New()
parentMessage := rabbitmq.NewMessage([]byte(`{"action": "process_order"}`)).
    WithType("order.initiated")

// Child messages correlated to parent
for i, step := range []string{"validate", "reserve", "charge"} {
    childMessage := rabbitmq.NewMessage([]byte(fmt.Sprintf(`{"step": "%s"}`, step))).
        WithCorrelationID(parentId). // Correlate to parent
        WithType(fmt.Sprintf("order.%s", step)).
        WithHeader("sequence", i+1)

    // Publish child message...
}
```

#### Time-Range Queries

ULIDs enable efficient time-based queries:

```sql
-- Find all messages from the last hour (PostgreSQL example)
SELECT * FROM messages
WHERE message_id >= '06bs7y0000000000000000000000'  -- 1 hour ago
  AND message_id <= '06bs864k6ss3s12tsqaknhy6y8'   -- now
ORDER BY message_id;  -- Natural chronological order
```

### ULID vs UUID Comparison

| Feature | ULID | UUID v4 | UUID v1 |
|---------|------|---------|---------|
| **Generation Speed** | ~150ns ⚡ | ~800ns | ~600ns |
| **Size (string)** | 26 chars ⚡ | 36 chars | 36 chars |
| **Sortable** | ✅ Natural | ❌ Random | ⚠️ Complex |
| **URL Safe** | ✅ No encoding | ❌ Needs encoding | ❌ Needs encoding |
| **Database B-tree** | ✅ Sequential | ❌ Random | ⚠️ Partial |
| **Human Readable** | ✅ Time prefix | ❌ Opaque | ⚠️ MAC exposed |
| **Privacy** | ✅ Anonymous | ✅ Anonymous | ❌ MAC address |
| **Collision Risk** | Negligible | Negligible | Negligible |

### Migration from UUIDs

If migrating from existing UUID-based systems:

```go
// Legacy UUID publishing (not recommended for new code)
err := publisher.Publish(ctx, rabbitmq.PublishConfig{
    Exchange:      "orders",
    RoutingKey:    "order.legacy",
    Message:       []byte(`{"order_id": "12345"}`),
    MessageID:     uuid.New().String(), // Legacy UUID
    ContentType:   "application/json",
})

// Modern ULID publishing (recommended)
message := rabbitmq.NewMessage([]byte(`{"order_id": "12345"}`)).
    WithType("order.created")

err := publisher.PublishMessage(ctx, rabbitmq.PublishMessageConfig{
    Exchange:   "orders",
    RoutingKey: "order.created",
    Message:    message, // Auto ULID message ID
})
```

### ULID Performance Benefits

**High-Throughput Scenarios:**

- ULIDs generate 1.21e+24 unique IDs per millisecond
- No coordination needed across multiple publishers
- Sequential database inserts improve write performance
- Natural partitioning for time-series data

**Memory Efficiency:**

- Smaller string representation saves memory
- Better compression ratios due to time prefix
- Reduced storage I/O in high-volume scenarios

**Observability:**

- Easier debugging with time-based correlation
- Natural log aggregation and sorting
- Simplified distributed tracing

For complete examples, see:

- `examples/ulid-messages/main.go` - Comprehensive ULID usage patterns
- `examples/ulid-verification/main.go` - ULID format verification and analysis

## Dead Letter Infrastructure

The package provides automatic dead letter infrastructure creation for production-grade message handling. This critical feature ensures failed messages are preserved for debugging and prevents message loss.

### Why Dead Letter Infrastructure?

**Production Safety:**

- **Message Preservation**: Failed messages are automatically routed to dead letter queues
- **Debugging Aid**: Analyze processing failures without losing message data
- **Operational Visibility**: Easy identification of problematic messages
- **System Resilience**: Prevents queue blocking from repeatedly failing messages

**Best Practices Compliance:**

- **Industry Standard**: All production RabbitMQ deployments should have DLX/DLQ
- **Zero Configuration**: Works out of the box with sensible defaults
- **Flexible Control**: Enable/disable per queue as needed

### Automatic Infrastructure Creation

By default, the package automatically creates complete dead letter infrastructure:

```go
// Default behavior - auto-creates DLX and DLQ
config := rabbitmq.DefaultQuorumQueueConfig("orders")
// Creates:
//   - orders (main queue)
//   - orders.dlx (dead letter exchange)
//   - orders.dlq (dead letter queue)
//   - Binding: orders.dlx -> orders.dlq
```

**Infrastructure Components:**

1. **Dead Letter Exchange (DLX)**: Routes failed messages
2. **Dead Letter Queue (DLQ)**: Stores failed messages
3. **Automatic Binding**: Connects DLX to DLQ
4. **Main Queue Configuration**: Points to DLX via `x-dead-letter-exchange`

### Configuration Options

#### Default Configuration

All default queue configurations have dead letter infrastructure enabled:

```go
// Quorum queue with auto-DLX
quorumConfig := rabbitmq.DefaultQuorumQueueConfig("payments")

// HA classic queue with auto-DLX
haConfig := rabbitmq.DefaultHAQueueConfig("notifications")

// Basic classic queue with auto-DLX
classicConfig := rabbitmq.DefaultClassicQueueConfig("logs")
```

**Default Settings:**

- DLX Suffix: `.dlx`
- DLQ Suffix: `.dlq`
- DLQ Message TTL: 7 days
- DLQ Replication: Same as main queue

#### Custom Dead Letter Configuration

```go
config := rabbitmq.DefaultQuorumQueueConfig("orders")

// Custom suffixes and TTL
config.WithDeadLetter(".dlx", ".dead", 3) // 3-day TTL
// Creates: orders.dlx and orders.dead

// Disable dead letter infrastructure
config.WithoutDeadLetter()

// Use existing DLX (manual setup)
config.WithCustomDeadLetter("global-dlx", "failed.orders")
```

#### Advanced Configuration

```go
config := rabbitmq.QueueConfig{
    Name:          "critical-orders",
    QueueType:     rabbitmq.QueueTypeQuorum,
    // Dead letter settings
    AutoCreateDLX: true,
    DLXSuffix:     ".failures",
    DLQSuffix:     ".failed",
    DLQMaxLength:  50000,  // Limit DLQ size
    DLQMessageTTL: 30 * 24 * 60 * 60 * 1000, // 30 days
}
```

### Message Flow and Routing

**Normal Processing:**

```text
Producer -> Exchange -> Queue -> Consumer -> ACK
```

**Failed Processing:**

```text
Producer -> Exchange -> Queue -> Consumer -> NACK/Reject
           ↓
Dead Letter Exchange -> Dead Letter Queue -> Manual Review
```

**Triggering Dead Letter Routing:**

Messages are sent to DLX when:

- Consumer explicitly rejects (`NACK` with `requeue=false`)
- Message TTL expires in the queue
- Queue reaches maximum length
- Consumer connection drops during processing

### Topology Setup with Dead Letter

The `SetupTopology` function automatically handles dead letter infrastructure:

```go
exchanges := []rabbitmq.ExchangeConfig{
    {Name: "orders", Type: rabbitmq.ExchangeTypeDirect, Durable: true},
}

queues := []rabbitmq.QueueConfig{
    // Auto-creates orders.dlx and orders.dlq
    rabbitmq.DefaultQuorumQueueConfig("orders"),

    // Custom DLX configuration
    func() rabbitmq.QueueConfig {
        config := rabbitmq.DefaultQuorumQueueConfig("payments")
        return *config.WithDeadLetter(".dlx", ".failures", 14) // 14-day TTL
    }(),
}

// This creates ALL infrastructure automatically:
// 1. Main exchanges
// 2. Dead letter exchanges
// 3. Dead letter queues
// 4. Main queues (with DLX configuration)
// 5. All necessary bindings
err := rabbitmq.SetupTopology(connection, exchanges, queues, bindings)
```

### Dead Letter Queue Properties

Dead letter queues inherit production-ready properties:

**Durability & Replication:**

- Same durability as main queue
- Same replication factor (for quorum queues)
- Same high availability settings (for HA queues)

**Safety Features:**

- No auto-delete (prevents accidental loss)
- Configurable message TTL (default: 7 days)
- Optional size limits
- No infinite DLX loops (DLQs don't have their own DLX)

### Consumer Integration

Consumers can trigger dead letter routing:

```go
handler := func(ctx context.Context, delivery amqp.Delivery) error {
    // Process message
    err := processOrder(delivery.Body)

    if err != nil {
        // Reject without requeue - sends to DLX
        delivery.Nack(false, false)

        emit.Error.StructuredFields("Message sent to dead letter queue",
            emit.ZString("message_id", delivery.MessageID),
            emit.ZString("error", err.Error()))

        return err
    }

    // Acknowledge successful processing
    delivery.Ack(false)
    return nil
}
```

### Monitoring and Operations

**RabbitMQ Management Console:**

- Dead letter exchanges appear as regular exchanges
- Dead letter queues show failed message counts
- Bindings are visible in topology view

**Operational Patterns:**

```go
// Monitor dead letter queue depth
dlqInfo, err := channel.QueueInspect("orders.dlq")
if dlqInfo.Messages > 100 {
    emit.Warn.StructuredFields("High dead letter queue depth",
        emit.ZString("queue", "orders.dlq"),
        emit.ZInt("message_count", dlqInfo.Messages))
}

// Republish messages from DLQ (manual recovery)
// This would typically be done by an operator or admin tool
```

### Production Considerations

**Capacity Planning:**

- Monitor DLQ growth rates
- Set appropriate TTL for failed messages
- Consider DLQ size limits for runaway failures

**Alerting:**

- Alert on DLQ message accumulation
- Monitor DLQ depth trends
- Track failed message patterns

**Recovery Strategies:**

- Republish corrected messages
- Analyze failure patterns
- Update processing logic based on DLQ contents

### Comparison with Manual Setup

**Traditional Manual Setup:**

```bash
# Manual RabbitMQ commands needed:
rabbitmqctl declare exchange orders.dlx direct
rabbitmqctl declare queue orders.dlq durable=true
rabbitmqctl bind_queue orders.dlx orders.dlq orders.dlq
rabbitmqctl declare queue orders durable=true x-dead-letter-exchange=orders.dlx
```

**Package Automatic Setup:**

```go
// Single line - everything created automatically
config := rabbitmq.DefaultQuorumQueueConfig("orders")
```

**Benefits of Automatic Setup:**

- ✅ **Zero Errors**: No manual command mistakes
- ✅ **Consistent**: Same pattern across all environments
- ✅ **Efficient**: Batch creation and deduplication
- ✅ **Maintainable**: Configuration as code
- ✅ **Documented**: Self-documenting through config

For complete examples, see:

- `examples/dead-letter-queues/main.go` - Comprehensive dead letter infrastructure demo

## Configuration

### Connection Configuration

```go
config := rabbitmq.ConnectionConfig{
    URL:            "amqp://localhost:5672",
    RetryAttempts:  5,
    RetryDelay:     time.Second * 2,
    Heartbeat:      time.Second * 10,
    ConnectionName: "my-service",
}
```

### Publisher Configuration

```go
config := rabbitmq.PublisherConfig{
    ConnectionConfig:  connectionConfig,
    DefaultExchange:   "my-exchange",
    DefaultRoutingKey: "default",
    Persistent:        true,
    Mandatory:         false,
    Immediate:         false,
}
```

### Consumer Configuration

```go
config := rabbitmq.ConsumerConfig{
    ConnectionConfig: connectionConfig,
    AutoAck:          false,
    Exclusive:        false,
    PrefetchCount:    10,
    PrefetchSize:     0,
    PrefetchGlobal:   false,
}
```

### Timeout Configuration

The package provides comprehensive timeout controls for production reliability and operational safety.

#### Connection Timeouts

Control connection establishment and channel creation timeouts:

```go
config := rabbitmq.ConnectionConfig{
    URL:            "amqp://localhost:5672",
    DialTimeout:    time.Second * 30, // TCP connection timeout
    ChannelTimeout: time.Second * 10, // Channel creation timeout
    // ...other settings
}
```

**Default Values:**

- `DialTimeout`: 30 seconds - TCP connection establishment
- `ChannelTimeout`: 10 seconds - AMQP channel creation

#### Publisher Timeouts

Configure publisher confirmation timeouts:

```go
config := rabbitmq.PublisherConfig{
    ConnectionConfig:    connectionConfig,
    ConfirmationTimeout: time.Second * 5, // Publisher confirmation timeout
    // ...other settings
}
```

**Default Values:**

- `ConfirmationTimeout`: 5 seconds - Waiting for publish confirmations

**Usage Example:**

```go
// Custom publisher with extended confirmation timeout
publisher, err := rabbitmq.NewPublisherWithConfig(rabbitmq.PublisherConfig{
    ConnectionConfig:    rabbitmq.DefaultConnectionConfig("amqp://localhost:5672"),
    ConfirmationTimeout: time.Second * 15, // Extended timeout for slow brokers
    Persistent:          true,
})
```

#### Consumer Timeouts

Configure message processing and shutdown timeouts:

```go
config := rabbitmq.ConsumerConfig{
    ConnectionConfig: rabbitmq.DefaultConnectionConfig("amqp://localhost:5672"),
    MessageTimeout:   time.Minute * 5,  // Individual message processing timeout
    ShutdownTimeout:  time.Second * 30, // Graceful shutdown timeout
}
```

**Default Values:**

- `MessageTimeout`: 5 minutes - Individual message processing limit
- `ShutdownTimeout`: 30 seconds - Graceful consumer shutdown

**Message Processing Timeout:**

- Automatically cancels long-running message handlers
- Sends NACK and requeues messages when timeout exceeded
- Prevents consumer blocking on stuck handlers

**Shutdown Timeout:**

- Controls graceful shutdown duration
- Forces immediate close if timeout exceeded
- Ensures timely resource cleanup

**Usage Example:**

```go
// Consumer with custom timeouts for long-running tasks
consumer, err := rabbitmq.NewConsumerWithConfig(rabbitmq.ConsumerConfig{
    ConnectionConfig: rabbitmq.DefaultConnectionConfig("amqp://localhost:5672"),
    MessageTimeout:   time.Minute * 15, // 15-minute processing limit
    ShutdownTimeout:  time.Minute * 2,  // 2-minute shutdown grace period
    PrefetchCount:    1,                // Process one message at a time
})

err = consumer.Consume(ctx, rabbitmq.ConsumeConfig{
    Queue: "long-running-tasks",
    Handler: func(ctx context.Context, message []byte) error {
        // Long-running task with automatic timeout protection
        return processLongRunningTask(ctx, message)
    },
})
```

#### Production Timeout Recommendations

**Connection Timeouts:**

- **Development**: Default values (30s dial, 10s channel)
- **Production**: Reduce for faster failure detection (15s dial, 5s channel)
- **High-latency networks**: Increase dial timeout (60s+)

**Publisher Timeouts:**

- **High-throughput**: 2-5 seconds for fast confirmation
- **Standard applications**: 5-10 seconds (default: 5s)
- **Batch processing**: 15-30 seconds for large messages

**Consumer Timeouts:**

- **Fast processing**: 30 seconds - 2 minutes
- **Standard processing**: 5 minutes (default)
- **Long-running tasks**: 15-30 minutes
- **Background jobs**: 1+ hours

**Shutdown Timeouts:**

- **Development**: 30 seconds (default)
- **Production**: 1-2 minutes for graceful drain
- **Critical systems**: 5+ minutes to prevent data loss

#### Timeout Best Practices

**1. Context Propagation**
Always respect context timeouts in message handlers:

```go
handler := func(ctx context.Context, message []byte) error {
    // Respect both consumer timeout and request context
    select {
    case <-ctx.Done():
        return ctx.Err() // Timeout or cancellation
    default:
        return processMessage(ctx, message)
    }
}
```

**2. Timeout Monitoring**
Monitor timeout occurrences for capacity planning:

```go
// Consumer will automatically log timeout events via emit library
// Monitor logs for patterns:
// "Message processing timeout" - Indicates insufficient MessageTimeout
// "Consumer shutdown timeout exceeded" - Indicates insufficient ShutdownTimeout
```

**3. Graceful Degradation**
Implement fallback strategies for timeout scenarios:

```go
handler := func(ctx context.Context, message []byte) error {
    done := make(chan error, 1)
    go func() {
        done <- heavyProcessing(message)
    }()

    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        // Timeout - save for later processing
        return saveForRetry(message)
    }
}
```

#### Timeout Troubleshooting

**Common Issues:**

- **Frequent message timeouts**: Increase `MessageTimeout` or optimize handler performance
- **Slow connection establishment**: Increase `DialTimeout` or check network latency
- **Publisher confirmation delays**: Increase `ConfirmationTimeout` or check broker performance
- **Unclean shutdowns**: Increase `ShutdownTimeout` or reduce message processing time

**Diagnostic Commands:**

```bash
# Check RabbitMQ connection metrics
rabbitmqctl list_connections name timeout

# Monitor queue processing rates
rabbitmqctl list_queues name messages_ready messages_unacknowledged

# Check consumer performance
rabbitmqctl list_consumers queue_name channel ack_required prefetch_count
```

## Best Practices

### 1. Connection Management

- Use a single connection per application
- Implement proper connection health monitoring
- Handle connection failures gracefully

### 2. Publishing

- Always use publisher confirmations for critical messages
- Set appropriate message persistence based on requirements
- Use proper content types (e.g., "application/json")

### 3. Consuming

- Disable auto-acknowledgment for reliability
- Implement proper error handling in message handlers
- Use appropriate prefetch settings for performance

### 4. Error Handling

- Check connection status before operations
- Implement retry logic for transient failures
- Use context for cancellation and timeouts

### 5. Testing

- Use Docker for integration tests
- Mock connections for unit tests
- Test error scenarios and edge cases

## Development

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

### Building

```bash
# Build all examples
make build

# Format and lint
make fmt
make lint

# Full CI pipeline
make ci
```

## Advanced Usage

### Custom Topology Setup

```go
conn, _ := rabbitmq.NewConnection(config)

err := rabbitmq.SetupTopology(conn,
    // Exchanges
    []rabbitmq.ExchangeConfig{{
        Name: "events",
        Type: rabbitmq.ExchangeTypeTopic,
        Durable: true,
    }},
    // Queues
    []rabbitmq.QueueConfig{{
        Name: "user-events",
        Durable: true,
    }},
    // Bindings
    []rabbitmq.BindingConfig{{
        QueueName:    "user-events",
        ExchangeName: "events",
        RoutingKey:   "user.*",
    }},
)
```

### Publisher with Confirmations

```go
err := publisher.PublishWithConfirmation(ctx, rabbitmq.PublishConfig{
    Exchange:   "events",
    RoutingKey: "user.created",
    Message:    messageData,
    Headers: map[string]any{
        "version": "1.0",
        "source":  "user-service",
    },
})
```

### Advanced Consumer with Raw Deliveries

```go
err := consumer.ConsumeWithDeliveryHandler(ctx, config,
    func(ctx context.Context, delivery amqp.Delivery) error {
        // Access full delivery information
        info := rabbitmq.ExtractDeliveryInfo(delivery)

        // Process message
        if err := processMessage(delivery.Body); err != nil {
            delivery.Nack(false, true) // Reject and requeue
            return err
        }

        delivery.Ack(false) // Acknowledge
        return nil
    })
```

## Performance Considerations

1. **Connection Pooling**: Use a single connection per application
2. **Channel Management**: Each publisher/consumer uses its own channel
3. **Prefetch Settings**: Tune based on message processing time
4. **Message Size**: Consider message batching for small messages
5. **Persistence**: Only use persistent messages when necessary

## Production Checklist

- [x] **Implement proper logging** (emit library integrated with structured, high-performance logging)
- [ ] Set up monitoring and metrics
- [x] **Configure appropriate timeouts** (connection, channel, message processing, confirmation, and shutdown timeouts)
- [x] **Implement graceful shutdown** (shutdown manager, in-flight tracking, signal handling, and coordinated shutdown)
- [ ] Test failover scenarios
- [ ] Monitor memory usage
- [ ] Set up alerting for connection failures

## Graceful Shutdown

The package provides comprehensive graceful shutdown functionality to ensure clean termination of RabbitMQ operations without data loss or resource leaks.

### Why Graceful Shutdown?

**Production Safety:**

- **Data Integrity**: Ensures in-flight messages are processed before shutdown
- **Resource Cleanup**: Proper connection and channel closure
- **Zero Data Loss**: Prevents message loss during service restarts
- **Operational Reliability**: Predictable shutdown behavior

**Best Practices Compliance:**

- **Signal Handling**: Standard SIGINT/SIGTERM signal processing
- **Timeout Controls**: Configurable shutdown timeouts
- **Component Coordination**: Unified shutdown across multiple components
- **Monitoring Integration**: Structured logging of shutdown events

### Core Components

#### 1. Shutdown Manager

Central coordinator for graceful shutdown across multiple RabbitMQ components:

```go
// Create shutdown manager with configuration
config := rabbitmq.DefaultShutdownConfig()
shutdownManager := rabbitmq.NewShutdownManager(config)

// Setup automatic signal handling
shutdownManager.SetupSignalHandler()

// Register components for coordinated shutdown
shutdownManager.Register(publisher)
shutdownManager.Register(consumer)

// Wait for shutdown completion
shutdownManager.Wait()
```

#### 2. In-Flight Operation Tracking

Automatically tracks and waits for ongoing operations:

- **Publishers**: Waits for pending publish confirmations
- **Consumers**: Waits for current message processing to complete
- **Timeout Protection**: Prevents indefinite waiting

#### 3. Configurable Timeouts

Fine-grained control over shutdown timing:

```go
config := rabbitmq.ShutdownConfig{
    Timeout:           time.Second * 30, // Overall shutdown timeout
    SignalTimeout:     time.Second * 5,  // Grace period after signal
    GracefulDrainTime: time.Second * 10, // Time for in-flight operations
}
```

### Publisher Graceful Shutdown

Publishers now include shutdown timeout configuration and in-flight tracking:

```go
publisherConfig := rabbitmq.PublisherConfig{
    ConnectionConfig:    rabbitmq.DefaultConnectionConfig("amqp://localhost:5672"),
    ShutdownTimeout:     time.Second * 15, // Graceful shutdown timeout
    ConfirmationTimeout: time.Second * 5,  // Publish confirmation timeout
}

publisher, _ := rabbitmq.NewPublisherWithConfig(publisherConfig)
```

**Publisher Shutdown Features:**

- **In-Flight Tracking**: Monitors pending publish operations
- **Confirmation Waiting**: Waits for outstanding confirmations
- **Timeout Protection**: Forces shutdown if timeout exceeded
- **New Operation Rejection**: Rejects new publishes during shutdown

### Consumer Graceful Shutdown

Consumers include message processing timeout and graceful drain:

```go
consumerConfig := rabbitmq.ConsumerConfig{
    ConnectionConfig: rabbitmq.DefaultConnectionConfig("amqp://localhost:5672"),
    MessageTimeout:   time.Minute * 5,  // Message processing timeout
    ShutdownTimeout:  time.Second * 30, // Graceful shutdown timeout
}

consumer, _ := rabbitmq.NewConsumerWithConfig(consumerConfig)
```

**Consumer Shutdown Features:**

- **Message Processing Completion**: Waits for current messages to finish
- **NACK During Shutdown**: Requeues messages received during shutdown
- **Processing Timeout**: Cancels long-running message handlers
- **Clean Acknowledgments**: Ensures proper message acknowledgment

### Basic Graceful Shutdown

Simple graceful shutdown with signal handling:

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"

    "github.com/cloudresty/emit"
    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create components
    publisher, _ := rabbitmq.NewPublisher("amqp://localhost:5672")
    consumer, _ := rabbitmq.NewConsumer("amqp://localhost:5672")

    // Setup signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Start operations...
    ctx, cancel := context.WithCancel(context.Background())
    go consumer.Consume(ctx, config)

    // Wait for shutdown signal
    <-sigChan
    emit.Info.Msg("Received shutdown signal")

    // Graceful shutdown
    cancel() // Stop consuming
    publisher.Close() // Wait for publisher shutdown
    consumer.Close()  // Wait for consumer shutdown

    emit.Info.Msg("Graceful shutdown completed")
}
```

### Advanced Graceful Shutdown

Using the shutdown manager for coordinated shutdown:

```go
package main

import (
    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create shutdown manager
    shutdownManager := rabbitmq.NewShutdownManager(
        rabbitmq.DefaultShutdownConfig())

    // Setup automatic signal handling
    shutdownManager.SetupSignalHandler()

    // Create and register components
    publisher, _ := rabbitmq.NewPublisher("amqp://localhost:5672")
    consumer, _ := rabbitmq.NewConsumer("amqp://localhost:5672")

    shutdownManager.Register(publisher)
    shutdownManager.Register(consumer)

    // Start operations...

    // Wait for shutdown (blocks until SIGINT/SIGTERM)
    shutdownManager.Wait()
}
```

### Shutdown Configuration

#### Shutdown Configuration Defaults

```go
config := rabbitmq.DefaultShutdownConfig()
// Timeout: 30 seconds (overall shutdown timeout)
// SignalTimeout: 5 seconds (grace period after signal)
// GracefulDrainTime: 10 seconds (in-flight operation timeout)
```

#### Custom Configuration

```go
config := rabbitmq.ShutdownConfig{
    Timeout:           time.Minute * 2,  // 2-minute overall timeout
    SignalTimeout:     time.Second * 10, // 10-second signal grace period
    GracefulDrainTime: time.Second * 30, // 30-second drain time
}

shutdownManager := rabbitmq.NewShutdownManager(config)
```

### Production Recommendations

#### Timeout Settings

**Development Environment:**

- Overall Timeout: 30 seconds
- Signal Timeout: 5 seconds
- Drain Time: 10 seconds

**Production Environment:**

- Overall Timeout: 2-5 minutes
- Signal Timeout: 30 seconds
- Drain Time: 1-2 minutes

**High-Throughput Systems:**

- Overall Timeout: 5-10 minutes
- Signal Timeout: 1 minute
- Drain Time: 3-5 minutes

#### Monitoring Integration

Monitor shutdown events for operational insights:

```go
// Shutdown events are automatically logged via emit library:
// - "Starting graceful shutdown" - Shutdown initiated
// - "Component shutdown successful" - Individual component closed
// - "Graceful shutdown completed successfully" - Full shutdown success
// - "Graceful shutdown timeout exceeded" - Timeout warnings
```

#### Container Integration

Proper shutdown in containerized environments:

```dockerfile
# Dockerfile
STOPSIGNAL SIGTERM
```

```yaml
# Kubernetes
spec:
  terminationGracePeriodSeconds: 300  # 5 minutes
  containers:
  - name: rabbitmq-app
    # ... other config
```

### Graceful Shutdown Best Practices

#### 1. Always Use Signal Handling

```go
// ✅ Good - Proper signal handling
shutdownManager.SetupSignalHandler()

// ❌ Bad - No signal handling
// Application may be forcefully terminated
```

#### 2. Set Appropriate Timeouts

```go
// ✅ Good - Reasonable timeouts
config := rabbitmq.ShutdownConfig{
    Timeout: time.Minute * 2, // Allows time for cleanup
}

// ❌ Bad - Too short timeout
config := rabbitmq.ShutdownConfig{
    Timeout: time.Second * 5, // May interrupt operations
}
```

#### 3. Register All Components

```go
// ✅ Good - All components registered
shutdownManager.Register(publisher)
shutdownManager.Register(consumer)
shutdownManager.Register(connection)

// ❌ Bad - Some components missing
shutdownManager.Register(publisher)
// Missing consumer registration
```

#### 4. Use Context for Operations

```go
// ✅ Good - Respect context cancellation
func processMessage(ctx context.Context, msg []byte) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        return doProcessing(msg)
    }
}

// ❌ Bad - Ignore context
func processMessage(ctx context.Context, msg []byte) error {
    // No context checking - may block shutdown
    return doProcessing(msg)
}
```

### Troubleshooting

#### Common Issues

**Shutdown Timeouts:**

- Increase overall shutdown timeout
- Optimize message processing speed
- Reduce in-flight operation counts

**Resource Leaks:**

- Ensure all components are registered with shutdown manager
- Check for goroutines that don't respect context cancellation
- Monitor connection and channel closure

**Data Loss:**

- Verify message acknowledgment before shutdown
- Check consumer auto-ack settings
- Ensure proper error handling during shutdown

#### Diagnostic Commands

```bash
# Monitor graceful shutdown logs
grep "graceful shutdown" /var/log/app.log

# Check for resource cleanup
netstat -an | grep :5672

# Verify process termination
ps aux | grep your-app
```

For complete examples, see:

- `examples/graceful-shutdown/main.go` - Comprehensive graceful shutdown demo
- `examples/consumer/main.go` - Consumer with signal handling
- `examples/reconnection-test/main.go` - Graceful shutdown during reconnection
