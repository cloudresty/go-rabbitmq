package main

import (
	"context"
	"os"

	"github.com/cloudresty/emit"
	"github.com/cloudresty/go-rabbitmq"
)

func main() {
	// Example 1: Using default RABBITMQ_ prefix
	emit.Info.Msg("Creating publisher from environment variables (RABBITMQ_ prefix)")

	publisher, err := rabbitmq.NewPublisher()
	if err != nil {
		emit.Error.StructuredFields("Failed to create publisher from environment",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = publisher.Close()
	}()

	// Example 2: Using custom prefix (e.g., MYAPP_RABBITMQ_)
	emit.Info.Msg("Creating consumer from environment variables with custom prefix")

	consumer, err := rabbitmq.NewConsumerWithPrefix("MYAPP_")
	if err != nil {
		emit.Error.StructuredFields("Failed to create consumer from environment with prefix",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = consumer.Close()
	}()

	// Example 3: Loading env config and modifying before use
	emit.Info.Msg("Loading environment config and customizing")

	envConfig, err := rabbitmq.LoadFromEnv()
	if err != nil {
		emit.Error.StructuredFields("Failed to load config from environment",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	// Display loaded configuration
	emit.Info.StructuredFields("Loaded RabbitMQ configuration from environment",
		emit.ZString("host", envConfig.Host),
		emit.ZInt("port", envConfig.Port),
		emit.ZString("username", envConfig.Username),
		emit.ZString("vhost", envConfig.VHost),
		emit.ZString("protocol", envConfig.Protocol),
		emit.ZString("connection_name", envConfig.ConnectionName),
		emit.ZString("amqp_url", envConfig.BuildAMQPURL()),
		emit.ZString("http_url", envConfig.BuildHTTPURL()))

	// Customize config before using
	envConfig.ConnectionName = "env-demo-custom"

	// Create connection with customized config
	conn, err := rabbitmq.NewConnection(envConfig.ToConnectionConfig())
	if err != nil {
		emit.Error.StructuredFields("Failed to create connection",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = conn.Close()
	}()

	// Example 4: Publish a message using env-configured publisher
	err = publisher.Publish(context.Background(), rabbitmq.PublishConfig{
		Exchange:   "env-demo",
		RoutingKey: "test",
		Message:    []byte("Hello from environment-configured RabbitMQ!"),
	})
	if err != nil {
		emit.Error.StructuredFields("Failed to publish message",
			emit.ZString("error", err.Error()))
		os.Exit(1)
	}

	emit.Info.Msg("Environment configuration demo completed successfully")
}
