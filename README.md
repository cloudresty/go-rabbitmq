# Go RabbitMQ

[Home](README.md) &nbsp;/

&nbsp;

**A modular, production-ready Go library for RabbitMQ with pluggable architecture.** Built on the **contract-implementation pattern**, where core interfaces live in the root package and concrete implementations are provided by specialized sub-packages. This design enables maximum flexibility, testability, and extensibility for enterprise messaging solutions.

&nbsp;

[![Go Reference](https://pkg.go.dev/badge/github.com/cloudresty/go-rabbitmq.svg)](https://pkg.go.dev/github.com/cloudresty/go-rabbitmq)
[![Go Tests](https://github.com/cloudresty/go-rabbitmq/actions/workflows/ci.yaml/badge.svg)](https://github.com/cloudresty/go-rabbitmq/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cloudresty/go-rabbitmq)](https://goreportcard.com/report/github.com/cloudresty/go-rabbitmq)
[![GitHub Tag](https://img.shields.io/github/v/tag/cloudresty/go-rabbitmq?label=Version)](https://github.com/cloudresty/go-rabbitmq/tags)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

&nbsp;

## Table of Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [Pluggable Sub-Packages](#pluggable-sub-packages)
- [Key Features](#key-features)
- [Simple Queue Configuration](#simple-queue-configuration)
- [Quick Start](#quick-start)
- [Production Usage](#production-usage)
- [Advanced Examples](#advanced-examples)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Requirements

- Go 1.24+ (recommended)
- RabbitMQ 4.0+ (recommended)

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Installation

```bash
go get github.com/cloudresty/go-rabbitmq
```

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Pluggable Sub-Packages

Each sub-package implements core interfaces defined in the root package, enabling you to mix and match features as needed:

| Package | Purpose | Key Features |
|---------|---------|--------------|
| **[compression/](compression/)** | Message compression | Gzip, Zlib with configurable thresholds |
| **[encryption/](encryption/)** | Message encryption | AES-256-GCM with secure key management |
| **[pool/](pool/)** | Connection pooling | Round-robin, health monitoring, auto-repair |
| **[performance/](performance/)** | Metrics & monitoring | Latency tracking, rate monitoring, statistics |
| **[saga/](saga/)** | Distributed transactions | Orchestration engine, compensation, atomic state |
| **[streams/](streams/)** | RabbitMQ Streams | High-throughput, durable, ordered messaging |
| **[shutdown/](shutdown/)** | Graceful shutdown | Signal handling, resource cleanup, timeouts |
| **[protobuf/](protobuf/)** | Protocol Buffers | Type-safe serialization, message routing |

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Key Features

### Contract-Implementation Architecture

- **Core Interfaces**: All contracts defined in the root package
- **Pluggable Implementations**: Concrete implementations in specialized sub-packages
- **Mix & Match**: Combine any features - encryption + compression + pooling + streams
- **Testing**: Easy mocking of interfaces for comprehensive unit testing

🔝 [back to top](#go-rabbitmq)

&nbsp;

### Production-Ready Features

- **Connection Pooling**: Distribute load across multiple connections with health monitoring
- **Message Encryption**: AES-256-GCM encryption with secure key management
- **Compression**: Gzip/Zlib compression with configurable thresholds
- **Saga Pattern**: Distributed transaction orchestration with automatic compensation
- **Streams Support**: High-throughput RabbitMQ Streams for event sourcing
- **Performance Monitoring**: Latency tracking, rate monitoring, comprehensive metrics

🔝 [back to top](#go-rabbitmq)

&nbsp;

### Developer Experience

- **ULID Message IDs**: 6x faster than UUIDs, database-optimized, lexicographically sortable
- **Auto-Reconnection**: Intelligent retry with configurable exponential backoff
- **Graceful Shutdown**: Signal handling with proper resource cleanup and timeouts
- **Comprehensive Documentation**: Each sub-package has detailed README with examples

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Simple Queue Configuration

This library provides a straightforward approach to queue configuration with user control over topology:

&nbsp;

### Quorum Queues by Default

- **High Availability**: Built-in replication across cluster nodes
- **Data Safety**: No message loss during node failures
- **Poison Message Protection**: Automatic delivery limits prevent infinite redelivery loops
- **Better Performance**: Optimized for throughput in clustered environments

🔝 [back to top](#go-rabbitmq)

&nbsp;

### Dead Letter Configuration

- **Manual Configuration**: Full control over dead letter exchange and routing configuration
- **Flexible Setup**: Configure dead letter handling exactly as needed for your topology
- **Error Handling**: Failed messages routed according to your dead letter configuration

🔝 [back to top](#go-rabbitmq)

&nbsp;

### Easy Customization

```go
// Default: Quorum queue (production-ready)
queue, _ := admin.DeclareQueue(ctx, "orders")

// Custom quorum settings
queue, _ := admin.DeclareQueue(ctx, "payments",
    rabbitmq.WithQuorumGroupSize(5),       // Custom cluster size
    rabbitmq.WithDeliveryLimit(3),         // Max retry attempts
)

// With dead letter configuration
queue, _ := admin.DeclareQueue(ctx, "processing",
    rabbitmq.WithDeadLetter("errors.dlx", "failed"), // Manual DLX setup
)

// Legacy compatibility (opt-in)
queue, _ := admin.DeclareQueue(ctx, "legacy",
    rabbitmq.WithClassicQueue(),           // Classic queue type
)
```

**Benefits**: Get enterprise-grade reliability and availability with simple, user-controlled configuration.

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create a basic client
    client, err := rabbitmq.NewClient(
        rabbitmq.WithHosts("localhost:5672"),
        rabbitmq.WithCredentials("guest", "guest"),
        rabbitmq.WithConnectionName("my-service"),
    )
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Declare a queue - uses quorum type by default (production-ready!)
    admin := client.Admin()
    queue, err := admin.DeclareQueue(context.Background(), "user-events")
    if err != nil {
        log.Fatal("Failed to declare queue:", err)
    }
    log.Printf("Created production-ready queue: %s", queue.Name)

    // Create a publisher
    publisher, err := client.NewPublisher(
        rabbitmq.WithDefaultExchange("events"),
    )
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    // Publish a message with auto-generated ULID
    message := rabbitmq.NewMessage([]byte(`{"event": "user_signup", "user_id": 123}`))
    err = publisher.Publish(context.Background(), "events", "user.created", message)
    if err != nil {
        log.Fatal("Failed to publish:", err)
    }

    // Create a consumer
    consumer, err := client.NewConsumer()
    if err != nil {
        log.Fatal("Failed to create consumer:", err)
    }
    defer consumer.Close()

    // Consume messages
    err = consumer.Consume(context.Background(), "user-events", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        log.Printf("Received: %s (ID: %s)", delivery.Body, delivery.MessageId)
        return nil
    })
    if err != nil {
        log.Fatal("Failed to consume:", err)
    }
}
```

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Production Usage

### Environment-Based Configuration

```bash
# Set environment variables for deployment
export RABBITMQ_HOST=rabbitmq.production.com
export RABBITMQ_USERNAME=myservice
export RABBITMQ_PASSWORD=securepassword
export RABBITMQ_VHOST=/production
export RABBITMQ_CONNECTION_NAME=order-service
```

🔝 [back to top](#go-rabbitmq)

&nbsp;

### High-Availability Setup

```go
// Production-ready client with HA
client, err := rabbitmq.NewClient(
    rabbitmq.WithHosts("rabbit1:5672", "rabbit2:5672", "rabbit3:5672"),
    rabbitmq.WithCredentials("user", "pass"),
    rabbitmq.WithVHost("/production"),
    rabbitmq.WithConnectionName("order-service"),
    rabbitmq.WithReconnectPolicy(&rabbitmq.ExponentialBackoff{
        InitialDelay: 1 * time.Second,
        MaxDelay:     30 * time.Second,
        Multiplier:   2.0,
        MaxAttempts:  10,
    }),
)

// Connection pooling for high throughput
pool, err := pool.New(20,
    pool.WithClientOptions(/* client options */),
    pool.WithHealthCheck(30 * time.Second),
    pool.WithAutoRepair(true),
)

// Performance monitoring
monitor := performance.NewMonitor()

// Graceful shutdown management
shutdownManager := shutdown.NewManager()
shutdownManager.RegisterComponents(publisher, consumer, pool)
shutdownManager.SetupSignalHandler()
```

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Advanced Examples

### Production Defaults Demo

See [examples/production-defaults/](examples/production-defaults/) for a comprehensive demonstration of the production-ready defaults including quorum queues, dead letter configuration, and customization options.

### Complete Feature Integration

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/compression"
    "github.com/cloudresty/go-rabbitmq/encryption"
    "github.com/cloudresty/go-rabbitmq/pool"
    "github.com/cloudresty/go-rabbitmq/performance"
    "github.com/cloudresty/go-rabbitmq/shutdown"
)

func main() {
    // Create connection pool for high-throughput
    connectionPool, err := pool.New(10,
        pool.WithClientOptions(
            rabbitmq.WithHosts("localhost:5672"),
            rabbitmq.WithCredentials("guest", "guest"),
        ),
        pool.WithHealthCheck(30*time.Second),
        pool.WithAutoRepair(true),
    )
    if err != nil {
        log.Fatal("Failed to create pool:", err)
    }
    defer connectionPool.Close()

    // Get client from pool
    client, err := connectionPool.Get()
    if err != nil {
        log.Fatal("No healthy connections:", err)
    }

    // Create pluggable features
    compressor := compression.NewGzip()
    encryptor, _ := encryption.NewAESGCM([]byte("your-32-byte-encryption-key-here"))
    monitor := performance.NewMonitor()

    // Create publisher with all features
    publisher, err := client.NewPublisher(
        rabbitmq.WithCompression(compressor),
        rabbitmq.WithEncryption(encryptor),
        rabbitmq.WithCompressionThreshold(100),
        rabbitmq.WithConfirmation(5*time.Second),
    )
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    // Publish with monitoring
    message := rabbitmq.NewMessage([]byte(`{"large": "payload with lots of data..."}`))

    start := time.Now()
    err = publisher.Publish(context.Background(), "events", "data.processed", message)
    duration := time.Since(start)

    // Record performance metrics
    monitor.RecordPublish(err == nil, duration)

    // Setup graceful shutdown
    shutdownManager := shutdown.NewManager()
    shutdownManager.RegisterComponents(publisher, consumer, connectionPool)

    // Handle shutdown signals
    shutdownManager.SetupSignalHandler()
    shutdownManager.Wait() // Blocks until SIGINT/SIGTERM
}
```

🔝 [back to top](#go-rabbitmq)

&nbsp;

### Saga Pattern for Distributed Transactions

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/saga"
)

func main() {
    client, _ := rabbitmq.NewClient(rabbitmq.WithHosts("localhost:5672"))
    store := saga.NewInMemoryStore()

    // Define step and compensation handlers
    stepHandlers := map[string]saga.StepHandler{
        "create_order":    createOrderHandler,
        "reserve_inventory": reserveInventoryHandler,
        "charge_payment":  chargePaymentHandler,
    }

    compensationHandlers := map[string]saga.CompensationHandler{
        "cancel_order":      cancelOrderHandler,
        "release_inventory": releaseInventoryHandler,
        "refund_payment":    refundPaymentHandler,
    }

    // Create saga manager with orchestration engine
    manager, err := saga.NewManager(client, store, saga.Config{
        SagaExchange:         "sagas",
        StepQueue:           "saga.steps",
        CompensateQueue:     "saga.compensate",
        StepHandlers:        stepHandlers,
        CompensationHandlers: compensationHandlers,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer manager.Close()

    // Start orchestration engine
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        if err := manager.Run(ctx); err != nil {
            log.Printf("Orchestration engine error: %v", err)
        }
    }()

    // Define distributed transaction steps
    steps := []saga.Step{
        {Name: "create_order", Action: "create_order", Compensation: "cancel_order"},
        {Name: "reserve_inventory", Action: "reserve_inventory", Compensation: "release_inventory"},
        {Name: "charge_payment", Action: "charge_payment", Compensation: "refund_payment"},
    }

    // Start saga (engine will orchestrate automatically)
    s, err := manager.Start(ctx, "order_processing", steps, map[string]any{
        "customer_id": "cust-123",
        "order_total": 99.99,
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Started saga: %s", s.ID)
}

// Implement your step and compensation handlers...
func createOrderHandler(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    // Business logic for order creation
    return nil
}
```

🔝 [back to top](#go-rabbitmq)

&nbsp;

### High-Performance Streams

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/streams"
)

func main() {
    client, _ := rabbitmq.NewClient(rabbitmq.WithHosts("localhost:5672"))
    streamsHandler := streams.NewHandler(client)

    // Create high-throughput stream
    config := rabbitmq.StreamConfig{
        MaxAge:            24 * time.Hour,
        MaxLengthMessages: 10_000_000,
        MaxLengthBytes:    10 * 1024 * 1024 * 1024, // 10GB
        InitialClusterSize: 3, // High availability
    }

    err := streamsHandler.CreateStream(context.Background(), "events.stream", config)
    if err != nil {
        log.Printf("Stream creation: %v", err)
    }

    // High-speed publishing
    for i := 0; i < 100000; i++ {
        message := rabbitmq.NewMessage([]byte(fmt.Sprintf("Event %d: %s", i, time.Now())))
        err := streamsHandler.PublishToStream(context.Background(), "events.stream", message)
        if err != nil {
            log.Printf("Failed to publish: %v", err)
            break
        }
    }

    // Consume from stream
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    err = streamsHandler.ConsumeFromStream(ctx, "events.stream", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        log.Printf("Stream message: %s", delivery.Body)
        return nil
    })
}
```

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Documentation

| Document | Description |
|----------|-------------|
| **Sub-Package READMEs** | Detailed documentation for each pluggable feature |
| [compression/](compression/) | Message compression with Gzip and Zlib |
| [encryption/](encryption/) | AES-256-GCM message encryption |
| [performance/](performance/) | Metrics collection and monitoring |
| [pool/](pool/) | Connection pooling with health monitoring |
| [protobuf/](protobuf/) | Protocol Buffers integration |
| [saga/](saga/) | Distributed transaction orchestration |
| [shutdown/](shutdown/) | Graceful shutdown management |
| [streams/](streams/) | High-throughput RabbitMQ Streams |
| **Additional Docs** | |
| [API Reference](docs/api-reference.md) | Complete function reference and usage patterns |
| [Environment Variables](docs/environment-variables.md) | List of environment variables for configuration |
| [Environment Variables](docs/environment-variables.md) | Complete environment variable reference and usage |
| [Production Features](docs/production-features.md) | Auto-reconnection, graceful shutdown, HA queues |
| [Examples](examples/) | Working examples for each feature |
| [ULID Message IDs](docs/ulid-message-ids.md) | Using ULIDs for message IDs in RabbitMQ |
| [Usage Patterns](docs/usage-patterns.md) | Common patterns for using the library effectively |

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Ensure all tests pass
5. Submit a pull request

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Security

If you discover a security vulnerability, please report it via email to [security@cloudresty.com](mailto:security@cloudresty.com).

🔝 [back to top](#go-rabbitmq)

&nbsp;

## License

This project is licensed under the MIT License - see the [LICENSE.txt](LICENSE.txt) file for details.

🔝 [back to top](#go-rabbitmq)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
