# Security Policy

[Home](README.md) &nbsp;/&nbsp; Security Policy

&nbsp;

We take the security of go-rabbitmq seriously. This document outlines our security practices and how to report security vulnerabilities.

&nbsp;

## Table of Contents

- [Supported Versions](#supported-versions)
- [Reporting a Vulnerability](#reporting-a-vulnerability)
- [Security Best Practices](#security-best-practices)
- [Known Security Considerations](#known-security-considerations)
- [Security Updates](#security-updates)
- [Dependencies](#dependencies)

🔝 [back to top](#security-policy)

&nbsp;

## Supported Versions

We provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| Latest  | ✅ Yes            |
| Previous Major | ✅ Yes (6 months) |
| Older Versions | ❌ No            |

🔝 [back to top](#security-policy)

&nbsp;

**Current Support:**

- **Latest Release**: Always supported with security updates
- **Previous Major Version**: Supported for 6 months after new major release
- **Development Branches**: Not supported for security updates

🔝 [back to top](#security-policy)

&nbsp;

## Reporting a Vulnerability

&nbsp;

### Please DO NOT report security vulnerabilities through public GitHub issues

&nbsp;

### How to Report

1. **Email**: Send details to [security@cloudresty.com](mailto:security@cloudresty.com)
2. **Subject**: Include "SECURITY" and brief description
3. **Encryption**: Use our PGP key for sensitive information (see below)

🔝 [back to top](#security-policy)

&nbsp;

### What to Include

- **Description**: Clear description of the vulnerability
- **Impact**: Potential impact and attack scenarios
- **Reproduction**: Step-by-step instructions to reproduce
- **Affected Versions**: Which versions are affected
- **Environment**: Go version, OS, RabbitMQ version
- **Proof of Concept**: Code example (if applicable)

🔝 [back to top](#security-policy)

&nbsp;

### Response Timeline

- **Initial Response**: Within 48 hours
- **Vulnerability Assessment**: Within 5 business days
- **Fix Development**: Timeline depends on severity
- **Public Disclosure**: After fix is available and users have time to update

🔝 [back to top](#security-policy)

&nbsp;

### PGP Key for Encryption

```text
-----BEGIN PGP PUBLIC KEY BLOCK-----
[PGP key would be here in a real security policy]
-----END PGP PUBLIC KEY BLOCK-----
```

Download: [security@cloudresty.com.asc](mailto:security@cloudresty.com)

🔝 [back to top](#security-policy)

&nbsp;

## Security Best Practices

&nbsp;

### Environment Configuration

&nbsp;

#### Secure Connection URLs

```bash
# ✅ Good: Use secure protocols
export RABBITMQ_PROTOCOL=amqps
export RABBITMQ_TLS_ENABLED=true

# ✅ Good: Strong authentication
export RABBITMQ_USERNAME=app_user
export RABBITMQ_PASSWORD=strong_random_password

# ❌ Avoid: Default credentials
export RABBITMQ_USERNAME=guest
export RABBITMQ_PASSWORD=guest
```

🔝 [back to top](#security-policy)

&nbsp;

#### TLS Configuration

```go
// Enable TLS with proper verification
// Load environment configuration using the new API
client, err := rabbitmq.NewClient(
    rabbitmq.FromEnv(),
    rabbitmq.WithConnectionName("secure-app"),
)
envConfig.TLSEnabled = true
envConfig.TLSInsecure = false // Always verify certificates in production

// Use custom TLS configuration
config := rabbitmq.ConnectionConfig{
    URL: "amqps://rabbitmq.example.com:5671",
    TLSConfig: &tls.Config{
        ServerName:         "rabbitmq.example.com",
        InsecureSkipVerify: false, // Never true in production
        MinVersion:         tls.VersionTLS12,
    },
}
```

🔝 [back to top](#security-policy)

&nbsp;

### Access Control

&nbsp;

#### Connection Permissions

- Use dedicated service accounts for applications
- Follow principle of least privilege
- Regularly rotate credentials
- Use strong, unique passwords

🔝 [back to top](#security-policy)

&nbsp;

#### RabbitMQ User Configuration

```bash
# Create dedicated user for your application
rabbitmqctl add_user myapp_user secure_password
rabbitmqctl set_permissions -p /myapp myapp_user ".*" ".*" ".*"

# Remove default guest user in production
rabbitmqctl delete_user guest
```

🔝 [back to top](#security-policy)

&nbsp;

### Network Security

&nbsp;

#### Firewall Configuration

```bash
# Allow only necessary ports
# AMQP: 5672 (plain) or 5671 (TLS)
# Management: 15672 (HTTP) or 15671 (HTTPS)
# Only from trusted networks
```

🔝 [back to top](#security-policy)

&nbsp;

#### VPC/Network Isolation

- Deploy RabbitMQ in private subnets
- Use VPC endpoints where available
- Implement network segmentation
- Monitor network traffic

🔝 [back to top](#security-policy)

&nbsp;

### Message Security

&nbsp;

#### Sensitive Data Handling

```go
// ❌ Avoid: Putting sensitive data in message headers
headers := map[string]interface{}{
    "credit_card": "4111-1111-1111-1111", // Never do this
}

// ✅ Good: Encrypt sensitive data before publishing
encryptedData, err := encrypt(sensitiveData)
message := rabbitmq.NewMessage(encryptedData).
    WithHeader("encrypted", true).
    WithHeader("algorithm", "AES-256-GCM")
```

🔝 [back to top](#security-policy)

&nbsp;

#### Message Encryption

```go
// Example: Encrypt message body before publishing
func publishSecureMessage(publisher *rabbitmq.Publisher, data []byte) error {
    // Encrypt the message body
    encryptedData, err := encryptAES256GCM(data, encryptionKey)
    if err != nil {
        return err
    }

    // Publish encrypted message
    return publisher.Publish(ctx, rabbitmq.PublishConfig{
        Exchange:   "secure-events",
        RoutingKey: "encrypted",
        Message:    encryptedData,
        Headers: map[string]interface{}{
            "encrypted": true,
            "algorithm": "AES-256-GCM",
        },
    })
}
```

🔝 [back to top](#security-policy)

&nbsp;

### Logging Security

&nbsp;

#### Automatic PII Protection

The package automatically sanitizes sensitive information in logs:

```go
// Connection URLs are automatically sanitized
emit.Info.StructuredFields("Connecting to RabbitMQ",
    emit.ZString("url", sanitizeURL(connectionURL))) // Passwords removed
```

🔝 [back to top](#security-policy)

&nbsp;

#### Custom Log Sanitization

```go
// Sanitize custom sensitive data
func logMessage(messageID string, userID string) {
    emit.Info.StructuredFields("Processing message",
        emit.ZString("message_id", messageID),
        emit.ZString("user_id", sanitizeUserID(userID))) // Custom sanitization
}

func sanitizeUserID(userID string) string {
    if len(userID) <= 4 {
        return "***"
    }
    return userID[:4] + "***"
}
```

🔝 [back to top](#security-policy)

&nbsp;

## Known Security Considerations

&nbsp;

### Message Durability vs Performance

- **Persistent messages** are safer but slower
- **Non-persistent messages** are faster but can be lost
- Choose based on your security/performance requirements

```go
// High security: Use persistent messages
config := rabbitmq.PublishConfig{
    Exchange:   "critical-events",
    Message:    data,
    Persistent: true, // Messages survive broker restarts
}

// High performance: Use non-persistent for non-critical data
config := rabbitmq.PublishConfig{
    Exchange:   "metrics",
    Message:    data,
    Persistent: false, // Faster but can be lost
}
```

🔝 [back to top](#security-policy)

&nbsp;

### Connection Security

&nbsp;

#### Connection Limits

```go
// Configure connection limits to prevent resource exhaustion
config := rabbitmq.ConnectionConfig{
    URL:        "amqp://localhost:5672",
    ChannelMax: 100,   // Limit channels per connection
    FrameSize:  131072, // Limit frame size
    Heartbeat:  time.Second * 10, // Detect connection issues
}
```

🔝 [back to top](#security-policy)

&nbsp;

### Dead Letter Queue Security

- Dead letter queues may contain sensitive failed messages
- Implement appropriate access controls for DLQs
- Consider data retention policies for failed messages
- Monitor DLQ contents for security issues

🔝 [back to top](#security-policy)

&nbsp;

### ULID Security

ULIDs are designed to be safe for use as public identifiers:

- **No sensitive information**: ULIDs don't contain user data
- **Collision resistant**: Cryptographically secure randomness
- **Time-based**: Only timestamp is extractable (no other metadata)
- **URL safe**: No encoding issues or injection risks

🔝 [back to top](#security-policy)

&nbsp;

## Security Updates

&nbsp;

### Update Process

1. **Assessment**: We evaluate reported vulnerabilities
2. **Classification**: Severity assessment using CVSS
3. **Development**: Fix development with security review
4. **Testing**: Comprehensive security testing
5. **Release**: Coordinated disclosure and patch release
6. **Notification**: Security advisory and user notification

🔝 [back to top](#security-policy)

&nbsp;

### Severity Levels

| Severity | Description | Response Time |
|----------|-------------|---------------|
| **Critical** | Immediate exploitation possible | 24-48 hours |
| **High** | Significant security risk | 1 week |
| **Medium** | Moderate security impact | 2 weeks |
| **Low** | Minor security consideration | Next release |

🔝 [back to top](#security-policy)

&nbsp;

### Security Advisories

Security updates are published through:

- **GitHub Security Advisories**: Primary notification channel
- **Release Notes**: Detailed in version release notes
- **Mailing List**: [security-announce@cloudresty.com](mailto:security-announce@cloudresty.com)
- **Documentation**: Updated security documentation

🔝 [back to top](#security-policy)

&nbsp;

## Dependencies

&nbsp;

### Dependency Security

We regularly audit and update dependencies:

- **Automated Scanning**: Dependabot and security scanners
- **Regular Updates**: Dependencies updated in minor releases
- **Vulnerability Monitoring**: Continuous monitoring for known CVEs
- **Minimal Dependencies**: We keep dependencies minimal to reduce attack surface

🔝 [back to top](#security-policy)

&nbsp;

### Current Dependencies

Main dependencies (as of latest version):

```text
github.com/rabbitmq/amqp091-go  # RabbitMQ client library
github.com/cloudresty/emit      # Structured logging
github.com/cloudresty/ulid      # ULID generation
```

🔝 [back to top](#security-policy)

&nbsp;

### Dependency Policy

- **Security Patches**: Applied as soon as available
- **Major Updates**: Evaluated for security improvements
- **New Dependencies**: Security review required
- **Deprecated Dependencies**: Replaced promptly

🔝 [back to top](#security-policy)

&nbsp;

## Compliance and Auditing

&nbsp;

### Security Standards

This package is designed to support:

- **SOC 2**: Audit logging and access controls
- **GDPR**: Data encryption and sanitization features
- **HIPAA**: Encryption and audit trail capabilities
- **PCI DSS**: Secure communication and data handling

🔝 [back to top](#security-policy)

&nbsp;

### Audit Trail

The package provides comprehensive audit capabilities:

```go
// All operations are automatically logged with structured data
// Including: connection attempts, message publishing, consumption
// Logs include: timestamps, user context, operation details

// Example audit log entry:
{"timestamp": "2024-01-01T12:00:00Z", "level": "info", "message": "Message published", "exchange": "orders", "routing_key": "order.created", "message_id": "01HQRS..."}
```

🔝 [back to top](#security-policy)

&nbsp;

## Contact Information

&nbsp;

### Security Team

- **Email**: [security@cloudresty.com](mailto:security@cloudresty.com)
- **Response Time**: 48 hours for initial response
- **Office Hours**: Monday-Friday, 9 AM - 5 PM UTC

🔝 [back to top](#security-policy)

&nbsp;

### For Security Researchers

We welcome responsible disclosure from security researchers:

- **Bug Bounty**: Contact us about our bug bounty program
- **Hall of Fame**: Public recognition for security contributors
- **Coordination**: We work with researchers on responsible disclosure

🔝 [back to top](#security-policy)

&nbsp;

---

## Acknowledgments

We thank the security community and all researchers who have contributed to making go-rabbitmq more secure.

🔝 [back to top](#security-policy)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
