package rabbitmq

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// Mock connection URL for testing
const testURL = "amqp://guest:guest@localhost:5672/"

func TestConnectionCreation(t *testing.T) {
	// Skip if no RabbitMQ available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := DefaultConnectionConfig(testURL)
	config.RetryAttempts = 1 // Reduce retry attempts for faster tests

	conn, err := NewConnection(config)
	if err != nil {
		t.Skipf("Could not connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	if !conn.IsConnected() {
		t.Error("Expected connection to be active")
	}
}

func TestPublisherCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	publisher, err := NewPublisher(testURL)
	if err != nil {
		t.Skipf("Could not create publisher: %v", err)
	}
	defer publisher.Close()

	if !publisher.IsConnected() {
		t.Error("Expected publisher to be connected")
	}
}

func TestConsumerCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	consumer, err := NewConsumer(testURL)
	if err != nil {
		t.Skipf("Could not create consumer: %v", err)
	}
	defer consumer.Close()

	if !consumer.IsConnected() {
		t.Error("Expected consumer to be connected")
	}
}

func TestPublishAndConsume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create publisher
	publisher, err := NewPublisher(testURL)
	if err != nil {
		t.Skipf("Could not create publisher: %v", err)
	}
	defer publisher.Close()

	// Create consumer
	consumer, err := NewConsumer(testURL)
	if err != nil {
		t.Skipf("Could not create consumer: %v", err)
	}
	defer consumer.Close()

	// Setup test queue
	testQueue := "test-queue-" + time.Now().Format("20060102150405")
	queue, err := consumer.DeclareQueue(testQueue, false, true, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Test message
	testMessage := []byte("Hello, RabbitMQ!")
	messageReceived := make(chan []byte, 1)

	// Start consuming
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		err := consumer.Consume(ctx, ConsumeConfig{
			Queue: queue.Name,
			Handler: func(ctx context.Context, message []byte) error {
				messageReceived <- message
				cancel() // Stop consuming after receiving message
				return nil
			},
		})
		if err != nil && err != context.Canceled {
			t.Errorf("Consumer error: %v", err)
		}
	}()

	// Give consumer time to start
	time.Sleep(100 * time.Millisecond)

	// Publish message
	err = publisher.Publish(ctx, PublishConfig{
		Exchange:   "", // Default exchange
		RoutingKey: queue.Name,
		Message:    testMessage,
	})
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}

	// Wait for message
	select {
	case received := <-messageReceived:
		if !bytes.Equal(received, testMessage) {
			t.Errorf("Expected %s, got %s", string(testMessage), string(received))
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestTopologySetup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	conn, err := NewConnection(DefaultConnectionConfig(testURL))
	if err != nil {
		t.Skipf("Could not create connection: %v", err)
	}
	defer conn.Close()

	testExchange := "test-exchange-" + time.Now().Format("20060102150405")
	testQueue := "test-queue-" + time.Now().Format("20060102150405")

	err = SetupTopology(conn,
		[]ExchangeConfig{
			{
				Name:       testExchange,
				Type:       ExchangeTypeDirect,
				Durable:    false,
				AutoDelete: true,
			},
		},
		[]QueueConfig{
			{
				Name:       testQueue,
				Durable:    false,
				AutoDelete: true,
			},
		},
		[]BindingConfig{
			{
				QueueName:    testQueue,
				ExchangeName: testExchange,
				RoutingKey:   "test",
			},
		},
	)

	if err != nil {
		t.Errorf("Failed to setup topology: %v", err)
	}
}
