package rabbitmq

import (
	"os"
	"testing"
	"time"
)

func TestLoadFromEnv(t *testing.T) {
	// Set test environment variables
	envVars := map[string]string{
		"RABBITMQ_USERNAME":        "testuser",
		"RABBITMQ_PASSWORD":        "testpass",
		"RABBITMQ_HOST":            "testhost",
		"RABBITMQ_PORT":            "5673",
		"RABBITMQ_VHOST":           "/test",
		"RABBITMQ_CONNECTION_NAME": "test-connection",
		"RABBITMQ_HEARTBEAT":       "20s",
		"RABBITMQ_RETRY_ATTEMPTS":  "3",
	}

	// Set environment variables
	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer func() {
		// Clean up environment variables
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// Load configuration from environment
	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config from environment: %v", err)
	}

	// Verify loaded values
	if config.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", config.Username)
	}
	if config.Password != "testpass" {
		t.Errorf("Expected password 'testpass', got '%s'", config.Password)
	}
	if config.Host != "testhost" {
		t.Errorf("Expected host 'testhost', got '%s'", config.Host)
	}
	if config.Port != 5673 {
		t.Errorf("Expected port 5673, got %d", config.Port)
	}
	if config.VHost != "/test" {
		t.Errorf("Expected vhost '/test', got '%s'", config.VHost)
	}
	if config.ConnectionName != "test-connection" {
		t.Errorf("Expected connection name 'test-connection', got '%s'", config.ConnectionName)
	}
	if config.Heartbeat != 20*time.Second {
		t.Errorf("Expected heartbeat 20s, got %v", config.Heartbeat)
	}
	if config.RetryAttempts != 3 {
		t.Errorf("Expected retry attempts 3, got %d", config.RetryAttempts)
	}
}

func TestLoadFromEnvWithPrefix(t *testing.T) {
	// Set test environment variables with custom prefix
	envVars := map[string]string{
		"MYAPP_RABBITMQ_USERNAME": "prefixuser",
		"MYAPP_RABBITMQ_HOST":     "prefixhost",
		"MYAPP_RABBITMQ_PORT":     "5674",
	}

	// Set environment variables
	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer func() {
		// Clean up environment variables
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// Load configuration from environment with prefix
	config, err := LoadFromEnvWithPrefix("MYAPP_")
	if err != nil {
		t.Fatalf("Failed to load config from environment with prefix: %v", err)
	}

	// Verify loaded values
	if config.Username != "prefixuser" {
		t.Errorf("Expected username 'prefixuser', got '%s'", config.Username)
	}
	if config.Host != "prefixhost" {
		t.Errorf("Expected host 'prefixhost', got '%s'", config.Host)
	}
	if config.Port != 5674 {
		t.Errorf("Expected port 5674, got %d", config.Port)
	}

	// Verify defaults are still applied for unset variables
	if config.Password != "guest" {
		t.Errorf("Expected default password 'guest', got '%s'", config.Password)
	}
}

func TestBuildAMQPURL(t *testing.T) {
	tests := []struct {
		name     string
		config   EnvConfig
		expected string
	}{
		{
			name: "default configuration",
			config: EnvConfig{
				Protocol: "amqp",
				Username: "guest",
				Password: "guest",
				Host:     "localhost",
				Port:     5672,
				VHost:    "/",
			},
			expected: "amqp://guest:guest@localhost:5672",
		},
		{
			name: "custom vhost",
			config: EnvConfig{
				Protocol: "amqp",
				Username: "user",
				Password: "pass",
				Host:     "example.com",
				Port:     5672,
				VHost:    "/myvhost",
			},
			expected: "amqp://user:pass@example.com:5672/myvhost",
		},
		{
			name: "vhost without leading slash",
			config: EnvConfig{
				Protocol: "amqp",
				Username: "user",
				Password: "pass",
				Host:     "example.com",
				Port:     5672,
				VHost:    "myvhost",
			},
			expected: "amqp://user:pass@example.com:5672/myvhost",
		},
		{
			name: "TLS configuration",
			config: EnvConfig{
				Protocol: "amqps",
				Username: "secure",
				Password: "password",
				Host:     "secure.example.com",
				Port:     5671,
				VHost:    "/",
			},
			expected: "amqps://secure:password@secure.example.com:5671",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.BuildAMQPURL()
			if result != tt.expected {
				t.Errorf("Expected URL '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestBuildHTTPURL(t *testing.T) {
	config := EnvConfig{
		HTTPProtocol: "http",
		Host:         "localhost",
		HTTPPort:     15672,
	}

	expected := "http://localhost:15672"
	result := config.BuildHTTPURL()

	if result != expected {
		t.Errorf("Expected HTTP URL '%s', got '%s'", expected, result)
	}
}

func TestToConnectionConfig(t *testing.T) {
	envConfig := EnvConfig{
		Username:             "testuser",
		Password:             "testpass",
		Host:                 "testhost",
		Port:                 5672,
		VHost:                "/test",
		Protocol:             "amqp",
		ConnectionName:       "test-conn",
		Heartbeat:            15 * time.Second,
		RetryAttempts:        3,
		RetryDelay:           5 * time.Second,
		AutoReconnect:        true,
		ReconnectDelay:       10 * time.Second,
		MaxReconnectAttempts: 5,
		DialTimeout:          30 * time.Second,
		ChannelTimeout:       10 * time.Second,
	}

	connConfig := envConfig.ToConnectionConfig()

	if connConfig.URL != "amqp://testuser:testpass@testhost:5672/test" {
		t.Errorf("Unexpected URL: %s", connConfig.URL)
	}
	if connConfig.ConnectionName != "test-conn" {
		t.Errorf("Expected connection name 'test-conn', got '%s'", connConfig.ConnectionName)
	}
	if connConfig.Heartbeat != 15*time.Second {
		t.Errorf("Expected heartbeat 15s, got %v", connConfig.Heartbeat)
	}
	if connConfig.RetryAttempts != 3 {
		t.Errorf("Expected retry attempts 3, got %d", connConfig.RetryAttempts)
	}
}

func TestEnvDefaults(t *testing.T) {
	// Ensure no relevant environment variables are set
	envVarsToUnset := []string{
		"RABBITMQ_USERNAME", "RABBITMQ_PASSWORD", "RABBITMQ_HOST", "RABBITMQ_PORT",
		"RABBITMQ_VHOST", "RABBITMQ_PROTOCOL", "RABBITMQ_CONNECTION_NAME",
	}

	for _, envVar := range envVarsToUnset {
		os.Unsetenv(envVar)
	}

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config with defaults: %v", err)
	}

	// Verify defaults
	if config.Username != "guest" {
		t.Errorf("Expected default username 'guest', got '%s'", config.Username)
	}
	if config.Password != "guest" {
		t.Errorf("Expected default password 'guest', got '%s'", config.Password)
	}
	if config.Host != "localhost" {
		t.Errorf("Expected default host 'localhost', got '%s'", config.Host)
	}
	if config.Port != 5672 {
		t.Errorf("Expected default port 5672, got %d", config.Port)
	}
	if config.VHost != "/" {
		t.Errorf("Expected default vhost '/', got '%s'", config.VHost)
	}
	if config.Protocol != "amqp" {
		t.Errorf("Expected default protocol 'amqp', got '%s'", config.Protocol)
	}
	if config.ConnectionName != "go-rabbitmq" {
		t.Errorf("Expected default connection name 'go-rabbitmq', got '%s'", config.ConnectionName)
	}
}
