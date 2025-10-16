package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	rabbitmq "github.com/cloudresty/go-rabbitmq"
)

func main() {
	// Create RabbitMQ client
	client, err := rabbitmq.NewClient(
		rabbitmq.WithHosts("localhost:5672"),
		rabbitmq.WithCredentials("guest", "guest"),
		rabbitmq.WithConnectionName("delivery-assurance-example"),
	)
	if err != nil {
		log.Fatal("Failed to create client:", err)
	}
	defer client.Close()

	// Declare exchange and queue for testing
	admin := client.Admin()
	ctx := context.Background()

	// Create exchange
	err = admin.DeclareExchange(ctx, "events", rabbitmq.ExchangeTypeTopic)
	if err != nil {
		log.Fatal("Failed to declare exchange:", err)
	}

	// Create queue
	queue, err := admin.DeclareQueue(ctx, "delivery-test-queue")
	if err != nil {
		log.Fatal("Failed to declare queue:", err)
	}

	// Bind queue to exchange
	err = admin.BindQueue(ctx, queue.Name, "events", "user.*")
	if err != nil {
		log.Fatal("Failed to bind queue:", err)
	}

	log.Println("✓ Topology created successfully")

	// Run examples
	fmt.Println("\n=== Example 1: Simple Delivery Assurance ===")
	simpleExample(client)

	fmt.Println("\n=== Example 2: Advanced Callbacks ===")
	advancedCallbackExample(client)

	fmt.Println("\n=== Example 3: Mandatory Publishing (Routing Failures) ===")
	mandatoryExample(client)

	fmt.Println("\n=== Example 4: Delivery Statistics ===")
	statisticsExample(client)

	fmt.Println("\n=== Example 5: Concurrent Publishing ===")
	concurrentExample(client)

	// Wait for user interrupt
	fmt.Println("\n=== Press Ctrl+C to exit ===")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}

// Example 1: Simple delivery assurance with default callback
func simpleExample(client *rabbitmq.Client) {
	// Create publisher with delivery assurance
	publisher, err := client.NewPublisher(
		rabbitmq.WithDeliveryAssurance(),
		rabbitmq.WithDefaultDeliveryCallback(func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
			switch outcome {
			case rabbitmq.DeliverySuccess:
				log.Printf("✓ Message %s delivered successfully", messageID)
			case rabbitmq.DeliveryFailed:
				log.Printf("✗ Message %s failed: %s", messageID, errorMessage)
			case rabbitmq.DeliveryNacked:
				log.Printf("⚠ Message %s nacked: %s", messageID, errorMessage)
			case rabbitmq.DeliveryTimeout:
				log.Printf("⏱ Message %s timed out", messageID)
			}
		}),
	)
	if err != nil {
		log.Fatal("Failed to create publisher:", err)
	}
	defer publisher.Close()

	// Publish a message
	message := rabbitmq.NewMessage([]byte(`{"event": "user.created", "user_id": "123"}`)).
		WithContentType("application/json")

	err = publisher.PublishWithDeliveryAssurance(
		context.Background(),
		"events",
		"user.created",
		message,
		rabbitmq.DeliveryOptions{
			MessageID: "simple-msg-1",
			Mandatory: true,
		},
	)
	if err != nil {
		log.Printf("Failed to publish: %v", err)
		return
	}

	log.Println("Message published, waiting for confirmation...")
	time.Sleep(1 * time.Second) // Wait for callback
}

// Example 2: Advanced callbacks with custom handling
func advancedCallbackExample(client *rabbitmq.Client) {
	// Track delivery outcomes
	var mu sync.Mutex
	outcomes := make(map[string]rabbitmq.DeliveryOutcome)

	publisher, err := client.NewPublisher(
		rabbitmq.WithDeliveryAssurance(),
		rabbitmq.WithDeliveryTimeout(5*time.Second),
	)
	if err != nil {
		log.Fatal("Failed to create publisher:", err)
	}
	defer publisher.Close()

	// Publish multiple messages with custom callbacks
	for i := 1; i <= 3; i++ {
		messageID := fmt.Sprintf("advanced-msg-%d", i)
		message := rabbitmq.NewMessage([]byte(fmt.Sprintf(`{"id": %d}`, i))).
			WithContentType("application/json")

		err = publisher.PublishWithDeliveryAssurance(
			context.Background(),
			"events",
			"user.updated",
			message,
			rabbitmq.DeliveryOptions{
				MessageID: messageID,
				Mandatory: true,
				Callback: func(msgID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
					mu.Lock()
					outcomes[msgID] = outcome
					mu.Unlock()

					log.Printf("Message %s: %s", msgID, outcome)
					if errorMessage != "" {
						log.Printf("  Error: %s", errorMessage)
					}

					// Custom business logic based on outcome
					if outcome == rabbitmq.DeliveryFailed {
						// Could trigger retry logic, alerting, etc.
						log.Printf("  → Triggering failure handler for %s", msgID)
					}
				},
			},
		)
		if err != nil {
			log.Printf("Failed to publish %s: %v", messageID, err)
		}
	}

	time.Sleep(2 * time.Second) // Wait for callbacks

	mu.Lock()
	log.Printf("Final outcomes: %+v", outcomes)
	mu.Unlock()
}

// Example 3: Mandatory publishing to detect routing failures
func mandatoryExample(client *rabbitmq.Client) {
	publisher, err := client.NewPublisher(
		rabbitmq.WithDeliveryAssurance(),
		rabbitmq.WithMandatoryByDefault(true), // All messages mandatory by default
		rabbitmq.WithDefaultDeliveryCallback(func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
			if outcome == rabbitmq.DeliveryFailed {
				log.Printf("⚠ Routing failure detected for %s: %s", messageID, errorMessage)
			} else {
				log.Printf("✓ Message %s: %s", messageID, outcome)
			}
		}),
	)
	if err != nil {
		log.Fatal("Failed to create publisher:", err)
	}
	defer publisher.Close()

	// Publish to a non-existent routing key (will be returned)
	message := rabbitmq.NewMessage([]byte(`{"test": "routing failure"}`))

	err = publisher.PublishWithDeliveryAssurance(
		context.Background(),
		"events",
		"nonexistent.routing.key", // This will fail routing
		message,
		rabbitmq.DeliveryOptions{
			MessageID: "routing-test-1",
		},
	)
	if err != nil {
		log.Printf("Failed to publish: %v", err)
		return
	}

	log.Println("Published to non-existent routing key, waiting for return...")
	time.Sleep(2 * time.Second)
}

// Example 4: Monitoring delivery statistics
func statisticsExample(client *rabbitmq.Client) {
	publisher, err := client.NewPublisher(
		rabbitmq.WithDeliveryAssurance(),
		rabbitmq.WithDefaultDeliveryCallback(func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
			// Silent callback for this example
		}),
	)
	if err != nil {
		log.Fatal("Failed to create publisher:", err)
	}
	defer publisher.Close()

	// Publish several messages
	for i := 1; i <= 5; i++ {
		message := rabbitmq.NewMessage([]byte(fmt.Sprintf(`{"count": %d}`, i)))
		err = publisher.PublishWithDeliveryAssurance(
			context.Background(),
			"events",
			"user.stats",
			message,
			rabbitmq.DeliveryOptions{
				MessageID: fmt.Sprintf("stats-msg-%d", i),
				Mandatory: true,
			},
		)
		if err != nil {
			log.Printf("Failed to publish: %v", err)
		}
	}

	// Wait for confirmations
	time.Sleep(1 * time.Second)

	// Get and display statistics
	stats := publisher.GetDeliveryStats()
	log.Printf("Delivery Statistics:")
	log.Printf("  Total Published:  %d", stats.TotalPublished)
	log.Printf("  Total Confirmed:  %d", stats.TotalConfirmed)
	log.Printf("  Total Returned:   %d", stats.TotalReturned)
	log.Printf("  Total Nacked:     %d", stats.TotalNacked)
	log.Printf("  Total Timed Out:  %d", stats.TotalTimedOut)
	log.Printf("  Pending Messages: %d", stats.PendingMessages)
	if !stats.LastConfirmation.IsZero() {
		log.Printf("  Last Confirmation: %s", stats.LastConfirmation.Format(time.RFC3339))
	}
}

// Example 5: Concurrent publishing with delivery assurance
func concurrentExample(client *rabbitmq.Client) {
	publisher, err := client.NewPublisher(
		rabbitmq.WithDeliveryAssurance(),
		rabbitmq.WithDefaultDeliveryCallback(func(messageID string, outcome rabbitmq.DeliveryOutcome, errorMessage string) {
			log.Printf("Concurrent: %s → %s", messageID, outcome)
		}),
	)
	if err != nil {
		log.Fatal("Failed to create publisher:", err)
	}
	defer publisher.Close()

	// Publish messages concurrently
	var wg sync.WaitGroup
	numMessages := 10

	for i := 1; i <= numMessages; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			message := rabbitmq.NewMessage([]byte(fmt.Sprintf(`{"concurrent_id": %d}`, id)))
			err := publisher.PublishWithDeliveryAssurance(
				context.Background(),
				"events",
				"user.concurrent",
				message,
				rabbitmq.DeliveryOptions{
					MessageID: fmt.Sprintf("concurrent-msg-%d", id),
					Mandatory: true,
				},
			)
			if err != nil {
				log.Printf("Failed to publish concurrent message %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	log.Printf("Published %d messages concurrently", numMessages)

	time.Sleep(2 * time.Second) // Wait for all callbacks

	stats := publisher.GetDeliveryStats()
	log.Printf("Concurrent stats: %d published, %d confirmed", stats.TotalPublished, stats.TotalConfirmed)
}
