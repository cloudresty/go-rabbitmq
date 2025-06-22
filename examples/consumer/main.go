package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

// Example message structure
type OrderMessage struct {
	OrderID   string  `json:"order_id"`
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

func main() {
	// Create a consumer
	consumer, err := rabbitmq.NewConsumer("amqp://guest:guest@localhost:5672/")
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = consumer.Close() // Ignore error during cleanup
	}()

	// Declare queue
	queue, err := consumer.DeclareQueue("order-processing", true, false, false, false, nil)
	if err != nil {
		emit.Error.StructuredFields("Failed to declare queue",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Bind queue to exchange
	err = consumer.BindQueue(queue.Name, "new-order", "orders", false, nil)
	if err != nil {
		emit.Error.StructuredFields("Failed to bind queue",
			emit.ZString("error", err.Error()),
			emit.ZString("queue_name", queue.Name))
		os.Exit(1)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		emit.Info.Msg("Received shutdown signal, stopping consumer...")
		cancel()
	}()

	// Start consuming messages
	emit.Info.StructuredFields("Starting to consume messages",
		emit.ZString("queue_name", queue.Name))

	err = consumer.Consume(ctx, rabbitmq.ConsumeConfig{
		Queue:    queue.Name,
		Consumer: "order-processor",
		Handler: func(ctx context.Context, message []byte) error {
			// Parse the message
			var order OrderMessage
			if err := json.Unmarshal(message, &order); err != nil {
				emit.Error.StructuredFields("Failed to unmarshal message",
					emit.ZString("error", err.Error()))
				return err
			}

			// Process the order
			emit.Info.StructuredFields("Processing order",
				emit.ZString("order_id", order.OrderID),
				emit.ZString("product_id", order.ProductID),
				emit.ZInt("quantity", order.Quantity),
				emit.ZFloat64("price", order.Price))

			// Simulate processing time
			// time.Sleep(100 * time.Millisecond)

			// Simulate business logic
			if err := processOrder(order); err != nil {
				emit.Error.StructuredFields("Failed to process order",
					emit.ZString("order_id", order.OrderID),
					emit.ZString("error", err.Error()))
				return err
			}

			emit.Info.StructuredFields("Successfully processed order",
				emit.ZString("order_id", order.OrderID))
			return nil
		},
	})

	if err != nil && err != context.Canceled {
		emit.Error.StructuredFields("Consumer error",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	emit.Info.Msg("Consumer stopped gracefully")
}

// processOrder simulates order processing business logic
func processOrder(order OrderMessage) error {
	// Validate order
	if order.Quantity <= 0 {
		return rabbitmq.NewConsumeError("invalid quantity", nil)
	}

	if order.Price <= 0 {
		return rabbitmq.NewConsumeError("invalid price", nil)
	}

	// Here you would typically:
	// 1. Update inventory
	// 2. Process payment
	// 3. Send confirmation
	// 4. Update order status in database

	return nil
}
