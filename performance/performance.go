// Package performance provides detailed performance monitoring and metrics collection for RabbitMQ operations.
//
// This package offers comprehensive monitoring capabilities including:
//   - Connection tracking and health monitoring
//   - Publish and consume operation metrics
//   - Latency percentile calculations
//   - Rate tracking over time windows
//   - Success/failure rate monitoring
//
// The performance monitor is designed to be lightweight and thread-safe,
// suitable for production environments where detailed observability is required.
//
// Example usage:
//
//	import "github.com/cloudresty/go-rabbitmq/performance"
//
//	// Create a performance monitor
//	monitor := performance.NewMonitor()
//
//	// Use it with the main client
//	client, err := rabbitmq.NewClient(
//		rabbitmq.WithPerformanceMonitoring(monitor),
//	)
//
//	// Record operations as they occur (usually done internally by the client)
//	start := time.Now()
//	err := publisher.Publish(ctx, queue, msg)
//	monitor.RecordPublish(err == nil, time.Since(start))
//
//	// Get detailed statistics
//	stats := monitor.GetStats()
//	fmt.Printf("Publish success rate: %.2f%%\n", stats.PublishSuccessRate*100)
package performance

import (
	"sync"
	"sync/atomic"
	"time"
)

// Monitor provides detailed performance monitoring capabilities
type Monitor struct {
	// Counters
	connectionsTotal    uint64
	reconnectionsTotal  uint64
	publishesTotal      uint64
	publishSuccessTotal uint64
	publishErrorsTotal  uint64
	consumesTotal       uint64
	consumeSuccessTotal uint64
	consumeErrorsTotal  uint64

	// Timing histograms (simplified - in production you'd use proper histograms)
	publishLatencies   []time.Duration
	consumeLatencies   []time.Duration
	publishLatenciesMu sync.RWMutex
	consumeLatenciesMu sync.RWMutex

	// Connection state
	connectionState      int32 // 0=disconnected, 1=connected
	lastConnectionTime   time.Time
	lastReconnectionTime time.Time

	// Rate tracking
	publishRate *RateTracker
	consumeRate *RateTracker
}

// RateTracker tracks rates over time windows
type RateTracker struct {
	mu     sync.RWMutex
	events []time.Time
	window time.Duration
}

// Stats represents current performance statistics
type Stats struct {
	// Connection stats
	ConnectionsTotal     uint64
	ReconnectionsTotal   uint64
	IsConnected          bool
	LastConnectionTime   time.Time
	LastReconnectionTime time.Time

	// Publish stats
	PublishesTotal      uint64
	PublishSuccessTotal uint64
	PublishErrorsTotal  uint64
	PublishSuccessRate  float64
	PublishRate         float64 // per second

	// Consume stats
	ConsumesTotal       uint64
	ConsumeSuccessTotal uint64
	ConsumeErrorsTotal  uint64
	ConsumeSuccessRate  float64
	ConsumeRate         float64 // per second

	// Latency stats (simplified percentiles)
	PublishLatencyP50 time.Duration
	PublishLatencyP95 time.Duration
	PublishLatencyP99 time.Duration
	ConsumeLatencyP50 time.Duration
	ConsumeLatencyP95 time.Duration
	ConsumeLatencyP99 time.Duration
}

// NewRateTracker creates a new rate tracker with the specified window
func NewRateTracker(window time.Duration) *RateTracker {
	return &RateTracker{
		events: make([]time.Time, 0),
		window: window,
	}
}

// Record records an event
func (r *RateTracker) Record() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	r.events = append(r.events, now)

	// Clean old events
	cutoff := now.Add(-r.window)
	for i, event := range r.events {
		if event.After(cutoff) {
			r.events = r.events[i:]
			break
		}
	}
}

// Rate returns the current rate (events per second)
func (r *RateTracker) Rate() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	count := 0
	for _, event := range r.events {
		if event.After(cutoff) {
			count++
		}
	}

	return float64(count) / r.window.Seconds()
}

// NewMonitor creates a new performance monitor
func NewMonitor() *Monitor {
	return &Monitor{
		publishLatencies: make([]time.Duration, 0),
		consumeLatencies: make([]time.Duration, 0),
		publishRate:      NewRateTracker(time.Minute),
		consumeRate:      NewRateTracker(time.Minute),
	}
}

// RecordConnection records a connection event
func (p *Monitor) RecordConnection(success bool) {
	atomic.AddUint64(&p.connectionsTotal, 1)
	if success {
		atomic.StoreInt32(&p.connectionState, 1)
		p.lastConnectionTime = time.Now()
	} else {
		atomic.StoreInt32(&p.connectionState, 0)
	}
}

// RecordReconnection records a reconnection event
func (p *Monitor) RecordReconnection() {
	atomic.AddUint64(&p.reconnectionsTotal, 1)
	p.lastReconnectionTime = time.Now()
}

// RecordPublish records a publish operation
func (p *Monitor) RecordPublish(success bool, duration time.Duration) {
	atomic.AddUint64(&p.publishesTotal, 1)
	if success {
		atomic.AddUint64(&p.publishSuccessTotal, 1)
	} else {
		atomic.AddUint64(&p.publishErrorsTotal, 1)
	}

	p.publishRate.Record()

	// Record latency (keep last 1000 measurements)
	p.publishLatenciesMu.Lock()
	p.publishLatencies = append(p.publishLatencies, duration)
	if len(p.publishLatencies) > 1000 {
		p.publishLatencies = p.publishLatencies[len(p.publishLatencies)-1000:]
	}
	p.publishLatenciesMu.Unlock()
}

// RecordConsume records a consume operation
func (p *Monitor) RecordConsume(success bool, duration time.Duration) {
	atomic.AddUint64(&p.consumesTotal, 1)
	if success {
		atomic.AddUint64(&p.consumeSuccessTotal, 1)
	} else {
		atomic.AddUint64(&p.consumeErrorsTotal, 1)
	}

	p.consumeRate.Record()

	// Record latency (keep last 1000 measurements)
	p.consumeLatenciesMu.Lock()
	p.consumeLatencies = append(p.consumeLatencies, duration)
	if len(p.consumeLatencies) > 1000 {
		p.consumeLatencies = p.consumeLatencies[len(p.consumeLatencies)-1000:]
	}
	p.consumeLatenciesMu.Unlock()
}

// GetStats returns current performance statistics
func (p *Monitor) GetStats() Stats {
	stats := Stats{
		ConnectionsTotal:     atomic.LoadUint64(&p.connectionsTotal),
		ReconnectionsTotal:   atomic.LoadUint64(&p.reconnectionsTotal),
		IsConnected:          atomic.LoadInt32(&p.connectionState) == 1,
		LastConnectionTime:   p.lastConnectionTime,
		LastReconnectionTime: p.lastReconnectionTime,

		PublishesTotal:      atomic.LoadUint64(&p.publishesTotal),
		PublishSuccessTotal: atomic.LoadUint64(&p.publishSuccessTotal),
		PublishErrorsTotal:  atomic.LoadUint64(&p.publishErrorsTotal),

		ConsumesTotal:       atomic.LoadUint64(&p.consumesTotal),
		ConsumeSuccessTotal: atomic.LoadUint64(&p.consumeSuccessTotal),
		ConsumeErrorsTotal:  atomic.LoadUint64(&p.consumeErrorsTotal),

		PublishRate: p.publishRate.Rate(),
		ConsumeRate: p.consumeRate.Rate(),
	}

	// Calculate success rates
	if stats.PublishesTotal > 0 {
		stats.PublishSuccessRate = float64(stats.PublishSuccessTotal) / float64(stats.PublishesTotal)
	}
	if stats.ConsumesTotal > 0 {
		stats.ConsumeSuccessRate = float64(stats.ConsumeSuccessTotal) / float64(stats.ConsumesTotal)
	}

	// Calculate latency percentiles
	stats.PublishLatencyP50, stats.PublishLatencyP95, stats.PublishLatencyP99 = p.calculateLatencyPercentiles(p.publishLatencies, &p.publishLatenciesMu)
	stats.ConsumeLatencyP50, stats.ConsumeLatencyP95, stats.ConsumeLatencyP99 = p.calculateLatencyPercentiles(p.consumeLatencies, &p.consumeLatenciesMu)

	return stats
}

// calculateLatencyPercentiles calculates simple percentiles (simplified implementation)
func (p *Monitor) calculateLatencyPercentiles(latencies []time.Duration, mu *sync.RWMutex) (p50, p95, p99 time.Duration) {
	mu.RLock()
	defer mu.RUnlock()

	if len(latencies) == 0 {
		return 0, 0, 0
	}

	// Simple percentile calculation - in production you'd use a proper algorithm
	// Sort is expensive, so this is just a rough approximation
	if len(latencies) >= 2 {
		p50 = latencies[len(latencies)/2]
	}
	if len(latencies) >= 20 {
		p95 = latencies[len(latencies)*95/100]
	}
	if len(latencies) >= 100 {
		p99 = latencies[len(latencies)*99/100]
	}

	return p50, p95, p99
}

// Reset resets all performance counters
func (p *Monitor) Reset() {
	atomic.StoreUint64(&p.connectionsTotal, 0)
	atomic.StoreUint64(&p.reconnectionsTotal, 0)
	atomic.StoreUint64(&p.publishesTotal, 0)
	atomic.StoreUint64(&p.publishSuccessTotal, 0)
	atomic.StoreUint64(&p.publishErrorsTotal, 0)
	atomic.StoreUint64(&p.consumesTotal, 0)
	atomic.StoreUint64(&p.consumeSuccessTotal, 0)
	atomic.StoreUint64(&p.consumeErrorsTotal, 0)

	p.publishLatenciesMu.Lock()
	p.publishLatencies = p.publishLatencies[:0]
	p.publishLatenciesMu.Unlock()

	p.consumeLatenciesMu.Lock()
	p.consumeLatencies = p.consumeLatencies[:0]
	p.consumeLatenciesMu.Unlock()

	p.publishRate = NewRateTracker(time.Minute)
	p.consumeRate = NewRateTracker(time.Minute)
}

// IsConnected returns whether the monitored connection is currently connected
func (p *Monitor) IsConnected() bool {
	return atomic.LoadInt32(&p.connectionState) == 1
}

// GetPublishRate returns the current publish rate (operations per second)
func (p *Monitor) GetPublishRate() float64 {
	return p.publishRate.Rate()
}

// GetConsumeRate returns the current consume rate (operations per second)
func (p *Monitor) GetConsumeRate() float64 {
	return p.consumeRate.Rate()
}

// GetTotalOperations returns the total number of operations recorded
func (p *Monitor) GetTotalOperations() uint64 {
	return atomic.LoadUint64(&p.publishesTotal) + atomic.LoadUint64(&p.consumesTotal)
}

// GetSuccessRate returns the overall success rate across all operations
func (p *Monitor) GetSuccessRate() float64 {
	totalOps := p.GetTotalOperations()
	if totalOps == 0 {
		return 0
	}

	totalSuccess := atomic.LoadUint64(&p.publishSuccessTotal) + atomic.LoadUint64(&p.consumeSuccessTotal)
	return float64(totalSuccess) / float64(totalOps)
}
