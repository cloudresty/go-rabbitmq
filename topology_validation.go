package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TopologyRegistry tracks declared topology for validation and auto-recreation
type TopologyRegistry struct {
	exchanges map[string]ExchangeConfig
	queues    map[string]QueueConfig
	bindings  map[string]BindingConfig
	mu        sync.RWMutex
}

// NewTopologyRegistry creates a new topology registry
func NewTopologyRegistry() *TopologyRegistry {
	return &TopologyRegistry{
		exchanges: make(map[string]ExchangeConfig),
		queues:    make(map[string]QueueConfig),
		bindings:  make(map[string]BindingConfig),
	}
}

// RegisterExchange records an exchange declaration for tracking
func (r *TopologyRegistry) RegisterExchange(config ExchangeConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exchanges[config.Name] = config
}

// RegisterQueue records a queue declaration for tracking
func (r *TopologyRegistry) RegisterQueue(config QueueConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queues[config.Name] = config
}

// RegisterBinding records a binding declaration for tracking
func (r *TopologyRegistry) RegisterBinding(config BindingConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s->%s:%s", config.QueueName, config.ExchangeName, config.RoutingKey)
	r.bindings[key] = config
}

// GetExchange retrieves a registered exchange configuration
func (r *TopologyRegistry) GetExchange(name string) (ExchangeConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	config, exists := r.exchanges[name]
	return config, exists
}

// GetQueue retrieves a registered queue configuration
func (r *TopologyRegistry) GetQueue(name string) (QueueConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	config, exists := r.queues[name]
	return config, exists
}

// GetBinding retrieves a registered binding configuration
func (r *TopologyRegistry) GetBinding(queue, exchange, routingKey string) (BindingConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := fmt.Sprintf("%s->%s:%s", queue, exchange, routingKey)
	config, exists := r.bindings[key]
	return config, exists
}

// ListExchanges returns all registered exchanges
func (r *TopologyRegistry) ListExchanges() []ExchangeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	exchanges := make([]ExchangeConfig, 0, len(r.exchanges))
	for _, config := range r.exchanges {
		exchanges = append(exchanges, config)
	}
	return exchanges
}

// ListQueues returns all registered queues
func (r *TopologyRegistry) ListQueues() []QueueConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	queues := make([]QueueConfig, 0, len(r.queues))
	for _, config := range r.queues {
		queues = append(queues, config)
	}
	return queues
}

// ListBindings returns all registered bindings
func (r *TopologyRegistry) ListBindings() []BindingConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	bindings := make([]BindingConfig, 0, len(r.bindings))
	for _, config := range r.bindings {
		bindings = append(bindings, config)
	}
	return bindings
}

// Clear removes all registered topology
func (r *TopologyRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exchanges = make(map[string]ExchangeConfig)
	r.queues = make(map[string]QueueConfig)
	r.bindings = make(map[string]BindingConfig)
}

// TopologyValidator handles topology validation and auto-recreation
type TopologyValidator struct {
	client   *Client
	admin    *AdminService
	registry *TopologyRegistry

	// Configuration
	enabled           bool
	autoRecreate      bool
	validationTimeout time.Duration

	// Background validation
	backgroundEnabled  bool
	validationInterval time.Duration
	stopCh             chan struct{}
	mu                 sync.RWMutex
}

// NewTopologyValidator creates a new topology validator
func NewTopologyValidator(client *Client, admin *AdminService, registry *TopologyRegistry) *TopologyValidator {
	return &TopologyValidator{
		client:             client,
		admin:              admin,
		registry:           registry,
		enabled:            false,
		autoRecreate:       false,
		validationTimeout:  5 * time.Second,
		backgroundEnabled:  false,
		validationInterval: 30 * time.Second,
		stopCh:             make(chan struct{}),
	}
}

// Enable enables topology validation
func (v *TopologyValidator) Enable() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.enabled = true
}

// EnableAutoRecreate enables automatic recreation of missing topology
func (v *TopologyValidator) EnableAutoRecreate() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.autoRecreate = true
}

// EnableBackgroundValidation starts background topology validation
func (v *TopologyValidator) EnableBackgroundValidation(interval time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.backgroundEnabled {
		return // Already running
	}

	v.backgroundEnabled = true
	v.validationInterval = interval

	go v.backgroundValidationLoop()
}

// Disable disables topology validation
func (v *TopologyValidator) Disable() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.enabled = false
	v.autoRecreate = false

	if v.backgroundEnabled {
		v.backgroundEnabled = false
		close(v.stopCh)
		v.stopCh = make(chan struct{}) // Reset for potential restart
	}
}

// IsEnabled returns whether topology validation is enabled
func (v *TopologyValidator) IsEnabled() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.enabled
}

// IsAutoRecreateEnabled returns whether auto-recreation is enabled
func (v *TopologyValidator) IsAutoRecreateEnabled() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.autoRecreate
}

// ValidateExchange checks if an exchange exists and optionally recreates it
func (v *TopologyValidator) ValidateExchange(name string) error {
	if !v.IsEnabled() {
		return nil // Validation disabled
	}

	// Get registered exchange configuration
	config, exists := v.registry.GetExchange(name)
	if !exists {
		// Exchange not registered, skip validation
		return nil
	}

	// Check if exchange exists using passive declaration
	exists, err := v.admin.ExchangeExists(context.TODO(), name)
	if err != nil {
		return fmt.Errorf("failed to check if exchange '%s' exists: %w", name, err)
	}

	if !exists {
		if v.IsAutoRecreateEnabled() {
			// Auto-recreate the exchange
			return v.recreateExchange(config)
		}
		return fmt.Errorf("exchange '%s' does not exist and auto-recreation is disabled", name)
	}

	return nil
}

// ValidateQueue checks if a queue exists and optionally recreates it
func (v *TopologyValidator) ValidateQueue(name string) error {
	if !v.IsEnabled() {
		return nil // Validation disabled
	}

	// Get registered queue configuration
	config, exists := v.registry.GetQueue(name)
	if !exists {
		// Queue not registered, skip validation
		return nil
	}

	// Check if queue exists using passive declaration
	exists, err := v.admin.QueueExists(context.TODO(), name)
	if err != nil {
		return fmt.Errorf("failed to check if queue '%s' exists: %w", name, err)
	}

	if !exists {
		if v.IsAutoRecreateEnabled() {
			// Before recreating the queue, ensure dependent exchanges exist
			if err := v.validateQueueDependencies(config); err != nil {
				return fmt.Errorf("failed to validate queue dependencies for '%s': %w", name, err)
			}

			// Auto-recreate the queue
			return v.recreateQueue(config)
		}
		return fmt.Errorf("queue '%s' does not exist and auto-recreation is disabled", name)
	}

	return nil
}

// recreateExchange recreates a missing exchange
func (v *TopologyValidator) recreateExchange(config ExchangeConfig) error {
	var opts []ExchangeOption
	if config.Durable {
		opts = append(opts, WithExchangeDurable())
	}
	if config.AutoDelete {
		opts = append(opts, WithExchangeAutoDelete())
	}
	if config.Internal {
		opts = append(opts, WithExchangeInternal())
	}
	if len(config.Arguments) > 0 {
		opts = append(opts, WithExchangeArguments(config.Arguments))
	}

	// Recreate the exchange
	err := v.admin.DeclareExchange(context.TODO(), config.Name, config.Type, opts...)
	if err != nil {
		return err
	}

	// Restore all bindings for this exchange
	return v.recreateExchangeBindings(config.Name)
}

// recreateQueue recreates a missing queue
func (v *TopologyValidator) recreateQueue(config QueueConfig) error {
	// Convert QueueConfig to queue options
	var opts []QueueOption
	if config.Durable {
		opts = append(opts, WithDurable())
	}
	if config.AutoDelete {
		opts = append(opts, WithAutoDelete())
	}
	if config.Exclusive {
		opts = append(opts, WithExclusiveQueue())
	}
	if len(config.Arguments) > 0 {
		opts = append(opts, WithArguments(config.Arguments))
	}
	if config.DeadLetterExchange != "" {
		opts = append(opts, WithDeadLetter(config.DeadLetterExchange, config.DeadLetterRoutingKey))
	}

	// Recreate the queue
	_, err := v.admin.DeclareQueue(context.TODO(), config.Name, opts...)
	if err != nil {
		return err
	}

	// Restore all bindings for this queue
	return v.recreateQueueBindings(config.Name)
}

// backgroundValidationLoop runs periodic topology validation in the background
func (v *TopologyValidator) backgroundValidationLoop() {
	ticker := time.NewTicker(v.validationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-v.stopCh:
			return
		case <-ticker.C:
			v.performBackgroundValidation()
		}
	}
}

// performBackgroundValidation validates all registered topology
func (v *TopologyValidator) performBackgroundValidation() {
	// Validate all registered exchanges
	for _, exchange := range v.registry.ListExchanges() {
		if err := v.ValidateExchange(exchange.Name); err != nil {
			// Log error (using client's logger if available)
			if v.client.config.Logger != nil {
				v.client.config.Logger.Error("Background topology validation failed for exchange",
					"exchange", exchange.Name,
					"error", err)
			}
		}
	}

	// Validate all registered queues
	for _, queue := range v.registry.ListQueues() {
		if err := v.ValidateQueue(queue.Name); err != nil {
			// Log error (using client's logger if available)
			if v.client.config.Logger != nil {
				v.client.config.Logger.Error("Background topology validation failed for queue",
					"queue", queue.Name,
					"error", err)
			}
		}
	}
}

// recreateQueueBindings restores all bindings for a specific queue
func (v *TopologyValidator) recreateQueueBindings(queueName string) error {
	bindings := v.registry.ListBindings()
	for _, binding := range bindings {
		if binding.QueueName == queueName {
			var opts []BindingOption
			if len(binding.Arguments) > 0 {
				opts = append(opts, WithBindingArguments(binding.Arguments))
			}

			err := v.admin.BindQueue(context.TODO(), binding.QueueName, binding.ExchangeName, binding.RoutingKey, opts...)
			if err != nil {
				return fmt.Errorf("failed to restore binding %s->%s:%s: %w",
					binding.QueueName, binding.ExchangeName, binding.RoutingKey, err)
			}
		}
	}
	return nil
}

// recreateExchangeBindings restores all bindings for a specific exchange
func (v *TopologyValidator) recreateExchangeBindings(exchangeName string) error {
	bindings := v.registry.ListBindings()
	for _, binding := range bindings {
		if binding.ExchangeName == exchangeName {
			var opts []BindingOption
			if len(binding.Arguments) > 0 {
				opts = append(opts, WithBindingArguments(binding.Arguments))
			}

			err := v.admin.BindQueue(context.TODO(), binding.QueueName, binding.ExchangeName, binding.RoutingKey, opts...)
			if err != nil {
				return fmt.Errorf("failed to restore binding %s->%s:%s: %w",
					binding.QueueName, binding.ExchangeName, binding.RoutingKey, err)
			}
		}
	}
	return nil
}

// validateQueueDependencies ensures all dependent exchanges exist before queue recreation
func (v *TopologyValidator) validateQueueDependencies(config QueueConfig) error {
	// Validate dead-letter exchange if configured
	if config.DeadLetterExchange != "" {
		if err := v.ValidateExchange(config.DeadLetterExchange); err != nil {
			return fmt.Errorf("failed to validate dead-letter exchange '%s': %w", config.DeadLetterExchange, err)
		}
	}

	// Validate any bindings this queue depends on
	bindings := v.registry.ListBindings()
	for _, binding := range bindings {
		if binding.QueueName == config.Name {
			if err := v.ValidateExchange(binding.ExchangeName); err != nil {
				return fmt.Errorf("failed to validate exchange '%s' for binding: %w", binding.ExchangeName, err)
			}
		}
	}

	return nil
}
