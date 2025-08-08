package rabbitmq

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"
)

// Connection options

// WithCredentials sets the username and password
func WithCredentials(username, password string) Option {
	return func(config *clientConfig) error {
		config.Username = username
		config.Password = password
		return nil
	}
}

// WithHosts sets multiple RabbitMQ hosts for failover support
// Each host should include port (e.g., "host1:5672,host2:5673")
// If no port is specified, defaults to 5672
func WithHosts(hosts ...string) Option {
	return func(config *clientConfig) error {
		if len(hosts) == 0 {
			return fmt.Errorf("at least one host must be provided")
		}

		// Validate and normalize hosts
		normalizedHosts := make([]string, len(hosts))
		for i, host := range hosts {
			if host == "" {
				return fmt.Errorf("host cannot be empty")
			}
			// Add default port if not specified
			if !strings.Contains(host, ":") {
				normalizedHosts[i] = host + ":5672"
			} else {
				normalizedHosts[i] = host
			}
		}

		config.Hosts = normalizedHosts
		return nil
	}
}

// WithVHost sets the virtual host
func WithVHost(vhost string) Option {
	return func(config *clientConfig) error {
		config.VHost = vhost
		return nil
	}
}

// WithTLS sets the TLS configuration
func WithTLS(tlsConfig *tls.Config) Option {
	return func(config *clientConfig) error {
		config.TLS = tlsConfig
		return nil
	}
}

// WithInsecureTLS enables TLS with certificate verification disabled
func WithInsecureTLS() Option {
	return func(config *clientConfig) error {
		config.TLS = &tls.Config{
			InsecureSkipVerify: true,
		}
		return nil
	}
}

// Connection behavior options

// WithConnectionName sets the connection name
func WithConnectionName(name string) Option {
	return func(config *clientConfig) error {
		config.ConnectionName = name
		return nil
	}
}

// WithHeartbeat sets the heartbeat interval
func WithHeartbeat(duration time.Duration) Option {
	return func(config *clientConfig) error {
		if duration < 0 {
			return fmt.Errorf("heartbeat duration cannot be negative")
		}
		config.Heartbeat = duration
		return nil
	}
}

// WithDialTimeout sets the connection dial timeout
func WithDialTimeout(timeout time.Duration) Option {
	return func(config *clientConfig) error {
		if timeout <= 0 {
			return fmt.Errorf("dial timeout must be positive")
		}
		config.DialTimeout = timeout
		return nil
	}
}

// WithChannelTimeout sets the channel operation timeout
func WithChannelTimeout(timeout time.Duration) Option {
	return func(config *clientConfig) error {
		if timeout <= 0 {
			return fmt.Errorf("channel timeout must be positive")
		}
		config.ChannelTimeout = timeout
		return nil
	}
}

// Reconnection policy options

// WithReconnectPolicy sets the reconnection policy
func WithReconnectPolicy(policy ReconnectPolicy) Option {
	return func(config *clientConfig) error {
		config.ReconnectPolicy = policy
		return nil
	}
}

// WithMaxReconnectAttempts sets the maximum number of reconnection attempts
func WithMaxReconnectAttempts(attempts int) Option {
	return func(config *clientConfig) error {
		if attempts < 0 {
			return fmt.Errorf("max reconnect attempts cannot be negative")
		}
		config.MaxReconnectAttempts = attempts
		return nil
	}
}

// WithReconnectDelay sets the delay between reconnection attempts
func WithReconnectDelay(delay time.Duration) Option {
	return func(config *clientConfig) error {
		if delay < 0 {
			return fmt.Errorf("reconnect delay cannot be negative")
		}
		config.ReconnectDelay = delay
		return nil
	}
}

// WithAutoReconnect enables or disables automatic reconnection
func WithAutoReconnect(enabled bool) Option {
	return func(config *clientConfig) error {
		config.AutoReconnect = enabled
		return nil
	}
}

// Observability options

// WithLogger sets the logger
func WithLogger(logger Logger) Option {
	return func(config *clientConfig) error {
		if logger == nil {
			return fmt.Errorf("logger cannot be nil")
		}
		config.Logger = logger
		return nil
	}
}

// WithMetrics sets the metrics collector
func WithMetrics(metrics MetricsCollector) Option {
	return func(config *clientConfig) error {
		if metrics == nil {
			return fmt.Errorf("metrics collector cannot be nil")
		}
		config.Metrics = metrics
		return nil
	}
}

// WithTracing sets the tracer
func WithTracing(tracer Tracer) Option {
	return func(config *clientConfig) error {
		if tracer == nil {
			return fmt.Errorf("tracer cannot be nil")
		}
		config.Tracer = tracer
		return nil
	}
}

// Security options

// WithAccessPolicy sets the access control policy
func WithAccessPolicy(policy *AccessPolicy) Option {
	return func(config *clientConfig) error {
		config.AccessPolicy = policy
		return nil
	}
}

// WithAuditLogging sets the audit logger
func WithAuditLogging(logger AuditLogger) Option {
	return func(config *clientConfig) error {
		if logger == nil {
			return fmt.Errorf("audit logger cannot be nil")
		}
		config.AuditLogger = logger
		return nil
	}
}

// WithPerformanceMonitoring sets the performance monitor
func WithPerformanceMonitoring(monitor PerformanceMonitor) Option {
	return func(config *clientConfig) error {
		if monitor == nil {
			return fmt.Errorf("performance monitor cannot be nil")
		}
		config.PerformanceMonitor = monitor
		return nil
	}
}

// WithConnectionPooler sets the connection pooler
func WithConnectionPooler(pooler ConnectionPooler) Option {
	return func(config *clientConfig) error {
		if pooler == nil {
			return fmt.Errorf("connection pooler cannot be nil")
		}
		config.ConnectionPooler = pooler
		return nil
	}
}

// WithStreamHandler sets the stream handler
func WithStreamHandler(handler StreamHandler) Option {
	return func(config *clientConfig) error {
		if handler == nil {
			return fmt.Errorf("stream handler cannot be nil")
		}
		config.StreamHandler = handler
		return nil
	}
}

// WithSagaOrchestrator sets the saga orchestrator
func WithSagaOrchestrator(orchestrator SagaOrchestrator) Option {
	return func(config *clientConfig) error {
		if orchestrator == nil {
			return fmt.Errorf("saga orchestrator cannot be nil")
		}
		config.SagaOrchestrator = orchestrator
		return nil
	}
}

// WithGracefulShutdown sets the graceful shutdown handler
func WithGracefulShutdown(shutdown GracefulShutdown) Option {
	return func(config *clientConfig) error {
		if shutdown == nil {
			return fmt.Errorf("graceful shutdown handler cannot be nil")
		}
		config.GracefulShutdown = shutdown
		return nil
	}
}

// Convenience functions for common configurations

// DefaultReconnectPolicy returns a sensible default reconnection policy
func DefaultReconnectPolicy() ReconnectPolicy {
	return &ExponentialBackoff{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  0, // Unlimited
	}
}

// ProductionReconnectPolicy returns a production-ready reconnection policy
func ProductionReconnectPolicy() ReconnectPolicy {
	return &ExponentialBackoff{
		InitialDelay: 5 * time.Second,
		MaxDelay:     5 * time.Minute,
		Multiplier:   2.0,
		MaxAttempts:  10,
	}
}

// TestingReconnectPolicy returns a fast reconnection policy for testing
func TestingReconnectPolicy() ReconnectPolicy {
	return &FixedDelay{
		Delay:       100 * time.Millisecond,
		MaxAttempts: 3,
	}
}

// Topology validation options

// WithTopologyValidation enables topology validation
// When enabled, the client will track declared topology and validate it before operations
func WithTopologyValidation() Option {
	return func(config *clientConfig) error {
		config.TopologyValidation = true
		return nil
	}
}

// WithTopologyAutoRecreation enables automatic recreation of missing topology
// Requires TopologyValidation to be enabled
func WithTopologyAutoRecreation() Option {
	return func(config *clientConfig) error {
		config.TopologyAutoRecreation = true
		return nil
	}
}

// WithTopologyBackgroundValidation enables background topology validation
// with a custom interval. Background validation is enabled by default (30s interval).
// Requires TopologyValidation to be enabled
func WithTopologyBackgroundValidation(interval time.Duration) Option {
	return func(config *clientConfig) error {
		if interval <= 0 {
			return fmt.Errorf("validation interval must be positive")
		}
		config.TopologyBackgroundValidation = true
		config.TopologyValidationInterval = interval
		return nil
	}
}

// WithTopologyValidationInterval sets the background validation interval
// This is a convenience method that automatically enables background validation
func WithTopologyValidationInterval(interval time.Duration) Option {
	return func(config *clientConfig) error {
		if interval <= 0 {
			return fmt.Errorf("validation interval must be positive")
		}
		config.TopologyBackgroundValidation = true
		config.TopologyValidationInterval = interval
		return nil
	}
}

// Opt-out topology validation options (for advanced users)

// WithoutTopologyValidation disables topology validation
// This disables all topology tracking, validation, and auto-recreation
func WithoutTopologyValidation() Option {
	return func(config *clientConfig) error {
		config.TopologyValidation = false
		config.TopologyAutoRecreation = false
		config.TopologyBackgroundValidation = false
		return nil
	}
}

// WithoutTopologyAutoRecreation disables automatic recreation while keeping validation
// Topology will be validated but not automatically recreated if missing
func WithoutTopologyAutoRecreation() Option {
	return func(config *clientConfig) error {
		config.TopologyAutoRecreation = false
		return nil
	}
}

// WithoutTopologyBackgroundValidation disables background validation while keeping on-demand validation
// Topology will be validated before operations but not periodically validated in the background
func WithoutTopologyBackgroundValidation() Option {
	return func(config *clientConfig) error {
		config.TopologyBackgroundValidation = false
		return nil
	}
}
