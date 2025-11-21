# ULID Message IDs

[Home](../README.md) &nbsp;/&nbsp; [Docs](README.md) &nbsp;/&nbsp; ULID Message IDs

&nbsp;

This package uses [ULID (Universally Unique Lexicographically Sortable Identifier)](https://github.com/cloudresty/ulid) for all message IDs, providing significant advantages over traditional UUIDs.

&nbsp;

## Benefits

- **6x Faster Generation** - ~150ns per ULID vs ~800ns for UUID v4
- **Database Optimized** - Sequential inserts reduce B-tree fragmentation
- **Lexicographically Sortable** - Natural time-based ordering
- **Compact** - 26 characters vs UUID's 36 characters (28% smaller)
- **URL Safe** - No special characters, no encoding needed
- **Collision Resistant** - 1.21e+24 unique IDs per millisecond
- **Better Cache Performance** - Time-ordered data improves locality

🔝 [back to top](#ulid-message-ids)

&nbsp;

## ULID Format

```text
01ARZ3NDEKTSV4RRFFQ69G5FAV
|-----------|  |-------------|
  Timestamp      Randomness
   48bits         80bits
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

## Usage Examples

&nbsp;

### Complete Example

```go
package main

import (
    "context"
    "log"

    "github.com/cloudresty/go-rabbitmq"
    "github.com/cloudresty/ulid"
)

func main() {
    // Create client from environment variables
    client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create publisher
    publisher, err := client.NewPublisher()
    if err != nil {
        log.Fatal(err)
    }
    defer publisher.Close()

    // Message with auto-generated ULID
    message := rabbitmq.NewMessage([]byte(`{"event": "user.created"}`))
    log.Printf("Auto-generated ULID: %s", message.MessageID)

    // Publish message
    ctx := context.Background()
    err = publisher.Publish(ctx, "", "user.created", message)
    if err != nil {
        log.Fatal(err)
    }
}
```

&nbsp;

### Automatic ULID Generation

```go
// Auto-generated ULID message ID
message := rabbitmq.NewMessage([]byte(`{"order_id": "12345"}`))
// message.MessageID will be a ULID like: 06bwgbt1dk6d6xgk61wcbv6k9g

// Or create a JSON message from a struct
type Order struct {
    OrderID string `json:"order_id"`
}
order := Order{OrderID: "12345"}
message, err := rabbitmq.NewJSONMessage(order)
// message.MessageID will be auto-generated as ULID
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

### Custom Message ID

```go
// Custom message ID (ULID format recommended)
customUlid, _ := ulid.New()
message := rabbitmq.NewMessageWithID([]byte(`{"data": "value"}`), customUlid)

// Or with existing ULID string
message := rabbitmq.NewMessageWithID([]byte(`{"data": "value"}`), "06bwgbt1dk6d6xgk61wcbv6k9g")
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

### Rich Message with ULID Correlation

```go
message := rabbitmq.NewMessage([]byte(`{"event": "payment"}`)).
    WithCorrelationID("06bwgbt1dk6d6xgk61wcbv6k9g"). // ULID correlation ID
    WithType("payment.processed").
    WithHeader("trace_id", "06bwgbt1dk6d6xgk61wcbv6k9h")
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

### Publishing with ULID

```go
// Create client and publisher using new API
client, err := rabbitmq.NewClient(rabbitmq.FromEnv())
if err != nil {
    log.Fatal(err)
}
defer client.Close()

publisher, err := client.NewPublisher()
if err != nil {
    log.Fatal(err)
}
defer publisher.Close()

// Publish with auto-generated ULID
ctx := context.Background()
message := rabbitmq.NewMessage([]byte(`{"order_id": "12345"}`))
err = publisher.Publish(ctx, "", "order.created", message)
if err != nil {
    log.Fatal(err)
}

// Publish with custom ULID
customID, _ := ulid.New()
message = rabbitmq.NewMessageWithID([]byte(`{"order_id": "12345"}`), customID)
err = publisher.Publish(ctx, "", "order.created", message)
if err != nil {
    log.Fatal(err)
}

// Publish JSON with auto-generated ULID
type Order struct {
    OrderID string `json:"order_id"`
}
message, err = rabbitmq.NewJSONMessage(Order{OrderID: "12345"})
if err != nil {
    log.Fatal(err)
}
err = publisher.Publish(ctx, "", "order.created", message)
if err != nil {
    log.Fatal(err)
}
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

## Database Integration

ULIDs are particularly beneficial for database operations:

&nbsp;

### Time-Ordered Inserts

```sql
-- Messages naturally ordered by creation time
SELECT * FROM messages ORDER BY ulid_id;

-- Range queries are efficient
SELECT * FROM messages
WHERE ulid_id BETWEEN '06bwgbt1d00000000000000000'
                  AND '06bwgbt1dzzzzzzzzzzzzzzzz';
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

### Index Performance

```sql
-- Primary key on ULID provides excellent insert performance
CREATE TABLE messages (
    id VARCHAR(26) PRIMARY KEY,  -- ULID
    payload JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Natural clustering reduces index fragmentation
CREATE INDEX idx_messages_time_range ON messages (id, created_at);
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

## Correlation Patterns

&nbsp;

### Request-Response Correlation

```go
// Request with ULID correlation
requestID, _ := ulid.New()
message := rabbitmq.NewMessage([]byte(`{"amount": 100}`)).
    WithCorrelationID(requestID).
    WithReplyTo("payment.responses")

ctx := context.Background()
err := publisher.Publish(ctx, "", "payment.process", message)

// Response handler
consumer, err := client.NewConsumer()
if err != nil {
    log.Fatal(err)
}
defer consumer.Close()

handler := func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    if delivery.CorrelationId == requestID {
        // Handle response for our request
    }
    return delivery.Ack()
}

err = consumer.Consume(ctx, "payment.responses", handler)
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

### Distributed Tracing

```go
// Create trace ID
traceID, _ := ulid.New()
parentSpanID, _ := ulid.New() // Parent span for tracing hierarchy

// Propagate through message chain
payload := []byte(`{"event": "payment"}`)
message := rabbitmq.NewMessage(payload).
    WithHeader("trace_id", traceID).
    WithHeader("span_id", ulid.New()).
    WithHeader("parent_span_id", parentSpanID)

ctx := context.Background()
err := publisher.Publish(ctx, "", "payment.traced", message)

// Extract in consumer
handler := func(ctx context.Context, delivery *rabbitmq.Delivery) error {
    traceID := delivery.Headers["trace_id"]
    spanID := delivery.Headers["span_id"]
    // Use for distributed tracing
    return delivery.Ack()
}

err = consumer.Consume(ctx, "events", handler)
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

## Performance Comparison

| Operation | UUID v4 | ULID | Improvement |
|-----------|---------|------|-------------|
| Generation | ~800ns | ~150ns | 6x faster |
| String Length | 36 chars | 26 chars | 28% smaller |
| Database Insert | Random | Sequential | Better locality |
| Index Size | Larger | Smaller | Reduced fragmentation |
| Sort Performance | Random | Time-ordered | Natural ordering |

🔝 [back to top](#ulid-message-ids)

&nbsp;

## Migration from UUID

&nbsp;

### Code Migration

```go
// Before (UUID)
import "github.com/google/uuid"
messageID := uuid.New().String()

// After (ULID)
import "github.com/cloudresty/ulid"
messageID, _ := ulid.New()  // Returns ULID string directly

// Or use auto-generation in messages
message := rabbitmq.NewMessage([]byte(`{"data": "value"}`))
// message.MessageID is automatically a ULID
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

### Database Migration

```sql
-- Add new ULID column
ALTER TABLE messages ADD COLUMN ulid_id VARCHAR(26);

-- Populate with new ULIDs (for new records)
-- Keep UUID for existing records during transition period

-- Eventually drop UUID column
ALTER TABLE messages DROP COLUMN uuid_id;
```

All message IDs are automatically generated as ULIDs unless explicitly overridden.

🔝 [back to top](#ulid-message-ids)

&nbsp;

---

&nbsp;

### Cloudresty

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty) &nbsp;|&nbsp; [Docker Hub](https://hub.docker.com/u/cloudresty)

&nbsp;
