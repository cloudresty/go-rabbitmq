# Usage Patterns

[Home](../README.md) &nbsp;/&nbsp; [Docs](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

## Client-Based Architecture (Recommended)

```go
// Create client with unified configuration options
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithConnectionName("notification-service"),
    rabbitmq.WithReconnectPolicy(rabbitmq.ExponentialBackoff{...}),
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Perform all topology setup via AdminService
admin := client.Admin()
err = admin.DeclareExchange(ctx, "events", "topic", rabbitmq.WithDurable())
if err != nil {
    log.Fatal(err)
}

_, err = admin.DeclareQueue(ctx, "user-events", rabbitmq.WithDurable())
if err != nil {
    log.Fatal(err)
}

err = admin.BindQueue(ctx, "user-events", "events", "user.*")
if err != nil {
    log.Fatal(err)
}

// Create lean publisher and consumer
publisher, err := client.NewPublisher(
    rabbitmq.WithDefaultExchange("events"),
    rabbitmq.WithPersistent(),
)
if err != nil {
    log.Fatal(err)
}
defer publisher.Close()

consumer, err := client.NewConsumer(
    rabbitmq.WithPrefetchCount(10),
    rabbitmq.WithConcurrency(5),
)
if err != nil {
    log.Fatal(err)
}
defer consumer.Close()
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Environment-Based Configuration

```go
// Unified configuration pattern - FromEnv() as an option
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithConnectionName("my-app"), // Composes perfectly with other options
)
if err != nil {
    log.Fatal(err)
}

// Custom prefix configuration
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnvWithPrefix("MYAPP_RABBITMQ_"),
    rabbitmq.WithConnectionName("my-app"),
)
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Multi-Host Failover

```go
// Multiple hosts for failover with environment configuration
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(), // Uses RABBITMQ_HOSTS environment variable
    rabbitmq.WithConnectionName("resilient-app"),
)

// Or explicitly set multiple hosts
client, err := rabbitmq.NewClient(
    rabbitmq.WithHosts("mq1.prod.com:5672", "mq2.prod.com:5672", "mq3.prod.com:5672"),
    rabbitmq.WithConnectionName("resilient-app"),
)
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Message Publishing

```go
// Create a message
message := rabbitmq.NewMessage([]byte("Hello, World!"))
message = message.
    WithCorrelationID("request-123").
    WithHeaders(map[string]any{
        "version": "1.0",
        "source":  "user-service",
    }).
    WithPersistent()

// Publish message
err := publisher.Publish(ctx, "my-exchange", "routing.key", message)
if err != nil {
    log.Fatal(err)
}

// For critical messages, create publisher with confirmations enabled
confirmingPublisher, err := client.NewPublisher(
    rabbitmq.WithDefaultExchange("my-exchange"),
    rabbitmq.WithConfirmation(5*time.Second),
)
if err != nil {
    log.Fatal(err)
}
defer confirmingPublisher.Close()

// Publish with confirmation (blocks until confirmed)
err = confirmingPublisher.Publish(ctx, "my-exchange", "routing.key", message)
if err != nil {
    log.Fatal(err)
}
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Message Consumption

```go
// Define message handler
handler := func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    message := delivery.GetMessage()

    // Process message
    log.Printf("Received: %s", string(message.Body))

    // Acknowledge message
    return delivery.Ack()
}

// Unified consume method with options
err := consumer.Consume(ctx, "my-queue", handler,
    rabbitmq.WithPrefetchCount(10),
    rabbitmq.WithConcurrency(5),
    rabbitmq.WithRejectRequeue(),
    rabbitmq.WithConsumeRetryPolicy(retryPolicy),
)
if err != nil {
    log.Fatal(err)
}
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Advanced Usage Patterns

&nbsp;

## Complete Topology Setup

Set up exchanges, queues, and bindings with the AdminService:

```go
// Get admin service from client
admin := client.Admin()

// Declare exchanges
err := admin.DeclareExchange(ctx, "events", "topic",
    rabbitmq.WithDurable(),
    rabbitmq.WithAutoDelete(false),
)

// Declare queues
queueInfo, err := admin.DeclareQueue(ctx, "user-events",
    rabbitmq.WithDurable(),
    rabbitmq.WithTTL(24*time.Hour),
    rabbitmq.WithMaxLength(10000),
)

// Bind queues to exchanges
err = admin.BindQueue(ctx, "user-events", "events", "user.*",
    rabbitmq.WithBindingHeaders(map[string]any{
        "priority": "high",
    }),
)

// Setup complete topology at once
err = admin.SetupTopology(ctx, exchanges, queues, bindings)
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Batch Publishing

Publish multiple messages efficiently:

```go
messages := []rabbitmq.PublishRequest{
    {Exchange: "events", RoutingKey: "user.created", Message: userMessage},
    {Exchange: "events", RoutingKey: "order.placed", Message: orderMessage},
}

// Publish batch
err := publisher.PublishBatch(ctx, messages)
if err != nil {
    log.Fatal(err)
}

// For batch publishing with confirmations, create publisher with confirmation enabled
confirmingPublisher, err := client.NewPublisher(
    rabbitmq.WithDefaultExchange("events"),
    rabbitmq.WithConfirmation(5*time.Second),
)
if err != nil {
    log.Fatal(err)
}
defer confirmingPublisher.Close()

// Publish batch with confirmations (each message will be confirmed)
err = confirmingPublisher.PublishBatch(ctx, messages)
if err != nil {
    log.Printf("Batch publish failed: %v", err)
}
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Advanced Consumer Configuration

Full control over message consumption with the unified Consume method:

```go
consumer, err := client.NewConsumer(
    rabbitmq.WithMessageTimeout(5*time.Minute),
    rabbitmq.WithConsumerRetryPolicy(retryPolicy),
)

// Single method with all options
err = consumer.Consume(ctx, "orders", handler,
    rabbitmq.WithPrefetchCount(100),
    rabbitmq.WithConcurrency(10),
    rabbitmq.WithAutoAck(false),
    rabbitmq.WithRejectRequeue(),
    rabbitmq.WithDeadLetterPolicy(deadLetterPolicy),
    rabbitmq.WithConsumeRetryPolicy(retryPolicy),
)
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Enhanced Connection Pool Usage

Use advanced connection pooling for high-throughput applications:

```go
import "github.com/cloudresty/go-rabbitmq/pool"

// Create connection pool with custom configuration
connectionPool, err := pool.New(5, // Pool size
    pool.WithHealthCheck(30*time.Second),
    pool.WithAutoRepair(true),
    pool.WithClientOptions(
        rabbitmq.FromEnv(),
        rabbitmq.WithConnectionName("high-throughput-app"),
    ),
)
if err != nil {
    log.Fatal(err)
}
defer connectionPool.Close()

// Get a healthy client from the pool
client, err := connectionPool.Get()
if err != nil {
    log.Fatal("No healthy connections available:", err)
}

// Monitor pool statistics
stats := connectionPool.GetStats()
log.Printf("Pool size: %d, Healthy: %d, Unhealthy: %d",
    stats.Size, stats.HealthyConnections, stats.UnhealthyConnections)

// Use client for high-throughput operations
publisher, err := client.NewPublisher(
    rabbitmq.WithDefaultExchange("events"),
    rabbitmq.WithPersistent(),
)
if err != nil {
    log.Fatal(err)
}
defer publisher.Close()
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Advanced Error Handling

Use sophisticated error classification for intelligent retry logic:

```go
func publishWithRetry(publisher *rabbitmq.Publisher, ctx context.Context, exchange, routingKey string, message *rabbitmq.Message, maxRetries int) error {
    for attempt := 0; attempt <= maxRetries; attempt++ {
        err := publisher.Publish(ctx, exchange, routingKey, message)
        if err == nil {
            return nil // Success
        }

        // Use intelligent error classification
        if !rabbitmq.IsRetryableError(err) {
            // Permanent failure - don't retry
            return fmt.Errorf("permanent error, not retrying: %w", err)
        }

        if rabbitmq.IsConnectionError(err) {
            // Connection error - wait longer before retry
            time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
        } else {
            // Other retryable error - shorter wait
            time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
        }

        log.Printf("Publish attempt %d failed, retrying: %v", attempt+1, err)
    }

    return fmt.Errorf("failed after %d attempts", maxRetries)
}

// Example usage in a message handler
func messageHandler(ctx context.Context, delivery *rabbitmq.Delivery) error {
    // Process message
    if err := processMessage(delivery.Body); err != nil {
        // Check if we should retry processing
        if rabbitmq.IsRetryableError(err) {
            return rabbitmq.NewRejectError(true, err) // Requeue for retry
        }
        return rabbitmq.NewRejectError(false, err) // Don't requeue
    }

    return delivery.Ack()
}
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Complete Workflow Example

Here's the elegant, unified workflow that demonstrates the refined API design:

```go
// 1. Create a single, unified client with composable options
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithConnectionName("notification-service"),
    rabbitmq.WithAutoReconnect(true),
    rabbitmq.WithReconnectPolicy(rabbitmq.ExponentialBackoff{
        InitialDelay: time.Second,
        MaxDelay:     30 * time.Second,
        Multiplier:   2.0,
        MaxAttempts:  10,
    }),
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// 2. Perform all topology setup via AdminService
admin := client.Admin()
err = admin.DeclareExchange(ctx, "events", "topic", rabbitmq.WithDurable())
if err != nil {
    log.Fatal(err)
}

_, err = admin.DeclareQueue(ctx, "user-events", rabbitmq.WithDurable())
if err != nil {
    log.Fatal(err)
}

err = admin.BindQueue(ctx, "user-events", "events", "user.*")
if err != nil {
    log.Fatal(err)
}

// 3. Create a lean Publisher and publish a message
publisher, err := client.NewPublisher(
    rabbitmq.WithDefaultExchange("events"),
    rabbitmq.WithPersistent(),
    rabbitmq.WithConfirmation(5*time.Second), // Enable confirmations
)
if err != nil {
    log.Fatal(err)
}
defer publisher.Close()

msg := rabbitmq.NewMessage([]byte("hello")).WithCorrelationID("123")
err = publisher.Publish(ctx, "", "user.created", msg)
if err != nil {
    log.Fatal(err)
}

// 4. Create a lean Consumer and start processing
consumer, err := client.NewConsumer()
if err != nil {
    log.Fatal(err)
}
defer consumer.Close()

handler := func(ctx context.Context, d *rabbitmq.Delivery) error {
    log.Printf("Received message: %s", string(d.Body))
    return d.Ack()
}

err = consumer.Consume(ctx, "user-events", handler,
    rabbitmq.WithPrefetchCount(10),
    rabbitmq.WithConcurrency(5),
)
if err != nil {
    log.Fatal(err)
}
```

🔝 [back to top](#usage-patterns)

&nbsp;

## Best Practices

&nbsp;

## Configuration Management

- Use `FromEnv()` as the primary configuration method for production
- Compose environment configuration with explicit options for flexibility
- Use `FromEnvWithPrefix()` for multi-service deployments
- Single client instance per application with unified configuration

🔝 [back to top](#usage-patterns)

&nbsp;

## Topology Management

- Use `AdminService` for all topology operations (exchanges, queues, bindings)
- Separate topology setup from message operations
- Use `SetupTopology()` for complete infrastructure-as-code scenarios
- Declare topology before creating publishers and consumers

🔝 [back to top](#usage-patterns)

&nbsp;

## Connection Management

- Configure multiple hosts for production failover using `RABBITMQ_HOSTS`
- Handle connection failures gracefully with auto-reconnect
- Use meaningful connection names for monitoring and debugging
- Implement proper graceful shutdown with timeouts

🔝 [back to top](#usage-patterns)

&nbsp;

## Publishing

- Use publisher confirmations for critical messages
- Set appropriate message persistence based on requirements
- Use proper content types and headers
- Implement graceful shutdown with timeouts
- Use batch publishing for high-throughput scenarios

🔝 [back to top](#usage-patterns)

&nbsp;

## Consuming

- Use the unified `Consume()` method with functional options
- Disable auto-acknowledgment for reliability
- Implement proper error handling in message handlers
- Use appropriate prefetch and concurrency settings
- Handle context cancellation properly
- Use reject/requeue patterns for retries

🔝 [back to top](#usage-patterns)

&nbsp;

## Error Handling

- Check connection status before operations
- Implement retry logic for transient failures
- Use context for cancellation and timeouts
- Log connection events for monitoring
- Use proper error types for different scenarios

🔝 [back to top](#usage-patterns)

&nbsp;

## Performance Considerations

1. **Multi-Host Failover**: Automatic failover to secondary hosts on connection failure
2. **Client Architecture**: Single client instance with multiple publishers/consumers
3. **Channel Management**: Each publisher/consumer uses its own channel
4. **Prefetch and Concurrency**: Tune based on message processing time and system capacity
5. **Message Batching**: Use batch publishing for high-throughput scenarios
6. **Persistence**: Only use persistent messages when necessary
7. **Confirmation Mode**: Use selectively for critical messages only
8. **Connection Pooling**: Managed automatically by the client
9. **Resource Cleanup**: Always close publishers, consumers, and clients properly

🔝 [back to top](#usage-patterns)

&nbsp;

---

&nbsp;

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
