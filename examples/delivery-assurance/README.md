# Delivery Assurance Example

This example demonstrates the built-in delivery assurance capabilities of the go-rabbitmq library.

## Overview

Delivery assurance provides reliable message delivery guarantees through asynchronous callbacks, eliminating the need for applications to manually handle publisher confirms, mandatory publishing, and confirmation processing.

## Features Demonstrated

1. **Simple Delivery Assurance** - Basic usage with default callbacks
2. **Advanced Callbacks** - Custom per-message callbacks with business logic
3. **Mandatory Publishing** - Detecting routing failures
4. **Delivery Statistics** - Monitoring delivery metrics
5. **Concurrent Publishing** - Thread-safe concurrent message publishing

## Prerequisites

- RabbitMQ server running on `localhost:5672`
- Default credentials (`guest`/`guest`)

## Running the Example

```bash
cd examples/delivery-assurance
go run main.go
```

## Key Concepts

### Delivery Outcomes

The library tracks four possible delivery outcomes:

- **DeliverySuccess**: Message confirmed by broker and successfully routed
- **DeliveryFailed**: Message returned by broker (no queue bound to routing key)
- **DeliveryNacked**: Message negatively acknowledged by broker
- **DeliveryTimeout**: No confirmation received within timeout period

### Publisher Configuration

```go
publisher, err := client.NewPublisher(
    rabbitmq.WithDeliveryAssurance(),                    // Enable delivery assurance
    rabbitmq.WithDefaultDeliveryCallback(callback),      // Set default callback
    rabbitmq.WithDeliveryTimeout(10 * time.Second),      // Set timeout
    rabbitmq.WithMandatoryByDefault(true),               // Make all messages mandatory
)
```

### Publishing with Delivery Assurance

```go
err := publisher.PublishWithDeliveryAssurance(
    ctx,
    "events",
    "user.created",
    message,
    rabbitmq.DeliveryOptions{
        MessageID: "unique-id",
        Mandatory: true,
        Callback: func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
            // Handle delivery outcome
        },
    },
)
```

### Monitoring Statistics

```go
stats := publisher.GetDeliveryStats()
fmt.Printf("Published: %d, Confirmed: %d, Pending: %d\n",
    stats.TotalPublished, stats.TotalConfirmed, stats.PendingMessages)
```

## Architecture

### Asynchronous Processing

Delivery assurance uses background goroutines to process confirmations and returns asynchronously:

- **Confirmation Handler**: Processes publisher confirms from RabbitMQ
- **Return Handler**: Processes returned messages (routing failures)
- **Timeout Manager**: Handles delivery timeouts

### Thread Safety

All delivery assurance operations are thread-safe:

- Concurrent publishing is fully supported
- Statistics are protected by mutexes
- Callbacks are invoked in separate goroutines

### Graceful Shutdown

When closing a publisher with delivery assurance:

1. Background goroutines are signaled to stop
2. Pending message timeouts are cancelled
3. Resources are cleaned up
4. Dedicated confirmation channel is closed

## Benefits Over Manual Implementation

### Before (Manual Implementation)

```go
// Applications had to bypass the library and use amqp091-go directly
channel, err := client.CreateChannel()
channel.Confirm(false)
confirmChan := make(chan amqp091.Confirmation, 100)
returnChan := make(chan amqp091.Return, 100)
channel.NotifyPublish(confirmChan)
channel.NotifyReturn(returnChan)

// Manual confirmation handling in goroutines...
go func() {
    for confirmation := range confirmChan {
        // Handle confirmation
    }
}()

go func() {
    for ret := range returnChan {
        // Handle return
    }
}()
```

### After (Built-in Delivery Assurance)

```go
// Simple, clean API with built-in reliability
publisher, err := client.NewPublisher(
    rabbitmq.WithDeliveryAssurance(),
    rabbitmq.WithDefaultDeliveryCallback(callback),
)

err = publisher.PublishWithDeliveryAssurance(ctx, exchange, routingKey, message,
    rabbitmq.DeliveryOptions{MessageID: "id", Mandatory: true})
```

## Use Cases

### 1. Critical Event Publishing

Ensure important events are successfully delivered:

```go
callback := func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
    if outcome != rabbitmq.DeliverySuccess {
        // Alert operations team
        alerting.SendAlert("Message delivery failed", messageID, errorMessage)
        // Store for retry
        retryQueue.Add(messageID)
    }
}
```

### 2. Audit Logging

Track all message delivery outcomes for compliance:

```go
callback := func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
    auditLog.Record(AuditEntry{
        MessageID: messageID,
        Outcome:   string(outcome),
        Timestamp: time.Now(),
        Error:     errorMessage,
    })
}
```

### 3. Metrics and Monitoring

Integrate with monitoring systems:

```go
callback := func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
    metrics.IncrementCounter("rabbitmq.delivery.outcome", map[string]string{
        "outcome": string(outcome),
    })
}
```

## Best Practices

1. **Always Set Message IDs**: Use unique, meaningful message IDs for tracking
2. **Use Mandatory for Critical Messages**: Detect routing failures early
3. **Monitor Statistics**: Regularly check delivery stats for anomalies
4. **Handle All Outcomes**: Implement logic for all delivery outcomes
5. **Set Appropriate Timeouts**: Balance between responsiveness and reliability
6. **Graceful Shutdown**: Always close publishers to clean up resources

## Troubleshooting

### Messages Timing Out

- Increase delivery timeout: `WithDeliveryTimeout(30 * time.Second)`
- Check RabbitMQ server health and network connectivity
- Monitor pending message count

### High Return Rate

- Verify queue bindings are correct
- Check routing keys match binding patterns
- Ensure queues exist before publishing

### Memory Growth

- Monitor pending message count
- Ensure callbacks are not blocking
- Check for proper publisher cleanup on shutdown

## Related Documentation

- [Main API Reference](../../docs/api-reference.md)
- [Production Features](../../docs/production-features.md)
- [Usage Patterns](../../docs/usage-patterns.md)

