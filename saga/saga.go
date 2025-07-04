// Package saga provides distributed transaction support using the Saga pattern.
//
// The Saga pattern is a sequence of local transactions where each transaction
// updates data within a single service. If a local transaction fails because
// it violates a business rule, the saga executes a compensating transaction
// to undo the impact of the preceding transactions.
//
// This implementation provides:
//   - Saga orchestration and coordination
//   - Step execution with compensating actions
//   - Pluggable persistence stores
//   - Built-in in-memory store for testing
//   - Message-driven step execution
//
// Example usage:
//
//	// Create saga manager
//	store := saga.NewInMemoryStore()
//	manager, err := saga.NewManager(client, store, saga.Config{
//		SagaExchange:    "sagas",
//		StepQueue:       "saga.steps",
//		CompensateQueue: "saga.compensate",
//	})
//
//	// Define saga steps
//	steps := []saga.Step{
//		{Name: "create_order", Action: "orders.create", Compensation: "orders.delete"},
//		{Name: "reserve_inventory", Action: "inventory.reserve", Compensation: "inventory.release"},
//		{Name: "charge_payment", Action: "payment.charge", Compensation: "payment.refund"},
//	}
//
//	// Start saga
//	s, err := manager.Start(ctx, "order_processing", steps, orderContext)
package saga

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
	"github.com/cloudresty/ulid"
)

// State represents the current state of a saga or step
type State string

const (
	StateStarted      State = "started"
	StateCompleted    State = "completed"
	StateFailed       State = "failed"
	StateCompensating State = "compensating"
	StateCompensated  State = "compensated"
)

// Step represents a single step in a saga
type Step struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Action       string         `json:"action"`       // The action to perform
	Compensation string         `json:"compensation"` // The compensation action
	Input        map[string]any `json:"input"`
	Output       map[string]any `json:"output"`
	Status       State          `json:"status"`
	Error        string         `json:"error,omitempty"`
	ExecutedAt   time.Time      `json:"executed_at"`
}

// Saga represents a distributed transaction saga
type Saga struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	State       State          `json:"state"`
	Steps       []Step         `json:"steps"`
	Context     map[string]any `json:"context"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// Store defines the interface for saga persistence with atomic operations
type Store interface {
	SaveSaga(ctx context.Context, saga *Saga) error
	LoadSaga(ctx context.Context, sagaID string) (*Saga, error)
	DeleteSaga(ctx context.Context, sagaID string) error
	ListActiveSagas(ctx context.Context) ([]*Saga, error)
	// Atomic operations for concurrency safety
	UpdateSagaStep(ctx context.Context, sagaID, stepID string, status State, output map[string]any, errorMsg string) (*Saga, error)
	UpdateSagaState(ctx context.Context, sagaID string, state State) (*Saga, error)
}

// Manager manages saga execution and coordination
type Manager struct {
	client    *rabbitmq.Client
	store     Store
	publisher *rabbitmq.Publisher
	consumer  *rabbitmq.Consumer
	config    Config
	running   bool
	stopCh    chan struct{}
}

// StepHandler defines the function signature for saga step handlers
type StepHandler func(ctx context.Context, saga *Saga, step *Step) error

// CompensationHandler defines the function signature for compensation handlers
type CompensationHandler func(ctx context.Context, saga *Saga, step *Step) error

// Config holds configuration for saga management
type Config struct {
	SagaExchange         string // Exchange for saga coordination messages
	StepQueue            string // Queue for saga step execution
	CompensateQueue      string // Queue for saga compensation
	StepHandlers         map[string]StepHandler
	CompensationHandlers map[string]CompensationHandler
}

// NewManager creates a new saga manager
func NewManager(client *rabbitmq.Client, store Store, config Config) (*Manager, error) {
	// Create publisher for saga coordination
	publisher, err := client.NewPublisher(
		rabbitmq.WithDefaultExchange(config.SagaExchange),
		rabbitmq.WithPersistent(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create saga publisher: %w", err)
	}

	// Create consumer for saga step processing
	consumer, err := client.NewConsumer(
		rabbitmq.WithPrefetchCount(1),
		rabbitmq.WithConcurrency(1),
	)
	if err != nil {
		publisher.Close()
		return nil, fmt.Errorf("failed to create saga consumer: %w", err)
	}

	manager := &Manager{
		client:    client,
		store:     store,
		publisher: publisher,
		consumer:  consumer,
		config:    config,
		stopCh:    make(chan struct{}),
	}

	emit.Info.StructuredFields("Saga manager created",
		emit.ZString("saga_exchange", config.SagaExchange),
		emit.ZString("step_queue", config.StepQueue),
		emit.ZString("compensate_queue", config.CompensateQueue))

	return manager, nil
}

// Start starts a new saga
func (sm *Manager) Start(ctx context.Context, name string, steps []Step, sagaContext map[string]any) (*Saga, error) {
	sagaID, err := ulid.New()
	if err != nil {
		return nil, fmt.Errorf("failed to generate saga ID: %w", err)
	}

	saga := &Saga{
		ID:        sagaID,
		Name:      name,
		State:     StateStarted,
		Steps:     steps,
		Context:   sagaContext,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Initialize step IDs if not set
	for i := range saga.Steps {
		if saga.Steps[i].ID == "" {
			stepID, err := ulid.New()
			if err != nil {
				return nil, fmt.Errorf("failed to generate step ID: %w", err)
			}
			saga.Steps[i].ID = stepID
		}
		saga.Steps[i].Status = StateStarted
	}

	// Save saga to store
	if err := sm.store.SaveSaga(ctx, saga); err != nil {
		return nil, fmt.Errorf("failed to save saga: %w", err)
	}

	// Publish first step
	if len(saga.Steps) > 0 {
		if err := sm.publishStep(ctx, saga, &saga.Steps[0]); err != nil {
			return nil, fmt.Errorf("failed to publish first step: %w", err)
		}
	}

	emit.Info.StructuredFields("Saga started",
		emit.ZString("saga_id", saga.ID),
		emit.ZString("saga_name", saga.Name),
		emit.ZInt("step_count", len(saga.Steps)))

	return saga, nil
}

// Compensate starts compensation for a failed saga
func (sm *Manager) Compensate(ctx context.Context, sagaID string) error {
	saga, err := sm.store.LoadSaga(ctx, sagaID)
	if err != nil {
		return fmt.Errorf("failed to load saga: %w", err)
	}

	saga.State = StateCompensating
	saga.UpdatedAt = time.Now()

	// Save updated state
	if err := sm.store.SaveSaga(ctx, saga); err != nil {
		return fmt.Errorf("failed to save saga state: %w", err)
	}

	// Start compensation from the last completed step backwards
	for i := len(saga.Steps) - 1; i >= 0; i-- {
		step := &saga.Steps[i]
		if step.Status == StateCompleted {
			if err := sm.publishCompensation(ctx, saga, step); err != nil {
				return fmt.Errorf("failed to publish compensation for step %s: %w", step.ID, err)
			}
		}
	}

	emit.Info.StructuredFields("Saga compensation started",
		emit.ZString("saga_id", saga.ID),
		emit.ZString("saga_name", saga.Name))

	return nil
}

// Get retrieves a saga by ID
func (sm *Manager) Get(ctx context.Context, sagaID string) (*Saga, error) {
	return sm.store.LoadSaga(ctx, sagaID)
}

// ListActive lists all active sagas
func (sm *Manager) ListActive(ctx context.Context) ([]*Saga, error) {
	return sm.store.ListActiveSagas(ctx)
}

// publishStep publishes a step execution message
func (sm *Manager) publishStep(ctx context.Context, saga *Saga, step *Step) error {
	stepMessage := map[string]any{
		"saga_id": saga.ID,
		"step_id": step.ID,
		"action":  "execute",
		"step":    step,
		"saga":    saga,
	}

	messageBytes, err := json.Marshal(stepMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal step message: %w", err)
	}

	messageID, err := ulid.New()
	if err != nil {
		return fmt.Errorf("failed to generate message ID: %w", err)
	}

	message := rabbitmq.NewMessage(messageBytes).
		WithContentType(rabbitmq.ContentTypeJSON).
		WithMessageID(messageID).
		WithType("saga.step.execute").
		WithHeader("saga_id", saga.ID).
		WithHeader("step_id", step.ID)

	return sm.publisher.Publish(ctx, "", "saga.steps", message)
}

// publishCompensation publishes a compensation message
func (sm *Manager) publishCompensation(ctx context.Context, saga *Saga, step *Step) error {
	compensationMessage := map[string]any{
		"saga_id": saga.ID,
		"step_id": step.ID,
		"action":  "compensate",
		"step":    step,
		"saga":    saga,
	}

	messageBytes, err := json.Marshal(compensationMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal compensation message: %w", err)
	}

	messageID, err := ulid.New()
	if err != nil {
		return fmt.Errorf("failed to generate message ID: %w", err)
	}

	message := rabbitmq.NewMessage(messageBytes).
		WithContentType(rabbitmq.ContentTypeJSON).
		WithMessageID(messageID).
		WithType("saga.step.compensate").
		WithHeader("saga_id", saga.ID).
		WithHeader("step_id", step.ID)

	return sm.publisher.Publish(ctx, "", "saga.compensations", message)
}

// Close closes the saga manager
func (sm *Manager) Close() error {
	var errs []error

	if sm.publisher != nil {
		if err := sm.publisher.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close publisher: %w", err))
		}
	}

	if sm.consumer != nil {
		if err := sm.consumer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close consumer: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing saga manager: %v", errs)
	}

	return nil
}

// InMemoryStore provides a simple in-memory saga store for testing
type InMemoryStore struct {
	sagas map[string]*Saga
	mu    sync.RWMutex // Add mutex for concurrency safety
}

// NewInMemoryStore creates a new in-memory saga store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		sagas: make(map[string]*Saga),
	}
}

// SaveSaga saves a saga to the in-memory store
func (s *InMemoryStore) SaveSaga(ctx context.Context, saga *Saga) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sagas[saga.ID] = saga
	return nil
}

// LoadSaga loads a saga from the in-memory store
func (s *InMemoryStore) LoadSaga(ctx context.Context, sagaID string) (*Saga, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	saga, exists := s.sagas[sagaID]
	if !exists {
		return nil, fmt.Errorf("saga not found: %s", sagaID)
	}
	return saga, nil
}

// DeleteSaga deletes a saga from the in-memory store
func (s *InMemoryStore) DeleteSaga(ctx context.Context, sagaID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sagas, sagaID)
	return nil
}

// ListActiveSagas lists all active sagas
func (s *InMemoryStore) ListActiveSagas(ctx context.Context) ([]*Saga, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var active []*Saga
	for _, saga := range s.sagas {
		if saga.State == StateStarted || saga.State == StateCompensating {
			active = append(active, saga)
		}
	}
	return active, nil
}

// UpdateSagaStep atomically updates a specific step within a saga
func (s *InMemoryStore) UpdateSagaStep(ctx context.Context, sagaID, stepID string, status State, output map[string]any, errorMsg string) (*Saga, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	saga, exists := s.sagas[sagaID]
	if !exists {
		return nil, fmt.Errorf("saga not found: %s", sagaID)
	}

	// Find and update the step
	var stepFound bool
	for i := range saga.Steps {
		if saga.Steps[i].ID == stepID {
			saga.Steps[i].Status = status
			saga.Steps[i].Output = output
			saga.Steps[i].Error = errorMsg
			saga.Steps[i].ExecutedAt = time.Now()
			stepFound = true
			break
		}
	}

	if !stepFound {
		return nil, fmt.Errorf("step not found: %s", stepID)
	}

	// Update saga state if needed
	saga.UpdatedAt = time.Now()

	// Check if all steps are completed
	allCompleted := true
	anyFailed := false
	for _, step := range saga.Steps {
		if step.Status == StateFailed {
			anyFailed = true
			break
		}
		if step.Status != StateCompleted {
			allCompleted = false
		}
	}

	if anyFailed {
		saga.State = StateFailed
	} else if allCompleted {
		saga.State = StateCompleted
		now := time.Now()
		saga.CompletedAt = &now
	}

	return saga, nil
}

// UpdateSagaState atomically updates the state of a saga
func (s *InMemoryStore) UpdateSagaState(ctx context.Context, sagaID string, state State) (*Saga, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	saga, exists := s.sagas[sagaID]
	if !exists {
		return nil, fmt.Errorf("saga not found: %s", sagaID)
	}

	saga.State = state
	saga.UpdatedAt = time.Now()

	if state == StateCompleted {
		now := time.Now()
		saga.CompletedAt = &now
	}

	return saga, nil
}

// IsCompleted returns true if the saga has completed successfully
func (s *Saga) IsCompleted() bool {
	return s.State == StateCompleted
}

// IsFailed returns true if the saga has failed
func (s *Saga) IsFailed() bool {
	return s.State == StateFailed
}

// IsCompensating returns true if the saga is currently compensating
func (s *Saga) IsCompensating() bool {
	return s.State == StateCompensating
}

// IsCompensated returns true if the saga has been fully compensated
func (s *Saga) IsCompensated() bool {
	return s.State == StateCompensated
}

// GetCompletedSteps returns all steps that have completed successfully
func (s *Saga) GetCompletedSteps() []Step {
	var completed []Step
	for _, step := range s.Steps {
		if step.Status == StateCompleted {
			completed = append(completed, step)
		}
	}
	return completed
}

// GetFailedSteps returns all steps that have failed
func (s *Saga) GetFailedSteps() []Step {
	var failed []Step
	for _, step := range s.Steps {
		if step.Status == StateFailed {
			failed = append(failed, step)
		}
	}
	return failed
}

// GetNextStep returns the next step to execute, or nil if no more steps
func (s *Saga) GetNextStep() *Step {
	for i := range s.Steps {
		if s.Steps[i].Status == StateStarted {
			return &s.Steps[i]
		}
	}
	return nil
}

// Run starts the saga orchestration engine - this is the heart of the orchestrator
func (sm *Manager) Run(ctx context.Context) error {
	if sm.running {
		return fmt.Errorf("saga manager is already running")
	}

	sm.running = true
	defer func() { sm.running = false }()

	emit.Info.Msg("Starting saga orchestration engine")

	// Start consuming step execution messages
	stepErr := make(chan error, 1)
	go func() {
		err := sm.consumer.Consume(ctx, sm.config.StepQueue, sm.handleStepMessage)
		if err != nil && err != context.Canceled {
			stepErr <- fmt.Errorf("step consumer error: %w", err)
		}
	}()

	// Start consuming compensation messages
	compensationErr := make(chan error, 1)
	go func() {
		err := sm.consumer.Consume(ctx, sm.config.CompensateQueue, sm.handleCompensationMessage)
		if err != nil && err != context.Canceled {
			compensationErr <- fmt.Errorf("compensation consumer error: %w", err)
		}
	}()

	emit.Info.StructuredFields("Saga orchestration engine started",
		emit.ZString("step_queue", sm.config.StepQueue),
		emit.ZString("compensate_queue", sm.config.CompensateQueue))

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		emit.Info.Msg("Saga orchestration engine stopping due to context cancellation")
		return ctx.Err()
	case <-sm.stopCh:
		emit.Info.Msg("Saga orchestration engine stopping due to stop signal")
		return nil
	case err := <-stepErr:
		emit.Error.StructuredFields("Saga step consumer error", emit.ZString("error", err.Error()))
		return err
	case err := <-compensationErr:
		emit.Error.StructuredFields("Saga compensation consumer error", emit.ZString("error", err.Error()))
		return err
	}
}

// handleStepMessage processes step execution messages
func (sm *Manager) handleStepMessage(ctx context.Context, delivery *rabbitmq.Delivery) error {
	defer func() {
		if r := recover(); r != nil {
			emit.Error.StructuredFields("Panic in step message handler",
				emit.ZString("error", fmt.Sprintf("%v", r)))
		}
	}()

	// Decode the message
	var stepMessage struct {
		SagaID string `json:"saga_id"`
		StepID string `json:"step_id"`
		Action string `json:"action"`
		Step   Step   `json:"step"`
		Saga   Saga   `json:"saga"`
	}

	if err := json.Unmarshal(delivery.Body, &stepMessage); err != nil {
		emit.Error.StructuredFields("Failed to unmarshal step message",
			emit.ZString("error", err.Error()))
		return fmt.Errorf("failed to unmarshal step message: %w", err)
	}

	emit.Info.StructuredFields("Processing step execution",
		emit.ZString("saga_id", stepMessage.SagaID),
		emit.ZString("step_id", stepMessage.StepID),
		emit.ZString("action", stepMessage.Step.Action))

	// Look up the step handler
	handler, exists := sm.config.StepHandlers[stepMessage.Step.Action]
	if !exists {
		emit.Error.StructuredFields("No handler found for step action",
			emit.ZString("action", stepMessage.Step.Action),
			emit.ZString("saga_id", stepMessage.SagaID),
			emit.ZString("step_id", stepMessage.StepID))

		// Mark step as failed due to missing handler
		_, err := sm.store.UpdateSagaStep(ctx, stepMessage.SagaID, stepMessage.StepID,
			StateFailed, nil, fmt.Sprintf("no handler found for action: %s", stepMessage.Step.Action))
		if err != nil {
			emit.Error.StructuredFields("Failed to update step status",
				emit.ZString("error", err.Error()))
		}

		// Mark saga as failed and start compensation
		sm.store.UpdateSagaState(ctx, stepMessage.SagaID, StateFailed)
		sm.Compensate(ctx, stepMessage.SagaID)
		return nil // Message processed successfully even though step failed
	}

	// Execute the step handler
	err := handler(ctx, &stepMessage.Saga, &stepMessage.Step)

	if err != nil {
		// Step failed - mark as failed and start compensation
		emit.Error.StructuredFields("Step execution failed",
			emit.ZString("saga_id", stepMessage.SagaID),
			emit.ZString("step_id", stepMessage.StepID),
			emit.ZString("action", stepMessage.Step.Action),
			emit.ZString("error", err.Error()))

		_, updateErr := sm.store.UpdateSagaStep(ctx, stepMessage.SagaID, stepMessage.StepID,
			StateFailed, nil, err.Error())
		if updateErr != nil {
			emit.Error.StructuredFields("Failed to update failed step",
				emit.ZString("error", updateErr.Error()))
			return updateErr
		}

		// Mark saga as failed and start compensation
		sm.store.UpdateSagaState(ctx, stepMessage.SagaID, StateFailed)
		sm.Compensate(ctx, stepMessage.SagaID)
		return nil // Message processed successfully even though step failed
	}

	// Step succeeded - mark as completed
	emit.Info.StructuredFields("Step execution completed",
		emit.ZString("saga_id", stepMessage.SagaID),
		emit.ZString("step_id", stepMessage.StepID),
		emit.ZString("action", stepMessage.Step.Action))

	updatedSaga, err := sm.store.UpdateSagaStep(ctx, stepMessage.SagaID, stepMessage.StepID,
		StateCompleted, stepMessage.Step.Output, "")
	if err != nil {
		emit.Error.StructuredFields("Failed to update completed step",
			emit.ZString("error", err.Error()))
		return err
	}

	// Check if there are more steps to execute
	nextStep := updatedSaga.GetNextStep()
	if nextStep != nil {
		// Publish next step
		if err := sm.publishStep(ctx, updatedSaga, nextStep); err != nil {
			emit.Error.StructuredFields("Failed to publish next step",
				emit.ZString("error", err.Error()))
			return err
		}
	} else {
		// All steps completed - mark saga as completed
		_, err := sm.store.UpdateSagaState(ctx, stepMessage.SagaID, StateCompleted)
		if err != nil {
			emit.Error.StructuredFields("Failed to mark saga as completed",
				emit.ZString("error", err.Error()))
			return err
		}

		emit.Info.StructuredFields("Saga completed successfully",
			emit.ZString("saga_id", stepMessage.SagaID),
			emit.ZString("saga_name", updatedSaga.Name))
	}

	return nil
}

// handleCompensationMessage processes compensation messages
func (sm *Manager) handleCompensationMessage(ctx context.Context, delivery *rabbitmq.Delivery) error {
	defer func() {
		if r := recover(); r != nil {
			emit.Error.StructuredFields("Panic in compensation message handler",
				emit.ZString("error", fmt.Sprintf("%v", r)))
		}
	}()

	// Decode the message
	var compensationMessage struct {
		SagaID string `json:"saga_id"`
		StepID string `json:"step_id"`
		Action string `json:"action"`
		Step   Step   `json:"step"`
		Saga   Saga   `json:"saga"`
	}

	if err := json.Unmarshal(delivery.Body, &compensationMessage); err != nil {
		emit.Error.StructuredFields("Failed to unmarshal compensation message",
			emit.ZString("error", err.Error()))
		return fmt.Errorf("failed to unmarshal compensation message: %w", err)
	}

	emit.Info.StructuredFields("Processing step compensation",
		emit.ZString("saga_id", compensationMessage.SagaID),
		emit.ZString("step_id", compensationMessage.StepID),
		emit.ZString("compensation", compensationMessage.Step.Compensation))

	// Look up the compensation handler
	handler, exists := sm.config.CompensationHandlers[compensationMessage.Step.Compensation]
	if !exists {
		emit.Error.StructuredFields("No compensation handler found",
			emit.ZString("compensation", compensationMessage.Step.Compensation),
			emit.ZString("saga_id", compensationMessage.SagaID),
			emit.ZString("step_id", compensationMessage.StepID))

		// Mark step as compensated even without handler (best effort)
		sm.store.UpdateSagaStep(ctx, compensationMessage.SagaID, compensationMessage.StepID,
			StateCompensated, nil, fmt.Sprintf("no compensation handler found for: %s", compensationMessage.Step.Compensation))
		return nil
	}

	// Execute the compensation handler
	err := handler(ctx, &compensationMessage.Saga, &compensationMessage.Step)

	if err != nil {
		emit.Error.StructuredFields("Compensation execution failed",
			emit.ZString("saga_id", compensationMessage.SagaID),
			emit.ZString("step_id", compensationMessage.StepID),
			emit.ZString("compensation", compensationMessage.Step.Compensation),
			emit.ZString("error", err.Error()))

		// Mark compensation as failed but continue (best effort compensation)
		sm.store.UpdateSagaStep(ctx, compensationMessage.SagaID, compensationMessage.StepID,
			StateCompensated, nil, fmt.Sprintf("compensation failed: %s", err.Error()))
	} else {
		emit.Info.StructuredFields("Compensation execution completed",
			emit.ZString("saga_id", compensationMessage.SagaID),
			emit.ZString("step_id", compensationMessage.StepID),
			emit.ZString("compensation", compensationMessage.Step.Compensation))

		// Mark step as compensated
		sm.store.UpdateSagaStep(ctx, compensationMessage.SagaID, compensationMessage.StepID,
			StateCompensated, nil, "")
	}

	// Check if all eligible steps have been compensated
	saga, err := sm.store.LoadSaga(ctx, compensationMessage.SagaID)
	if err != nil {
		emit.Error.StructuredFields("Failed to load saga for compensation check",
			emit.ZString("error", err.Error()))
		return nil
	}

	allCompensated := true
	for _, step := range saga.Steps {
		if step.Status == StateCompleted && step.Status != StateCompensated {
			allCompensated = false
			break
		}
	}

	if allCompensated {
		// Mark saga as fully compensated
		sm.store.UpdateSagaState(ctx, compensationMessage.SagaID, StateCompensated)
		emit.Info.StructuredFields("Saga fully compensated",
			emit.ZString("saga_id", compensationMessage.SagaID),
			emit.ZString("saga_name", saga.Name))
	}

	return nil
}

// Stop stops the saga orchestration engine
func (sm *Manager) Stop() {
	if sm.running {
		close(sm.stopCh)
	}
}

// Interface compliance methods for rabbitmq.SagaOrchestrator

// StartSaga implements the simplified SagaOrchestrator interface
func (sm *Manager) StartSaga(ctx context.Context, sagaName string, steps []rabbitmq.SagaStep, context map[string]any) (string, error) {
	// Convert interface steps to internal steps
	internalSteps := make([]Step, len(steps))
	for i, step := range steps {
		internalSteps[i] = Step{
			Name:         step.Name,
			Action:       step.Action,
			Compensation: step.Compensation,
		}
	}

	// Start the saga using the internal method
	saga, err := sm.Start(ctx, sagaName, internalSteps, context)
	if err != nil {
		return "", err
	}

	return saga.ID, nil
}

// CompensateSaga implements the simplified SagaOrchestrator interface
func (sm *Manager) CompensateSaga(ctx context.Context, sagaID string) error {
	return sm.Compensate(ctx, sagaID)
}

// GetSagaStatus implements the simplified SagaOrchestrator interface
func (sm *Manager) GetSagaStatus(ctx context.Context, sagaID string) (rabbitmq.SagaStatus, error) {
	saga, err := sm.Get(ctx, sagaID)
	if err != nil {
		return rabbitmq.SagaStatus{}, err
	}

	// Convert internal saga to interface status
	steps := make([]rabbitmq.SagaStepStatus, len(saga.Steps))
	for i, step := range saga.Steps {
		steps[i] = rabbitmq.SagaStepStatus{
			Name:      step.Name,
			State:     string(step.Status),
			Error:     step.Error,
			Output:    step.Output,
			Timestamp: step.ExecutedAt,
		}
	}

	return rabbitmq.SagaStatus{
		ID:    saga.ID,
		State: string(saga.State),
		Steps: steps,
	}, nil
}

// Ensure Manager implements the interface at compile time
var _ rabbitmq.SagaOrchestrator = (*Manager)(nil)
