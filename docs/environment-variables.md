# Environment Variables

[Home](../README.md) &nbsp;/&nbsp; [Docs](README.md) &nbsp;/&nbsp; Environment Variables

&nbsp;

The go-rabbitmq package supports comprehensive configuration through environment variables, making it ideal for containerized deployments, microservices, and cloud-native applications. All configuration can be loaded automatically using `FromEnv()` or `FromEnvWithPrefix()`.

&nbsp;

## Core Connection Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `RABBITMQ_HOSTS` | RabbitMQ host addresses (comma-separated for failover) | `localhost:5672` | `mq1:5672,mq2:5672,mq3:5672` |
| `RABBITMQ_USERNAME` | Authentication username | `guest` | `myuser` |
| `RABBITMQ_PASSWORD` | Authentication password | `guest` | `mypassword` |
| `RABBITMQ_VHOST` | Virtual host | `/` | `/production` |
| `RABBITMQ_CONNECTION_NAME` | Connection identifier for logging and monitoring | `go-rabbitmq` | `my-service` |

🔝 [back to top](#environment-variables)

&nbsp;

## Protocol and Security Settings

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `RABBITMQ_PROTOCOL` | Connection protocol (amqp or amqps) | `amqp` | `amqps` |
| `RABBITMQ_TLS_ENABLED` | Enable TLS/SSL connections | `false` | `true` |
| `RABBITMQ_TLS_INSECURE` | Skip TLS certificate verification (development only) | `false` | `true` |

🔝 [back to top](#environment-variables)

&nbsp;

## Timeout and Connection Settings

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `RABBITMQ_DIAL_TIMEOUT` | Connection establishment timeout | `30s` | `60s` |
| `RABBITMQ_CHANNEL_TIMEOUT` | Channel creation timeout | `10s` | `15s` |
| `RABBITMQ_HEARTBEAT` | Connection heartbeat interval | `10s` | `30s` |
| `RABBITMQ_RETRY_ATTEMPTS` | Initial connection retry attempts | `5` | `3` |
| `RABBITMQ_RETRY_DELAY` | Delay between initial connection retries | `2s` | `5s` |

🔝 [back to top](#environment-variables)

&nbsp;

## Auto-Reconnection Settings

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `RABBITMQ_AUTO_RECONNECT` | Enable automatic reconnection on connection loss | `true` | `false` |
| `RABBITMQ_RECONNECT_DELAY` | Delay between reconnection attempts | `5s` | `10s` |
| `RABBITMQ_MAX_RECONNECT_ATTEMPTS` | Maximum reconnection attempts (0 = unlimited) | `0` | `10` |

🔝 [back to top](#environment-variables)

&nbsp;

## Publisher Settings

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT` | Publisher confirmation timeout | `5s` | `10s` |
| `RABBITMQ_PUBLISHER_SHUTDOWN_TIMEOUT` | Publisher graceful shutdown timeout | `15s` | `30s` |
| `RABBITMQ_PUBLISHER_PERSISTENT` | Make messages persistent by default | `true` | `false` |

🔝 [back to top](#environment-variables)

&nbsp;

## Consumer Settings

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `RABBITMQ_CONSUMER_PREFETCH_COUNT` | Default prefetch count for consumers | `1` | `10` |
| `RABBITMQ_CONSUMER_AUTO_ACK` | Enable automatic acknowledgment | `false` | `true` |
| `RABBITMQ_CONSUMER_MESSAGE_TIMEOUT` | Message processing timeout | `5m` | `10m` |
| `RABBITMQ_CONSUMER_SHUTDOWN_TIMEOUT` | Consumer graceful shutdown timeout | `30s` | `60s` |

🔝 [back to top](#environment-variables)

&nbsp;

## Topology Validation and Auto-Healing Settings

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `RABBITMQ_TOPOLOGY_VALIDATION` | Enable topology validation before operations | `true` | `false` |
| `RABBITMQ_TOPOLOGY_AUTO_RECREATION` | Auto-recreate missing exchanges, queues, and bindings | `true` | `false` |
| `RABBITMQ_TOPOLOGY_BACKGROUND_VALIDATION` | Enable periodic background topology validation | `true` | `false` |
| `RABBITMQ_TOPOLOGY_VALIDATION_INTERVAL` | Background validation interval | `30s` | `60s` |

These settings provide enterprise-grade reliability by ensuring your topology remains consistent and is automatically recreated if deleted externally.

🔝 [back to top](#environment-variables)

&nbsp;

## HTTP Management API Settings

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `RABBITMQ_HTTP_PROTOCOL` | Management API protocol (http or https) | `http` | `https` |
| `RABBITMQ_HTTP_HOST` | Management API host | `localhost` | `rabbitmq-mgmt.prod.com` |
| `RABBITMQ_HTTP_PORT` | Management API port | `15672` | `15671` |

🔝 [back to top](#environment-variables)

&nbsp;

## Configuration Precedence

When creating clients, configuration follows this precedence order:

1. **Explicit options** passed to `NewClient()`
2. **Environment variables** (when using `FromEnv()` or `FromEnvWithPrefix()`)
3. **Package defaults** (as fallback)

This allows you to use environment variables for base configuration while overriding specific values programmatically.

```go
// Environment: RABBITMQ_HOSTS=prod-rabbit.com:5672
// Environment: RABBITMQ_HEARTBEAT=30s

client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(), // Load base config from environment
    rabbitmq.WithHeartbeat(60*time.Second), // Override environment RABBITMQ_HEARTBEAT
    rabbitmq.WithConnectionName("priority-service"), // Override connection name
)
```

🔝 [back to top](#environment-variables)

&nbsp;

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client using environment variables
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal("Failed to create client:", err)
    }
    defer client.Close()

    // Test connection
    if err := client.Ping(context.Background()); err != nil {
        log.Fatal("Connection failed:", err)
    }

    log.Printf("Connected to RabbitMQ as: %s", client.ConnectionName())
}
```

🔝 [back to top](#environment-variables)

&nbsp;

## Custom Prefix Support

Use custom prefixes to isolate environment variables for different services or environments:

```go
// Create client with custom-prefixed environment variables
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnvWithPrefix("MYAPP_"),
    rabbitmq.WithConnectionName("my-service"),
)
if err != nil {
    log.Fatal("Failed to create client:", err)
}
defer client.Close()

// Now looks for:
// MYAPP_RABBITMQ_HOSTS
// MYAPP_RABBITMQ_USERNAME
// MYAPP_RABBITMQ_PASSWORD
// etc.
```

🔝 [back to top](#environment-variables)

&nbsp;

## Complete Usage Examples

### Basic Environment-Driven Application

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client from environment
    client, err := rabbitmq.NewClient(
        rabbitmq.FromEnv(),
        rabbitmq.WithConnectionName("notification-service"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Setup topology
    admin := client.Admin()
    err = admin.DeclareExchange(context.Background(), "notifications", "topic")
    if err != nil {
        log.Fatal(err)
    }

    _, err = admin.DeclareQueue(context.Background(), "email-queue")
    if err != nil {
        log.Fatal(err)
    }

    // Create publisher
    publisher, err := client.NewPublisher(
        rabbitmq.WithDefaultExchange("notifications"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer publisher.Close()

    // Create consumer
    consumer, err := client.NewConsumer()
    if err != nil {
        log.Fatal(err)
    }
    defer consumer.Close()

    // Publish a message
    message := rabbitmq.NewMessage([]byte("Hello from env config!"))
    err = publisher.Publish(context.Background(), "notifications", "email.send", message)
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Application running with environment configuration")
}
```

🔝 [back to top](#environment-variables)

&nbsp;

### Configuration Validation Pattern

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client with validation
    client, err := createValidatedClient()
    if err != nil {
        log.Fatal("Configuration validation failed:", err)
    }
    defer client.Close()

    log.Println("RabbitMQ client configured and validated successfully")
}

func createValidatedClient() (*rabbitmq.Client, error) {
    // Create client from environment
    client, err := rabbitmq.NewClient(
        rabbitmq.FromEnv(),
        rabbitmq.WithConnectionName("validated-service"),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create client: %w", err)
    }

    // Validate connection
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := client.Ping(ctx); err != nil {
        client.Close()
        return nil, fmt.Errorf("connection validation failed: %w", err)
    }

    // Log connection details (without sensitive info)
    log.Printf("Connected to RabbitMQ as: %s", client.ConnectionName())
    log.Printf("Primary connection URL: %s", maskPassword(client.URL()))

    return client, nil
}

func maskPassword(url string) string {
    // Simple password masking for logging
    // In production, use a proper URL parsing library
    return "amqp://***:***@host:port/vhost"
}
```

🔝 [back to top](#environment-variables)

&nbsp;

## Using Environment Variables

### Basic Usage with `FromEnv()`

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Create client using environment variables
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Test connection
    if err := client.Ping(context.Background()); err != nil {
        log.Fatal("Connection failed:", err)
    }

    log.Printf("Connected to RabbitMQ as: %s", client.ConnectionName())
}
```

🔝 [back to top](#environment-variables)

&nbsp;

### Custom Prefix with `FromEnvWithPrefix()`

```go
// Use custom environment variable prefix
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnvWithPrefix("MYAPP_"),
    rabbitmq.WithConnectionName("my-service"), // Override specific settings
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Now uses MYAPP_RABBITMQ_HOSTS, MYAPP_RABBITMQ_USERNAME, etc.
```

🔝 [back to top](#environment-variables)

&nbsp;

### Combining Environment and Explicit Options

```go
// Environment variables provide base configuration,
// explicit options override specific settings
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(), // Load base configuration from environment
    rabbitmq.WithConnectionName("priority-service"), // Override connection name
    rabbitmq.WithHeartbeat(45*time.Second), // Override heartbeat
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

🔝 [back to top](#environment-variables)

&nbsp;

## Example Configurations

### Development Environment (.env file)

```bash
# Basic development configuration
RABBITMQ_HOSTS=localhost:5672
RABBITMQ_USERNAME=guest
RABBITMQ_PASSWORD=guest
RABBITMQ_VHOST=/
RABBITMQ_CONNECTION_NAME=myapp-dev
RABBITMQ_CONSUMER_PREFETCH_COUNT=1
RABBITMQ_PUBLISHER_PERSISTENT=true
RABBITMQ_AUTO_RECONNECT=true
```

🔝 [back to top](#environment-variables)

&nbsp;

### Production Environment

```bash
# Production environment with failover and security
RABBITMQ_HOSTS=mq1.prod.com:5672,mq2.prod.com:5672,mq3.prod.com:5672
RABBITMQ_USERNAME=produser
RABBITMQ_PASSWORD=secure_password_from_vault
RABBITMQ_VHOST=/production
RABBITMQ_CONNECTION_NAME=payment-service-prod
RABBITMQ_PROTOCOL=amqps
RABBITMQ_TLS_ENABLED=true
RABBITMQ_HEARTBEAT=30s
RABBITMQ_DIAL_TIMEOUT=60s
RABBITMQ_AUTO_RECONNECT=true
RABBITMQ_RECONNECT_DELAY=10s
RABBITMQ_MAX_RECONNECT_ATTEMPTS=10
RABBITMQ_CONSUMER_PREFETCH_COUNT=100
RABBITMQ_CONSUMER_MESSAGE_TIMEOUT=10m
RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT=10s

# Topology validation (production-ready defaults)
RABBITMQ_TOPOLOGY_VALIDATION=true
RABBITMQ_TOPOLOGY_AUTO_RECREATION=true
RABBITMQ_TOPOLOGY_BACKGROUND_VALIDATION=true
RABBITMQ_TOPOLOGY_VALIDATION_INTERVAL=30s
```

🔝 [back to top](#environment-variables)

&nbsp;

### Cloud-Native Deployment

```bash
# Kubernetes/Docker deployment with service discovery
RABBITMQ_HOSTS=rabbitmq-cluster.messaging.svc.cluster.local:5672
RABBITMQ_USERNAME=app-user
RABBITMQ_PASSWORD=${RABBITMQ_SECRET} # From Kubernetes secret
RABBITMQ_VHOST=/microservices
RABBITMQ_CONNECTION_NAME=${POD_NAME}-${SERVICE_NAME}
RABBITMQ_TLS_ENABLED=true
RABBITMQ_HEARTBEAT=60s
RABBITMQ_AUTO_RECONNECT=true
RABBITMQ_RECONNECT_DELAY=15s
RABBITMQ_CONSUMER_PREFETCH_COUNT=50
RABBITMQ_PUBLISHER_PERSISTENT=true
```

🔝 [back to top](#environment-variables)

&nbsp;

### High-Throughput Configuration

```bash
# Optimized for high-throughput scenarios
RABBITMQ_HOSTS=mq-cluster.internal:5672
RABBITMQ_USERNAME=high-perf-user
RABBITMQ_PASSWORD=performance_password
RABBITMQ_VHOST=/fast-lane
RABBITMQ_CONNECTION_NAME=analytics-processor
RABBITMQ_HEARTBEAT=120s
RABBITMQ_CONSUMER_PREFETCH_COUNT=1000
RABBITMQ_CONSUMER_AUTO_ACK=false
RABBITMQ_CONSUMER_MESSAGE_TIMEOUT=30s
RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT=1s
RABBITMQ_PUBLISHER_PERSISTENT=false # For speed, accept message loss risk
```

🔝 [back to top](#environment-variables)

&nbsp;

### Multi-Service Configuration

```bash
# Service A environment variables
SERVICE_A_RABBITMQ_HOSTS=mq.internal:5672
SERVICE_A_RABBITMQ_USERNAME=service-a
SERVICE_A_RABBITMQ_PASSWORD=service_a_pass
SERVICE_A_RABBITMQ_VHOST=/service-a
SERVICE_A_RABBITMQ_CONNECTION_NAME=user-service

# Service B environment variables
SERVICE_B_RABBITMQ_HOSTS=mq.internal:5672
SERVICE_B_RABBITMQ_USERNAME=service-b
SERVICE_B_RABBITMQ_PASSWORD=service_b_pass
SERVICE_B_RABBITMQ_VHOST=/service-b
SERVICE_B_RABBITMQ_CONNECTION_NAME=order-service
```

```go
// In Service A
clientA, err := rabbitmq.NewClient(rabbitmq.FromEnvWithPrefix("SERVICE_A_"))

// In Service B
clientB, err := rabbitmq.NewClient(rabbitmq.FromEnvWithPrefix("SERVICE_B_"))
```

🔝 [back to top](#environment-variables)

&nbsp;

## Deployment Examples

### Docker Compose

```yaml
version: '3.8'

services:
  app:
    image: myapp:latest
    environment:
      RABBITMQ_HOSTS: rabbitmq:5672
      RABBITMQ_USERNAME: guest
      RABBITMQ_PASSWORD: guest
      RABBITMQ_VHOST: /
      RABBITMQ_CONNECTION_NAME: myapp-docker
      RABBITMQ_CONSUMER_PREFETCH_COUNT: 10
    depends_on:
      - rabbitmq

  rabbitmq:
    image: rabbitmq:3-management
    ports:
      - "5672:5672"
      - "15672:15672"
```

🔝 [back to top](#environment-variables)

&nbsp;

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: myapp
        image: myapp:latest
        env:
        - name: RABBITMQ_HOSTS
          value: "rabbitmq-service.messaging.svc.cluster.local:5672"
        - name: RABBITMQ_USERNAME
          valueFrom:
            secretKeyRef:
              name: rabbitmq-secret
              key: username
        - name: RABBITMQ_PASSWORD
          valueFrom:
            secretKeyRef:
              name: rabbitmq-secret
              key: password
        - name: RABBITMQ_VHOST
          value: "/production"
        - name: RABBITMQ_CONNECTION_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: RABBITMQ_TLS_ENABLED
          value: "true"
        - name: RABBITMQ_CONSUMER_PREFETCH_COUNT
          value: "50"
```

🔝 [back to top](#environment-variables)

&nbsp;

## Best Practices

### Security

1. **Never hardcode passwords** - Use environment variables or secret management systems
2. **Enable TLS in production** - Set `RABBITMQ_TLS_ENABLED=true`
3. **Use proper vhosts** - Isolate different environments and services
4. **Limit connection names** - Use descriptive but not sensitive information

🔝 [back to top](#environment-variables)

&nbsp;

### Reliability

1. **Configure multiple hosts** - Use comma-separated hosts for failover
2. **Set appropriate timeouts** - Balance responsiveness with stability
3. **Enable auto-reconnect** - Ensure `RABBITMQ_AUTO_RECONNECT=true` in production
4. **Monitor heartbeats** - Set reasonable heartbeat intervals

🔝 [back to top](#environment-variables)

&nbsp;

### Performance

1. **Tune prefetch counts** - Higher values for throughput, lower for fairness
2. **Adjust timeouts** - Based on your message processing requirements
3. **Consider persistence** - Balance durability with performance needs
4. **Use confirmation timeouts** - Set based on your latency requirements

🔝 [back to top](#environment-variables)

&nbsp;

### Deployment

1. **Use environment-specific configurations** - Different settings per environment
2. **Leverage container orchestration** - Kubernetes secrets, Docker Compose
3. **Document required variables** - Make deployment requirements clear
4. **Validate configuration** - Test connections in health checks

🔝 [back to top](#environment-variables)

&nbsp;

The package validates environment variables at startup and provides clear error messages:

```bash
# Invalid timeout format
RABBITMQ_DIAL_TIMEOUT=invalid

# Error: invalid duration format for RABBITMQ_DIAL_TIMEOUT: time: invalid duration "invalid"
```

```bash
# Invalid boolean value
RABBITMQ_AUTO_RECONNECT=maybe

# Error: invalid boolean value for RABBITMQ_AUTO_RECONNECT: strconv.ParseBool: parsing "maybe": invalid syntax
```

```bash
# Invalid port in hosts
RABBITMQ_HOSTS=localhost:invalid

# Error: invalid port in RABBITMQ_HOSTS: strconv.Atoi: parsing "invalid": invalid syntax
```

🔝 [back to top](#environment-variables)

&nbsp;

---

&nbsp;

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
