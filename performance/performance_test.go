package performance

import (
	"testing"
	"time"
)

func TestNewMonitor(t *testing.T) {
	monitor := NewMonitor()

	if monitor == nil {
		t.Error("expected non-nil monitor")
	}

	stats := monitor.GetStats()
	if stats.ConnectionsTotal != 0 {
		t.Error("expected initial connections total to be 0")
	}

	if stats.IsConnected {
		t.Error("expected initial connection state to be disconnected")
	}
}

func TestRecordConnection(t *testing.T) {
	monitor := NewMonitor()

	// Test successful connection
	monitor.RecordConnection(true)
	stats := monitor.GetStats()

	if stats.ConnectionsTotal != 1 {
		t.Errorf("expected connections total 1, got %d", stats.ConnectionsTotal)
	}

	if !stats.IsConnected {
		t.Error("expected connection state to be connected")
	}

	if stats.LastConnectionTime.IsZero() {
		t.Error("expected last connection time to be set")
	}

	// Test failed connection
	monitor.RecordConnection(false)
	stats = monitor.GetStats()

	if stats.ConnectionsTotal != 2 {
		t.Errorf("expected connections total 2, got %d", stats.ConnectionsTotal)
	}

	if stats.IsConnected {
		t.Error("expected connection state to be disconnected after failed connection")
	}
}

func TestRecordReconnection(t *testing.T) {
	monitor := NewMonitor()

	monitor.RecordReconnection()
	stats := monitor.GetStats()

	if stats.ReconnectionsTotal != 1 {
		t.Errorf("expected reconnections total 1, got %d", stats.ReconnectionsTotal)
	}

	if stats.LastReconnectionTime.IsZero() {
		t.Error("expected last reconnection time to be set")
	}
}

func TestRecordPublish(t *testing.T) {
	monitor := NewMonitor()

	// Test successful publish
	duration := 10 * time.Millisecond
	monitor.RecordPublish(true, duration)

	stats := monitor.GetStats()
	if stats.PublishesTotal != 1 {
		t.Errorf("expected publishes total 1, got %d", stats.PublishesTotal)
	}

	if stats.PublishSuccessTotal != 1 {
		t.Errorf("expected publish success total 1, got %d", stats.PublishSuccessTotal)
	}

	if stats.PublishErrorsTotal != 0 {
		t.Errorf("expected publish errors total 0, got %d", stats.PublishErrorsTotal)
	}

	if stats.PublishSuccessRate != 1.0 {
		t.Errorf("expected publish success rate 1.0, got %f", stats.PublishSuccessRate)
	}

	// Test failed publish
	monitor.RecordPublish(false, duration)

	stats = monitor.GetStats()
	if stats.PublishesTotal != 2 {
		t.Errorf("expected publishes total 2, got %d", stats.PublishesTotal)
	}

	if stats.PublishSuccessTotal != 1 {
		t.Errorf("expected publish success total 1, got %d", stats.PublishSuccessTotal)
	}

	if stats.PublishErrorsTotal != 1 {
		t.Errorf("expected publish errors total 1, got %d", stats.PublishErrorsTotal)
	}

	if stats.PublishSuccessRate != 0.5 {
		t.Errorf("expected publish success rate 0.5, got %f", stats.PublishSuccessRate)
	}
}

func TestRecordConsume(t *testing.T) {
	monitor := NewMonitor()

	// Test successful consume
	duration := 5 * time.Millisecond
	monitor.RecordConsume(true, duration)

	stats := monitor.GetStats()
	if stats.ConsumesTotal != 1 {
		t.Errorf("expected consumes total 1, got %d", stats.ConsumesTotal)
	}

	if stats.ConsumeSuccessTotal != 1 {
		t.Errorf("expected consume success total 1, got %d", stats.ConsumeSuccessTotal)
	}

	if stats.ConsumeErrorsTotal != 0 {
		t.Errorf("expected consume errors total 0, got %d", stats.ConsumeErrorsTotal)
	}

	if stats.ConsumeSuccessRate != 1.0 {
		t.Errorf("expected consume success rate 1.0, got %f", stats.ConsumeSuccessRate)
	}

	// Test failed consume
	monitor.RecordConsume(false, duration)

	stats = monitor.GetStats()
	if stats.ConsumesTotal != 2 {
		t.Errorf("expected consumes total 2, got %d", stats.ConsumesTotal)
	}

	if stats.ConsumeSuccessTotal != 1 {
		t.Errorf("expected consume success total 1, got %d", stats.ConsumeSuccessTotal)
	}

	if stats.ConsumeErrorsTotal != 1 {
		t.Errorf("expected consume errors total 1, got %d", stats.ConsumeErrorsTotal)
	}

	if stats.ConsumeSuccessRate != 0.5 {
		t.Errorf("expected consume success rate 0.5, got %f", stats.ConsumeSuccessRate)
	}
}

func TestReset(t *testing.T) {
	monitor := NewMonitor()

	// Record some operations
	monitor.RecordConnection(true)
	monitor.RecordPublish(true, 10*time.Millisecond)
	monitor.RecordConsume(true, 5*time.Millisecond)

	// Verify operations were recorded
	stats := monitor.GetStats()
	if stats.ConnectionsTotal == 0 || stats.PublishesTotal == 0 || stats.ConsumesTotal == 0 {
		t.Error("expected some operations to be recorded before reset")
	}

	// Reset and verify
	monitor.Reset()
	stats = monitor.GetStats()

	if stats.ConnectionsTotal != 0 {
		t.Errorf("expected connections total 0 after reset, got %d", stats.ConnectionsTotal)
	}

	if stats.PublishesTotal != 0 {
		t.Errorf("expected publishes total 0 after reset, got %d", stats.PublishesTotal)
	}

	if stats.ConsumesTotal != 0 {
		t.Errorf("expected consumes total 0 after reset, got %d", stats.ConsumesTotal)
	}
}

func TestNewRateTracker(t *testing.T) {
	tracker := NewRateTracker(time.Minute)

	if tracker == nil {
		t.Error("expected non-nil rate tracker")
	}

	rate := tracker.Rate()
	if rate != 0 {
		t.Errorf("expected initial rate 0, got %f", rate)
	}
}

func TestRateTrackerRecord(t *testing.T) {
	tracker := NewRateTracker(time.Second)

	// Record some events
	for range 5 {
		tracker.Record()
	}

	rate := tracker.Rate()
	if rate == 0 {
		t.Error("expected non-zero rate after recording events")
	}

	// Rate should be approximately 5 events per second
	if rate < 4 || rate > 6 {
		t.Errorf("expected rate around 5, got %f", rate)
	}
}

func TestRateTrackerWindow(t *testing.T) {
	tracker := NewRateTracker(100 * time.Millisecond)

	// Record events
	tracker.Record()
	tracker.Record()

	rate1 := tracker.Rate()

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	rate2 := tracker.Rate()

	// Rate should decrease as events age out
	if rate2 >= rate1 {
		t.Errorf("expected rate to decrease over time, got %f then %f", rate1, rate2)
	}
}

func TestMonitorHelperMethods(t *testing.T) {
	monitor := NewMonitor()

	// Test IsConnected
	if monitor.IsConnected() {
		t.Error("expected initial connection state to be false")
	}

	monitor.RecordConnection(true)
	if !monitor.IsConnected() {
		t.Error("expected connection state to be true after successful connection")
	}

	// Test GetTotalOperations
	if monitor.GetTotalOperations() != 0 {
		t.Error("expected initial total operations to be 0")
	}

	monitor.RecordPublish(true, time.Millisecond)
	monitor.RecordConsume(true, time.Millisecond)

	if monitor.GetTotalOperations() != 2 {
		t.Errorf("expected total operations 2, got %d", monitor.GetTotalOperations())
	}

	// Test GetSuccessRate
	rate := monitor.GetSuccessRate()
	if rate != 1.0 {
		t.Errorf("expected success rate 1.0, got %f", rate)
	}

	// Add a failed operation
	monitor.RecordPublish(false, time.Millisecond)

	rate = monitor.GetSuccessRate()
	expectedRate := 2.0 / 3.0 // 2 successes out of 3 total
	if rate < expectedRate-0.01 || rate > expectedRate+0.01 {
		t.Errorf("expected success rate around %f, got %f", expectedRate, rate)
	}
}

func TestLatencyPercentiles(t *testing.T) {
	monitor := NewMonitor()

	// Record some publish operations with different latencies
	latencies := []time.Duration{
		1 * time.Millisecond,
		2 * time.Millisecond,
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
	}

	for _, latency := range latencies {
		monitor.RecordPublish(true, latency)
	}

	stats := monitor.GetStats()

	// With our simplified percentile calculation, we should get some values
	if stats.PublishLatencyP50 == 0 {
		t.Error("expected non-zero P50 latency")
	}

	// P50 should be less than or equal to P95
	if stats.PublishLatencyP50 > stats.PublishLatencyP95 && stats.PublishLatencyP95 != 0 {
		t.Error("expected P50 <= P95")
	}

	// P95 should be less than or equal to P99
	if stats.PublishLatencyP95 > stats.PublishLatencyP99 && stats.PublishLatencyP99 != 0 {
		t.Error("expected P95 <= P99")
	}
}

// Benchmark tests
func BenchmarkRecordPublish(b *testing.B) {
	monitor := NewMonitor()
	duration := 10 * time.Millisecond

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			monitor.RecordPublish(true, duration)
		}
	})
}

func BenchmarkGetStats(b *testing.B) {
	monitor := NewMonitor()

	// Pre-populate with some data
	for i := range 100 {
		monitor.RecordPublish(true, time.Duration(i)*time.Microsecond)
		monitor.RecordConsume(true, time.Duration(i)*time.Microsecond)
	}

	for b.Loop() {
		monitor.GetStats()
	}
}

func BenchmarkRateTrackerRecord(b *testing.B) {
	tracker := NewRateTracker(time.Minute)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tracker.Record()
		}
	})
}

func BenchmarkRateTrackerRate(b *testing.B) {
	tracker := NewRateTracker(time.Minute)

	// Pre-populate with some events
	for range 100 {
		tracker.Record()
	}

	for b.Loop() {
		tracker.Rate()
	}
}
