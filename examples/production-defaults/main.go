package main

import (
	"context"
	"fmt"
	"log"
	"time"

	rabbitmq "github.com/cloudresty/go-rabbitmq"
)

func main() {
	ctx := context.Background()

	// Create RabbitMQ client
	client, err := rabbitmq.NewClient(
		rabbitmq.WithCredentials("guest", "guest"),
		rabbitmq.WithHosts("localhost:5672"),
		rabbitmq.WithConnectionName("production-defaults-example"),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Get admin service
	admin := client.Admin()

	fmt.Println("=== Production-Ready Queue Defaults Demo ===")
	fmt.Println()

	// 1. Default behavior: Quorum queue with DLQ enabled
	fmt.Println("1. Creating queue with production defaults (quorum + DLQ):")
	queue1, err := admin.DeclareQueue(ctx, "production-queue")
	if err != nil {
		log.Fatalf("Failed to declare production queue: %v", err)
	}
	fmt.Printf("   ✓ Queue: %s (quorum type with DLQ enabled by default)\n", queue1.Name)
	fmt.Printf("   ✓ DLX: production-queue.dlx\n")
	fmt.Printf("   ✓ DLQ: production-queue.dlq (7-day TTL)\n")
	fmt.Println()

	// 2. Customizing quorum queue settings
	fmt.Println("2. Creating customized quorum queue:")
	queue2, err := admin.DeclareQueue(ctx, "custom-quorum",
		rabbitmq.WithQuorumGroupSize(5),          // Custom quorum size
		rabbitmq.WithDeliveryLimit(3),            // Max 3 delivery attempts
		rabbitmq.WithDLQTTL(24*time.Hour),        // 1-day DLQ TTL
		rabbitmq.WithDLQSuffixes(".dlx", ".dlq"), // Custom suffixes
	)
	if err != nil {
		log.Fatalf("Failed to declare custom quorum queue: %v", err)
	}
	fmt.Printf("   ✓ Queue: %s (quorum size: 5, delivery limit: 3)\n", queue2.Name)
	fmt.Printf("   ✓ DLQ TTL: 24 hours\n")
	fmt.Println()

	// 3. Explicit classic queue (legacy mode)
	fmt.Println("3. Creating classic queue (opt-in for legacy compatibility):")
	queue3, err := admin.DeclareQueue(ctx, "legacy-classic",
		rabbitmq.WithClassicQueue(), // Explicitly request classic queue
		rabbitmq.WithTTL(time.Hour), // Classic queue features still work
	)
	if err != nil {
		log.Fatalf("Failed to declare classic queue: %v", err)
	}
	fmt.Printf("   ✓ Queue: %s (classic type with DLQ enabled)\n", queue3.Name)
	fmt.Printf("   ✓ Message TTL: 1 hour\n")
	fmt.Println()

	// 4. Opting out of DLQ (edge case)
	fmt.Println("4. Creating queue without DLQ (edge case):")
	queue4, err := admin.DeclareQueue(ctx, "no-dlq-queue",
		rabbitmq.WithoutDLQ(), // Explicitly disable DLQ
	)
	if err != nil {
		log.Fatalf("Failed to declare queue without DLQ: %v", err)
	}
	fmt.Printf("   ✓ Queue: %s (quorum type, no DLQ)\n", queue4.Name)
	fmt.Println()

	// 5. Using dedicated quorum queue method (same as default now)
	fmt.Println("5. Using DeclareQuorumQueue method (equivalent to default):")
	queue5, err := admin.DeclareQuorumQueue(ctx, "explicit-quorum",
		rabbitmq.WithQuorumDeliveryLimit(5),
	)
	if err != nil {
		log.Fatalf("Failed to declare explicit quorum queue: %v", err)
	}
	fmt.Printf("   ✓ Queue: %s (explicit quorum declaration)\n", queue5.Name)
	fmt.Println()

	fmt.Println("=== Summary ===")
	fmt.Println("✓ Quorum queues are now the default (production-ready)")
	fmt.Println("✓ Dead Letter Queues are enabled by default")
	fmt.Println("✓ Easy opt-out with WithClassicQueue() and WithoutDLQ()")
	fmt.Println("✓ Enterprise-ready defaults with simple overrides")
	fmt.Println()
	fmt.Println("This configuration provides:")
	fmt.Println("• High availability (quorum consensus)")
	fmt.Println("• Data safety (no message loss on node failures)")
	fmt.Println("• Poison message protection (delivery limits)")
	fmt.Println("• Automatic dead letter handling")
	fmt.Println("• Production-ready defaults")
}
