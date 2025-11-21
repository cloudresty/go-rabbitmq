package rabbitmq

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TestGetRetryCount tests the retry count extraction logic
func TestGetRetryCount(t *testing.T) {
	consumer := &Consumer{}

	tests := []struct {
		name     string
		headers  amqp.Table
		expected int
	}{
		{
			name:     "no headers",
			headers:  nil,
			expected: 0,
		},
		{
			name:     "empty headers",
			headers:  amqp.Table{},
			expected: 0,
		},
		{
			name:     "quorum queue - x-delivery-count int",
			headers:  amqp.Table{"x-delivery-count": 3},
			expected: 2, // x-delivery-count starts at 1, so 3 means 2 retries
		},
		{
			name:     "quorum queue - x-delivery-count int32",
			headers:  amqp.Table{"x-delivery-count": int32(5)},
			expected: 4,
		},
		{
			name:     "quorum queue - x-delivery-count int64",
			headers:  amqp.Table{"x-delivery-count": int64(2)},
			expected: 1,
		},
		{
			name:     "classic queue - x-retry-count int",
			headers:  amqp.Table{"x-retry-count": 3},
			expected: 3,
		},
		{
			name:     "classic queue - x-retry-count int32",
			headers:  amqp.Table{"x-retry-count": int32(2)},
			expected: 2,
		},
		{
			name:     "classic queue - x-retry-count int64",
			headers:  amqp.Table{"x-retry-count": int64(1)},
			expected: 1,
		},
		{
			name: "quorum queue takes precedence",
			headers: amqp.Table{
				"x-delivery-count": 5,
				"x-retry-count":    10,
			},
			expected: 4, // x-delivery-count wins
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delivery := &amqp.Delivery{
				Headers: tt.headers,
			}

			got := consumer.getRetryCount(delivery)
			if got != tt.expected {
				t.Errorf("getRetryCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestIsQuorumQueue tests the quorum queue detection logic
func TestIsQuorumQueue(t *testing.T) {
	consumer := &Consumer{}

	tests := []struct {
		name     string
		headers  amqp.Table
		expected bool
	}{
		{
			name:     "no headers",
			headers:  nil,
			expected: false,
		},
		{
			name:     "empty headers",
			headers:  amqp.Table{},
			expected: false,
		},
		{
			name:     "has x-delivery-count",
			headers:  amqp.Table{"x-delivery-count": 1},
			expected: true,
		},
		{
			name:     "only x-retry-count",
			headers:  amqp.Table{"x-retry-count": 1},
			expected: false,
		},
		{
			name: "has both headers",
			headers: amqp.Table{
				"x-delivery-count": 1,
				"x-retry-count":    1,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delivery := &amqp.Delivery{
				Headers: tt.headers,
			}

			got := consumer.isQuorumQueue(delivery)
			if got != tt.expected {
				t.Errorf("isQuorumQueue() = %v, want %v", got, tt.expected)
			}
		})
	}
}
