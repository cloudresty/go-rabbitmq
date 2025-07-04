package shutdown

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Mock component for testing
type mockComponent struct {
	closeDelay time.Duration
	closeError error
	closed     bool
	mu         sync.Mutex
}

func (mc *mockComponent) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closeDelay > 0 {
		time.Sleep(mc.closeDelay)
	}

	mc.closed = true
	return mc.closeError
}

func (mc *mockComponent) IsClosed() bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.closed
}

// Mock logger for testing
type mockLogger struct {
	logs []logEntry
	mu   sync.Mutex
}

type logEntry struct {
	level   string
	message string
	fields  []Field
}

func (ml *mockLogger) Debug(msg string, fields ...Field) {
	ml.addLog("DEBUG", msg, fields...)
}

func (ml *mockLogger) Info(msg string, fields ...Field) {
	ml.addLog("INFO", msg, fields...)
}

func (ml *mockLogger) Warn(msg string, fields ...Field) {
	ml.addLog("WARN", msg, fields...)
}

func (ml *mockLogger) Error(msg string, fields ...Field) {
	ml.addLog("ERROR", msg, fields...)
}

func (ml *mockLogger) addLog(level, msg string, fields ...Field) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.logs = append(ml.logs, logEntry{
		level:   level,
		message: msg,
		fields:  fields,
	})
}

func (ml *mockLogger) GetLogs() []logEntry {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	result := make([]logEntry, len(ml.logs))
	copy(result, ml.logs)
	return result
}

func TestDefaultShutdownConfig(t *testing.T) {
	config := DefaultShutdownConfig()

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", config.Timeout)
	}

	if config.SignalTimeout != 5*time.Second {
		t.Errorf("Expected signal timeout 5s, got %v", config.SignalTimeout)
	}

	if config.GracefulDrainTime != 10*time.Second {
		t.Errorf("Expected drain time 10s, got %v", config.GracefulDrainTime)
	}

	if config.Logger == nil {
		t.Error("Expected logger to be set")
	}
}

func TestNewShutdownManager(t *testing.T) {
	config := DefaultShutdownConfig()
	manager := NewShutdownManager(config)

	if manager == nil {
		t.Error("NewShutdownManager returned nil")
		return
	}

	if manager.timeout != config.Timeout {
		t.Errorf("Expected timeout %v, got %v", config.Timeout, manager.timeout)
	}

	if len(manager.components) != 0 {
		t.Errorf("Expected 0 components, got %d", len(manager.components))
	}

	if manager.inFlight == nil {
		t.Error("InFlight tracker should be initialized")
	}
}

func TestShutdownManager_Register(t *testing.T) {
	config := DefaultShutdownConfig()
	manager := NewShutdownManager(config)

	component1 := &mockComponent{}
	component2 := &mockComponent{}

	manager.Register(component1)
	if len(manager.components) != 1 {
		t.Errorf("Expected 1 component, got %d", len(manager.components))
	}

	manager.Register(component2)
	if len(manager.components) != 2 {
		t.Errorf("Expected 2 components, got %d", len(manager.components))
	}
}

func TestShutdownManager_Shutdown(t *testing.T) {
	logger := &mockLogger{}
	config := DefaultShutdownConfig()
	config.Timeout = time.Second * 2
	config.Logger = logger

	manager := NewShutdownManager(config)

	component1 := &mockComponent{closeDelay: time.Millisecond * 100}
	component2 := &mockComponent{closeDelay: time.Millisecond * 150}

	manager.Register(component1)
	manager.Register(component2)

	// Shutdown should not block indefinitely
	done := make(chan struct{})
	go func() {
		manager.Shutdown()
		close(done)
	}()

	// Wait for shutdown to complete
	select {
	case <-done:
		// Success
	case <-time.After(time.Second * 5):
		t.Fatal("Shutdown did not complete within timeout")
	}

	// Verify components were closed
	if !component1.IsClosed() {
		t.Error("Component 1 was not closed")
	}

	if !component2.IsClosed() {
		t.Error("Component 2 was not closed")
	}

	// Verify shutdown state
	if !manager.IsShutdown() {
		t.Error("Manager should be in shutdown state")
	}

	// Verify logs
	logs := logger.GetLogs()
	hasStartLog := false
	hasCompleteLog := false

	for _, log := range logs {
		if log.message == "Starting graceful shutdown" {
			hasStartLog = true
		}
		if log.message == "Graceful shutdown completed successfully" {
			hasCompleteLog = true
		}
	}

	if !hasStartLog {
		t.Error("Expected 'Starting graceful shutdown' log")
	}

	if !hasCompleteLog {
		t.Error("Expected 'Graceful shutdown completed successfully' log")
	}
}

func TestShutdownManager_ShutdownWithError(t *testing.T) {
	logger := &mockLogger{}
	config := DefaultShutdownConfig()
	config.Timeout = time.Second
	config.Logger = logger

	manager := NewShutdownManager(config)

	component1 := &mockComponent{}
	component2 := &mockComponent{closeError: errors.New("test error")}

	manager.Register(component1)
	manager.Register(component2)

	manager.Shutdown()
	manager.Wait()

	// Both components should be processed even if one fails
	if !component1.IsClosed() {
		t.Error("Component 1 was not closed")
	}

	if !component2.IsClosed() {
		t.Error("Component 2 was not closed")
	}

	// Check for error in logs
	logs := logger.GetLogs()
	hasErrorLog := false

	for _, log := range logs {
		if log.level == "ERROR" && log.message == "Error shutting down component" {
			hasErrorLog = true
		}
	}

	if !hasErrorLog {
		t.Error("Expected error log for component shutdown failure")
	}
}

func TestShutdownManager_Wait(t *testing.T) {
	config := DefaultShutdownConfig()
	manager := NewShutdownManager(config)

	component := &mockComponent{closeDelay: time.Millisecond * 100}
	manager.Register(component)

	// Start shutdown in goroutine
	go func() {
		time.Sleep(time.Millisecond * 50)
		manager.Shutdown()
	}()

	// Wait should block until shutdown completes
	start := time.Now()
	manager.Wait()
	duration := time.Since(start)

	if duration < time.Millisecond*50 {
		t.Error("Wait returned too early")
	}

	if !component.IsClosed() {
		t.Error("Component was not closed")
	}
}

func TestShutdownManager_WaitWithContext(t *testing.T) {
	config := DefaultShutdownConfig()
	manager := NewShutdownManager(config)

	component := &mockComponent{closeDelay: time.Millisecond * 200}
	manager.Register(component)

	// Start shutdown
	go func() {
		time.Sleep(time.Millisecond * 50)
		manager.Shutdown()
	}()

	// Wait with context that times out before shutdown completes
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	err := manager.WaitWithContext(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestShutdownManager_SetupSignalHandler(t *testing.T) {
	config := DefaultShutdownConfig()
	manager := NewShutdownManager(config)

	component := &mockComponent{}
	manager.Register(component)

	// Setup signal handler
	_ = manager.SetupSignalHandler()

	// Manually trigger shutdown to test the component closure
	manager.Shutdown()

	// Wait for shutdown to complete
	manager.Wait()

	if !component.IsClosed() {
		t.Error("Component was not closed after shutdown")
	}
}

func TestInFlightTracker(t *testing.T) {
	tracker := NewInFlightTracker()

	if tracker.IsClosed() {
		t.Error("New tracker should not be closed")
	}

	// Test starting operations
	if !tracker.Start() {
		t.Error("Should be able to start operation")
	}

	if !tracker.Start() {
		t.Error("Should be able to start multiple operations")
	}

	// Mark operations as done
	tracker.Done()
	tracker.Done()

	// Close tracker
	err := tracker.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	if !tracker.IsClosed() {
		t.Error("Tracker should be closed")
	}

	// Should not be able to start new operations after close
	if tracker.Start() {
		t.Error("Should not be able to start operation after close")
	}
}

func TestInFlightTracker_CloseWithTimeout(t *testing.T) {
	tracker := NewInFlightTracker()

	// Start an operation that won't complete
	tracker.Start()
	// Intentionally not calling Done()

	// Close with short timeout should timeout
	err := tracker.CloseWithTimeout(time.Millisecond * 10)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}

	if !tracker.IsClosed() {
		t.Error("Tracker should be closed even after timeout")
	}
}

func TestInFlightTracker_CloseWithTimeout_Success(t *testing.T) {
	tracker := NewInFlightTracker()

	// Start and complete operations quickly
	go func() {
		tracker.Start()
		time.Sleep(time.Millisecond * 5)
		tracker.Done()
	}()

	// Close with sufficient timeout should succeed
	err := tracker.CloseWithTimeout(time.Millisecond * 100)
	if err != nil {
		t.Errorf("Close should succeed, got error: %v", err)
	}
}

func TestSimpleFields(t *testing.T) {
	stringField := StringField("test", "value")
	if stringField.Key() != "test" {
		t.Errorf("Expected key 'test', got %s", stringField.Key())
	}
	if stringField.Value() != "value" {
		t.Errorf("Expected value 'value', got %v", stringField.Value())
	}

	intField := IntField("count", 42)
	if intField.Key() != "count" {
		t.Errorf("Expected key 'count', got %s", intField.Key())
	}
	if intField.Value() != 42 {
		t.Errorf("Expected value 42, got %v", intField.Value())
	}

	durationField := DurationField("timeout", time.Second)
	if durationField.Key() != "timeout" {
		t.Errorf("Expected key 'timeout', got %s", durationField.Key())
	}
	if durationField.Value() != time.Second {
		t.Errorf("Expected value 1s, got %v", durationField.Value())
	}
}

func TestNoOpLogger(t *testing.T) {
	logger := NoOpLogger{}

	// Should not panic
	logger.Debug("test")
	logger.Info("test")
	logger.Warn("test")
	logger.Error("test")
}

func TestShutdownManager_OnceGuarantee(t *testing.T) {
	config := DefaultShutdownConfig()
	manager := NewShutdownManager(config)

	component := &mockComponent{}
	manager.Register(component)

	// Call shutdown multiple times
	go manager.Shutdown()
	go manager.Shutdown()
	go manager.Shutdown()

	manager.Wait()

	// Component should only be closed once
	if !component.IsClosed() {
		t.Error("Component was not closed")
	}

	// Shutdown should be idempotent
	manager.Shutdown() // Should not block or cause issues
}
