# Shutdown Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [Shutdown Package](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides the complete API reference for the `shutdown` sub-package. The shutdown package provides coordinated graceful shutdown management for RabbitMQ components and other closable resources, including signal management, timeout control, and tracking of in-flight operations.

&nbsp;

## Configuration Functions

| Function | Description |
|----------|-------------|
| `DefaultShutdownConfig()` | Returns sensible default shutdown configuration with 30s timeout, 5s signal timeout, and 10s drain time |

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

## Constructor Functions

| Function | Description |
|----------|-------------|
| `NewShutdownManager(config ShutdownConfig)` | Creates a new shutdown manager with the specified configuration |
| `NewInFlightTracker()` | Creates a new in-flight operation tracker for monitoring active operations during shutdown |

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

## ShutdownManager Methods

| Function | Description |
|----------|-------------|
| `Register(component Closable)` | Registers a component for coordinated shutdown |
| `SetupSignalHandler()` | Sets up OS signal handling for graceful shutdown on SIGINT/SIGTERM |
| `Shutdown()` | Initiates graceful shutdown of all registered components |
| `Wait()` | Blocks until shutdown is complete |
| `WaitWithContext(ctx context.Context)` | Waits for shutdown completion with context cancellation support |
| `IsShutdown()` | Returns true if shutdown has been initiated |
| `GetInFlightTracker()` | Returns the in-flight operation tracker for monitoring active operations |

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

## Interface Compliance Methods

| Function | Description |
|----------|-------------|
| `RegisterComponent(component rabbitmq.Closable)` | Registers a component using the simplified GracefulShutdown interface |
| `ShutdownGracefully(ctx context.Context)` | Initiates and waits for graceful shutdown with context support |
| `SetupSignalHandling()` | Sets up signal handling and returns a channel that closes when shutdown begins |
| `IsShutdownComplete()` | Returns true if shutdown has completed |

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

## InFlightTracker Methods

| Function | Description |
|----------|-------------|
| `Start()` | Marks the start of an operation, returns false if tracker is closed |
| `Done()` | Marks the completion of an operation |
| `Close()` | Prevents new operations and waits for existing ones to complete |
| `CloseWithTimeout(timeout time.Duration)` | Waits for in-flight operations with a timeout |
| `IsClosed()` | Returns true if the tracker is closed to new operations |

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

## Internal Methods

| Function | Description |
|----------|-------------|
| `shutdownComponents()` | Shuts down all registered components (called internally by Shutdown) |

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

## Field Helper Functions

| Function | Description |
|----------|-------------|
| `StringField(key, value string)` | Creates a string field for structured logging |
| `IntField(key string, value int)` | Creates an integer field for structured logging |
| `DurationField(key string, value time.Duration)` | Creates a duration field for structured logging |

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

## Logger Interface Methods

| Function | Description |
|----------|-------------|
| `Debug(msg string, fields ...Field)` | Logs debug-level messages with structured fields |
| `Info(msg string, fields ...Field)` | Logs info-level messages with structured fields |
| `Warn(msg string, fields ...Field)` | Logs warning-level messages with structured fields |
| `Error(msg string, fields ...Field)` | Logs error-level messages with structured fields |

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

## Types and Structures

| Type | Description |
|------|-------------|
| `ShutdownManager` | Main coordinator for graceful shutdown of multiple components |
| `InFlightTracker` | Tracks in-flight operations to ensure they complete before shutdown |
| `ShutdownConfig` | Configuration structure for shutdown behavior and timeouts |
| `Closable` | Interface for components that can be gracefully closed |
| `Logger` | Interface for structured logging during shutdown events |
| `Field` | Interface for structured log fields |
| `SimpleField` | Basic implementation of the Field interface |
| `NoOpLogger` | No-operation logger implementation for when logging is not needed |

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

## Usage Examples

&nbsp;

### Basic Shutdown Management

```go
import "github.com/cloudresty/go-rabbitmq/shutdown"

// Create shutdown manager with default configuration
config := shutdown.DefaultShutdownConfig()
shutdownManager := shutdown.NewShutdownManager(config)

// Register components for shutdown
shutdownManager.Register(client)
shutdownManager.Register(publisher)
shutdownManager.Register(consumer)

// Setup signal handling
signalCh := shutdownManager.SetupSignalHandler()

// Wait for shutdown signal
go func() {
    <-signalCh
    log.Println("Shutdown signal received")
    shutdownManager.Shutdown()
}()

// Wait for shutdown to complete
shutdownManager.Wait()
log.Println("All components shut down gracefully")
```

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

### Custom Shutdown Configuration

```go
// Create custom shutdown configuration
config := shutdown.ShutdownConfig{
    Timeout:           45 * time.Second, // Total shutdown timeout
    SignalTimeout:     10 * time.Second, // Grace period after signal
    GracefulDrainTime: 15 * time.Second, // Time for in-flight operations
    Logger:            myCustomLogger,   // Custom logger
}

shutdownManager := shutdown.NewShutdownManager(config)
```

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

### In-Flight Operation Tracking

```go
// Get the in-flight tracker
tracker := shutdownManager.GetInFlightTracker()

// Track operations
func processMessage(ctx context.Context, delivery *rabbitmq.Delivery) error {
    // Start tracking this operation
    if !tracker.Start() {
        return errors.New("shutdown in progress, not accepting new operations")
    }
    defer tracker.Done() // Mark operation complete

    // Process the message
    return processBusinessLogic(delivery)
}

// In another goroutine, handle shutdown
go func() {
    <-shutdownSignal

    log.Println("Shutdown initiated, waiting for in-flight operations...")

    // Close tracker and wait for operations to complete
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := tracker.CloseWithTimeout(30 * time.Second); err != nil {
        log.Printf("Timeout waiting for in-flight operations: %v", err)
    } else {
        log.Println("All in-flight operations completed")
    }
}()
```

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

### Simplified Interface Usage

```go
// Use the simplified GracefulShutdown interface
var gracefulShutdown rabbitmq.GracefulShutdown = shutdownManager

// Register components
gracefulShutdown.RegisterComponent(client)
gracefulShutdown.RegisterComponent(publisher)

// Setup signal handling that automatically triggers shutdown
shutdownComplete := gracefulShutdown.SetupSignalHandling()

// Wait for shutdown to complete
<-shutdownComplete
log.Println("Graceful shutdown completed")
```

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

### Custom Logger Implementation

```go
// Implement custom logger
type MyLogger struct {
    logger *log.Logger
}

func (l *MyLogger) Info(msg string, fields ...shutdown.Field) {
    var fieldStrs []string
    for _, field := range fields {
        fieldStrs = append(fieldStrs, fmt.Sprintf("%s=%v", field.Key(), field.Value()))
    }
    l.logger.Printf("INFO: %s %s", msg, strings.Join(fieldStrs, " "))
}

// ... implement other logger methods

// Use custom logger
config := shutdown.ShutdownConfig{
    Timeout: 30 * time.Second,
    Logger:  &MyLogger{logger: log.New(os.Stdout, "[SHUTDOWN] ", log.LstdFlags)},
}
```

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

### Context-Based Shutdown

```go
// Use context for shutdown coordination
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

// Initiate shutdown
shutdownManager.Shutdown()

// Wait with context
if err := shutdownManager.WaitWithContext(ctx); err != nil {
    log.Printf("Shutdown did not complete within timeout: %v", err)
} else {
    log.Println("Shutdown completed successfully")
}
```

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

### Complete Application Integration

```go
func main() {
    // Setup shutdown management
    config := shutdown.DefaultShutdownConfig()
    config.Logger = myLogger
    shutdownManager := shutdown.NewShutdownManager(config)

    // Create RabbitMQ components
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal(err)
    }

    publisher, err := client.NewPublisher()
    if err != nil {
        log.Fatal(err)
    }

    consumer, err := client.NewConsumer()
    if err != nil {
        log.Fatal(err)
    }

    // Register all components
    shutdownManager.Register(consumer)
    shutdownManager.Register(publisher)
    shutdownManager.Register(client)

    // Setup signal handling
    signalCh := shutdownManager.SetupSignalHandler()

    // Start application logic
    go startConsumers(consumer, shutdownManager.GetInFlightTracker())
    go startPublishers(publisher)

    // Wait for shutdown signal
    <-signalCh
    log.Println("Shutdown signal received, initiating graceful shutdown...")

    // Trigger shutdown
    shutdownManager.Shutdown()

    // Wait for completion
    shutdownManager.Wait()
    log.Println("Application shutdown complete")
}
```

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

## Best Practices

1. **Signal Handling**: Always setup signal handlers to gracefully handle SIGINT and SIGTERM
2. **Component Registration**: Register components in reverse order of their dependencies (dependents first)
3. **In-Flight Tracking**: Use InFlightTracker for long-running operations to ensure they complete before shutdown
4. **Timeout Configuration**: Set appropriate timeouts based on your application's needs and SLA requirements
5. **Logging**: Use structured logging to monitor shutdown progress and identify issues
6. **Context Usage**: Use context-based shutdown methods for better timeout and cancellation control
7. **Error Handling**: Handle shutdown errors appropriately, especially timeout scenarios
8. **Testing**: Test shutdown scenarios including timeout cases and signal handling

🔝 [back to top](#shutdown-package-api-reference)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
