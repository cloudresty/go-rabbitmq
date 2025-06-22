package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

// Example message structure
type OrderMessage struct {
	OrderID   string    `json:"order_id"`
	ProductID string    `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	// Create a publisher
	publisher, err := rabbitmq.NewPublisher("amqp://guest:guest@localhost:5672/")
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = publisher.Close() // Ignore error during cleanup
	}()

	// Declare exchange
	err = publisher.DeclareExchange("orders", "direct", true, false, false, false, nil)
	if err != nil {
		emit.Error.StructuredFields("Failed to declare exchange",
			emit.ZString("error", err.Error()),
			emit.ZString("exchange_name", "orders"))
		os.Exit(1)
	}

	// Create sample messages
	orders := []OrderMessage{
		{
			OrderID:   "order-001",
			ProductID: "product-123",
			Quantity:  2,
			Price:     29.99,
			Timestamp: time.Now(),
		},
		{
			OrderID:   "order-002",
			ProductID: "product-456",
			Quantity:  1,
			Price:     149.99,
			Timestamp: time.Now(),
		},
		{
			OrderID:   "order-003",
			ProductID: "product-789",
			Quantity:  3,
			Price:     9.99,
			Timestamp: time.Now(),
		},
	}

	// Publish messages
	for _, order := range orders {
		// Marshal to JSON
		messageBody, err := json.Marshal(order)
		if err != nil {
			emit.Error.StructuredFields("Failed to marshal order",
				emit.ZString("order_id", order.OrderID),
				emit.ZString("error", err.Error()))
			continue
		}

		// Publish with confirmation
		err = publisher.PublishWithConfirmation(context.Background(), rabbitmq.PublishConfig{
			Exchange:    "orders",
			RoutingKey:  "new-order",
			Message:     messageBody,
			ContentType: "application/json",
			Headers: map[string]any{
				"order_id": order.OrderID,
				"source":   "order-service",
			},
		})

		if err != nil {
			emit.Error.StructuredFields("Failed to publish order",
				emit.ZString("order_id", order.OrderID),
				emit.ZString("error", err.Error()))
		} else {
			emit.Info.StructuredFields("Successfully published order",
				emit.ZString("order_id", order.OrderID),
				emit.ZString("product_id", order.ProductID),
				emit.ZFloat64("price", order.Price))
		}

		// Small delay between messages
		time.Sleep(100 * time.Millisecond)
	}

	emit.Info.StructuredFields("All messages published successfully",
		emit.ZInt("total_orders", len(orders)))
}
