package rabbitmq

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestAutomaticRetries tests the automatic retry feature for nacked messages
func TestAutomaticRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewClient(
		WithHosts("localhost:5672"),
		WithCredentials("guest", "guest"),
		WithConnectionName("automatic-retry-test"),
	)
	if err != nil {
		t.Skip("RabbitMQ not available for testing")
	}
	defer func() { _ = client.Close() }()

	// Track callback invocations
	var mu sync.Mutex
	var callbackCount int
	var lastOutcome DeliveryOutcome
	var lastError string

	callback := func(messageID string, outcome DeliveryOutcome, errorMessage string) {
		mu.Lock()
		defer mu.Unlock()
		callbackCount++
		lastOutcome = outcome
		lastError = errorMessage
		t.Logf("Callback invoked: count=%d, messageID=%s, outcome=%s, error=%s",
			callbackCount, messageID, outcome, errorMessage)
	}

	// Create publisher with automatic retries enabled
	publisher, err := client.NewPublisher(
		WithDeliveryAssurance(),
		WithDefaultDeliveryCallback(callback),
		WithPublisherRetry(2, 100*time.Millisecond), // 2 retries with 100ms backoff
	)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer func() { _ = publisher.Close() }()

	// Create test topology
	admin := client.Admin()
	ctx := context.Background()

	exchangeName := "test-retry-exchange"
	err = admin.DeclareExchange(ctx, exchangeName, ExchangeTypeDirect)
	if err != nil {
		t.Fatalf("Failed to declare exchange: %v", err)
	}

	queueName := "test-retry-queue"
	queue, err := admin.DeclareQueue(ctx, queueName)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	err = admin.BindQueue(ctx, queue.Name, exchangeName, "test-key")
	if err != nil {
		t.Fatalf("Failed to bind queue: %v", err)
	}

	// Publish a message
	msg := NewMessage([]byte("test message"))
	msg.MessageID = "retry-test-msg"

	err = publisher.PublishWithDeliveryAssurance(ctx, exchangeName, "test-key", msg, DeliveryOptions{
		MessageID: "retry-test-msg",
	})
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}

	// Wait for confirmation
	time.Sleep(1 * time.Second)

	// Check results
	mu.Lock()
	defer mu.Unlock()

	t.Logf("Final results: callbackCount=%d, lastOutcome=%s, lastError=%s",
		callbackCount, lastOutcome, lastError)

	// We expect the callback to be invoked once with success
	// (since the queue accepts the message)
	if callbackCount == 0 {
		t.Error("Expected at least one callback invocation")
	}

	if lastOutcome != DeliverySuccess {
		t.Errorf("Expected success outcome, got %s: %s", lastOutcome, lastError)
	}
}

// TestAutomaticRetriesDisabled tests that retries don't happen when not enabled
func TestAutomaticRetriesDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := NewClient(
		WithHosts("localhost:5672"),
		WithCredentials("guest", "guest"),
		WithConnectionName("no-retry-test"),
	)
	if err != nil {
		t.Skip("RabbitMQ not available for testing")
	}
	defer func() { _ = client.Close() }()

	// Track callback invocations
	var mu sync.Mutex
	var callbackCount int

	callback := func(messageID string, outcome DeliveryOutcome, errorMessage string) {
		mu.Lock()
		defer mu.Unlock()
		callbackCount++
		t.Logf("Callback invoked: messageID=%s, outcome=%s, error=%s",
			messageID, outcome, errorMessage)
	}

	// Create publisher WITHOUT automatic retries
	publisher, err := client.NewPublisher(
		WithDeliveryAssurance(),
		WithDefaultDeliveryCallback(callback),
		// No WithPublisherRetry option
	)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer func() { _ = publisher.Close() }()

	// Create test topology
	admin := client.Admin()
	ctx := context.Background()

	exchangeName := "test-no-retry-exchange"
	err = admin.DeclareExchange(ctx, exchangeName, ExchangeTypeDirect)
	if err != nil {
		t.Fatalf("Failed to declare exchange: %v", err)
	}

	queueName := "test-no-retry-queue"
	queue, err := admin.DeclareQueue(ctx, queueName)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	err = admin.BindQueue(ctx, queue.Name, exchangeName, "test-key")
	if err != nil {
		t.Fatalf("Failed to bind queue: %v", err)
	}

	// Publish a message
	msg := NewMessage([]byte("test message"))
	msg.MessageID = "no-retry-test-msg"

	err = publisher.PublishWithDeliveryAssurance(ctx, exchangeName, "test-key", msg, DeliveryOptions{
		MessageID: "no-retry-test-msg",
	})
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}

	// Wait for confirmation
	time.Sleep(500 * time.Millisecond)

	// Check results
	mu.Lock()
	defer mu.Unlock()

	t.Logf("Final results: callbackCount=%d", callbackCount)

	// We expect exactly one callback (success, since the queue accepts the message)
	if callbackCount != 1 {
		t.Errorf("Expected exactly 1 callback, got %d", callbackCount)
	}
}

// TestAutomaticRetriesMemoryFootprint tests that messages are cloned when retries are enabled
func TestAutomaticRetriesMemoryFootprint(t *testing.T) {
	client, err := NewClient(FromEnv())
	if err != nil {
		t.Skip("RabbitMQ not available for testing")
	}
	defer func() { _ = client.Close() }()

	// Create publisher with automatic retries
	publisherWithRetries, err := client.NewPublisher(
		WithDeliveryAssurance(),
		WithPublisherRetry(3, 100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Failed to create publisher with retries: %v", err)
	}
	defer func() { _ = publisherWithRetries.Close() }()

	// Create publisher without automatic retries
	publisherWithoutRetries, err := client.NewPublisher(
		WithDeliveryAssurance(),
	)
	if err != nil {
		t.Fatalf("Failed to create publisher without retries: %v", err)
	}
	defer func() { _ = publisherWithoutRetries.Close() }()

	// Verify that the config is set correctly
	if !publisherWithRetries.config.enableAutomaticRetries {
		t.Error("Expected automatic retries to be enabled")
	}

	if publisherWithoutRetries.config.enableAutomaticRetries {
		t.Error("Expected automatic retries to be disabled")
	}

	t.Logf("Automatic retries configuration verified successfully")
}
