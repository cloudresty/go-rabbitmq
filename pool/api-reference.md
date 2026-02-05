# Pool Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [Pool Package](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides the complete API reference for the `pool` sub-package. The pool package provides connection pooling functionality for high-throughput RabbitMQ applications, allowing you to maintain multiple connections and distribute load across them.

&nbsp;

## Constructor Functions

| Function | Description |
| :--- | :--- |
| `New(size int, opts ...Option)` | Creates a new connection pool with the specified size and configuration options |

&nbsp;

🔝 [back to top](#pool-package-api-reference)

&nbsp;

## Configuration Options

| Function | Description |
| :--- | :--- |
| `WithHealthCheck(interval time.Duration)` | Enables health monitoring with the specified interval for automatic connection health checks |
| `WithAutoRepair(enabled bool)` | Enables or disables automatic connection repair when unhealthy connections are detected |
| `WithClientOptions(opts ...rabbitmq.Option)` | Sets options for the RabbitMQ clients in the pool (applied to all connections) |

&nbsp;

🔝 [back to top](#pool-package-api-reference)

&nbsp;

## ConnectionPool Methods

| Function | Description |
| :--- | :--- |
| `Get()` | Gets a healthy client from the pool using round-robin selection |
| `GetClientByIndex(index int)` | Gets a client by specific index from the pool |
| `Size()` | Returns the total size of the connection pool |
| `HealthCheck(ctx context.Context)` | Performs a manual health check on all connections in the pool |
| `GetStats()` | Returns detailed statistics about the connection pool health and performance |
| `Stats()` | Returns pool statistics in the format expected by the main rabbitmq package |
| `HealthyCount()` | Returns the number of currently healthy connections in the pool |
| `Close()` | Gracefully closes all connections in the pool |

&nbsp;

🔝 [back to top](#pool-package-api-reference)

&nbsp;

## Internal Methods

| Function | Description |
| :--- | :--- |
| `startHealthMonitoring()` | Begins periodic health checks (called automatically when health monitoring is enabled) |
| `updateHealthyClients()` | Updates the list of healthy clients (called internally during health checks) |
| `performHealthCheck()` | Checks all connections and repairs if needed (called automatically by health monitoring) |
| `repairConnections(indices []int)` | Repairs unhealthy connections at the specified indices |

&nbsp;

🔝 [back to top](#pool-package-api-reference)

&nbsp;

## Types and Structures

| Type | Description |
| :--- | :--- |
| `ConnectionPool` | Main structure that manages multiple connections for high-throughput applications |
| `Stats` | Public statistics structure containing pool health information, connection counts, and error details |
| `Option` | Configuration function type for customizing pool behavior |

&nbsp;

🔝 [back to top](#pool-package-api-reference)

&nbsp;

## Usage Examples

&nbsp;

### Basic Pool Creation

```go
import "github.com/cloudresty/go-rabbitmq/pool"

// Create a connection pool with 5 connections
pool, err := pool.New(5,
    pool.WithClientOptions(rabbitmq.WithURL("amqp://localhost")),
    pool.WithHealthCheck(30*time.Second),
    pool.WithAutoRepair(true),
)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()
```

&nbsp;

🔝 [back to top](#pool-package-api-reference)

&nbsp;

### Getting Clients from Pool

```go
// Get a healthy client from the pool
client, err := pool.Get()
if err != nil {
    log.Fatal("no healthy connections available:", err)
}

// Use the client normally
publisher, err := client.NewPublisher(...)
if err != nil {
    log.Fatal(err)
}
defer publisher.Close()
```

&nbsp;

🔝 [back to top](#pool-package-api-reference)

&nbsp;

### Monitoring Pool Health

```go
// Get pool statistics
stats := pool.GetStats()
log.Printf("Pool size: %d, Healthy: %d, Unhealthy: %d",
    stats.Size, stats.HealthyConnections, stats.UnhealthyConnections)

// Manual health check
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

err := pool.HealthCheck(ctx)
if err != nil {
    log.Printf("Health check failed: %v", err)
}
```

&nbsp;

🔝 [back to top](#pool-package-api-reference)

&nbsp;

### Advanced Configuration

```go
// Create pool with custom client options and health monitoring
pool, err := pool.New(10,
    pool.WithClientOptions(
        rabbitmq.FromEnv(),
        rabbitmq.WithConnectionName("high-throughput-pool"),
        rabbitmq.WithAutoReconnect(true),
    ),
    pool.WithHealthCheck(15*time.Second), // Check every 15 seconds
    pool.WithAutoRepair(true),            // Auto-repair failed connections
)
```

&nbsp;

🔝 [back to top](#pool-package-api-reference)

&nbsp;

## Best Practices

1. **Pool Size**: Choose pool size based on your application's concurrency requirements and RabbitMQ server capacity
2. **Health Monitoring**: Enable health monitoring for production deployments to automatically detect and repair failed connections
3. **Auto Repair**: Enable auto-repair to maintain pool health without manual intervention
4. **Client Options**: Use `WithClientOptions()` to apply consistent configuration across all pooled connections
5. **Graceful Shutdown**: Always call `Close()` to properly shutdown all connections in the pool
6. **Error Handling**: Check the return value of `Get()` method as it may return an error if no healthy connections are available

&nbsp;

🔝 [back to top](#pool-package-api-reference)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

<sub>&copy; Cloudresty - All rights reserved</sub>

&nbsp;
