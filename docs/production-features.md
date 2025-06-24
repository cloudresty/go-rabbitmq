# Production Features

[Home](../README.md) &nbsp;/&nbsp; [Docs](README.md) &nbsp;/&nbsp; Production Features

&nbsp;

This document covers all production-ready features designed for high-availability, fault-tolerant deployments.

&nbsp;

## Auto-Reconnection

Intelligent reconnection functionality handles network interruptions gracefully.

&nbsp;

### Configuration

```bash
# Environment variables for auto-reconnection
export RABBITMQ_AUTO_RECONNECT=true
export RABBITMQ_RECONNECT_DELAY=5s
export RABBITMQ_MAX_RECONNECT_ATTEMPTS=10  # 0 = unlimited
export RABBITMQ_HEARTBEAT=10s
```

🔝 [back to top](#production-features)

&nbsp;

### Usage

```go
// Use environment configuration (recommended)
publisher, err := rabbitmq.NewPublisher()
consumer, err := rabbitmq.NewConsumer()

// Or customize environment config
envConfig, err := rabbitmq.LoadFromEnv()
envConfig.AutoReconnect = true
envConfig.ReconnectDelay = time.Second * 10
envConfig.MaxReconnectAttempts = 5

connectionConfig := envConfig.ToConnectionConfig()
publisher, err := rabbitmq.NewPublisherWithConfig(rabbitmq.PublisherConfig{
    ConnectionConfig: connectionConfig,
})
```

🔝 [back to top](#production-features)

&nbsp;

### Features

- **Automatic Detection**: Monitors connection health via heartbeats
- **Intelligent Retry**: Configurable delay and maximum attempts
- **Graceful Recovery**: Seamless reconnection without data loss
- **Production Ready**: Handles network issues, server restarts, etc.

🔝 [back to top](#production-features)

&nbsp;

## Production-Ready Queues

Built-in support for production-ready queue configurations.

&nbsp;

### Quick Start

```go
// Create publisher/consumer from environment
publisher, err := rabbitmq.NewPublisher()
consumer, err := rabbitmq.NewConsumer()

// Declare production-ready queues
err := publisher.DeclareQuorumQueue("orders")    // Recommended for HA clusters
err := consumer.DeclareHAQueue("notifications")  // For backward compatibility
```

🔝 [back to top](#production-features)

&nbsp;

### Custom Queue Configuration

```go
config := rabbitmq.QueueConfig{
    Name:               "payments",
    QueueType:          rabbitmq.QueueTypeQuorum,
    ReplicationFactor:  5,                        // 5-node quorum
    MaxLength:          100000,                   // Message limit
    MessageTTL:         int(time.Hour * 24),      // 24-hour TTL
    // Dead Letter Infrastructure (enabled by default)
    AutoCreateDLX:      true,                     // Auto-create DLX and DLQ
    DLXSuffix:          ".dlx",                   // DLX naming: payments.dlx
    DLQSuffix:          ".dlq",                   // DLQ naming: payments.dlq
    DLQMessageTTL:      7 * 24 * 60 * 60 * 1000,  // 7-day TTL in DLQ
}
err := publisher.DeclareQueueWithConfig(config)
```

🔝 [back to top](#production-features)

&nbsp;

### Queue Types

&nbsp;

#### Quorum Queues (Recommended)

- Raft-based replicated queues for new applications
- Built-in replication and leader election
- Configurable replication factor (default: 3)
- Better performance and safety than HA classic queues

🔝 [back to top](#production-features)

&nbsp;

#### HA Classic Queues

- Mirror-based replicated queues for compatibility
- Automatic mirroring across cluster nodes
- Compatible with older RabbitMQ features

🔝 [back to top](#production-features)

&nbsp;

### Queue Features

- **Durability**: All queues are durable by default
- **High Availability**: Automatic replication across cluster nodes
- **Message Persistence**: Messages survive broker restarts
- **Automatic Dead Letter Infrastructure**: Built-in DLX and DLQ creation
- **Size Limits**: Configure max length and byte limits
- **TTL Support**: Automatic message expiration

🔝 [back to top](#production-features)

&nbsp;

## Dead Letter Infrastructure

Automatic creation of dead letter exchanges (DLX) and dead letter queues (DLQ) for production safety.

&nbsp;

### Dead Letter Setup

```go
// Create publisher/consumer from environment
publisher, err := rabbitmq.NewPublisher()

// Default: Auto-creates payments.dlx and payments.dlq
config := rabbitmq.DefaultQuorumQueueConfig("payments")
// config.AutoCreateDLX = true (enabled by default)

// Disable dead letter infrastructure
config.WithoutDeadLetter()

// Custom dead letter settings
config.WithDeadLetter(".dlx", ".dead", 3) // 3-day TTL in DLQ

// Use existing DLX (manual setup)
config.WithCustomDeadLetter("my-dlx", "failed.routing")

err := publisher.DeclareQueueWithConfig(config)
```

🔝 [back to top](#production-features)

&nbsp;

### Benefits

- **Message Safety**: Failed messages are preserved for debugging
- **Automatic Setup**: No manual DLX/DLQ creation needed
- **Production Ready**: Proper replication and TTL configuration
- **Operational Visibility**: Easy identification of processing failures
- **Flexible Configuration**: Enable/disable per queue as needed

🔝 [back to top](#production-features)

&nbsp;

## Timeout Configuration

Comprehensive timeout controls for production reliability.

&nbsp;

### Environment Variables

```bash
# Timeout environment variables
export RABBITMQ_DIAL_TIMEOUT=30s
export RABBITMQ_CHANNEL_TIMEOUT=10s
export RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT=10s
export RABBITMQ_PUBLISHER_SHUTDOWN_TIMEOUT=30s
export RABBITMQ_CONSUMER_MESSAGE_TIMEOUT=5m
export RABBITMQ_CONSUMER_SHUTDOWN_TIMEOUT=30s
```

🔝 [back to top](#production-features)

&nbsp;

### Programmatic Configuration

```go
// Use environment configuration (recommended)
publisher, err := rabbitmq.NewPublisher()
consumer, err := rabbitmq.NewConsumer()

// Or customize after loading from environment
envConfig, err := rabbitmq.LoadFromEnv()
envConfig.DialTimeout = time.Second * 60
envConfig.PublisherConfirmationTimeout = time.Second * 15
envConfig.ConsumerMessageTimeout = time.Minute * 10

publisherConfig := envConfig.ToPublisherConfig()
consumerConfig := envConfig.ToConsumerConfig()

publisher, err := rabbitmq.NewPublisherWithConfig(publisherConfig)
consumer, err := rabbitmq.NewConsumerWithConfig(consumerConfig)
```

🔝 [back to top](#production-features)

&nbsp;

### Timeout Types

- **Connection Timeouts**: Control TCP connection and AMQP channel creation
- **Message Processing**: Automatic cancellation of long-running message handlers
- **Publisher Confirmations**: Configurable wait time for delivery confirmations
- **Graceful Shutdown**: Coordinated shutdown with timeout protection

🔝 [back to top](#production-features)

&nbsp;

### Detailed Timeout Configuration

&nbsp;

#### Connection Timeouts

```go
config := rabbitmq.ConnectionConfig{
    URL:            "amqp://localhost:5672",
    DialTimeout:    time.Second * 30, // TCP connection timeout
    ChannelTimeout: time.Second * 10, // Channel creation timeout
}
```

**Default Values:**

- `DialTimeout`: 30 seconds - TCP connection establishment
- `ChannelTimeout`: 10 seconds - AMQP channel creation

🔝 [back to top](#production-features)

&nbsp;

#### Publisher Timeouts

```go
publisher, err := rabbitmq.NewPublisherWithConfig(rabbitmq.PublisherConfig{
    ConnectionConfig:    rabbitmq.DefaultConnectionConfig("amqp://localhost:5672"),
    ConfirmationTimeout: time.Second * 15, // Extended timeout for slow brokers
    Persistent:          true,
})
```

**Default Values:**

- `ConfirmationTimeout`: 5 seconds - Waiting for publish confirmations

🔝 [back to top](#production-features)

&nbsp;

#### Consumer Timeouts

```go
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

**Default Values:**

- `MessageTimeout`: 5 minutes - Individual message processing limit
- `ShutdownTimeout`: 30 seconds - Graceful consumer shutdown

🔝 [back to top](#production-features)

&nbsp;

### Production Timeout Recommendations

&nbsp;

#### Environment-Based Recommendations

**Development:**

- Connection: Default values (30s dial, 10s channel)
- Publisher: 5-10 seconds (default: 5s)
- Consumer: 5 minutes (default)
- Shutdown: 30 seconds (default)

**Production:**

- Connection: Reduce for faster failure detection (15s dial, 5s channel)
- Publisher: 2-5 seconds for fast confirmation
- Consumer: 1-2 minutes for graceful drain
- Shutdown: 1-2 minutes for graceful drain

**High-Throughput Systems:**

- Connection: High-latency networks may need 60s+ dial timeout
- Publisher: 15-30 seconds for large messages
- Consumer: 15-30 minutes for long-running tasks
- Shutdown: 5+ minutes to prevent data loss

🔝 [back to top](#production-features)

&nbsp;

### Timeout Best Practices

&nbsp;

#### Context Propagation

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

🔝 [back to top](#production-features)

&nbsp;

#### Graceful Degradation

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

🔝 [back to top](#production-features)

&nbsp;

## Graceful Shutdown

Production-ready graceful shutdown with coordinated resource cleanup.

&nbsp;

### Basic Graceful Shutdown

```go
func main() {
    // Create from environment variables
    publisher, _ := rabbitmq.NewPublisher()
    consumer, _ := rabbitmq.NewConsumer()

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
    cancel()           // Stop consuming new messages
    publisher.Close()  // Wait for pending publishes
    consumer.Close()   // Wait for current message processing
}
```

🔝 [back to top](#production-features)

&nbsp;

### Advanced Coordinated Shutdown

```go
func main() {
    // Load shutdown timeouts from environment
    envConfig, _ := rabbitmq.LoadFromEnv()

    // Create shutdown manager with environment timeouts
    shutdownConfig := rabbitmq.ShutdownConfig{
        GracePeriod: envConfig.PublisherShutdownTimeout,
        ForceTimeout: envConfig.ConsumerShutdownTimeout,
    }
    shutdownManager := rabbitmq.NewShutdownManager(shutdownConfig)

    // Setup automatic signal handling
    shutdownManager.SetupSignalHandler()

    // Create components from environment
    publisher, _ := rabbitmq.NewPublisher()
    consumer, _ := rabbitmq.NewConsumer()

    // Register components for coordinated shutdown
    shutdownManager.Register(publisher)
    shutdownManager.Register(consumer)

    // Start operations...

    // Wait for shutdown (blocks until SIGINT/SIGTERM)
    shutdownManager.Wait()
}
```

🔝 [back to top](#production-features)

&nbsp;

### Shutdown Features

- **Signal Handling**: Automatic SIGINT/SIGTERM signal processing
- **In-Flight Tracking**: Waits for pending operations to complete
- **Timeout Protection**: Prevents indefinite waiting during shutdown
- **Component Coordination**: Unified shutdown across multiple components
- **Zero Data Loss**: Ensures message processing completion before exit

🔝 [back to top](#production-features)

&nbsp;

## Performance Characteristics

- **ULID Generation**: ~150ns (6x faster than UUID v4)
- **Zero-allocation Logging**: Optimized for high-throughput scenarios
- **Memory Efficient**: Minimal allocations and garbage collection pressure
- **Connection Pooling**: Single connection per application with channel management
- **Batch Operations**: Support for high-throughput message processing

🔝 [back to top](#production-features)

&nbsp;

## Production Checklist

Essential items for production deployment:

**✅ Completed Features:**

- **Implement proper logging** - emit library integrated with structured, high-performance logging
- **Configure appropriate timeouts** - connection, channel, message processing, confirmation, and shutdown timeouts
- **Implement graceful shutdown** - shutdown manager, in-flight tracking, signal handling, and coordinated shutdown
- **Environment-first configuration** - zero-config setup with RABBITMQ_* environment variables
- **Auto-reconnection** - intelligent retry with configurable backoff
- **Dead letter infrastructure** - automatic DLX/DLQ creation
- **ULID message IDs** - high-performance, database-optimized identifiers
- **Production-ready queues** - quorum and HA queue configurations

**📋 Additional Recommended Items:**

- [ ] Set up monitoring and metrics
- [ ] Test failover scenarios
- [ ] Monitor memory usage
- [ ] Set up alerting for connection failures
- [ ] Configure load balancing for RabbitMQ cluster
- [ ] Implement health checks
- [ ] Set up backup and disaster recovery
- [ ] Performance testing under load
- [ ] Security audit and hardening

🔝 [back to top](#production-features)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
