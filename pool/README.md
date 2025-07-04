# Connection Pool Package

[Home](../README.md) &nbsp;/&nbsp; Connection Pool Package

&nbsp;

The `pool` package provides connection pooling functionality for high-throughput RabbitMQ applications. Connection pooling allows applications to maintain multiple connections to RabbitMQ and distribute load across them, which is particularly useful for scenarios where a single connection might become a bottleneck.

&nbsp;

## Features

- **Round-robin connection selection** - Distribute load evenly across connections
- **Automatic health monitoring** - Periodic health checks with configurable intervals
- **Connection repair and recovery** - Automatically recreate failed connections
- **Pool statistics and metrics** - Monitor pool health and performance
- **Configurable behavior** - Customize pool size, health check intervals, and repair settings
- **Thread-safe operations** - Safe for concurrent use across goroutines

🔝 [back to top](#connection-pool-package)

&nbsp;

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/pool"
)

func main() {
    // Create a connection pool with 5 connections
    connectionPool, err := pool.New(5,
        pool.WithClientOptions(
            rabbitmq.WithHosts("localhost:5672"),
            rabbitmq.WithCredentials("guest", "guest"),
        ),
    )
    if err != nil {
        log.Fatal("Failed to create connection pool:", err)
    }
    defer connectionPool.Close()

    // Get a client from the pool
    client, err := connectionPool.Get()
    if err != nil {
        log.Fatal("No healthy connections available:", err)
    }

    // Use the client normally
    publisher, err := client.NewPublisher()
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    // Publish a message
    err = publisher.Publish(context.Background(), "test.queue", rabbitmq.Publishing{
        Body: []byte("Hello from pool!"),
    })
    if err != nil {
        log.Fatal("Failed to publish:", err)
    }
}
```

🔝 [back to top](#connection-pool-package)

&nbsp;

### Advanced Configuration

```go
// Create a pool with custom configuration using functional options
connectionPool, err := pool.New(10,
    pool.WithHealthCheck(30*time.Second),     // Health check frequency
    pool.WithAutoRepair(true),                // Auto-repair failed connections
    pool.WithClientOptions(
        rabbitmq.WithHosts("localhost:5672"),
        rabbitmq.WithCredentials("user", "pass"),
        rabbitmq.WithVHost("/prod"),
    ),
)
if err != nil {
    log.Fatal(err)
}
defer connectionPool.Close()
```

🔝 [back to top](#connection-pool-package)

&nbsp;

## Pool Management

### Getting Connections

```go
// Get any available healthy client (fast, non-blocking)
client, err := connectionPool.Get()
if err != nil {
    log.Printf("No healthy connections available: %v", err)
    return
}

// Get a specific client by index (useful for sharding)
client := connectionPool.GetClientByIndex(2)
```

🔝 [back to top](#connection-pool-package)

&nbsp;

### Pool Statistics

```go
stats := connectionPool.GetStats()
fmt.Printf("Pool size: %d\n", stats.Size)
fmt.Printf("Healthy connections: %d\n", stats.HealthyConnections)
fmt.Printf("Unhealthy connections: %d\n", stats.UnhealthyConnections)
fmt.Printf("Total repair attempts: %d\n", stats.TotalRepairAttempts)
fmt.Printf("Last health check: %v\n", stats.LastHealthCheck)
fmt.Printf("Health monitoring enabled: %t\n", stats.HealthMonitoringEnabled)
fmt.Printf("Auto repair enabled: %t\n", stats.AutoRepairEnabled)

if len(stats.Errors) > 0 {
    fmt.Printf("Connection errors: %v\n", stats.Errors)
}
```

🔝 [back to top](#connection-pool-package)

&nbsp;

### Configuration Options

The pool package uses functional options for configuration:

```go
// WithHealthCheck enables health monitoring with specified interval
// Setting interval to 0 disables health monitoring
pool.WithHealthCheck(30 * time.Second)

// WithAutoRepair enables or disables automatic connection repair
pool.WithAutoRepair(true)

// WithClientOptions sets options for the RabbitMQ clients in the pool
pool.WithClientOptions(
    rabbitmq.WithHosts("localhost:5672"),
    rabbitmq.WithCredentials("user", "pass"),
    rabbitmq.WithConnectionName("my-pool"),
)
```

🔝 [back to top](#connection-pool-package)

&nbsp;

### Health Monitoring

Health monitoring and connection repair are configured at pool creation time:

```go
// Create pool with health monitoring disabled
pool, err := pool.New(5,
    pool.WithHealthCheck(0), // 0 disables health monitoring
    pool.WithClientOptions(rabbitmq.WithHosts("localhost:5672")),
)

// Create pool with custom health check interval
pool, err := pool.New(5,
    pool.WithHealthCheck(15*time.Second), // Check every 15 seconds
    pool.WithClientOptions(rabbitmq.WithHosts("localhost:5672")),
)

// Manual health check
ctx := context.Background()
err := connectionPool.HealthCheck(ctx)
if err != nil {
    log.Printf("Health check failed: %v", err)
}
```

🔝 [back to top](#connection-pool-package)

&nbsp;

### Connection Repair

Automatic connection repair is also configured at creation time:

```go
// Create pool with auto repair enabled (default)
pool, err := pool.New(5,
    pool.WithAutoRepair(true),
    pool.WithClientOptions(rabbitmq.WithHosts("localhost:5672")),
)

// Create pool with auto repair disabled
pool, err := pool.New(5,
    pool.WithAutoRepair(false),
    pool.WithClientOptions(rabbitmq.WithHosts("localhost:5672")),
)
```

🔝 [back to top](#connection-pool-package)

&nbsp;

## Best Practices

### Pool Sizing

- **Small pools (2-5 connections)**: For low-to-medium traffic applications
- **Medium pools (5-20 connections)**: For high-traffic applications
- **Large pools (20+ connections)**: For very high-throughput scenarios

&nbsp;

```go
// For most applications, start with 5 connections
pool, err := pool.New(5, opts...)

// Scale up based on your traffic patterns
pool, err := pool.New(20, opts...)  // High traffic
```

🔝 [back to top](#connection-pool-package)

&nbsp;

### Health Monitoring Options

- **Development**: Disable or use long intervals (5+ minutes)
- **Production**: Use reasonable intervals (30-60 seconds)

&nbsp;

```go
// Development - minimal monitoring
config := pool.Config{
    Size:                5,
    HealthCheckInterval: 5 * time.Minute,  // Less frequent
    RepairEnabled:       false,            // Manual intervention
}

// Production - active monitoring
config := pool.Config{
    Size:                10,
    HealthCheckInterval: 30 * time.Second, // Regular checks
    RepairEnabled:       true,             // Auto-repair
    RepairThreshold:     10 * time.Second,
}
```

🔝 [back to top](#connection-pool-package)

&nbsp;

### Error Handling

```go
client, err := connectionPool.Get()
if err != nil {
    // No healthy connections available
    // Check pool stats for debugging
    stats := connectionPool.GetStats()
    log.Printf("Pool stats: %+v", stats)
    log.Printf("Error: %v", err)

    // Consider fallback strategy
    return handleNoConnectionsAvailable()
}
```

🔝 [back to top](#connection-pool-package)

&nbsp;

### Resource Management

```go
// Always close the pool when done
defer connectionPool.Close()

// In web servers, create pool once and reuse
var globalPool *pool.ConnectionPool

func init() {
    var err error
    globalPool, err = pool.New(10,
        rabbitmq.WithHosts("localhost:5672"),
    )
    if err != nil {
        log.Fatal(err)
    }
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
    client, err := globalPool.Get()
    if err != nil {
        http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
        return
    }
    // Use client...
}
```

🔝 [back to top](#connection-pool-package)

&nbsp;

## Configuration Reference

### Pool.Config

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `Size` | `int` | Number of connections in pool | `5` |
| `MaxReconnectBackoff` | `time.Duration` | Maximum backoff for reconnection | `30s` |
| `HealthCheckInterval` | `time.Duration` | Interval between health checks | `30s` |
| `RepairEnabled` | `bool` | Enable automatic connection repair | `true` |
| `RepairThreshold` | `time.Duration` | Time before attempting repair | `10s` |

🔝 [back to top](#connection-pool-package)

&nbsp;

### Pool.Stats

| Field | Type | Description |
|-------|------|-------------|
| `Size` | `int` | Total pool size |
| `HealthyConnections` | `int` | Number of healthy connections |
| `UnhealthyConnections` | `int` | Number of unhealthy connections |
| `Closed` | `bool` | Whether pool is closed |
| `Errors` | `[]string` | Current connection errors |
| `TotalRepairAttempts` | `int64` | Total repair attempts made |
| `LastHealthCheck` | `time.Time` | Time of last health check |
| `HealthMonitoringEnabled` | `bool` | Health monitoring status |
| `RepairEnabled` | `bool` | Repair status |

🔝 [back to top](#connection-pool-package)

&nbsp;

## Integration Examples

### With HTTP Server

```go
func main() {
    // Create pool once
    connectionPool, err := pool.New(10,
        rabbitmq.WithHosts("localhost:5672"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer connectionPool.Close()

    // Use in handlers
    http.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
        client, err := connectionPool.Get()
        if err != nil {
            http.Error(w, "No connections available", http.StatusServiceUnavailable)
            return
        }

        publisher, err := client.NewPublisher()
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        defer publisher.Close()

        // Publish message...
    })

    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

🔝 [back to top](#connection-pool-package)

&nbsp;

### With Worker Pool

```go
func startWorkers(connectionPool *pool.ConnectionPool, numWorkers int) {
    for i := 0; i < numWorkers; i++ {
        go func(workerID int) {
            // Each worker gets its own client
            client := connectionPool.GetClientByIndex(workerID % connectionPool.Size())

            consumer, err := client.NewConsumer("task.queue")
            if err != nil {
                log.Printf("Worker %d failed to create consumer: %v", workerID, err)
                return
            }
            defer consumer.Close()

            // Consume messages...
        }(i)
    }
}
```

🔝 [back to top](#connection-pool-package)

&nbsp;

## Performance Considerations

- **Connection overhead**: Each connection uses memory and file descriptors
- **Health check cost**: Frequent health checks add network overhead
- **Repair impact**: Connection repairs cause temporary unavailability
- **Concurrent access**: Pool operations are thread-safe but may block briefly

🔝 [back to top](#connection-pool-package)

&nbsp;

## Testing

The pool package includes comprehensive tests. To run them:

```bash
go test ./pool
```

Note: Tests require a running RabbitMQ instance on localhost:5672.

🔝 [back to top](#connection-pool-package)

&nbsp;

The connection pool provides thread-safe access to multiple RabbitMQ connections with automatic health monitoring and repair capabilities.

🔝 [back to top](#connection-pool-package)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
