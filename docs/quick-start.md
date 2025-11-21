# Quick Start

[Home](../README.md) &nbsp;/&nbsp; [Docs](README.md) &nbsp;/&nbsp; Quick Start

&nbsp;

This guide provides a quick start for using the `go-rabbitmq` library. For more detailed information, please refer to the [documentation](README.md).

&nbsp;

## Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create a basic client (topology auto-healing enabled by default!)
    client, err := rabbitmq.NewClient(
        rabbitmq.WithHosts("localhost:5672"),
        rabbitmq.WithCredentials("guest", "guest"),
        rabbitmq.WithConnectionName("my-service"),
    )
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Declare a queue - automatically tracked and protected from deletion!
    admin := client.Admin()
    queue, err := admin.DeclareQueue(context.Background(), "user-events")
    if err != nil {
        log.Fatal("Failed to declare queue:", err)
    }
    log.Printf("Created auto-healing production queue: %s", queue.Name)

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

🔝 [back to top](#quick-start)

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

🔝 [back to top](#quick-start)

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

🔝 [back to top](#quick-start)

&nbsp;

## Advanced Examples

### Production Defaults Demo

See [examples/production-defaults/](../examples/production-defaults/) for a comprehensive demonstration of the production-ready defaults including quorum queues, dead letter configuration, and customization options.

### Topology Auto-Healing Demo

See [examples/topology-features/](../examples/topology-features/) for a comprehensive demonstration of topology validation, auto-recreation, environment configuration, and opt-out options.

### Delivery Assurance Demo

See [examples/delivery-assurance/](../examples/delivery-assurance/) for comprehensive examples of reliable message delivery with asynchronous callbacks, including simple usage, advanced callbacks, mandatory publishing, statistics monitoring, and concurrent publishing.

```go
// Create publisher with delivery assurance
publisher, err := client.NewPublisher(
    rabbitmq.WithDeliveryAssurance(),
    rabbitmq.WithDefaultDeliveryCallback(func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
        switch outcome {
        case rabbitmq.DeliverySuccess:
            log.Printf("✓ Message %s delivered successfully", messageID)
        case rabbitmq.DeliveryFailed:
            log.Printf("✗ Message %s failed: %s", messageID, errorMessage)
        case rabbitmq.DeliveryNacked:
            log.Printf("⚠ Message %s nacked", messageID)
        case rabbitmq.DeliveryTimeout:
            log.Printf("⏱ Message %s timed out", messageID)
        }
    }),
)

// Publish with delivery tracking
err = publisher.PublishWithDeliveryAssurance(ctx, "events", "user.created", message,
    rabbitmq.DeliveryOptions{
        MessageID: "event-123",
        Mandatory: true,
    })
```

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

🔝 [back to top](#quick-start)

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

🔝 [back to top](#quick-start)

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

🔝 [back to top](#quick-start)

&nbsp;

---

&nbsp;

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
