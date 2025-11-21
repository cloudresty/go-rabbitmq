# Production Features

[Home](../README.md) &nbsp;/&nbsp; [Docs](README.md) &nbsp;/&nbsp; Production Features

&nbsp;

This document covers all production-ready features designed for high-availability, fault-tolerant deployments. The `go-rabbitmq` package is built from the ground up for enterprise-scale applications with comprehensive resilience and observability features.

&nbsp;

## Table of Contents

- [Auto-Reconnection](#auto-reconnection)
- [Multi-Host Failover](#multi-host-failover)
- [Health Monitoring](#health-monitoring)
- [Topology Auto-Healing](#topology-auto-healing)
- [Connection Pooling](#connection-pooling)
- [Graceful Shutdown](#graceful-shutdown)
- [Message Reliability](#message-reliability)
  - [Publisher Confirmations](#publisher-confirmations)
  - [Delivery Assurance](#delivery-assurance)
  - [Publisher Retry](#publisher-retry)
  - [Consumer Reliability](#consumer-reliability)
  - [Consumer Retry](#consumer-retry)
- [Timeout Configuration](#timeout-configuration)
- [Advanced Security & Performance](#advanced-security--performance)
- [Production Checklist](#production-checklist)

&nbsp;

## Auto-Reconnection

Intelligent reconnection with exponential backoff for network resilience and zero-downtime operations.

&nbsp;

### Basic Auto-Reconnection

```go
package main

import (
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client with auto-reconnection enabled by default
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Or configure programmatically with reconnection settings
    client, err = rabbitmq.NewClient(
        rabbitmq.WithHosts("prod-rabbit1.com:5672", "prod-rabbit2.com:5672"),
        rabbitmq.WithAutoReconnect(true),
        rabbitmq.WithReconnectDelay(5*time.Second),
        rabbitmq.WithMaxReconnectAttempts(0), // Unlimited attempts
        rabbitmq.WithConnectionName("resilient-service"),
    )
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Create publisher from client
    publisher, err := client.NewPublisher()
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    log.Printf("Client ready with auto-reconnection: %s", client.ConnectionName())
}
```

🔝 [back to top](#production-features)

&nbsp;

### Environment Configuration

```bash
# Production auto-reconnection settings
export RABBITMQ_HOSTS=rabbit1.prod.com:5672,rabbit2.prod.com:5672,rabbit3.prod.com:5672
export RABBITMQ_AUTO_RECONNECT=true
export RABBITMQ_RECONNECT_DELAY=5s
export RABBITMQ_MAX_RECONNECT_ATTEMPTS=0
export RABBITMQ_HEARTBEAT=30s
export RABBITMQ_DIAL_TIMEOUT=30s
export RABBITMQ_CHANNEL_TIMEOUT=10s
```

🔝 [back to top](#production-features)

&nbsp;

### Reconnection Features

- **Exponential Backoff**: Intelligent delay progression to avoid overwhelming servers
- **Multiple Host Failover**: Automatic failover between multiple RabbitMQ nodes
- **Connection State Tracking**: Real-time monitoring of connection status
- **Automatic Recovery**: Seamless operation resumption after reconnection
- **Zero Message Loss**: In-flight message protection during reconnection
- **Heartbeat Monitoring**: Proactive connection health detection

🔝 [back to top](#production-features)

&nbsp;

## Multi-Host Failover

Enterprise-grade high availability with automatic failover across multiple RabbitMQ nodes.

&nbsp;

### High Availability Configuration

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Multi-node cluster configuration
    client, err := rabbitmq.NewClient(
        rabbitmq.WithHosts(
            "rabbit-node1.prod.com:5672",
            "rabbit-node2.prod.com:5672",
            "rabbit-node3.prod.com:5672",
        ),
        rabbitmq.WithAutoReconnect(true),
        rabbitmq.WithConnectionName("payment-service-ha"),
    )
    if err != nil {
        log.Fatal("Failed to create HA client:", err)
    }
    defer client.Close()

    // Create publisher from client
    publisher, err := client.NewPublisher()
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    // Publish critical messages with automatic failover
    ctx := context.Background()
    for i := 0; i < 1000; i++ {
        message := rabbitmq.NewMessage([]byte("payment-data")).
            WithMessageID(fmt.Sprintf("payment-%d", i)).
            WithPersistent()

        err := publisher.Publish(ctx, "", "payments.queue", message)
        if err != nil {
            log.Printf("Publish failed, but will auto-retry: %v", err)
            continue
        }

        if i%100 == 0 {
            log.Printf("Published %d messages successfully", i+1)
        }
    }
}
```

🔝 [back to top](#production-features)

&nbsp;

### Cluster Environment Variables

```bash
# Production cluster configuration
export RABBITMQ_HOSTS=rabbit1.prod.com:5672,rabbit2.prod.com:5672,rabbit3.prod.com:5672
export RABBITMQ_USERNAME=prod_user
export RABBITMQ_PASSWORD=secure_cluster_password
export RABBITMQ_VHOST=/production
export RABBITMQ_CONNECTION_NAME=payment-service-cluster
export RABBITMQ_AUTO_RECONNECT=true
export RABBITMQ_HEARTBEAT=30s
```

🔝 [back to top](#production-features)

&nbsp;

### Failover Features

- **Automatic Node Selection**: Intelligent routing to healthy cluster nodes
- **Connection Load Balancing**: Distributes connections across available nodes
- **Failure Detection**: Rapid detection of node failures and network issues
- **Seamless Failover**: Zero-configuration failover to backup nodes
- **Node Recovery**: Automatic reconnection to recovered nodes
- **Cluster Topology Awareness**: Adapts to changing cluster configurations

🔝 [back to top](#production-features)

&nbsp;

## Health Monitoring

Comprehensive health monitoring with automated checks and detailed diagnostics.

&nbsp;

### Connection Health Checks

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Simple health check
    ctx := context.Background()
    if err := client.Ping(ctx); err != nil {
        log.Printf("RabbitMQ health check failed: %v", err)
    } else {
        log.Println("RabbitMQ cluster is healthy")
    }

    // Detailed connection information
    log.Printf("Connected to: %s", client.URL())
    log.Printf("Connection name: %s", client.ConnectionName())
}
```

🔝 [back to top](#production-features)

&nbsp;

### Automated Health Monitoring

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func healthMonitor(client *rabbitmq.Client) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

            if err := client.Ping(ctx); err != nil {
                log.Printf("Health check failed: %v", err)
                // Trigger alerting system
                // sendAlert("RabbitMQ health check failed", err)
            } else {
                log.Printf("Health check passed - Connection: %s", client.ConnectionName())
            }

            cancel()
        }
    }
}

func main() {
    client, err := rabbitmq.NewClient(
        rabbitmq.FromEnv(),
        rabbitmq.WithConnectionName("health-monitored-service"),
    )
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Start background health monitoring
    go healthMonitor(client)

    // Your application logic here...
    select {} // Keep running
}
```

🔝 [back to top](#production-features)

&nbsp;

### Health Check Features

- **Connection Validation**: Verifies active connection to RabbitMQ cluster
- **Automated Monitoring**: Background health checks at configurable intervals
- **Early Issue Detection**: Proactive identification of connection problems
- **Cluster Status**: Health status across multiple cluster nodes
- **Timeout Protection**: Configurable timeouts prevent hanging health checks
- **Integration Ready**: Easy integration with monitoring systems and alerting

🔝 [back to top](#production-features)

&nbsp;

## Topology Auto-Healing

Enterprise-grade topology validation and automatic recreation for maximum reliability. Ensures your exchanges, queues, and bindings remain consistent even if deleted externally.

&nbsp;

### Default Auto-Healing Behavior

Topology validation is enabled by default with zero configuration required:

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Auto-healing enabled by default - no configuration needed!
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    admin := client.Admin()
    ctx := context.Background()

    // Declare topology - automatically tracked for validation
    err = admin.DeclareExchange(ctx, "orders", rabbitmq.ExchangeTypeTopic,
        rabbitmq.WithExchangeDurable(),
    )
    if err != nil {
        log.Fatal("Failed to declare exchange:", err)
    }

    queue, err := admin.DeclareQueue(ctx, "order-processing",
        rabbitmq.WithDurable(),
    )
    if err != nil {
        log.Fatal("Failed to declare queue:", err)
    }

    // Binding automatically registered for monitoring
    err = admin.BindQueue(ctx, queue.Name, "orders", "order.*")
    if err != nil {
        log.Fatal("Failed to bind queue:", err)
    }

    // All operations now validate topology exists before executing
    // If deleted externally, topology is automatically recreated
    log.Println("Topology declared and protected by auto-healing")
}
```

&nbsp;

### Environment Configuration (Topology)

Configure topology validation through environment variables:

```bash
# Enable/disable topology validation (default: true)
RABBITMQ_TOPOLOGY_VALIDATION=true

# Enable/disable automatic recreation (default: true)
RABBITMQ_TOPOLOGY_AUTO_RECREATION=true

# Enable/disable background monitoring (default: true)
RABBITMQ_TOPOLOGY_BACKGROUND_VALIDATION=true

# Background validation interval (default: 30s)
RABBITMQ_TOPOLOGY_VALIDATION_INTERVAL=30s
```

&nbsp;

### Custom Configuration

Advanced users can customize topology validation behavior:

```go
// Custom validation interval for critical systems
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithTopologyValidationInterval(10*time.Second), // More frequent validation
)

// Disable background validation only
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithoutTopologyBackgroundValidation(), // Keep operation-time validation
)

// Disable auto-recreation, keep validation
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithoutTopologyAutoRecreation(), // Validate but don't recreate
)

// Disable all topology features (advanced users only)
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithoutTopologyValidation(), // Completely disable
)
```

&nbsp;

### Production Benefits

- **Zero Downtime**: Operations continue seamlessly even when topology is deleted externally
- **Automatic Recovery**: Missing exchanges, queues, and bindings are recreated transparently
- **Background Monitoring**: Periodic validation catches issues before they affect operations
- **Production Ready**: Enabled by default with sensible intervals for enterprise use
- **Configurable**: Full control over validation behavior for different environments

🔝 [back to top](#production-features)

&nbsp;

## Connection Pooling

High-performance connection management for applications requiring massive throughput and optimal resource utilization.

&nbsp;

### Enterprise Connection Pool

```go
package main

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/pool"
)

func main() {
    // Create a connection pool for high-throughput applications
    connectionPool, err := pool.New(10, // Pool size
        pool.WithClientOptions(
            rabbitmq.WithHosts("rabbit1.prod.com:5672", "rabbit2.prod.com:5672"),
            rabbitmq.WithConnectionName("high-throughput-pool"),
            rabbitmq.WithAutoReconnect(true),
        ),
    )
    if err != nil {
        log.Fatal("Failed to create connection pool:", err)
    }
    defer connectionPool.Close()

    // Health check the entire pool
    ctx := context.Background()
    healthyCount := connectionPool.HealthyCount()
    totalCount := connectionPool.Size()
    log.Printf("Pool health: %d/%d connections healthy", healthyCount, totalCount)

    // Use multiple publishers from the pool
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()

            // Get a client from the pool
            client, err := connectionPool.Get()
            if err != nil {
                log.Printf("Worker %d: Failed to get client from pool: %v", workerID, err)
                return
            }

            publisher, err := client.NewPublisher()
            if err != nil {
                log.Printf("Worker %d: Failed to create publisher: %v", workerID, err)
                return
            }
            defer publisher.Close()

            // High-volume publishing
            for j := 0; j < 1000; j++ {
                message := rabbitmq.NewMessage([]byte(fmt.Sprintf("message-%d-%d", workerID, j))).
                    WithMessageID(fmt.Sprintf("msg-%d-%d", workerID, j))

                err := publisher.Publish(ctx, "", "high-volume.queue", message)
                if err != nil {
                    log.Printf("Worker %d: Publish failed: %v", workerID, err)
                }
            }

            log.Printf("Worker %d: Completed 1000 publishes", workerID)
        }(i)
    }

    wg.Wait()

    // Final pool statistics
    finalStats := connectionPool.Stats()
    log.Printf("Pool stats - Size: %d, Healthy: %d",
        finalStats.TotalConnections, finalStats.HealthyConnections)
}
```

🔝 [back to top](#production-features)

&nbsp;

### Pool Configuration

```bash
# High-performance production pool settings
export RABBITMQ_HOSTS=rabbit1.prod.com:5672,rabbit2.prod.com:5672,rabbit3.prod.com:5672
export RABBITMQ_USERNAME=pool_user
export RABBITMQ_PASSWORD=pool_secure_password
export RABBITMQ_CONNECTION_NAME=enterprise-pool
export RABBITMQ_HEARTBEAT=60s
export RABBITMQ_AUTO_RECONNECT=true
export RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT=10s
export RABBITMQ_PUBLISHER_SHUTDOWN_TIMEOUT=30s
```

🔝 [back to top](#production-features)

&nbsp;

### Connection Pool Features

- **Round-Robin Load Balancing**: Evenly distributes load across pool connections
- **Automatic Scaling**: Dynamic connection management based on demand
- **Health Monitoring**: Per-connection health tracking and reporting
- **Resource Optimization**: Efficient connection reuse and resource management
- **Failover Support**: Pool-level failover with connection replacement
- **Performance Metrics**: Detailed statistics and performance monitoring

🔝 [back to top](#production-features)

&nbsp;

## Graceful Shutdown

Production-ready graceful shutdown with coordinated resource cleanup and zero message loss.

&nbsp;

### Basic Graceful Shutdown

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client and components
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }

    publisher, err := client.NewPublisher()
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }

    consumer, err := client.NewConsumer()
    if err != nil {
        log.Fatal("Failed to create consumer:", err)
    }

    // Set up graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    var wg sync.WaitGroup

    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Start consumer with graceful shutdown support
    wg.Add(1)
    go func() {
        defer wg.Done()

        err := consumer.Consume(ctx, "orders.queue", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
            // Process message
            log.Printf("Processing order: %s", string(delivery.Body))

            // Simulate processing time
            time.Sleep(100 * time.Millisecond)

            return delivery.Ack()
        })

        if err != nil {
            log.Printf("Consumer stopped: %v", err)
        }
    }()

    // Wait for shutdown signal
    <-sigChan
    log.Println("Received shutdown signal, initiating graceful shutdown...")

    // Cancel context to stop new operations
    cancel()

    // Give ongoing operations time to complete
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    // Wait for graceful shutdown or timeout
    select {
    case <-done:
        log.Println("Graceful shutdown completed successfully")
    case <-time.After(30 * time.Second):
        log.Println("Shutdown timeout reached, forcing exit")
    }

    // Close resources
    consumer.Close()
    publisher.Close()
    client.Close()

    log.Println("Application shutdown complete")
}
```

🔝 [back to top](#production-features)

&nbsp;

### Advanced Coordinated Shutdown

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

type ShutdownManager struct {
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup
    resources  []interface{ Close() error }
    timeout    time.Duration
    mu         sync.Mutex
}

func NewShutdownManager(timeout time.Duration) *ShutdownManager {
    ctx, cancel := context.WithCancel(context.Background())
    return &ShutdownManager{
        ctx:       ctx,
        cancel:    cancel,
        timeout:   timeout,
        resources: make([]interface{ Close() error }, 0),
    }
}

func (sm *ShutdownManager) Register(resource interface{ Close() error }) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.resources = append(sm.resources, resource)
}

func (sm *ShutdownManager) Context() context.Context {
    return sm.ctx
}

func (sm *ShutdownManager) Add(delta int) {
    sm.wg.Add(delta)
}

func (sm *ShutdownManager) Done() {
    sm.wg.Done()
}

func (sm *ShutdownManager) SetupSignalHandler() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigChan
        log.Println("Shutdown signal received, initiating graceful shutdown...")
        sm.Shutdown()
    }()
}

func (sm *ShutdownManager) Shutdown() {
    // Cancel context to stop new operations
    sm.cancel()

    // Wait for ongoing operations with timeout
    done := make(chan struct{})
    go func() {
        sm.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        log.Println("All operations completed gracefully")
    case <-time.After(sm.timeout):
        log.Println("Shutdown timeout reached, forcing cleanup")
    }

    // Close all registered resources
    sm.mu.Lock()
    defer sm.mu.Unlock()

    for i, resource := range sm.resources {
        if err := resource.Close(); err != nil {
            log.Printf("Error closing resource %d: %v", i, err)
        }
    }

    log.Println("Graceful shutdown completed")
}

func main() {
    shutdownManager := NewShutdownManager(30 * time.Second)
    shutdownManager.SetupSignalHandler()

    // Create client and services
    client, _ := rabbitmq.NewClient(rabbitmq.FromEnv())
    publisher, _ := client.NewPublisher()
    consumer, _ := client.NewConsumer()

    // Register for coordinated shutdown
    shutdownManager.Register(consumer)
    shutdownManager.Register(publisher)
    shutdownManager.Register(client)

    // Start background services
    shutdownManager.Add(2)

    go func() {
        defer shutdownManager.Done()
        // Publisher worker
        publisherLoop(shutdownManager.Context(), publisher)
    }()

    go func() {
        defer shutdownManager.Done()
        // Consumer worker
        consumerLoop(shutdownManager.Context(), consumer)
    }()

    // Wait for shutdown
    select {
    case <-shutdownManager.Context().Done():
        log.Println("Application shutting down...")
    }
}

func publisherLoop(ctx context.Context, publisher *rabbitmq.Publisher) {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            message := rabbitmq.NewMessage([]byte("health-check"))
            publisher.Publish(ctx, "", "health.queue", message)
        }
    }
}

func consumerLoop(ctx context.Context, consumer *rabbitmq.Consumer) {
    consumer.Consume(ctx, "orders.queue", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        // Process message with context awareness
        select {
        case <-ctx.Done():
            log.Println("Shutdown requested, stopping message processing")
            return delivery.Nack(true)
        default:
            // Normal processing
            time.Sleep(50 * time.Millisecond)
            return delivery.Ack()
        }
    })
}
```

🔝 [back to top](#production-features)

&nbsp;

### Shutdown Features

- **Signal Handling**: Automatic SIGINT/SIGTERM signal processing
- **Graceful Termination**: Waits for in-flight operations to complete
- **Timeout Protection**: Prevents indefinite waiting during shutdown
- **Resource Coordination**: Unified shutdown across multiple components
- **Zero Message Loss**: Ensures message processing completion before exit
- **Context Awareness**: Propagates shutdown signals through operation contexts

🔝 [back to top](#production-features)

&nbsp;

## Message Reliability

Enterprise-grade message delivery guarantees with publisher confirmations, consumer acknowledgments, and dead letter handling.

&nbsp;

### Publisher Confirmations

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Publisher with confirmation mode for critical messages
    publisher, err := client.NewPublisher(
        rabbitmq.WithConfirmation(10*time.Second),
    )
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    ctx := context.Background()

    // Publish critical financial transaction
    message := rabbitmq.NewMessage([]byte(`{"amount": 1000, "currency": "USD", "account": "12345"}`)).
        WithMessageID("txn-2024-001").
        WithPersistent()

    err = publisher.Publish(ctx, "", "financial.transactions", message)
    if err != nil {
        log.Printf("Critical transaction failed: %v", err)
        // Trigger compensation logic
    } else {
        log.Println("Transaction confirmed by RabbitMQ broker")
    }

    // Batch publishing with confirmations
    messages := []rabbitmq.PublishRequest{
        {
            Exchange:   "",
            RoutingKey: "orders.processing",
            Message: rabbitmq.NewMessage([]byte(`{"order_id": "order-001", "status": "pending"}`)).
                WithMessageID("order-001").
                WithPersistent(),
        },
        {
            Exchange:   "",
            RoutingKey: "orders.processing",
            Message: rabbitmq.NewMessage([]byte(`{"order_id": "order-002", "status": "pending"}`)).
                WithMessageID("order-002").
                WithPersistent(),
        },
    }

    results, err := publisher.PublishBatch(ctx, messages)
    if err != nil {
        log.Printf("Batch publish failed: %v", err)
    }

    for i, result := range results {
        if result.Error != nil {
            log.Printf("Message %d failed: %v", i, result.Error)
        } else {
            log.Printf("Message %d confirmed", i)
        }
    }
}
```

🔝 [back to top](#production-features)

&nbsp;

### Delivery Assurance

Asynchronous delivery tracking with callbacks for non-blocking, reliable message publishing.

&nbsp;

**Key Differences from Publisher Confirmations:**
- **Publisher Confirmations** (`WithConfirmation`): Blocks on each publish until confirmed
- **Delivery Assurance** (`WithDeliveryAssurance`): Asynchronous with callbacks, non-blocking

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Publisher with delivery assurance and default callback
    publisher, err := client.NewPublisher(
        rabbitmq.WithDeliveryAssurance(),
        rabbitmq.WithDefaultDeliveryCallback(func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
            switch outcome {
            case rabbitmq.DeliverySuccess:
                log.Printf("✓ Message %s delivered successfully", messageID)
            case rabbitmq.DeliveryFailed:
                log.Printf("✗ Message %s failed: %s", messageID, errorMessage)
                // Trigger alerting, compensation logic, etc.
            case rabbitmq.DeliveryNacked:
                log.Printf("⚠ Message %s nacked by broker: %s", messageID, errorMessage)
                // Broker rejected due to resource constraints
            case rabbitmq.DeliveryTimeout:
                log.Printf("⏱ Message %s timed out waiting for confirmation", messageID)
            }
        }),
        rabbitmq.WithDeliveryTimeout(30*time.Second),
        rabbitmq.WithMandatoryByDefault(true), // Detect routing failures
    )
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    ctx := context.Background()

    // Publish with delivery assurance (non-blocking)
    message := rabbitmq.NewMessage([]byte(`{"order_id": "12345", "amount": 99.99}`)).
        WithContentType("application/json").
        WithPersistent()

    err = publisher.PublishWithDeliveryAssurance(ctx, "orders", "order.created", message,
        rabbitmq.DeliveryOptions{
            MessageID: "order-12345",
            Mandatory: true,
        })
    if err != nil {
        log.Printf("Failed to publish: %v", err)
        return
    }

    log.Println("Message published, callback will be invoked asynchronously")

    // Publish with per-message callback (overrides default)
    err = publisher.PublishWithDeliveryAssurance(ctx, "orders", "order.updated", message,
        rabbitmq.DeliveryOptions{
            MessageID: "order-12346",
            Mandatory: true,
            Callback: func(msgID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
                if outcome == rabbitmq.DeliverySuccess {
                    log.Printf("Order update confirmed: %s", msgID)
                    // Update database status
                } else {
                    log.Printf("Order update failed: %s - %s", msgID, errorMessage)
                    // Trigger retry or compensation
                }
            },
        })
    if err != nil {
        log.Printf("Failed to publish: %v", err)
    }

    // Get delivery statistics
    time.Sleep(2 * time.Second) // Wait for confirmations
    stats := publisher.GetDeliveryStats()
    log.Printf("Delivery Stats:")
    log.Printf("  Total Published:  %d", stats.TotalPublished)
    log.Printf("  Total Confirmed:  %d", stats.TotalConfirmed)
    log.Printf("  Total Returned:   %d", stats.TotalReturned)
    log.Printf("  Total Nacked:     %d", stats.TotalNacked)
    log.Printf("  Total Timed Out:  %d", stats.TotalTimedOut)
    log.Printf("  Pending Messages: %d", stats.PendingMessages)
}
```

🔝 [back to top](#production-features)

&nbsp;

### Publisher Retry

Automatic re-publishing of nacked messages with configurable backoff for at-least-once delivery guarantees.

&nbsp;

**When to Use:**
- Critical messages that must be delivered (financial transactions, orders, etc.)
- Transient broker issues (resource constraints, temporary unavailability)
- At-least-once delivery requirements

**Important:** This feature stores messages in memory until confirmed. Monitor memory usage for high-throughput scenarios.

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Publisher with automatic retry on nack
    publisher, err := client.NewPublisher(
        rabbitmq.WithDeliveryAssurance(),
        rabbitmq.WithPublisherRetry(3, 1*time.Second), // 3 retries, 1s backoff
        rabbitmq.WithDefaultDeliveryCallback(func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
            switch outcome {
            case rabbitmq.DeliverySuccess:
                log.Printf("✓ Message %s delivered (may have been retried)", messageID)
            case rabbitmq.DeliveryNacked:
                log.Printf("⚠ Message %s nacked, will retry automatically", messageID)
            case rabbitmq.DeliveryFailed:
                log.Printf("✗ Message %s failed after all retries: %s", messageID, errorMessage)
                // All retry attempts exhausted - trigger compensation
            }
        }),
    )
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    ctx := context.Background()

    // Publish critical message - will be retried automatically if nacked
    message := rabbitmq.NewMessage([]byte(`{"transaction_id": "txn-001", "amount": 1000}`)).
        WithPersistent()

    err = publisher.PublishWithDeliveryAssurance(ctx, "payments", "payment.process", message,
        rabbitmq.DeliveryOptions{
            MessageID: "payment-txn-001",
            Mandatory: true,
        })
    if err != nil {
        log.Printf("Failed to publish: %v", err)
        return
    }

    log.Println("Critical message published with automatic retry protection")
    time.Sleep(5 * time.Second) // Wait for potential retries and confirmation
}
```

🔝 [back to top](#production-features)

&nbsp;

### Consumer Reliability

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Consumer with reliability features
    consumer, err := client.NewConsumer(
        rabbitmq.WithPrefetchCount(10),
        rabbitmq.WithMessageTimeout(5*time.Minute),
    )
    if err != nil {
        log.Fatal("Failed to create consumer:", err)
    }
    defer consumer.Close()

    ctx := context.Background()

    // Consume with reliable message processing
    err = consumer.Consume(ctx, "orders.processing", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        // Extract message metadata for reliability tracking
        messageID := delivery.MessageId
        timestamp := delivery.Timestamp
        deliveryTag := delivery.DeliveryTag
        redelivered := delivery.Redelivered

        log.Printf("Processing message ID: %s, Delivery: %d, Redelivered: %v",
            messageID, deliveryTag, redelivered)

        // Check for redelivery to detect potential processing issues
        if redelivered {
            log.Printf("Message %s is being redelivered", messageID)
        }

        // Process the message
        if err := processOrder(delivery.Body); err != nil {
            log.Printf("Failed to process order %s: %v", messageID, err)

            // Determine retry strategy based on error type
            if isRetryableError(err) {
                log.Printf("Rejecting message %s for retry", messageID)
                return delivery.Nack(true) // Will be retried
            } else {
                log.Printf("Rejecting message %s permanently", messageID)
                return delivery.Reject(false) // Will go to dead letter queue
            }
        }

        log.Printf("Successfully processed order %s", messageID)
        return delivery.Ack()
    })

    if err != nil {
        log.Printf("Consumer error: %v", err)
    }
}

func processOrder(body []byte) error {
    // Your order processing logic here
    time.Sleep(100 * time.Millisecond) // Simulate processing
    return nil
}

func isRetryableError(err error) bool {
    // Determine if error is transient and should be retried
    // e.g., network timeouts, temporary database issues
    return true
}
```

🔝 [back to top](#production-features)

&nbsp;

### Dead Letter Queue Handling

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    admin := client.Admin()
    ctx := context.Background()

    // Set up dead letter queue topology
    err = admin.DeclareExchange(ctx, "orders.dlx", rabbitmq.ExchangeTypeDirect,
        rabbitmq.WithDurable(),
    )
    if err != nil {
        log.Fatal("Failed to declare DLX:", err)
    }

    _, err = admin.DeclareQueue(ctx, "orders.processing",
        rabbitmq.WithDurable(),
        rabbitmq.WithDeadLetterExchange("orders.dlx"),
        rabbitmq.WithDeadLetterRoutingKey("failed"),
        rabbitmq.WithMessageTTL(300000), // 5 minutes
        rabbitmq.WithMaxRetries(3),
    )
    if err != nil {
        log.Fatal("Failed to declare main queue:", err)
    }

    _, err = admin.DeclareQueue(ctx, "orders.failed",
        rabbitmq.WithDurable(),
    )
    if err != nil {
        log.Fatal("Failed to declare DLQ:", err)
    }

    err = admin.BindQueue(ctx, "orders.failed", "orders.dlx", "failed")
    if err != nil {
        log.Fatal("Failed to bind DLQ:", err)
    }

    // Consumer for processing failed messages
    consumer, err := client.NewConsumer()
    if err != nil {
        log.Fatal("Failed to create consumer:", err)
    }
    defer consumer.Close()

    // Monitor dead letter queue for failed messages
    go func() {
        consumer.Consume(ctx, "orders.failed", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
            log.Printf("Dead letter message received: %s", string(delivery.Body))

            // Log failure details for analysis
            headers := delivery.Headers
            if deathReason, ok := headers["x-first-death-reason"]; ok {
                log.Printf("Death reason: %v", deathReason)
            }

            // Send to monitoring/alerting system
            // alerting.SendFailureAlert(delivery)

            return delivery.Ack()
        })
    }()

    log.Println("Dead letter queue monitoring started")
    select {} // Keep running
}
```

🔝 [back to top](#production-features)

&nbsp;

### Consumer Retry

Header-based automatic retry mechanism that works across distributed consumers with support for both Quorum and Classic queues.

&nbsp;

**Key Features:**
- **Distributed-Safe**: Retry count travels with the message in headers
- **Quorum Queue Support**: Uses broker-tracked `x-delivery-count` header
- **Classic Queue Support**: Uses application-tracked `x-retry-count` header
- **Automatic DLX Routing**: Sends to Dead Letter Exchange after max retries
- **Configurable Backoff**: Optional delay between retry attempts

**How It Works:**

**Quorum Queues (Recommended):**
1. Message fails → Consumer calls `Nack(requeue=true)`
2. RabbitMQ increments `x-delivery-count` header automatically
3. Any consumer instance receives the message and checks the header
4. If count ≥ maxAttempts → `Nack(requeue=false)` → DLX

**Classic Queues (Fallback):**
1. Message fails → Consumer calls `Ack()` to remove original
2. Consumer republishes to tail of queue with `x-retry-count` incremented
3. Any consumer instance receives the message and checks the header
4. If count ≥ maxAttempts → `Nack(requeue=false)` → DLX

```go
package main

import (
    "context"
    "errors"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    admin := client.Admin()
    ctx := context.Background()

    // Set up Dead Letter Exchange and Queue
    err = admin.DeclareExchange(ctx, "orders.dlx", rabbitmq.ExchangeTypeDirect)
    if err != nil {
        log.Fatal("Failed to declare DLX:", err)
    }

    _, err = admin.DeclareQueue(ctx, "orders.dlq")
    if err != nil {
        log.Fatal("Failed to declare DLQ:", err)
    }

    err = admin.BindQueue(ctx, "orders.dlq", "orders.dlx", "failed")
    if err != nil {
        log.Fatal("Failed to bind DLQ:", err)
    }

    // Declare main queue with DLX configured (Quorum queue by default)
    _, err = admin.DeclareQueue(ctx, "orders.processing",
        rabbitmq.WithDeadLetter("orders.dlx", "failed"),
        rabbitmq.WithDeliveryLimit(3), // Quorum queue delivery limit
    )
    if err != nil {
        log.Fatal("Failed to declare queue:", err)
    }

    // Consumer with automatic retry (3 attempts, 1s backoff)
    consumer, err := client.NewConsumer(
        rabbitmq.WithConsumerRetry(3, 1*time.Second),
        rabbitmq.WithPrefetchCount(10),
    )
    if err != nil {
        log.Fatal("Failed to create consumer:", err)
    }
    defer consumer.Close()

    // Consume with automatic retry handling
    err = consumer.Consume(ctx, "orders.processing", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        log.Printf("Processing order: %s", delivery.MessageId)

        // Simulate processing that might fail
        if err := processOrder(delivery.Body); err != nil {
            log.Printf("Order processing failed: %v", err)
            return err // Consumer will automatically retry based on header count
        }

        log.Printf("Order processed successfully: %s", delivery.MessageId)
        return nil // Success - clears retry tracking
    })
    if err != nil {
        log.Fatal("Consumer error:", err)
    }
}

func processOrder(body []byte) error {
    // Simulate transient failures
    // In production, this could be database timeouts, API failures, etc.
    if time.Now().Unix()%3 == 0 {
        return errors.New("transient processing error")
    }
    return nil
}
```

**Example with Classic Queue:**

```go
// Declare classic queue with DLX
_, err = admin.DeclareQueue(ctx, "legacy.processing",
    rabbitmq.WithClassicQueue(), // Explicitly use classic queue
    rabbitmq.WithDeadLetter("orders.dlx", "failed"),
)
if err != nil {
    log.Fatal("Failed to declare classic queue:", err)
}

// Consumer works the same way - automatically detects queue type
consumer, err := client.NewConsumer(
    rabbitmq.WithConsumerRetry(3, 1*time.Second),
)
if err != nil {
    log.Fatal("Failed to create consumer:", err)
}
defer consumer.Close()

// Consume - retry logic adapts to classic queue automatically
err = consumer.Consume(ctx, "legacy.processing", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    // Same handler code - retry mechanism adapts automatically
    return processOrder(delivery.Body)
})
```

**Monitoring Retry Behavior:**

```go
err = consumer.Consume(ctx, "orders.processing", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    // Check if message has been retried
    retryCount := 0
    if delivery.Headers != nil {
        if count, ok := delivery.Headers["x-delivery-count"]; ok {
            // Quorum queue
            if c, ok := count.(int); ok {
                retryCount = c - 1
            }
        } else if count, ok := delivery.Headers["x-retry-count"]; ok {
            // Classic queue
            if c, ok := count.(int); ok {
                retryCount = c
            }
        }
    }

    log.Printf("Processing message %s (attempt %d)", delivery.MessageId, retryCount+1)

    if err := processOrder(delivery.Body); err != nil {
        log.Printf("Processing failed (attempt %d): %v", retryCount+1, err)
        return err
    }

    log.Printf("Processing succeeded on attempt %d", retryCount+1)
    return nil
})
```

🔝 [back to top](#production-features)

&nbsp;

### Message Reliability Features

- **Publisher Confirmations**: Broker acknowledgments for message delivery guarantees
- **Delivery Assurance**: Asynchronous confirmation tracking with callbacks
- **Publisher Retry**: Automatic re-publishing of nacked messages
- **Consumer Acknowledgments**: Manual message acknowledgment with retry control
- **Consumer Retry**: Header-based retry tracking across distributed consumers
- **Dead Letter Queues**: Manual configuration for failed message processing
- **Message Persistence**: Durable message storage surviving broker restarts
- **Delivery Tracking**: Comprehensive delivery status and retry monitoring
- **Mandatory Publishing**: Ensures messages reach bound queues

🔝 [back to top](#production-features)

&nbsp;

## Timeout Configuration

Comprehensive timeout controls for production reliability and predictable behavior.

&nbsp;

### Production Timeout Configuration

```go
package main

import (
    "time"
    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Comprehensive timeout configuration for production
    client, err := rabbitmq.NewClient(
        rabbitmq.FromEnv(),

        // Connection timeouts
        rabbitmq.WithDialTimeout(45*time.Second),          // TCP connection establishment
        rabbitmq.WithChannelTimeout(15*time.Second),       // AMQP channel creation
        rabbitmq.WithHeartbeat(30*time.Second),            // Connection keepalive

        // Reconnection behavior
        rabbitmq.WithReconnectDelay(10*time.Second),       // Initial reconnect delay
        rabbitmq.WithMaxReconnectAttempts(5),              // Limit reconnection attempts
    )
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Publisher with timeouts
    publisher, err := client.NewPublisher(
        rabbitmq.WithConfirmation(30*time.Second), // Publisher confirmation timeout
    )
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    // Consumer with timeouts
    consumer, err := client.NewConsumer(
        rabbitmq.WithMessageTimeout(10*time.Minute), // Message processing timeout
    )
    if err != nil {
        log.Fatal("Failed to create consumer:", err)
    }
    defer consumer.Close()
}
```

🔝 [back to top](#production-features)

&nbsp;

### Environment-Based Timeout Configuration

```bash
# Production timeout environment variables
export RABBITMQ_HOSTS=rabbit1.prod.com:5672,rabbit2.prod.com:5672

# Connection timeouts
export RABBITMQ_DIAL_TIMEOUT=45s
export RABBITMQ_CHANNEL_TIMEOUT=15s
export RABBITMQ_HEARTBEAT=30s

# Reconnection settings
export RABBITMQ_RECONNECT_DELAY=10s
export RABBITMQ_MAX_RECONNECT_ATTEMPTS=5

# Publisher settings
export RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT=30s
export RABBITMQ_PUBLISHER_SHUTDOWN_TIMEOUT=60s

# Consumer settings
export RABBITMQ_CONSUMER_MESSAGE_TIMEOUT=10m
export RABBITMQ_CONSUMER_SHUTDOWN_TIMEOUT=90s
```

🔝 [back to top](#production-features)

&nbsp;

### Timeout Best Practices

- **Connection Timeouts**: Set based on network latency and infrastructure
- **Publisher Confirmations**: Allow sufficient time for broker acknowledgments
- **Consumer Processing**: Timeout based on maximum expected processing time
- **Shutdown Timeouts**: Provide adequate time for graceful resource cleanup
- **Environment-Specific**: Different timeout values per deployment environment
- **Monitoring Integration**: Track timeout events for infrastructure optimization

🔝 [back to top](#production-features)

&nbsp;

## Advanced Security & Performance

Enterprise-grade message encryption, compression, and advanced performance optimizations for demanding production workloads.

&nbsp;

### Message Encryption

End-to-end message encryption with industry-standard AES-256-GCM for sensitive data protection.

```go
package main

import (
    "context"
    "crypto/rand"
    "log"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/encryption"
)

func main() {
    // Create client
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Create AES-256-GCM encryptor for sensitive data
    encryptionKey := make([]byte, 32) // 256-bit key
    if _, err := rand.Read(encryptionKey); err != nil {
        log.Fatal("Failed to generate encryption key:", err)
    }

    encryptor, err := encryption.NewAESGCMEncryptor(encryptionKey)
    if err != nil {
        log.Fatal("Failed to create encryptor:", err)
    }

    // Publisher with automatic encryption
    publisher, err := client.NewPublisher(
        rabbitmq.WithEncryption(encryptor),
        rabbitmq.WithConnectionName("secure-financial-service"),
    )
    if err != nil {
        log.Fatal("Failed to create encrypted publisher:", err)
    }
    defer publisher.Close()

    // Consumer with automatic decryption
    consumer, err := client.NewConsumer(
        rabbitmq.WithConsumerEncryption(encryptor),
    )
    if err != nil {
        log.Fatal("Failed to create encrypted consumer:", err)
    }
    defer consumer.Close()

    ctx := context.Background()

    // Publish sensitive financial data (automatically encrypted)
    sensitiveData := `{
        "account": "12345678901234567890",
        "amount": 50000.00,
        "currency": "USD",
        "customer_ssn": "123-45-6789",
        "routing_number": "021000021"
    }`

    message := rabbitmq.NewMessage([]byte(sensitiveData)).
        WithMessageID("secure-txn-001").
        WithPersistent()

    err = publisher.Publish(ctx, "", "financial.transactions", message)
    if err != nil {
        log.Fatal("Failed to publish encrypted message:", err)
    }

    log.Println("Sensitive financial data published with AES-256-GCM encryption")

    // Consume and automatically decrypt
    err = consumer.Consume(ctx, "financial.transactions", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        // Message is automatically decrypted before reaching this handler
        log.Printf("Received decrypted financial transaction: %s",
            string(delivery.Body[:50])+"...") // Truncate for logging security

        // Verify encryption header
        if encHeader, ok := delivery.Headers["x-encryption"]; ok {
            log.Printf("Message was encrypted with: %v", encHeader)
        }

        return delivery.Ack()
    })

    if err != nil {
        log.Printf("Consumer error: %v", err)
    }
}
```

🔝 [back to top](#production-features)

&nbsp;

### Message Compression

Intelligent message compression with configurable algorithms and thresholds for bandwidth optimization.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/compression"
)

func main() {
    // Create client
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Gzip compressor for large messages (>1KB threshold)
    gzipCompressor := compression.NewGzipCompressor(1024, compression.BestCompression)

    // Publisher with compression for large payloads
    publisher, err := client.NewPublisher(
        rabbitmq.WithCompression(gzipCompressor),
        rabbitmq.WithConnectionName("high-volume-data-service"),
    )
    if err != nil {
        log.Fatal("Failed to create compressing publisher:", err)
    }
    defer publisher.Close()

    // Consumer with automatic decompression
    consumer, err := client.NewConsumer(
        rabbitmq.WithConsumerCompression(gzipCompressor),
    )
    if err != nil {
        log.Fatal("Failed to create decompressing consumer:", err)
    }
    defer consumer.Close()

    ctx := context.Background()

    // Generate large payload (will be automatically compressed)
    largePayload := generateLargeDataset(50000) // 50KB of data

    originalSize := len(largePayload)
    log.Printf("Publishing large dataset: %d bytes", originalSize)

    message := rabbitmq.NewMessage(largePayload).
        WithMessageID("dataset-001").
        WithPersistent()

    err = publisher.Publish(ctx, "", "analytics.large-datasets", message)
    if err != nil {
        log.Fatal("Failed to publish large dataset:", err)
    }

    log.Println("Large dataset published with automatic compression")

    // Consume and measure compression effectiveness
    err = consumer.Consume(ctx, "analytics.large-datasets", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        // Message is automatically decompressed
        receivedSize := len(delivery.Body)

        // Check compression header for details
        if compHeader, ok := delivery.Headers["x-compression"]; ok {
            log.Printf("🗜️  Message compressed with: %v", compHeader)

            // Calculate compression ratio (this is illustrative)
            if originalSize > 0 {
                compressionRatio := float64(receivedSize) / float64(originalSize) * 100
                log.Printf("Compression effectiveness: %.1f%% of original size",
                    compressionRatio)
            }
        }

        log.Printf("📦 Received decompressed dataset: %d bytes", receivedSize)
        return delivery.Ack()
    })

    if err != nil {
        log.Printf("Consumer error: %v", err)
    }
}

func generateLargeDataset(size int) []byte {
    // Generate sample data for demonstration
    data := make([]byte, size)
    for i := 0; i < size; i++ {
        data[i] = byte('A' + (i % 26)) // Repeating pattern for good compression
    }
    return data
}
```

🔝 [back to top](#production-features)

&nbsp;

### Combined Encryption + Compression

Ultimate data protection and bandwidth optimization with combined encryption and compression.

```go
package main

import (
    "context"
    "crypto/rand"
    "log"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/compression"
    "github.com/cloudresty/go-rabbitmq/encryption"
)

func main() {
    // Create client
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Setup encryption
    encryptionKey := make([]byte, 32)
    rand.Read(encryptionKey)
    encryptor, _ := encryption.NewAESGCMEncryptor(encryptionKey)

    // Setup compression
    compressor := compression.NewGzipCompressor(512, compression.BestCompression)

    // Publisher with both encryption and compression
    publisher, err := client.NewPublisher(
        rabbitmq.WithEncryption(encryptor),       // Encrypt first
        rabbitmq.WithCompression(compressor),     // Then compress
        rabbitmq.WithConnectionName("secure-high-volume-service"),
    )
    if err != nil {
        log.Fatal("Failed to create secure+compressed publisher:", err)
    }
    defer publisher.Close()

    // Consumer with automatic decompression and decryption
    consumer, err := client.NewConsumer(
        rabbitmq.WithConsumerCompression(compressor), // Decompress first
        rabbitmq.WithConsumerEncryption(encryptor),   // Then decrypt
    )
    if err != nil {
        log.Fatal("Failed to create secure+decompressing consumer:", err)
    }
    defer consumer.Close()

    ctx := context.Background()

    // Sensitive large payload (medical records, financial reports, etc.)
    sensitiveReport := generateSensitiveReport(10000) // 10KB sensitive data

    log.Printf("Publishing sensitive report: %d bytes", len(sensitiveReport))

    message := rabbitmq.NewMessage(sensitiveReport).
        WithMessageID("medical-report-001").
        WithPersistent()

    err = publisher.Publish(ctx, "", "medical.encrypted-reports", message)
    if err != nil {
        log.Fatal("Failed to publish secure report:", err)
    }

    log.Println("Sensitive report published with encryption + compression")

    // Consume with automatic security processing
    err = consumer.Consume(ctx, "medical.encrypted-reports", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        // Message is automatically decompressed and decrypted
        headers := delivery.Headers

        log.Printf("Security headers - Encryption: %v, Compression: %v",
            headers["x-encryption"], headers["x-compression"])

        log.Printf("Received secure report: %d bytes (processed)", len(delivery.Body))

        // Process sensitive data securely
        // processSecureMedicalReport(delivery.Body)

        return delivery.Ack()
    })

    if err != nil {
        log.Printf("Consumer error: %v", err)
    }
}

func generateSensitiveReport(size int) []byte {
    // Generate sample sensitive data
    return []byte(fmt.Sprintf(`{
        "patient_id": "P123456789",
        "medical_data": "%s",
        "diagnosis": "Sensitive medical information",
        "treatment_plan": "Confidential treatment details",
        "insurance": "Private insurance information"
    }`, string(make([]byte, size-200)))) // Pad to requested size
}
```

🔝 [back to top](#production-features)

&nbsp;

### Distributed Tracing

Enterprise observability with OpenTelemetry-compatible distributed tracing across message flows.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
)

// CustomTracer implements the rabbitmq.Tracer interface
type CustomTracer struct {
    tracer trace.Tracer
}

func (t *CustomTracer) StartSpan(ctx context.Context, operation string) (context.Context, rabbitmq.Span) {
    ctx, span := t.tracer.Start(ctx, operation)
    return ctx, &CustomSpan{span: span}
}

// CustomSpan implements the rabbitmq.Span interface
type CustomSpan struct {
    span trace.Span
}

func (s *CustomSpan) SetAttribute(key string, value interface{}) {
    s.span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
}

func (s *CustomSpan) SetStatus(code rabbitmq.SpanStatusCode, description string) {
    var otelCode codes.Code
    switch code {
    case rabbitmq.SpanStatusOK:
        otelCode = codes.Ok
    case rabbitmq.SpanStatusError:
        otelCode = codes.Error
    default:
        otelCode = codes.Unset
    }
    s.span.SetStatus(otelCode, description)
}

func (s *CustomSpan) End() {
    s.span.End()
}

func main() {
    // Initialize OpenTelemetry tracer
    tracer := otel.Tracer("rabbitmq-service")
    customTracer := &CustomTracer{tracer: tracer}

    // Create client with distributed tracing
    client, err := rabbitmq.NewClient(
        rabbitmq.FromEnv(),
        rabbitmq.WithTracing(customTracer),
        rabbitmq.WithConnectionName("traced-order-service"),
    )
    if err != nil {
        log.Fatal("Failed to create traced client:", err)
    }
    defer client.Close()

    // Publisher with distributed tracing
    publisher, err := client.NewPublisher()
    if err != nil {
        log.Fatal("Failed to create traced publisher:", err)
    }
    defer publisher.Close()

    // Consumer with distributed tracing
    consumer, err := client.NewConsumer()
    if err != nil {
        log.Fatal("Failed to create traced consumer:", err)
    }
    defer consumer.Close()

    // Create root span for business operation
    ctx := context.Background()
    ctx, span := tracer.Start(ctx, "process-customer-order")
    defer span.End()

    // Publish with trace context propagation
    orderData := `{"order_id": "ORDER-001", "customer_id": "CUST-123", "amount": 299.99}`

    message := rabbitmq.NewMessage([]byte(orderData)).
        WithMessageID("ORDER-001").
        WithPersistent()

    err = publisher.Publish(ctx, "", "orders.processing", message)
    if err != nil {
        span.SetStatus(codes.Error, "Failed to publish order")
        log.Fatal("Failed to publish traced message:", err)
    }

    span.SetAttributes(
        attribute.String("order.id", "ORDER-001"),
        attribute.String("queue", "orders.processing"),
    )
    span.SetStatus(codes.Ok, "Order published successfully")

    log.Println("Order published with distributed tracing")

    // Consume with trace continuation
    err = consumer.Consume(ctx, "orders.processing", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        // Extract trace context from message headers
        // Span is automatically created and linked to the publishing span

        _, processingSpan := tracer.Start(ctx, "process-order-payment")
        defer processingSpan.End()

        // Add business attributes to the span
        processingSpan.SetAttributes(
            attribute.String("message.id", delivery.MessageId),
            attribute.Int("message.size", len(delivery.Body)),
            attribute.String("queue", "orders.processing"),
        )

        log.Printf("💳 Processing order with trace: %s", delivery.MessageId)

        // Simulate payment processing
        if processPayment(ctx, delivery.Body) {
            processingSpan.SetStatus(codes.Ok, "Payment processed successfully")
            log.Println("Payment processed successfully")
        } else {
            processingSpan.SetStatus(codes.Error, "Payment processing failed")
            log.Println("Payment processing failed")
            return delivery.Nack(true) // Will be retried with new span
        }

        return delivery.Ack()
    })

    if err != nil {
        log.Printf("Consumer error: %v", err)
    }
}

func processPayment(ctx context.Context, orderData []byte) bool {
    // Simulate payment processing with nested span
    tracer := otel.Tracer("payment-processor")
    _, span := tracer.Start(ctx, "charge-credit-card")
    defer span.End()

    // Simulate processing time
    time.Sleep(100 * time.Millisecond)

    // Simulate success/failure
    return true
}
```

🔝 [back to top](#production-features)

&nbsp;

### Advanced Retry Policies

Sophisticated retry mechanisms with exponential backoff, circuit breakers, and dead letter handling.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Define sophisticated retry policies
    exponentialBackoff := rabbitmq.ExponentialBackoff{
        InitialDelay: 1 * time.Second,
        MaxDelay:     30 * time.Second,
        MaxAttempts:  5,
        Multiplier:   2.0,
    }

    // Publisher with retry policy for transient failures
    publisher, err := client.NewPublisher(
        rabbitmq.WithRetryPolicy(exponentialBackoff),
        rabbitmq.WithConnectionName("resilient-service"),
    )
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    // Consumer with intelligent retry handling
    consumer, err := client.NewConsumer(
        rabbitmq.WithConsumerRetryPolicy(exponentialBackoff),
        rabbitmq.WithPrefetchCount(1), // Process one at a time for retry accuracy
    )
    if err != nil {
        log.Fatal("Failed to create consumer:", err)
    }
    defer consumer.Close()

    ctx := context.Background()

    // Publish critical business events
    for i := 0; i < 10; i++ {
        eventData := fmt.Sprintf(`{
            "event_id": "EVENT-%03d",
            "timestamp": "%s",
            "data": "Critical business event data"
        }`, i, time.Now().Format(time.RFC3339))

        message := rabbitmq.NewMessage([]byte(eventData)).
            WithMessageID(fmt.Sprintf("EVENT-%03d", i)).
            WithPersistent()

        err = publisher.Publish(ctx, "", "critical.events", message)
        if err != nil {
            log.Printf("Failed to publish event %d after retries: %v", i, err)
        } else {
            log.Printf("Published critical event %d", i)
        }
    }

    // Consume with sophisticated error handling
    err = consumer.Consume(ctx, "critical.events", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
        messageID := delivery.MessageId
        attempt := getRetryAttempt(delivery) // Extract from headers

        log.Printf("Processing event %s (attempt %d)", messageID, attempt)

        // Simulate different types of failures
        if err := processBusinessEvent(delivery.Body, attempt); err != nil {
            log.Printf("Processing failed for %s: %v", messageID, err)

            // Determine retry strategy based on error type
            switch err.Error() {
            case "transient_error":
                log.Printf("Transient error for %s, will retry", messageID)
                return delivery.Nack(true) // Will be retried
            case "permanent_error":
                log.Printf("Permanent error for %s, sending to DLQ", messageID)
                return delivery.Reject(false) // Will go to dead letter queue
            default:
                // Unknown error, retry with exponential backoff
                if attempt < 3 {
                    delay := time.Duration(attempt) * time.Second
                    time.Sleep(delay)
                    return delivery.Nack(true)
                } else {
                    log.Printf("Max retries exceeded for %s", messageID)
                    return delivery.Reject(false)
                }
            }
        }

        log.Printf("Successfully processed event %s", messageID)
        return delivery.Ack()
    })

    if err != nil {
        log.Printf("Consumer error: %v", err)
    }
}

func processBusinessEvent(data []byte, attempt int) error {
    // Simulate business logic with various failure scenarios
    switch attempt {
    case 1:
        return fmt.Errorf("transient_error")
    case 2:
        return fmt.Errorf("network_timeout")
    case 3:
        if len(data) < 10 {
            return fmt.Errorf("permanent_error")
        }
        return nil // Success
    default:
        return nil // Success
    }
}

func getRetryAttempt(delivery *rabbitmq.Delivery) int {
    headers := delivery.Headers
    if retryCount, ok := headers["x-retry-count"]; ok {
        if count, ok := retryCount.(int); ok {
            return count + 1
        }
    }
    return 1
}
```

🔝 [back to top](#production-features)

&nbsp;

### Advanced Security Features

- **AES-256-GCM Encryption**: Industry-standard encryption for sensitive data
- **Configurable Compression**: Gzip and Zlib compression with custom thresholds
- **Combined Security**: Encryption + compression for ultimate data protection
- **Distributed Tracing**: OpenTelemetry-compatible tracing across message flows
- **Advanced Retry Policies**: Exponential backoff, linear backoff, and custom policies
- **Circuit Breaker Integration**: Prevent cascade failures in distributed systems
- **Message Integrity**: Built-in message validation and corruption detection

🔝 [back to top](#production-features)

&nbsp;

## Production Checklist

Essential items for production deployment and operational excellence.

&nbsp;

### Core Production Features

**Connection Resilience:**

- Multi-host failover configuration
- Auto-reconnection with exponential backoff
- Connection health monitoring
- Heartbeat monitoring and timeout configuration
- TLS/SSL encryption for secure connections

**Message Reliability:**

- Publisher confirmations for critical messages
- Consumer acknowledgments with proper error handling
- Dead letter queue configuration
- Message persistence for durability
- Duplicate message handling strategies

**Performance & Scalability:**

- Connection pooling for high-throughput applications
- Prefetch configuration for optimal consumer performance
- Batch publishing for efficient message throughput
- Environment-based configuration management
- ULID message IDs for high-performance sorting
- Message compression (Gzip/Zlib) for bandwidth optimization
- Advanced retry policies with exponential backoff

**Operational Excellence:**

- Graceful shutdown with in-flight message protection
- Structured logging with correlation IDs
- Health check endpoints for monitoring
- Metrics collection and observability
- Connection name identification for debugging
- Distributed tracing with OpenTelemetry compatibility
- Advanced security with AES-256-GCM encryption

🔝 [back to top](#production-features)

&nbsp;

### Additional Recommended Items

**Monitoring & Alerting:**

- Set up RabbitMQ cluster monitoring
- Configure alerts for connection failures
- Monitor queue lengths and consumer lag
- Track message throughput and latency
- Set up dead letter queue monitoring

**Security & Compliance:**

- Implement proper authentication and authorization
- Use TLS encryption for all connections
- Configure end-to-end message encryption for sensitive data
- Regular security audits and vulnerability assessments
- Audit logging for message processing
- Compliance with data protection regulations

**Disaster Recovery:**

- Regular backup of RabbitMQ configuration and messages
- Test failover scenarios and recovery procedures
- Document runbooks for common operational issues
- Implement cross-region disaster recovery
- Regular disaster recovery testing

**Performance Optimization:**

- Load testing under expected production volumes
- Performance profiling and optimization
- Resource usage monitoring and capacity planning
- Network latency optimization
- Database query optimization for message processing
- Message compression for large payloads
- Connection pooling for high-throughput scenarios

🔝 [back to top](#production-features)

&nbsp;

---

&nbsp;

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
