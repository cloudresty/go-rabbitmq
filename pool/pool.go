// Package pool provides connection pooling functionality for high-throughput RabbitMQ applications.
//
// Connection pooling allows applications to maintain multiple connections to RabbitMQ
// and distribute load across them. This is particularly useful for high-throughput
// scenarios where a single connection might become a bottleneck.
//
// Features:
//   - Round-robin connection selection
//   - Automatic health monitoring
//   - Connection repair and recovery
//   - Pool statistics and metrics
//   - Configurable pool size and behavior
//
// Example usage:
//
//	// Create a connection pool with 5 connections
//	pool, err := pool.New(5, pool.WithClientOptions(rabbitmq.WithURL("amqp://localhost")))
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer pool.Close()
//
//	// Get a client from the pool
//	client, err := pool.Get()
//	if err != nil {
//		log.Fatal("no healthy connections available:", err)
//	}
//
//	// Use the client normally
//	publisher, err := client.NewPublisher(...)
package pool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudresty/go-rabbitmq"
)

// ConnectionPool manages multiple connections for high-throughput applications
type ConnectionPool struct {
	connections    []*rabbitmq.Client
	healthyClients []*rabbitmq.Client // Pre-vetted healthy clients for fast access
	size           int
	current        uint64
	mu             sync.RWMutex
	closed         int32 // Use atomic for race-free access
	clientOptions  []rabbitmq.Option
	logger         rabbitmq.Logger

	// Health monitoring (immutable after creation)
	healthTicker    *time.Ticker
	healthInterval  time.Duration
	stopHealthCheck chan struct{}
	healthStopped   chan struct{} // Signal that health monitoring has stopped
	autoRepair      bool

	// Pool statistics
	stats internalStats
}

// Stats contains statistics about the connection pool (public API)
type Stats struct {
	Size                    int
	HealthyConnections      int
	UnhealthyConnections    int
	TotalRepairAttempts     int64
	LastHealthCheck         time.Time
	Closed                  bool
	Errors                  []string
	HealthMonitoringEnabled bool
	AutoRepairEnabled       bool
}

// internalStats contains internal statistics with mutex protection
type internalStats struct {
	mu                   sync.RWMutex
	HealthyConnections   int
	UnhealthyConnections int
	TotalRepairAttempts  int64
	LastHealthCheck      time.Time
}

// Option configures the connection pool
type Option func(*poolConfig)

// poolConfig holds internal configuration
type poolConfig struct {
	healthCheckInterval time.Duration
	autoRepair          bool
	clientOptions       []rabbitmq.Option
	logger              rabbitmq.Logger
}

// WithHealthCheck enables health monitoring with the specified interval
func WithHealthCheck(interval time.Duration) Option {
	return func(c *poolConfig) {
		c.healthCheckInterval = interval
	}
}

// WithAutoRepair enables automatic connection repair
func WithAutoRepair(enabled bool) Option {
	return func(c *poolConfig) {
		c.autoRepair = enabled
	}
}

// WithClientOptions sets options for the RabbitMQ clients in the pool
func WithClientOptions(opts ...rabbitmq.Option) Option {
	return func(c *poolConfig) {
		c.clientOptions = opts
	}
}

// WithLogger sets the logger for the connection pool
func WithLogger(logger rabbitmq.Logger) Option {
	return func(c *poolConfig) {
		c.logger = logger
	}
}

// New creates a new connection pool with the specified size and options
func New(size int, opts ...Option) (*ConnectionPool, error) {
	if size <= 0 {
		return nil, fmt.Errorf("connection pool size must be positive, got %d", size)
	}

	// Default configuration
	config := &poolConfig{
		healthCheckInterval: 30 * time.Second,
		autoRepair:          true,
		clientOptions:       []rabbitmq.Option{},
		logger:              rabbitmq.NewNopLogger(), // Default to no-op logger
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	pool := &ConnectionPool{
		connections:     make([]*rabbitmq.Client, size),
		healthyClients:  make([]*rabbitmq.Client, 0, size),
		size:            size,
		clientOptions:   config.clientOptions,
		logger:          config.logger,
		healthInterval:  config.healthCheckInterval,
		stopHealthCheck: make(chan struct{}),
		healthStopped:   make(chan struct{}),
		autoRepair:      config.autoRepair,
		stats:           internalStats{},
	}

	// Create all connections in the pool
	for i := range size {
		connectionName := fmt.Sprintf("pool-connection-%d", i)
		connOpts := append(config.clientOptions, rabbitmq.WithConnectionName(connectionName))

		client, err := rabbitmq.NewClient(connOpts...)
		if err != nil {
			// Close any already created connections
			_ = pool.Close() // Ignore close error when creation fails
			return nil, fmt.Errorf("failed to create connection %d: %w", i, err)
		}
		pool.connections[i] = client
	}

	// Initialize healthy clients list
	pool.mu.Lock()
	pool.updateHealthyClients()
	pool.mu.Unlock()

	// Start health monitoring if enabled
	if pool.healthInterval > 0 {
		// Add a small delay to ensure proper initialization in tests
		go func() {
			time.Sleep(50 * time.Millisecond)
			pool.startHealthMonitoring()
		}()
	}

	pool.logger.Info("Connection pool created successfully",
		"size", size,
		"health_interval", pool.healthInterval,
		"auto_repair", pool.autoRepair)

	return pool, nil
}

// startHealthMonitoring begins periodic health checks
func (p *ConnectionPool) startHealthMonitoring() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.closed) == 1 {
		return
	}

	if p.healthTicker != nil {
		p.healthTicker.Stop()
	}

	p.healthTicker = time.NewTicker(p.healthInterval)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.logger.Error("Health monitoring goroutine panicked",
					"error", fmt.Sprintf("%v", r),
					"stack", fmt.Sprintf("%+v", r))
			}
			// Signal that health monitoring has stopped
			close(p.healthStopped)
		}()

		for {
			select {
			case <-p.stopHealthCheck:
				return
			case _, ok := <-p.healthTicker.C:
				if !ok {
					// Ticker was stopped and channel closed
					return
				}
				// Check if pool is still open before performing health check
				if atomic.LoadInt32(&p.closed) == 1 {
					return
				}

				p.performHealthCheck()
			}
		}
	}()
}

// updateHealthyClients updates the list of healthy clients (call with lock held)
func (p *ConnectionPool) updateHealthyClients() {
	// Check if pool is closed
	if atomic.LoadInt32(&p.closed) == 1 {
		p.healthyClients = nil
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	healthy := make([]*rabbitmq.Client, 0, len(p.connections))

	for _, client := range p.connections {
		if client != nil {
			if err := client.Ping(ctx); err == nil {
				healthy = append(healthy, client)
			}
		}
	}

	p.healthyClients = healthy
}

// performHealthCheck checks all connections and repairs if needed
// This function performs all I/O operations (Ping) WITHOUT holding the main lock,
// only acquiring the lock briefly to swap the healthy clients pointer.
func (p *ConnectionPool) performHealthCheck() {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("performHealthCheck panicked",
				"error", fmt.Sprintf("%v", r))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Step 1: Copy connections list with RLock (fast, no I/O)
	p.mu.RLock()
	if atomic.LoadInt32(&p.closed) == 1 {
		p.mu.RUnlock()
		return
	}
	connections := make([]*rabbitmq.Client, len(p.connections))
	copy(connections, p.connections)
	p.mu.RUnlock()

	p.stats.mu.Lock()
	p.stats.LastHealthCheck = time.Now()
	p.stats.mu.Unlock()

	// Step 2: Perform health checks WITHOUT holding any lock
	// Build the healthy clients list during the health check to avoid double Ping
	healthyCount := 0
	unhealthyCount := 0
	var repairNeeded []int
	healthy := make([]*rabbitmq.Client, 0, len(connections))

	for i, client := range connections {
		if client == nil {
			unhealthyCount++
			repairNeeded = append(repairNeeded, i)
			continue
		}

		// Double check client is not nil and has a valid connection
		func() {
			defer func() {
				if r := recover(); r != nil {
					p.logger.Error("Client ping panicked",
						"connection_index", i,
						"error", fmt.Sprintf("%v", r))
					unhealthyCount++
					if p.autoRepair {
						repairNeeded = append(repairNeeded, i)
					}
				}
			}()

			if err := client.Ping(ctx); err != nil {
				unhealthyCount++
				p.logger.Warn("Connection health check failed",
					"connection_index", i,
					"error", err.Error())

				if p.autoRepair && rabbitmq.IsConnectionError(err) {
					repairNeeded = append(repairNeeded, i)
				}
			} else {
				healthyCount++
				// Add to healthy list - already verified by Ping
				healthy = append(healthy, client)
			}
		}()
	}

	// Step 3: Only acquire lock to swap pointers (nanosecond operation, no I/O)
	p.mu.Lock()
	// Check if pool was closed while we were doing health checks
	if atomic.LoadInt32(&p.closed) == 1 {
		p.mu.Unlock()
		return
	}
	p.healthyClients = healthy
	p.stats.mu.Lock()
	p.stats.HealthyConnections = healthyCount
	p.stats.UnhealthyConnections = unhealthyCount
	p.stats.mu.Unlock()
	p.mu.Unlock()

	// Repair failed connections if enabled
	if p.autoRepair && len(repairNeeded) > 0 {
		p.repairConnections(repairNeeded)
	}
}

// repairConnections attempts to recreate failed connections
// This function rebuilds the healthy clients list directly after repair
// without redundant Ping calls (newly created connections are healthy by definition)
func (p *ConnectionPool) repairConnections(indices []int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.closed) == 1 {
		return
	}

	for _, idx := range indices {
		if idx < 0 || idx >= len(p.connections) {
			continue
		}

		// Close the old connection if it exists
		if old := p.connections[idx]; old != nil {
			_ = old.Close() // Ignore close error when repairing
		}

		// Create a new connection
		connectionName := fmt.Sprintf("pool-connection-%d-repaired", idx)
		connOpts := append(p.clientOptions, rabbitmq.WithConnectionName(connectionName))

		client, err := rabbitmq.NewClient(connOpts...)
		if err != nil {
			p.logger.Error("Failed to repair connection",
				"connection_index", idx,
				"error", err.Error())
			p.connections[idx] = nil
			continue
		}

		p.connections[idx] = client

		p.stats.mu.Lock()
		p.stats.TotalRepairAttempts++
		p.stats.mu.Unlock()

		p.logger.Info("Connection repaired successfully",
			"connection_index", idx)
	}

	// Rebuild healthy clients list directly from connections
	// Newly created connections are healthy by definition (just connected)
	// This avoids redundant Ping calls while holding the lock
	healthy := make([]*rabbitmq.Client, 0, len(p.connections))
	for _, client := range p.connections {
		if client != nil {
			healthy = append(healthy, client)
		}
	}
	p.healthyClients = healthy
}

// Get returns a healthy client from the pool (fast, non-blocking)
func (p *ConnectionPool) Get() (*rabbitmq.Client, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if atomic.LoadInt32(&p.closed) == 1 {
		return nil, fmt.Errorf("connection pool is closed")
	}

	if len(p.healthyClients) == 0 {
		return nil, fmt.Errorf("no healthy connections available")
	}

	// Round-robin selection from healthy clients
	index := atomic.AddUint64(&p.current, 1) % uint64(len(p.healthyClients))
	return p.healthyClients[index], nil
}

// GetClientByIndex returns a specific client by index (useful for sharding)
func (p *ConnectionPool) GetClientByIndex(index int) *rabbitmq.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if atomic.LoadInt32(&p.closed) == 1 || index < 0 || index >= p.size {
		return nil
	}

	return p.connections[index]
}

// Size returns the size of the connection pool
func (p *ConnectionPool) Size() int {
	return p.size
}

// HealthCheck performs health checks on all connections in the pool
func (p *ConnectionPool) HealthCheck(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if atomic.LoadInt32(&p.closed) == 1 {
		return fmt.Errorf("connection pool is closed")
	}

	var errors []error
	for i, client := range p.connections {
		if client == nil {
			errors = append(errors, fmt.Errorf("connection %d is nil", i))
			continue
		}

		if err := client.Ping(ctx); err != nil {
			errors = append(errors, fmt.Errorf("connection %d health check failed: %w", i, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("health check failed for %d connections: %v", len(errors), errors)
	}

	return nil
}

// GetStats returns statistics about the connection pool
func (p *ConnectionPool) GetStats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()

	// Create a copy without the mutex to avoid copying lock value
	stats := Stats{
		Size:                    p.size,
		Closed:                  atomic.LoadInt32(&p.closed) == 1,
		HealthyConnections:      p.stats.HealthyConnections,
		UnhealthyConnections:    p.stats.UnhealthyConnections,
		TotalRepairAttempts:     p.stats.TotalRepairAttempts,
		LastHealthCheck:         p.stats.LastHealthCheck,
		HealthMonitoringEnabled: p.healthTicker != nil,
		AutoRepairEnabled:       p.autoRepair,
	}

	if atomic.LoadInt32(&p.closed) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for i, client := range p.connections {
			if client == nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("connection %d: nil", i))
				continue
			}

			if err := client.Ping(ctx); err != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("connection %d: %v", i, err))
			}
		}
	}

	return stats
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.closed) == 1 {
		return nil
	}

	// Set closed flag first to signal shutdown
	atomic.StoreInt32(&p.closed, 1)

	// Stop health monitoring
	if p.healthTicker != nil {
		p.healthTicker.Stop()
		// Don't set to nil here - let the goroutine handle cleanup
	}

	// Signal health monitoring to stop (only if channel isn't closed)
	select {
	case p.stopHealthCheck <- struct{}{}:
		// Wait for health monitoring to stop completely
		<-p.healthStopped
	default:
		// Channel might be closed or full, that's okay
	}

	var errors []error
	for i, client := range p.connections {
		if client != nil {
			if err := client.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close connection %d: %w", i, err))
			}
		}
	}

	p.logger.Info("Connection pool closed",
		"size", p.size,
		"errors", len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("errors closing connections: %v", errors)
	}

	return nil
}

// Interface compliance methods for rabbitmq.ConnectionPooler

// Stats converts internal stats to the interface PoolStats type
func (p *ConnectionPool) Stats() rabbitmq.PoolStats {
	stats := p.GetStats()
	return rabbitmq.PoolStats{
		TotalConnections:   int64(stats.Size),
		HealthyConnections: int64(stats.HealthyConnections),
		FailedConnections:  int64(stats.UnhealthyConnections),
		RepairAttempts:     stats.TotalRepairAttempts,
		LastHealthCheck:    stats.LastHealthCheck,
	}
}

// HealthyCount returns the number of healthy connections
func (p *ConnectionPool) HealthyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.healthyClients)
}

// Ensure ConnectionPool implements the interface at compile time
var _ rabbitmq.ConnectionPooler = (*ConnectionPool)(nil)
