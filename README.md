# Go RabbitMQ

[Home](README.md) &nbsp;/

&nbsp;

**A modular, production-ready Go library for RabbitMQ with pluggable architecture.** Built on the **contract-implementation pattern**, where core interfaces live in the root package and concrete implementations are provided by specialized sub-packages. This design enables maximum flexibility, testability, and extensibility for enterprise messaging solutions.

&nbsp;

[![Go Reference](https://pkg.go.dev/badge/github.com/cloudresty/go-rabbitmq.svg)](https://pkg.go.dev/github.com/cloudresty/go-rabbitmq)
[![Go Tests](https://github.com/cloudresty/go-rabbitmq/actions/workflows/ci.yaml/badge.svg)](https://github.com/cloudresty/go-rabbitmq/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cloudresty/go-rabbitmq)](https://goreportcard.com/report/github.com/cloudresty/go-rabbitmq)
[![GitHub Tag](https://img.shields.io/github/v/tag/cloudresty/go-rabbitmq?label=Version)](https://github.com/cloudresty/go-rabbitmq/tags)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

&nbsp;

## Table of Contents

- [Requirements](#requirements)
- [Installation](#installation)
- [Pluggable Sub-Packages](#pluggable-sub-packages)
- [Key Features](#key-features)
- [Simple Queue Configuration](#simple-queue-configuration)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Requirements

- Go 1.24+ (recommended)
- RabbitMQ 4.0+ (recommended)

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Installation

```bash
go get github.com/cloudresty/go-rabbitmq
```

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Pluggable Sub-Packages

Each sub-package implements core interfaces defined in the root package, enabling you to mix and match features as needed:

| Package | Purpose | Key Features |
|---------|---------|--------------|
| **[compression/](compression/)** | Message compression | Gzip, Zlib with configurable thresholds |
| **[encryption/](encryption/)** | Message encryption | AES-256-GCM with secure key management |
| **[pool/](pool/)** | Connection pooling | Round-robin, health monitoring, auto-repair |
| **[performance/](performance/)** | Metrics & monitoring | Latency tracking, rate monitoring, statistics |
| **[saga/](saga/)** | Distributed transactions | Orchestration engine, compensation, atomic state |
| **[streams/](streams/)** | RabbitMQ Streams | High-throughput, durable, ordered messaging |
| **[shutdown/](shutdown/)** | Graceful shutdown | Signal handling, resource cleanup, timeouts |
| **[protobuf/](protobuf/)** | Protocol Buffers | Type-safe serialization, message routing |

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Key Features

### Contract-Implementation Architecture

- **Core Interfaces**: All contracts defined in the root package
- **Pluggable Implementations**: Concrete implementations in specialized sub-packages
- **Mix & Match**: Combine any features - encryption + compression + pooling + streams
- **Testing**: Easy mocking of interfaces for comprehensive unit testing

🔝 [back to top](#go-rabbitmq)

&nbsp;

### Production-Ready Features

- **Delivery Assurance**: Built-in publisher confirms with asynchronous callbacks for reliable message delivery
- **Publisher Retry**: Automatic re-publishing of nacked messages with configurable backoff and max attempts
- **Consumer Retry**: Header-based retry tracking that works across distributed consumers (supports both Quorum and Classic queues)
- **Topology Auto-Healing**: Automatic topology validation and recreation enabled by default
- **Connection Pooling**: Distribute load across multiple connections with health monitoring
- **Message Encryption**: AES-256-GCM encryption with secure key management
- **Compression**: Gzip/Zlib compression with configurable thresholds
- **Saga Pattern**: Distributed transaction orchestration with automatic compensation
- **Streams Support**: High-throughput RabbitMQ Streams for event sourcing
- **Performance Monitoring**: Latency tracking, rate monitoring, comprehensive metrics

🔝 [back to top](#go-rabbitmq)

&nbsp;

### Developer Experience

- **ULID Message IDs**: 6x faster than UUIDs, database-optimized, lexicographically sortable
- **Auto-Reconnection**: Intelligent retry with configurable exponential backoff
- **Graceful Shutdown**: Signal handling with proper resource cleanup and timeouts
- **Comprehensive Documentation**: Each sub-package has detailed README with examples

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Simple Queue Configuration

This library provides a straightforward approach to queue configuration with user control over topology:

&nbsp;

### Quorum Queues by Default

- **High Availability**: Built-in replication across cluster nodes
- **Data Safety**: No message loss during node failures
- **Poison Message Protection**: Automatic delivery limits prevent infinite redelivery loops
- **Better Performance**: Optimized for throughput in clustered environments

🔝 [back to top](#go-rabbitmq)

&nbsp;

### Dead Letter Configuration

- **Manual Configuration**: Full control over dead letter exchange and routing configuration
- **Flexible Setup**: Configure dead letter handling exactly as needed for your topology
- **Error Handling**: Failed messages routed according to your dead letter configuration

🔝 [back to top](#go-rabbitmq)

&nbsp;

### Topology Auto-Healing (Enabled by Default)

- **Automatic Validation**: Every publish/consume operation validates topology exists
- **Auto-Recreation**: Missing exchanges, queues, and bindings are automatically recreated
- **Background Monitoring**: Periodic validation every 30 seconds (default, customizable)
- **Zero Configuration**: Works out of the box - no setup required

🔝 [back to top](#go-rabbitmq)

&nbsp;

### Easy Customization

```go
// Default: Auto-healing quorum queue (production-ready!)
client, _ := rabbitmq.NewClient(rabbitmq.FromEnv())
admin := client.Admin()
queue, _ := admin.DeclareQueue(ctx, "orders")  // Automatically tracked & protected

// Custom quorum settings
queue, _ := admin.DeclareQueue(ctx, "payments",
    rabbitmq.WithQuorumGroupSize(5),       // Custom cluster size
    rabbitmq.WithDeliveryLimit(3),         // Max retry attempts
)

// With dead letter configuration
queue, _ := admin.DeclareQueue(ctx, "processing",
    rabbitmq.WithDeadLetter("errors.dlx", "failed"), // Manual DLX setup
)

// Advanced users can opt-out if needed
client, _ := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithoutTopologyValidation(),            // Disable auto-healing
    rabbitmq.WithoutTopologyAutoRecreation(),        // Keep validation, disable auto-recreation
    rabbitmq.WithoutTopologyBackgroundValidation(),  // Disable background monitoring only
)

// Or customize the background validation interval
client, _ := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithTopologyValidationInterval(10*time.Second), // Custom interval
)

// Legacy compatibility (opt-in)
queue, _ := admin.DeclareQueue(ctx, "legacy",
    rabbitmq.WithClassicQueue(),           // Classic queue type
)
```

**Benefits**: Get enterprise-grade reliability and availability with simple, user-controlled configuration.

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Documentation

| Document | Description |
|----------|-------------|
| **Sub-Package READMEs** | Detailed documentation for each pluggable feature |
| [compression/](compression/) | Message compression with Gzip and Zlib |
| [encryption/](encryption/) | AES-256-GCM message encryption |
| [performance/](performance/) | Metrics collection and monitoring |
| [pool/](pool/) | Connection pooling with health monitoring |
| [protobuf/](protobuf/) | Protocol Buffers integration |
| [saga/](saga/) | Distributed transaction orchestration |
| [shutdown/](shutdown/) | Graceful shutdown management |
| [streams/](streams/) | High-throughput RabbitMQ Streams |
| **Additional Docs** | |
| [Quick Start](docs/quick-start.md) | Get up and running with minimal code |
| [API Reference](docs/api-reference.md) | Complete function reference and usage patterns |
| [Environment Variables](docs/environment-variables.md) | List of environment variables for configuration |
| [Environment Variables](docs/environment-variables.md) | Complete environment variable reference and usage |
| [Production Features](docs/production-features.md) | Auto-reconnection, graceful shutdown, HA queues |
| [Examples](examples/) | Working examples for each feature |
| [ULID Message IDs](docs/ulid-message-ids.md) | Using ULIDs for message IDs in RabbitMQ |
| [Usage Patterns](docs/usage-patterns.md) | Common patterns for using the library effectively |

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Ensure all tests pass
5. Submit a pull request

🔝 [back to top](#go-rabbitmq)

&nbsp;

## Security

If you discover a security vulnerability, please report it via email to [security@cloudresty.com](mailto:security@cloudresty.com).

🔝 [back to top](#go-rabbitmq)

&nbsp;

## License

This project is licensed under the MIT License - see the [LICENSE.txt](LICENSE.txt) file for details.

🔝 [back to top](#go-rabbitmq)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
