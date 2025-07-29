# Compression Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [Compression](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides a complete API reference for the `compression` package, which implements message compression functionality for the go-rabbitmq library.

&nbsp;

## Constructor Functions

| Function | Description |
|----------|-------------|
| `NewGzip(threshold int, level int)` | Creates a new gzip compressor with specified threshold and compression level |
| `NewZlib(threshold int, level int)` | Creates a new zlib compressor with specified threshold and compression level |
| `NewNop()` | Creates a no-operation compressor that doesn't compress data |

🔝 [back to top](#compression-package-api-reference)

&nbsp;

## Compression Level Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `BestSpeed` | 1 | Fastest compression with larger output |
| `BestCompression` | 9 | Best compression ratio with slower speed |
| `DefaultLevel` | -1 | Default compression level (balanced speed/ratio) |
| `NoCompression` | 0 | No compression applied |

🔝 [back to top](#compression-package-api-reference)

&nbsp;

## Interface Methods

All compressor types implement the `rabbitmq.MessageCompressor` interface:

| Method | Description |
|--------|-------------|
| `Compress(data []byte) ([]byte, error)` | Compresses data if it exceeds the threshold size |
| `Decompress(data []byte) ([]byte, error)` | Decompresses previously compressed data |
| `Algorithm() string` | Returns the compression algorithm name |
| `Threshold() int` | Returns the minimum size threshold for compression |

🔝 [back to top](#compression-package-api-reference)

&nbsp;

## Gzip Compressor

### Constructor

```go
compressor := compression.NewGzip(threshold, level)
```

### Methods

| Method | Description |
|--------|-------------|
| `Algorithm()` | Returns "gzip" |
| `Threshold()` | Returns the configured size threshold |
| `Compress(data []byte)` | Compresses data using gzip if size exceeds threshold |
| `Decompress(data []byte)` | Decompresses gzip data with validation |

### Parameters

- **threshold**: Minimum message size in bytes to trigger compression (default: 1024)
- **level**: Compression level from 1 (fastest) to 9 (best compression)

🔝 [back to top](#compression-package-api-reference)

&nbsp;

## Zlib Compressor

### Zlib Constructor

```go
compressor := compression.NewZlib(threshold, level)
```

### Zlib Methods

| Method | Description |
|--------|-------------|
| `Algorithm()` | Returns "zlib" |
| `Threshold()` | Returns the configured size threshold |
| `Compress(data []byte)` | Compresses data using zlib if size exceeds threshold |
| `Decompress(data []byte)` | Decompresses zlib data with validation |

### Zlib Parameters

- **threshold**: Minimum message size in bytes to trigger compression (default: 1024)
- **level**: Compression level from 1 (fastest) to 9 (best compression)

🔝 [back to top](#compression-package-api-reference)

&nbsp;

## No-Op Compressor

### No-Op Constructor

```go
compressor := compression.NewNop()
```

### No-Op Methods

| Method | Description |
|--------|-------------|
| `Algorithm()` | Returns "none" |
| `Threshold()` | Returns 0 (no threshold) |
| `Compress(data []byte)` | Returns data unchanged |
| `Decompress(data []byte)` | Returns data unchanged |

### Use Cases

- Testing and development environments
- Disabling compression without code changes
- Baseline performance comparisons

🔝 [back to top](#compression-package-api-reference)

&nbsp;

## Usage Examples

### Basic Gzip Compression

```go
import "github.com/cloudresty/go-rabbitmq/compression"

// Create gzip compressor with 1KB threshold and default level
compressor := compression.NewGzip(1024, compression.DefaultLevel)

// Use with publisher
publisher, err := client.NewPublisher(
    rabbitmq.WithCompression(compressor),
    rabbitmq.WithCompressionThreshold(1024),
)
```

### High-Performance Compression

```go
// Fast compression for high-throughput scenarios
fastCompressor := compression.NewGzip(512, compression.BestSpeed)

// Best compression for bandwidth-constrained environments
bestCompressor := compression.NewZlib(2048, compression.BestCompression)
```

### Testing with No-Op

```go
// Use no-op compressor for testing
testCompressor := compression.NewNop()

publisher, err := client.NewPublisher(
    rabbitmq.WithCompression(testCompressor),
)
```

🔝 [back to top](#compression-package-api-reference)

&nbsp;

## Error Handling

All compression operations return descriptive errors:

| Error Type | Description |
|------------|-------------|
| Compression Errors | Issues creating or writing compressed data |
| Decompression Errors | Invalid compressed data or decompression failures |
| Validation Errors | Data format validation failures |

🔝 [back to top](#compression-package-api-reference)

&nbsp;

## Performance Considerations

| Factor | Impact |
|--------|--------|
| **Threshold Size** | Messages below threshold are not compressed |
| **Compression Level** | Higher levels = better compression but slower speed |
| **Message Size** | Larger messages benefit more from compression |
| **Content Type** | Text/JSON compresses better than binary data |
| **CPU vs Bandwidth** | Trade CPU time for reduced network usage |

🔝 [back to top](#compression-package-api-reference)

&nbsp;

## Best Practices

1. **Choose Appropriate Threshold**: Set threshold based on your average message size
2. **Balance Speed vs Ratio**: Use `BestSpeed` for high-throughput, `BestCompression` for bandwidth savings
3. **Monitor Compression Efficiency**: Check that compressed messages are actually smaller
4. **Test with Real Data**: Compression effectiveness varies by content type
5. **Consider CPU Impact**: Monitor CPU usage in high-throughput scenarios

🔝 [back to top](#compression-package-api-reference)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
