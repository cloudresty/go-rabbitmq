package rabbitmq

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func refusedConsumer(code int) ConsumerHealth {
	h := healthyConsumer()
	h.Consuming, h.ChannelOpen, h.NotConsumingSince = false, false, ago(10*time.Minute)
	h.ResubscribeFailures, h.LastSubscribeErrorCode = 40, code
	return h
}

func TestConsumerAssessSubscribeRefusalIsNeverStalled(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		want       HealthLevel
		reasonPart string
	}{
		{"404 queue missing for 10m", 404, HealthDegraded, "queue not found"},
		{"403 access refused for 10m", 403, HealthDegraded, "access refused"},
		{"406 precondition failed for 10m", 406, HealthDegraded, "precondition failed"},
		{"no reply code (no channel, dial error): still stalled", 0, HealthStalled, "cannot consume for"},
		{"other reply code (320 connection forced): still stalled", 320, HealthStalled, "cannot consume for"},
		{"504 channel error: still stalled", 504, HealthStalled, "cannot consume for"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lvl, reason := refusedConsumer(tc.code).Assess(healthyClient(), ConsumerPolicy{}, healthNow)
			if lvl != tc.want || !strings.Contains(reason, tc.reasonPart) {
				t.Fatalf("got %s (%s), want %s containing %q", lvl, reason, tc.want, tc.reasonPart)
			}
		})
	}
}

func TestConsumerHealthRecordsAndClearsSubscribeErrorCode(t *testing.T) {
	c := &Consumer{client: &Client{}}
	c.health.begin("q")

	notFound := &amqp.Error{Code: 404, Reason: "NOT_FOUND - no queue 'q'", Server: true}
	c.health.failed(0, fmt.Errorf("failed to start consuming: %w", notFound))
	if got := c.Health().LastSubscribeErrorCode; got != 404 {
		t.Fatalf("after wrapped 404: code %d", got)
	}

	// The latest failure decides: a generic one clears the refusal.
	c.health.failed(0, errors.New("no channel"))
	if got := c.Health().LastSubscribeErrorCode; got != 0 {
		t.Fatalf("after generic failure: code %d", got)
	}

	c.health.failed(0, &amqp.Error{Code: 403, Reason: "ACCESS_REFUSED"})
	h := c.Health()
	cs := healthyClient()
	h.NotConsumingSince = healthNow.Add(-10 * time.Minute)
	if lvl, reason := h.Assess(cs, ConsumerPolicy{}, healthNow); h.LastSubscribeErrorCode != 403 || lvl != HealthDegraded || !strings.Contains(reason, "access refused") {
		t.Fatalf("403 via Health: code %d, %s (%s)", h.LastSubscribeErrorCode, lvl, reason)
	}

	c.health.subscribed(0, nil)
	if got := c.Health().LastSubscribeErrorCode; got != 0 {
		t.Fatalf("successful subscribe must clear the code, got %d", got)
	}
}

func TestConsumerHealthSubscribeErrorCodePerSubscription(t *testing.T) {
	c := &Consumer{client: &Client{}}
	c.health.begin("q")
	c.health.failed(1, &amqp.Error{Code: 404})
	c.health.failed(2, &amqp.Error{Code: 404})
	if got := c.Health().LastSubscribeErrorCode; got != 404 {
		t.Fatalf("all workers refused: code %d", got)
	}
	// A live worker's earlier failure must not be reported once it recovered.
	c.health.subscribed(1, nil)
	c.health.subscribed(2, nil)
	if got := c.Health().LastSubscribeErrorCode; got != 0 {
		t.Fatalf("all workers recovered: code %d", got)
	}
}
