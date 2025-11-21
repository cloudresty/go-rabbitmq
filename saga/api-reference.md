# Saga Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [Saga Package](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides the complete API reference for the `saga` sub-package. The saga package provides distributed transaction support using the Saga pattern, allowing you to coordinate complex workflows across multiple services with compensating actions.

&nbsp;

## Manager Functions

| Function | Description |
|----------|-------------|
| `NewManager(client *rabbitmq.Client, store Store, config Config)` | Creates a new saga manager with the specified client, persistence store, and configuration |

🔝 [back to top](#saga-package-api-reference)

&nbsp;

## Manager Methods

| Function | Description |
|----------|-------------|
| `Start(ctx context.Context, name string, steps []Step, sagaContext map[string]any)` | Starts a new saga with the given name, steps, and context data |
| `Compensate(ctx context.Context, sagaID string)` | Initiates compensation for a failed saga, executing compensating actions for completed steps |
| `Get(ctx context.Context, sagaID string)` | Retrieves a saga by its ID from the persistence store |
| `ListActive(ctx context.Context)` | Returns a list of all currently active sagas |
| `Close()` | Gracefully closes the saga manager and its associated resources |

🔝 [back to top](#saga-package-api-reference)

&nbsp;

## Internal Manager Methods

| Function | Description |
|----------|-------------|
| `publishStep(ctx context.Context, saga *Saga, step *Step)` | Publishes a step execution message (called internally during saga execution) |
| `publishCompensation(ctx context.Context, saga *Saga, step *Step)` | Publishes a compensation message (called internally during saga compensation) |

🔝 [back to top](#saga-package-api-reference)

&nbsp;

## Store Functions

| Function | Description |
|----------|-------------|
| `NewInMemoryStore()` | Creates a new in-memory saga store for testing and development purposes |

🔝 [back to top](#saga-package-api-reference)

&nbsp;

## Store Interface Methods

| Function | Description |
|----------|-------------|
| `SaveSaga(ctx context.Context, saga *Saga)` | Persists a saga to the store |
| `LoadSaga(ctx context.Context, sagaID string)` | Loads a saga from the store by its ID |
| `DeleteSaga(ctx context.Context, sagaID string)` | Removes a saga from the store |
| `ListActiveSagas(ctx context.Context)` | Returns all active sagas from the store |
| `UpdateSagaStep(ctx context.Context, sagaID, stepID string, status State, output map[string]any, errorMsg string)` | Atomically updates a specific step within a saga |
| `UpdateSagaState(ctx context.Context, sagaID string, state State)` | Atomically updates the overall saga state |

🔝 [back to top](#saga-package-api-reference)

&nbsp;

## Saga Methods

| Function | Description |
|----------|-------------|
| `IsCompleted()` | Returns true if the saga has completed successfully |
| `IsFailed()` | Returns true if the saga has failed |
| `IsCompensating()` | Returns true if the saga is currently compensating (rolling back) |
| `IsCompensated()` | Returns true if the saga has been fully compensated |
| `GetCompletedSteps()` | Returns a slice of all completed steps in the saga |
| `GetFailedSteps()` | Returns a slice of all failed steps in the saga |

🔝 [back to top](#saga-package-api-reference)

&nbsp;

## Types and Structures

| Type | Description |
|------|-------------|
| `Manager` | Main saga orchestrator that manages saga execution and coordination |
| `Saga` | Represents a distributed transaction saga with steps, state, and context |
| `Step` | Individual step within a saga containing action, compensation, and execution details |
| `Store` | Interface for saga persistence with atomic operations for concurrency safety |
| `InMemoryStore` | In-memory implementation of the Store interface for testing |
| `State` | Enumeration of possible saga and step states |
| `Config` | Configuration structure for saga manager setup |

🔝 [back to top](#saga-package-api-reference)

&nbsp;

## Constants and States

| Constant | Description |
|----------|-------------|
| `StateStarted` | Saga or step has been started |
| `StateCompleted` | Saga or step has completed successfully |
| `StateFailed` | Saga or step has failed |
| `StateCompensating` | Saga is currently executing compensating actions |
| `StateCompensated` | Saga has been fully compensated (rolled back) |

🔝 [back to top](#saga-package-api-reference)

&nbsp;

## Usage Examples

&nbsp;

### Creating and Starting a Saga

```go
import "github.com/cloudresty/go-rabbitmq/saga"

// Create saga store and manager
store := saga.NewInMemoryStore()
manager, err := saga.NewManager(client, store, saga.Config{
    SagaExchange:    "sagas",
    StepQueue:       "saga.steps",
    CompensateQueue: "saga.compensate",
})
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// Define saga steps
steps := []saga.Step{
    {
        Name:         "create_order",
        Action:       "orders.create",
        Compensation: "orders.delete",
    },
    {
        Name:         "reserve_inventory",
        Action:       "inventory.reserve",
        Compensation: "inventory.release",
    },
    {
        Name:         "charge_payment",
        Action:       "payment.charge",
        Compensation: "payment.refund",
    },
}

// Start saga with context data
sagaContext := map[string]any{
    "order_id":     "12345",
    "customer_id":  "67890",
    "total_amount": 99.99,
}

s, err := manager.Start(ctx, "order_processing", steps, sagaContext)
if err != nil {
    log.Fatal(err)
}

log.Printf("Started saga: %s", s.ID)
```

🔝 [back to top](#saga-package-api-reference)

&nbsp;

### Compensating a Failed Saga

```go
// Compensate a saga that has failed
err := manager.Compensate(ctx, sagaID)
if err != nil {
    log.Printf("Failed to compensate saga %s: %v", sagaID, err)
} else {
    log.Printf("Successfully initiated compensation for saga %s", sagaID)
}
```

🔝 [back to top](#saga-package-api-reference)

&nbsp;

### Monitoring Saga Status

```go
// Get saga details
s, err := manager.Get(ctx, sagaID)
if err != nil {
    log.Fatal(err)
}

// Check saga state
if s.IsCompleted() {
    log.Println("Saga completed successfully")
} else if s.IsFailed() {
    log.Println("Saga failed")

    // Get failed steps for analysis
    failedSteps := s.GetFailedSteps()
    for _, step := range failedSteps {
        log.Printf("Step %s failed: %s", step.Name, step.Error)
    }
} else if s.IsCompensating() {
    log.Println("Saga is compensating")
} else if s.IsCompensated() {
    log.Println("Saga has been compensated")
}

// Get completed steps
completedSteps := s.GetCompletedSteps()
log.Printf("Completed %d out of %d steps", len(completedSteps), len(s.Steps))
```

🔝 [back to top](#saga-package-api-reference)

&nbsp;

### Listing Active Sagas

```go
// Get all active sagas
activeSagas, err := manager.ListActive(ctx)
if err != nil {
    log.Fatal(err)
}

log.Printf("Found %d active sagas", len(activeSagas))
for _, saga := range activeSagas {
    log.Printf("Saga %s (%s): %s", saga.ID, saga.Name, saga.State)
}
```

🔝 [back to top](#saga-package-api-reference)

&nbsp;

### Custom Store Implementation

```go
// Implement custom persistence store
type DatabaseStore struct {
    db *sql.DB
}

func (s *DatabaseStore) SaveSaga(ctx context.Context, saga *saga.Saga) error {
    // Implement database persistence
    return nil
}

func (s *DatabaseStore) LoadSaga(ctx context.Context, sagaID string) (*saga.Saga, error) {
    // Implement database loading
    return nil, nil
}

// ... implement other Store interface methods

// Use custom store
store := &DatabaseStore{db: myDB}
manager, err := saga.NewManager(client, store, config)
```

🔝 [back to top](#saga-package-api-reference)

&nbsp;

### Step Definition with Context

```go
steps := []saga.Step{
    {
        Name:         "validate_order",
        Action:       "orders.validate",
        Compensation: "orders.cancel",
        Input: map[string]any{
            "validation_rules": []string{"inventory", "payment_method"},
        },
    },
    {
        Name:         "process_payment",
        Action:       "payments.process",
        Compensation: "payments.refund",
        Input: map[string]any{
            "payment_method": "credit_card",
            "retry_attempts": 3,
        },
    },
}
```

🔝 [back to top](#saga-package-api-reference)

&nbsp;

## Best Practices

1. **Idempotency**: Ensure all saga steps and compensations are idempotent to handle retries safely
2. **Error Handling**: Implement comprehensive error handling and logging in step handlers
3. **State Management**: Use atomic store operations to prevent race conditions in concurrent environments
4. **Timeouts**: Implement timeouts for saga steps to prevent hanging transactions
5. **Monitoring**: Monitor saga completion rates and failure patterns for operational insights
6. **Compensation Logic**: Design compensations to be as reliable as possible, as they are critical for maintaining consistency
7. **Persistence**: Use durable storage for production sagas to survive service restarts
8. **Step Granularity**: Design steps to be fine-grained enough for precise error handling but coarse enough to minimize complexity

🔝 [back to top](#saga-package-api-reference)

&nbsp;

---

&nbsp;

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
