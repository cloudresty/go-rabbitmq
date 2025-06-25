package rabbitmq

import (
	"bytes"
	"context"
	"os"
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
	defer func() {
		_ = conn.Close() // Ignore error during cleanup
	}()

	if !conn.IsConnected() {
		t.Error("Expected connection to be active")
	}
}

func TestPublisherCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := PublisherConfig{
		ConnectionConfig: DefaultConnectionConfig(testURL),
	}
	publisher, err := NewPublisherWithConfig(config)
	if err != nil {
		t.Skipf("Could not create publisher: %v", err)
	}
	defer func() {
		_ = publisher.Close() // Ignore error during cleanup
	}()

	if !publisher.IsConnected() {
		t.Error("Expected publisher to be connected")
	}
}

func TestConsumerCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := ConsumerConfig{
		ConnectionConfig: DefaultConnectionConfig(testURL),
	}
	consumer, err := NewConsumerWithConfig(config)
	if err != nil {
		t.Skipf("Could not create consumer: %v", err)
	}
	defer func() {
		_ = consumer.Close() // Ignore error during cleanup
	}()

	if !consumer.IsConnected() {
		t.Error("Expected consumer to be connected")
	}
}

func TestPublishAndConsume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create publisher
	publisherConfig := PublisherConfig{
		ConnectionConfig: DefaultConnectionConfig(testURL),
	}
	publisher, err := NewPublisherWithConfig(publisherConfig)
	if err != nil {
		t.Skipf("Could not create publisher: %v", err)
	}
	defer func() {
		// Add a small delay to let any in-flight logging complete
		// This helps avoid race conditions in the emit library's timestamp code
		time.Sleep(10 * time.Millisecond)
		_ = publisher.Close() // Ignore error during cleanup
	}()

	// Create consumer
	consumerConfig := ConsumerConfig{
		ConnectionConfig: DefaultConnectionConfig(testURL),
	}
	consumer, err := NewConsumerWithConfig(consumerConfig)
	if err != nil {
		t.Skipf("Could not create consumer: %v", err)
	}
	defer func() {
		// Add a small delay to let any in-flight logging complete
		// This helps avoid race conditions in the emit library's timestamp code
		time.Sleep(10 * time.Millisecond)
		_ = consumer.Close() // Ignore error during cleanup
	}()

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
	defer func() {
		_ = conn.Close() // Ignore error during cleanup
	}()

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

// TestEnvironmentBinding verifies that go-env v1.0.1 correctly handles
// three scenarios for environment variable binding:
// 1. All config values from code defaults (no env vars set)
// 2. Mixed environment and code defaults (some env vars set)
// 3. All values from environment variables (all env vars set)
func TestEnvironmentBinding(t *testing.T) {
	// Store original environment to restore later
	originalEnv := make(map[string]string)
	envVars := []string{
		"RABBITMQ_USERNAME", "RABBITMQ_PASSWORD", "RABBITMQ_HOST", "RABBITMQ_PORT",
		"RABBITMQ_VHOST", "RABBITMQ_PROTOCOL", "RABBITMQ_TLS_ENABLED", "RABBITMQ_CONNECTION_NAME",
		"RABBITMQ_HEARTBEAT", "RABBITMQ_RETRY_ATTEMPTS", "RABBITMQ_RETRY_DELAY",
		"RABBITMQ_DIAL_TIMEOUT", "RABBITMQ_AUTO_RECONNECT", "RABBITMQ_RECONNECT_DELAY",
	}

	for _, envVar := range envVars {
		if val, exists := os.LookupEnv(envVar); exists {
			originalEnv[envVar] = val
		}
	}

	// Clean up function to restore environment
	cleanup := func() {
		// Clear all test env vars
		for _, envVar := range envVars {
			os.Unsetenv(envVar)
		}
		// Restore original values
		for envVar, val := range originalEnv {
			os.Setenv(envVar, val)
		}
	}
	defer cleanup()

	t.Run("AllDefaults", func(t *testing.T) {
		// Clear all environment variables
		for _, envVar := range envVars {
			os.Unsetenv(envVar)
		}

		// Load config - should use all defaults from struct tags
		config, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify defaults are applied correctly
		if config.Username != "guest" {
			t.Errorf("Expected username 'guest', got '%s'", config.Username)
		}
		if config.Password != "guest" {
			t.Errorf("Expected password 'guest', got '%s'", config.Password)
		}
		if config.Host != "localhost" {
			t.Errorf("Expected host 'localhost', got '%s'", config.Host)
		}
		if config.Port != 5672 {
			t.Errorf("Expected port 5672, got %d", config.Port)
		}
		if config.VHost != "/" {
			t.Errorf("Expected vhost '/', got '%s'", config.VHost)
		}
		if config.Protocol != "amqp" {
			t.Errorf("Expected protocol 'amqp', got '%s'", config.Protocol)
		}
		if config.TLSEnabled != false {
			t.Errorf("Expected TLSEnabled false, got %t", config.TLSEnabled)
		}
		if config.ConnectionName != "go-rabbitmq" {
			t.Errorf("Expected connection name 'go-rabbitmq', got '%s'", config.ConnectionName)
		}
		if config.Heartbeat != 10*time.Second {
			t.Errorf("Expected heartbeat 10s, got %v", config.Heartbeat)
		}
		if config.RetryAttempts != 5 {
			t.Errorf("Expected retry attempts 5, got %d", config.RetryAttempts)
		}
		if config.RetryDelay != 2*time.Second {
			t.Errorf("Expected retry delay 2s, got %v", config.RetryDelay)
		}
		if config.DialTimeout != 30*time.Second {
			t.Errorf("Expected dial timeout 30s, got %v", config.DialTimeout)
		}
		if config.AutoReconnect != true {
			t.Errorf("Expected auto reconnect true, got %t", config.AutoReconnect)
		}
		if config.ReconnectDelay != 5*time.Second {
			t.Errorf("Expected reconnect delay 5s, got %v", config.ReconnectDelay)
		}
	})

	t.Run("MixedEnvAndDefaults", func(t *testing.T) {
		// Clear all environment variables first
		for _, envVar := range envVars {
			os.Unsetenv(envVar)
		}

		// Set only some environment variables
		os.Setenv("RABBITMQ_USERNAME", "testuser")
		os.Setenv("RABBITMQ_HOST", "test.rabbitmq.com")
		os.Setenv("RABBITMQ_PORT", "5673")
		os.Setenv("RABBITMQ_RETRY_ATTEMPTS", "10")
		os.Setenv("RABBITMQ_TLS_ENABLED", "true")

		// Load config - should use mix of env vars and defaults
		config, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify environment values are used
		if config.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", config.Username)
		}
		if config.Host != "test.rabbitmq.com" {
			t.Errorf("Expected host 'test.rabbitmq.com', got '%s'", config.Host)
		}
		if config.Port != 5673 {
			t.Errorf("Expected port 5673, got %d", config.Port)
		}
		if config.RetryAttempts != 10 {
			t.Errorf("Expected retry attempts 10, got %d", config.RetryAttempts)
		}
		if config.TLSEnabled != true {
			t.Errorf("Expected TLSEnabled true, got %t", config.TLSEnabled)
		}

		// Verify defaults are still used for unset values
		if config.Password != "guest" {
			t.Errorf("Expected password default 'guest', got '%s'", config.Password)
		}
		if config.VHost != "/" {
			t.Errorf("Expected vhost default '/', got '%s'", config.VHost)
		}
		if config.Protocol != "amqp" {
			t.Errorf("Expected protocol default 'amqp', got '%s'", config.Protocol)
		}
		if config.ConnectionName != "go-rabbitmq" {
			t.Errorf("Expected connection name default 'go-rabbitmq', got '%s'", config.ConnectionName)
		}
		if config.Heartbeat != 10*time.Second {
			t.Errorf("Expected heartbeat default 10s, got %v", config.Heartbeat)
		}
	})

	t.Run("AllEnvironmentVariables", func(t *testing.T) {
		// Clear all environment variables first
		for _, envVar := range envVars {
			os.Unsetenv(envVar)
		}

		// Set all environment variables
		os.Setenv("RABBITMQ_USERNAME", "envuser")
		os.Setenv("RABBITMQ_PASSWORD", "envpass")
		os.Setenv("RABBITMQ_HOST", "env.rabbitmq.com")
		os.Setenv("RABBITMQ_PORT", "5674")
		os.Setenv("RABBITMQ_VHOST", "/envvhost")
		os.Setenv("RABBITMQ_PROTOCOL", "amqps")
		os.Setenv("RABBITMQ_TLS_ENABLED", "true")
		os.Setenv("RABBITMQ_CONNECTION_NAME", "env-connection")
		os.Setenv("RABBITMQ_HEARTBEAT", "15s")
		os.Setenv("RABBITMQ_RETRY_ATTEMPTS", "3")
		os.Setenv("RABBITMQ_RETRY_DELAY", "1s")
		os.Setenv("RABBITMQ_DIAL_TIMEOUT", "20s")
		os.Setenv("RABBITMQ_AUTO_RECONNECT", "false")
		os.Setenv("RABBITMQ_RECONNECT_DELAY", "10s")

		// Load config - should use all environment values
		config, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify all environment values are used correctly
		if config.Username != "envuser" {
			t.Errorf("Expected username 'envuser', got '%s'", config.Username)
		}
		if config.Password != "envpass" {
			t.Errorf("Expected password 'envpass', got '%s'", config.Password)
		}
		if config.Host != "env.rabbitmq.com" {
			t.Errorf("Expected host 'env.rabbitmq.com', got '%s'", config.Host)
		}
		if config.Port != 5674 {
			t.Errorf("Expected port 5674, got %d", config.Port)
		}
		if config.VHost != "/envvhost" {
			t.Errorf("Expected vhost '/envvhost', got '%s'", config.VHost)
		}
		if config.Protocol != "amqps" {
			t.Errorf("Expected protocol 'amqps', got '%s'", config.Protocol)
		}
		if config.TLSEnabled != true {
			t.Errorf("Expected TLSEnabled true, got %t", config.TLSEnabled)
		}
		if config.ConnectionName != "env-connection" {
			t.Errorf("Expected connection name 'env-connection', got '%s'", config.ConnectionName)
		}
		if config.Heartbeat != 15*time.Second {
			t.Errorf("Expected heartbeat 15s, got %v", config.Heartbeat)
		}
		if config.RetryAttempts != 3 {
			t.Errorf("Expected retry attempts 3, got %d", config.RetryAttempts)
		}
		if config.RetryDelay != 1*time.Second {
			t.Errorf("Expected retry delay 1s, got %v", config.RetryDelay)
		}
		if config.DialTimeout != 20*time.Second {
			t.Errorf("Expected dial timeout 20s, got %v", config.DialTimeout)
		}
		if config.AutoReconnect != false {
			t.Errorf("Expected auto reconnect false, got %t", config.AutoReconnect)
		}
		if config.ReconnectDelay != 10*time.Second {
			t.Errorf("Expected reconnect delay 10s, got %v", config.ReconnectDelay)
		}
	})
}

// TestEnvironmentBindingWithConnections verifies that the environment binding
// works correctly when creating actual connections with the loaded config
func TestEnvironmentBindingWithConnections(t *testing.T) {
	// This test requires RabbitMQ to be available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Store original environment
	originalUsername, hasUsername := os.LookupEnv("RABBITMQ_USERNAME")
	originalHost, hasHost := os.LookupEnv("RABBITMQ_HOST")
	originalPort, hasPort := os.LookupEnv("RABBITMQ_PORT")

	cleanup := func() {
		os.Unsetenv("RABBITMQ_USERNAME")
		os.Unsetenv("RABBITMQ_HOST")
		os.Unsetenv("RABBITMQ_PORT")
		if hasUsername {
			os.Setenv("RABBITMQ_USERNAME", originalUsername)
		}
		if hasHost {
			os.Setenv("RABBITMQ_HOST", originalHost)
		}
		if hasPort {
			os.Setenv("RABBITMQ_PORT", originalPort)
		}
	}
	defer cleanup()

	t.Run("ConnectionWithDefaults", func(t *testing.T) {
		// Clear environment variables to test defaults
		os.Unsetenv("RABBITMQ_USERNAME")
		os.Unsetenv("RABBITMQ_HOST")
		os.Unsetenv("RABBITMQ_PORT")

		// Load config using defaults
		envConfig, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("Failed to load env config: %v", err)
		}

		// Create connection config from environment config
		connConfig := envConfig.ToConnectionConfig()

		// Verify the URL construction uses defaults
		expectedURL := "amqp://guest:guest@localhost:5672"
		if connConfig.URL != expectedURL {
			t.Errorf("Expected URL '%s', got '%s'", expectedURL, connConfig.URL)
		}

		// Try to create a connection (may fail if RabbitMQ not available, but that's ok)
		conn, err := NewConnection(connConfig)
		if err != nil {
			t.Skipf("Could not connect to RabbitMQ with defaults: %v", err)
		}
		defer func() {
			_ = conn.Close()
		}()

		if !conn.IsConnected() {
			t.Error("Expected connection to be active with default config")
		}
	})

	t.Run("ConnectionWithEnvironment", func(t *testing.T) {
		// Set environment variables
		os.Setenv("RABBITMQ_USERNAME", "guest")
		os.Setenv("RABBITMQ_HOST", "localhost")
		os.Setenv("RABBITMQ_PORT", "5672")

		// Load config using environment
		envConfig, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("Failed to load env config: %v", err)
		}

		// Verify environment values were loaded
		if envConfig.Username != "guest" {
			t.Errorf("Expected username 'guest', got '%s'", envConfig.Username)
		}
		if envConfig.Host != "localhost" {
			t.Errorf("Expected host 'localhost', got '%s'", envConfig.Host)
		}
		if envConfig.Port != 5672 {
			t.Errorf("Expected port 5672, got %d", envConfig.Port)
		}

		// Create connection config from environment config
		connConfig := envConfig.ToConnectionConfig()

		// Verify the URL construction uses environment values
		expectedURL := "amqp://guest:guest@localhost:5672"
		if connConfig.URL != expectedURL {
			t.Errorf("Expected URL '%s', got '%s'", expectedURL, connConfig.URL)
		}

		// Try to create a connection (may fail if RabbitMQ not available, but that's ok)
		conn, err := NewConnection(connConfig)
		if err != nil {
			t.Skipf("Could not connect to RabbitMQ with environment config: %v", err)
		}
		defer func() {
			_ = conn.Close()
		}()

		if !conn.IsConnected() {
			t.Error("Expected connection to be active with environment config")
		}
	})
}

// TestMixedPrefixAndDefaults verifies that when using a custom prefix,
// only the prefixed environment variables are used, and missing ones fall back to defaults
func TestMixedPrefixAndDefaults(t *testing.T) {
	// Clean up any existing environment variables
	envVars := []string{
		"MIXED_RABBITMQ_USERNAME", "MIXED_RABBITMQ_PASSWORD", "MIXED_RABBITMQ_HOST", "MIXED_RABBITMQ_PORT",
	}
	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
	defer func() {
		for _, envVar := range envVars {
			os.Unsetenv(envVar)
		}
	}()

	// Set only the username with the custom prefix
	os.Setenv("MIXED_RABBITMQ_USERNAME", "custom_user")
	// Don't set MIXED_RABBITMQ_PASSWORD, MIXED_RABBITMQ_HOST, MIXED_RABBITMQ_PORT
	// These should fall back to defaults

	// Load config with the custom prefix
	config, err := LoadFromEnvWithPrefix("MIXED_")
	if err != nil {
		t.Fatalf("Failed to load config with mixed prefix: %v", err)
	}

	// Verify that the prefixed env var was used
	if config.Username != "custom_user" {
		t.Errorf("Expected username 'custom_user', got '%s'", config.Username)
	}

	// Verify that missing prefixed env vars fell back to defaults
	if config.Password != "guest" {
		t.Errorf("Expected password default 'guest', got '%s'", config.Password)
	}
	if config.Host != "localhost" {
		t.Errorf("Expected host default 'localhost', got '%s'", config.Host)
	}
	if config.Port != 5672 {
		t.Errorf("Expected port default 5672, got %d", config.Port)
	}
	if config.VHost != "/" {
		t.Errorf("Expected vhost default '/', got '%s'", config.VHost)
	}
	if config.Protocol != "amqp" {
		t.Errorf("Expected protocol default 'amqp', got '%s'", config.Protocol)
	}
}
