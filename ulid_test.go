package rabbitmq

import (
	"testing"
	"time"

	"github.com/cloudresty/ulid"
)

// TestULIDMessageGeneration tests that message IDs are generated using ULID format
func TestULIDMessageGeneration(t *testing.T) {

	// Test NewMessage generates valid ULID
	msg := NewMessage([]byte("test message"))

	if msg.MessageID == "" {
		t.Fatal("Expected message ID to be generated, but got empty string")
	}

	// Verify it's a valid ULID format (26 characters)
	if len(msg.MessageID) != 26 {
		t.Errorf("Expected ULID length of 26 characters, got %d", len(msg.MessageID))
	}

	// Verify it can be parsed as a valid ULID
	parsedUlid, err := ulid.Parse(msg.MessageID)
	if err != nil {
		t.Fatalf("Generated message ID is not a valid ULID: %v", err)
	}

	// Verify the timestamp is recent (within last 10 seconds)
	ulidTimeMs := parsedUlid.GetTime()
	// Safely convert uint64 milliseconds to time.Time
	if ulidTimeMs > uint64(1<<63-1) { // Check for overflow
		t.Fatalf("ULID timestamp too large to convert safely: %d", ulidTimeMs)
	}
	ulidTime := time.UnixMilli(int64(ulidTimeMs)) // Convert milliseconds to time.Time
	now := time.Now()
	if now.Sub(ulidTime) > 10*time.Second {
		t.Errorf("ULID timestamp is too old: %v, expected recent time around %v", ulidTime, now)
	}

	t.Logf("Generated ULID: %s (timestamp: %v)", msg.MessageID, ulidTime)

}

// TestULIDUniqueness tests that multiple ULIDs are unique
func TestULIDUniqueness(t *testing.T) {

	messageIDs := make(map[string]bool)

	// Generate 100 messages and verify all ULIDs are unique
	for range 100 {
		msg := NewMessage([]byte("test message"))

		if messageIDs[msg.MessageID] {
			t.Fatalf("Duplicate ULID generated: %s", msg.MessageID)
		}

		// Store the ULID
		messageIDs[msg.MessageID] = true

		// Also verify it's a valid ULID
		_, err := ulid.Parse(msg.MessageID)
		if err != nil {
			t.Fatalf("Invalid ULID generated: %s, error: %v", msg.MessageID, err)
		}
	}

	t.Logf("Successfully generated %d unique ULIDs", len(messageIDs))

}

// TestULIDTemporalOrdering tests that ULIDs generated in sequence are temporally ordered
func TestULIDTemporalOrdering(t *testing.T) {

	const numMessages = 10
	messages := make([]*Message, numMessages)

	// Generate messages with small delays to ensure temporal ordering
	for i := range numMessages {
		messages[i] = NewMessage([]byte("test message"))
		time.Sleep(1 * time.Millisecond) // Small delay to ensure different timestamps

		// Parse the ULID to get the timestamp
		parsedUlid, err := ulid.Parse(messages[i].MessageID)
		if err != nil {
			t.Fatalf("Invalid ULID generated: %s, error: %v", messages[i].MessageID, err)
		}

		ulidTimeMs := parsedUlid.GetTime()
		if ulidTimeMs > uint64(1<<63-1) { // Check for overflow
			t.Fatalf("ULID timestamp too large to convert safely: %d", ulidTimeMs)
		}
		currentTime := time.UnixMilli(int64(ulidTimeMs))
		t.Logf("Message %d: ULID=%s, Time=%v", i+1, messages[i].MessageID, currentTime)
	}

	// Verify temporal ordering - each ULID should be >= the previous one
	for i := 1; i < numMessages; i++ {
		if messages[i].MessageID < messages[i-1].MessageID {
			t.Errorf("ULIDs are not temporally ordered: %s < %s",
				messages[i].MessageID, messages[i-1].MessageID)
		}
	}

}

// TestCustomMessageID tests NewMessageWithID function
func TestCustomMessageID(t *testing.T) {

	customID := "01ARZ3NDEKTSV4RRFFQ69G5FAV" // Valid ULID
	msg := NewMessageWithID([]byte("test message"), customID)

	if msg.MessageID != customID {
		t.Errorf("Expected custom message ID %s, got %s", customID, msg.MessageID)
	}

}

// TestMessageIDOverride tests that ToPublishing preserves existing message ID
func TestMessageIDOverride(t *testing.T) {

	msg := NewMessage([]byte("test message"))
	originalID := msg.MessageID

	// Convert to AMQP publishing
	publishing := msg.ToPublishing()

	// Verify the message ID is preserved
	if publishing.MessageId != originalID {
		t.Errorf("Expected message ID to be preserved: %s, got %s", originalID, publishing.MessageId)
	}

}

// TestMessageIDAutoGeneration tests that ToPublishing generates ID when empty
func TestMessageIDAutoGeneration(t *testing.T) {

	msg := &Message{
		Body: []byte("test message"),
	}

	// Convert to AMQP publishing - should auto-generate ID
	publishing := msg.ToPublishing()

	// Verify an ID was generated
	if publishing.MessageId == "" {
		t.Error("Expected message ID to be auto-generated, but got empty string")
	}

	// Verify it's a valid ULID
	_, err := ulid.Parse(publishing.MessageId)
	if err != nil {
		t.Errorf("Auto-generated message ID is not a valid ULID: %v", err)
	}

}
