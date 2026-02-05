# Saga Pattern Package

[Home](../README.md) &nbsp;/&nbsp; Saga Pattern Package

&nbsp;

The `saga` package provides distributed transaction support using the Saga pattern for RabbitMQ applications. The Saga pattern is a sequence of local transactions where each transaction updates data within a single service. If a local transaction fails, the saga executes compensating transactions to undo the impact of preceding transactions.

&nbsp;

## Features

- **Orchestration Engine** - Fully automated saga orchestration with message-driven step execution
- **Atomic State Updates** - Concurrency-safe state management with atomic step and saga updates
- **Step Execution** - Execute saga steps with automatic progression and error handling
- **Compensation Logic** - Automatic rollback through compensating actions
- **Pluggable Storage** - Interface for different persistence implementations with atomic operations
- **In-Memory Store** - Built-in concurrent-safe store for testing and development
- **Message-Driven** - Uses RabbitMQ for reliable step coordination and orchestration
- **Error Handling** - Comprehensive error tracking, recovery, and compensation triggers
- **Production Ready** - Designed for high-concurrency production workloads

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

## Quick Start

### Setting Up the Orchestration Engine

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/saga"
)

func main() {
    // Create RabbitMQ client
    client, err := rabbitmq.NewClient(
        rabbitmq.WithHosts("localhost:5672"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create saga store
    store := saga.NewInMemoryStore()

    // Define step and compensation handlers
    stepHandlers := map[string]saga.StepHandler{
        "orders.create": createOrderHandler,
        "inventory.reserve": reserveInventoryHandler,
        "payment.charge": chargePaymentHandler,
    }

    compensationHandlers := map[string]saga.CompensationHandler{
        "orders.delete": deleteOrderHandler,
        "inventory.release": releaseInventoryHandler,
        "payment.refund": refundPaymentHandler,
    }

    // Create saga manager with handlers
    manager, err := saga.NewManager(client, store, saga.Config{
        SagaExchange:         "sagas",
        StepQueue:           "saga.steps",
        CompensateQueue:     "saga.compensate",
        StepHandlers:        stepHandlers,
        CompensationHandlers: compensationHandlers,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer manager.Close()

    // Start the orchestration engine (this is the heart of the saga system)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Run the orchestration engine in the background
    go func() {
        if err := manager.Run(ctx); err != nil {
            log.Printf("Orchestration engine error: %v", err)
        }
    }()

    // Define saga steps
    steps := []saga.Step{
        {
            Name:         "create_order",
            Action:       "orders.create",
            Compensation: "orders.delete",
            Input: map[string]any{
                "product_id": "prod-123",
                "quantity":   2,
            },
        },
        {
            Name:         "reserve_inventory",
            Action:       "inventory.reserve",
            Compensation: "inventory.release",
            Input: map[string]any{
                "product_id": "prod-123",
                "quantity":   2,
            },
        },
        {
            Name:         "charge_payment",
            Action:       "payment.charge",
            Compensation: "payment.refund",
            Input: map[string]any{
                "amount":         99.99,
                "payment_method": "card-456",
            },
        },
    }

    // Start saga (orchestration engine will automatically execute steps)
    orderContext := map[string]any{
        "customer_id": "cust-789",
        "order_total": 99.99,
    }

    s, err := manager.Start(context.Background(), "order_processing", steps, orderContext)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Started saga: %s", s.ID)

    // Monitor saga progress
    for {
        time.Sleep(1 * time.Second)

        currentSaga, err := manager.Get(context.Background(), s.ID)
        if err != nil {
            log.Printf("Error getting saga: %v", err)
            continue
        }

        log.Printf("Saga %s is %s", currentSaga.ID, currentSaga.State)

        if currentSaga.IsCompleted() {
            log.Println("Saga completed successfully!")
            break
        } else if currentSaga.IsFailed() || currentSaga.IsCompensated() {
            log.Printf("Saga failed or compensated: %s", currentSaga.State)
            break
        }
    }
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Implementing Step and Compensation Handlers

```go
// Step handler for creating orders
func createOrderHandler(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    // Extract input data
    productID := step.Input["product_id"].(string)
    quantity := step.Input["quantity"].(int)
    customerID := s.Context["customer_id"].(string)

    log.Printf("Creating order for customer %s: %d x %s", customerID, quantity, productID)

    // Simulate order creation logic
    orderID := generateOrderID()

    // Perform actual order creation
    if err := createOrderInDatabase(orderID, customerID, productID, quantity); err != nil {
        return fmt.Errorf("failed to create order: %w", err)
    }

    // Update step output for use in subsequent steps or compensation
    step.Output = map[string]any{
        "order_id": orderID,
        "status":   "created",
        "created_at": time.Now(),
    }

    log.Printf("Order created successfully: %s", orderID)
    return nil
}

// Step handler for inventory reservation
func reserveInventoryHandler(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    productID := step.Input["product_id"].(string)
    quantity := step.Input["quantity"].(int)

    log.Printf("Reserving %d units of %s", quantity, productID)

    // Check inventory availability
    available, err := checkInventoryAvailability(productID, quantity)
    if err != nil {
        return fmt.Errorf("failed to check inventory: %w", err)
    }

    if !available {
        return fmt.Errorf("insufficient inventory for product %s (need %d)", productID, quantity)
    }

    // Reserve inventory
    reservationID, err := reserveInventory(productID, quantity)
    if err != nil {
        return fmt.Errorf("failed to reserve inventory: %w", err)
    }

    step.Output = map[string]any{
        "reservation_id": reservationID,
        "reserved_qty":   quantity,
        "reserved_at":    time.Now(),
    }

    log.Printf("Inventory reserved: %s", reservationID)
    return nil
}

// Step handler for payment processing
func chargePaymentHandler(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    amount := step.Input["amount"].(float64)
    paymentMethod := step.Input["payment_method"].(string)
    customerID := s.Context["customer_id"].(string)

    log.Printf("Charging $%.2f to %s for customer %s", amount, paymentMethod, customerID)

    // Process payment with timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    transactionID, err := processPayment(ctx, paymentMethod, amount, customerID)
    if err != nil {
        return fmt.Errorf("payment failed: %w", err)
    }

    step.Output = map[string]any{
        "transaction_id":  transactionID,
        "amount_charged":  amount,
        "charged_at":      time.Now(),
        "payment_method":  paymentMethod,
    }

    log.Printf("Payment processed successfully: %s", transactionID)
    return nil
}

// Compensation handler for order deletion
func deleteOrderHandler(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    // Extract order ID from step output
    orderID, exists := step.Output["order_id"]
    if !exists {
        log.Printf("No order ID found in step output, skipping deletion")
        return nil // Idempotent - if no order was created, nothing to delete
    }

    log.Printf("Deleting order %s", orderID)

    if err := deleteOrderFromDatabase(orderID.(string)); err != nil {
        return fmt.Errorf("failed to delete order: %w", err)
    }

    log.Printf("Order %s deleted successfully", orderID)
    return nil
}

// Compensation handler for inventory release
func releaseInventoryHandler(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    reservationID, exists := step.Output["reservation_id"]
    if !exists {
        log.Printf("No reservation ID found, skipping inventory release")
        return nil // Idempotent
    }

    log.Printf("Releasing inventory reservation %s", reservationID)

    if err := releaseInventoryReservation(reservationID.(string)); err != nil {
        return fmt.Errorf("failed to release inventory: %w", err)
    }

    log.Printf("Inventory reservation %s released successfully", reservationID)
    return nil
}

// Compensation handler for payment refund
func refundPaymentHandler(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    transactionID, exists := step.Output["transaction_id"]
    if !exists {
        log.Printf("No transaction ID found, skipping refund")
        return nil // Idempotent
    }

    amount := step.Output["amount_charged"].(float64)
    log.Printf("Refunding $%.2f for transaction %s", amount, transactionID)

    refundID, err := processRefund(transactionID.(string), amount)
    if err != nil {
        return fmt.Errorf("failed to process refund: %w", err)
    }

    log.Printf("Refund processed successfully: %s", refundID)
    return nil
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

## Saga States and Lifecycle

### Orchestration Engine

The saga package includes a complete orchestration engine that automatically manages saga execution:

```go
// Start the orchestration engine
go func() {
    if err := manager.Run(ctx); err != nil {
        log.Printf("Orchestration engine error: %v", err)
    }
}()
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

**What the orchestration engine does:**

1. **Message Processing**: Consumes step execution and compensation messages from RabbitMQ queues
2. **Handler Execution**: Looks up and executes registered step/compensation handlers
3. **State Management**: Atomically updates saga and step states in the store
4. **Flow Control**: Automatically publishes the next step message when a step completes
5. **Error Handling**: Triggers compensation when steps fail
6. **Concurrency Safety**: Uses atomic operations to prevent race conditions

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Atomic State Updates

The package provides concurrency-safe state updates through atomic operations:

```go
// Store interface includes atomic update methods
type Store interface {
    SaveSaga(ctx context.Context, saga *Saga) error
    LoadSaga(ctx context.Context, sagaID string) (*Saga, error)

    // Atomic operations for concurrency safety
    UpdateSagaStep(ctx context.Context, sagaID, stepID string, status State, output map[string]any, errorMsg string) (*Saga, error)
    UpdateSagaState(ctx context.Context, sagaID string, state State) (*Saga, error)
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

**Benefits of atomic updates:**

- **Race Condition Prevention**: Multiple concurrent processes can safely update the same saga
- **Consistency**: Guarantees that state changes are atomic and consistent
- **Performance**: Eliminates the need for load-modify-save patterns
- **Reliability**: Ensures saga state is always accurate, even under high concurrency

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Saga States

```go
saga.StateStarted      // Saga has been initiated
saga.StateCompleted    // All steps completed successfully
saga.StateFailed       // One or more steps failed
saga.StateCompensating // Compensation is in progress
saga.StateCompensated  // All compensations completed
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Step States

```go
saga.StateStarted   // Step is ready to execute
saga.StateCompleted // Step executed successfully
saga.StateFailed    // Step execution failed
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Monitoring Saga Progress

```go
// Get saga status
s, err := manager.Get(context.Background(), sagaID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Saga %s is %s\n", s.ID, s.State)

// Check individual steps
for _, step := range s.Steps {
    fmt.Printf("Step %s (%s): %s\n", step.Name, step.Action, step.Status)
    if step.Error != "" {
        fmt.Printf("  Error: %s\n", step.Error)
    }
}

// Use helper methods
if s.IsCompleted() {
    fmt.Println("Saga completed successfully!")
}

if s.IsFailed() {
    fmt.Println("Saga failed, starting compensation...")
    err := manager.Compensate(context.Background(), s.ID)
    if err != nil {
        log.Printf("Compensation failed: %v", err)
    }
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

## Error Handling and Compensation

### Automatic Compensation

The orchestration engine automatically triggers compensation when steps fail:

```go
// No manual intervention needed - the engine handles this automatically
// When a step handler returns an error:
func problematicStepHandler(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    // If this returns an error, the orchestration engine will:
    // 1. Mark the step as failed
    // 2. Mark the saga as failed
    // 3. Automatically start compensation
    return fmt.Errorf("step failed for some reason")
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Manual Compensation Trigger

You can also manually trigger compensation:

```go
// Manually trigger compensation for a saga
err := manager.Compensate(context.Background(), sagaID)
if err != nil {
    log.Printf("Failed to start compensation: %v", err)
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Monitoring Saga State

```go
// Monitor saga progress
saga, err := manager.Get(context.Background(), sagaID)
if err != nil {
    log.Printf("Error getting saga: %v", err)
    return
}

// Check saga state
switch saga.State {
case saga.StateCompleted:
    log.Println("Saga completed successfully!")
case saga.StateFailed:
    log.Println("Saga failed, compensation should start automatically")
case saga.StateCompensating:
    log.Println("Saga is currently being compensated")
case saga.StateCompensated:
    log.Println("Saga has been fully compensated")
default:
    log.Printf("Saga is in progress: %s", saga.State)
}

// Check individual steps
for _, step := range saga.Steps {
    if step.Status == saga.StateFailed {
        log.Printf("Step %s failed: %s", step.Name, step.Error)
    }
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

## Custom Persistence Store

Implement the `Store` interface for production use with atomic operations:

```go
type PostgresSagaStore struct {
    db *sql.DB
}

func (p *PostgresSagaStore) SaveSaga(ctx context.Context, saga *saga.Saga) error {
    query := `
        INSERT INTO sagas (id, name, state, steps, context, created_at, updated_at, completed_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        ON CONFLICT (id) DO UPDATE SET
            state = $3, steps = $4, context = $5, updated_at = $7, completed_at = $8
    `

    stepsJSON, _ := json.Marshal(saga.Steps)
    contextJSON, _ := json.Marshal(saga.Context)

    _, err := p.db.ExecContext(ctx, query,
        saga.ID, saga.Name, saga.State, stepsJSON, contextJSON,
        saga.CreatedAt, saga.UpdatedAt, saga.CompletedAt)

    return err
}

func (p *PostgresSagaStore) LoadSaga(ctx context.Context, sagaID string) (*saga.Saga, error) {
    query := `
        SELECT id, name, state, steps, context, created_at, updated_at, completed_at
        FROM sagas WHERE id = $1
    `

    var s saga.Saga
    var stepsJSON, contextJSON []byte

    err := p.db.QueryRowContext(ctx, query, sagaID).Scan(
        &s.ID, &s.Name, &s.State, &stepsJSON, &contextJSON,
        &s.CreatedAt, &s.UpdatedAt, &s.CompletedAt)

    if err != nil {
        return nil, err
    }

    json.Unmarshal(stepsJSON, &s.Steps)
    json.Unmarshal(contextJSON, &s.Context)

    return &s, nil
}

// Atomic step update - critical for concurrency safety
func (p *PostgresSagaStore) UpdateSagaStep(ctx context.Context, sagaID, stepID string, status saga.State, output map[string]any, errorMsg string) (*saga.Saga, error) {
    tx, err := p.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    // Lock the saga row for update
    var s saga.Saga
    var stepsJSON, contextJSON []byte

    query := `
        SELECT id, name, state, steps, context, created_at, updated_at, completed_at
        FROM sagas WHERE id = $1 FOR UPDATE
    `

    err = tx.QueryRowContext(ctx, query, sagaID).Scan(
        &s.ID, &s.Name, &s.State, &stepsJSON, &contextJSON,
        &s.CreatedAt, &s.UpdatedAt, &s.CompletedAt)

    if err != nil {
        return nil, err
    }

    json.Unmarshal(stepsJSON, &s.Steps)
    json.Unmarshal(contextJSON, &s.Context)

    // Find and update the step
    stepFound := false
    for i := range s.Steps {
        if s.Steps[i].ID == stepID {
            s.Steps[i].Status = status
            s.Steps[i].Output = output
            s.Steps[i].Error = errorMsg
            s.Steps[i].ExecutedAt = time.Now()
            stepFound = true
            break
        }
    }

    if !stepFound {
        return nil, fmt.Errorf("step not found: %s", stepID)
    }

    // Update saga state based on step states
    s.UpdatedAt = time.Now()
    allCompleted := true
    anyFailed := false

    for _, step := range s.Steps {
        if step.Status == saga.StateFailed {
            anyFailed = true
            break
        }
        if step.Status != saga.StateCompleted {
            allCompleted = false
        }
    }

    if anyFailed {
        s.State = saga.StateFailed
    } else if allCompleted {
        s.State = saga.StateCompleted
        now := time.Now()
        s.CompletedAt = &now
    }

    // Save updated saga
    updatedStepsJSON, _ := json.Marshal(s.Steps)
    updatedContextJSON, _ := json.Marshal(s.Context)

    updateQuery := `
        UPDATE sagas SET
            state = $2, steps = $3, context = $4, updated_at = $5, completed_at = $6
        WHERE id = $1
    `

    _, err = tx.ExecContext(ctx, updateQuery,
        s.ID, s.State, updatedStepsJSON, updatedContextJSON, s.UpdatedAt, s.CompletedAt)

    if err != nil {
        return nil, err
    }

    return &s, tx.Commit()
}

// Atomic saga state update
func (p *PostgresSagaStore) UpdateSagaState(ctx context.Context, sagaID string, state saga.State) (*saga.Saga, error) {
    tx, err := p.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    // Lock and load saga
    var s saga.Saga
    var stepsJSON, contextJSON []byte

    query := `
        SELECT id, name, state, steps, context, created_at, updated_at, completed_at
        FROM sagas WHERE id = $1 FOR UPDATE
    `

    err = tx.QueryRowContext(ctx, query, sagaID).Scan(
        &s.ID, &s.Name, &s.State, &stepsJSON, &contextJSON,
        &s.CreatedAt, &s.UpdatedAt, &s.CompletedAt)

    if err != nil {
        return nil, err
    }

    json.Unmarshal(stepsJSON, &s.Steps)
    json.Unmarshal(contextJSON, &s.Context)

    // Update state
    s.State = state
    s.UpdatedAt = time.Now()

    if state == saga.StateCompleted {
        now := time.Now()
        s.CompletedAt = &now
    }

    // Save updated saga
    updateQuery := `
        UPDATE sagas SET state = $2, updated_at = $3, completed_at = $4
        WHERE id = $1
    `

    _, err = tx.ExecContext(ctx, updateQuery, s.ID, s.State, s.UpdatedAt, s.CompletedAt)
    if err != nil {
        return nil, err
    }

    return &s, tx.Commit()
}

// Implement remaining Store methods...
func (p *PostgresSagaStore) DeleteSaga(ctx context.Context, sagaID string) error {
    _, err := p.db.ExecContext(ctx, "DELETE FROM sagas WHERE id = $1", sagaID)
    return err
}

func (p *PostgresSagaStore) ListActiveSagas(ctx context.Context) ([]*saga.Saga, error) {
    query := `
        SELECT id, name, state, steps, context, created_at, updated_at, completed_at
        FROM sagas WHERE state IN ('started', 'compensating')
    `

    rows, err := p.db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var sagas []*saga.Saga
    for rows.Next() {
        var s saga.Saga
        var stepsJSON, contextJSON []byte

        err := rows.Scan(&s.ID, &s.Name, &s.State, &stepsJSON, &contextJSON,
            &s.CreatedAt, &s.UpdatedAt, &s.CompletedAt)
        if err != nil {
            return nil, err
        }

        json.Unmarshal(stepsJSON, &s.Steps)
        json.Unmarshal(contextJSON, &s.Context)

        sagas = append(sagas, &s)
    }

    return sagas, rows.Err()
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

## Advanced Patterns

### Parallel Steps

```go
// Define parallel steps with dependencies
steps := []saga.Step{
    {Name: "create_order", Action: "orders.create"},

    // These can run in parallel
    {Name: "reserve_inventory", Action: "inventory.reserve"},
    {Name: "validate_address", Action: "shipping.validate"},

    // This waits for parallel steps
    {Name: "charge_payment", Action: "payment.charge"},
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Conditional Steps

```go
stepHandler := func(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    // Check saga context for conditions
    if s.Context["requires_approval"].(bool) {
        // Execute approval step
        return requestApproval(s.Context)
    }

    // Skip this step
    return nil
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Saga Composition

```go
// Compose larger sagas from smaller ones
func createOrderSaga(orderData map[string]any) []saga.Step {
    steps := []saga.Step{
        {Name: "validate_order", Action: "orders.validate"},
    }

    // Add payment steps if required
    if orderData["payment_required"].(bool) {
        steps = append(steps, createPaymentSteps(orderData)...)
    }

    // Add shipping steps if physical goods
    if orderData["requires_shipping"].(bool) {
        steps = append(steps, createShippingSteps(orderData)...)
    }

    return steps
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

## Integration Examples

### Production Service Setup

```go
func main() {
    // Create RabbitMQ client
    client, err := rabbitmq.NewClient(
        rabbitmq.WithHosts("localhost:5672"),
        rabbitmq.WithConnectionName("saga-service"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create production store (PostgreSQL example)
    store, err := NewPostgresSagaStore(databaseURL)
    if err != nil {
        log.Fatal(err)
    }

    // Create saga manager with all handlers
    manager, err := saga.NewManager(client, store, saga.Config{
        SagaExchange:         "sagas",
        StepQueue:           "saga.steps",
        CompensateQueue:     "saga.compensate",
        StepHandlers:        buildStepHandlers(),
        CompensationHandlers: buildCompensationHandlers(),
    })
    if err != nil {
        log.Fatal(err)
    }
    defer manager.Close()

    // Start orchestration engine
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle graceful shutdown
    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        <-sigChan
        log.Println("Shutting down saga service...")
        cancel()
    }()

    // Start the orchestration engine
    log.Println("Starting saga orchestration engine...")
    if err := manager.Run(ctx); err != nil && err != context.Canceled {
        log.Printf("Orchestration engine error: %v", err)
    }
}

func buildStepHandlers() map[string]saga.StepHandler {
    return map[string]saga.StepHandler{
        "orders.create":       orderService.CreateOrder,
        "inventory.reserve":   inventoryService.ReserveInventory,
        "payment.charge":      paymentService.ChargePayment,
        "shipping.schedule":   shippingService.ScheduleShipment,
        "notifications.send":  notificationService.SendNotification,
    }
}

func buildCompensationHandlers() map[string]saga.CompensationHandler {
    return map[string]saga.CompensationHandler{
        "orders.delete":        orderService.DeleteOrder,
        "inventory.release":    inventoryService.ReleaseInventory,
        "payment.refund":       paymentService.RefundPayment,
        "shipping.cancel":      shippingService.CancelShipment,
        "notifications.cancel": notificationService.CancelNotification,
    }
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### With HTTP API

```go
func startOrderSaga(w http.ResponseWriter, r *http.Request) {
    var orderRequest OrderRequest
    if err := json.NewDecoder(r.Body).Decode(&orderRequest); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    steps := createOrderSteps(orderRequest)
    context := map[string]any{
        "customer_id": orderRequest.CustomerID,
        "order_data":  orderRequest,
        "request_id":  r.Header.Get("X-Request-ID"),
    }

    saga, err := sagaManager.Start(r.Context(), "order_processing", steps, context)
    if err != nil {
        log.Printf("Failed to start saga: %v", err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    response := map[string]any{
        "saga_id": saga.ID,
        "status":  saga.State,
        "message": "Order processing started",
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func getSagaStatus(w http.ResponseWriter, r *http.Request) {
    sagaID := mux.Vars(r)["sagaId"]

    saga, err := sagaManager.Get(r.Context(), sagaID)
    if err != nil {
        http.Error(w, "Saga not found", http.StatusNotFound)
        return
    }

    // Create detailed response
    response := map[string]any{
        "saga_id":      saga.ID,
        "name":         saga.Name,
        "state":        saga.State,
        "created_at":   saga.CreatedAt,
        "updated_at":   saga.UpdatedAt,
        "completed_at": saga.CompletedAt,
        "steps": func() []map[string]any {
            steps := make([]map[string]any, len(saga.Steps))
            for i, step := range saga.Steps {
                steps[i] = map[string]any{
                    "id":          step.ID,
                    "name":        step.Name,
                    "action":      step.Action,
                    "status":      step.Status,
                    "executed_at": step.ExecutedAt,
                    "error":       step.Error,
                }
            }
            return steps
        }(),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Webhook endpoint for external service notifications
func handleStepCompletion(w http.ResponseWriter, r *http.Request) {
    var notification struct {
        SagaID   string                 `json:"saga_id"`
        StepID   string                 `json:"step_id"`
        Status   string                 `json:"status"`
        Output   map[string]any `json:"output"`
        Error    string                 `json:"error"`
    }

    if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
        http.Error(w, "Invalid notification", http.StatusBadRequest)
        return
    }

    // External services can notify saga completion via webhooks
    // The orchestration engine will handle the next steps automatically
    log.Printf("Received step completion notification: saga=%s, step=%s, status=%s",
        notification.SagaID, notification.StepID, notification.Status)

    w.WriteHeader(http.StatusOK)
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### With Worker Pattern

```go
func startSagaWorkers(manager *saga.Manager, config saga.Config) {
    // Start step execution workers
    go func() {
        for {
            // Consume step execution messages
            err := consumer.Consume(context.Background(), config.StepQueue,
                func(ctx context.Context, delivery rabbitmq.Delivery) error {
                    return handleStepExecution(ctx, manager, delivery)
                })
            if err != nil {
                log.Printf("Step consumer error: %v", err)
            }
        }
    }()

    // Start compensation workers
    go func() {
        for {
            // Consume compensation messages
            err := consumer.Consume(context.Background(), config.CompensateQueue,
                func(ctx context.Context, delivery rabbitmq.Delivery) error {
                    return handleCompensation(ctx, manager, delivery)
                })
            if err != nil {
                log.Printf("Compensation consumer error: %v", err)
            }
        }
    }()
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

## Best Practices

### 1. Orchestration Engine Management

```go
// Always run the orchestration engine in production
func startSagaService() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Graceful shutdown
    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        <-sigChan
        log.Println("Shutting down...")
        cancel()
    }()

    // Start orchestration engine
    if err := manager.Run(ctx); err != nil && err != context.Canceled {
        log.Printf("Engine error: %v", err)
    }
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### 2. Idempotent Operations

```go
// Make steps idempotent to handle retries safely
stepHandler := func(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    // Check if already processed
    if step.Output["processed"] == true {
        return nil
    }

    // Perform operation
    result, err := performOperation(step.Input)
    if err != nil {
        return err
    }

    // Mark as processed
    step.Output = map[string]any{
        "processed": true,
        "result":    result,
        "timestamp": time.Now(),
    }

    return nil
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### 3. Timeout Handling

```go
stepHandler := func(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    // Create timeout context for step execution
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Perform operation with timeout
    return performOperationWithTimeout(ctx, step.Input)
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### 4. Retry Logic with Exponential Backoff

```go
stepHandler := func(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    maxRetries := 3
    baseDelay := time.Second

    for attempt := 0; attempt < maxRetries; attempt++ {
        err := performOperation(step.Input)
        if err == nil {
            return nil
        }

        // Check if this is the last attempt
        if attempt == maxRetries-1 {
            return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, err)
        }

        // Exponential backoff
        delay := baseDelay * time.Duration(1<<attempt)
        log.Printf("Step %s failed (attempt %d), retrying in %v: %v",
            step.Name, attempt+1, delay, err)

        select {
        case <-time.After(delay):
            continue
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    return nil
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### 5. Error Classification and Handling

```go
// Define error types for better compensation decisions
type StepError struct {
    Code    string
    Message string
    Retryable bool
}

func (e StepError) Error() string {
    return e.Message
}

stepHandler := func(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    result, err := performBusinessOperation(step.Input)
    if err != nil {
        // Classify the error
        switch {
        case isTemporaryError(err):
            return StepError{
                Code: "TEMPORARY_ERROR",
                Message: err.Error(),
                Retryable: true,
            }
        case isBusinessRuleViolation(err):
            return StepError{
                Code: "BUSINESS_RULE_VIOLATION",
                Message: err.Error(),
                Retryable: false,
            }
        default:
            return err
        }
    }

    step.Output = map[string]any{
        "result": result,
        "success": true,
    }

    return nil
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### 6. Monitoring and Observability

```go
func monitorSagas(manager *saga.Manager) {
    ticker := time.NewTicker(time.Minute)
    go func() {
        defer ticker.Stop()
        for range ticker.C {
            active, err := manager.ListActive(context.Background())
            if err != nil {
                log.Printf("Failed to list active sagas: %v", err)
                continue
            }

            log.Printf("Active sagas: %d", len(active))

            // Check for stale sagas
            staleThreshold := time.Hour
            for _, saga := range active {
                if time.Since(saga.UpdatedAt) > staleThreshold {
                    log.Printf("Stale saga detected: %s (last updated: %v)",
                        saga.ID, saga.UpdatedAt)

                    // Emit metrics for alerting
                    emitStaleSagaMetric(saga.ID, saga.Name, time.Since(saga.UpdatedAt))
                }
            }
        }
    }()
}

func emitStaleSagaMetric(sagaID, sagaName string, staleDuration time.Duration) {
    // Integrate with your monitoring system (Prometheus, etc.)
    staleSagaCounter.WithLabelValues(sagaName).Inc()
    staleDurationGauge.WithLabelValues(sagaName).Set(staleDuration.Seconds())
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### 7. Resource Management

```go
// Properly manage resources in handlers
func createOrderHandler(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    // Use connection pools for database operations
    db := getDBConnection()
    defer db.Close()

    // Use transactions for consistency
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Perform operations
    orderID, err := createOrderInTx(tx, step.Input)
    if err != nil {
        return err
    }

    // Commit transaction
    if err := tx.Commit(); err != nil {
        return err
    }

    step.Output = map[string]any{
        "order_id": orderID,
        "created_at": time.Now(),
    }

    return nil
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### 8. Compensation Best Practices

```go
// Make compensations idempotent and safe
func deleteOrderCompensation(ctx context.Context, s *saga.Saga, step *saga.Step) error {
    orderID, exists := step.Output["order_id"]
    if !exists {
        log.Printf("No order ID found for compensation, skipping")
        return nil // Safe to skip if no order was created
    }

    // Check if order still exists
    exists, err := orderExists(ctx, orderID.(string))
    if err != nil {
        return fmt.Errorf("failed to check order existence: %w", err)
    }

    if !exists {
        log.Printf("Order %s already deleted, compensation idempotent", orderID)
        return nil // Idempotent - already deleted
    }

    // Delete the order
    if err := deleteOrder(ctx, orderID.(string)); err != nil {
        return fmt.Errorf("failed to delete order %s: %w", orderID, err)
    }

    log.Printf("Order %s deleted successfully", orderID)
    return nil
}
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

## Configuration Reference

### Config Structure

| Field | Type | Description |
| :--- | :--- | :--- |
| `SagaExchange` | `string` | Exchange for saga coordination messages |
| `StepQueue` | `string` | Queue for step execution messages |
| `CompensateQueue` | `string` | Queue for compensation messages |
| `StepHandlers` | `map[string]StepHandler` | Step execution handlers (required for orchestration) |
| `CompensationHandlers` | `map[string]CompensationHandler` | Compensation handlers (required for orchestration) |

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Store Interface

The Store interface now includes atomic operations for concurrency safety:

| Method | Description |
| :--- | :--- |
| `SaveSaga` | Save a complete saga (used for initial creation) |
| `LoadSaga` | Load a saga by ID |
| `DeleteSaga` | Delete a saga |
| `ListActiveSagas` | List all active sagas (started or compensating) |
| `UpdateSagaStep` | **Atomically** update a specific step within a saga |
| `UpdateSagaState` | **Atomically** update the overall saga state |

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Saga Structure

| Field | Type | Description |
| :--- | :--- | :--- |
| `ID` | `string` | Unique saga identifier (ULID) |
| `Name` | `string` | Saga name/type |
| `State` | `State` | Current saga state |
| `Steps` | `[]Step` | Saga steps with execution details |
| `Context` | `map[string]any` | Saga-wide context data |
| `CreatedAt` | `time.Time` | Creation timestamp |
| `UpdatedAt` | `time.Time` | Last update timestamp |
| `CompletedAt` | `*time.Time` | Completion timestamp (nil if not completed) |

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Step Structure

| Field | Type | Description |
| :--- | :--- | :--- |
| `ID` | `string` | Unique step identifier (ULID) |
| `Name` | `string` | Human-readable step name |
| `Action` | `string` | Action identifier for handler lookup |
| `Compensation` | `string` | Compensation action identifier |
| `Input` | `map[string]any` | Step input data |
| `Output` | `map[string]any` | Step output data |
| `Status` | `State` | Current step status |
| `Error` | `string` | Error message if step failed |
| `ExecutedAt` | `time.Time` | Step execution timestamp |

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Manager Methods

| Method | Description |
| :--- | :--- |
| `NewManager` | Create a new saga manager with handlers |
| `Run` | **Start the orchestration engine** (blocks until context cancelled) |
| `Start` | Start a new saga (returns immediately) |
| `Get` | Retrieve a saga by ID |
| `ListActive` | List all active sagas |
| `Compensate` | Manually trigger compensation for a saga |
| `Stop` | Stop the orchestration engine gracefully |
| `Close` | Close all resources (publisher, consumer) |

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

## Testing

```bash
# Run saga package tests
go test ./saga

# Run with race detection
go test -race ./saga
```

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

### Orchestration Pattern

The saga package provides a complete orchestration engine with automatic step execution:

```go
// Create manager with handlers
manager, err := saga.NewManager(client, store, saga.Config{
    SagaExchange:         "sagas",
    StepQueue:            "saga.steps",
    CompensateQueue:      "saga.compensate",
    StepHandlers:         stepHandlers,     // Required
    CompensationHandlers: compHandlers,    // Required
})

// Start orchestration engine (critical!)
go func() {
    if err := manager.Run(ctx); err != nil {
        log.Printf("Engine error: %v", err)
    }
}()

// Start sagas (they execute automatically)
s, err := manager.Start(ctx, name, steps, context)
```

The orchestration engine automatically handles step execution, state updates, and compensation flow.

&nbsp;

🔝 [back to top](#saga-pattern-package)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

<sub>&copy; Cloudresty - All rights reserved</sub>

&nbsp;
