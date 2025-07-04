package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudresty/go-rabbitmq"
	"github.com/cloudresty/go-rabbitmq/performance"
)

func main() {
	// Create an advanced performance monitor
	monitor := performance.NewMonitor()

	// Create a client with advanced performance monitoring
	client, err := rabbitmq.NewClient(
		rabbitmq.WithHosts("localhost:5672"),
		rabbitmq.WithCredentials("guest", "guest"),
		rabbitmq.WithConnectionName("performance-example"),
		rabbitmq.WithPerformanceMonitoring(monitor), // Use the advanced monitor
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		_ = client.Close() // Ignore close error in defer
	}()

	// Create a publisher
	publisher, err := client.NewPublisher(
		rabbitmq.WithDefaultExchange("test-exchange"),
	)
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer func() {
		_ = publisher.Close() // Ignore close error in defer
	}()

	// Publish some messages to generate metrics
	fmt.Println("Publishing messages to generate performance metrics...")
	for i := range 100 {
		message := &rabbitmq.Message{
			Body: fmt.Appendf(nil, "Message %d", i),
		}

		if err := publisher.Publish(context.Background(), "test-exchange", "test-key", message); err != nil {
			log.Printf("Failed to publish message %d: %v", i, err)
		}

		// Small delay to see rate tracking in action
		time.Sleep(10 * time.Millisecond)
	}

	// Get detailed performance statistics
	stats := monitor.GetStats()

	fmt.Println("\n=== Advanced Performance Statistics ===")
	fmt.Printf("Connection Stats:\n")
	fmt.Printf("  Total Connections: %d\n", stats.ConnectionsTotal)
	fmt.Printf("  Reconnections: %d\n", stats.ReconnectionsTotal)
	fmt.Printf("  Currently Connected: %v\n", stats.IsConnected)

	fmt.Printf("\nPublish Stats:\n")
	fmt.Printf("  Total Publishes: %d\n", stats.PublishesTotal)
	fmt.Printf("  Successful Publishes: %d\n", stats.PublishSuccessTotal)
	fmt.Printf("  Failed Publishes: %d\n", stats.PublishErrorsTotal)
	fmt.Printf("  Success Rate: %.2f%%\n", stats.PublishSuccessRate*100)
	fmt.Printf("  Current Publish Rate: %.2f ops/sec\n", stats.PublishRate)

	fmt.Printf("\nPublish Latency Percentiles:\n")
	fmt.Printf("  P50: %v\n", stats.PublishLatencyP50)
	fmt.Printf("  P95: %v\n", stats.PublishLatencyP95)
	fmt.Printf("  P99: %v\n", stats.PublishLatencyP99)

	fmt.Printf("\nConsume Stats:\n")
	fmt.Printf("  Total Consumes: %d\n", stats.ConsumesTotal)
	fmt.Printf("  Successful Consumes: %d\n", stats.ConsumeSuccessTotal)
	fmt.Printf("  Failed Consumes: %d\n", stats.ConsumeErrorsTotal)
	fmt.Printf("  Success Rate: %.2f%%\n", stats.ConsumeSuccessRate*100)
	fmt.Printf("  Current Consume Rate: %.2f ops/sec\n", stats.ConsumeRate)

	fmt.Printf("\nAdditional Monitor Methods:\n")
	fmt.Printf("  Total Operations: %d\n", monitor.GetTotalOperations())
	fmt.Printf("  Overall Success Rate: %.2f%%\n", monitor.GetSuccessRate()*100)
	fmt.Printf("  Current Publish Rate: %.2f ops/sec\n", monitor.GetPublishRate())
	fmt.Printf("  Current Consume Rate: %.2f ops/sec\n", monitor.GetConsumeRate())
	fmt.Printf("  Connection Status: %v\n", monitor.IsConnected())

	// Demonstrate reset functionality
	fmt.Println("\nResetting performance counters...")
	monitor.Reset()

	statsAfterReset := monitor.GetStats()
	fmt.Printf("Stats after reset - Total Operations: %d\n", statsAfterReset.PublishesTotal+statsAfterReset.ConsumesTotal)
}
