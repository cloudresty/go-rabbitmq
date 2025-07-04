// Package protobuf provides support for Protocol Buffer message serialization and routing
// for the RabbitMQ library. It includes automatic marshaling/unmarshaling of protobuf
// messages and type-safe message routing capabilities.
package protobuf

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"

	"github.com/cloudresty/go-rabbitmq"
)

// Protobuf content type constant
const ContentTypeProtobuf = "application/protobuf"

// Message represents a RabbitMQ message (minimal interface for the protobuf package)
type Message interface {
	WithContentType(contentType string) Message
	WithHeader(key string, value any) Message
}

// Delivery represents a RabbitMQ delivery (minimal interface for the protobuf package)
type Delivery struct {
	amqp.Delivery
	ReceivedAt time.Time
}

// BasicMessage provides a basic implementation of the Message interface
type BasicMessage struct {
	Body        []byte
	ContentType string
	Headers     map[string]any
}

// NewMessage creates a new basic message with the given body
func NewMessage(body []byte) *BasicMessage {
	return &BasicMessage{
		Body:    body,
		Headers: make(map[string]any),
	}
}

// WithContentType sets the content type for the message
func (m *BasicMessage) WithContentType(contentType string) Message {
	m.ContentType = contentType
	return m
}

// WithHeader adds a header to the message
func (m *BasicMessage) WithHeader(key string, value any) Message {
	m.Headers[key] = value
	return m
}

// NewProtobufMessage creates a new Message from a Protocol Buffers message
// This function automatically:
// 1. Serializes the protobuf message using proto.Marshal()
// 2. Sets the content type to "application/protobuf"
// 3. Sets the "x-message-type" header with the fully qualified message type name
func NewProtobufMessage(pb proto.Message) (Message, error) {
	if pb == nil {
		return nil, fmt.Errorf("protobuf message cannot be nil")
	}

	// Serialize the protobuf message
	data, err := proto.Marshal(pb)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal protobuf message: %w", err)
	}

	// Get the fully qualified message type name
	messageType := string(pb.ProtoReflect().Descriptor().FullName())

	// Create the message with protobuf-specific headers
	message := NewMessage(data).
		WithContentType(ContentTypeProtobuf).
		WithHeader("x-message-type", messageType)

	return message, nil
}

// ProtobufHandler represents a type-safe handler for a specific protobuf message type
type ProtobufHandler struct {
	messageType reflect.Type
	handler     func(ctx context.Context, msg proto.Message) error
}

// MessageMux provides automatic routing of protobuf messages to type-safe handlers
type MessageMux struct {
	mu              sync.RWMutex
	handlers        map[string]*ProtobufHandler // message type name -> handler
	defaultHandler  func(ctx context.Context, delivery *Delivery) error
	unknownProtobuf func(ctx context.Context, messageType string, data []byte) error
}

// NewMessageMux creates a new message multiplexer for routing protobuf messages
func NewMessageMux() *MessageMux {
	return &MessageMux{
		handlers: make(map[string]*ProtobufHandler),
	}
}

// RegisterHandler registers a type-safe handler for a specific protobuf message type
// The handler function will receive the fully unmarshaled protobuf message
// Usage: RegisterHandler[*pb.UserCreated](mux, func(ctx context.Context, msg *pb.UserCreated) error {...})
func RegisterHandler[T proto.Message](mux *MessageMux, handlerFunc func(ctx context.Context, msg T) error) {
	// Create a new instance to get the message type information
	var zero T

	// Use reflection to create a new instance
	zeroType := reflect.TypeOf((*T)(nil)).Elem()
	if zeroType.Kind() == reflect.Ptr {
		// T is *SomeType, so create new(SomeType)
		elementType := zeroType.Elem()
		newInstance := reflect.New(elementType).Interface().(T)
		zero = newInstance
	} else {
		// T is SomeType, create new instance differently
		newInstance := reflect.New(zeroType).Interface().(T)
		zero = newInstance
	}

	// Get the protobuf message type name
	messageTypeName := string(zero.ProtoReflect().Descriptor().FullName())

	// Wrap the typed handler to work with proto.Message interface
	wrappedHandler := func(ctx context.Context, msg proto.Message) error {
		typedMsg, ok := msg.(T)
		if !ok {
			return fmt.Errorf("message type mismatch: expected %T, got %T", zero, msg)
		}
		return handlerFunc(ctx, typedMsg)
	}

	// Store the handler with type information
	handler := &ProtobufHandler{
		messageType: reflect.TypeOf(zero),
		handler:     wrappedHandler,
	}

	mux.mu.Lock()
	mux.handlers[messageTypeName] = handler
	mux.mu.Unlock()
}

// SetDefaultHandler sets a fallback handler for non-protobuf messages or messages
// without registered handlers. This handler receives the raw delivery.
func (m *MessageMux) SetDefaultHandler(handlerFunc func(ctx context.Context, delivery *Delivery) error) {
	m.mu.Lock()
	m.defaultHandler = handlerFunc
	m.mu.Unlock()
}

// SetUnknownProtobufHandler sets a handler for protobuf messages that don't have
// a registered handler. This handler receives the message type name and raw data.
func (m *MessageMux) SetUnknownProtobufHandler(handlerFunc func(ctx context.Context, messageType string, data []byte) error) {
	m.mu.Lock()
	m.unknownProtobuf = handlerFunc
	m.mu.Unlock()
}

// Handle is the main routing method that implements the MessageHandler interface
// It automatically routes messages based on their type and content
func (m *MessageMux) Handle(ctx context.Context, delivery *Delivery) error {
	// Check if this is a protobuf message
	if delivery.ContentType != ContentTypeProtobuf {
		// Not a protobuf message, use default handler
		return m.handleDefault(ctx, delivery)
	}

	// Get the message type from headers
	messageTypeHeader, exists := delivery.Headers["x-message-type"]
	if !exists {
		// Protobuf message without type header, use default handler
		return m.handleDefault(ctx, delivery)
	}

	messageType, ok := messageTypeHeader.(string)
	if !ok {
		// Invalid message type header, use default handler
		return m.handleDefault(ctx, delivery)
	}

	// Look up the handler for this message type
	m.mu.RLock()
	handler, exists := m.handlers[messageType]
	m.mu.RUnlock()

	if !exists {
		// No handler registered for this protobuf type
		return m.handleUnknownProtobuf(ctx, messageType, delivery.Body)
	}

	// Create a new instance of the target type
	newInstance := reflect.New(handler.messageType.Elem()).Interface().(proto.Message)

	// Unmarshal the protobuf data
	if err := proto.Unmarshal(delivery.Body, newInstance); err != nil {
		return fmt.Errorf("failed to unmarshal protobuf message of type %s: %w", messageType, err)
	}

	// Call the type-safe handler
	return handler.handler(ctx, newInstance)
}

// handleDefault handles non-protobuf messages or messages without handlers
func (m *MessageMux) handleDefault(ctx context.Context, delivery *Delivery) error {
	m.mu.RLock()
	handler := m.defaultHandler
	m.mu.RUnlock()

	if handler == nil {
		return fmt.Errorf("no default handler registered for non-protobuf message")
	}

	return handler(ctx, delivery)
}

// handleUnknownProtobuf handles protobuf messages without registered handlers
func (m *MessageMux) handleUnknownProtobuf(ctx context.Context, messageType string, data []byte) error {
	m.mu.RLock()
	handler := m.unknownProtobuf
	m.mu.RUnlock()

	if handler != nil {
		return handler(ctx, messageType, data)
	}

	// Fall back to default handler if no unknown protobuf handler is set
	// Create a minimal delivery for the default handler
	delivery := &Delivery{
		Delivery: amqp.Delivery{
			ContentType: ContentTypeProtobuf,
			Body:        data,
			Headers: amqp.Table{
				"x-message-type": messageType,
			},
		},
		ReceivedAt: time.Now(),
	}

	return m.handleDefault(ctx, delivery)
}

// GetRegisteredTypes returns a list of all registered protobuf message types
func (m *MessageMux) GetRegisteredTypes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	types := make([]string, 0, len(m.handlers))
	for messageType := range m.handlers {
		types = append(types, messageType)
	}
	return types
}

// IsProtobufMessage checks if a delivery contains a protobuf message
func IsProtobufMessage(delivery *Delivery) bool {
	if delivery.ContentType != ContentTypeProtobuf {
		return false
	}

	_, hasTypeHeader := delivery.Headers["x-message-type"]
	return hasTypeHeader
}

// GetProtobufMessageType extracts the protobuf message type from a delivery
func GetProtobufMessageType(delivery *Delivery) (string, bool) {
	if !IsProtobufMessage(delivery) {
		return "", false
	}

	messageType, ok := delivery.Headers["x-message-type"].(string)
	return messageType, ok
}

// Serializer implements the rabbitmq.MessageSerializer interface for Protocol Buffers
type Serializer struct{}

// NewSerializer creates a new protobuf serializer
func NewSerializer() *Serializer {
	return &Serializer{}
}

// Serialize marshals a protobuf message to bytes
func (s *Serializer) Serialize(msg any) ([]byte, error) {
	protoMsg, ok := msg.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("message must implement proto.Message interface, got %T", msg)
	}

	data, err := proto.Marshal(protoMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal protobuf message: %w", err)
	}

	return data, nil
}

// Deserialize unmarshals bytes to a protobuf message
func (s *Serializer) Deserialize(data []byte, target any) error {
	protoMsg, ok := target.(proto.Message)
	if !ok {
		return fmt.Errorf("target must implement proto.Message interface, got %T", target)
	}

	err := proto.Unmarshal(data, protoMsg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal protobuf message: %w", err)
	}

	return nil
}

// ContentType returns the content type for protobuf messages
func (s *Serializer) ContentType() string {
	return ContentTypeProtobuf
}

// Ensure Serializer implements the interface at compile time
var _ rabbitmq.MessageSerializer = (*Serializer)(nil)
