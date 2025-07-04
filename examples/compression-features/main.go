package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudresty/go-rabbitmq"
	"github.com/cloudresty/go-rabbitmq/compression"
)

func main() {
	// Example 1: Basic compression (no-op from main package)
	fmt.Println("=== Basic Compression Example ===")

	// Create a client with basic no-op compression
	client, err := rabbitmq.NewClient(
		rabbitmq.WithHosts("localhost:5672"),
		rabbitmq.WithCredentials("guest", "guest"),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create a publisher with basic compression (no-op)
	basicPublisher, err := client.NewPublisher(
		rabbitmq.WithDefaultExchange("test-exchange"),
		rabbitmq.WithCompression(rabbitmq.NewNopCompressor()), // Basic from main package
		rabbitmq.WithCompressionThreshold(1024),
	)
	if err != nil {
		log.Fatalf("Failed to create basic publisher: %v", err)
	}
	defer basicPublisher.Close()

	// Publish a message (will not be compressed due to no-op compressor)
	message := &rabbitmq.Message{
		Body: []byte("This message will not be compressed"),
	}

	if err := basicPublisher.Publish(context.Background(), "test-exchange", "test-key", message); err != nil {
		log.Printf("Failed to publish message: %v", err)
	} else {
		fmt.Println("Published message with basic (no-op) compression")
	}

	// Example 2: Advanced compression from sub-package
	fmt.Println("\n=== Advanced Compression Example ===")

	// Create advanced gzip compressor from compression sub-package
	gzipCompressor := compression.NewGzip(512, compression.DefaultLevel) // 512 byte threshold, default compression level

	// Create a publisher with advanced compression
	advancedPublisher, err := client.NewPublisher(
		rabbitmq.WithDefaultExchange("test-exchange"),
		rabbitmq.WithCompression(gzipCompressor), // Advanced from sub-package
	)
	if err != nil {
		log.Fatalf("Failed to create advanced publisher: %v", err)
	}
	defer advancedPublisher.Close()

	// Publish a larger message that will be compressed
	largeMessage := &rabbitmq.Message{
		Body: []byte(generateLargeContent(1000)), // > 512 bytes, will be compressed
	}

	if err := advancedPublisher.Publish(context.Background(), "test-exchange", "test-key-compressed", largeMessage); err != nil {
		log.Printf("Failed to publish compressed message: %v", err)
	} else {
		fmt.Printf("Published message with gzip compression (algorithm: %s, threshold: %d)\n",
			gzipCompressor.Algorithm(), gzipCompressor.Threshold())
	}
	// Example 3: Show compression algorithms comparison
	fmt.Println("\n=== Compression Algorithm Comparison ===")

	// Create zlib compressor
	zlibCompressor := compression.NewZlib(256, compression.BestSpeed) // 256 byte threshold, best speed
	fmt.Printf("Created zlib compressor (algorithm: %s, threshold: %d bytes)\n",
		zlibCompressor.Algorithm(), zlibCompressor.Threshold())

	// Example 4: Compare compression algorithms
	fmt.Println("\n=== Compression Performance Test ===")

	testData := []byte(generateLargeContent(2000))
	fmt.Printf("Original data size: %d bytes\n", len(testData))

	// Test different compressors
	compressors := []rabbitmq.MessageCompressor{
		rabbitmq.NewNopCompressor(),
		compression.NewGzip(100, compression.DefaultLevel),
		compression.NewZlib(100, compression.DefaultLevel),
	}

	for _, comp := range compressors {
		compressed, err := comp.Compress(testData)
		if err != nil {
			log.Printf("Compression failed for %s: %v", comp.Algorithm(), err)
			continue
		}

		ratio := float64(len(compressed)) / float64(len(testData)) * 100
		fmt.Printf("Algorithm: %-8s | Compressed size: %4d bytes | Ratio: %5.1f%%\n",
			comp.Algorithm(), len(compressed), ratio)
	}
}

func generateLargeContent(size int) string {
	content := "This is a test message that will be repeated to create a large payload for compression testing. "
	result := ""
	for len(result) < size {
		result += content
	}
	return result[:size]
}
