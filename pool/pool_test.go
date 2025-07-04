package pool

import (
	"context"
	"testing"
	"time"

	"github.com/cloudresty/go-rabbitmq"
)

func TestNew(t *testing.T) {
	t.Run("valid pool size", func(t *testing.T) {
		pool, err := New(3, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer pool.Close()

		if pool.Size() != 3 {
			t.Errorf("expected pool size 3, got %d", pool.Size())
		}
	})

	t.Run("zero pool size should fail", func(t *testing.T) {
		_, err := New(0)
		if err == nil {
			t.Error("expected error for zero pool size")
		}
	})

	t.Run("negative pool size should fail", func(t *testing.T) {
		_, err := New(-1)
		if err == nil {
			t.Error("expected error for negative pool size")
		}
	})

	t.Run("with health check option", func(t *testing.T) {
		pool, err := New(2,
			WithHealthCheck(10*time.Second),
			WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer pool.Close()

		// Wait a moment for health monitoring to start
		time.Sleep(100 * time.Millisecond)

		stats := pool.GetStats()
		if !stats.HealthMonitoringEnabled {
			t.Error("expected health monitoring to be enabled")
		}
	})

	t.Run("with auto repair option", func(t *testing.T) {
		pool, err := New(2,
			WithAutoRepair(true),
			WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer pool.Close()

		stats := pool.GetStats()
		if !stats.AutoRepairEnabled {
			t.Error("expected auto repair to be enabled")
		}
	})

	t.Run("disabled health monitoring", func(t *testing.T) {
		pool, err := New(2,
			WithHealthCheck(0),
			WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer pool.Close()

		stats := pool.GetStats()
		if stats.HealthMonitoringEnabled {
			t.Error("expected health monitoring to be disabled")
		}
	})

	t.Run("with multiple client options", func(t *testing.T) {
		pool, err := New(2, WithClientOptions(
			rabbitmq.WithHosts("localhost:5672"),
			rabbitmq.WithConnectionName("test-pool"),
		))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer pool.Close()

		if pool.Size() != 2 {
			t.Errorf("expected pool size 2, got %d", pool.Size())
		}
	})
}

func TestGet(t *testing.T) {
	t.Run("get client", func(t *testing.T) {
		pool, err := New(2, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer pool.Close()

		client, err := pool.Get()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if client == nil {
			t.Error("expected non-nil client")
		}
	})

	t.Run("round-robin behavior", func(t *testing.T) {
		pool, err := New(2, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer pool.Close()

		// Get multiple clients and verify they're returned
		clients := make([]*rabbitmq.Client, 10)
		for i := 0; i < 10; i++ {
			client, err := pool.Get()
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			clients[i] = client
			if clients[i] == nil {
				t.Errorf("expected non-nil client at index %d", i)
			}
		}
	})

	t.Run("closed pool should return error", func(t *testing.T) {
		pool, err := New(2, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}

		pool.Close()

		client, err := pool.Get()
		if err == nil {
			t.Error("expected error from closed pool")
		}
		if client != nil {
			t.Error("expected nil client from closed pool")
		}
	})
}

func TestGetClientByIndex(t *testing.T) {
	pool, err := New(3, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
	if err != nil {
		t.Skip("RabbitMQ not available for testing")
	}
	defer pool.Close()

	// Test valid indices
	for i := range 3 {
		client := pool.GetClientByIndex(i)
		if client == nil {
			t.Errorf("expected non-nil client for index %d", i)
		}
	}

	// Test invalid indices
	if client := pool.GetClientByIndex(-1); client != nil {
		t.Error("expected nil client for negative index")
	}

	if client := pool.GetClientByIndex(3); client != nil {
		t.Error("expected nil client for out-of-bounds index")
	}
}

func TestHealthCheck(t *testing.T) {
	pool, err := New(2, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
	if err != nil {
		t.Skip("RabbitMQ not available for testing")
	}
	defer pool.Close()

	ctx := context.Background()
	if err := pool.HealthCheck(ctx); err != nil {
		t.Errorf("unexpected health check error: %v", err)
	}
}

func TestGetStats(t *testing.T) {
	pool, err := New(2,
		WithHealthCheck(10*time.Second),
		WithAutoRepair(true),
		WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
	if err != nil {
		t.Skip("RabbitMQ not available for testing")
	}
	defer pool.Close()

	// Wait a moment for health monitoring to start
	time.Sleep(100 * time.Millisecond)

	stats := pool.GetStats()

	if stats.Size != 2 {
		t.Errorf("expected size 2, got %d", stats.Size)
	}

	if stats.Closed {
		t.Error("expected pool to not be closed")
	}

	if !stats.HealthMonitoringEnabled {
		t.Error("expected health monitoring to be enabled")
	}

	if !stats.AutoRepairEnabled {
		t.Error("expected auto repair to be enabled")
	}
}

func TestClose(t *testing.T) {
	pool, err := New(2, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
	if err != nil {
		t.Skip("RabbitMQ not available for testing")
	}

	if err := pool.Close(); err != nil {
		t.Errorf("unexpected error closing pool: %v", err)
	}

	// Closing again should not error
	if err := pool.Close(); err != nil {
		t.Errorf("unexpected error closing pool again: %v", err)
	}

	stats := pool.GetStats()
	if !stats.Closed {
		t.Error("expected pool to be closed")
	}
}

func TestConcurrentAccess(t *testing.T) {
	pool, err := New(5, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
	if err != nil {
		t.Skip("RabbitMQ not available for testing")
	}
	defer pool.Close()

	// Test concurrent access to Get()
	done := make(chan bool, 10)
	for range 10 {
		go func() {
			defer func() { done <- true }()
			for range 100 {
				client, err := pool.Get()
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if client == nil {
					t.Error("expected non-nil client")
					return
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	for range 10 {
		<-done
	}
}

// Benchmarks
func BenchmarkGet(b *testing.B) {
	pool, err := New(5, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
	if err != nil {
		b.Skip("RabbitMQ not available for benchmarking")
	}
	defer pool.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client, _ := pool.Get()
			_ = client
		}
	})
}

func BenchmarkGetClientByIndex(b *testing.B) {
	pool, err := New(5, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
	if err != nil {
		b.Skip("RabbitMQ not available for benchmarking")
	}
	defer pool.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client := pool.GetClientByIndex(0)
			_ = client
		}
	})
}

func BenchmarkGetStats(b *testing.B) {
	pool, err := New(5, WithClientOptions(rabbitmq.WithHosts("localhost:5672")))
	if err != nil {
		b.Skip("RabbitMQ not available for benchmarking")
	}
	defer pool.Close()

	for b.Loop() {
		stats := pool.GetStats()
		_ = stats
	}
}
