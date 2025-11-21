# Shutdown Package

[Home](../README.md) &nbsp;/&nbsp; Shutdown Package

&nbsp;

This package provides comprehensive graceful shutdown management for RabbitMQ applications and other closable resources. It handles signal management, timeout control, in-flight operation tracking, and coordinated component shutdown.

&nbsp;

## Features

- **Signal Handling**: Automatic handling of SIGINT, SIGTERM, and SIGHUP signals
- **Component Management**: Register and coordinate shutdown of multiple components
- **In-Flight Tracking**: Track and wait for ongoing operations to complete
- **Timeout Control**: Configurable timeouts for shutdown operations
- **Concurrent Shutdown**: Parallel shutdown of components for faster completion
- **Structured Logging**: Optional structured logging of shutdown events
- **Context Support**: Context-aware operations with cancellation support

🔝 [back to top](#shutdown-package)

&nbsp;

## Basic Usage

### Simple Shutdown Manager

```go
import (
    "github.com/cloudresty/go-rabbitmq/shutdown"
    "time"
)

// Create shutdown manager with default configuration
config := shutdown.DefaultShutdownConfig()
shutdownManager := shutdown.NewShutdownManager(config)

// Register components that need graceful shutdown
shutdownManager.Register(publisher)
shutdownManager.Register(consumer)
shutdownManager.Register(connection)

// Setup signal handling
shutdownManager.SetupSignalHandler()

// Your application logic here...

// Wait for shutdown to complete
shutdownManager.Wait()
```

🔝 [back to top](#shutdown-package)

&nbsp;

### Custom Configuration

```go
config := shutdown.ShutdownConfig{
    Timeout:           time.Minute,     // 1 minute total shutdown timeout
    SignalTimeout:     time.Second * 10, // 10 seconds after signal
    GracefulDrainTime: time.Second * 30, // 30 seconds for in-flight operations
    Logger:            myCustomLogger,    // Optional custom logger
}

shutdownManager := shutdown.NewShutdownManager(config)
```

🔝 [back to top](#shutdown-package)

&nbsp;

### With Custom Logger

```go
// Implement the Logger interface
type MyLogger struct{}

func (l MyLogger) Debug(msg string, fields ...shutdown.Field) {
    // Your debug logging implementation
}

func (l MyLogger) Info(msg string, fields ...shutdown.Field) {
    // Your info logging implementation
}

func (l MyLogger) Warn(msg string, fields ...shutdown.Field) {
    // Your warning logging implementation
}

func (l MyLogger) Error(msg string, fields ...shutdown.Field) {
    // Your error logging implementation
}

// Use custom logger
config := shutdown.DefaultShutdownConfig()
config.Logger = MyLogger{}
shutdownManager := shutdown.NewShutdownManager(config)
```

🔝 [back to top](#shutdown-package)

&nbsp;

## Advanced Usage

### In-Flight Operation Tracking

```go
// Get the in-flight tracker
tracker := shutdownManager.GetInFlightTracker()

// Before starting an operation
if tracker.Start() {
    defer tracker.Done() // Ensure this is called when operation completes

    // Perform your operation
    processMessage(message)
} else {
    // Shutdown is in progress, don't start new operations
    return ErrShutdownInProgress
}
```

🔝 [back to top](#shutdown-package)

&nbsp;

### Manual Shutdown

```go
// Trigger shutdown programmatically
go func() {
    time.Sleep(5 * time.Minute) // Some condition
    shutdownManager.Shutdown()
}()

// Wait for shutdown with context
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := shutdownManager.WaitWithContext(ctx); err != nil {
    log.Printf("Shutdown wait interrupted: %v", err)
}
```

🔝 [back to top](#shutdown-package)

&nbsp;

### Checking Shutdown Status

```go
if shutdownManager.IsShutdown() {
    // Shutdown has been initiated
    return ErrShutdownInProgress
}

// Safe to continue with operations
```

🔝 [back to top](#shutdown-package)

&nbsp;

## Integration Patterns

### Web Server Integration

```go
func runWebServer() {
    // Setup shutdown manager
    config := shutdown.DefaultShutdownConfig()
    shutdownManager := shutdown.NewShutdownManager(config)

    // Create HTTP server
    server := &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }

    // Register server for shutdown
    shutdownManager.Register(&serverCloser{server})

    // Setup signal handling
    shutdownManager.SetupSignalHandler()

    // Start server
    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Printf("Server error: %v", err)
        }
    }()

    // Wait for shutdown
    shutdownManager.Wait()
}

// Custom closer for HTTP server
type serverCloser struct {
    server *http.Server
}

func (sc *serverCloser) Close() error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    return sc.server.Shutdown(ctx)
}
```

🔝 [back to top](#shutdown-package)

&nbsp;

### Database Connection Pool

```go
// Custom closer for database connections
type dbCloser struct {
    db *sql.DB
}

func (dc *dbCloser) Close() error {
    return dc.db.Close()
}

// Register database for shutdown
shutdownManager.Register(&dbCloser{db})
```

🔝 [back to top](#shutdown-package)

&nbsp;

### Worker Pool Integration

```go
type workerPool struct {
    workers   []Worker
    shutdown  chan struct{}
    wg        sync.WaitGroup
    tracker   *shutdown.InFlightTracker
}

func (wp *workerPool) Start(shutdownManager *shutdown.ShutdownManager) {
    wp.tracker = shutdownManager.GetInFlightTracker()

    for _, worker := range wp.workers {
        wp.wg.Add(1)
        go func(w Worker) {
            defer wp.wg.Done()
            wp.runWorker(w)
        }(worker)
    }
}

func (wp *workerPool) runWorker(worker Worker) {
    for {
        select {
        case job := <-worker.JobChan():
            // Track in-flight operation
            if wp.tracker.Start() {
                wp.processJob(job)
                wp.tracker.Done()
            } else {
                // Shutdown in progress, reject job
                job.Reject()
            }
        case <-wp.shutdown:
            return
        }
    }
}

func (wp *workerPool) Close() error {
    close(wp.shutdown)
    wp.wg.Wait()
    return nil
}
```

🔝 [back to top](#shutdown-package)

&nbsp;

## Best Practices

### 1. Component Registration Order

Register components in reverse dependency order (most dependent first):

```go
// Register in order of dependency (reverse of startup order)
shutdownManager.Register(httpServer)    // Depends on services
shutdownManager.Register(messageQueue)  // Depends on database
shutdownManager.Register(database)      // Independent
```

🔝 [back to top](#shutdown-package)

&nbsp;

### 2. Timeout Configuration

Set appropriate timeouts based on your application's needs:

```go
config := shutdown.ShutdownConfig{
    Timeout:           time.Minute * 2,  // Overall timeout
    SignalTimeout:     time.Second * 15, // Grace period
    GracefulDrainTime: time.Minute,      // Drain time
}
```

🔝 [back to top](#shutdown-package)

&nbsp;

### 3. In-Flight Operation Patterns

Always use defer for tracking completion:

```go
if tracker.Start() {
    defer tracker.Done() // Critical: always call Done()

    // Your operation here
    return processMessage(msg)
}
```

🔝 [back to top](#shutdown-package)

&nbsp;

### 4. Error Handling

Handle shutdown errors gracefully:

```go
type resilientCloser struct {
    component Component
    logger    Logger
}

func (rc *resilientCloser) Close() error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := rc.component.Shutdown(ctx); err != nil {
        rc.logger.Warn("Component shutdown failed",
            shutdown.StringField("error", err.Error()))
        // Don't return error to avoid blocking other shutdowns
        return nil
    }
    return nil
}
```

🔝 [back to top](#shutdown-package)

&nbsp;

### 5. Testing Shutdown Logic

Test shutdown behavior in your applications:

```go
func TestGracefulShutdown(t *testing.T) {
    config := shutdown.DefaultShutdownConfig()
    config.Timeout = time.Second // Short timeout for testing

    manager := shutdown.NewShutdownManager(config)

    // Register test components
    manager.Register(&testComponent{})

    // Trigger shutdown
    go manager.Shutdown()

    // Verify shutdown completes within timeout
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    if err := manager.WaitWithContext(ctx); err != nil {
        t.Errorf("Shutdown timeout: %v", err)
    }
}
```

🔝 [back to top](#shutdown-package)

&nbsp;

## Error Handling

The shutdown package handles various error scenarios:

🔝 [back to top](#shutdown-package)

&nbsp;

### Component Shutdown Errors

```go
// Components that fail to shutdown don't block others
type faultyComponent struct{}

func (fc *faultyComponent) Close() error {
    return errors.New("simulated failure")
}

// This won't prevent other components from shutting down
manager.Register(&faultyComponent{})
```

🔝 [back to top](#shutdown-package)

&nbsp;

### Timeout Handling

```go
// Shutdown will timeout after configured duration
config := shutdown.ShutdownConfig{
    Timeout: time.Second * 5, // Short timeout
}

manager := shutdown.NewShutdownManager(config)
// If shutdown takes longer than 5 seconds, it will timeout
```

🔝 [back to top](#shutdown-package)

&nbsp;

### Signal Handling

```go
// Automatic handling of common shutdown signals
signals := manager.SetupSignalHandler()

// You can also monitor signals manually
go func() {
    sig := <-signals
    log.Printf("Received signal: %s", sig)
    // Shutdown is automatically triggered
}()
```

🔝 [back to top](#shutdown-package)

&nbsp;

## Performance Considerations

- **Concurrent Shutdown**: Components shut down in parallel for faster completion
- **Resource Cleanup**: Automatic cleanup of internal resources
- **Memory Efficiency**: Minimal memory overhead for tracking operations
- **Signal Handling**: Efficient signal handling with minimal goroutines

🔝 [back to top](#shutdown-package)

&nbsp;

## Troubleshooting

### Common Issues

- **Shutdown Timeout**

```text
WARN: Graceful shutdown timeout exceeded
```

Solution: Increase timeout or optimize component shutdown logic.

🔝 [back to top](#shutdown-package)

&nbsp;

- **Component Registration After Shutdown**

```go
if !manager.IsShutdown() {
    manager.Register(component)
}
```

🔝 [back to top](#shutdown-package)

&nbsp;

- **In-Flight Operation Leaks**

```go
// Always use defer
if tracker.Start() {
    defer tracker.Done() // Don't forget this!
    // ... operation ...
}
```

🔝 [back to top](#shutdown-package)

&nbsp;

For more examples and patterns, see the `examples/` directory in the main repository.

🔝 [back to top](#shutdown-package)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
