# Protobuf Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [Protobuf Package](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides the complete API reference for the `protobuf` sub-package. The protobuf package provides support for Protocol Buffer message serialization, routing, and type-safe message handling for RabbitMQ applications.

&nbsp;

## Message Creation Functions

| Function | Description |
|----------|-------------|
| `NewMessage(body []byte)` | Creates a new basic message with the given body |
| `NewProtobufMessage(pb proto.Message)` | Creates a new Message from a Protocol Buffers message with automatic serialization and type headers |

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## Message Interface Methods

| Function | Description |
|----------|-------------|
| `WithContentType(contentType string)` | Sets the content type for the message and returns the updated message |
| `WithHeader(key string, value any)` | Adds a header to the message and returns the updated message |

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## Message Multiplexer Functions

| Function | Description |
|----------|-------------|
| `NewMessageMux()` | Creates a new message multiplexer for type-safe protobuf message routing |
| `RegisterHandler[T proto.Message](mux *MessageMux, handlerFunc func(ctx context.Context, msg T) error)` | Registers a typed protobuf handler for messages of type T |

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## MessageMux Methods

| Function | Description |
|----------|-------------|
| `SetDefaultHandler(handlerFunc func(ctx context.Context, delivery *Delivery) error)` | Sets the default handler for non-protobuf messages |
| `SetUnknownProtobufHandler(handlerFunc func(ctx context.Context, messageType string, data []byte) error)` | Sets handler for unknown protobuf message types |
| `Handle(ctx context.Context, delivery *Delivery)` | Routes and handles incoming messages using registered handlers |
| `GetRegisteredTypes()` | Returns a list of all registered protobuf message types |

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## Utility Functions

| Function | Description |
|----------|-------------|
| `IsProtobufMessage(delivery *Delivery)` | Checks if a delivery contains a protobuf message |
| `GetProtobufMessageType(delivery *Delivery)` | Extracts the protobuf message type from delivery headers |

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## Serializer Functions

| Function | Description |
|----------|-------------|
| `NewSerializer()` | Creates a new protobuf serializer for message marshaling/unmarshaling |

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## Serializer Methods

| Function | Description |
|----------|-------------|
| `Serialize(msg any)` | Serializes a protobuf message to bytes |
| `Deserialize(data []byte, target any)` | Deserializes bytes into a protobuf message |
| `ContentType()` | Returns the content type for protobuf messages ("application/protobuf") |

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## Internal Methods

| Function | Description |
|----------|-------------|
| `handleDefault(ctx context.Context, delivery *Delivery)` | Handles messages using the default handler (called internally) |
| `handleUnknownProtobuf(ctx context.Context, messageType string, data []byte)` | Handles unknown protobuf types (called internally) |

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## Types and Structures

| Type | Description |
|------|-------------|
| `Message` | Interface for RabbitMQ messages with content type and header support |
| `Delivery` | Wrapper around amqp.Delivery with additional timestamp information |
| `BasicMessage` | Basic implementation of the Message interface |
| `MessageMux` | Message multiplexer for automatic routing of protobuf messages to type-safe handlers |
| `ProtobufHandler` | Internal structure holding type-safe handler information |
| `Serializer` | Protobuf serializer for message marshaling/unmarshaling |

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## Constants

| Constant | Description |
|----------|-------------|
| `ContentTypeProtobuf` | Content type constant for protobuf messages ("application/protobuf") |

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## Usage Examples

&nbsp;

### Creating and Publishing Protobuf Messages

```go
import (
    "github.com/cloudresty/go-rabbitmq/protobuf"
    "your-project/proto/events" // Your protobuf definitions
)

// Create a protobuf message
userEvent := &events.UserCreated{
    UserId: "12345",
    Email:  "user@example.com",
    Name:   "John Doe",
}

// Convert to RabbitMQ message
message, err := protobuf.NewProtobufMessage(userEvent)
if err != nil {
    log.Fatal(err)
}

// Publish the message
err = publisher.Publish(ctx, "events", "user.created", message)
if err != nil {
    log.Fatal(err)
}
```

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

### Type-Safe Message Handling

```go
import "github.com/cloudresty/go-rabbitmq/protobuf"

// Create message multiplexer
mux := protobuf.NewMessageMux()

// Register typed handlers
protobuf.RegisterHandler(mux, func(ctx context.Context, msg *events.UserCreated) error {
    log.Printf("User created: %s (%s)", msg.Name, msg.Email)
    return nil
})

protobuf.RegisterHandler(mux, func(ctx context.Context, msg *events.OrderPlaced) error {
    log.Printf("Order placed: %s for $%.2f", msg.OrderId, msg.Amount)
    return nil
})

// Set handlers for edge cases
mux.SetDefaultHandler(func(ctx context.Context, delivery *protobuf.Delivery) error {
    log.Printf("Non-protobuf message received: %s", delivery.Body)
    return nil
})

mux.SetUnknownProtobufHandler(func(ctx context.Context, messageType string, data []byte) error {
    log.Printf("Unknown protobuf type: %s", messageType)
    return nil
})

// Use in consumer
consumer.Consume(ctx, "events", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    // Convert to protobuf delivery
    pbDelivery := &protobuf.Delivery{Delivery: delivery.Delivery}
    return mux.Handle(ctx, pbDelivery)
})
```

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

### Message Type Detection

```go
// Check if message is protobuf
if protobuf.IsProtobufMessage(delivery) {
    messageType, ok := protobuf.GetProtobufMessageType(delivery)
    if ok {
        log.Printf("Received protobuf message of type: %s", messageType)
    }
} else {
    log.Println("Received non-protobuf message")
}
```

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

### Custom Serialization

```go
// Create serializer
serializer := protobuf.NewSerializer()

// Serialize message
data, err := serializer.Serialize(userEvent)
if err != nil {
    log.Fatal(err)
}

// Deserialize message
var received events.UserCreated
err = serializer.Deserialize(data, &received)
if err != nil {
    log.Fatal(err)
}

log.Printf("Content type: %s", serializer.ContentType())
```

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

## Best Practices

1. **Type Safety**: Use `RegisterHandler[T]()` for compile-time type safety when handling protobuf messages
2. **Message Types**: Always set meaningful message types in your protobuf definitions for proper routing
3. **Error Handling**: Implement proper error handling in your message handlers
4. **Default Handlers**: Set default and unknown protobuf handlers to gracefully handle unexpected messages
5. **Message Validation**: Validate protobuf messages in your handlers before processing
6. **Content Type**: The package automatically sets the correct content type, but you can override it if needed
7. **Header Usage**: Use message headers for routing metadata while keeping business data in the protobuf message body

🔝 [back to top](#protobuf-package-api-reference)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
