package protobuf

import (
	"context"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestNewMessageMux(t *testing.T) {
	mux := NewMessageMux()

	if mux == nil {
		t.Error("NewMessageMux() returned nil")
	}

	if mux.handlers == nil {
		t.Error("Message mux handlers map is nil")
	}

	if len(mux.handlers) != 0 {
		t.Error("Expected empty handlers map")
	}
}

func TestMessageMux_SetDefaultHandler(t *testing.T) {
	mux := NewMessageMux()

	called := false
	handler := func(ctx context.Context, delivery *Delivery) error {
		called = true
		return nil
	}

	mux.SetDefaultHandler(handler)

	// Create a non-protobuf delivery
	delivery := &Delivery{
		Delivery: amqp.Delivery{
			ContentType: "application/json",
			Body:        []byte(`{"message": "test"}`),
		},
		ReceivedAt: time.Now(),
	}

	err := mux.Handle(context.Background(), delivery)
	if err != nil {
		t.Fatalf("Failed to handle delivery: %v", err)
	}

	if !called {
		t.Error("Default handler was not called")
	}
}

func TestMessageMux_SetUnknownProtobufHandler(t *testing.T) {
	mux := NewMessageMux()

	var receivedMessageType string
	var receivedData []byte

	handler := func(ctx context.Context, messageType string, data []byte) error {
		receivedMessageType = messageType
		receivedData = data
		return nil
	}

	mux.SetUnknownProtobufHandler(handler)

	// Create a protobuf delivery with unknown type
	delivery := &Delivery{
		Delivery: amqp.Delivery{
			ContentType: ContentTypeProtobuf,
			Body:        []byte("test protobuf data"),
			Headers: amqp.Table{
				"x-message-type": "unknown.MessageType",
			},
		},
		ReceivedAt: time.Now(),
	}

	err := mux.Handle(context.Background(), delivery)
	if err != nil {
		t.Fatalf("Failed to handle delivery: %v", err)
	}

	if receivedMessageType != "unknown.MessageType" {
		t.Errorf("Expected message type 'unknown.MessageType', got %s", receivedMessageType)
	}

	if string(receivedData) != "test protobuf data" {
		t.Errorf("Expected data 'test protobuf data', got %s", string(receivedData))
	}
}

func TestIsProtobufMessage(t *testing.T) {
	tests := []struct {
		name     string
		delivery *Delivery
		expected bool
	}{
		{
			name: "Valid protobuf message",
			delivery: &Delivery{
				Delivery: amqp.Delivery{
					ContentType: ContentTypeProtobuf,
					Headers: amqp.Table{
						"x-message-type": "test.Message",
					},
				},
			},
			expected: true,
		},
		{
			name: "Wrong content type",
			delivery: &Delivery{
				Delivery: amqp.Delivery{
					ContentType: "application/json",
					Headers: amqp.Table{
						"x-message-type": "test.Message",
					},
				},
			},
			expected: false,
		},
		{
			name: "Missing message type header",
			delivery: &Delivery{
				Delivery: amqp.Delivery{
					ContentType: ContentTypeProtobuf,
					Headers:     amqp.Table{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsProtobufMessage(tt.delivery)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetProtobufMessageType(t *testing.T) {
	tests := []struct {
		name         string
		delivery     *Delivery
		expectedType string
		expectedOk   bool
	}{
		{
			name: "Valid protobuf message",
			delivery: &Delivery{
				Delivery: amqp.Delivery{
					ContentType: ContentTypeProtobuf,
					Headers: amqp.Table{
						"x-message-type": "test.Message",
					},
				},
			},
			expectedType: "test.Message",
			expectedOk:   true,
		},
		{
			name: "Non-protobuf message",
			delivery: &Delivery{
				Delivery: amqp.Delivery{
					ContentType: "application/json",
					Headers: amqp.Table{
						"x-message-type": "test.Message",
					},
				},
			},
			expectedType: "",
			expectedOk:   false,
		},
		{
			name: "Invalid header type",
			delivery: &Delivery{
				Delivery: amqp.Delivery{
					ContentType: ContentTypeProtobuf,
					Headers: amqp.Table{
						"x-message-type": 123, // Not a string
					},
				},
			},
			expectedType: "",
			expectedOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messageType, ok := GetProtobufMessageType(tt.delivery)
			if messageType != tt.expectedType {
				t.Errorf("Expected message type %s, got %s", tt.expectedType, messageType)
			}
			if ok != tt.expectedOk {
				t.Errorf("Expected ok %v, got %v", tt.expectedOk, ok)
			}
		})
	}
}

func TestBasicMessage_WithMethods(t *testing.T) {
	msg := NewMessage([]byte("test data"))

	// Test WithContentType
	msg = msg.WithContentType("application/json").(*BasicMessage)
	if msg.ContentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got %s", msg.ContentType)
	}

	// Test WithHeader
	msg = msg.WithHeader("test-key", "test-value").(*BasicMessage)
	value, exists := msg.Headers["test-key"]
	if !exists {
		t.Error("Expected header 'test-key' to be set")
	}
	if value != "test-value" {
		t.Errorf("Expected header value 'test-value', got %v", value)
	}

	// Test chaining
	msg = msg.WithHeader("another-key", 123).(*BasicMessage)
	value2, exists2 := msg.Headers["another-key"]
	if !exists2 {
		t.Error("Expected header 'another-key' to be set")
	}
	if value2 != 123 {
		t.Errorf("Expected header value 123, got %v", value2)
	}
}

func TestContentTypeProtobuf(t *testing.T) {
	expected := "application/protobuf"
	if ContentTypeProtobuf != expected {
		t.Errorf("Expected ContentTypeProtobuf to be %s, got %s", expected, ContentTypeProtobuf)
	}
}

func TestMessageMux_HandleWithoutHandlers(t *testing.T) {
	mux := NewMessageMux()

	// No default handler set, should return error
	delivery := &Delivery{
		Delivery: amqp.Delivery{
			ContentType: "application/json",
			Body:        []byte(`{"message": "test"}`),
		},
		ReceivedAt: time.Now(),
	}

	err := mux.Handle(context.Background(), delivery)
	if err == nil {
		t.Error("Expected error when no default handler is set")
	}

	expectedMsg := "no default handler registered for non-protobuf message"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestMessageMux_HandleProtobufWithoutTypeHeader(t *testing.T) {
	mux := NewMessageMux()

	called := false
	mux.SetDefaultHandler(func(ctx context.Context, delivery *Delivery) error {
		called = true
		return nil
	})

	// Protobuf content type but missing x-message-type header
	delivery := &Delivery{
		Delivery: amqp.Delivery{
			ContentType: ContentTypeProtobuf,
			Body:        []byte("protobuf data"),
			Headers:     amqp.Table{}, // No x-message-type header
		},
		ReceivedAt: time.Now(),
	}

	err := mux.Handle(context.Background(), delivery)
	if err != nil {
		t.Fatalf("Failed to handle delivery: %v", err)
	}

	if !called {
		t.Error("Default handler should be called for protobuf message without type header")
	}
}

func TestMessageMux_GetRegisteredTypes_Empty(t *testing.T) {
	mux := NewMessageMux()

	types := mux.GetRegisteredTypes()
	if len(types) != 0 {
		t.Errorf("Expected empty types list, got %d types", len(types))
	}
}
