package rabbitmq

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cloudresty/emit"
)

// ShutdownManager provides coordinated graceful shutdown for RabbitMQ components
type ShutdownManager struct {
	components []Closable
	timeout    time.Duration
	mu         sync.RWMutex
	done       chan struct{}
	once       sync.Once
	inFlight   *InFlightTracker
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
}

// DefaultShutdownConfig returns sensible default shutdown configuration
func DefaultShutdownConfig() ShutdownConfig {

	return ShutdownConfig{
		Timeout:           time.Second * 30, // Total shutdown timeout
		SignalTimeout:     time.Second * 5,  // Grace period after signal
		GracefulDrainTime: time.Second * 10, // Time for in-flight operations
	}

}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(config ShutdownConfig) *ShutdownManager {

	return &ShutdownManager{
		components: make([]Closable, 0),
		timeout:    config.Timeout,
		done:       make(chan struct{}),
		inFlight:   NewInFlightTracker(),
	}

}

// Register adds a component to be managed during shutdown
func (sm *ShutdownManager) Register(component Closable) {

	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.components = append(sm.components, component)

	emit.Debug.StructuredFields("Component registered for graceful shutdown",
		emit.ZInt("total_components", len(sm.components)))

}

// SetupSignalHandler sets up signal handling for graceful shutdown
func (sm *ShutdownManager) SetupSignalHandler() <-chan os.Signal {

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig := <-sigChan
		emit.Info.StructuredFields("Received shutdown signal",
			emit.ZString("signal", sig.String()))
		sm.Shutdown()
	}()

	return sigChan

}

// Shutdown performs graceful shutdown of all registered components
func (sm *ShutdownManager) Shutdown() {

	sm.once.Do(func() {
		emit.Info.StructuredFields("Starting graceful shutdown",
			emit.ZDuration("timeout", sm.timeout),
			emit.ZInt("components", len(sm.components)))

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
			emit.Info.Msg("Graceful shutdown completed successfully")
		case <-ctx.Done():
			emit.Warn.StructuredFields("Graceful shutdown timeout exceeded",
				emit.ZDuration("timeout", sm.timeout))
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

			emit.Debug.StructuredFields("Shutting down component",
				emit.ZInt("component_index", idx))

			if err := comp.Close(); err != nil {
				emit.Error.StructuredFields("Error shutting down component",
					emit.ZInt("component_index", idx),
					emit.ZString("error", err.Error()))
				errors <- err
			} else {
				emit.Debug.StructuredFields("Component shutdown successful",
					emit.ZInt("component_index", idx))
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
		emit.Warn.StructuredFields("Some components failed to shutdown cleanly",
			emit.ZInt("error_count", errorCount),
			emit.ZInt("total_components", len(components)))
	} else {
		emit.Info.StructuredFields("All components shutdown successfully",
			emit.ZInt("total_components", len(components)))
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

	emit.Debug.Msg("Waiting for in-flight operations to complete")
	ift.wg.Wait()
	emit.Debug.Msg("All in-flight operations completed")

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
		emit.Debug.Msg("All in-flight operations completed")
		return nil
	case <-time.After(timeout):
		emit.Warn.StructuredFields("Timeout waiting for in-flight operations to complete",
			emit.ZDuration("timeout", timeout))
		return context.DeadlineExceeded
	}

}

// IsClosed returns true if the tracker is closed
func (ift *InFlightTracker) IsClosed() bool {

	ift.mu.RLock()
	defer ift.mu.RUnlock()
	return ift.closed

}
