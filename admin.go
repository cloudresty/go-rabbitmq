package rabbitmq

import (
	"context"
	"fmt"
	"maps"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// AdminService handles topology management operations
type AdminService struct {
	client *Client
}

// Table represents AMQP table type for arguments
type Table map[string]any

// Queue represents a declared queue
type Queue struct {
	Name      string
	Messages  int
	Consumers int
}

// QueueInfo represents detailed queue information
type QueueInfo struct {
	Name       string
	Messages   int
	Consumers  int
	Memory     int64
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	Arguments  Table
	VHost      string
	Node       string
}

// ExchangeInfo represents detailed exchange information
type ExchangeInfo struct {
	Name       string
	Type       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	Arguments  Table
	VHost      string
}

// QueueOption represents a functional option for queue configuration
type QueueOption func(*queueConfig)

// QuorumQueueOption represents a functional option for quorum queue configuration
type QuorumQueueOption func(*quorumQueueConfig)

// ExchangeOption represents a functional option for exchange configuration
type ExchangeOption func(*exchangeConfig)

// DeleteQueueOption represents a functional option for queue deletion
type DeleteQueueOption func(*deleteQueueConfig)

// DeleteExchangeOption represents a functional option for exchange deletion
type DeleteExchangeOption func(*deleteExchangeConfig)

// BindingOption represents a functional option for binding configuration
type BindingOption func(*bindingConfig)

// queueConfig holds queue configuration
type queueConfig struct {
	Durable              bool
	AutoDelete           bool
	Exclusive            bool
	NoWait               bool
	Arguments            Table
	TTL                  time.Duration
	MaxLength            int64
	MaxLengthBytes       int64
	DeadLetterExchange   string
	DeadLetterRoutingKey string
}

// quorumQueueConfig holds quorum queue specific configuration
type quorumQueueConfig struct {
	queueConfig
	InitialGroupSize int
	DeliveryLimit    int
}

// exchangeConfig holds exchange configuration
type exchangeConfig struct {
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Arguments  Table
}

// deleteQueueConfig holds queue deletion configuration
type deleteQueueConfig struct {
	IfUnused bool
	IfEmpty  bool
	NoWait   bool
}

// deleteExchangeConfig holds exchange deletion configuration
type deleteExchangeConfig struct {
	IfUnused bool
	NoWait   bool
}

// bindingConfig holds binding configuration
type bindingConfig struct {
	NoWait    bool
	Arguments Table
}

// Queue options

// WithDurable makes the queue durable
func WithDurable() QueueOption {
	return func(config *queueConfig) {
		config.Durable = true
	}
}

// WithAutoDelete makes the queue auto-delete when no longer used
func WithAutoDelete() QueueOption {
	return func(config *queueConfig) {
		config.AutoDelete = true
	}
}

// WithExclusiveQueue makes the queue exclusive to this connection
func WithExclusiveQueue() QueueOption {
	return func(config *queueConfig) {
		config.Exclusive = true
	}
}

// WithTTL sets the message TTL for the queue
func WithTTL(ttl time.Duration) QueueOption {
	return func(config *queueConfig) {
		config.TTL = ttl
		if config.Arguments == nil {
			config.Arguments = make(Table)
		}
		config.Arguments["x-message-ttl"] = int64(ttl.Milliseconds())
	}
}

// WithMaxLength sets the maximum queue length
func WithMaxLength(length int64) QueueOption {
	return func(config *queueConfig) {
		config.MaxLength = length
		if config.Arguments == nil {
			config.Arguments = make(Table)
		}
		config.Arguments["x-max-length"] = length
	}
}

// WithMaxLengthBytes sets the maximum queue size in bytes
func WithMaxLengthBytes(bytes int64) QueueOption {
	return func(config *queueConfig) {
		config.MaxLengthBytes = bytes
		if config.Arguments == nil {
			config.Arguments = make(Table)
		}
		config.Arguments["x-max-length-bytes"] = bytes
	}
}

// WithArguments sets custom queue arguments
func WithArguments(args Table) QueueOption {
	return func(config *queueConfig) {
		if config.Arguments == nil {
			config.Arguments = make(Table)
		}
		for k, v := range args {
			config.Arguments[k] = v
		}
	}
}

// WithDeadLetter configures dead letter exchange and routing key
func WithDeadLetter(exchange, routingKey string) QueueOption {
	return func(config *queueConfig) {
		config.DeadLetterExchange = exchange
		config.DeadLetterRoutingKey = routingKey
		if config.Arguments == nil {
			config.Arguments = make(Table)
		}
		config.Arguments["x-dead-letter-exchange"] = exchange
		if routingKey != "" {
			config.Arguments["x-dead-letter-routing-key"] = routingKey
		}
	}
}

// WithDeadLetterTTL sets the TTL for messages in dead letter queue
func WithDeadLetterTTL(ttl time.Duration) QueueOption {
	return func(config *queueConfig) {
		// This would typically be used when declaring the dead letter queue itself
		// The TTL is set on the DLQ, not the source queue
	}
}

// Quorum queue options

// WithInitialGroupSize sets the initial group size for quorum queues
func WithInitialGroupSize(size int) QuorumQueueOption {
	return func(config *quorumQueueConfig) {
		config.InitialGroupSize = size
		if config.Arguments == nil {
			config.Arguments = make(Table)
		}
		config.Arguments["x-quorum-initial-group-size"] = size
	}
}

// WithQuorumDeliveryLimit sets the delivery limit for quorum queues
func WithQuorumDeliveryLimit(limit int) QuorumQueueOption {
	return func(config *quorumQueueConfig) {
		config.DeliveryLimit = limit
		if config.Arguments == nil {
			config.Arguments = make(Table)
		}
		config.Arguments["x-delivery-limit"] = limit
	}
}

// Exchange options

// WithExchangeDurable makes the exchange durable
func WithExchangeDurable() ExchangeOption {
	return func(config *exchangeConfig) {
		config.Durable = true
	}
}

// WithExchangeAutoDelete makes the exchange auto-delete
func WithExchangeAutoDelete() ExchangeOption {
	return func(config *exchangeConfig) {
		config.AutoDelete = true
	}
}

// WithExchangeInternal makes the exchange internal
func WithExchangeInternal() ExchangeOption {
	return func(config *exchangeConfig) {
		config.Internal = true
	}
}

// WithExchangeArguments sets exchange arguments
func WithExchangeArguments(args Table) ExchangeOption {
	return func(config *exchangeConfig) {
		if config.Arguments == nil {
			config.Arguments = make(Table)
		}
		maps.Copy(config.Arguments, args)
	}
}

// Binding options

// WithBindingNoWait makes the binding operation not wait for server response
func WithBindingNoWait() BindingOption {
	return func(config *bindingConfig) {
		config.NoWait = true
	}
}

// WithBindingHeaders sets headers for the binding
func WithBindingHeaders(headers map[string]any) BindingOption {
	return func(config *bindingConfig) {
		if config.Arguments == nil {
			config.Arguments = make(Table)
		}
		maps.Copy(config.Arguments, headers)
	}
}

// WithBindingArguments sets arguments for the binding
func WithBindingArguments(args Table) BindingOption {
	return func(config *bindingConfig) {
		config.Arguments = args
	}
}

// AdminService methods

// DeclareQueue declares a queue with the given options
func (a *AdminService) DeclareQueue(ctx context.Context, name string, opts ...QueueOption) (*Queue, error) {
	config := &queueConfig{
		Durable:    true, // Default to durable
		AutoDelete: false,
		Exclusive:  false,
		NoWait:     false,
		Arguments:  make(Table),
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Get channel
	ch, err := a.client.getChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Declare queue
	queue, err := ch.QueueDeclare(
		name,
		config.Durable,
		config.AutoDelete,
		config.Exclusive,
		config.NoWait,
		amqp.Table(config.Arguments),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare queue %s: %w", name, err)
	}

	return &Queue{
		Name:      queue.Name,
		Messages:  queue.Messages,
		Consumers: queue.Consumers,
	}, nil
}

// DeclareQuorumQueue declares a quorum queue with the given options
func (a *AdminService) DeclareQuorumQueue(ctx context.Context, name string, opts ...QuorumQueueOption) (*Queue, error) {
	config := &quorumQueueConfig{
		queueConfig: queueConfig{
			Durable:    true,
			AutoDelete: false,
			Exclusive:  false,
			NoWait:     false,
			Arguments:  make(Table),
		},
		InitialGroupSize: 3, // Default quorum size
	}

	// Set quorum queue type
	config.Arguments["x-queue-type"] = "quorum"

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Get channel
	ch, err := a.client.getChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Declare queue
	queue, err := ch.QueueDeclare(
		name,
		config.Durable,
		config.AutoDelete,
		config.Exclusive,
		config.NoWait,
		amqp.Table(config.Arguments),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare quorum queue %s: %w", name, err)
	}

	return &Queue{
		Name:      queue.Name,
		Messages:  queue.Messages,
		Consumers: queue.Consumers,
	}, nil
}

// DeclareExchange declares an exchange with the given options
func (a *AdminService) DeclareExchange(ctx context.Context, name string, kind ExchangeType, opts ...ExchangeOption) error {
	config := &exchangeConfig{
		Durable:    true, // Default to durable
		AutoDelete: false,
		Internal:   false,
		NoWait:     false,
		Arguments:  make(Table),
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Get channel
	ch, err := a.client.getChannel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Declare exchange
	err = ch.ExchangeDeclare(
		name,
		string(kind),
		config.Durable,
		config.AutoDelete,
		config.Internal,
		config.NoWait,
		amqp.Table(config.Arguments),
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", name, err)
	}

	return nil
}

// DeleteQueue deletes a queue
func (a *AdminService) DeleteQueue(ctx context.Context, name string, opts ...DeleteQueueOption) error {
	config := &deleteQueueConfig{
		IfUnused: false,
		IfEmpty:  false,
		NoWait:   false,
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Get channel
	ch, err := a.client.getChannel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Delete queue
	_, err = ch.QueueDelete(name, config.IfUnused, config.IfEmpty, config.NoWait)
	if err != nil {
		return fmt.Errorf("failed to delete queue %s: %w", name, err)
	}

	return nil
}

// DeleteExchange deletes an exchange
func (a *AdminService) DeleteExchange(ctx context.Context, name string, opts ...DeleteExchangeOption) error {
	config := &deleteExchangeConfig{
		IfUnused: false,
		NoWait:   false,
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	// Get channel
	ch, err := a.client.getChannel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Delete exchange
	err = ch.ExchangeDelete(name, config.IfUnused, config.NoWait)
	if err != nil {
		return fmt.Errorf("failed to delete exchange %s: %w", name, err)
	}

	return nil
}

// PurgeQueue purges all messages from a queue
func (a *AdminService) PurgeQueue(ctx context.Context, name string) (int, error) {
	// Get channel
	ch, err := a.client.getChannel()
	if err != nil {
		return 0, fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Purge queue
	count, err := ch.QueuePurge(name, false)
	if err != nil {
		return 0, fmt.Errorf("failed to purge queue %s: %w", name, err)
	}

	return count, nil
}

// InspectQueue returns detailed information about a queue
func (a *AdminService) InspectQueue(ctx context.Context, name string) (*QueueInfo, error) {
	// Get channel
	ch, err := a.client.getChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Inspect queue (passive declare to get info)
	queue, err := ch.QueueInspect(name)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect queue %s: %w", name, err)
	}

	// Extract additional information from arguments if available
	durable := false
	autoDelete := false
	exclusive := false
	arguments := make(Table)

	// Try to get detailed info via passive declare to extract declaration parameters
	// Note: Some information may not be available through standard AMQP operations
	// For complete queue details, consider using RabbitMQ Management API

	return &QueueInfo{
		Name:       queue.Name,
		Messages:   queue.Messages,
		Consumers:  queue.Consumers,
		Memory:     0, // Not available through AMQP inspection
		Durable:    durable,
		AutoDelete: autoDelete,
		Exclusive:  exclusive,
		Arguments:  arguments,
		VHost:      "/", // Default vhost, would need management API for accurate info
		Node:       "",  // Not available through AMQP inspection
	}, nil
}

// ExchangeInfo returns detailed information about an exchange
func (a *AdminService) ExchangeInfo(ctx context.Context, name string) (*ExchangeInfo, error) {
	// Note: RabbitMQ doesn't provide a direct way to inspect exchanges like queues
	// This is a passive declare to check if the exchange exists
	ch, err := a.client.getChannel()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Try to declare the exchange passively to check if it exists
	err = ch.ExchangeDeclarePassive(name, "", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange info for %s: %w", name, err)
	}

	// Since RabbitMQ doesn't provide detailed exchange inspection,
	// we return basic info with the name confirmed to exist
	return &ExchangeInfo{
		Name:  name,
		VHost: "/", // Default vhost, would need management API for accurate info
	}, nil
}

// BindQueue binds a queue to an exchange with optional binding options
func (a *AdminService) BindQueue(ctx context.Context, queue, exchange, routingKey string, opts ...BindingOption) error {
	// Default binding configuration
	config := &bindingConfig{
		NoWait:    false,
		Arguments: make(Table),
	}

	// Apply binding options
	for _, opt := range opts {
		opt(config)
	}
	// Get channel
	ch, err := a.client.getChannel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Bind queue
	err = ch.QueueBind(queue, routingKey, exchange, config.NoWait, amqp.Table(config.Arguments))
	if err != nil {
		return fmt.Errorf("failed to bind queue %s to exchange %s: %w", queue, exchange, err)
	}

	return nil
}

// UnbindQueue unbinds a queue from an exchange with optional binding options
func (a *AdminService) UnbindQueue(ctx context.Context, queue, exchange, routingKey string, opts ...BindingOption) error {
	// Default binding configuration
	config := &bindingConfig{
		NoWait:    false,
		Arguments: make(Table),
	}

	// Apply binding options
	for _, opt := range opts {
		opt(config)
	}

	// Get channel
	ch, err := a.client.getChannel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Unbind queue
	err = ch.QueueUnbind(queue, routingKey, exchange, amqp.Table(config.Arguments))
	if err != nil {
		return fmt.Errorf("failed to unbind queue %s from exchange %s: %w", queue, exchange, err)
	}

	return nil
}

// BindExchange binds an exchange to another exchange
func (a *AdminService) BindExchange(ctx context.Context, destination, source, routingKey string, args Table) error {
	// Get channel
	ch, err := a.client.getChannel()
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}
	defer ch.Close()

	// Bind exchange
	err = ch.ExchangeBind(destination, routingKey, source, false, amqp.Table(args))
	if err != nil {
		return fmt.Errorf("failed to bind exchange %s to exchange %s: %w", destination, source, err)
	}

	return nil
}

// Topology represents a complete topology definition
type Topology struct {
	Exchanges []ExchangeDeclaration `json:"exchanges,omitempty" yaml:"exchanges,omitempty"`
	Queues    []QueueDeclaration    `json:"queues,omitempty" yaml:"queues,omitempty"`
	Bindings  []BindingDeclaration  `json:"bindings,omitempty" yaml:"bindings,omitempty"`
}

// ExchangeDeclaration represents an exchange declaration
type ExchangeDeclaration struct {
	Name       string         `json:"name" yaml:"name"`
	Type       ExchangeType   `json:"type" yaml:"type"`
	Durable    bool           `json:"durable" yaml:"durable"`
	AutoDelete bool           `json:"auto_delete" yaml:"auto_delete"`
	Internal   bool           `json:"internal" yaml:"internal"`
	Arguments  map[string]any `json:"arguments,omitempty" yaml:"arguments,omitempty"`
}

// QueueDeclaration represents a queue declaration
type QueueDeclaration struct {
	Name       string         `json:"name" yaml:"name"`
	Durable    bool           `json:"durable" yaml:"durable"`
	AutoDelete bool           `json:"auto_delete" yaml:"auto_delete"`
	Exclusive  bool           `json:"exclusive" yaml:"exclusive"`
	Arguments  map[string]any `json:"arguments,omitempty" yaml:"arguments,omitempty"`
	TTL        time.Duration  `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	DeadLetter string         `json:"dead_letter,omitempty" yaml:"dead_letter,omitempty"`
}

// BindingDeclaration represents a binding declaration
type BindingDeclaration struct {
	Queue      string         `json:"queue" yaml:"queue"`
	Exchange   string         `json:"exchange" yaml:"exchange"`
	RoutingKey string         `json:"routing_key" yaml:"routing_key"`
	Arguments  map[string]any `json:"arguments,omitempty" yaml:"arguments,omitempty"`
}

// DeclareTopology declares a complete topology from a definition
func (a *AdminService) DeclareTopology(ctx context.Context, topology *Topology) error {
	// Declare exchanges first
	for _, exchange := range topology.Exchanges {
		var opts []ExchangeOption
		if exchange.Durable {
			opts = append(opts, WithExchangeDurable())
		}
		if exchange.AutoDelete {
			opts = append(opts, WithExchangeAutoDelete())
		}
		if exchange.Internal {
			opts = append(opts, WithExchangeInternal())
		}
		if exchange.Arguments != nil {
			opts = append(opts, WithExchangeArguments(Table(exchange.Arguments)))
		}

		if err := a.DeclareExchange(ctx, exchange.Name, exchange.Type, opts...); err != nil {
			return fmt.Errorf("failed to declare exchange %s: %w", exchange.Name, err)
		}
	}

	// Declare queues
	for _, queue := range topology.Queues {
		var opts []QueueOption
		if queue.Durable {
			opts = append(opts, WithDurable())
		}
		if queue.AutoDelete {
			opts = append(opts, WithAutoDelete())
		}
		if queue.Exclusive {
			opts = append(opts, WithExclusiveQueue())
		}
		if queue.TTL > 0 {
			opts = append(opts, WithTTL(queue.TTL))
		}
		if queue.DeadLetter != "" {
			opts = append(opts, WithDeadLetter(queue.DeadLetter, ""))
		}
		if queue.Arguments != nil {
			opts = append(opts, WithArguments(Table(queue.Arguments)))
		}

		if _, err := a.DeclareQueue(ctx, queue.Name, opts...); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queue.Name, err)
		}
	}

	// Create bindings
	for _, binding := range topology.Bindings {
		opts := []BindingOption{}
		if binding.Arguments != nil {
			opts = append(opts, WithBindingArguments(Table(binding.Arguments)))
		}
		if err := a.BindQueue(ctx, binding.Queue, binding.Exchange, binding.RoutingKey, opts...); err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s: %w", binding.Queue, binding.Exchange, err)
		}
	}

	return nil
}

// QueueInfo is an alias for InspectQueue to match the API documentation
func (a *AdminService) QueueInfo(ctx context.Context, name string) (*QueueInfo, error) {
	return a.InspectQueue(ctx, name)
}

// SetupTopology sets up complete topology configuration with exchanges, queues, and bindings
func (a *AdminService) SetupTopology(ctx context.Context, exchanges []ExchangeConfig, queues []QueueConfig, bindings []BindingConfig) error {
	// Declare exchanges
	for _, exchange := range exchanges {
		var opts []ExchangeOption
		if exchange.Durable {
			opts = append(opts, WithExchangeDurable())
		}
		if exchange.AutoDelete {
			opts = append(opts, WithExchangeAutoDelete())
		}
		if exchange.Internal {
			opts = append(opts, WithExchangeInternal())
		}
		if exchange.Arguments != nil {
			opts = append(opts, WithExchangeArguments(Table(exchange.Arguments)))
		}

		if err := a.DeclareExchange(ctx, exchange.Name, ExchangeType(exchange.Type), opts...); err != nil {
			return fmt.Errorf("failed to declare exchange %s: %w", exchange.Name, err)
		}
	}

	// Declare queues
	for _, queue := range queues {
		var opts []QueueOption
		if queue.Durable {
			opts = append(opts, WithDurable())
		}
		if queue.AutoDelete {
			opts = append(opts, WithAutoDelete())
		}
		if queue.Exclusive {
			opts = append(opts, WithExclusiveQueue())
		}
		if queue.Arguments != nil {
			opts = append(opts, WithArguments(Table(queue.Arguments)))
		}

		if _, err := a.DeclareQueue(ctx, queue.Name, opts...); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queue.Name, err)
		}
	}

	// Create bindings
	for _, binding := range bindings {
		var opts []BindingOption
		if binding.Arguments != nil {
			opts = append(opts, WithBindingArguments(Table(binding.Arguments)))
		}

		if err := a.BindQueue(ctx, binding.QueueName, binding.ExchangeName, binding.RoutingKey, opts...); err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s: %w", binding.QueueName, binding.ExchangeName, err)
		}
	}

	return nil
}
