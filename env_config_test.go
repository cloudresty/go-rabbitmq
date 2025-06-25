package rabbitmq

import (
	"os"
	"testing"
	"time"
)

// TestConfigurationScenarios tests all three configuration scenarios with go-env v1.0.1
func TestConfigurationScenarios(t *testing.T) {
	// Clean up environment before tests
	cleanupEnv()

	t.Run("Scenario1_OnlyCodeDefaults", testOnlyCodeDefaults)
	t.Run("Scenario2_MixedEnvAndDefaults", testMixedEnvAndDefaults)
	t.Run("Scenario3_AllFromEnvironment", testAllFromEnvironment)
}

// Scenario 1: Only code/default values - no environment variables set
func testOnlyCodeDefaults(t *testing.T) {
	// Ensure no RabbitMQ env vars are set
	cleanupEnv()

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config with only defaults: %v", err)
	}

	// Verify default values
	expectedDefaults := map[string]interface{}{
		"Username":                     "guest",
		"Password":                     "guest",
		"Host":                         "localhost",
		"Port":                         5672,
		"VHost":                        "/",
		"Protocol":                     "amqp",
		"TLSEnabled":                   false,
		"TLSInsecure":                  false,
		"HTTPProtocol":                 "http",
		"HTTPPort":                     15672,
		"ConnectionName":               "go-rabbitmq",
		"Heartbeat":                    10 * time.Second,
		"RetryAttempts":                5,
		"RetryDelay":                   2 * time.Second,
		"DialTimeout":                  30 * time.Second,
		"ChannelTimeout":               10 * time.Second,
		"AutoReconnect":                true,
		"ReconnectDelay":               5 * time.Second,
		"MaxReconnectAttempts":         0,
		"PublisherConfirmationTimeout": 5 * time.Second,
		"PublisherShutdownTimeout":     15 * time.Second,
		"PublisherPersistent":          true,
		"ConsumerPrefetchCount":        1,
		"ConsumerAutoAck":              false,
		"ConsumerMessageTimeout":       5 * time.Minute,
		"ConsumerShutdownTimeout":      30 * time.Second,
	}

	validateConfig(t, config, expectedDefaults, "Scenario 1 (defaults only)")
}

// Scenario 2: Mix of environment variables and code defaults
func testMixedEnvAndDefaults(t *testing.T) {
	// Clean up first
	cleanupEnv()

	// Set only some environment variables
	envVars := map[string]string{
		"RABBITMQ_HOST":              "rabbitmq.example.com",
		"RABBITMQ_USERNAME":          "testuser",
		"RABBITMQ_PASSWORD":          "testpass",
		"RABBITMQ_PORT":              "5673",
		"RABBITMQ_CONNECTION_NAME":   "test-connection",
		"RABBITMQ_AUTO_RECONNECT":    "false",
		"RABBITMQ_RETRY_ATTEMPTS":    "10",
		"RABBITMQ_CONSUMER_AUTO_ACK": "true",
		"RABBITMQ_TLS_ENABLED":       "true",
		"RABBITMQ_HTTP_PORT":         "15673",
	}

	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer cleanupEnv()

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config with mixed env/defaults: %v", err)
	}

	// Expected values: env vars override defaults, others remain default
	expectedValues := map[string]interface{}{
		"Host":            "rabbitmq.example.com", // from env
		"Username":        "testuser",             // from env
		"Password":        "testpass",             // from env
		"Port":            5673,                   // from env
		"ConnectionName":  "test-connection",      // from env
		"AutoReconnect":   false,                  // from env
		"RetryAttempts":   10,                     // from env
		"ConsumerAutoAck": true,                   // from env
		"TLSEnabled":      true,                   // from env
		"HTTPPort":        15673,                  // from env
		// These should remain as defaults since not set in env
		"VHost":                        "/",
		"Protocol":                     "amqp",
		"TLSInsecure":                  false,
		"HTTPProtocol":                 "http",
		"Heartbeat":                    10 * time.Second,
		"RetryDelay":                   2 * time.Second,
		"DialTimeout":                  30 * time.Second,
		"ChannelTimeout":               10 * time.Second,
		"ReconnectDelay":               5 * time.Second,
		"MaxReconnectAttempts":         0,
		"PublisherConfirmationTimeout": 5 * time.Second,
		"PublisherShutdownTimeout":     15 * time.Second,
		"PublisherPersistent":          true,
		"ConsumerPrefetchCount":        1,
		"ConsumerMessageTimeout":       5 * time.Minute,
		"ConsumerShutdownTimeout":      30 * time.Second,
	}

	validateConfig(t, config, expectedValues, "Scenario 2 (mixed env/defaults)")
}

// Scenario 3: All values from environment variables
func testAllFromEnvironment(t *testing.T) {
	// Clean up first
	cleanupEnv()

	// Set ALL environment variables
	envVars := map[string]string{
		"RABBITMQ_USERNAME":                       "produser",
		"RABBITMQ_PASSWORD":                       "prodpass",
		"RABBITMQ_HOST":                           "prod.rabbitmq.com",
		"RABBITMQ_PORT":                           "5671",
		"RABBITMQ_VHOST":                          "/prod",
		"RABBITMQ_PROTOCOL":                       "amqps",
		"RABBITMQ_TLS_ENABLED":                    "true",
		"RABBITMQ_TLS_INSECURE":                   "false",
		"RABBITMQ_HTTP_PROTOCOL":                  "https",
		"RABBITMQ_HTTP_PORT":                      "15671",
		"RABBITMQ_CONNECTION_NAME":                "prod-service",
		"RABBITMQ_HEARTBEAT":                      "30s",
		"RABBITMQ_RETRY_ATTEMPTS":                 "3",
		"RABBITMQ_RETRY_DELAY":                    "5s",
		"RABBITMQ_DIAL_TIMEOUT":                   "60s",
		"RABBITMQ_CHANNEL_TIMEOUT":                "15s",
		"RABBITMQ_AUTO_RECONNECT":                 "true",
		"RABBITMQ_RECONNECT_DELAY":                "10s",
		"RABBITMQ_MAX_RECONNECT_ATTEMPTS":         "20",
		"RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT": "10s",
		"RABBITMQ_PUBLISHER_SHUTDOWN_TIMEOUT":     "30s",
		"RABBITMQ_PUBLISHER_PERSISTENT":           "false",
		"RABBITMQ_CONSUMER_PREFETCH_COUNT":        "5",
		"RABBITMQ_CONSUMER_AUTO_ACK":              "false",
		"RABBITMQ_CONSUMER_MESSAGE_TIMEOUT":       "10m",
		"RABBITMQ_CONSUMER_SHUTDOWN_TIMEOUT":      "60s",
	}

	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer cleanupEnv()

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config with all env vars: %v", err)
	}

	// All values should come from environment
	expectedValues := map[string]interface{}{
		"Username":                     "produser",
		"Password":                     "prodpass",
		"Host":                         "prod.rabbitmq.com",
		"Port":                         5671,
		"VHost":                        "/prod",
		"Protocol":                     "amqps",
		"TLSEnabled":                   true,
		"TLSInsecure":                  false,
		"HTTPProtocol":                 "https",
		"HTTPPort":                     15671,
		"ConnectionName":               "prod-service",
		"Heartbeat":                    30 * time.Second,
		"RetryAttempts":                3,
		"RetryDelay":                   5 * time.Second,
		"DialTimeout":                  60 * time.Second,
		"ChannelTimeout":               15 * time.Second,
		"AutoReconnect":                true,
		"ReconnectDelay":               10 * time.Second,
		"MaxReconnectAttempts":         20,
		"PublisherConfirmationTimeout": 10 * time.Second,
		"PublisherShutdownTimeout":     30 * time.Second,
		"PublisherPersistent":          false,
		"ConsumerPrefetchCount":        5,
		"ConsumerAutoAck":              false,
		"ConsumerMessageTimeout":       10 * time.Minute,
		"ConsumerShutdownTimeout":      60 * time.Second,
	}

	validateConfig(t, config, expectedValues, "Scenario 3 (all from env)")
}

// Test custom prefix functionality
func TestCustomPrefix(t *testing.T) {
	cleanupEnv()
	cleanupCustomPrefixEnv("CUSTOM_")

	// Set environment variables with custom prefix
	envVars := map[string]string{
		"CUSTOM_RABBITMQ_USERNAME":        "custom_user",
		"CUSTOM_RABBITMQ_PASSWORD":        "custom_pass",
		"CUSTOM_RABBITMQ_HOST":            "custom.host.com",
		"CUSTOM_RABBITMQ_PORT":            "5674",
		"CUSTOM_RABBITMQ_CONNECTION_NAME": "custom-connection",
	}

	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer cleanupCustomPrefixEnv("CUSTOM_")

	config, err := LoadFromEnvWithPrefix("CUSTOM_")
	if err != nil {
		t.Fatalf("Failed to load config with custom prefix: %v", err)
	}

	// Verify custom prefix values were loaded
	expectedValues := map[string]interface{}{
		"Username":       "custom_user",
		"Password":       "custom_pass",
		"Host":           "custom.host.com",
		"Port":           5674,
		"ConnectionName": "custom-connection",
		// Others should be defaults
		"VHost":         "/",
		"Protocol":      "amqp",
		"TLSEnabled":    false,
		"HTTPPort":      15672,
		"AutoReconnect": true,
	}

	validateConfig(t, config, expectedValues, "Custom prefix test")
}

// Test URL building functionality
func TestURLBuilding(t *testing.T) {
	tests := []struct {
		name        string
		config      EnvConfig
		expectedURL string
	}{
		{
			name: "default_config",
			config: EnvConfig{
				Protocol: "amqp",
				Username: "guest",
				Password: "guest",
				Host:     "localhost",
				Port:     5672,
				VHost:    "/",
			},
			expectedURL: "amqp://guest:guest@localhost:5672",
		},
		{
			name: "custom_vhost",
			config: EnvConfig{
				Protocol: "amqps",
				Username: "user",
				Password: "pass",
				Host:     "example.com",
				Port:     5671,
				VHost:    "/production",
			},
			expectedURL: "amqps://user:pass@example.com:5671/production",
		},
		{
			name: "root_vhost_explicit",
			config: EnvConfig{
				Protocol: "amqp",
				Username: "test",
				Password: "test123",
				Host:     "test.local",
				Port:     5672,
				VHost:    "/",
			},
			expectedURL: "amqp://test:test123@test.local:5672",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualURL := tt.config.BuildAMQPURL()
			if actualURL != tt.expectedURL {
				t.Errorf("BuildAMQPURL() = %v, want %v", actualURL, tt.expectedURL)
			}
		})
	}
}

// Test conversion to other config types
func TestConfigConversions(t *testing.T) {
	cleanupEnv()

	// Set some environment variables
	os.Setenv("RABBITMQ_HOST", "test.rabbitmq.com")
	os.Setenv("RABBITMQ_USERNAME", "testuser")
	os.Setenv("RABBITMQ_PUBLISHER_PERSISTENT", "false")
	os.Setenv("RABBITMQ_CONSUMER_AUTO_ACK", "true")
	defer cleanupEnv()

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test ConnectionConfig conversion
	connConfig := config.ToConnectionConfig()
	if connConfig.URL != "amqp://testuser:guest@test.rabbitmq.com:5672" {
		t.Errorf("ConnectionConfig URL = %v, want %v", connConfig.URL, "amqp://testuser:guest@test.rabbitmq.com:5672")
	}
	if connConfig.AutoReconnect != true {
		t.Errorf("ConnectionConfig AutoReconnect = %v, want %v", connConfig.AutoReconnect, true)
	}

	// Test PublisherConfig conversion
	pubConfig := config.ToPublisherConfig()
	if pubConfig.Persistent != false {
		t.Errorf("PublisherConfig Persistent = %v, want %v", pubConfig.Persistent, false)
	}

	// Test ConsumerConfig conversion
	consConfig := config.ToConsumerConfig()
	if consConfig.AutoAck != true {
		t.Errorf("ConsumerConfig AutoAck = %v, want %v", consConfig.AutoAck, true)
	}
}

// Helper functions

func cleanupEnv() {
	envVars := []string{
		"RABBITMQ_USERNAME", "RABBITMQ_PASSWORD", "RABBITMQ_HOST", "RABBITMQ_PORT", "RABBITMQ_VHOST",
		"RABBITMQ_PROTOCOL", "RABBITMQ_TLS_ENABLED", "RABBITMQ_TLS_INSECURE",
		"RABBITMQ_HTTP_PROTOCOL", "RABBITMQ_HTTP_PORT", "RABBITMQ_CONNECTION_NAME",
		"RABBITMQ_HEARTBEAT", "RABBITMQ_RETRY_ATTEMPTS", "RABBITMQ_RETRY_DELAY",
		"RABBITMQ_DIAL_TIMEOUT", "RABBITMQ_CHANNEL_TIMEOUT", "RABBITMQ_AUTO_RECONNECT",
		"RABBITMQ_RECONNECT_DELAY", "RABBITMQ_MAX_RECONNECT_ATTEMPTS",
		"RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT", "RABBITMQ_PUBLISHER_SHUTDOWN_TIMEOUT",
		"RABBITMQ_PUBLISHER_PERSISTENT", "RABBITMQ_CONSUMER_PREFETCH_COUNT",
		"RABBITMQ_CONSUMER_AUTO_ACK", "RABBITMQ_CONSUMER_MESSAGE_TIMEOUT",
		"RABBITMQ_CONSUMER_SHUTDOWN_TIMEOUT",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}

func cleanupCustomPrefixEnv(prefix string) {
	envVars := []string{
		"RABBITMQ_USERNAME", "RABBITMQ_PASSWORD", "RABBITMQ_HOST", "RABBITMQ_PORT", "RABBITMQ_VHOST", "RABBITMQ_PROTOCOL",
		"RABBITMQ_TLS_ENABLED", "RABBITMQ_TLS_INSECURE", "RABBITMQ_HTTP_PROTOCOL", "RABBITMQ_HTTP_PORT",
		"RABBITMQ_CONNECTION_NAME", "RABBITMQ_HEARTBEAT", "RABBITMQ_RETRY_ATTEMPTS", "RABBITMQ_RETRY_DELAY",
		"RABBITMQ_DIAL_TIMEOUT", "RABBITMQ_CHANNEL_TIMEOUT", "RABBITMQ_AUTO_RECONNECT", "RABBITMQ_RECONNECT_DELAY",
		"RABBITMQ_MAX_RECONNECT_ATTEMPTS", "RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT",
		"RABBITMQ_PUBLISHER_SHUTDOWN_TIMEOUT", "RABBITMQ_PUBLISHER_PERSISTENT",
		"RABBITMQ_CONSUMER_PREFETCH_COUNT", "RABBITMQ_CONSUMER_AUTO_ACK", "RABBITMQ_CONSUMER_MESSAGE_TIMEOUT",
		"RABBITMQ_CONSUMER_SHUTDOWN_TIMEOUT",
	}

	for _, envVar := range envVars {
		os.Unsetenv(prefix + envVar)
	}
}

func validateConfig(t *testing.T, config *EnvConfig, expected map[string]interface{}, scenario string) {
	for field, expectedValue := range expected {
		var actualValue interface{}

		switch field {
		case "Username":
			actualValue = config.Username
		case "Password":
			actualValue = config.Password
		case "Host":
			actualValue = config.Host
		case "Port":
			actualValue = config.Port
		case "VHost":
			actualValue = config.VHost
		case "Protocol":
			actualValue = config.Protocol
		case "TLSEnabled":
			actualValue = config.TLSEnabled
		case "TLSInsecure":
			actualValue = config.TLSInsecure
		case "HTTPProtocol":
			actualValue = config.HTTPProtocol
		case "HTTPPort":
			actualValue = config.HTTPPort
		case "ConnectionName":
			actualValue = config.ConnectionName
		case "Heartbeat":
			actualValue = config.Heartbeat
		case "RetryAttempts":
			actualValue = config.RetryAttempts
		case "RetryDelay":
			actualValue = config.RetryDelay
		case "DialTimeout":
			actualValue = config.DialTimeout
		case "ChannelTimeout":
			actualValue = config.ChannelTimeout
		case "AutoReconnect":
			actualValue = config.AutoReconnect
		case "ReconnectDelay":
			actualValue = config.ReconnectDelay
		case "MaxReconnectAttempts":
			actualValue = config.MaxReconnectAttempts
		case "PublisherConfirmationTimeout":
			actualValue = config.PublisherConfirmationTimeout
		case "PublisherShutdownTimeout":
			actualValue = config.PublisherShutdownTimeout
		case "PublisherPersistent":
			actualValue = config.PublisherPersistent
		case "ConsumerPrefetchCount":
			actualValue = config.ConsumerPrefetchCount
		case "ConsumerAutoAck":
			actualValue = config.ConsumerAutoAck
		case "ConsumerMessageTimeout":
			actualValue = config.ConsumerMessageTimeout
		case "ConsumerShutdownTimeout":
			actualValue = config.ConsumerShutdownTimeout
		default:
			t.Errorf("%s: Unknown field %s", scenario, field)
			continue
		}

		if actualValue != expectedValue {
			t.Errorf("%s: %s = %v, want %v", scenario, field, actualValue, expectedValue)
		}
	}
}
