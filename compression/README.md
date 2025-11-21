# Compression Package

[Home](../README.md) &nbsp;/&nbsp; Compression Package

&nbsp;

The `compression` package provides message compression functionality for the `go-rabbitmq` library. It implements various compression algorithms that can reduce message size before publishing to RabbitMQ, helping to save bandwidth and improve the efficiency of message flow.

&nbsp;

## Features

- **Gzip Compression**: Industry-standard compression with configurable levels
- **Zlib Compression**: Alternative compression algorithm with good performance
- **No-op Compressor**: For testing and development scenarios
- **Threshold-based**: Only compress messages above a configurable size threshold
- **Efficiency Check**: Only uses compressed data if it's actually smaller than original

🔝 [back to top](#compression-package)

&nbsp;

## Installation

```bash
go get github.com/cloudresty/go-rabbitmq/compression
```

🔝 [back to top](#compression-package)

&nbsp;

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/compression"
)

func main() {
    // Create a client
    client, err := rabbitmq.NewClient(
        rabbitmq.WithHosts("localhost:5672"),
        rabbitmq.WithCredentials("guest", "guest"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create a gzip compressor (compress messages > 1KB with default level)
    compressor := compression.NewGzip(1024, compression.DefaultLevel)

    // Create publisher with compression
    publisher, err := client.NewPublisher(
        rabbitmq.WithCompression(compressor),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer publisher.Close()

    // Large message will be automatically compressed
    largeMessage := rabbitmq.NewMessage(make([]byte, 5000)) // 5KB message
    err = publisher.Publish(context.Background(), "", "test.queue", largeMessage)
    if err != nil {
        log.Fatal(err)
    }
}
```

🔝 [back to top](#compression-package)

&nbsp;

## Available Compressors

### Gzip Compressor

```go
// Create with custom threshold and compression level
compressor := compression.NewGzip(
    2048,                          // 2KB threshold
    compression.BestCompression,   // Maximum compression
)

// Compression levels
compression.BestSpeed       // Fastest compression (level 1)
compression.BestCompression // Best compression ratio (level 9)
compression.DefaultLevel    // Balanced speed/ratio (level 6)
compression.NoCompression   // No compression (level 0)
```

🔝 [back to top](#compression-package)

&nbsp;

### Zlib Compressor

```go
// Create zlib compressor
compressor := compression.NewZlib(1024, compression.DefaultLevel)
```

🔝 [back to top](#compression-package)

&nbsp;

### No-op Compressor

```go
// For testing or when compression is not desired
compressor := compression.NewNop()
```

🔝 [back to top](#compression-package)

&nbsp;

## Interface

All compressors implement the `Compressor` interface:

```go
type Compressor interface {
    Compress(data []byte) ([]byte, error)
    Decompress(data []byte) ([]byte, error)
    Algorithm() string
    Threshold() int
}
```

🔝 [back to top](#compression-package)

&nbsp;

## Best Practices

### Threshold Selection

- **Small messages (< 500 bytes)**: Usually not worth compressing due to overhead
- **Medium messages (500B - 10KB)**: Set threshold around 512-1024 bytes
- **Large messages (> 10KB)**: Set lower threshold (256-512 bytes) for maximum savings

### Compression Level Selection

- **High throughput**: Use `BestSpeed` for minimal CPU overhead
- **Bandwidth limited**: Use `BestCompression` for maximum size reduction
- **Balanced**: Use `DefaultLevel` for good compromise

🔝 [back to top](#compression-package)

&nbsp;

### Performance Considerations

```go
// For high-throughput scenarios
fastCompressor := compression.NewGzip(1024, compression.BestSpeed)

// For bandwidth-limited scenarios
efficientCompressor := compression.NewGzip(512, compression.BestCompression)

// For development/testing
noCompressor := compression.NewNop()
```

🔝 [back to top](#compression-package)

&nbsp;

## Examples

### Consumer with Decompression

```go
// Consumer automatically decompresses messages
consumer, err := client.NewConsumer(
    rabbitmq.WithConsumerCompression(compressor),
)

err = consumer.Consume(ctx, "compressed.queue", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    // Message body is automatically decompressed
    fmt.Printf("Received: %s\n", string(delivery.Body))
    return delivery.Ack()
})
```

🔝 [back to top](#compression-package)

&nbsp;

### Custom Compression Logic

```go
// Implement custom compression behavior
type CustomCompressor struct {
    threshold int
}

func (c *CustomCompressor) Compress(data []byte) ([]byte, error) {
    // Your custom compression logic
    return data, nil
}

func (c *CustomCompressor) Decompress(data []byte) ([]byte, error) {
    // Your custom decompression logic
    return data, nil
}

func (c *CustomCompressor) Algorithm() string {
    return "custom"
}

func (c *CustomCompressor) Threshold() int {
    return c.threshold
}
```

🔝 [back to top](#compression-package)

&nbsp;

## Performance

Benchmark results (approximate, varies by data):

| Algorithm | Compression Ratio | Speed | CPU Usage |
|-----------|------------------|-------|-----------|
| Gzip (BestSpeed) | 60-70% | Fast | Low |
| Gzip (Default) | 50-60% | Medium | Medium |
| Gzip (BestCompression) | 40-50% | Slow | High |
| Zlib (Default) | 50-60% | Medium | Medium |
| No-op | 0% | Fastest | Minimal |

🔝 [back to top](#compression-package)

&nbsp;

## Thread Safety

All compressor implementations are thread-safe and can be used concurrently across multiple goroutines.

🔝 [back to top](#compression-package)

&nbsp;

&nbsp;

---

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
