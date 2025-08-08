package rabbitmq

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/cloudresty/go-env"
)

// EnvConfig holds all RabbitMQ configuration that can be loaded from environment variables
type EnvConfig struct {
	// Connection basics
	Username string   `env:"RABBITMQ_USERNAME,default=guest"`
	Password string   `env:"RABBITMQ_PASSWORD,default=guest"`
	Hosts    []string `env:"RABBITMQ_HOSTS,default=localhost:5672"`
	VHost    string   `env:"RABBITMQ_VHOST,default=/"`

	// Protocol and security
	Protocol    string `env:"RABBITMQ_PROTOCOL,default=amqp"` // amqp or amqps
	TLSEnabled  bool   `env:"RABBITMQ_TLS_ENABLED,default=false"`
	TLSInsecure bool   `env:"RABBITMQ_TLS_INSECURE,default=false"` // Skip cert verification

	// HTTP Management API
	HTTPProtocol string `env:"RABBITMQ_HTTP_PROTOCOL,default=http"` // http or https
	HTTPPort     int    `env:"RABBITMQ_HTTP_PORT,default=15672"`
	HTTPHost     string `env:"RABBITMQ_HTTP_HOST,default=localhost"` // Separate host for HTTP API

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

	// Topology validation and auto-healing (enabled by default for production reliability)
	TopologyValidation           bool          `env:"RABBITMQ_TOPOLOGY_VALIDATION,default=true"`
	TopologyAutoRecreation       bool          `env:"RABBITMQ_TOPOLOGY_AUTO_RECREATION,default=true"`
	TopologyBackgroundValidation bool          `env:"RABBITMQ_TOPOLOGY_BACKGROUND_VALIDATION,default=true"`
	TopologyValidationInterval   time.Duration `env:"RABBITMQ_TOPOLOGY_VALIDATION_INTERVAL,default=30s"`
}

// BuildAMQPURL constructs the AMQP connection URL from environment configuration
// For multiple hosts, returns the first host URL (AMQP library handles failover)
func (e *EnvConfig) BuildAMQPURL() string {
	if len(e.Hosts) == 0 {
		return ""
	}

	// Use the first host for the primary connection URL
	host := e.Hosts[0]

	// Parse host:port, default to 5672 if no port specified
	if !strings.Contains(host, ":") {
		host = host + ":5672"
	}

	vhost := e.VHost
	if vhost == "/" {
		vhost = ""
	} else if vhost[0] != '/' {
		vhost = "/" + vhost
	}

	return fmt.Sprintf("%s://%s:%s@%s%s",
		e.Protocol, e.Username, e.Password, host, vhost)
}

// BuildAMQPURLs constructs all AMQP connection URLs for failover support
func (e *EnvConfig) BuildAMQPURLs() []string {
	if len(e.Hosts) == 0 {
		return nil
	}

	urls := make([]string, 0, len(e.Hosts))

	vhost := e.VHost
	if vhost == "/" {
		vhost = ""
	} else if vhost[0] != '/' {
		vhost = "/" + vhost
	}

	for _, host := range e.Hosts {
		// Parse host:port, default to 5672 if no port specified
		if !strings.Contains(host, ":") {
			host = host + ":5672"
		}

		url := fmt.Sprintf("%s://%s:%s@%s%s",
			e.Protocol, e.Username, e.Password, host, vhost)
		urls = append(urls, url)
	}

	return urls
}

// BuildHTTPURL constructs the HTTP management API URL from environment configuration
func (e *EnvConfig) BuildHTTPURL() string {
	return fmt.Sprintf("%s://%s:%d", e.HTTPProtocol, e.HTTPHost, e.HTTPPort)
}

// FromEnv creates a client option that loads configuration from environment variables
func FromEnv() Option {
	return FromEnvWithPrefix("")
}

// FromEnvWithPrefix creates a client option that loads configuration from environment variables with a custom prefix
func FromEnvWithPrefix(prefix string) Option {
	return func(config *clientConfig) error {
		var envConfig EnvConfig
		var err error

		if prefix != "" {
			// Use custom prefix
			err = env.Bind(&envConfig, env.BindingOptions{
				Tag:      "env",
				Prefix:   prefix,
				Required: false,
			})
		} else {
			// Use default RABBITMQ_ prefix
			bindOptions := env.DefaultBindingOptions()
			err = env.Bind(&envConfig, bindOptions)
		}

		if err != nil {
			return fmt.Errorf("failed to parse environment configuration: %w", err)
		}

		// Apply environment configuration to client config
		return applyEnvConfigToClient(config, &envConfig)
	}
}

// applyEnvConfigToClient applies environment configuration to the client config
func applyEnvConfigToClient(config *clientConfig, envConfig *EnvConfig) error {
	// Build URLs from multiple hosts for failover support
	urls := envConfig.BuildAMQPURLs()
	if len(urls) > 0 {
		config.URLs = urls
		config.URL = urls[0] // Primary URL
		config.Hosts = envConfig.Hosts
	}

	config.Username = envConfig.Username
	config.Password = envConfig.Password
	config.VHost = envConfig.VHost

	// Connection behavior
	config.ConnectionName = envConfig.ConnectionName
	config.Heartbeat = envConfig.Heartbeat
	config.DialTimeout = envConfig.DialTimeout
	config.ChannelTimeout = envConfig.ChannelTimeout

	// TLS configuration
	if envConfig.TLSEnabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: envConfig.TLSInsecure,
		}
		config.TLS = tlsConfig
	}

	// Reconnection settings
	config.AutoReconnect = envConfig.AutoReconnect
	config.ReconnectDelay = envConfig.ReconnectDelay
	config.MaxReconnectAttempts = envConfig.MaxReconnectAttempts

	// Set up retry policy from environment
	config.ReconnectPolicy = &ExponentialBackoff{
		InitialDelay: envConfig.RetryDelay,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  envConfig.RetryAttempts,
	}

	// Topology validation settings (production-ready defaults)
	config.TopologyValidation = envConfig.TopologyValidation
	config.TopologyAutoRecreation = envConfig.TopologyAutoRecreation
	config.TopologyBackgroundValidation = envConfig.TopologyBackgroundValidation
	config.TopologyValidationInterval = envConfig.TopologyValidationInterval

	return nil
}
