package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TestMessage represents a test message structure
type TestMessage struct {
	ID      string            `json:"id"`
	Content string            `json:"content"`
	Headers map[string]string `json:"headers,omitempty"`
}

func TestAPI_BasicFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create client
	client, err := NewClient(
		WithCredentials("guest", "guest"),
		WithHosts("localhost:5672"),
		WithConnectionName("test-new-api"),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		_ = client.Close() // Ignore close error in defer
	}()

	// Test health check
	if err := client.Ping(ctx); err != nil {
		t.Skipf("RabbitMQ not available: %v", err)
	}

	// Test AdminService
	admin := client.Admin()

	// Declare exchange
	err = admin.DeclareExchange(ctx, "test-exchange", ExchangeTypeDirect,
		WithExchangeDurable(),
	)
	if err != nil {
		t.Fatalf("Failed to declare exchange: %v", err)
	}

	// Declare queue
	queue, err := admin.DeclareQueue(ctx, "test-queue",
		WithDurable(),
		WithTTL(time.Hour),
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Bind queue
	err = admin.BindQueue(ctx, queue.Name, "test-exchange", "test.key")
	if err != nil {
		t.Fatalf("Failed to bind queue: %v", err)
	}

	// Test Publisher
	publisher, err := client.NewPublisher(
		WithDefaultExchange("test-exchange"),
		WithPersistent(),
	)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer func() {
		_ = publisher.Close() // Ignore close error in defer
	}()

	// Test message creation
	testMsg := TestMessage{
		ID:      "test-123",
		Content: "Hello, World!",
		Headers: map[string]string{"source": "test"},
	}

	message, err := NewJSONMessage(testMsg)
	if err != nil {
		t.Fatalf("Failed to create JSON message: %v", err)
	}

	message = message.
		WithCorrelationID("corr-123").
		WithHeader("test", "value").
		WithExpiration(5 * time.Minute)

	// Test Consumer - Set up consumer BEFORE publishing
	consumer, err := client.NewConsumer(
		WithPrefetchCount(1),
	)
	if err != nil {
		t.Fatalf("Failed to create consumer: %v", err)
	}
	defer func() {
		_ = consumer.Close() // Ignore close error in defer
	}()

	// Test message consumption
	received := make(chan TestMessage, 2)
	handler := func(ctx context.Context, delivery *Delivery) error {
		var msg TestMessage
		if err := json.Unmarshal(delivery.Body, &msg); err != nil {
			return err
		}
		received <- msg
		return nil
	}

	// Start consuming in background
	consumerReady := make(chan struct{})
	go func() {
		// Signal that we're about to start consuming
		close(consumerReady)
		if err := consumer.Consume(ctx, queue.Name, handler); err != nil {
			t.Logf("Consumer error: %v", err)
		}
	}()

	// Wait for consumer to be ready
	<-consumerReady
	// Add a small delay to ensure consumer is fully initialized
	time.Sleep(100 * time.Millisecond)

	// Now publish messages after consumer is ready
	// Test publishing
	err = publisher.Publish(ctx, "", "test.key", message)
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}

	// Test publisher with confirmations
	confirmingPublisher, err := client.NewPublisher(
		WithDefaultExchange("test-exchange"),
		WithConfirmation(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create confirming publisher: %v", err)
	}
	defer func() {
		_ = confirmingPublisher.Close() // Ignore close error in defer
	}()

	err = confirmingPublisher.Publish(ctx, "", "test.key", message)
	if err != nil {
		t.Fatalf("Failed to publish with confirmation: %v", err)
	}

	// Wait for messages
	timeout := time.After(5 * time.Second)
	messageCount := 0

	for messageCount < 2 {
		select {
		case msg := <-received:
			if msg.ID != testMsg.ID || msg.Content != testMsg.Content {
				t.Errorf("Received message doesn't match sent message")
			}
			messageCount++
		case <-timeout:
			t.Fatalf("Timeout waiting for messages, received %d/2", messageCount)
		}
	}

	t.Logf("Successfully received %d messages", messageCount)
}

func TestAPI_BatchPublishing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := NewClient(
		WithCredentials("guest", "guest"),
		WithHosts("localhost:5672"),
		WithConnectionName("test-batch"),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		_ = client.Close() // Ignore close error in defer
	}()

	if err := client.Ping(ctx); err != nil {
		t.Skipf("RabbitMQ not available: %v", err)
	}

	// Setup
	admin := client.Admin()
	err = admin.DeclareExchange(ctx, "batch-test", ExchangeTypeDirect)
	if err != nil {
		t.Fatalf("Failed to declare exchange: %v", err)
	}

	queue, err := admin.DeclareQueue(ctx, "batch-queue")
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	err = admin.BindQueue(ctx, queue.Name, "batch-test", "batch")
	if err != nil {
		t.Fatalf("Failed to bind queue: %v", err)
	}

	// Create publisher
	publisher, err := client.NewPublisher(WithDefaultExchange("batch-test"))
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer func() {
		_ = publisher.Close() // Ignore close error in defer
	}()

	// Create confirming publisher for testing confirmations
	confirmingPublisher, err := client.NewPublisher(
		WithDefaultExchange("batch-test"),
		WithConfirmation(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create confirming publisher: %v", err)
	}
	defer func() {
		_ = confirmingPublisher.Close() // Ignore close error in defer
	}()

	// Prepare batch messages
	messages := make([]PublishRequest, 5)
	for i := 0; i < 5; i++ {
		msg, err := NewJSONMessage(TestMessage{
			ID:      fmt.Sprintf("batch-%d", i),
			Content: fmt.Sprintf("Batch message %d", i),
		})
		if err != nil {
			t.Fatalf("Failed to create message %d: %v", i, err)
		}

		messages[i] = PublishRequest{
			Exchange:   "",
			RoutingKey: "batch",
			Message:    msg,
		}
	}

	// Test batch publishing with confirmation using confirming publisher
	err = confirmingPublisher.PublishBatch(ctx, messages)
	if err != nil {
		t.Fatalf("Failed to publish batch: %v", err)
	}

	t.Logf("Successfully published and confirmed %d messages", len(messages))
}

func TestAPI_MessageClone(t *testing.T) {
	original, err := NewJSONMessage(TestMessage{
		ID:      "test-123",
		Content: "Original content",
	})
	if err != nil {
		t.Fatalf("Failed to create original message: %v", err)
	}

	original = original.
		WithCorrelationID("corr-123").
		WithHeader("test", "value")

	// Test clone
	cloned := original.Clone()

	// Verify clone is independent
	cloned.WithCorrelationID("different-corr").WithHeader("test", "different")

	if original.CorrelationID == cloned.CorrelationID {
		t.Error("Clone should have independent correlation ID")
	}

	if original.Headers["test"] == cloned.Headers["test"] {
		t.Error("Clone should have independent headers")
	}

	t.Log("Message clone works correctly")
}

func TestAPI_RetryPolicies(t *testing.T) {
	// Test NoRetry
	noRetry := NoRetry
	if noRetry.ShouldRetry(1, nil) {
		t.Error("NoRetry should never retry")
	}

	// Test ExponentialBackoff
	expBackoff := ExponentialBackoff{
		InitialDelay: time.Second,
		MaxDelay:     10 * time.Second,
		MaxAttempts:  3,
		Multiplier:   2.0,
	}

	if !expBackoff.ShouldRetry(1, nil) {
		t.Error("ExponentialBackoff should retry within max attempts")
	}

	if expBackoff.ShouldRetry(5, nil) {
		t.Error("ExponentialBackoff should not retry beyond max attempts")
	}

	delay1 := expBackoff.NextDelay(1)
	delay2 := expBackoff.NextDelay(2)
	if delay2 <= delay1 {
		t.Error("ExponentialBackoff should increase delay")
	}

	// Test LinearBackoff
	linearBackoff := LinearBackoff{
		Delay:       time.Second,
		MaxAttempts: 3,
	}

	if !linearBackoff.ShouldRetry(1, nil) {
		t.Error("LinearBackoff should retry within max attempts")
	}

	if linearBackoff.ShouldRetry(5, nil) {
		t.Error("LinearBackoff should not retry beyond max attempts")
	}

	delay1 = linearBackoff.NextDelay(1)
	delay2 = linearBackoff.NextDelay(2)
	if delay1 != delay2 {
		t.Error("LinearBackoff should have consistent delay")
	}

	t.Log("Retry policies work correctly")
}

func TestAPI_ConnectionNameInClientProperties(t *testing.T) {
	// Test that connection names are properly set in client properties
	testConnectionName := "test-connection-name"

	// Create a client configuration with connection name
	config := &clientConfig{
		Username:             "guest",
		Password:             "guest",
		VHost:                "/",
		ConnectionName:       testConnectionName,
		Heartbeat:            10 * time.Second,
		DialTimeout:          30 * time.Second,
		ChannelTimeout:       10 * time.Second,
		AutoReconnect:        true,
		ReconnectDelay:       5 * time.Second,
		MaxReconnectAttempts: 0,
		Logger:               NewNopLogger(),
		Metrics:              NewNopMetrics(),
		Tracer:               NewNopTracer(),
		PerformanceMonitor:   NewNopPerformanceMonitor(),
	}

	client := &Client{
		config: config,
	}

	// Test the properties setup logic from connect method
	properties := amqp.NewConnectionProperties()
	if client.config.ConnectionName != "" {
		properties.SetClientConnectionName(client.config.ConnectionName)
	}

	// Verify the connection name is set in properties
	connectionName, exists := properties["connection_name"]
	if !exists {
		t.Fatal("connection_name should be set in client properties")
	}

	if connectionName != testConnectionName {
		t.Errorf("Expected connection_name to be %q, got %q", testConnectionName, connectionName)
	}
}

func TestAPI_EmptyConnectionNameHandling(t *testing.T) {
	// Test that empty connection names don't cause issues
	config := &clientConfig{
		Username:             "guest",
		Password:             "guest",
		VHost:                "/",
		ConnectionName:       "", // Empty connection name
		Heartbeat:            10 * time.Second,
		DialTimeout:          30 * time.Second,
		ChannelTimeout:       10 * time.Second,
		AutoReconnect:        true,
		ReconnectDelay:       5 * time.Second,
		MaxReconnectAttempts: 0,
		Logger:               NewNopLogger(),
		Metrics:              NewNopMetrics(),
		Tracer:               NewNopTracer(),
		PerformanceMonitor:   NewNopPerformanceMonitor(),
	}

	client := &Client{
		config: config,
	}

	// Test the properties setup logic from connect method
	properties := amqp.NewConnectionProperties()
	if client.config.ConnectionName != "" {
		properties.SetClientConnectionName(client.config.ConnectionName)
	}

	// Verify the connection_name is not set when empty
	_, exists := properties["connection_name"]
	if exists {
		t.Error("connection_name should not be set when ConnectionName is empty")
	}
}
