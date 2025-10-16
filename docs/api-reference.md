# Main Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [Docs](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides the complete API reference for the main `go-rabbitmq` package. It covers all functions, methods, and types available at the main package level for client creation, publishing, consuming, and administration.

&nbsp;

## Client Creation and Management

| Function | Description |
|----------|-------------|
| `NewClient(opts ...Option)` | Creates a new RabbitMQ client with configuration options |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Client Methods

| Function | Description |
|----------|-------------|
| `client.Ping(ctx context.Context)` | Tests connection health with the RabbitMQ server |
| `client.Close()` | Gracefully closes the client and all associated resources |
| `client.Admin()` | Returns the AdminService for topology management operations |
| `client.URL()` | Returns the primary connection URL being used |
| `client.ConnectionName()` | Returns the configured connection name |
| `client.NewPublisher(opts ...PublisherOption)` | Creates a new publisher with the specified options |
| `client.NewConsumer(opts ...ConsumerOption)` | Creates a new consumer with the specified options |
| `client.CreateChannel()` | Creates a new AMQP channel for advanced use cases |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Client Configuration Options

| Function | Description |
|----------|-------------|
| `WithCredentials(username, password string)` | Sets authentication credentials for RabbitMQ connection |
| `WithHosts(hosts ...string)` | Sets multiple RabbitMQ hosts for failover support |
| `WithVHost(vhost string)` | Sets the virtual host to connect to |
| `WithTLS(tlsConfig *tls.Config)` | Sets TLS configuration for secure connections |
| `WithInsecureTLS()` | Enables TLS with insecure certificate verification |
| `WithConnectionName(name string)` | Sets connection name for identification and logging |
| `WithHeartbeat(duration time.Duration)` | Sets connection heartbeat interval |
| `WithDialTimeout(timeout time.Duration)` | Sets connection dial timeout |
| `WithChannelTimeout(timeout time.Duration)` | Sets channel creation timeout |
| `WithReconnectPolicy(policy ReconnectPolicy)` | Sets reconnection policy for handling connection failures |
| `WithMaxReconnectAttempts(attempts int)` | Sets maximum number of reconnection attempts |
| `WithReconnectDelay(delay time.Duration)` | Sets delay between reconnection attempts |
| `WithAutoReconnect(enabled bool)` | Enables or disables automatic reconnection |
| `WithLogger(logger Logger)` | Sets custom logger for client operations |
| `WithMetrics(metrics MetricsCollector)` | Sets metrics collector for monitoring |
| `WithTracing(tracer Tracer)` | Sets distributed tracing for client operations |
| `WithAccessPolicy(policy *AccessPolicy)` | Sets access control policy |
| `WithAuditLogging(logger AuditLogger)` | Sets audit logging for compliance |
| `WithPerformanceMonitoring(monitor PerformanceMonitor)` | Sets performance monitoring (use `performance` sub-package) |
| `WithConnectionPooler(pooler ConnectionPooler)` | Sets connection pooler (use `pool` sub-package) |
| `WithStreamHandler(handler StreamHandler)` | Sets stream handler (use `streams` sub-package) |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Environment Configuration

| Function | Description |
|----------|-------------|
| `FromEnv()` | Creates a client option that loads configuration from RABBITMQ_* environment variables |
| `FromEnvWithPrefix(prefix string)` | Creates a client option that loads configuration with custom environment variable prefix |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Publisher Methods

| Function | Description |
|----------|-------------|
| `publisher.Publish(ctx context.Context, exchange, routingKey string, message *Message)` | Publishes a message to the specified exchange with routing key |
| `publisher.PublishBatch(ctx context.Context, messages []PublishRequest)` | Publishes multiple messages in a single batch operation |
| `publisher.Close()` | Gracefully closes the publisher and releases resources |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Publisher Options

| Function | Description |
|----------|-------------|
| `WithDefaultExchange(exchange string)` | Sets default exchange for publishing when not specified |
| `WithMandatory()` | Enables mandatory publishing (message must be routable) |
| `WithImmediate()` | Enables immediate publishing (message must be immediately deliverable) |
| `WithPersistent()` | Makes all published messages persistent by default |
| `WithConfirmation(timeout time.Duration)` | Enables publisher confirmations with timeout |
| `WithRetryPolicy(policy RetryPolicy)` | Sets retry policy for failed publish operations |
| `WithCompression(compressor MessageCompressor)` | Sets message compression (use `compression` sub-package) |
| `WithCompressionThreshold(threshold int)` | Sets compression threshold in bytes |
| `WithSerializer(serializer MessageSerializer)` | Sets message serializer for custom serialization |
| `WithEncryption(encryptor MessageEncryptor)` | Sets message encryption (use `encryption` sub-package) |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Consumer Methods

| Function | Description |
|----------|-------------|
| `consumer.Consume(ctx context.Context, queue string, handler MessageHandler, opts ...ConsumeOption)` | Starts consuming messages from the specified queue with message handler |
| `consumer.Close()` | Gracefully closes the consumer and stops message consumption |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Consumer Options

| Function | Description |
|----------|-------------|
| `WithAutoAck()` | Enables automatic acknowledgment of messages |
| `WithPrefetchCount(count int)` | Sets the number of unacknowledged messages per consumer |
| `WithPrefetchSize(size int)` | Sets the prefetch size in bytes |
| `WithConsumerTag(tag string)` | Sets a custom consumer tag for identification |
| `WithExclusiveConsumer()` | Makes the consumer exclusive to the queue |
| `WithNoLocal()` | Prevents delivery of messages published on the same connection |
| `WithNoWait()` | Makes the consume operation not wait for server response |
| `WithMessageTimeout(timeout time.Duration)` | Sets timeout for processing individual messages |
| `WithConcurrency(workers int)` | Sets number of concurrent message processing workers |
| `WithConsumerRetryPolicy(policy RetryPolicy)` | Sets retry policy for message processing failures |
| `WithConsumerCompression(compressor MessageCompressor)` | Sets compression for consumer (use `compression` sub-package) |
| `WithConsumerEncryption(encryptor MessageEncryptor)` | Sets encryption for consumer (use `encryption` sub-package) |
| `WithConsumerSerialization(serializer MessageSerializer)` | Sets serialization for consumer |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Consume Options

| Function | Description |
|----------|-------------|
| `WithRejectRequeue()` | Configures message rejection to requeue messages for retry |
| `WithConsumeRetryPolicy(policy RetryPolicy)` | Sets retry policy for consume operations |
| `WithDeadLetterPolicy(policy DeadLetterPolicy)` | Sets dead letter policy for failed messages |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## AdminService Methods

| Function | Description |
|----------|-------------|
| `admin.DeclareQueue(ctx context.Context, name string, opts ...QueueOption)` | Declares a queue with the specified options |
| `admin.DeclareQuorumQueue(ctx context.Context, name string, opts ...QuorumQueueOption)` | Declares a quorum queue with enhanced durability |
| `admin.DeclareExchange(ctx context.Context, name string, kind ExchangeType, opts ...ExchangeOption)` | Declares an exchange of the specified type |
| `admin.DeleteQueue(ctx context.Context, name string, opts ...DeleteQueueOption)` | Deletes a queue |
| `admin.DeleteExchange(ctx context.Context, name string, opts ...DeleteExchangeOption)` | Deletes an exchange |
| `admin.PurgeQueue(ctx context.Context, name string)` | Purges all messages from a queue, returns message count |
| `admin.InspectQueue(ctx context.Context, name string)` | Returns detailed information about a queue |
| `admin.QueueInfo(ctx context.Context, name string)` | Returns queue information (alias for InspectQueue) |
| `admin.ExchangeInfo(ctx context.Context, name string)` | Returns information about an exchange |
| `admin.BindQueue(ctx context.Context, queue, exchange, routingKey string, opts ...BindingOption)` | Binds a queue to an exchange with routing key |
| `admin.UnbindQueue(ctx context.Context, queue, exchange, routingKey string, opts ...BindingOption)` | Unbinds a queue from an exchange |
| `admin.BindExchange(ctx context.Context, destination, source, routingKey string, args Table)` | Binds one exchange to another exchange |
| `admin.DeclareTopology(ctx context.Context, topology *Topology)` | Declares complex topology from configuration |
| `admin.SetupTopology(ctx context.Context, exchanges []ExchangeConfig, queues []QueueConfig, bindings []BindingConfig)` | Sets up complete topology from configuration slices |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Queue Options

| Function | Description |
|----------|-------------|
| `WithDurable()` | Makes queue durable (survives server restarts) |
| `WithAutoDelete()` | Makes queue auto-delete when last consumer disconnects |
| `WithExclusiveQueue()` | Makes queue exclusive to the declaring connection |
| `WithTTL(ttl time.Duration)` | Sets time-to-live for messages in the queue |
| `WithMaxLength(length int64)` | Sets maximum number of messages in the queue |
| `WithMaxLengthBytes(bytes int64)` | Sets maximum size of the queue in bytes |
| `WithArguments(args Table)` | Sets custom queue arguments |
| `WithDeadLetter(exchange, routingKey string)` | Sets dead letter exchange and routing key |
| `WithDeadLetterTTL(ttl time.Duration)` | Sets TTL for dead letter messages |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Quorum Queue Options

| Function | Description |
|----------|-------------|
| `WithInitialGroupSize(size int)` | Sets initial quorum group size for the queue |
| `WithQuorumDeliveryLimit(limit int)` | Sets delivery limit for quorum queues |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Exchange Options

| Function | Description |
|----------|-------------|
| `WithExchangeDurable()` | Makes exchange durable (survives server restarts) |
| `WithExchangeAutoDelete()` | Makes exchange auto-delete when no longer used |
| `WithExchangeInternal()` | Makes exchange internal (can't be published to directly) |
| `WithExchangeArguments(args Table)` | Sets custom exchange arguments |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Binding Options

| Function | Description |
|----------|-------------|
| `WithBindingNoWait()` | Makes binding operation not wait for server response |
| `WithBindingHeaders(headers map[string]any)` | Sets headers for header exchange bindings |
| `WithBindingArguments(args Table)` | Sets custom binding arguments |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Message Creation and Management

| Function | Description |
|----------|-------------|
| `NewMessage(body []byte)` | Creates a new message with the specified body |
| `NewMessageWithID(body []byte, messageID string)` | Creates a new message with custom message ID |
| `NewJSONMessage(v interface{})` | Creates a new message with JSON-serialized body |
| `NewTextMessage(body []byte)` | Creates a new text message |
| `FromAMQPDelivery(delivery amqp.Delivery)` | Creates a message from AMQP delivery |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Message Methods

| Function | Description |
|----------|-------------|
| `message.WithCorrelationID(correlationID string)` | Sets correlation ID for request-response patterns |
| `message.WithReplyTo(replyTo string)` | Sets reply-to queue for responses |
| `message.WithType(messageType string)` | Sets message type for routing and handling |
| `message.WithAppID(appID string)` | Sets application ID that created the message |
| `message.WithUserID(userID string)` | Sets user ID for security validation |
| `message.WithExpiration(expiration time.Duration)` | Sets message expiration time |
| `message.WithPriority(priority uint8)` | Sets message priority (0-255) |
| `message.WithHeader(key string, value any)` | Adds a single header to the message |
| `message.WithHeaders(headers map[string]any)` | Sets multiple headers on the message |
| `message.WithContentType(contentType string)` | Sets MIME content type of the message body |
| `message.WithMessageID(id string)` | Sets unique message identifier |
| `message.WithTimestamp(t time.Time)` | Sets message timestamp |
| `message.WithPersistent()` | Makes message persistent (written to disk) |
| `message.WithTransient()` | Makes message transient (memory only) |
| `message.ToAMQPPublishing()` | Converts message to AMQP publishing format |
| `message.ToPublishing()` | Converts message to publishing format (alias) |
| `message.Validate()` | Validates message structure and required fields |
| `message.Clone()` | Creates a deep copy of the message |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Error Handling

| Function | Description |
|----------|-------------|
| `NewConnectionError(message string, cause error)` | Creates a new connection-related error |
| `NewPublishError(message string, cause error)` | Creates a new publish-related error |
| `NewConsumeError(message string, cause error)` | Creates a new consume-related error |
| `IsRetryableError(err error)` | Checks if an error is retryable |
| `IsConnectionError(err error)` | Checks if an error is connection-related |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Utility Functions

| Function | Description |
|----------|-------------|
| `DefaultQuorumQueueConfig(name string)` | Creates default configuration for quorum queues |
| `DefaultHAQueueConfig(name string)` | Creates default configuration for highly available queues |
| `DefaultClassicQueueConfig(name string)` | Creates default configuration for classic queues |
| `ExtractDeliveryInfo(delivery *amqp.Delivery)` | Extracts delivery information from AMQP delivery |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Types and Structures

| Type | Description |
|------|-------------|
| `Client` | Main RabbitMQ client for connection management |
| `Publisher` | Message publisher for sending messages to exchanges |
| `Consumer` | Message consumer for receiving messages from queues |
| `AdminService` | Service for managing RabbitMQ topology (exchanges, queues, bindings) |
| `Message` | Core message structure with headers, body, and metadata |
| `Delivery` | Enhanced delivery wrapper with acknowledgment methods |
| `QueueConfig` | Configuration structure for queue declaration |
| `ExchangeConfig` | Configuration structure for exchange declaration |
| `BindingConfig` | Configuration structure for queue-exchange bindings |
| `Topology` | Complete topology configuration with exchanges, queues, and bindings |
| `EnvConfig` | Environment-based configuration structure |
| `Table` | Map type for AMQP table arguments |
| `Error` | Base error type with category and cause information |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Retry Policies

| Type | Description |
|------|-------------|
| `ExponentialBackoff` | Exponential backoff retry policy with configurable multiplier |
| `LinearBackoff` | Linear backoff retry policy with constant increments |
| `FixedDelay` | Fixed delay retry policy with constant intervals |
| `NoRetryPolicy` | No-retry policy for immediate failure |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Sub-Package APIs

For advanced features, see the dedicated API references for each sub-package:

| Sub-Package | Description | API Reference |
|-------------|-------------|---------------|
| `compression` | Message compression with gzip, zlib support | [API Reference](../compression/api-reference.md) |
| `encryption` | Message encryption with AES-GCM | [API Reference](../encryption/api-reference.md) |
| `performance` | Performance monitoring and metrics | [API Reference](../performance/api-reference.md) |
| `pool` | Connection pooling for high-throughput scenarios | [API Reference](../pool/api-reference.md) |
| `protobuf` | Protocol Buffers support and type-safe routing | [API Reference](../protobuf/api-reference.md) |
| `saga` | Distributed transaction support with Saga pattern | [API Reference](../saga/api-reference.md) |
| `shutdown` | Graceful shutdown management | [API Reference](../shutdown/api-reference.md) |
| `streams` | RabbitMQ streams for high-throughput messaging | [API Reference](../streams/api-reference.md) |

🔝 [back to top](#main-package-api-reference)

&nbsp;

## Usage Examples

&nbsp;

### Basic Client Creation and Usage

```go
// Create client with environment configuration
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithConnectionName("my-service"),
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Test connection
if err := client.Ping(context.Background()); err != nil {
    log.Fatal("Connection failed:", err)
}
```

🔝 [back to top](#main-package-api-reference)

&nbsp;

### Publishing Messages

```go
// Create publisher
publisher, err := client.NewPublisher(
    rabbitmq.WithDefaultExchange("events"),
    rabbitmq.WithPersistent(),
)
if err != nil {
    log.Fatal(err)
}
defer publisher.Close()

// Create and publish message
message := rabbitmq.NewMessage([]byte("Hello, World!")).
    WithContentType("text/plain").
    WithCorrelationID("request-123").
    WithHeaders(map[string]any{
        "source": "notification-service",
        "version": "1.0",
    })

err = publisher.Publish(ctx, "events", "user.created", message)
if err != nil {
    log.Fatal(err)
}
```

🔝 [back to top](#main-package-api-reference)

&nbsp;

### Publishing with Delivery Assurance

Delivery assurance provides reliable message delivery guarantees through asynchronous callbacks, eliminating the need to manually handle publisher confirms and returns.

#### Basic Usage

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
            log.Printf("⚠ Message %s nacked: %s", messageID, errorMessage)
        case rabbitmq.DeliveryTimeout:
            log.Printf("⏱ Message %s timed out", messageID)
        }
    }),
)
if err != nil {
    log.Fatal(err)
}
defer publisher.Close()

// Publish with delivery assurance
message := rabbitmq.NewMessage([]byte("Important event"))
err = publisher.PublishWithDeliveryAssurance(
    ctx,
    "events",
    "user.created",
    message,
    rabbitmq.DeliveryOptions{
        MessageID: "event-123",
        Mandatory: true,
    },
)
if err != nil {
    log.Fatal(err)
}
```

#### Publisher Options for Delivery Assurance

| Option | Description |
|--------|-------------|
| `WithDeliveryAssurance()` | Enables delivery assurance with asynchronous confirmation tracking |
| `WithDefaultDeliveryCallback(callback)` | Sets the default callback for delivery outcomes |
| `WithDeliveryTimeout(duration)` | Sets the timeout for delivery confirmations (default: 30s) |
| `WithMandatoryByDefault(bool)` | Makes all messages mandatory by default |

#### Delivery Options

```go
type DeliveryOptions struct {
    MessageID   string           // Unique identifier for tracking
    Mandatory   bool             // Return message if no queue bound
    Callback    DeliveryCallback // Per-message callback (overrides default)
    Timeout     time.Duration    // Per-message timeout (overrides default)
    RetryOnNack bool             // Retry on broker nack
    MaxRetries  int              // Maximum retry attempts
}
```

#### Delivery Outcomes

- **DeliverySuccess**: Message confirmed by broker and successfully routed
- **DeliveryFailed**: Message returned by broker (no queue bound to routing key)
- **DeliveryNacked**: Message negatively acknowledged by broker
- **DeliveryTimeout**: No confirmation received within timeout period

#### Advanced Usage with Per-Message Callbacks

```go
err = publisher.PublishWithDeliveryAssurance(
    ctx,
    "events",
    "critical.event",
    message,
    rabbitmq.DeliveryOptions{
        MessageID: "critical-123",
        Mandatory: true,
        Timeout:   10 * time.Second,
        Callback: func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
            // Custom handling for this specific message
            if outcome != rabbitmq.DeliverySuccess {
                alerting.SendAlert("Critical message failed", messageID, errorMessage)
                retryQueue.Add(messageID)
            }
        },
    },
)
```

#### Monitoring Delivery Statistics

```go
stats := publisher.GetDeliveryStats()
log.Printf("Delivery Statistics:")
log.Printf("  Total Published:  %d", stats.TotalPublished)
log.Printf("  Total Confirmed:  %d", stats.TotalConfirmed)
log.Printf("  Total Returned:   %d", stats.TotalReturned)
log.Printf("  Total Nacked:     %d", stats.TotalNacked)
log.Printf("  Total Timed Out:  %d", stats.TotalTimedOut)
log.Printf("  Pending Messages: %d", stats.PendingMessages)
```

#### Key Benefits

- **Asynchronous**: Non-blocking publish operations with callback-based notifications
- **Reliable**: Tracks confirmations, returns, and nacks automatically
- **Thread-Safe**: Fully concurrent publishing support
- **Observable**: Built-in statistics and metrics
- **Flexible**: Per-message or default callbacks

#### Best Practices

1. **Always set unique message IDs** for tracking and correlation
2. **Use mandatory flag** for critical messages to detect routing failures
3. **Monitor statistics** regularly to detect delivery issues
4. **Handle all outcomes** in callbacks (success, failed, nacked, timeout)
5. **Set appropriate timeouts** based on your SLA requirements
6. **Close publishers gracefully** to ensure pending confirmations are handled

See the [delivery-assurance example](../../examples/delivery-assurance) for complete working examples.

🔝 [back to top](#main-package-api-reference)

&nbsp;

### Consuming Messages

```go
// Create consumer
consumer, err := client.NewConsumer(
    rabbitmq.WithPrefetchCount(10),
    rabbitmq.WithConcurrency(5),
)
if err != nil {
    log.Fatal(err)
}
defer consumer.Close()

// Define message handler
handler := func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    log.Printf("Received: %s", string(delivery.Body))
    return delivery.Ack()
}

// Start consuming
err = consumer.Consume(ctx, "user-events", handler)
if err != nil {
    log.Fatal(err)
}
```

🔝 [back to top](#main-package-api-reference)

&nbsp;

### Topology Management

```go
// Get admin service
admin := client.Admin()

// Declare exchange
err := admin.DeclareExchange(ctx, "events", "topic",
    rabbitmq.WithExchangeDurable(),
)
if err != nil {
    log.Fatal(err)
}

// Declare queue
queue, err := admin.DeclareQueue(ctx, "user-events",
    rabbitmq.WithDurable(),
    rabbitmq.WithTTL(24*time.Hour),
)
if err != nil {
    log.Fatal(err)
}

// Bind queue to exchange
err = admin.BindQueue(ctx, "user-events", "events", "user.*")
if err != nil {
    log.Fatal(err)
}
```

🔝 [back to top](#main-package-api-reference)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
