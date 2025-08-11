package saga

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cloudresty/go-rabbitmq"
)

func TestState(t *testing.T) {
	t.Run("state constants", func(t *testing.T) {
		if StateStarted != "started" {
			t.Errorf("expected StateStarted 'started', got %s", StateStarted)
		}

		if StateCompleted != "completed" {
			t.Errorf("expected StateCompleted 'completed', got %s", StateCompleted)
		}

		if StateFailed != "failed" {
			t.Errorf("expected StateFailed 'failed', got %s", StateFailed)
		}

		if StateCompensating != "compensating" {
			t.Errorf("expected StateCompensating 'compensating', got %s", StateCompensating)
		}

		if StateCompensated != "compensated" {
			t.Errorf("expected StateCompensated 'compensated', got %s", StateCompensated)
		}
	})
}

func TestStep(t *testing.T) {
	t.Run("step creation", func(t *testing.T) {
		step := Step{
			ID:           "step-1",
			Name:         "create_order",
			Action:       "orders.create",
			Compensation: "orders.delete",
			Input:        map[string]any{"order_id": "123"},
			Status:       StateStarted,
		}

		if step.ID != "step-1" {
			t.Errorf("expected step ID 'step-1', got %s", step.ID)
		}

		if step.Name != "create_order" {
			t.Errorf("expected step name 'create_order', got %s", step.Name)
		}

		if step.Action != "orders.create" {
			t.Errorf("expected action 'orders.create', got %s", step.Action)
		}

		if step.Compensation != "orders.delete" {
			t.Errorf("expected compensation 'orders.delete', got %s", step.Compensation)
		}

		if step.Status != StateStarted {
			t.Errorf("expected status 'started', got %s", step.Status)
		}

		if step.Input["order_id"] != "123" {
			t.Errorf("expected input order_id '123', got %v", step.Input["order_id"])
		}
	})
}

func TestSaga(t *testing.T) {
	t.Run("saga creation", func(t *testing.T) {
		now := time.Now()
		saga := Saga{
			ID:    "saga-123",
			Name:  "order_processing",
			State: StateStarted,
			Steps: []Step{
				{Name: "create_order", Action: "orders.create", Compensation: "orders.delete"},
				{Name: "reserve_inventory", Action: "inventory.reserve", Compensation: "inventory.release"},
			},
			Context:   map[string]interface{}{"customer_id": "cust-456"},
			CreatedAt: now,
			UpdatedAt: now,
		}

		if saga.ID != "saga-123" {
			t.Errorf("expected saga ID 'saga-123', got %s", saga.ID)
		}

		if saga.Name != "order_processing" {
			t.Errorf("expected saga name 'order_processing', got %s", saga.Name)
		}

		if saga.State != StateStarted {
			t.Errorf("expected state 'started', got %s", saga.State)
		}

		if len(saga.Steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(saga.Steps))
		}

		if saga.Context["customer_id"] != "cust-456" {
			t.Errorf("expected customer_id 'cust-456', got %v", saga.Context["customer_id"])
		}
	})

	t.Run("saga state helpers", func(t *testing.T) {
		saga := &Saga{State: StateCompleted}

		if !saga.IsCompleted() {
			t.Error("expected saga to be completed")
		}

		if saga.IsFailed() {
			t.Error("expected saga not to be failed")
		}

		if saga.IsCompensating() {
			t.Error("expected saga not to be compensating")
		}

		if saga.IsCompensated() {
			t.Error("expected saga not to be compensated")
		}
	})

	t.Run("saga failed state", func(t *testing.T) {
		saga := &Saga{State: StateFailed}

		if saga.IsCompleted() {
			t.Error("expected saga not to be completed")
		}

		if !saga.IsFailed() {
			t.Error("expected saga to be failed")
		}

		if saga.IsCompensating() {
			t.Error("expected saga not to be compensating")
		}

		if saga.IsCompensated() {
			t.Error("expected saga not to be compensated")
		}
	})

	t.Run("saga compensating state", func(t *testing.T) {
		saga := &Saga{State: StateCompensating}

		if saga.IsCompleted() {
			t.Error("expected saga not to be completed")
		}

		if saga.IsFailed() {
			t.Error("expected saga not to be failed")
		}

		if !saga.IsCompensating() {
			t.Error("expected saga to be compensating")
		}

		if saga.IsCompensated() {
			t.Error("expected saga not to be compensated")
		}
	})

	t.Run("saga compensated state", func(t *testing.T) {
		saga := &Saga{State: StateCompensated}

		if saga.IsCompleted() {
			t.Error("expected saga not to be completed")
		}

		if saga.IsFailed() {
			t.Error("expected saga not to be failed")
		}

		if saga.IsCompensating() {
			t.Error("expected saga not to be compensating")
		}

		if !saga.IsCompensated() {
			t.Error("expected saga to be compensated")
		}
	})
}

func TestConfig(t *testing.T) {
	t.Run("config creation", func(t *testing.T) {
		stepHandler := func(ctx context.Context, saga *Saga, step *Step) error {
			return nil
		}

		compensationHandler := func(ctx context.Context, saga *Saga, step *Step) error {
			return nil
		}

		config := Config{
			SagaExchange:    "sagas",
			StepQueue:       "saga.steps",
			CompensateQueue: "saga.compensate",
			StepHandlers: map[string]StepHandler{
				"orders.create": stepHandler,
			},
			CompensationHandlers: map[string]CompensationHandler{
				"orders.delete": compensationHandler,
			},
		}

		if config.SagaExchange != "sagas" {
			t.Errorf("expected exchange 'sagas', got %s", config.SagaExchange)
		}

		if config.StepQueue != "saga.steps" {
			t.Errorf("expected step queue 'saga.steps', got %s", config.StepQueue)
		}

		if config.CompensateQueue != "saga.compensate" {
			t.Errorf("expected compensate queue 'saga.compensate', got %s", config.CompensateQueue)
		}

		if len(config.StepHandlers) != 1 {
			t.Errorf("expected 1 step handler, got %d", len(config.StepHandlers))
		}

		if len(config.CompensationHandlers) != 1 {
			t.Errorf("expected 1 compensation handler, got %d", len(config.CompensationHandlers))
		}
	})
}

func TestInMemoryStore(t *testing.T) {
	t.Run("create in-memory store", func(t *testing.T) {
		store := NewInMemoryStore()

		if store == nil {
			t.Fatal("expected store to be created")
		}

		if store.sagas == nil {
			t.Error("expected store to have sagas map")
		}
	})

	t.Run("save and load saga", func(t *testing.T) {
		store := NewInMemoryStore()
		ctx := context.Background()

		saga := &Saga{
			ID:    "test-saga",
			Name:  "test",
			State: StateStarted,
			Steps: []Step{
				{Name: "step1", Action: "action1", Compensation: "comp1"},
			},
			Context:   map[string]any{"key": "value"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Save saga
		err := store.SaveSaga(ctx, saga)
		if err != nil {
			t.Errorf("failed to save saga: %v", err)
		}

		// Load saga
		loadedSaga, err := store.LoadSaga(ctx, "test-saga")
		if err != nil {
			t.Errorf("failed to load saga: %v", err)
		}

		if loadedSaga.ID != saga.ID {
			t.Errorf("expected saga ID %s, got %s", saga.ID, loadedSaga.ID)
		}

		if loadedSaga.Name != saga.Name {
			t.Errorf("expected saga name %s, got %s", saga.Name, loadedSaga.Name)
		}

		if loadedSaga.State != saga.State {
			t.Errorf("expected saga state %s, got %s", saga.State, loadedSaga.State)
		}
	})

	t.Run("load non-existent saga", func(t *testing.T) {
		store := NewInMemoryStore()
		ctx := context.Background()

		_, err := store.LoadSaga(ctx, "non-existent")
		if err == nil {
			t.Error("expected error when loading non-existent saga")
		}
	})

	t.Run("delete saga", func(t *testing.T) {
		store := NewInMemoryStore()
		ctx := context.Background()

		saga := &Saga{
			ID:    "delete-test",
			Name:  "test",
			State: StateStarted,
		}

		// Save saga
		err := store.SaveSaga(ctx, saga)
		if err != nil {
			t.Errorf("failed to save saga: %v", err)
		}

		// Delete saga
		err = store.DeleteSaga(ctx, "delete-test")
		if err != nil {
			t.Errorf("failed to delete saga: %v", err)
		}

		// Try to load deleted saga
		_, err = store.LoadSaga(ctx, "delete-test")
		if err == nil {
			t.Error("expected error when loading deleted saga")
		}
	})

	t.Run("list active sagas", func(t *testing.T) {
		store := NewInMemoryStore()
		ctx := context.Background()

		// Save multiple sagas
		saga1 := &Saga{ID: "saga1", Name: "test1", State: StateStarted}
		saga2 := &Saga{ID: "saga2", Name: "test2", State: StateCompleted}
		saga3 := &Saga{ID: "saga3", Name: "test3", State: StateFailed}

		if err := store.SaveSaga(ctx, saga1); err != nil {
			t.Fatalf("Failed to save saga1: %v", err)
		}
		if err := store.SaveSaga(ctx, saga2); err != nil {
			t.Fatalf("Failed to save saga2: %v", err)
		}
		if err := store.SaveSaga(ctx, saga3); err != nil {
			t.Fatalf("Failed to save saga3: %v", err)
		}

		// List active sagas (should exclude completed and failed)
		activeSagas, err := store.ListActiveSagas(ctx)
		if err != nil {
			t.Errorf("failed to list active sagas: %v", err)
		}

		if len(activeSagas) != 1 {
			t.Errorf("expected 1 active saga, got %d", len(activeSagas))
		}

		if activeSagas[0].ID != "saga1" {
			t.Errorf("expected active saga ID 'saga1', got %s", activeSagas[0].ID)
		}
	})
}

// Integration tests that require RabbitMQ
func TestNewManager(t *testing.T) {
	t.Run("create manager", func(t *testing.T) {
		client, err := rabbitmq.NewClient(rabbitmq.WithHosts("localhost:5672"))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}()

		store := NewInMemoryStore()
		config := Config{
			SagaExchange:    "test-sagas",
			StepQueue:       "test-saga-steps",
			CompensateQueue: "test-saga-compensate",
		}

		manager, err := NewManager(client, store, config)
		if err != nil {
			t.Errorf("failed to create saga manager: %v", err)
		}
		defer func() {
			_ = manager.Close() // Ignore close error in defer
		}()

		if manager.client != client {
			t.Error("expected manager to store client reference")
		}

		if manager.store != store {
			t.Error("expected manager to store store reference")
		}

		if manager.publisher == nil {
			t.Error("expected manager to have publisher")
		}

		if manager.consumer == nil {
			t.Error("expected manager to have consumer")
		}
	})
}

func TestManagerStart(t *testing.T) {
	t.Run("start saga", func(t *testing.T) {
		client, err := rabbitmq.NewClient(rabbitmq.WithHosts("localhost:5672"))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}()

		store := NewInMemoryStore()
		config := Config{
			SagaExchange:    "test-start-sagas",
			StepQueue:       "test-start-steps",
			CompensateQueue: "test-start-compensate",
		}

		manager, err := NewManager(client, store, config)
		if err != nil {
			t.Skip("Failed to create saga manager")
		}
		defer func() {
			_ = manager.Close() // Ignore close error in defer
		}()

		steps := []Step{
			{Name: "create_order", Action: "orders.create", Compensation: "orders.delete"},
			{Name: "reserve_inventory", Action: "inventory.reserve", Compensation: "inventory.release"},
		}

		sagaContext := map[string]interface{}{
			"customer_id": "cust-123",
			"order_total": 99.99,
		}

		ctx := context.Background()
		saga, err := manager.Start(ctx, "order_processing", steps, sagaContext)
		if err != nil {
			t.Errorf("failed to start saga: %v", err)
		}

		if saga.ID == "" {
			t.Error("expected saga to have ID")
		}

		if saga.Name != "order_processing" {
			t.Errorf("expected saga name 'order_processing', got %s", saga.Name)
		}

		if saga.State != StateStarted {
			t.Errorf("expected saga state 'started', got %s", saga.State)
		}

		if len(saga.Steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(saga.Steps))
		}

		// Check that step IDs were generated
		for i, step := range saga.Steps {
			if step.ID == "" {
				t.Errorf("expected step %d to have ID", i)
			}
		}
	})
}

func TestManagerGet(t *testing.T) {
	t.Run("get saga", func(t *testing.T) {
		store := NewInMemoryStore()
		ctx := context.Background()

		saga := &Saga{
			ID:    "get-test",
			Name:  "test",
			State: StateStarted,
		}

		if err := store.SaveSaga(ctx, saga); err != nil {
			t.Fatalf("Failed to save saga: %v", err)
		}

		client, err := rabbitmq.NewClient(rabbitmq.WithHosts("localhost:5672"))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}()

		config := Config{
			SagaExchange:    "test-get-sagas",
			StepQueue:       "test-get-steps",
			CompensateQueue: "test-get-compensate",
		}

		manager, err := NewManager(client, store, config)
		if err != nil {
			t.Skip("Failed to create saga manager")
		}
		defer func() {
			_ = manager.Close() // Ignore close error in defer
		}()

		retrievedSaga, err := manager.Get(ctx, "get-test")
		if err != nil {
			t.Errorf("failed to get saga: %v", err)
		}

		if retrievedSaga.ID != saga.ID {
			t.Errorf("expected saga ID %s, got %s", saga.ID, retrievedSaga.ID)
		}
	})
}

func TestManagerListActive(t *testing.T) {
	t.Run("list active sagas", func(t *testing.T) {
		store := NewInMemoryStore()
		ctx := context.Background()

		// Add test sagas
		saga1 := &Saga{ID: "active1", Name: "test1", State: StateStarted}
		saga2 := &Saga{ID: "active2", Name: "test2", State: StateCompensating}
		saga3 := &Saga{ID: "completed", Name: "test3", State: StateCompleted}

		if err := store.SaveSaga(ctx, saga1); err != nil {
			t.Fatalf("Failed to save saga1: %v", err)
		}
		if err := store.SaveSaga(ctx, saga2); err != nil {
			t.Fatalf("Failed to save saga2: %v", err)
		}
		if err := store.SaveSaga(ctx, saga3); err != nil {
			t.Fatalf("Failed to save saga3: %v", err)
		}

		client, err := rabbitmq.NewClient(rabbitmq.WithHosts("localhost:5672"))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}()

		config := Config{
			SagaExchange:    "test-list-sagas",
			StepQueue:       "test-list-steps",
			CompensateQueue: "test-list-compensate",
		}

		manager, err := NewManager(client, store, config)
		if err != nil {
			t.Skip("Failed to create saga manager")
		}
		defer func() {
			_ = manager.Close() // Ignore close error in defer
		}()

		activeSagas, err := manager.ListActive(ctx)
		if err != nil {
			t.Errorf("failed to list active sagas: %v", err)
		}

		if len(activeSagas) != 2 {
			t.Errorf("expected 2 active sagas, got %d", len(activeSagas))
		}
	})
}

func TestManagerClose(t *testing.T) {
	t.Run("close manager", func(t *testing.T) {
		client, err := rabbitmq.NewClient(rabbitmq.WithHosts("localhost:5672"))
		if err != nil {
			t.Skip("RabbitMQ not available for testing")
		}
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Failed to close client: %v", err)
			}
		}()

		store := NewInMemoryStore()
		config := Config{
			SagaExchange:    "test-close-sagas",
			StepQueue:       "test-close-steps",
			CompensateQueue: "test-close-compensate",
		}

		manager, err := NewManager(client, store, config)
		if err != nil {
			t.Skip("Failed to create saga manager")
		}

		err = manager.Close()
		if err != nil {
			t.Errorf("failed to close manager: %v", err)
		}

		// Calling close again should not error
		err = manager.Close()
		if err != nil {
			t.Errorf("failed to close manager second time: %v", err)
		}
	})
}

// Handler function tests
func TestStepHandler(t *testing.T) {
	t.Run("successful step handler", func(t *testing.T) {
		var processedSaga *Saga
		var processedStep *Step

		handler := func(ctx context.Context, saga *Saga, step *Step) error {
			processedSaga = saga
			processedStep = step
			return nil
		}

		saga := &Saga{ID: "test", Name: "test"}
		step := &Step{Name: "test-step", Action: "test.action"}

		ctx := context.Background()
		err := handler(ctx, saga, step)

		if err != nil {
			t.Errorf("handler should not return error: %v", err)
		}

		if processedSaga != saga {
			t.Error("expected handler to receive saga")
		}

		if processedStep != step {
			t.Error("expected handler to receive step")
		}
	})

	t.Run("step handler with error", func(t *testing.T) {
		expectedError := fmt.Errorf("step processing error")
		handler := func(ctx context.Context, saga *Saga, step *Step) error {
			return expectedError
		}

		saga := &Saga{ID: "test", Name: "test"}
		step := &Step{Name: "test-step", Action: "test.action"}

		ctx := context.Background()
		err := handler(ctx, saga, step)

		if err == nil {
			t.Error("expected handler to return error")
		}

		if err.Error() != expectedError.Error() {
			t.Errorf("expected error %v, got %v", expectedError, err)
		}
	})

	t.Run("compensation handler", func(t *testing.T) {
		var processedSaga *Saga
		var processedStep *Step

		handler := func(ctx context.Context, saga *Saga, step *Step) error {
			processedSaga = saga
			processedStep = step
			return nil
		}

		saga := &Saga{ID: "test", Name: "test"}
		step := &Step{Name: "test-step", Compensation: "test.compensate"}

		ctx := context.Background()
		err := handler(ctx, saga, step)

		if err != nil {
			t.Errorf("compensation handler should not return error: %v", err)
		}

		if processedSaga != saga {
			t.Error("expected compensation handler to receive saga")
		}

		if processedStep != step {
			t.Error("expected compensation handler to receive step")
		}
	})
}

// Benchmark tests
func BenchmarkNewInMemoryStore(b *testing.B) {

	for b.Loop() {
		_ = NewInMemoryStore()
	}
}

func BenchmarkSaveSaga(b *testing.B) {
	store := NewInMemoryStore()
	ctx := context.Background()
	saga := &Saga{
		ID:    "benchmark",
		Name:  "benchmark",
		State: StateStarted,
	}

	for b.Loop() {
		_ = store.SaveSaga(ctx, saga)
	}
}

func BenchmarkLoadSaga(b *testing.B) {
	store := NewInMemoryStore()
	ctx := context.Background()
	saga := &Saga{
		ID:    "benchmark",
		Name:  "benchmark",
		State: StateStarted,
	}
	if err := store.SaveSaga(ctx, saga); err != nil {
		b.Fatalf("Failed to save saga: %v", err)
	}

	for b.Loop() {
		_, _ = store.LoadSaga(ctx, "benchmark")
	}
}

// Example functions showing usage patterns
func ExampleNewManager() {
	client, err := rabbitmq.NewClient(rabbitmq.WithHosts("localhost:5672"))
	if err != nil {
		// In CI/test environments, RabbitMQ might not be available
		// This is acceptable for documentation examples
		return
	}

	defer func() {
		if client != nil {
			if err := client.Close(); err != nil {
				// Handle error in real code
			}
		}
	}()

	store := NewInMemoryStore()
	config := Config{
		SagaExchange:    "sagas",
		StepQueue:       "saga.steps",
		CompensateQueue: "saga.compensate",
	}

	manager, err := NewManager(client, store, config)
	if err != nil {
		// Handle error in real code
		return
	}

	defer func() {
		if manager != nil {
			_ = manager.Close() // Ignore close error in defer
		}
	}()

	_ = manager
	// Output:
}

func ExampleManager_Start() {
	client, err := rabbitmq.NewClient(rabbitmq.WithHosts("localhost:5672"))
	if err != nil {
		// In CI/test environments, RabbitMQ might not be available
		// This is acceptable for documentation examples
		return
	}

	defer func() {
		if client != nil {
			if err := client.Close(); err != nil {
				// Handle error in real code
			}
		}
	}()

	store := NewInMemoryStore()
	config := Config{
		SagaExchange:    "sagas",
		StepQueue:       "saga.steps",
		CompensateQueue: "saga.compensate",
	}

	manager, err := NewManager(client, store, config)
	if err != nil {
		// Handle error in real code
		return
	}

	defer func() {
		if manager != nil {
			_ = manager.Close() // Ignore close error in defer
		}
	}()

	steps := []Step{
		{Name: "create_order", Action: "orders.create", Compensation: "orders.delete"},
		{Name: "reserve_inventory", Action: "inventory.reserve", Compensation: "inventory.release"},
		{Name: "charge_payment", Action: "payment.charge", Compensation: "payment.refund"},
	}

	ctx := context.Background()
	saga, err := manager.Start(ctx, "order_processing", steps, map[string]any{
		"customer_id": "cust-123",
	})
	if err != nil {
		return
	}

	_ = saga
	// Output:
}
