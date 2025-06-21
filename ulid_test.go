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

	if msg.MessageId == "" {
		t.Fatal("Expected message ID to be generated, but got empty string")
	}

	// Verify it's a valid ULID format (26 characters)
	if len(msg.MessageId) != 26 {
		t.Errorf("Expected ULID length of 26 characters, got %d", len(msg.MessageId))
	}

	// Verify it can be parsed as a valid ULID
	parsedUlid, err := ulid.Parse(msg.MessageId)
	if err != nil {
		t.Fatalf("Generated message ID is not a valid ULID: %v", err)
	}

	// Verify the timestamp is recent (within last 10 seconds)
	ulidTimeMs := parsedUlid.GetTime()
	ulidTime := time.Unix(int64(ulidTimeMs/1000), int64(ulidTimeMs%1000)*1000000) // Convert milliseconds to time.Time
	now := time.Now()
	if now.Sub(ulidTime) > 10*time.Second {
		t.Errorf("ULID timestamp is too old: %v, expected recent time around %v", ulidTime, now)
	}

	t.Logf("Generated ULID: %s (timestamp: %v)", msg.MessageId, ulidTime)
}

// TestULIDUniqueness tests that generated ULIDs are unique
func TestULIDUniqueness(t *testing.T) {
	const numMessages = 1000
	messageIds := make(map[string]bool)

	for i := 0; i < numMessages; i++ {
		msg := NewMessage([]byte("test message"))

		if messageIds[msg.MessageId] {
			t.Fatalf("Duplicate ULID generated: %s", msg.MessageId)
		}

		messageIds[msg.MessageId] = true

		// Verify it's a valid ULID
		_, err := ulid.Parse(msg.MessageId)
		if err != nil {
			t.Fatalf("Invalid ULID generated: %s, error: %v", msg.MessageId, err)
		}
	}

	t.Logf("Successfully generated %d unique ULIDs", numMessages)
}

// TestULIDSortability tests that ULIDs are time-sortable
func TestULIDSortability(t *testing.T) {
	var messages []*Message
	var previousTimeMs uint64

	// Generate messages with small delays to ensure different timestamps
	for i := 0; i < 10; i++ {
		if i > 0 {
			time.Sleep(1 * time.Millisecond) // Small delay to ensure different timestamps
		}

		msg := NewMessage([]byte("test message"))
		messages = append(messages, msg)

		// Parse the ULID to get its timestamp
		parsedUlid, err := ulid.Parse(msg.MessageId)
		if err != nil {
			t.Fatalf("Invalid ULID: %v", err)
		}

		currentTimeMs := parsedUlid.GetTime()
		currentTime := time.Unix(int64(currentTimeMs/1000), int64(currentTimeMs%1000)*1000000)

		// Verify timestamps are in ascending order
		if i > 0 && currentTimeMs < previousTimeMs {
			t.Errorf("ULID timestamps are not in order: current %d, previous %d", currentTimeMs, previousTimeMs)
		}

		previousTimeMs = currentTimeMs
		t.Logf("Message %d: ULID=%s, Time=%v", i+1, msg.MessageId, currentTime)
	}

	// Verify that the ULIDs are lexicographically sortable
	for i := 1; i < len(messages); i++ {
		if messages[i].MessageId < messages[i-1].MessageId {
			t.Errorf("ULIDs are not lexicographically sorted: %s should be >= %s",
				messages[i].MessageId, messages[i-1].MessageId)
		}
	}
}

// TestMessageWithCustomId tests that custom IDs are preserved
func TestMessageWithCustomId(t *testing.T) {
	customId := "custom-message-id-123"
	msg := NewMessageWithId([]byte("test message"), customId)

	if msg.MessageId != customId {
		t.Errorf("Expected custom ID %s, got %s", customId, msg.MessageId)
	}
}

// TestToPublishingWithULID tests that Message.ToPublishing preserves ULID
func TestToPublishingWithULID(t *testing.T) {
	msg := NewMessage([]byte("test message"))
	originalId := msg.MessageId

	publishing := msg.ToPublishing()

	if publishing.MessageId != originalId {
		t.Errorf("Expected publishing message ID %s, got %s", originalId, publishing.MessageId)
	}

	// Verify it's still a valid ULID
	_, err := ulid.Parse(publishing.MessageId)
	if err != nil {
		t.Fatalf("Publishing message ID is not a valid ULID: %v", err)
	}
}
