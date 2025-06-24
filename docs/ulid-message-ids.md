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

### Automatic ULID Generation

```go
// Auto-generated ULID message ID
message := rabbitmq.NewMessage([]byte(`{"order_id": "12345"}`))
// message.MessageID will be a ULID like: 06bs864k6ss3s12tsqaknhy6y8
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

### Custom Message ID

```go
// Custom message ID (still ULID format recommended)
customUlid, _ := ulid.New()
message := rabbitmq.NewMessageWithID([]byte(`{"data": "value"}`), customUlid)
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

### Rich Message with ULID Correlation

```go
message := rabbitmq.NewMessage([]byte(`{"event": "payment"}`)).
    WithCorrelationID("06bs864k6vsf4z56brhryn2kyr"). // ULID correlation ID
    WithType("payment.processed").
    WithHeader("trace_id", "06bs864k6ss3s12tsqaknhy6y8")
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

### Publishing with ULID

```go
publisher, _ := rabbitmq.NewPublisher()

// Publish with auto-generated ULID
err := publisher.Publish(context.Background(), rabbitmq.PublishConfig{
    Exchange:    "events",
    RoutingKey:  "order.created",
    Message:     []byte(`{"order_id": "12345"}`),
    ContentType: "application/json",
    // MessageID is auto-generated as ULID if not provided
})

// Publish with custom ULID
customID, _ := ulid.New()
err = publisher.Publish(context.Background(), rabbitmq.PublishConfig{
    Exchange:    "events",
    RoutingKey:  "order.created",
    Message:     []byte(`{"order_id": "12345"}`),
    ContentType: "application/json",
    MessageID:   customID.String(),
})
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
WHERE ulid_id BETWEEN '06bs864k0000000000000000'
                  AND '06bs864kzzzzzzzzzzzzzz';
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
err := publisher.Publish(ctx, rabbitmq.PublishConfig{
    Exchange:      "requests",
    RoutingKey:    "payment.process",
    Message:       []byte(`{"amount": 100}`),
    CorrelationID: requestID.String(),
    ReplyTo:       "payment.responses",
})

// Response handler
consumer.Consume(ctx, rabbitmq.ConsumeConfig{
    Queue: "payment.responses",
    Handler: func(ctx context.Context, msg rabbitmq.Message) error {
        if msg.CorrelationID == requestID.String() {
            // Handle response for our request
        }
        return nil
    },
})
```

🔝 [back to top](#ulid-message-ids)

&nbsp;

### Distributed Tracing

```go
// Create trace ID
traceID, _ := ulid.New()

// Propagate through message chain
message := rabbitmq.NewMessage(payload).
    WithHeader("trace_id", traceID.String()).
    WithHeader("span_id", ulid.New().String()).
    WithHeader("parent_span_id", parentSpanID)

// Extract in consumer
consumer.Consume(ctx, rabbitmq.ConsumeConfig{
    Queue: "events",
    Handler: func(ctx context.Context, msg rabbitmq.Message) error {
        traceID := msg.Headers["trace_id"]
        spanID := msg.Headers["span_id"]
        // Use for distributed tracing
        return nil
    },
})
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

All message IDs are automatically generated as ULIDs unless explicitly overridden. See [`examples/ulid-messages/`](../examples/ulid-messages/) and [`examples/ulid-verification/`](../examples/ulid-verification/) for complete demonstrations.

🔝 [back to top](#ulid-message-ids)

&nbsp;

---

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
