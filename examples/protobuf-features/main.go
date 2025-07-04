package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudresty/go-rabbitmq/protobuf"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	fmt.Println("=== Protobuf Feature Comparison ===")

	// Create a sample timestamp (simulating a protobuf message)
	timestamp := timestamppb.Now()

	// Example 1: Protobuf message creation (sub-package only)
	fmt.Println("\n1. Protobuf Message Creation:")

	// Use the protobuf sub-package function
	basicMessage, err := protobuf.NewProtobufMessage(timestamp)
	if err != nil {
		log.Fatalf("Failed to create protobuf message: %v", err)
	}

	// Convert to BasicMessage to access fields
	if basicMsg, ok := basicMessage.(*protobuf.BasicMessage); ok {
		fmt.Printf("✓ Created message with content type: %s\n", basicMsg.ContentType)
		fmt.Printf("✓ Message size: %d bytes\n", len(basicMsg.Body))

		// Check message properties directly from the message
		if basicMsg.ContentType == protobuf.ContentTypeProtobuf {
			if messageType, exists := basicMsg.Headers["x-message-type"]; exists {
				fmt.Printf("✓ Message type detected: %s\n", messageType)
			}
		}
	}

	// Example 2: Using the new MessageSerializer interface
	fmt.Println("\n2. Using MessageSerializer Interface (Contract-Implementation Pattern):")

	// Create a protobuf serializer from the sub-package
	serializer := protobuf.NewSerializer()
	fmt.Printf("✓ Protobuf serializer content type: %s\n", serializer.ContentType())

	// Serialize a protobuf message
	serializedData, err := serializer.Serialize(timestamp)
	if err != nil {
		log.Fatalf("Failed to serialize protobuf message: %v", err)
	}
	fmt.Printf("✓ Serialized message size: %d bytes\n", len(serializedData))

	// Deserialize back to a protobuf message
	deserializedTimestamp := &timestamppb.Timestamp{}
	err = serializer.Deserialize(serializedData, deserializedTimestamp)
	if err != nil {
		log.Fatalf("Failed to deserialize protobuf message: %v", err)
	}
	fmt.Printf("✓ Deserialized timestamp: %v\n", deserializedTimestamp.AsTime())

	// Example 3: Advanced protobuf features from sub-package
	fmt.Println("\n3. Advanced Protobuf (Sub-Package):")

	// Create a protobuf message using the advanced sub-package
	_, err = protobuf.NewProtobufMessage(timestamp)
	if err != nil {
		log.Fatalf("Failed to create advanced protobuf message: %v", err)
	}

	fmt.Printf("✓ Advanced message created with additional features\n")
	fmt.Printf("✓ Message has protobuf metadata and routing capabilities\n")

	// Example 4: Message routing capabilities (sub-package)
	fmt.Println("\n4. Advanced Message Routing (Sub-Package):")

	// Create a message multiplexer (from protobuf sub-package)
	mux := protobuf.NewMessageMux()

	// Register a type-safe handler for timestamp messages
	protobuf.RegisterHandler[*timestamppb.Timestamp](mux, func(ctx context.Context, ts *timestamppb.Timestamp) error {
		fmt.Printf("✓ Received timestamp: %v\n", ts.AsTime())
		return nil
	})

	// Show registered message types
	registeredTypes := mux.GetRegisteredTypes()
	fmt.Printf("✓ Registered message types: %v\n", registeredTypes)

	// Example 5: Feature summary
	fmt.Println("\n=== Feature Summary ===")

	fmt.Println("Protobuf Sub-Package (contract-implementation pattern):")
	fmt.Println("  ✓ NewProtobufMessage() - Create protobuf messages")
	fmt.Println("  ✓ IsProtobufMessage() - Check if message is protobuf")
	fmt.Println("  ✓ ContentTypeProtobuf constant")
	fmt.Println("  ✓ MessageSerializer interface implementation")
	fmt.Println("  ✓ NewSerializer() - Contract-implementation pattern")
	fmt.Println("  ✓ Type-safe message handlers")
	fmt.Println("  ✓ Automatic message routing")
	fmt.Println("  ✓ Message multiplexing")
	fmt.Println("  ✓ Unknown protobuf type handling")
	fmt.Println("  ✓ Default fallback handlers")

	fmt.Println("\n=== Recommendation ===")
	fmt.Println("• Use protobuf.NewProtobufMessage() for protobuf message creation")
	fmt.Println("• Use protobuf.NewSerializer() with publisher/consumer WithSerializer() options")
	fmt.Println("• Use protobuf sub-package for complex routing and type-safe handlers")
}
