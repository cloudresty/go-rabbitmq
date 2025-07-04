// Package shutdown provides coordinated graceful shutdown management for RabbitMQ components
// and other closable resources. It handles signal management, timeout control, and
// tracking of in-flight operations during shutdown.
package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cloudresty/go-rabbitmq"
)

// Logger interface for structured logging
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
}

// Field represents a log field
type Field interface {
	Key() string
	Value() any
}

// SimpleField implements the Field interface for simple key-value pairs
type SimpleField struct {
	key   string
	value interface{}
}

func (f SimpleField) Key() string {
	return f.key
}

func (f SimpleField) Value() any {
	return f.value
}

// Helper functions for creating fields
func StringField(key, value string) Field {
	return SimpleField{key: key, value: value}
}

func IntField(key string, value int) Field {
	return SimpleField{key: key, value: value}
}

func DurationField(key string, value time.Duration) Field {
	return SimpleField{key: key, value: value}
}

// NoOpLogger provides a no-operation logger implementation
type NoOpLogger struct{}

func (l NoOpLogger) Debug(msg string, fields ...Field) {}
func (l NoOpLogger) Info(msg string, fields ...Field)  {}
func (l NoOpLogger) Warn(msg string, fields ...Field)  {}
func (l NoOpLogger) Error(msg string, fields ...Field) {}

// ShutdownManager provides coordinated graceful shutdown for RabbitMQ components
type ShutdownManager struct {
	components []Closable
	timeout    time.Duration
	mu         sync.RWMutex
	done       chan struct{}
	once       sync.Once
	inFlight   *InFlightTracker
	logger     Logger
}

// Closable interface for components that can be gracefully closed
type Closable interface {
	Close() error
}

// ShutdownConfig holds configuration for the shutdown manager
type ShutdownConfig struct {
	Timeout           time.Duration // Overall shutdown timeout
	SignalTimeout     time.Duration // Additional time to wait after receiving signal
	GracefulDrainTime time.Duration // Time to allow for in-flight operations to complete
	Logger            Logger        // Logger for shutdown events
}

// DefaultShutdownConfig returns sensible default shutdown configuration
func DefaultShutdownConfig() ShutdownConfig {
	return ShutdownConfig{
		Timeout:           time.Second * 30, // Total shutdown timeout
		SignalTimeout:     time.Second * 5,  // Grace period after signal
		GracefulDrainTime: time.Second * 10, // Time for in-flight operations
		Logger:            NoOpLogger{},     // No-op logger by default
	}
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(config ShutdownConfig) *ShutdownManager {
	logger := config.Logger
	if logger == nil {
		logger = NoOpLogger{}
	}

	return &ShutdownManager{
		components: make([]Closable, 0),
		timeout:    config.Timeout,
		done:       make(chan struct{}),
		inFlight:   NewInFlightTracker(),
		logger:     logger,
	}
}

// Register adds a component to be managed during shutdown
func (sm *ShutdownManager) Register(component Closable) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.components = append(sm.components, component)

	sm.logger.Debug("Component registered for graceful shutdown",
		IntField("total_components", len(sm.components)))
}

// SetupSignalHandler sets up signal handling for graceful shutdown
func (sm *ShutdownManager) SetupSignalHandler() <-chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig := <-sigChan
		sm.logger.Info("Received shutdown signal",
			StringField("signal", sig.String()))
		sm.Shutdown()
	}()

	return sigChan
}

// Shutdown performs graceful shutdown of all registered components
func (sm *ShutdownManager) Shutdown() {
	sm.once.Do(func() {
		sm.logger.Info("Starting graceful shutdown",
			DurationField("timeout", sm.timeout),
			IntField("components", len(sm.components)))

		// Create timeout context
		ctx, cancel := context.WithTimeout(context.Background(), sm.timeout)
		defer cancel()

		// Channel to signal completion
		done := make(chan struct{})

		go func() {
			defer close(done)
			sm.shutdownComponents()
		}()

		// Wait for shutdown completion or timeout
		select {
		case <-done:
			sm.logger.Info("Graceful shutdown completed successfully")
		case <-ctx.Done():
			sm.logger.Warn("Graceful shutdown timeout exceeded",
				DurationField("timeout", sm.timeout))
		}

		close(sm.done)
	})
}

// shutdownComponents shuts down all registered components
func (sm *ShutdownManager) shutdownComponents() {
	sm.mu.RLock()
	components := make([]Closable, len(sm.components))
	copy(components, sm.components)
	sm.mu.RUnlock()

	var wg sync.WaitGroup
	errors := make(chan error, len(components))

	// Shutdown all components concurrently
	for i, component := range components {
		wg.Add(1)
		go func(idx int, comp Closable) {
			defer wg.Done()

			sm.logger.Debug("Shutting down component",
				IntField("component_index", idx))

			if err := comp.Close(); err != nil {
				sm.logger.Error("Error shutting down component",
					IntField("component_index", idx),
					StringField("error", err.Error()))
				errors <- err
			} else {
				sm.logger.Debug("Component shutdown successful",
					IntField("component_index", idx))
			}
		}(i, component)
	}

	// Wait for all components to shutdown
	wg.Wait()
	close(errors)

	// Log any errors that occurred
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		sm.logger.Warn("Some components failed to shutdown cleanly",
			IntField("error_count", errorCount),
			IntField("total_components", len(components)))
	} else {
		sm.logger.Info("All components shutdown successfully",
			IntField("total_components", len(components)))
	}

	// Wait for in-flight operations to complete
	_ = sm.inFlight.CloseWithTimeout(sm.timeout) // Ignore error during shutdown
}

// Wait blocks until shutdown is complete
func (sm *ShutdownManager) Wait() {
	<-sm.done
}

// WaitWithContext blocks until shutdown is complete or context is cancelled
func (sm *ShutdownManager) WaitWithContext(ctx context.Context) error {
	select {
	case <-sm.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsShutdown returns true if shutdown has been initiated
func (sm *ShutdownManager) IsShutdown() bool {
	select {
	case <-sm.done:
		return true
	default:
		return false
	}
}

// GetInFlightTracker returns the in-flight operations tracker
func (sm *ShutdownManager) GetInFlightTracker() *InFlightTracker {
	return sm.inFlight
}

// InFlightTracker tracks in-flight operations for graceful shutdown
type InFlightTracker struct {
	wg     sync.WaitGroup
	closed bool
	mu     sync.RWMutex
}

// NewInFlightTracker creates a new in-flight operations tracker
func NewInFlightTracker() *InFlightTracker {
	return &InFlightTracker{}
}

// Start marks the beginning of an operation
func (ift *InFlightTracker) Start() bool {
	ift.mu.RLock()
	defer ift.mu.RUnlock()

	if ift.closed {
		return false // Don't allow new operations during shutdown
	}

	ift.wg.Add(1)
	return true
}

// Done marks the completion of an operation
func (ift *InFlightTracker) Done() {
	ift.wg.Done()
}

// Close prevents new operations and waits for existing ones to complete
func (ift *InFlightTracker) Close() error {
	ift.mu.Lock()
	ift.closed = true
	ift.mu.Unlock()

	ift.wg.Wait()
	return nil
}

// CloseWithTimeout waits for in-flight operations with a timeout
func (ift *InFlightTracker) CloseWithTimeout(timeout time.Duration) error {
	ift.mu.Lock()
	ift.closed = true
	ift.mu.Unlock()

	done := make(chan struct{})
	go func() {
		ift.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return context.DeadlineExceeded
	}
}

// IsClosed returns true if the tracker is closed
func (ift *InFlightTracker) IsClosed() bool {
	ift.mu.RLock()
	defer ift.mu.RUnlock()
	return ift.closed
}

// Interface compliance methods for rabbitmq.GracefulShutdown

// RegisterComponent implements the simplified GracefulShutdown interface
func (sm *ShutdownManager) RegisterComponent(component rabbitmq.Closable) error {
	sm.Register(component)
	return nil
}

// ShutdownGracefully implements the simplified GracefulShutdown interface
func (sm *ShutdownManager) ShutdownGracefully(ctx context.Context) error {
	sm.Shutdown()
	return sm.WaitWithContext(ctx)
}

// SetupSignalHandling implements the simplified GracefulShutdown interface
func (sm *ShutdownManager) SetupSignalHandling() <-chan struct{} {
	signalCh := sm.SetupSignalHandler()
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		<-signalCh // Wait for signal
		sm.Shutdown()
	}()

	return doneCh
}

// IsShutdownComplete implements the simplified GracefulShutdown interface
func (sm *ShutdownManager) IsShutdownComplete() bool {
	return sm.IsShutdown()
}

// Ensure ShutdownManager implements the interface at compile time
var _ rabbitmq.GracefulShutdown = (*ShutdownManager)(nil)
