# API Reference

[Home](../README.md) &nbsp;/&nbsp; [Docs](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides a comprehensive overview of all available functions in the go-rabbitmq package.

&nbsp;

## Core Functions

| Function | Description |
|----------|-------------|
| `NewPublisher()` | Creates a publisher using environment variables (RABBITMQ_*) |
| `NewConsumer()` | Creates a consumer using environment variables (RABBITMQ_*) |
| `NewPublisherWithPrefix(prefix)` | Creates a publisher with custom env var prefix (e.g., "MYAPP_") |
| `NewConsumerWithPrefix(prefix)` | Creates a consumer with custom env var prefix (e.g., "MYAPP_") |
| `NewPublisherWithConfig(config)` | Creates a publisher with full custom configuration |
| `NewConsumerWithConfig(config)` | Creates a consumer with full custom configuration |
| `NewConnection(config)` | Creates a direct connection with custom configuration |

🔝 [back to top](#api-reference)

&nbsp;

## Environment Configuration

| Function | Description |
|----------|-------------|
| `LoadFromEnv()` | Loads configuration from RABBITMQ_* environment variables |
| `LoadFromEnvWithPrefix(prefix)` | Loads configuration with custom prefix (e.g., MYAPP_RABBITMQ_*) |
| `envConfig.ToPublisherConfig()` | Converts env config to publisher configuration |
| `envConfig.ToConsumerConfig()` | Converts env config to consumer configuration |
| `envConfig.ToConnectionConfig()` | Converts env config to connection configuration |
| `envConfig.BuildAMQPURL()` | Builds AMQP URL from environment variables |
| `envConfig.BuildHTTPURL()` | Builds HTTP management URL from environment variables |

🔝 [back to top](#api-reference)

&nbsp;

## Publisher Operations

| Function | Description |
|----------|-------------|
| `publisher.Publish(ctx, config)` | Publishes a message to an exchange |
| `publisher.PublishWithConfirmation(ctx, config)` | Publishes with broker confirmation |
| `publisher.PublishMessage(ctx, config)` | Publishes a Message object |
| `publisher.DeclareExchange(...)` | Declares an exchange with custom parameters |
| `publisher.DeclareQueue(...)` | Declares a queue with custom parameters |
| `publisher.DeclareQuorumQueue(name)` | Declares a production-ready quorum queue |
| `publisher.DeclareHAQueue(name)` | Declares a production-ready HA classic queue |
| `publisher.DeclareQueueWithConfig(config)` | Declares a queue using QueueConfig |
| `publisher.Close()` | Gracefully closes the publisher |
| `publisher.IsConnected()` | Checks if publisher is connected |
| `publisher.GetConnection()` | Returns underlying connection for advanced use |

🔝 [back to top](#api-reference)

&nbsp;

## Consumer Operations

| Function | Description |
|----------|-------------|
| `consumer.Consume(ctx, config)` | Starts consuming messages from a queue |
| `consumer.DeclareQueue(...)` | Declares a queue with custom parameters |
| `consumer.DeclareQuorumQueue(name)` | Declares a production-ready quorum queue |
| `consumer.DeclareHAQueue(name)` | Declares a production-ready HA classic queue |
| `consumer.DeclareQueueWithConfig(config)` | Declares a queue using QueueConfig |
| `consumer.Close()` | Gracefully closes the consumer |
| `consumer.IsConnected()` | Checks if consumer is connected |
| `consumer.GetConnection()` | Returns underlying connection for advanced use |

🔝 [back to top](#api-reference)

&nbsp;

## Message Creation

| Function | Description |
|----------|-------------|
| `NewMessage(body)` | Creates a message with auto-generated ULID |
| `NewMessageWithID(body, id)` | Creates a message with custom ID |
| `NewJSONMessage(body)` | Creates a JSON message with proper content type |
| `NewTextMessage(body)` | Creates a text message with proper content type |
| `message.WithHeader(key, value)` | Adds a header to the message |
| `message.WithCorrelationID(id)` | Sets correlation ID (ULID recommended) |
| `message.WithType(msgType)` | Sets message type |
| `message.WithReplyTo(queue)` | Sets reply-to queue |

🔝 [back to top](#api-reference)

&nbsp;

## Configuration Helpers

| Function | Description |
|----------|-------------|
| `DefaultConnectionConfig(url)` | Creates default connection configuration |
| `DefaultQuorumQueueConfig(name)` | Creates production-ready quorum queue config |
| `DefaultHAQueueConfig(name)` | Creates production-ready HA queue config |
| `queueConfig.WithDeadLetter(dlx, dlq, ttl)` | Configures dead letter infrastructure |
| `queueConfig.WithoutDeadLetter()` | Disables dead letter infrastructure |
| `queueConfig.WithCustomDeadLetter(dlx, key)` | Uses existing dead letter exchange |

🔝 [back to top](#api-reference)

&nbsp;

## Topology Management

| Function | Description |
|----------|-------------|
| `SetupTopology(conn, exchanges, queues, bindings)` | Sets up complete RabbitMQ topology |

🔝 [back to top](#api-reference)

&nbsp;

## Shutdown Management

| Function | Description |
|----------|-------------|
| `NewShutdownManager(config)` | Creates a coordinated shutdown manager |
| `shutdownManager.SetupSignalHandler()` | Configures automatic signal handling |
| `shutdownManager.Register(component)` | Registers component for coordinated shutdown |
| `shutdownManager.Wait()` | Waits for shutdown signal |

🔝 [back to top](#api-reference)

&nbsp;

## Error Types

| Function | Description |
|----------|-------------|
| `NewConnectionError(msg, cause)` | Creates a connection-related error |
| `NewPublishError(msg, cause)` | Creates a publish-related error |
| `NewConsumeError(msg, cause)` | Creates a consume-related error |

🔝 [back to top](#api-reference)

&nbsp;

## Usage Patterns

&nbsp;

### Environment-First (Recommended)

```go
publisher, err := rabbitmq.NewPublisher()           // Uses RABBITMQ_* env vars
consumer, err := rabbitmq.NewConsumer()             // Uses RABBITMQ_* env vars
```

🔝 [back to top](#api-reference)

&nbsp;

### Custom Prefix

```go
publisher, err := rabbitmq.NewPublisherWithPrefix("MYAPP_")   // Uses MYAPP_RABBITMQ_* env vars
```

🔝 [back to top](#api-reference)

&nbsp;

### Full Configuration

```go
config := rabbitmq.PublisherConfig{...}
publisher, err := rabbitmq.NewPublisherWithConfig(config)
```

🔝 [back to top](#api-reference)

&nbsp;

## Advanced Usage Patterns

### Custom Topology Setup

Complete RabbitMQ topology creation with exchanges, queues, and bindings:

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

🔝 [back to top](#api-reference)

&nbsp;

### Publisher with Confirmations

Reliable message publishing with broker acknowledgments:

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

🔝 [back to top](#api-reference)

&nbsp;

### Advanced Consumer with Raw Deliveries

Access full AMQP delivery information for advanced message handling:

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

🔝 [back to top](#api-reference)

&nbsp;

## Best Practices

### Connection Management

- Use a single connection per application
- Implement proper connection health monitoring
- Handle connection failures gracefully

### Publishing

- Always use publisher confirmations for critical messages
- Set appropriate message persistence based on requirements
- Use proper content types (e.g., "application/json")

### Consuming

- Disable auto-acknowledgment for reliability
- Implement proper error handling in message handlers
- Use appropriate prefetch settings for performance

### Error Handling

- Check connection status before operations
- Implement retry logic for transient failures
- Use context for cancellation and timeouts

### Testing

- Use Docker for integration tests
- Mock connections for unit tests
- Test error scenarios and edge cases

🔝 [back to top](#api-reference)

&nbsp;

## Performance Considerations

1. **Connection Pooling**: Use a single connection per application
2. **Channel Management**: Each publisher/consumer uses its own channel
3. **Prefetch Settings**: Tune based on message processing time
4. **Message Size**: Consider message batching for small messages
5. **Persistence**: Only use persistent messages when necessary

🔝 [back to top](#api-reference)

&nbsp;

---

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
