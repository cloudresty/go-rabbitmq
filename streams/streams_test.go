package streams

import (
	"context"
	"testing"
	"time"

	"github.com/cloudresty/go-rabbitmq"
)

func TestHandler_ContractImplementationPattern(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client
	client, err := rabbitmq.NewClient(
		rabbitmq.WithCredentials("guest", "guest"),
		rabbitmq.WithHosts("localhost:5672"),
		rabbitmq.WithConnectionName("streams-test"),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create streams handler using contract-implementation pattern
	handler := NewHandler(client)

	// Test stream creation
	streamConfig := rabbitmq.StreamConfig{
		MaxAge:            24 * time.Hour,
		MaxLengthMessages: 1000,
	}

	streamName := "test-stream"
	err = handler.CreateStream(ctx, streamName, streamConfig)
	if err != nil {
		t.Logf("Stream might already exist: %v", err)
	}

	// Test publishing to stream
	message := rabbitmq.NewMessage([]byte("test stream message"))
	message.MessageID = "test-msg-1"

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
		if delivery.MessageId != "test-msg-1" {
			t.Errorf("Expected message ID 'test-msg-1', got %s", delivery.MessageId)
		}
		t.Log("Successfully received message from stream")
	case <-time.After(10 * time.Second):
		t.Error("Timeout waiting for stream message")
	}
}
