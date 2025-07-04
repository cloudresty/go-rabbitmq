# Encryption Package

[Home](../README.md) &nbsp;/&nbsp; Encryption Package

&nbsp;

This package provides comprehensive message encryption capabilities for the `go-rabbitmq` library, including industry-standard AES-GCM encryption and a pluggable interface for custom encryption implementations.

&nbsp;

## Features

- **AES-256-GCM Encryption**: Industry-standard authenticated encryption
- **Automatic Nonce Generation**: Secure random nonce for each encryption operation
- **Authenticated Encryption**: Built-in integrity verification prevents tampering
- **Zero Configuration**: Secure defaults with minimal setup required
- **Performance Optimized**: Efficient implementations for high-throughput scenarios
- **Pluggable Interface**: Easy to implement custom encryption algorithms

🔝 [back to top](#encryption-package)

&nbsp;

## Installation

```bash
go get github.com/cloudresty/go-rabbitmq/encryption
```

🔝 [back to top](#encryption-package)

&nbsp;

## Basic Usage

### AES-GCM Encryption

```go
import (
    "crypto/rand"
    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/encryption"
)

func main() {
    // Generate a secure 256-bit encryption key
    key := make([]byte, 32)
    if _, err := rand.Read(key); err != nil {
        log.Fatal("Failed to generate encryption key:", err)
    }

    // Create AES-GCM encryptor
    encryptor, err := encryption.NewAESGCM(key)
    if err != nil {
        log.Fatal("Failed to create encryptor:", err)
    }

    // Create publisher with encryption
    publisher, err := rabbitmq.NewPublisher(
        rabbitmq.WithHosts("localhost:5672"),
        rabbitmq.WithCredentials("guest", "guest"),
        rabbitmq.WithEncryption(encryptor),
        rabbitmq.WithConnectionName("secure-publisher"),
    )
    if err != nil {
        log.Fatal("Failed to create publisher:", err)
    }
    defer publisher.Close()

    // Messages are automatically encrypted before publishing
    message := rabbitmq.NewMessage([]byte("sensitive data"))
    err = publisher.Publish(ctx, "secure.exchange", "routing.key", message)
    if err != nil {
        log.Fatal("Failed to publish encrypted message:", err)
    }
}
```

🔝 [back to top](#encryption-package)

&nbsp;

### Consumer with Automatic Decryption

```go
import (
    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/encryption"
)

func main() {
    // Use the same encryption key as the publisher
    encryptor, err := encryption.NewAESGCM(key)
    if err != nil {
        log.Fatal("Failed to create encryptor:", err)
    }

    // Create consumer with automatic decryption
    consumer, err := rabbitmq.NewConsumer(
        rabbitmq.WithHosts("localhost:5672"),
        rabbitmq.WithCredentials("guest", "guest"),
        rabbitmq.WithConsumerEncryption(encryptor),
        rabbitmq.WithConnectionName("secure-consumer"),
    )
    if err != nil {
        log.Fatal("Failed to create consumer:", err)
    }
    defer consumer.Close()

    // Consume messages (automatically decrypted)
    err = consumer.Consume(ctx, "secure.queue", func(delivery *rabbitmq.Delivery) error {
        // Message body is automatically decrypted before reaching this handler
        log.Printf("Received decrypted message: %s", string(delivery.Body))

        // Check encryption header
        if alg, ok := delivery.Headers["x-encryption"]; ok {
            log.Printf("Message was encrypted with: %v", alg)
        }

        return nil
    })
}
```

🔝 [back to top](#encryption-package)

&nbsp;

## Advanced Usage

### Custom Encryption Implementation

You can implement your own encryption algorithm by satisfying the `MessageEncryptor` interface:

```go
type CustomEncryptor struct {
    // Your custom encryption state
}

func (c *CustomEncryptor) Encrypt(data []byte) ([]byte, error) {
    // Your encryption logic
    return encryptedData, nil
}

func (c *CustomEncryptor) Decrypt(data []byte) ([]byte, error) {
    // Your decryption logic
    return decryptedData, nil
}

func (c *CustomEncryptor) Algorithm() string {
    return "custom-algorithm-v1"
}

// Use with publisher/consumer
publisher, err := rabbitmq.NewPublisher(
    rabbitmq.WithEncryption(&CustomEncryptor{}),
)
```

🔝 [back to top](#encryption-package)

&nbsp;

### Key Management Best Practices

```go
import (
    "crypto/rand"
    "encoding/base64"
    "os"
)

func generateSecureKey() []byte {
    key := make([]byte, 32) // 256-bit key
    if _, err := rand.Read(key); err != nil {
        panic("Failed to generate secure key")
    }
    return key
}

func keyFromEnvironment() ([]byte, error) {
    keyStr := os.Getenv("ENCRYPTION_KEY")
    if keyStr == "" {
        return nil, fmt.Errorf("ENCRYPTION_KEY environment variable not set")
    }

    return base64.StdEncoding.DecodeString(keyStr)
}

func main() {
    // Option 1: Generate new key (for development)
    key := generateSecureKey()

    // Option 2: Load from environment (for production)
    // key, err := keyFromEnvironment()

    encryptor, err := encryption.NewAESGCM(key)
    // ... rest of setup
}
```

🔝 [back to top](#encryption-package)

&nbsp;

### Combined with Compression

Encryption works seamlessly with other features like compression:

```go
import (
    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/go-rabbitmq/encryption"
    "github.com/cloudresty/go-rabbitmq/compression"
)

func main() {
    encryptor, _ := encryption.NewAESGCM(key)
    compressor := compression.NewGzip()

    // Publisher with both encryption and compression
    publisher, err := rabbitmq.NewPublisher(
        rabbitmq.WithEncryption(encryptor),    // Encrypt first
        rabbitmq.WithCompression(compressor),  // Then compress
        rabbitmq.WithCompressionThreshold(100), // Compress messages >100 bytes
    )

    // Consumer with automatic decompression and decryption
    consumer, err := rabbitmq.NewConsumer(
        rabbitmq.WithConsumerCompression(compressor), // Decompress first
        rabbitmq.WithConsumerEncryption(encryptor),   // Then decrypt
    )
}
```

🔝 [back to top](#encryption-package)

&nbsp;

## Security Considerations

### Key Management

- **Use 256-bit keys**: Always use 32-byte keys for AES-256-GCM
- **Secure key generation**: Use `crypto/rand` for key generation
- **Key rotation**: Implement regular key rotation policies
- **Environment variables**: Store keys in environment variables, not in code
- **Key derivation**: Consider using key derivation functions (KDF) for password-based keys

🔝 [back to top](#encryption-package)

&nbsp;

### Best Practices

```go
// ✅ Good: Secure key generation
key := make([]byte, 32)
if _, err := rand.Read(key); err != nil {
    log.Fatal("Failed to generate key:", err)
}

// ✅ Good: Load key from environment
keyStr := os.Getenv("ENCRYPTION_KEY")
key, err := base64.StdEncoding.DecodeString(keyStr)

// ❌ Bad: Hardcoded key
key := []byte("this-is-not-secure-32-byte-key!!") // Don't do this!

// ❌ Bad: Weak key generation
key := make([]byte, 32)
for i := range key {
    key[i] = byte(i) // Predictable pattern
}
```

🔝 [back to top](#encryption-package)

&nbsp;

### Compliance and Standards

- **FIPS 140-2**: AES-GCM is FIPS approved for sensitive data
- **Common Criteria**: EAL4+ certified implementations available
- **GDPR**: Encryption satisfies "appropriate technical measures" requirement
- **HIPAA**: Suitable for PHI encryption requirements
- **PCI DSS**: Approved for payment card data protection

🔝 [back to top](#encryption-package)

&nbsp;

## Performance Considerations

### Benchmarks

On typical hardware (Intel i7, 2.6GHz):

- **Encryption**: ~500 MB/s for 1KB messages
- **Decryption**: ~520 MB/s for 1KB messages
- **Memory overhead**: ~28 bytes per message (nonce + auth tag)

🔝 [back to top](#encryption-package)

&nbsp;

### Optimization Tips

```go
// For high-throughput scenarios
publisher, err := rabbitmq.NewPublisher(
    rabbitmq.WithEncryption(encryptor),
    rabbitmq.WithCompression(compressor),
    rabbitmq.WithCompressionThreshold(512), // Only compress larger messages
    rabbitmq.WithConnectionPoolSize(10), // Connection pooling
)
```

🔝 [back to top](#encryption-package)

&nbsp;

## Error Handling

```go
encryptor, err := encryption.NewAESGCM(key)
if err != nil {
    // Handle key validation errors
    log.Printf("Invalid encryption key: %v", err)
    return
}

err = publisher.Publish(ctx, exchange, routingKey, message)
if err != nil {
    // Encryption errors are wrapped in publish errors
    if strings.Contains(err.Error(), "encryption failed") {
        log.Printf("Message encryption failed: %v", err)
    }
}
```

🔝 [back to top](#encryption-package)

&nbsp;

## Dependencies

- **Go**: 1.19 or later
- **crypto/aes**: Standard library AES implementation
- **crypto/cipher**: Standard library GCM mode
- **crypto/rand**: Secure random number generation

🔝 [back to top](#encryption-package)

&nbsp;

## Troubleshooting

### Common Issues

**Invalid key length error**:

```text
AES-GCM requires 32-byte key, got 16 bytes
```

Solution: Use exactly 32 bytes for AES-256-GCM.

&nbsp;

**Decryption failed error**:

```text
decryption failed: cipher: message authentication failed
```

Solution: Ensure publisher and consumer use the same encryption key.

&nbsp;

**Ciphertext too short error**:

```text
ciphertext too short: expected at least 12 bytes, got 8
```

Solution: The message may be corrupted or not properly encrypted.

&nbsp;

### Debug Mode

```go
// Enable debug logging to troubleshoot encryption issues
publisher, err := rabbitmq.NewPublisher(
    rabbitmq.WithEncryption(encryptor),
    rabbitmq.WithLogger(logger), // Add structured logging
)
```

🔝 [back to top](#encryption-package)

&nbsp;

## Examples

Complete examples are available in the [examples/encryption-features](../examples/encryption-features) directory.

&nbsp;

---

&nbsp;

**Security Notice**: This implementation is suitable for production use but should be reviewed by security professionals for high-security applications. Consider additional measures like key rotation, secure key storage, and audit logging for enterprise deployments.

🔝 [back to top](#encryption-package)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
