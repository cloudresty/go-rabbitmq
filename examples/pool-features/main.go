package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cloudresty/go-rabbitmq"
	"github.com/cloudresty/go-rabbitmq/pool"
)

func main() {
	fmt.Println("=== Connection Pool Feature Demonstration ===")

	// Example 1: Create a connection pool using the sub-package
	fmt.Println("\n1. Creating Connection Pool (Sub-Package):")

	// Create a pool with specific configuration
	connectionPool, err := pool.New(3, // 3 connections in pool
		pool.WithClientOptions(
			rabbitmq.WithHosts("localhost:5672"),
			rabbitmq.WithCredentials("guest", "guest"),
			rabbitmq.WithConnectionName("pool-example"),
		),
		pool.WithHealthCheck(5*time.Second),
		pool.WithAutoRepair(true),
	)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		_ = connectionPool.Close() // Ignore close error in defer
	}()

	fmt.Printf("✓ Connection pool created with %d connections\n", connectionPool.Size())
	fmt.Printf("✓ Healthy connections: %d\n", connectionPool.HealthyCount())

	// Example 2: Get a client from the pool
	fmt.Println("\n2. Using Pool (Contract-Implementation Pattern):")

	client, err := connectionPool.Get()
	if err != nil {
		log.Fatalf("Failed to get client from pool: %v", err)
	}

	fmt.Printf("✓ Retrieved client from pool: %s\n", client.ConnectionName())

	// Example 3: Use the pool as a pluggable component
	fmt.Println("\n3. Pool as Pluggable Component:")

	// Create a new client that can use the pool as a pluggable component
	clientWithPool, err := rabbitmq.NewClient(
		rabbitmq.WithHosts("localhost:5672"),
		rabbitmq.WithCredentials("guest", "guest"),
		rabbitmq.WithConnectionPooler(connectionPool), // Plug in the pool
	)
	if err != nil {
		log.Fatalf("Failed to create client with pool: %v", err)
	}
	defer func() {
		_ = clientWithPool.Close() // Ignore close error in defer
	}()

	fmt.Printf("✓ Client configured with connection pooler\n")

	// Example 4: Pool statistics
	fmt.Println("\n4. Pool Statistics:")

	stats := connectionPool.Stats()
	fmt.Printf("✓ Total connections: %d\n", stats.TotalConnections)
	fmt.Printf("✓ Healthy connections: %d\n", stats.HealthyConnections)
	fmt.Printf("✓ Failed connections: %d\n", stats.FailedConnections)
	fmt.Printf("✓ Repair attempts: %d\n", stats.RepairAttempts)
	fmt.Printf("✓ Last health check: %v\n", stats.LastHealthCheck)

	// Example 5: Feature comparison summary
	fmt.Println("\n=== Feature Summary ===")

	fmt.Println("Pool Sub-Package (advanced features):")
	fmt.Println("  ✓ pool.New() - Contract-implementation pattern")
	fmt.Println("  ✓ Connection health monitoring")
	fmt.Println("  ✓ Automatic connection repair")
	fmt.Println("  ✓ Round-robin connection selection")
	fmt.Println("  ✓ Pool statistics and metrics")
	fmt.Println("  ✓ Configurable pool size and behavior")

	fmt.Println("\nContract-Implementation Benefits:")
	fmt.Println("  ✓ Explicit import - users choose what they need")
	fmt.Println("  ✓ Pluggable architecture via ConnectionPooler interface")
	fmt.Println("  ✓ No coupling between core client and pool implementation")
	fmt.Println("  ✓ Easy to create alternative pool implementations")

	fmt.Println("\n=== Recommendation ===")
	fmt.Println("• Use pool.New() for high-throughput applications")
	fmt.Println("• Use WithConnectionPooler() option to plug pools into clients")
	fmt.Println("• Monitor pool statistics for production deployments")
}
