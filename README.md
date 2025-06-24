# Go RabbitMQ

[Home](README.md) &nbsp;/

&nbsp;

A modern, production-ready Go package for RabbitMQ operations with environment-first configuration, ULID message IDs, and comprehensive production features.

&nbsp;

[![Go Reference](https://pkg.go.dev/badge/github.com/cloudresty/go-rabbitmq.svg)](https://pkg.go.dev/github.com/cloudresty/go-rabbitmq)
[![Go Tests](https://github.com/cloudresty/go-rabbitmq/actions/workflows/ci.yaml/badge.svg)](https://github.com/cloudresty/go-rabbitmq/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cloudresty/go-rabbitmq)](https://goreportcard.com/report/github.com/cloudresty/go-rabbitmq)
[![GitHub Tag](https://img.shields.io/github/v/tag/cloudresty/go-rabbitmq?label=Version)](https://github.com/cloudresty/go-rabbitmq/tags)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

&nbsp;

## Table of Contents

- [Key Features](#key-features)
- [Quick Start](#quick-start)
  - [Installation](#installation)
  - [Basic Usage](#basic-usage)
  - [Environment Configuration](#environment-configuration)
- [Documentation](#documentation)
- [Why This Package?](#why-this-package)
- [Production Usage](#production-usage)
- [Requirements](#requirements)
- [Contributing](#contributing)
- [License](#license)

&nbsp;

## Key Features

- **Environment-First**: Configure via environment variables for cloud-native deployments
- **ULID Message IDs**: 6x faster generation, database-optimized, lexicographically sortable
- **Auto-Reconnection**: Intelligent retry with configurable backoff
- **Production-Ready**: Graceful shutdown, timeouts, dead letter queues, HA support
- **High Performance**: Zero-allocation logging, optimized for throughput
- **Fully Tested**: Comprehensive test coverage with CI/CD pipeline

&nbsp;

## Quick Start

&nbsp;

### Installation

```bash
go get github.com/cloudresty/go-rabbitmq
```

&nbsp;

### Basic Usage

```go
package main

import (
    "context"
    "github.com/cloudresty/go-rabbitmq"
)

func main() {
    // Publisher - uses RABBITMQ_* environment variables
    publisher, err := rabbitmq.NewPublisher()
    if err != nil {
        panic(err)
    }
    defer publisher.Close()

    // Publish a message with auto-generated ULID
    err = publisher.Publish(context.Background(), rabbitmq.PublishConfig{
        Exchange:   "events",
        RoutingKey: "user.created",
        Message:    []byte(`{"user_id": "123", "name": "John"}`),
    })

    // Consumer - uses RABBITMQ_* environment variables
    consumer, err := rabbitmq.NewConsumer()
    if err != nil {
        panic(err)
    }
    defer consumer.Close()

    // Consume messages
    err = consumer.Consume(context.Background(), rabbitmq.ConsumeConfig{
        Queue: "user-events",
        Handler: func(ctx context.Context, message []byte) error {
            // Process message
            return nil
        },
    })
}
```

&nbsp;

### Environment Configuration

Set environment variables for your deployment:

```bash
export RABBITMQ_HOST=localhost
export RABBITMQ_USERNAME=guest
export RABBITMQ_PASSWORD=guest
export RABBITMQ_CONNECTION_NAME=my-service
```

&nbsp;

## Documentation

| Document | Description |
|----------|-------------|
| [API Reference](docs/api-reference.md) | Complete function reference and usage patterns |
| [Environment Configuration](docs/environment-configuration.md) | Environment variables and deployment configurations |
| [Production Features](docs/production-features.md) | Auto-reconnection, graceful shutdown, HA queues, dead letters |
| [ULID Message IDs](docs/ulid-message-ids.md) | High-performance, database-optimized message identifiers |
| [Examples](docs/examples.md) | Comprehensive examples and usage patterns |

&nbsp;

## Why This Package?

This package is designed for modern cloud-native applications that require robust, high-performance messaging solutions. It leverages the power of RabbitMQ while providing a developer-friendly API that integrates seamlessly with environment-based configurations.

&nbsp;

### Environment-First Design

Perfect for modern cloud deployments with Docker, Kubernetes, and CI/CD pipelines. No more hardcoded connection strings.

&nbsp;

### ULID Message IDs

Get 6x faster message ID generation with better database performance compared to UUIDs. Natural time-ordering and collision resistance.

&nbsp;

### Production-Ready

Built-in support for high availability, graceful shutdown, automatic reconnection, and comprehensive timeout controls.

&nbsp;

### Performance Optimized

Zero-allocation logging, efficient ULID generation, and optimized for high-throughput scenarios.

&nbsp;

## Production Usage

```go
// Use custom environment prefix for multi-service deployments
publisher, err := rabbitmq.NewPublisherWithPrefix("PAYMENTS_")

// Declare production-ready queues with dead letter support
err = publisher.DeclareQuorumQueue("payments")

// Graceful shutdown with signal handling
shutdownManager := rabbitmq.NewShutdownManager(config)
shutdownManager.SetupSignalHandler()
shutdownManager.Register(publisher, consumer)
shutdownManager.Wait() // Blocks until SIGINT/SIGTERM
```

&nbsp;

## Requirements

- Go 1.24+ (recommended)
- RabbitMQ 4.0+ (recommended)

&nbsp;

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Ensure all tests pass
5. Submit a pull request

## Security

If you discover a security vulnerability, please report it via email to [security@cloudresty.com](mailto:security@cloudresty.com).

&nbsp;

## License

This project is licensed under the MIT License - see the [LICENSE.txt](LICENSE.txt) file for details.

&nbsp;

---

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
