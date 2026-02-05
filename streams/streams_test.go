package streams

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cloudresty/go-rabbitmq"
)

func TestHandler_ContractImplementationPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if stream port is available before attempting connection
	// This prevents the rabbitmq-stream-go-client from logging errors to stderr
	host := "localhost"
	port := 5552
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "5552"), 2*time.Second)
	if err != nil {
		t.Skip("RabbitMQ streams not available for testing (port 5552 not reachable)")
	}
	_ = conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create streams handler using native stream protocol (port 5552)
	handler, err := NewHandler(Options{
		Host:     host,
		Port:     port,
		Username: "guest",
		Password: "guest",
	})
	if err != nil {
		t.Skip("RabbitMQ streams not available for testing (port 5552)")
	}
	defer func() {
		if err := handler.Close(); err != nil {
			t.Errorf("Failed to close handler: %v", err)
		}
	}()

	// Verify handler implements StreamHandler interface
	var _ rabbitmq.StreamHandler = handler

	streamName := "test-stream"

	// Delete the stream first to ensure we start with a clean slate
	err = handler.DeleteStream(ctx, streamName)
	if err != nil {
		t.Logf("Stream might not exist yet (this is fine): %v", err)
	}

	// Test stream creation
	streamConfig := rabbitmq.StreamConfig{
		MaxAge:            24 * time.Hour,
		MaxLengthMessages: 1000,
	}

	err = handler.CreateStream(ctx, streamName, streamConfig)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}
	// Test publishing to stream
	message := rabbitmq.NewMessage([]byte("test stream message"))
	// Store the auto-generated ULID for comparison
	expectedMessageID := message.MessageID

	err = handler.PublishToStream(ctx, streamName, message)
	if err != nil {
		t.Fatalf("Failed to publish to stream: %v", err)
	}

	// Test consuming from stream
	received := make(chan *rabbitmq.Delivery, 1)
	messageHandler := func(ctx context.Context, delivery *rabbitmq.Delivery) error {
		received <- delivery
		return nil
	}

	// Start consuming in background
	consumeCtx, consumeCancel := context.WithTimeout(ctx, 5*time.Second)
	defer consumeCancel()

	go func() {
		err := handler.ConsumeFromStream(consumeCtx, streamName, messageHandler)
		if err != nil {
			t.Logf("Consume ended: %v", err)
		}
	}()
	// Wait for message
	select {
	case delivery := <-received:
		if string(delivery.Body) != "test stream message" {
			t.Errorf("Expected 'test stream message', got %s", string(delivery.Body))
		}

		// Verify that message ID (ULID) is preserved through streams
		if delivery.MessageId != expectedMessageID {
			t.Errorf("Message ID not preserved! Expected '%s', got '%s'", expectedMessageID, delivery.MessageId)
		} else {
			t.Log("SUCCESS: Message ID properly preserved through RabbitMQ streams!")
		}
		t.Log("Successfully received message from stream")
	case <-time.After(10 * time.Second):
		t.Error("Timeout waiting for stream message")
	}
}
