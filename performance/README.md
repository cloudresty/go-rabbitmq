# Performance Monitoring Package

[Home](../README.md) &nbsp;/&nbsp; Performance Monitoring Package

&nbsp;

The `performance` package provides comprehensive performance monitoring and metrics collection for RabbitMQ operations. This package is designed to give you deep observability into your RabbitMQ application's behavior and performance characteristics.

&nbsp;

## Features

- **Connection Tracking** - Monitor connection establishment, failures, and reconnections
- **Operation Metrics** - Track publish and consume operations with success/failure rates
- **Latency Monitoring** - Record and calculate latency percentiles (P50, P95, P99)
- **Rate Tracking** - Monitor operations per second over configurable time windows
- **Thread-Safe** - All operations are safe for concurrent use
- **Lightweight** - Minimal overhead suitable for production environments
- **Comprehensive Stats** - Detailed statistics and metrics export

🔝 [back to top](#performance-monitoring-package)

&nbsp;

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/performance"
)

func main() {
    // Create a performance monitor
    monitor := performance.NewMonitor()

    // Create a client (assuming you have connection setup)
    client, err := rabbitmq.NewClient(
        rabbitmq.WithHosts("localhost:5672"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Record connection success
    monitor.RecordConnection(true)

    // Create a publisher
    publisher, err := client.NewPublisher()
    if err != nil {
        log.Fatal(err)
    }
    defer publisher.Close()

    // Publish with performance monitoring
    for i := 0; i < 100; i++ {
        start := time.Now()

        err := publisher.Publish(context.Background(), "test.queue", rabbitmq.Publishing{
            Body: []byte(fmt.Sprintf("Message %d", i)),
        })

        // Record the operation
        duration := time.Since(start)
        monitor.RecordPublish(err == nil, duration)

        if err != nil {
            log.Printf("Publish failed: %v", err)
        }
    }

    // Get and display statistics
    stats := monitor.GetStats()
    printStats(stats)
}

func printStats(stats performance.Stats) {
    fmt.Printf("=== Performance Statistics ===\n")
    fmt.Printf("Connections: %d total, %d reconnections\n",
        stats.ConnectionsTotal, stats.ReconnectionsTotal)
    fmt.Printf("Connected: %v\n", stats.IsConnected)

    fmt.Printf("\nPublish Operations:\n")
    fmt.Printf("  Total: %d\n", stats.PublishesTotal)
    fmt.Printf("  Success: %d (%.2f%%)\n",
        stats.PublishSuccessTotal, stats.PublishSuccessRate*100)
    fmt.Printf("  Errors: %d\n", stats.PublishErrorsTotal)
    fmt.Printf("  Rate: %.2f ops/sec\n", stats.PublishRate)

    fmt.Printf("\nLatency Percentiles:\n")
    fmt.Printf("  P50: %v\n", stats.PublishLatencyP50)
    fmt.Printf("  P95: %v\n", stats.PublishLatencyP95)
    fmt.Printf("  P99: %v\n", stats.PublishLatencyP99)
}
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

### Consumer Monitoring

```go
func monitorConsumer(monitor *performance.Monitor, consumer *rabbitmq.Consumer) {
    for {
        start := time.Now()

        // Simulate consume operation
        delivery, err := consumer.Receive(context.Background())
        duration := time.Since(start)

        if err != nil {
            monitor.RecordConsume(false, duration)
            log.Printf("Consume error: %v", err)
            continue
        }

        // Process the message
        processMessage(delivery)

        // Record successful consume
        monitor.RecordConsume(true, duration)

        // Acknowledge the message
        delivery.Ack(false)
    }
}
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

## Rate Tracking

The performance monitor includes rate tracking functionality for monitoring operations per second:

```go
monitor := performance.NewMonitor()

// Record some operations
for i := 0; i < 50; i++ {
    monitor.RecordPublish(true, time.Millisecond)
    time.Sleep(20 * time.Millisecond) // Simulate ~50 ops/sec
}

// Get current rates
publishRate := monitor.GetPublishRate()
fmt.Printf("Current publish rate: %.2f ops/sec\n", publishRate)

// Get comprehensive stats
stats := monitor.GetStats()
fmt.Printf("Publish rate from stats: %.2f ops/sec\n", stats.PublishRate)
fmt.Printf("Consume rate: %.2f ops/sec\n", stats.ConsumeRate)
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

### Custom Rate Tracker

You can also use the rate tracker independently:

```go
// Create a rate tracker with 30-second window
tracker := performance.NewRateTracker(30 * time.Second)

// Record events
for i := 0; i < 10; i++ {
    tracker.Record()
    time.Sleep(100 * time.Millisecond)
}

// Get the current rate
rate := tracker.Rate()
fmt.Printf("Rate: %.2f events/sec\n", rate)
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

## Statistics and Metrics

### Connection Metrics

```go
stats := monitor.GetStats()

fmt.Printf("Connection Status:\n")
fmt.Printf("  Is Connected: %v\n", stats.IsConnected)
fmt.Printf("  Total Connections: %d\n", stats.ConnectionsTotal)
fmt.Printf("  Reconnections: %d\n", stats.ReconnectionsTotal)
fmt.Printf("  Last Connection: %v\n", stats.LastConnectionTime)
fmt.Printf("  Last Reconnection: %v\n", stats.LastReconnectionTime)
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

### Operation Metrics

```go
stats := monitor.GetStats()

fmt.Printf("Publish Metrics:\n")
fmt.Printf("  Total Operations: %d\n", stats.PublishesTotal)
fmt.Printf("  Successful: %d\n", stats.PublishSuccessTotal)
fmt.Printf("  Failed: %d\n", stats.PublishErrorsTotal)
fmt.Printf("  Success Rate: %.2f%%\n", stats.PublishSuccessRate*100)
fmt.Printf("  Operations/sec: %.2f\n", stats.PublishRate)

fmt.Printf("Consume Metrics:\n")
fmt.Printf("  Total Operations: %d\n", stats.ConsumesTotal)
fmt.Printf("  Successful: %d\n", stats.ConsumeSuccessTotal)
fmt.Printf("  Failed: %d\n", stats.ConsumeErrorsTotal)
fmt.Printf("  Success Rate: %.2f%%\n", stats.ConsumeSuccessRate*100)
fmt.Printf("  Operations/sec: %.2f\n", stats.ConsumeRate)
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

### Latency Analysis

```go
stats := monitor.GetStats()

fmt.Printf("Publish Latency Percentiles:\n")
fmt.Printf("  P50 (median): %v\n", stats.PublishLatencyP50)
fmt.Printf("  P95: %v\n", stats.PublishLatencyP95)
fmt.Printf("  P99: %v\n", stats.PublishLatencyP99)

fmt.Printf("Consume Latency Percentiles:\n")
fmt.Printf("  P50 (median): %v\n", stats.ConsumeLatencyP50)
fmt.Printf("  P95: %v\n", stats.ConsumeLatencyP95)
fmt.Printf("  P99: %v\n", stats.ConsumeLatencyP99)
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

## Integration with Monitoring Systems

### Prometheus Integration

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    publishTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rabbitmq_publish_total",
            Help: "Total number of publish operations",
        },
        []string{"status"},
    )

    publishLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "rabbitmq_publish_duration_seconds",
            Help: "Publish operation duration",
        },
        []string{},
    )
)

func recordToPrometheus(monitor *performance.Monitor) {
    stats := monitor.GetStats()

    // Update Prometheus metrics
    publishTotal.WithLabelValues("success").Add(float64(stats.PublishSuccessTotal))
    publishTotal.WithLabelValues("error").Add(float64(stats.PublishErrorsTotal))

    // Record latencies (simplified)
    if stats.PublishLatencyP50 > 0 {
        publishLatency.WithLabelValues().Observe(stats.PublishLatencyP50.Seconds())
    }
}
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

### Periodic Stats Export

```go
func startStatsExporter(monitor *performance.Monitor, interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            stats := monitor.GetStats()

            // Export to your monitoring system
            exportToInfluxDB(stats)
            exportToDatadog(stats)
            exportToCloudWatch(stats)
        }
    }()
}

func exportToInfluxDB(stats performance.Stats) {
    // Implementation depends on your InfluxDB client
    // Example structure:
    point := map[string]interface{}{
        "connections_total":      stats.ConnectionsTotal,
        "publish_success_rate":   stats.PublishSuccessRate,
        "publish_rate":          stats.PublishRate,
        "publish_latency_p50":   stats.PublishLatencyP50.Nanoseconds(),
        "publish_latency_p95":   stats.PublishLatencyP95.Nanoseconds(),
        "consume_rate":          stats.ConsumeRate,
        "is_connected":          stats.IsConnected,
    }
    // Write point to InfluxDB...
}
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

## Advanced Usage

### Custom Monitoring Wrapper

```go
type MonitoredPublisher struct {
    publisher *rabbitmq.Publisher
    monitor   *performance.Monitor
}

func NewMonitoredPublisher(publisher *rabbitmq.Publisher, monitor *performance.Monitor) *MonitoredPublisher {
    return &MonitoredPublisher{
        publisher: publisher,
        monitor:   monitor,
    }
}

func (mp *MonitoredPublisher) Publish(ctx context.Context, queue string, msg rabbitmq.Publishing) error {
    start := time.Now()
    err := mp.publisher.Publish(ctx, queue, msg)
    duration := time.Since(start)

    mp.monitor.RecordPublish(err == nil, duration)
    return err
}
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

### Health Check Integration

```go
func healthCheckHandler(monitor *performance.Monitor) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        stats := monitor.GetStats()

        // Consider unhealthy if not connected or high error rate
        if !stats.IsConnected || stats.PublishSuccessRate < 0.95 {
            w.WriteHeader(http.StatusServiceUnavailable)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "status": "unhealthy",
                "stats":  stats,
            })
            return
        }

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "healthy",
            "stats":  stats,
        })
    }
}
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

## Helper Methods

The monitor provides several convenience methods:

```go
monitor := performance.NewMonitor()

// Connection state
isConnected := monitor.IsConnected()

// Current rates
publishRate := monitor.GetPublishRate()
consumeRate := monitor.GetConsumeRate()

// Total operations across publish and consume
totalOps := monitor.GetTotalOperations()

// Overall success rate
successRate := monitor.GetSuccessRate()

// Reset all counters
monitor.Reset()
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

## Best Practices

### 1. Monitor Setup

```go
// Create one monitor per client/connection
client, _ := rabbitmq.NewClient(opts...)
monitor := performance.NewMonitor()

// Record connection status
monitor.RecordConnection(true)
```

### 2. Operation Recording

```go
// Always record operations, both success and failure
start := time.Now()
err := operation()
duration := time.Since(start)
monitor.RecordPublish(err == nil, duration)
```

### 3. Periodic Stats Collection

```go
// Export stats every 30 seconds
ticker := time.NewTicker(30 * time.Second)
go func() {
    for range ticker.C {
        stats := monitor.GetStats()
        logStats(stats)
    }
}()
```

### 4. Resource Management

```go
// Reset counters periodically to prevent memory growth
if time.Since(lastReset) > time.Hour {
    monitor.Reset()
    lastReset = time.Now()
}
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

## Configuration Reference

### Stats Structure

| Field | Type | Description |
|-------|------|-------------|
| `ConnectionsTotal` | `uint64` | Total connection attempts |
| `ReconnectionsTotal` | `uint64` | Total reconnection attempts |
| `IsConnected` | `bool` | Current connection status |
| `LastConnectionTime` | `time.Time` | Time of last connection |
| `LastReconnectionTime` | `time.Time` | Time of last reconnection |
| `PublishesTotal` | `uint64` | Total publish operations |
| `PublishSuccessTotal` | `uint64` | Successful publish operations |
| `PublishErrorsTotal` | `uint64` | Failed publish operations |
| `PublishSuccessRate` | `float64` | Publish success rate (0.0-1.0) |
| `PublishRate` | `float64` | Publish operations per second |
| `ConsumesTotal` | `uint64` | Total consume operations |
| `ConsumeSuccessTotal` | `uint64` | Successful consume operations |
| `ConsumeErrorsTotal` | `uint64` | Failed consume operations |
| `ConsumeSuccessRate` | `float64` | Consume success rate (0.0-1.0) |
| `ConsumeRate` | `float64` | Consume operations per second |
| `PublishLatencyP50` | `time.Duration` | 50th percentile publish latency |
| `PublishLatencyP95` | `time.Duration` | 95th percentile publish latency |
| `PublishLatencyP99` | `time.Duration` | 99th percentile publish latency |
| `ConsumeLatencyP50` | `time.Duration` | 50th percentile consume latency |
| `ConsumeLatencyP95` | `time.Duration` | 95th percentile consume latency |
| `ConsumeLatencyP99` | `time.Duration` | 99th percentile consume latency |

🔝 [back to top](#performance-monitoring-package)

&nbsp;

## Performance Considerations

- **Memory Usage**: The monitor keeps the last 1000 latency measurements for each operation type
- **Rate Tracking**: Uses a 1-minute sliding window by default
- **Thread Safety**: All operations use atomic operations or mutexes for safety
- **Overhead**: Minimal performance impact - typically < 1% overhead

🔝 [back to top](#performance-monitoring-package)

&nbsp;

## Testing

The performance package includes comprehensive tests:

```bash
# Run tests
go test ./performance

# Run benchmarks
go test -bench=. ./performance

# Run with race detection
go test -race ./performance
```

🔝 [back to top](#performance-monitoring-package)

&nbsp;

The performance monitor provides thread-safe operation tracking and comprehensive metrics collection.

🔝 [back to top](#performance-monitoring-package)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
