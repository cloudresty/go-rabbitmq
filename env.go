package rabbitmq

import (
	"fmt"
	"time"

	"github.com/cloudresty/go-env"
)

// EnvConfig holds all RabbitMQ configuration that can be loaded from environment variables
type EnvConfig struct {
	// Connection basics
	Username string `env:"RABBITMQ_USERNAME,default=guest"`
	Password string `env:"RABBITMQ_PASSWORD,default=guest"`
	Host     string `env:"RABBITMQ_HOST,default=localhost"`
	Port     int    `env:"RABBITMQ_PORT,default=5672"`
	VHost    string `env:"RABBITMQ_VHOST,default=/"`

	// Protocol and security
	Protocol    string `env:"RABBITMQ_PROTOCOL,default=amqp"` // amqp or amqps
	TLSEnabled  bool   `env:"RABBITMQ_TLS_ENABLED,default=false"`
	TLSInsecure bool   `env:"RABBITMQ_TLS_INSECURE,default=false"` // Skip cert verification

	// HTTP Management API
	HTTPProtocol string `env:"RABBITMQ_HTTP_PROTOCOL,default=http"` // http or https
	HTTPPort     int    `env:"RABBITMQ_HTTP_PORT,default=15672"`

	// Connection behavior
	ConnectionName string        `env:"RABBITMQ_CONNECTION_NAME,default=go-rabbitmq"`
	Heartbeat      time.Duration `env:"RABBITMQ_HEARTBEAT,default=10s"`
	RetryAttempts  int           `env:"RABBITMQ_RETRY_ATTEMPTS,default=5"`
	RetryDelay     time.Duration `env:"RABBITMQ_RETRY_DELAY,default=2s"`

	// Timeouts
	DialTimeout    time.Duration `env:"RABBITMQ_DIAL_TIMEOUT,default=30s"`
	ChannelTimeout time.Duration `env:"RABBITMQ_CHANNEL_TIMEOUT,default=10s"`

	// Auto-reconnection
	AutoReconnect        bool          `env:"RABBITMQ_AUTO_RECONNECT,default=true"`
	ReconnectDelay       time.Duration `env:"RABBITMQ_RECONNECT_DELAY,default=5s"`
	MaxReconnectAttempts int           `env:"RABBITMQ_MAX_RECONNECT_ATTEMPTS,default=0"` // 0 = unlimited

	// Publisher settings
	PublisherConfirmationTimeout time.Duration `env:"RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT,default=5s"`
	PublisherShutdownTimeout     time.Duration `env:"RABBITMQ_PUBLISHER_SHUTDOWN_TIMEOUT,default=15s"`
	PublisherPersistent          bool          `env:"RABBITMQ_PUBLISHER_PERSISTENT,default=true"`

	// Consumer settings
	ConsumerPrefetchCount   int           `env:"RABBITMQ_CONSUMER_PREFETCH_COUNT,default=1"`
	ConsumerAutoAck         bool          `env:"RABBITMQ_CONSUMER_AUTO_ACK,default=false"`
	ConsumerMessageTimeout  time.Duration `env:"RABBITMQ_CONSUMER_MESSAGE_TIMEOUT,default=5m"`
	ConsumerShutdownTimeout time.Duration `env:"RABBITMQ_CONSUMER_SHUTDOWN_TIMEOUT,default=30s"`
}

// LoadFromEnv loads configuration from environment variables using default RABBITMQ_ prefix
func LoadFromEnv() (*EnvConfig, error) {
	var config EnvConfig
	bindOptions := env.DefaultBindingOptions()
	err := env.Bind(&config, bindOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to load RabbitMQ configuration from environment: %w", err)
	}
	return &config, nil
}

// LoadFromEnvWithPrefix loads configuration from environment variables with custom prefix
func LoadFromEnvWithPrefix(prefix string) (*EnvConfig, error) {
	var config EnvConfig

	// Bind environment variables using the specified prefix
	err := env.Bind(&config, env.BindingOptions{
		Tag:      "env",
		Prefix:   prefix,
		Required: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load RabbitMQ configuration from environment with prefix %s: %w", prefix, err)
	}
	return &config, nil
}

// BuildAMQPURL constructs the AMQP connection URL from environment configuration
func (e *EnvConfig) BuildAMQPURL() string {
	vhost := e.VHost
	if vhost == "/" {
		vhost = ""
	} else if vhost[0] != '/' {
		vhost = "/" + vhost
	}

	return fmt.Sprintf("%s://%s:%s@%s:%d%s",
		e.Protocol, e.Username, e.Password, e.Host, e.Port, vhost)
}

// BuildHTTPURL constructs the HTTP management API URL from environment configuration
func (e *EnvConfig) BuildHTTPURL() string {
	return fmt.Sprintf("%s://%s:%d", e.HTTPProtocol, e.Host, e.HTTPPort)
}

// ToConnectionConfig converts EnvConfig to ConnectionConfig
func (e *EnvConfig) ToConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		URL:                  e.BuildAMQPURL(),
		RetryAttempts:        e.RetryAttempts,
		RetryDelay:           e.RetryDelay,
		Heartbeat:            e.Heartbeat,
		ConnectionName:       e.ConnectionName,
		AutoReconnect:        e.AutoReconnect,
		ReconnectDelay:       e.ReconnectDelay,
		MaxReconnectAttempts: e.MaxReconnectAttempts,
		DialTimeout:          e.DialTimeout,
		ChannelTimeout:       e.ChannelTimeout,
	}
}

// ToPublisherConfig converts EnvConfig to PublisherConfig
func (e *EnvConfig) ToPublisherConfig() PublisherConfig {
	return PublisherConfig{
		ConnectionConfig:    e.ToConnectionConfig(),
		Persistent:          e.PublisherPersistent,
		ConfirmationTimeout: e.PublisherConfirmationTimeout,
		ShutdownTimeout:     e.PublisherShutdownTimeout,
	}
}

// ToConsumerConfig converts EnvConfig to ConsumerConfig
func (e *EnvConfig) ToConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		ConnectionConfig: e.ToConnectionConfig(),
		AutoAck:          e.ConsumerAutoAck,
		PrefetchCount:    e.ConsumerPrefetchCount,
		MessageTimeout:   e.ConsumerMessageTimeout,
		ShutdownTimeout:  e.ConsumerShutdownTimeout,
	}
}
