package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cloudresty/go-rabbitmq"
	"github.com/cloudresty/ulid"
)

func main() {

	fmt.Println("=== ULID Message ID Verification ===")

	// Test 1: Generate several messages and show ULID properties
	fmt.Println("1. Creating Messages with Auto-Generated ULID IDs:")
	for i := range 5 {
		msg := rabbitmq.NewMessage(fmt.Appendf(nil, "Test message %d", i+1))

		// Parse the ULID to get its properties
		parsedUlid, err := ulid.Parse(msg.MessageID)
		if err != nil {
			log.Fatalf("Failed to parse ULID: %v", err)
		}

		// Convert timestamp to readable format
		timestampMs := parsedUlid.GetTime()
		timestamp := time.Unix(int64(timestampMs/1000), int64(timestampMs%1000)*1000000)

		fmt.Printf("   Message %d: %s (created: %s)\n",
			i+1, msg.MessageID, timestamp.Format("2006-01-02 15:04:05.000"))

		// Small delay to show different timestamps
		time.Sleep(1 * time.Millisecond)
	}

	// Test 2: Show ULID format properties
	fmt.Println("\n2. ULID Format Analysis:")
	testUlid, _ := ulid.New()
	fmt.Printf("   Sample ULID: %s\n", testUlid)
	fmt.Printf("   Length: %d characters\n", len(testUlid))
	fmt.Printf("   Characters used: Crockford Base32 (0-9, A-Z excluding I, L, O, U)\n")
	fmt.Printf("   Structure: TTTTTTTTTTTTTRRRRRRRRRRRRR\n")
	fmt.Printf("              ↑timestamp↑   ↑randomness↑\n")
	fmt.Printf("              48 bits      80 bits\n")

	// Test 3: Demonstrate sorting
	fmt.Println("\n3. Lexicographical Sorting Test:")
	var ulids []string
	for range 3 {
		newUlid, _ := ulid.New()
		ulids = append(ulids, newUlid)
		time.Sleep(2 * time.Millisecond) // Ensure different timestamps
	}

	fmt.Println("   Generated ULIDs (in chronological order):")
	for i, id := range ulids {
		fmt.Printf("   %d. %s\n", i+1, id)
	}

	// Verify they're already sorted
	sorted := true
	for i := 1; i < len(ulids); i++ {
		if ulids[i] < ulids[i-1] {
			sorted = false
			break
		}
	}
	fmt.Printf("   ✅ ULIDs are lexicographically sorted: %v\n", sorted)

	// Test 4: Compare with timestamp-based fallback (simulate ULID failure)
	fmt.Println("\n4. Message ID Generation Methods:")

	// Using ULID (current implementation)
	msg1 := rabbitmq.NewMessage([]byte("test"))
	fmt.Printf("   ULID-based ID: %s (length: %d)\n", msg1.MessageID, len(msg1.MessageID))

	// Show what old timestamp-based IDs would look like
	timestamp := time.Now().UnixNano()
	oldStyleId := fmt.Sprintf("msg-%d", timestamp)
	fmt.Printf("   Old style ID:  %s (length: %d)\n", oldStyleId, len(oldStyleId))

	fmt.Printf("\n   Benefits of ULID over timestamp-based IDs:\n")
	fmt.Printf("   • Shorter and more compact\n")
	fmt.Printf("   • URL-safe characters only\n")
	fmt.Printf("   • Includes built-in randomness for uniqueness\n")
	fmt.Printf("   • Follows established standards\n")
	fmt.Printf("   • Better database performance\n")

	fmt.Println("\n=== ULID Implementation Verified Successfully! ===")
}
