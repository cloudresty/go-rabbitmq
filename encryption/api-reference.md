# Encryption Package API Reference

[Home](../README.md) &nbsp;/&nbsp; [Encryption](README.md) &nbsp;/&nbsp; API Reference

&nbsp;

This document provides a complete API reference for the `encryption` package, which implements message encryption functionality for the go-rabbitmq library using industry-standard algorithms.

&nbsp;

## Constructor Functions

| Function | Description |
|----------|-------------|
| `NewAESGCM(key []byte) (*AESGCM, error)` | Creates a new AES-256-GCM encryptor with the provided 32-byte key |
| `NewNoEncryption()` | Creates a no-operation encryptor that doesn't encrypt data |

🔝 [back to top](#encryption-package-api-reference)

&nbsp;

## Interface Methods

All encryptor types implement the `rabbitmq.MessageEncryptor` interface:

| Method | Description |
|--------|-------------|
| `Encrypt(data []byte) ([]byte, error)` | Encrypts the provided data and returns encrypted bytes |
| `Decrypt(data []byte) ([]byte, error)` | Decrypts previously encrypted data |
| `Algorithm() string` | Returns the encryption algorithm identifier |

🔝 [back to top](#encryption-package-api-reference)

&nbsp;

## AES-GCM Encryptor

### Constructor

```go
encryptor, err := encryption.NewAESGCM(key)
```

### Methods

| Method | Description |
|--------|-------------|
| `Algorithm()` | Returns "AES-256-GCM" |
| `Encrypt(data []byte)` | Encrypts data using AES-256-GCM with random nonce |
| `Decrypt(data []byte)` | Decrypts AES-GCM data with nonce validation |

### Key Requirements

- **Key Size**: Must be exactly 32 bytes (256 bits)
- **Key Generation**: Use `crypto/rand` for secure key generation
- **Key Storage**: Store keys securely, preferably in environment variables

### Security Features

| Feature | Description |
|---------|-------------|
| **Authenticated Encryption** | Provides both confidentiality and integrity |
| **Automatic Nonce Generation** | Generates secure random nonce for each encryption |
| **Nonce Prepending** | Nonce is automatically prepended to ciphertext |
| **Tamper Detection** | Decryption fails if data has been modified |

🔝 [back to top](#encryption-package-api-reference)

&nbsp;

## No-Op Encryptor

### No-Op Constructor

```go
encryptor := encryption.NewNoEncryption()
```

### No-Op Methods

| Method | Description |
|--------|-------------|
| `Algorithm()` | Returns "none" |
| `Encrypt(data []byte)` | Returns data unchanged |
| `Decrypt(data []byte)` | Returns data unchanged |

### Use Cases

- Testing and development environments
- Disabling encryption without code changes
- Performance baseline measurements
- Debugging encrypted message flows

🔝 [back to top](#encryption-package-api-reference)

&nbsp;

## Usage Examples

### Secure Key Generation

```go
import (
    "crypto/rand"
    "github.com/cloudresty/go-rabbitmq/encryption"
)

// Generate a secure 256-bit key
key := make([]byte, 32)
if _, err := rand.Read(key); err != nil {
    log.Fatal("Failed to generate encryption key:", err)
}

// Create AES-GCM encryptor
encryptor, err := encryption.NewAESGCM(key)
if err != nil {
    log.Fatal("Failed to create encryptor:", err)
}
```

### Environment-Based Key Management

```go
import (
    "encoding/base64"
    "os"
)

// Load key from environment variable
keyStr := os.Getenv("ENCRYPTION_KEY")
if keyStr == "" {
    log.Fatal("ENCRYPTION_KEY environment variable not set")
}

key, err := base64.StdEncoding.DecodeString(keyStr)
if err != nil {
    log.Fatal("Invalid encryption key format:", err)
}

encryptor, err := encryption.NewAESGCM(key)
```

### Publisher with Encryption

```go
// Create publisher with encryption
publisher, err := client.NewPublisher(
    rabbitmq.WithEncryption(encryptor),
    rabbitmq.WithDefaultExchange("secure.events"),
)
if err != nil {
    log.Fatal("Failed to create publisher:", err)
}

// Messages are automatically encrypted before publishing
message := rabbitmq.NewMessage([]byte("sensitive data"))
err = publisher.Publish(ctx, "secure.events", "data.key", message)
```

### Consumer with Automatic Decryption

```go
// Create consumer with same encryptor for decryption
consumer, err := client.NewConsumer(
    rabbitmq.WithConsumerEncryption(encryptor),
)
if err != nil {
    log.Fatal("Failed to create consumer:", err)
}

// Messages are automatically decrypted before handler
err = consumer.Consume(ctx, "secure.queue", func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    // delivery.Body contains decrypted data
    log.Printf("Received decrypted message: %s", delivery.Body)
    return delivery.Ack()
})
```

🔝 [back to top](#encryption-package-api-reference)

&nbsp;

## Error Handling

| Error Type | Description | Cause |
|------------|-------------|-------|
| `Key Size Error` | Invalid key length provided | Key must be exactly 32 bytes |
| `Cipher Creation Error` | Failed to create AES cipher | Invalid key or system error |
| `GCM Creation Error` | Failed to create GCM mode | Cipher initialization failure |
| `Nonce Generation Error` | Failed to generate random nonce | Insufficient entropy or system error |
| `Ciphertext Too Short` | Invalid encrypted data format | Corrupted or non-encrypted data |
| `Decryption Failed` | Authentication or decryption failure | Wrong key, tampered data, or corruption |

🔝 [back to top](#encryption-package-api-reference)

&nbsp;

## Security Considerations

### Key Management

| Best Practice | Description |
|---------------|-------------|
| **Secure Generation** | Use `crypto/rand` for key generation |
| **Proper Storage** | Store keys in environment variables or secure vaults |
| **Key Rotation** | Implement regular key rotation policies |
| **Access Control** | Limit access to encryption keys |
| **No Hardcoding** | Never hardcode keys in source code |

### Operational Security

| Practice | Description |
|----------|-------------|
| **Transport Security** | Use TLS for additional transport encryption |
| **Message Headers** | Encryption algorithm is automatically added to headers |
| **Error Handling** | Don't expose sensitive information in error messages |
| **Logging** | Never log encryption keys or decrypted content |
| **Testing** | Use no-op encryptor for non-production testing |

🔝 [back to top](#encryption-package-api-reference)

&nbsp;

## Performance Considerations

| Factor | Impact |
|--------|--------|
| **Algorithm Efficiency** | AES-GCM is hardware-accelerated on modern CPUs |
| **Message Size** | Encryption overhead is constant (~28 bytes) |
| **CPU Usage** | Minimal impact with hardware acceleration |
| **Memory Usage** | Low memory footprint for encryption operations |
| **Nonce Generation** | Uses secure random number generation |

### Benchmarks

Typical performance on modern hardware (Intel i7, 2.6GHz):

| Operation | Throughput | Overhead |
|-----------|------------|----------|
| **Encryption** | ~500 MB/s | ~28 bytes per message |
| **Decryption** | ~520 MB/s | Validation included |
| **Key Creation** | ~instantaneous | One-time setup cost |

🔝 [back to top](#encryption-package-api-reference)

&nbsp;

## Compliance and Standards

| Standard | Description |
|----------|-------------|
| **FIPS 140-2** | AES-GCM is FIPS approved for sensitive data |
| **Common Criteria** | EAL4+ certified implementations available |
| **NIST SP 800-38D** | AES-GCM specification compliance |
| **RFC 5116** | Authenticated Encryption specification |

### Regulatory Compliance

| Regulation | Applicability |
|------------|---------------|
| **GDPR** | Satisfies "appropriate technical measures" requirement |
| **HIPAA** | Suitable for PHI (Protected Health Information) encryption |
| **PCI DSS** | Approved for payment card data protection |
| **SOX** | Meets financial data protection requirements |

🔝 [back to top](#encryption-package-api-reference)

&nbsp;

## Best Practices

1. **Key Management**: Use environment variables or secure key management systems
2. **Key Rotation**: Implement regular key rotation (e.g., every 90 days)
3. **Error Handling**: Handle encryption errors gracefully without exposing sensitive data
4. **Testing**: Use `NewNoEncryption()` for development and testing
5. **Monitoring**: Monitor encryption/decryption performance and error rates
6. **Documentation**: Document your key management and rotation procedures
7. **Backup**: Ensure encrypted messages can be decrypted after key rotation

🔝 [back to top](#encryption-package-api-reference)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
