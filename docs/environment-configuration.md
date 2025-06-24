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

```go
// Use custom prefix (e.g., MYAPP_RABBITMQ_HOST instead of RABBITMQ_HOST)
publisher, err := rabbitmq.NewPublisherWithPrefix("MYAPP_")
consumer, err := rabbitmq.NewConsumerWithPrefix("MYAPP_")

// Load with custom prefix
envConfig, err := rabbitmq.LoadFromEnvWithPrefix("MYAPP_")
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

---

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
