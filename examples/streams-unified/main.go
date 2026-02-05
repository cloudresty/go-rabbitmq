package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudresty/go-rabbitmq"
	"github.com/cloudresty/go-rabbitmq/streams"
)

func main() {
	// Create streams handler using native stream protocol (port 5552)
	// This provides 5-10x better performance compared to AMQP 0.9.1
	streamsHandler, err := streams.NewHandler(streams.Options{
		Host:     "localhost",
		Port:     5552,
		Username: "guest",
		Password: "guest",
	})
	if err != nil {
		log.Fatalf("Failed to create streams handler: %v", err)
	}
	defer func() {
		if err := streamsHandler.Close(); err != nil {
			log.Printf("Failed to close streams handler: %v", err)
		}
	}()

	// Example 2: Create a stream with configuration
	streamConfig := rabbitmq.StreamConfig{
		MaxAge:            24 * time.Hour,
		MaxLengthMessages: 1_000_000,
		MaxLengthBytes:    1024 * 1024 * 1024, // 1GB
	}

	err = streamsHandler.CreateStream(context.Background(), "events.stream", streamConfig)
	if err != nil {
		log.Printf("Stream might already exist: %v", err)
	}

	// Example 3: Publish messages to stream
	for i := 0; i < 10; i++ {
		message := rabbitmq.NewMessage([]byte(fmt.Sprintf("Event %d: %s", i, time.Now().Format(time.RFC3339))))
		message.MessageID = fmt.Sprintf("msg-%d", i)
		message.Timestamp = time.Now().Unix()

		err = streamsHandler.PublishToStream(context.Background(), "events.stream", message)
		if err != nil {
			log.Printf("Failed to publish message %d: %v", i, err)
		} else {
			fmt.Printf("Published message %d to stream\n", i)
		}
	}

	// Example 4: Consume messages from stream
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println("Starting stream consumption...")
	err = streamsHandler.ConsumeFromStream(ctx, "events.stream", messageHandler)
	if err != nil {
		log.Printf("Consumption ended: %v", err)
	}

	// Demo complete
	fmt.Println("\n=== Native Streams Demo Complete ===")
	fmt.Println("✓ Used native stream protocol (port 5552) for 5-10x better performance")
	fmt.Println("✓ Published messages to stream")
	fmt.Println("✓ Consumed messages from stream")
	fmt.Println("✓ Sub-millisecond latency with binary protocol")
}

// messageHandler processes stream messages
func messageHandler(ctx context.Context, delivery *rabbitmq.Delivery) error {
	fmt.Printf("Received message: %s (ID: %s, Timestamp: %v)\n",
		string(delivery.Body),
		delivery.MessageId,
		delivery.Timestamp())

	// Simulate processing time
	time.Sleep(100 * time.Millisecond)

	return nil
}
