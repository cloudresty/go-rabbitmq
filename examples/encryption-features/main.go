package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"github.com/cloudresty/go-rabbitmq"
	"github.com/cloudresty/go-rabbitmq/encryption"
)

func main() {
	// Generate a secure 256-bit encryption key
	encryptionKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		log.Fatalf("Failed to generate encryption key: %v", err)
	}

	// Create RabbitMQ client
	client, err := rabbitmq.NewClient(
		rabbitmq.WithHosts("localhost:5672"),
		rabbitmq.WithCredentials("guest", "guest"),
		rabbitmq.WithConnectionName("encryption-example"),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Example 1: Create AES-GCM encryptor using contract-implementation pattern
	encryptor, err := encryption.NewAESGCM(encryptionKey)
	if err != nil {
		log.Fatalf("Failed to create encryptor: %v", err)
	}

	// Example 2: Create publisher with encryption
	publisher, err := client.NewPublisher(
		rabbitmq.WithDefaultExchange("secure-exchange"),
		rabbitmq.WithEncryption(encryptor),
		rabbitmq.WithConfirmation(5*time.Second),
		rabbitmq.WithPersistent(),
	)
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	// Example 3: Create consumer with automatic decryption
	consumer, err := client.NewConsumer(
		rabbitmq.WithPrefetchCount(10),
		rabbitmq.WithConsumerEncryption(encryptor),
	)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	// Example 4: Publish sensitive messages (automatically encrypted)
	ctx := context.Background()

	sensitiveMessages := []string{
		`{"customer_id": "12345", "ssn": "123-45-6789", "account": "9876543210"}`,
		`{"credit_card": "4111-1111-1111-1111", "cvv": "123", "expiry": "12/25"}`,
		`{"medical_record": "patient has diabetes", "doctor": "Dr. Smith"}`,
	}

	for i, data := range sensitiveMessages {
		message := rabbitmq.NewMessage([]byte(data))
		message.MessageID = fmt.Sprintf("secure-msg-%d", i+1)
		message.Headers = map[string]interface{}{
			"content-type": "application/json",
			"importance":   "high",
		}

		err = publisher.Publish(ctx, "secure-exchange", "sensitive.data", message)
		if err != nil {
			log.Printf("Failed to publish encrypted message %d: %v", i+1, err)
		} else {
			fmt.Printf("Published encrypted message %d (algorithm: %s)\n", i+1, encryptor.Algorithm())
		}
	}

	// Example 5: Consume and automatically decrypt messages
	fmt.Println("\nStarting encrypted message consumption...")

	consumeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messageCount := 0
	err = consumer.Consume(consumeCtx, "secure.queue", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
		messageCount++

		// Message is automatically decrypted before reaching this handler
		fmt.Printf("Received decrypted message %d:\n", messageCount)
		fmt.Printf("  Message ID: %s\n", delivery.MessageId)
		fmt.Printf("  Content: %s\n", string(delivery.Body)[:50]+"...") // Truncate for security

		// Check encryption header
		if encAlg, ok := delivery.Headers["x-encryption"]; ok {
			fmt.Printf("  Encryption: %v\n", encAlg)
		}

		// Check custom headers
		if importance, ok := delivery.Headers["importance"]; ok {
			fmt.Printf("  Importance: %v\n", importance)
		}

		fmt.Printf("  Timestamp: %v\n", delivery.Timestamp())
		fmt.Println()

		return nil
	})

	if err != nil {
		log.Printf("Consumer error: %v", err)
	}

	// Example 6: Demonstrate encryption without publisher/consumer
	fmt.Println("=== Direct Encryption Demo ===")

	originalData := []byte("This is sensitive information that needs to be encrypted!")
	fmt.Printf("Original data: %s\n", string(originalData))

	// Encrypt manually
	encryptedData, err := encryptor.Encrypt(originalData)
	if err != nil {
		log.Printf("Encryption failed: %v", err)
	} else {
		fmt.Printf("Encrypted data length: %d bytes (algorithm: %s)\n",
			len(encryptedData), encryptor.Algorithm())
	}

	// Decrypt manually
	decryptedData, err := encryptor.Decrypt(encryptedData)
	if err != nil {
		log.Printf("Decryption failed: %v", err)
	} else {
		fmt.Printf("Decrypted data: %s\n", string(decryptedData))
	}

	// Example 7: Demonstrate no-encryption mode for testing
	fmt.Println("\n=== No-Encryption Mode Demo ===")

	noEncryptor := encryption.NewNoEncryption()
	fmt.Printf("Algorithm: %s\n", noEncryptor.Algorithm())

	testData := []byte("test message")
	encrypted, _ := noEncryptor.Encrypt(testData)
	decrypted, _ := noEncryptor.Decrypt(encrypted)

	fmt.Printf("Original:  %s\n", string(testData))
	fmt.Printf("Encrypted: %s (no change)\n", string(encrypted))
	fmt.Printf("Decrypted: %s (no change)\n", string(decrypted))

	// Demo complete
	fmt.Println("\n=== Encryption Contract-Implementation Pattern Demo Complete ===")
	fmt.Println("✓ Used encryption.NewAESGCM() to create pluggable implementation")
	fmt.Println("✓ Published messages with automatic encryption")
	fmt.Println("✓ Consumed messages with automatic decryption")
	fmt.Println("✓ Demonstrated direct encryption/decryption")
	fmt.Println("✓ Clean separation between core rabbitmq and encryption implementation")
}
