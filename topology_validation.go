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

// ValidateCompleteTopology performs a comprehensive validation of all registered topology
// This method can be called on-demand to ensure entire topology is consistent
func (v *TopologyValidator) ValidateCompleteTopology() error {
	if !v.IsEnabled() {
		return nil // Validation disabled
	}

	if v.client.config.Logger != nil {
		v.client.config.Logger.Info("Starting comprehensive topology validation")
	}

	// Use the same intelligent validation as background validation
	v.performBackgroundValidation()

	if v.client.config.Logger != nil {
		v.client.config.Logger.Info("Completed comprehensive topology validation")
	}

	return nil
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

	// Smart dependency validation - ensure all required exchanges exist first
	if config.DeadLetterExchange != "" {
		if err := v.ensureExchangeExists(config.DeadLetterExchange); err != nil {
			return fmt.Errorf("failed to ensure dead-letter-exchange '%s' exists for queue '%s': %w",
				config.DeadLetterExchange, name, err)
		}
	}

	// Ensure exchanges that this queue is bound to exist
	bindings := v.registry.ListBindings()
	for _, binding := range bindings {
		if binding.QueueName == name {
			if err := v.ensureExchangeExists(binding.ExchangeName); err != nil {
				return fmt.Errorf("failed to ensure exchange '%s' exists for queue '%s' binding: %w",
					binding.ExchangeName, name, err)
			}
		}
	}

	// Check if queue exists and validate its configuration
	exists, err := v.admin.QueueExists(context.TODO(), name)
	if err != nil {
		return fmt.Errorf("failed to check if queue '%s' exists: %w", name, err)
	}

	if !exists {
		if v.IsAutoRecreateEnabled() {
			// Queue genuinely doesn't exist - safe to recreate
			return v.recreateQueue(config)
		}
		return fmt.Errorf("queue '%s' does not exist and auto-recreation is disabled", name)
	}

	// Queue exists - NEVER delete existing queues due to configuration differences!
	// Instead, sync our registry with the actual RabbitMQ configuration to prevent disruption
	actualInfo, err := v.admin.InspectQueue(context.TODO(), name)
	if err != nil {
		// If we can't inspect, assume it's fine since it exists
		return nil
	}

	// Check if the actual configuration matches our registry
	actualDLE := ""
	actualDLRK := ""
	if actualInfo.Arguments != nil {
		if dle, ok := actualInfo.Arguments["x-dead-letter-exchange"]; ok {
			if dlExchange, ok := dle.(string); ok {
				actualDLE = dlExchange
			}
		}
		if dlrk, ok := actualInfo.Arguments["x-dead-letter-routing-key"]; ok {
			if dlRoutingKey, ok := dlrk.(string); ok {
				actualDLRK = dlRoutingKey
			}
		}
	}

	// Check for configuration mismatches that require registry updates
	if actualDLE != config.DeadLetterExchange || actualDLRK != config.DeadLetterRoutingKey {
		if v.client.config.Logger != nil {
			v.client.config.Logger.Info("Queue configuration differs from registry, updating registry to match reality",
				"queue", name,
				"expected_dle", config.DeadLetterExchange,
				"actual_dle", actualDLE,
				"expected_dlrk", config.DeadLetterRoutingKey,
				"actual_dlrk", actualDLRK)
		}

		// Update registry to match actual configuration instead of forcing recreation
		updatedConfig := config
		updatedConfig.DeadLetterExchange = actualDLE
		updatedConfig.DeadLetterRoutingKey = actualDLRK
		updatedConfig.Arguments = actualInfo.Arguments
		v.registry.RegisterQueue(updatedConfig)
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

// performBackgroundValidation performs intelligent topology drift detection and correction
func (v *TopologyValidator) performBackgroundValidation() {
	if v.client.config.Logger != nil {
		v.client.config.Logger.Debug("Starting intelligent topology validation and drift correction")
	}

	// Phase 1: Validate and recreate exchanges (foundations first)
	v.validateAndRecreateExchanges()

	// Phase 2: Validate and recreate queues (after their dependent exchanges exist)
	v.validateAndRecreateQueues()

	// Phase 3: Validate and recreate bindings (after both exchanges and queues exist)
	v.validateAndRecreateBindings()

	if v.client.config.Logger != nil {
		v.client.config.Logger.Debug("Completed intelligent topology validation")
	}
}

// validateAndRecreateExchanges ensures all registered exchanges exist
func (v *TopologyValidator) validateAndRecreateExchanges() {
	exchanges := v.registry.ListExchanges()
	if len(exchanges) == 0 {
		return
	}

	if v.client.config.Logger != nil {
		v.client.config.Logger.Debug("Validating exchanges", "count", len(exchanges))
	}

	for _, exchange := range exchanges {
		exists, err := v.admin.ExchangeExists(context.TODO(), exchange.Name)
		if err != nil {
			if v.client.config.Logger != nil {
				v.client.config.Logger.Error("Failed to check exchange existence",
					"exchange", exchange.Name,
					"error", err)
			}
			continue
		}

		if !exists {
			if v.client.config.Logger != nil {
				v.client.config.Logger.Info("Exchange missing, recreating",
					"exchange", exchange.Name,
					"type", exchange.Type)
			}

			if err := v.recreateExchange(exchange); err != nil {
				if v.client.config.Logger != nil {
					v.client.config.Logger.Error("Failed to recreate missing exchange",
						"exchange", exchange.Name,
						"error", err)
				}
			} else {
				if v.client.config.Logger != nil {
					v.client.config.Logger.Info("Successfully recreated exchange",
						"exchange", exchange.Name)
				}
			}
		}
	}
}

// validateAndRecreateQueues ensures all registered queues exist and validates their dependencies
func (v *TopologyValidator) validateAndRecreateQueues() {
	queues := v.registry.ListQueues()
	if len(queues) == 0 {
		return
	}

	if v.client.config.Logger != nil {
		v.client.config.Logger.Debug("Validating queues", "count", len(queues))
	}

	for _, queue := range queues {
		exists, err := v.admin.QueueExists(context.TODO(), queue.Name)
		if err != nil {
			if v.client.config.Logger != nil {
				v.client.config.Logger.Error("Failed to check queue existence",
					"queue", queue.Name,
					"error", err)
			}
			continue
		}

		if !exists {
			// Before recreating queue, ensure its dead-letter-exchange exists
			if queue.DeadLetterExchange != "" {
				if err := v.ensureExchangeExists(queue.DeadLetterExchange); err != nil {
					if v.client.config.Logger != nil {
						v.client.config.Logger.Error("Failed to ensure dead-letter-exchange exists before queue recreation",
							"queue", queue.Name,
							"dead_letter_exchange", queue.DeadLetterExchange,
							"error", err)
					}
					continue
				}
			}

			if v.client.config.Logger != nil {
				v.client.config.Logger.Info("Queue missing, recreating",
					"queue", queue.Name,
					"type", queue.QueueType,
					"dead_letter_exchange", queue.DeadLetterExchange)
			}

			if err := v.recreateQueue(queue); err != nil {
				if v.client.config.Logger != nil {
					v.client.config.Logger.Error("Failed to recreate missing queue",
						"queue", queue.Name,
						"error", err)
				}
			} else {
				if v.client.config.Logger != nil {
					v.client.config.Logger.Info("Successfully recreated queue",
						"queue", queue.Name)
				}
			}
		}
	}
}

// validateAndRecreateBindings ensures all registered bindings exist
func (v *TopologyValidator) validateAndRecreateBindings() {
	bindings := v.registry.ListBindings()
	if len(bindings) == 0 {
		return
	}

	if v.client.config.Logger != nil {
		v.client.config.Logger.Debug("Validating bindings", "count", len(bindings))
	}

	for _, binding := range bindings {
		// Ensure both exchange and queue exist before checking binding
		if err := v.ensureExchangeExists(binding.ExchangeName); err != nil {
			if v.client.config.Logger != nil {
				v.client.config.Logger.Error("Cannot validate binding - exchange missing",
					"binding", fmt.Sprintf("%s->%s:%s", binding.QueueName, binding.ExchangeName, binding.RoutingKey),
					"exchange", binding.ExchangeName,
					"error", err)
			}
			continue
		}

		queueExists, err := v.admin.QueueExists(context.TODO(), binding.QueueName)
		if err != nil || !queueExists {
			if v.client.config.Logger != nil {
				v.client.config.Logger.Error("Cannot validate binding - queue missing",
					"binding", fmt.Sprintf("%s->%s:%s", binding.QueueName, binding.ExchangeName, binding.RoutingKey),
					"queue", binding.QueueName,
					"error", err)
			}
			continue
		}

		// Check if binding exists (we don't have a direct API for this, so we'll attempt to recreate)
		var opts []BindingOption
		if len(binding.Arguments) > 0 {
			opts = append(opts, WithBindingArguments(binding.Arguments))
		}

		// Attempt to bind - if it already exists, this is idempotent
		err = v.admin.BindQueue(context.TODO(), binding.QueueName, binding.ExchangeName, binding.RoutingKey, opts...)
		if err != nil {
			if v.client.config.Logger != nil {
				v.client.config.Logger.Error("Failed to ensure binding exists",
					"binding", fmt.Sprintf("%s->%s:%s", binding.QueueName, binding.ExchangeName, binding.RoutingKey),
					"error", err)
			}
		} else {
			if v.client.config.Logger != nil {
				v.client.config.Logger.Debug("Ensured binding exists",
					"binding", fmt.Sprintf("%s->%s:%s", binding.QueueName, binding.ExchangeName, binding.RoutingKey))
			}
		}
	}
}

// ensureExchangeExists checks if an exchange exists and recreates it if missing
func (v *TopologyValidator) ensureExchangeExists(exchangeName string) error {
	// Skip default exchange
	if exchangeName == "" {
		return nil
	}

	exists, err := v.admin.ExchangeExists(context.TODO(), exchangeName)
	if err != nil {
		return fmt.Errorf("failed to check exchange existence: %w", err)
	}

	if !exists {
		// Try to find the exchange configuration in registry
		config, found := v.registry.GetExchange(exchangeName)
		if !found {
			// Create a default durable topic exchange if not in registry
			config = ExchangeConfig{
				Name:    exchangeName,
				Type:    ExchangeTypeTopic,
				Durable: true,
			}
			if v.client.config.Logger != nil {
				v.client.config.Logger.Info("Exchange not in registry, creating with default configuration",
					"exchange", exchangeName,
					"type", "topic",
					"durable", true)
			}
		}

		if err := v.recreateExchange(config); err != nil {
			return fmt.Errorf("failed to recreate exchange '%s': %w", exchangeName, err)
		}

		if v.client.config.Logger != nil {
			v.client.config.Logger.Info("Successfully ensured exchange exists",
				"exchange", exchangeName)
		}
	}

	return nil
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
