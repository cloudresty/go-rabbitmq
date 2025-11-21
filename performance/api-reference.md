# Performance Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [Performance](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides a complete API reference for the `performance` package, which provides comprehensive performance monitoring and metrics collection for RabbitMQ operations.

&nbsp;

## Constructor Functions

| Function | Description |
|----------|-------------|
| `NewMonitor()` | Creates a new performance monitor with default settings |
| `NewRateTracker(window time.Duration)` | Creates a new rate tracker with specified time window |

🔝 [back to top](#performance-package-api-reference)

&nbsp;

## Monitor Methods

### Recording Operations

| Method | Description |
|--------|-------------|
| `RecordConnection(success bool)` | Records a connection attempt with success/failure status |
| `RecordReconnection()` | Records a reconnection event |
| `RecordPublish(success bool, duration time.Duration)` | Records a publish operation with timing |
| `RecordConsume(success bool, duration time.Duration)` | Records a consume operation with timing |

### Retrieving Statistics

| Method | Description |
|--------|-------------|
| `GetStats()` | Returns comprehensive performance statistics |
| `IsConnected()` | Returns current connection status |
| `GetPublishRate()` | Returns current publish rate (operations per second) |
| `GetConsumeRate()` | Returns current consume rate (operations per second) |
| `GetTotalOperations()` | Returns total number of operations recorded |
| `GetSuccessRate()` | Returns overall success rate across all operations |

### Utility Methods

| Method | Description |
|--------|-------------|
| `Reset()` | Resets all performance counters and statistics |

🔝 [back to top](#performance-package-api-reference)

&nbsp;

## RateTracker Methods

| Method | Description |
|--------|-------------|
| `Record()` | Records an event at the current time |
| `Rate()` | Returns current rate (events per second) over the time window |

🔝 [back to top](#performance-package-api-reference)

&nbsp;

## Stats Structure

The `GetStats()` method returns a `Stats` struct containing:

### Connection Statistics

| Field | Type | Description |
|-------|------|-------------|
| `ConnectionsTotal` | `uint64` | Total number of connection attempts |
| `ReconnectionsTotal` | `uint64` | Total number of reconnection attempts |
| `IsConnected` | `bool` | Current connection status |
| `LastConnectionTime` | `time.Time` | Timestamp of last successful connection |
| `LastReconnectionTime` | `time.Time` | Timestamp of last reconnection attempt |

### Publish Statistics

| Field | Type | Description |
|-------|------|-------------|
| `PublishesTotal` | `uint64` | Total number of publish operations |
| `PublishSuccessTotal` | `uint64` | Number of successful publish operations |
| `PublishErrorsTotal` | `uint64` | Number of failed publish operations |
| `PublishSuccessRate` | `float64` | Publish success rate (0.0-1.0) |
| `PublishRate` | `float64` | Current publish rate (operations per second) |

### Consume Statistics

| Field | Type | Description |
|-------|------|-------------|
| `ConsumesTotal` | `uint64` | Total number of consume operations |
| `ConsumeSuccessTotal` | `uint64` | Number of successful consume operations |
| `ConsumeErrorsTotal` | `uint64` | Number of failed consume operations |
| `ConsumeSuccessRate` | `float64` | Consume success rate (0.0-1.0) |
| `ConsumeRate` | `float64` | Current consume rate (operations per second) |

### Latency Statistics

| Field | Type | Description |
|-------|------|-------------|
| `PublishLatencyP50` | `time.Duration` | 50th percentile (median) publish latency |
| `PublishLatencyP95` | `time.Duration` | 95th percentile publish latency |
| `PublishLatencyP99` | `time.Duration` | 99th percentile publish latency |
| `ConsumeLatencyP50` | `time.Duration` | 50th percentile (median) consume latency |
| `ConsumeLatencyP95` | `time.Duration` | 95th percentile consume latency |
| `ConsumeLatencyP99` | `time.Duration` | 99th percentile consume latency |

🔝 [back to top](#performance-package-api-reference)

&nbsp;

## Usage Examples

### Basic Performance Monitoring

```go
import "github.com/cloudresty/go-rabbitmq/performance"

// Create a performance monitor
monitor := performance.NewMonitor()

// Record connection success
monitor.RecordConnection(true)

// Record publish operations
start := time.Now()
err := publisher.Publish(ctx, exchange, routingKey, message)
duration := time.Since(start)
monitor.RecordPublish(err == nil, duration)

// Get statistics
stats := monitor.GetStats()
fmt.Printf("Publish success rate: %.2f%%\n", stats.PublishSuccessRate*100)
fmt.Printf("Average publish latency (P50): %v\n", stats.PublishLatencyP50)
```

### Comprehensive Monitoring Setup

```go
// Create monitor
monitor := performance.NewMonitor()

// Create publisher with monitoring
publisher, err := client.NewPublisher(
    rabbitmq.WithDefaultExchange("events"),
)
if err != nil {
    log.Fatal(err)
}

// Wrap publish operations with monitoring
func monitoredPublish(ctx context.Context, exchange, routingKey string, message *rabbitmq.Message) error {
    start := time.Now()
    err := publisher.Publish(ctx, exchange, routingKey, message)
    duration := time.Since(start)

    monitor.RecordPublish(err == nil, duration)
    return err
}

// Monitor consumer operations
func monitoredConsume(ctx context.Context, queue string, handler rabbitmq.MessageHandler) error {
    wrappedHandler := func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        start := time.Now()
        err := handler(ctx, delivery)
        duration := time.Since(start)

        monitor.RecordConsume(err == nil, duration)
        return err
    }

    return consumer.Consume(ctx, queue, wrappedHandler)
}
```

### Rate Tracking

```go
// Create a custom rate tracker with 30-second window
tracker := performance.NewRateTracker(30 * time.Second)

// Record events
for i := 0; i < 10; i++ {
    tracker.Record()
    time.Sleep(100 * time.Millisecond)
}

// Get current rate
rate := tracker.Rate()
fmt.Printf("Current rate: %.2f events/second\n", rate)
```

### Periodic Statistics Reporting

```go
func startPerformanceReporting(monitor *performance.Monitor, interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        defer ticker.Stop()
        for range ticker.C {
            stats := monitor.GetStats()

            log.Printf("Performance Stats:")
            log.Printf("  Connection: %v (Total: %d, Reconnections: %d)",
                stats.IsConnected, stats.ConnectionsTotal, stats.ReconnectionsTotal)
            log.Printf("  Publish: %d total, %.2f%% success, %.2f ops/sec",
                stats.PublishesTotal, stats.PublishSuccessRate*100, stats.PublishRate)
            log.Printf("  Consume: %d total, %.2f%% success, %.2f ops/sec",
                stats.ConsumesTotal, stats.ConsumeSuccessRate*100, stats.ConsumeRate)
            log.Printf("  Latency P50/P95/P99: %v/%v/%v",
                stats.PublishLatencyP50, stats.PublishLatencyP95, stats.PublishLatencyP99)
        }
    }()
}

// Start reporting every 30 seconds
startPerformanceReporting(monitor, 30*time.Second)
```

🔝 [back to top](#performance-package-api-reference)

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

func exportToPrometheus(monitor *performance.Monitor) {
    stats := monitor.GetStats()

    // Update counters
    publishTotal.WithLabelValues("success").Add(float64(stats.PublishSuccessTotal))
    publishTotal.WithLabelValues("error").Add(float64(stats.PublishErrorsTotal))

    // Record latencies
    if stats.PublishLatencyP50 > 0 {
        publishLatency.WithLabelValues().Observe(stats.PublishLatencyP50.Seconds())
    }
}
```

### InfluxDB Integration

```go
func exportToInfluxDB(monitor *performance.Monitor) {
    stats := monitor.GetStats()

    point := map[string]interface{}{
        "connections_total":      stats.ConnectionsTotal,
        "publish_success_rate":   stats.PublishSuccessRate,
        "publish_rate":          stats.PublishRate,
        "publish_latency_p50":   stats.PublishLatencyP50.Nanoseconds(),
        "publish_latency_p95":   stats.PublishLatencyP95.Nanoseconds(),
        "consume_rate":          stats.ConsumeRate,
        "is_connected":          stats.IsConnected,
    }

    // Write to InfluxDB (implementation depends on your client)
    writePointToInfluxDB("rabbitmq_performance", point)
}
```

🔝 [back to top](#performance-package-api-reference)

&nbsp;

## Health Check Integration

```go
func healthCheckHandler(monitor *performance.Monitor) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        stats := monitor.GetStats()

        // Define health criteria
        isHealthy := stats.IsConnected &&
                    stats.PublishSuccessRate >= 0.95 &&
                    stats.ConsumeSuccessRate >= 0.95

        if !isHealthy {
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

🔝 [back to top](#performance-package-api-reference)

&nbsp;

## Performance Considerations

| Factor | Impact | Description |
|--------|--------|-------------|
| **Memory Usage** | Low | Keeps last 1000 latency measurements per operation type |
| **CPU Overhead** | Minimal | Uses atomic operations and minimal locking |
| **Rate Window** | Configurable | Default 1-minute window for rate calculations |
| **Thread Safety** | Full | All operations are safe for concurrent use |
| **Precision** | Good | Nanosecond precision for latency measurements |

### Memory Management

- **Latency History**: Automatically limited to last 1000 measurements
- **Rate Tracking**: Uses sliding window with automatic cleanup
- **Reset Capability**: `Reset()` method clears all stored data
- **Atomic Operations**: Minimal memory overhead for counters

🔝 [back to top](#performance-package-api-reference)

&nbsp;

## Best Practices

1. **Single Monitor Instance**: Use one monitor per client/connection
2. **Regular Reporting**: Export statistics every 30-60 seconds
3. **Resource Management**: Call `Reset()` periodically to prevent memory growth
4. **Health Thresholds**: Define clear success rate thresholds for health checks
5. **Latency Monitoring**: Monitor P95/P99 latencies for performance insights
6. **Rate Monitoring**: Track operations per second for capacity planning
7. **Error Classification**: Monitor both connection and operation errors separately

🔝 [back to top](#performance-package-api-reference)

&nbsp;

## Troubleshooting

### Common Scenarios

| Issue | Symptoms | Solutions |
|-------|----------|-----------|
| **High Latency** | P95/P99 latencies increasing | Check network, broker load, message size |
| **Low Success Rate** | Success rate < 95% | Investigate connection stability, broker errors |
| **Memory Growth** | Increasing memory usage | Call `Reset()` periodically or reduce retention |
| **Rate Drops** | Unexpected rate decreases | Check connection status, consumer health |

### Debugging Tips

```go
// Log detailed statistics for debugging
stats := monitor.GetStats()
log.Printf("Debug Stats: %+v", stats)

// Check specific metrics
if !stats.IsConnected {
    log.Printf("Connection issue: last connected at %v", stats.LastConnectionTime)
}

if stats.PublishSuccessRate < 0.9 {
    log.Printf("Publish issues: %d errors out of %d attempts",
        stats.PublishErrorsTotal, stats.PublishesTotal)
}
```

🔝 [back to top](#performance-package-api-reference)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
