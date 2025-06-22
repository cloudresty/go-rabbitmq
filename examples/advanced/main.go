package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

// Event represents a system event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

func main() {
	// Create connection
	conn, err := rabbitmq.NewConnection(rabbitmq.DefaultConnectionConfig("amqp://guest:guest@localhost:5672/"))
	if err != nil {
		emit.Error.StructuredFields("Failed to create connection",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = conn.Close() // Ignore error during cleanup
	}()

	// Setup topology
	err = rabbitmq.SetupTopology(conn,
		// Exchanges
		[]rabbitmq.ExchangeConfig{
			{
				Name:    "events",
				Type:    rabbitmq.ExchangeTypeTopic,
				Durable: true,
			},
		},
		// Queues
		[]rabbitmq.QueueConfig{
			{
				Name:    "user-events",
				Durable: true,
			},
			{
				Name:    "order-events",
				Durable: true,
			},
			{
				Name:    "audit-events",
				Durable: true,
			},
		},
		// Bindings
		[]rabbitmq.BindingConfig{
			{
				QueueName:    "user-events",
				ExchangeName: "events",
				RoutingKey:   "user.*",
			},
			{
				QueueName:    "order-events",
				ExchangeName: "events",
				RoutingKey:   "order.*",
			},
			{
				QueueName:    "audit-events",
				ExchangeName: "events",
				RoutingKey:   "*.*",
			},
		},
	)
	if err != nil {
		emit.Error.StructuredFields("Failed to setup topology",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Create publisher with custom config
	publisherConfig := rabbitmq.PublisherConfig{
		ConnectionConfig: rabbitmq.DefaultConnectionConfig("amqp://guest:guest@localhost:5672/"),
		DefaultExchange:  "events",
		Persistent:       true,
		Mandatory:        false,
	}
	publisherConfig.ConnectionName = "event-publisher"

	publisher, err := rabbitmq.NewPublisherWithConfig(publisherConfig)
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = publisher.Close() // Ignore error during cleanup
	}()

	// Publish various events
	events := []Event{
		{
			ID:     "evt-001",
			Type:   "user.created",
			Source: "user-service",
			Data: map[string]any{
				"user_id": "user-123",
				"email":   "user@example.com",
			},
			Timestamp: time.Now(),
		},
		{
			ID:     "evt-002",
			Type:   "order.placed",
			Source: "order-service",
			Data: map[string]any{
				"order_id": "order-456",
				"amount":   99.99,
			},
			Timestamp: time.Now(),
		},
		{
			ID:     "evt-003",
			Type:   "user.updated",
			Source: "user-service",
			Data: map[string]any{
				"user_id": "user-123",
				"field":   "profile",
			},
			Timestamp: time.Now(),
		},
	}

	for _, event := range events {
		eventData, err := json.Marshal(event)
		if err != nil {
			emit.Error.StructuredFields("Failed to marshal event",
				emit.ZString("event_id", event.ID),
				emit.ZString("error", err.Error()))
			continue
		}

		err = publisher.PublishWithConfirmation(context.Background(), rabbitmq.PublishConfig{
			Exchange:    "events",
			RoutingKey:  event.Type,
			Message:     eventData,
			ContentType: "application/json",
			Headers: map[string]interface{}{
				"event_id":   event.ID,
				"event_type": event.Type,
				"source":     event.Source,
			},
		})

		if err != nil {
			emit.Error.StructuredFields("Failed to publish event",
				emit.ZString("event_id", event.ID),
				emit.ZString("error", err.Error()))
		} else {
			emit.Info.StructuredFields("Published event",
				emit.ZString("event_id", event.ID),
				emit.ZString("event_type", event.Type),
				emit.ZString("source", event.Source))
		}
	}

	emit.Info.Msg("Advanced example completed!")
}
