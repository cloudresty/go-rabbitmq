package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudresty/go-rabbitmq"
)

func main() {
	fmt.Println("RabbitMQ Topology Validation and Auto-Recreation Demo")
	fmt.Println("===================================================")

	// This example demonstrates the comprehensive topology validation and auto-recreation
	// features that are enabled by default in go-rabbitmq for production reliability.

	demonstrateDefaultBehavior()
	demonstrateEnvironmentConfiguration()
	demonstrateOptOutOptions()
	demonstrateCustomConfiguration()
}

// demonstrateDefaultBehavior shows the zero-configuration default behavior
func demonstrateDefaultBehavior() {
	fmt.Println("\n1. Default Behavior (Zero Configuration)")
	fmt.Println("---------------------------------------")

	// Create client with defaults - topology validation is automatically enabled
	client, err := rabbitmq.NewClient(
		rabbitmq.WithHosts("localhost:5672"),
		rabbitmq.WithCredentials("guest", "guest"),
		rabbitmq.WithConnectionName("topology-demo-default"),
	)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	fmt.Println("Client created with default topology validation:")
	fmt.Println("- Validation: Enabled")
	fmt.Println("- Auto-recreation: Enabled")
	fmt.Println("- Background monitoring: Enabled (30s interval)")

	// Declare topology - automatically registered for validation
	declareTopology(client, "default")

	fmt.Println("Topology declared and automatically registered for monitoring.")
	fmt.Println("If deleted externally, it will be auto-recreated on next operation or background check.")
}

// demonstrateEnvironmentConfiguration shows how to use environment variables
func demonstrateEnvironmentConfiguration() {
	fmt.Println("\n2. Environment Variable Configuration")
	fmt.Println("-----------------------------------")

	fmt.Println("Available environment variables:")
	fmt.Println("RABBITMQ_TOPOLOGY_VALIDATION=true|false (default: true)")
	fmt.Println("RABBITMQ_TOPOLOGY_AUTO_RECREATION=true|false (default: true)")
	fmt.Println("RABBITMQ_TOPOLOGY_BACKGROUND_VALIDATION=true|false (default: true)")
	fmt.Println("RABBITMQ_TOPOLOGY_VALIDATION_INTERVAL=duration (default: 30s)")

	// Show current environment settings
	fmt.Printf("\nCurrent environment settings:\n")
	fmt.Printf("RABBITMQ_TOPOLOGY_VALIDATION: %s\n", getEnvWithDefault("RABBITMQ_TOPOLOGY_VALIDATION", "true"))
	fmt.Printf("RABBITMQ_TOPOLOGY_AUTO_RECREATION: %s\n", getEnvWithDefault("RABBITMQ_TOPOLOGY_AUTO_RECREATION", "true"))
	fmt.Printf("RABBITMQ_TOPOLOGY_BACKGROUND_VALIDATION: %s\n", getEnvWithDefault("RABBITMQ_TOPOLOGY_BACKGROUND_VALIDATION", "true"))
	fmt.Printf("RABBITMQ_TOPOLOGY_VALIDATION_INTERVAL: %s\n", getEnvWithDefault("RABBITMQ_TOPOLOGY_VALIDATION_INTERVAL", "30s"))

	// Create client using environment configuration
	client, err := rabbitmq.NewClient(
		rabbitmq.FromEnv(), // Loads all configuration from environment variables
	)
	if err != nil {
		log.Printf("Failed to create client from env: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	fmt.Println("Client created successfully from environment configuration.")
}

// demonstrateOptOutOptions shows how advanced users can disable features
func demonstrateOptOutOptions() {
	fmt.Println("\n3. Opt-Out Options (Advanced Users)")
	fmt.Println("----------------------------------")

	examples := []struct {
		name        string
		description string
		options     []rabbitmq.Option
	}{
		{
			name:        "Disable background validation only",
			description: "Keeps validation and auto-recreation, disables periodic monitoring",
			options: []rabbitmq.Option{
				rabbitmq.WithHosts("localhost:5672"),
				rabbitmq.WithCredentials("guest", "guest"),
				rabbitmq.WithConnectionName("no-background"),
				rabbitmq.WithoutTopologyBackgroundValidation(),
			},
		},
		{
			name:        "Disable auto-recreation",
			description: "Keeps validation, disables automatic recreation of missing topology",
			options: []rabbitmq.Option{
				rabbitmq.WithHosts("localhost:5672"),
				rabbitmq.WithCredentials("guest", "guest"),
				rabbitmq.WithConnectionName("no-recreation"),
				rabbitmq.WithoutTopologyAutoRecreation(),
			},
		},
		{
			name:        "Disable all topology features",
			description: "Completely disables topology validation and auto-recreation",
			options: []rabbitmq.Option{
				rabbitmq.WithHosts("localhost:5672"),
				rabbitmq.WithCredentials("guest", "guest"),
				rabbitmq.WithConnectionName("no-topology"),
				rabbitmq.WithoutTopologyValidation(),
			},
		},
	}

	for _, example := range examples {
		fmt.Printf("\n%s:\n", example.name)
		fmt.Printf("  %s\n", example.description)

		client, err := rabbitmq.NewClient(example.options...)
		if err != nil {
			log.Printf("  Failed to create client: %v", err)
			continue
		}

		validator := client.TopologyValidator()
		if validator != nil && validator.IsEnabled() {
			fmt.Printf("  Status: Validation enabled, Auto-recreation: %v\n", validator.IsAutoRecreateEnabled())
		} else {
			fmt.Printf("  Status: Topology validation disabled\n")
		}

		_ = client.Close()
	}
}

// demonstrateCustomConfiguration shows how to customize validation intervals
func demonstrateCustomConfiguration() {
	fmt.Println("\n4. Custom Configuration")
	fmt.Println("----------------------")

	// Custom validation interval
	client, err := rabbitmq.NewClient(
		rabbitmq.WithHosts("localhost:5672"),
		rabbitmq.WithCredentials("guest", "guest"),
		rabbitmq.WithConnectionName("custom-interval"),
		rabbitmq.WithTopologyValidationInterval(10*time.Second),
	)
	if err != nil {
		log.Printf("Failed to create client with custom interval: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	fmt.Println("Client created with custom 10-second background validation interval.")
	fmt.Println("This provides more frequent validation for critical systems.")

	fmt.Println("\nProduction Recommendations:")
	fmt.Println("- Keep defaults for most applications (30s background validation)")
	fmt.Println("- Use shorter intervals (5-15s) for mission-critical systems")
	fmt.Println("- Use longer intervals (60s+) for development environments")
	fmt.Println("- Disable background validation only if you have external monitoring")
}

// declareTopology declares sample topology for demonstration
func declareTopology(client *rabbitmq.Client, suffix string) {
	admin := client.Admin()
	ctx := context.Background()

	exchangeName := fmt.Sprintf("demo-exchange-%s", suffix)
	queueName := fmt.Sprintf("demo-queue-%s", suffix)

	err := admin.DeclareExchange(ctx, exchangeName, rabbitmq.ExchangeTypeTopic,
		rabbitmq.WithExchangeDurable(),
	)
	if err != nil {
		log.Printf("Failed to declare exchange: %v", err)
		return
	}

	_, err = admin.DeclareQueue(ctx, queueName, rabbitmq.WithDurable())
	if err != nil {
		log.Printf("Failed to declare queue: %v", err)
		return
	}

	err = admin.BindQueue(ctx, queueName, exchangeName, "demo.key")
	if err != nil {
		log.Printf("Failed to bind queue: %v", err)
		return
	}
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
