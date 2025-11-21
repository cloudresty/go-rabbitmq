# Protocol Buffer Package

[Home](../README.md) &nbsp;/&nbsp; Protocol Buffer Package

&nbsp;

This package provides comprehensive Protocol Buffer (protobuf) message support for the go-rabbitmq library, including automatic serialization, type-safe message routing, and message multiplexing capabilities.

&nbsp;

## Features

- **Automatic Serialization**: Seamlessly convert protobuf messages to RabbitMQ messages
- **Type-Safe Routing**: Register handlers for specific protobuf message types with compile-time safety
- **Message Multiplexing**: Route different protobuf message types to appropriate handlers
- **Fallback Handling**: Handle non-protobuf messages and unknown protobuf types
- **Header Management**: Automatic setting of content type and message type headers

🔝 [back to top](#protocol-buffer-package)

&nbsp;

## Installation

```bash
go get google.golang.org/protobuf
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

## Basic Usage

### Creating Protobuf Messages

```go
import (
    "github.com/cloudresty/go-rabbitmq/protobuf"
    pb "your-project/protos" // Your generated protobuf code
)

// Create a protobuf message
userEvent := &pb.UserCreated{
    UserId: "12345",
    Email:  "user@example.com",
    Name:   "John Doe",
}

// Convert to RabbitMQ message (automatically sets headers and content type)
message, err := protobuf.NewProtobufMessage(userEvent)
if err != nil {
    log.Fatal(err)
}

// Publish the message
err = publisher.Publish(message, "user.events", "user.created")
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

### Type-Safe Message Handling

```go
import (
    "context"
    "github.com/cloudresty/go-rabbitmq/protobuf"
    pb "your-project/protos"
)

// Create a message multiplexer
mux := protobuf.NewMessageMux()

// Register type-safe handlers for specific protobuf message types
protobuf.RegisterHandler[*pb.UserCreated](mux, func(ctx context.Context, msg *pb.UserCreated) error {
    log.Printf("User created: %s (%s)", msg.Name, msg.Email)
    // Handle user creation logic
    return nil
})

protobuf.RegisterHandler[*pb.UserUpdated](mux, func(ctx context.Context, msg *pb.UserUpdated) error {
    log.Printf("User updated: %s", msg.UserId)
    // Handle user update logic
    return nil
})

// Set a default handler for non-protobuf messages
mux.SetDefaultHandler(func(ctx context.Context, delivery *protobuf.Delivery) error {
    log.Printf("Received non-protobuf message: %s", string(delivery.Body))
    return nil
})

// Set a handler for unknown protobuf types
mux.SetUnknownProtobufHandler(func(ctx context.Context, messageType string, data []byte) error {
    log.Printf("Unknown protobuf message type: %s", messageType)
    return nil
})

// Use the multiplexer as a message handler
consumer.StartConsuming(mux.Handle, "user.events", []string{"user.*"})
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

### Message Inspection

```go
// Check if a delivery contains a protobuf message
if protobuf.IsProtobufMessage(delivery) {
    messageType, ok := protobuf.GetProtobufMessageType(delivery)
    if ok {
        log.Printf("Received protobuf message of type: %s", messageType)
    }
}

// Get all registered message types
types := mux.GetRegisteredTypes()
log.Printf("Registered types: %v", types)
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

## Advanced Usage

### Custom Message Types

The package works with any protobuf message that implements the `proto.Message` interface. Make sure your protobuf files are properly generated:

```bash
protoc --go_out=. --go_opt=paths=source_relative your_proto_file.proto
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

### Error Handling

The package provides detailed error information for common scenarios:

```go
message, err := protobuf.NewProtobufMessage(userEvent)
if err != nil {
    // Handle serialization errors
    log.Printf("Failed to create protobuf message: %v", err)
}
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

### Integration with Main Package

When using with the main rabbitmq package, you can create a bridge:

```go
import (
    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/protobuf"
)

// Convert protobuf message to main package message
protobufMsg, err := protobuf.NewProtobufMessage(userEvent)
if err != nil {
    return err
}

// Create main package message with the same properties
mainMsg := rabbitmq.NewMessage(protobufMsg.(*protobuf.BasicMessage).Body).
    WithContentType(protobuf.ContentTypeProtobuf)

for key, value := range protobufMsg.(*protobuf.BasicMessage).Headers {
    mainMsg = mainMsg.WithHeader(key, value)
}
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

## Best Practices

### 1. Message Type Consistency

Ensure protobuf message types are consistent across publishers and consumers:

```go
// Good: Use consistent message types
protobuf.RegisterHandler[*pb.UserCreated](mux, handleUserCreated)

// Avoid: Mixing different versions of the same message type
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

### 2. Handler Organization

Group related handlers together and use clear naming:

```go
// User-related handlers
protobuf.RegisterHandler[*pb.UserCreated](mux, handleUserCreated)
protobuf.RegisterHandler[*pb.UserUpdated](mux, handleUserUpdated)
protobuf.RegisterHandler[*pb.UserDeleted](mux, handleUserDeleted)

// Order-related handlers
protobuf.RegisterHandler[*pb.OrderCreated](mux, handleOrderCreated)
protobuf.RegisterHandler[*pb.OrderShipped](mux, handleOrderShipped)
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

### 3. Error Handling

Always handle serialization and deserialization errors:

```go
protobuf.RegisterHandler[*pb.UserCreated](mux, func(ctx context.Context, msg *pb.UserCreated) error {
    if err := validateUser(msg); err != nil {
        return fmt.Errorf("invalid user data: %w", err)
    }

    return processUser(msg)
})
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

### 4. Schema Evolution

Design your protobuf schemas with forward and backward compatibility in mind:

```protobuf
syntax = "proto3";

message UserCreated {
    string user_id = 1;
    string email = 2;
    string name = 3;

    // New fields should be optional and have higher field numbers
    string phone = 4;     // Added in v2
    string company = 5;   // Added in v3
}
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

## Migration from Root Package

If you're migrating from protobuf functionality in the root package:

1. **Update imports**:

```go
// Old
import "github.com/cloudresty/go-rabbitmq"

// New
import "github.com/cloudresty/go-rabbitmq/protobuf"
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

2. **Update function calls**:

```go
// Old
message, err := rabbitmq.NewProtobufMessage(userEvent)

// New
message, err := protobuf.NewProtobufMessage(userEvent)
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

3. **Update handler registration**:

```go
// Old
rabbitmq.RegisterHandler[*pb.UserCreated](mux, handler)

// New
protobuf.RegisterHandler[*pb.UserCreated](mux, handler)
```

🔝 [back to top](#protocol-buffer-package)

&nbsp;

## Dependencies

- `google.golang.org/protobuf/proto` - Protocol Buffer runtime library
- `github.com/rabbitmq/amqp091-go` - RabbitMQ client library

🔝 [back to top](#protocol-buffer-package)

&nbsp;

## Performance Considerations

- **Message Size**: Protobuf messages are generally smaller than JSON, improving network performance
- **Serialization Speed**: Protobuf serialization is typically faster than JSON
- **Type Safety**: Compile-time type checking reduces runtime errors
- **Memory Usage**: The multiplexer uses reflection for type handling, which has minimal overhead

🔝 [back to top](#protocol-buffer-package)

&nbsp;

## Troubleshooting

### Common Issues

1. **Message Type Not Found**

```text
Error: No handler registered for protobuf type 'your.package.MessageType'
```

Solution: Ensure the handler is registered before starting consumption.

🔝 [back to top](#protocol-buffer-package)

&nbsp;

2. **Serialization Errors**

```text
Error: failed to marshal protobuf message
```

Solution: Verify the protobuf message is properly initialized and valid.

🔝 [back to top](#protocol-buffer-package)

&nbsp;

3. **Type Mismatch**

```text
Error: message type mismatch: expected *pb.UserCreated, got *pb.UserUpdated
```

Solution: Check message type headers and handler registration.

🔝 [back to top](#protocol-buffer-package)

&nbsp;

For more examples, see the `examples/` directory in the main repository.

🔝 [back to top](#protocol-buffer-package)

&nbsp;

---

&nbsp;

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
