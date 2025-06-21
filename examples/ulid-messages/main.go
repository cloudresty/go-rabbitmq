package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
	"github.com/cloudresty/ulid"
)

// OrderEvent represents a business event with ULID-based message ID
type OrderEvent struct {
	OrderID   string    `json:"order_id"`
	EventType string    `json:"event_type"`
	UserID    string    `json:"user_id"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Timestamp time.Time `json:"timestamp"`
	Version   int       `json:"version"`
}

func main() {
	// Create publisher and consumer with ULID message IDs
	publisher, err := rabbitmq.NewPublisher("amqp://guest:guest@localhost:5672/")
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer publisher.Close()

	consumer, err := rabbitmq.NewConsumer("amqp://guest:guest@localhost:5672/")
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer consumer.Close()

	// Setup topology
	err = publisher.DeclareExchange("order-events", "topic", true, false, false, false, nil)
	if err != nil {
		emit.Error.StructuredFields("Failed to declare exchange",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	err = publisher.DeclareQuorumQueue("order-processing")
	if err != nil {
		emit.Error.StructuredFields("Failed to declare queue",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Example 1: Basic message with auto-generated ULID
	emit.Info.Msg("=== Example 1: Auto-generated ULID Message IDs ===")

	orderID, _ := ulid.New()
	userID, _ := ulid.New()

	orderEvent := OrderEvent{
		OrderID:   orderID,
		EventType: "order.created",
		UserID:    userID,
		Amount:    99.99,
		Currency:  "USD",
		Timestamp: time.Now(),
		Version:   1,
	}

	eventData, _ := json.Marshal(orderEvent)

	// Create message with auto-generated ULID message ID
	message := rabbitmq.NewJSONMessage(eventData).
		WithType("OrderEvent").
		WithAppID("order-service").
		WithHeader("event_type", orderEvent.EventType).
		WithHeader("order_id", orderEvent.OrderID).
		WithPriority(5)

	err = publisher.PublishMessage(context.Background(), rabbitmq.PublishMessageConfig{
		Exchange:   "order-events",
		RoutingKey: "order.created",
		Message:    message,
	})
	if err != nil {
		emit.Error.StructuredFields("Failed to publish message",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	emit.Info.StructuredFields("Published order event with auto-generated ULID",
		emit.ZString("message_id", message.MessageID),
		emit.ZString("order_id", orderEvent.OrderID),
		emit.ZString("event_type", orderEvent.EventType))

	// Example 2: Message with custom ULID message ID
	emit.Info.Msg("=== Example 2: Custom ULID Message IDs ===")

	// Generate specific ULID for message correlation
	correlationID, _ := ulid.New()
	customMessageID, _ := ulid.New()

	orderUpdateEvent := OrderEvent{
		OrderID:   orderID, // Same order as before
		EventType: "order.payment_processed",
		UserID:    userID,
		Amount:    99.99,
		Currency:  "USD",
		Timestamp: time.Now(),
		Version:   2,
	}

	updateData, _ := json.Marshal(orderUpdateEvent)

	// Create message with custom ULID message ID
	updateMessage := rabbitmq.NewMessageWithID(updateData, customMessageID).
		WithCorrelationID(correlationID).
		WithType("OrderEvent").
		WithAppID("payment-service").
		WithHeader("event_type", orderUpdateEvent.EventType).
		WithHeader("order_id", orderUpdateEvent.OrderID).
		WithHeader("previous_version", orderUpdateEvent.Version-1).
		WithPriority(8)

	err = publisher.PublishMessage(context.Background(), rabbitmq.PublishMessageConfig{
		Exchange:   "order-events",
		RoutingKey: "order.payment.processed",
		Message:    updateMessage,
	})
	if err != nil {
		emit.Error.StructuredFields("Failed to publish update message",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	emit.Info.StructuredFields("Published order update with custom ULID",
		emit.ZString("message_id", updateMessage.MessageID),
		emit.ZString("correlation_id", updateMessage.CorrelationID),
		emit.ZString("order_id", orderUpdateEvent.OrderID),
		emit.ZString("event_type", orderUpdateEvent.EventType))

	// Example 3: Message chain with correlated ULIDs
	emit.Info.Msg("=== Example 3: Message Chain with Correlation ===")

	// Create a chain of related messages
	chainEvents := []struct {
		eventType string
		version   int
		priority  uint8
	}{
		{"order.validated", 3, 6},
		{"order.inventory_reserved", 4, 7},
		{"order.shipped", 5, 9},
		{"order.delivered", 6, 10},
	}

	for _, event := range chainEvents {
		chainEvent := OrderEvent{
			OrderID:   orderID, // Same order throughout the chain
			EventType: event.eventType,
			UserID:    userID,
			Amount:    99.99,
			Currency:  "USD",
			Timestamp: time.Now(),
			Version:   event.version,
		}

		chainData, _ := json.Marshal(chainEvent)

		// All messages in the chain share the same correlation ID
		chainMessage := rabbitmq.NewJSONMessage(chainData).
			WithCorrelationID(correlationID). // Same correlation ID
			WithType("OrderEvent").
			WithAppID("fulfillment-service").
			WithHeader("event_type", chainEvent.EventType).
			WithHeader("order_id", chainEvent.OrderID).
			WithHeader("chain_position", event.version).
			WithPriority(event.priority)

		err = publisher.PublishMessage(context.Background(), rabbitmq.PublishMessageConfig{
			Exchange:   "order-events",
			RoutingKey: "order.fulfillment.*",
			Message:    chainMessage,
		})
		if err != nil {
			emit.Error.StructuredFields("Failed to publish chain message",
				emit.ZString("error", err.Error()),
				emit.ZString("event_type", event.eventType))
			continue
		}

		emit.Info.StructuredFields("Published chain event",
			emit.ZString("message_id", chainMessage.MessageID),
			emit.ZString("correlation_id", chainMessage.CorrelationID),
			emit.ZString("event_type", chainEvent.EventType),
			emit.ZInt("version", chainEvent.Version),
			emit.ZInt("priority", int(event.priority)))

		// Small delay to show temporal ordering
		time.Sleep(10 * time.Millisecond)
	}

	// Example 4: Demonstrate ULID properties
	emit.Info.Msg("=== Example 4: ULID Properties Demonstration ===")

	// Generate multiple ULIDs to show ordering
	emit.Info.Msg("Generating 5 ULIDs to demonstrate lexicographical ordering:")

	var ulids []string
	for range 5 {
		id, _ := ulid.New()
		ulids = append(ulids, id)
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}

	for i, id := range ulids {
		emit.Info.StructuredFields("Generated ULID",
			emit.ZInt("sequence", i+1),
			emit.ZString("ulid", id),
			emit.ZInt("length", len(id)))
	}

	emit.Info.Msg("Benefits of ULID-based Message IDs:")
	emit.Info.Msg("✅ Lexicographically sortable - messages sort by creation time")
	emit.Info.Msg("✅ URL-safe - no special characters, can be used in URLs")
	emit.Info.Msg("✅ Compact - 26 characters vs UUID's 36 characters")
	emit.Info.Msg("✅ Time-based - prefix indicates when message was created")
	emit.Info.Msg("✅ Globally unique - no coordination needed across services")
	emit.Info.Msg("✅ Database-friendly - sequential inserts, better B-tree performance")
	emit.Info.Msg("✅ High performance - ~6x faster generation than UUID v4")

	emit.Info.Msg("ULID Message ID implementation complete!")
}
