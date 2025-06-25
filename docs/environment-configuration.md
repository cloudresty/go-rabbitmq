# Environment Configuration

[Home](../README.md) &nbsp;/&nbsp; [Docs](README.md) &nbsp;/&nbsp; Environment Configuration

&nbsp;

The package supports loading configuration from environment variables, making it ideal for containerized deployments and CI/CD pipelines.

&nbsp;

## Quick Start

```go
// Load with default RABBITMQ_ prefix
publisher, err := rabbitmq.NewPublisher()
consumer, err := rabbitmq.NewConsumer()

// Load configuration and customize before use
envConfig, err := rabbitmq.LoadFromEnv()
if err != nil {
    log.Fatal(err)
}

// Customize after loading
envConfig.ConnectionName = "my-custom-service"
publisher, err := rabbitmq.NewPublisherWithConfig(envConfig.ToPublisherConfig())
```

🔝 [back to top](#environment-configuration)

&nbsp;

## Custom Prefix

For multi-service deployments or when you need to avoid environment variable conflicts, you can use custom prefixes. The prefix is prepended to the standard `RABBITMQ_*` variable names.

🔝 [back to top](#environment-configuration)

&nbsp;

### How Custom Prefixes Work

When you specify a custom prefix like `MYAPP_`, the environment variable names become:

| Standard Variable | With Prefix `MYAPP_` | With Prefix `PAYMENTS_` |
|-------------------|---------------------|------------------------|
| `RABBITMQ_HOST` | `MYAPP_RABBITMQ_HOST` | `PAYMENTS_RABBITMQ_HOST` |
| `RABBITMQ_USERNAME` | `MYAPP_RABBITMQ_USERNAME` | `PAYMENTS_RABBITMQ_USERNAME` |
| `RABBITMQ_PASSWORD` | `MYAPP_RABBITMQ_PASSWORD` | `PAYMENTS_RABBITMQ_PASSWORD` |
| `RABBITMQ_PORT` | `MYAPP_RABBITMQ_PORT` | `PAYMENTS_RABBITMQ_PORT` |

🔝 [back to top](#environment-configuration)

&nbsp;

### Usage Examples

```go
// Use custom prefix for environment loading
publisher, err := rabbitmq.NewPublisherWithPrefix("MYAPP_")
consumer, err := rabbitmq.NewConsumerWithPrefix("MYAPP_")

// Or load configuration first, then create instances
envConfig, err := rabbitmq.LoadFromEnvWithPrefix("MYAPP_")
if err != nil {
    log.Fatal(err)
}

publisher, err := rabbitmq.NewPublisherWithConfig(envConfig.ToPublisherConfig())
consumer, err := rabbitmq.NewConsumerWithConfig(envConfig.ToConsumerConfig())
```

🔝 [back to top](#environment-configuration)

&nbsp;

### Environment Variable Setup

```bash
# For MYAPP_ prefix
export MYAPP_RABBITMQ_HOST=app-rabbitmq.internal
export MYAPP_RABBITMQ_USERNAME=myapp_user
export MYAPP_RABBITMQ_PASSWORD=myapp_secret
export MYAPP_RABBITMQ_CONNECTION_NAME=myapp-service

# For PAYMENTS_ prefix
export PAYMENTS_RABBITMQ_HOST=payments-rabbitmq.internal
export PAYMENTS_RABBITMQ_USERNAME=payments_user
export PAYMENTS_RABBITMQ_PASSWORD=payments_secret
export PAYMENTS_RABBITMQ_CONNECTION_NAME=payments-service
```

🔝 [back to top](#environment-configuration)

&nbsp;

### Multi-Service Docker Compose

```yaml
version: '3.8'
services:
  user-service:
    image: user-service:latest
    environment:
      USERS_RABBITMQ_HOST: rabbitmq
      USERS_RABBITMQ_USERNAME: users_app
      USERS_RABBITMQ_PASSWORD: users_secret
      USERS_RABBITMQ_CONNECTION_NAME: user-service

  payments-service:
    image: payments-service:latest
    environment:
      PAYMENTS_RABBITMQ_HOST: rabbitmq
      PAYMENTS_RABBITMQ_USERNAME: payments_app
      PAYMENTS_RABBITMQ_PASSWORD: payments_secret
      PAYMENTS_RABBITMQ_CONNECTION_NAME: payments-service
```

🔝 [back to top](#environment-configuration)

&nbsp;

## Supported Environment Variables

&nbsp;

### Connection Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RABBITMQ_USERNAME` | `guest` | RabbitMQ username |
| `RABBITMQ_PASSWORD` | `guest` | RabbitMQ password |
| `RABBITMQ_HOST` | `localhost` | RabbitMQ host |
| `RABBITMQ_PORT` | `5672` | AMQP port |
| `RABBITMQ_VHOST` | `/` | Virtual host |
| `RABBITMQ_PROTOCOL` | `amqp` | Protocol (amqp/amqps) |
| `RABBITMQ_CONNECTION_NAME` | `go-rabbitmq` | Connection identifier |
| `RABBITMQ_HEARTBEAT` | `10s` | Connection heartbeat |
| `RABBITMQ_RETRY_ATTEMPTS` | `5` | Connection retry attempts |
| `RABBITMQ_RETRY_DELAY` | `2s` | Delay between retries |
| `RABBITMQ_DIAL_TIMEOUT` | `30s` | TCP connection timeout |
| `RABBITMQ_CHANNEL_TIMEOUT` | `10s` | AMQP channel timeout |
| `RABBITMQ_AUTO_RECONNECT` | `true` | Enable auto-reconnection |
| `RABBITMQ_RECONNECT_DELAY` | `5s` | Reconnection delay |
| `RABBITMQ_MAX_RECONNECT_ATTEMPTS` | `0` | Max reconnect attempts (0=unlimited) |
| `RABBITMQ_TLS_ENABLED` | `false` | Enable TLS |
| `RABBITMQ_TLS_INSECURE` | `false` | Skip TLS verification |
| `RABBITMQ_HTTP_PROTOCOL` | `http` | Management API protocol |
| `RABBITMQ_HTTP_PORT` | `15672` | Management API port |

🔝 [back to top](#environment-configuration)

&nbsp;

### Publisher Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RABBITMQ_PUBLISHER_CONFIRMATION_TIMEOUT` | `5s` | Publish confirmation timeout |
| `RABBITMQ_PUBLISHER_SHUTDOWN_TIMEOUT` | `15s` | Publisher shutdown timeout |
| `RABBITMQ_PUBLISHER_PERSISTENT` | `true` | Enable message persistence |

🔝 [back to top](#environment-configuration)

&nbsp;

### Consumer Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RABBITMQ_CONSUMER_PREFETCH_COUNT` | `1` | Message prefetch count |
| `RABBITMQ_CONSUMER_AUTO_ACK` | `false` | Enable auto-acknowledgment |
| `RABBITMQ_CONSUMER_MESSAGE_TIMEOUT` | `5m` | Message processing timeout |
| `RABBITMQ_CONSUMER_SHUTDOWN_TIMEOUT` | `30s` | Consumer shutdown timeout |

🔝 [back to top](#environment-configuration)

&nbsp;

## Environment File Support

Create a `.env` file in your project root:

```bash
RABBITMQ_USERNAME=myuser
RABBITMQ_PASSWORD=mypassword
RABBITMQ_HOST=rabbitmq.example.com
RABBITMQ_PORT=5672
RABBITMQ_VHOST=/production
RABBITMQ_CONNECTION_NAME=my-production-service
RABBITMQ_HEARTBEAT=15s
RABBITMQ_PUBLISHER_PERSISTENT=true
RABBITMQ_CONSUMER_PREFETCH_COUNT=10
```

🔝 [back to top](#environment-configuration)

&nbsp;

## Docker/Kubernetes Integration

&nbsp;

### Docker Compose

```yaml
version: '3.8'
services:
  my-app:
    image: my-app:latest
    environment:
      RABBITMQ_HOST: rabbitmq
      RABBITMQ_USERNAME: app_user
      RABBITMQ_PASSWORD: secure_password
      RABBITMQ_VHOST: /myapp
      RABBITMQ_CONNECTION_NAME: my-app-instance
    depends_on:
      - rabbitmq
```

🔝 [back to top](#environment-configuration)

&nbsp;

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
      - name: my-app
        image: my-app:latest
        env:
        - name: RABBITMQ_HOST
          value: "rabbitmq-service"
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
```

🔝 [back to top](#environment-configuration)

&nbsp;

## URL Construction

Environment variables automatically construct proper AMQP and HTTP URLs:

```go
envConfig, _ := rabbitmq.LoadFromEnv()

// Automatically builds: amqp://user:pass@host:5672/vhost
amqpURL := envConfig.BuildAMQPURL()

// Automatically builds: http://host:15672
httpURL := envConfig.BuildHTTPURL()
```

🔝 [back to top](#environment-configuration)

&nbsp;

## Configuration Scenarios

The package supports three distinct configuration scenarios, allowing for flexible deployment strategies:

🔝 [back to top](#environment-configuration)

&nbsp;

### Scenario 1: All Defaults

When no environment variables are set, the package automatically applies sensible defaults for local development:

```go
// No environment variables set
publisher, err := rabbitmq.NewPublisher()
// Results in: amqp://guest:guest@localhost:5672/ with default settings
```

**Default Values Applied:**

- `RABBITMQ_HOST=localhost`
- `RABBITMQ_USERNAME=guest`
- `RABBITMQ_PASSWORD=guest`
- `RABBITMQ_PORT=5672`
- All other configuration uses package defaults

🔝 [back to top](#environment-configuration)

&nbsp;

### Scenario 2: Mixed Environment and Defaults

Set only the environment variables you need to customize, others fall back to defaults:

```bash
export RABBITMQ_HOST=production.rabbitmq.com
export RABBITMQ_USERNAME=myapp
export RABBITMQ_PASSWORD=secure_password
# Other variables use defaults
```

```go
publisher, err := rabbitmq.NewPublisher()
// Results in: amqp://myapp:secure_password@production.rabbitmq.com:5672/
// Port, vhost, timeouts, etc. use package defaults
```

🔝 [back to top](#environment-configuration)

&nbsp;

### Scenario 3: Full Environment Configuration

For production deployments, set all required environment variables:

```bash
export RABBITMQ_HOST=prod-rabbit-cluster.internal
export RABBITMQ_USERNAME=production_user
export RABBITMQ_PASSWORD=production_secret
export RABBITMQ_PORT=5672
export RABBITMQ_VHOST=/production
export RABBITMQ_CONNECTION_NAME=payment-service
export RABBITMQ_HEARTBEAT=30s
export RABBITMQ_PUBLISHER_PERSISTENT=true
```

```go
publisher, err := rabbitmq.NewPublisher()
// Results in: amqp://production_user:production_secret@prod-rabbit-cluster.internal:5672/production
// All values loaded from environment variables
```

🔝 [back to top](#environment-configuration)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
