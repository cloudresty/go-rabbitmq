package rabbitmq

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestBackoffGrowsAndIsCapped pins the schedule, because the numbers are the
// point: they have to outlast a typical outage without outlasting the queue's
// delivery-limit.
//
// With a quorum queue's default delivery-limit of 20, this schedule spends
// roughly eight minutes before dead-lettering. An immediate requeue spends
// microseconds — which is why unpaced retrying is barely better than none.
func TestBackoffGrowsAndIsCapped(t *testing.T) {
	tests := []struct {
		count int
		want  time.Duration
	}{
		{0, backoffBase}, // a first delivery has no count header
		{1, backoffBase},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, backoffMax}, // 32s would exceed the cap
		{20, backoffMax},
		{-1, backoffBase}, // a nonsense count must not produce a nonsense wait
	}

	for _, tc := range tests {
		if got := BackoffForDeliveryCount(tc.count); got != tc.want {
			t.Errorf("BackoffForDeliveryCount(%d) = %v, want %v", tc.count, got, tc.want)
		}
	}

	// The cap exists so the wait cannot by itself trip a per-message deadline.
	if backoffMax >= 60*time.Second {
		t.Error("the backoff cap is at or above a typical MessageTimeout, so the wait alone could fail the message")
	}
}

// TestRequeueWithBackoffStopsWaitingOnShutdown pins the property that keeps a
// rolling deploy quick.
//
// An unconditional sleep holds the worker for the full backoff on every
// in-flight message, so a pod cannot exit until the longest wait elapses. That
// is also how in-flight messages end up nacked during termination.
func TestRequeueWithBackoffStopsWaitingOnShutdown(t *testing.T) {
	// A high delivery count would otherwise wait the full cap.
	delivery := &Delivery{}
	delivery.Headers = map[string]any{"x-delivery-count": 10}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already shutting down

	start := time.Now()
	err := RequeueWithBackoff(ctx, delivery, errors.New("downstream unavailable"))
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("waited %v after the context was cancelled — a shutdown would block for the full backoff", elapsed)
	}

	var rejectErr *RejectError
	if !errors.As(err, &rejectErr) {
		t.Fatalf("returned %T, want a RejectError", err)
	}
	if !rejectErr.Requeue {
		t.Error("returned a reject that does not requeue; a retryable failure would dead-letter instead")
	}
}

// TestRequeueWithBackoffAlwaysCarriesACause pins a small but sharp edge: this
// library logs Cause.Error(), so a nil cause on a failure path would panic
// inside error handling.
func TestRequeueWithBackoffAlwaysCarriesACause(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RequeueWithBackoff(ctx, &Delivery{}, nil)

	var rejectErr *RejectError
	if !errors.As(err, &rejectErr) {
		t.Fatalf("returned %T, want a RejectError", err)
	}
	if rejectErr.Cause == nil {
		t.Fatal("a nil cause was passed through; logging it would panic on a path that is already handling a failure")
	}
}

// TestDeliveryCountIsAbsentOnFirstDelivery documents the fact that made the
// old quorum detection wrong, so nobody reintroduces it.
//
// x-delivery-count appears from the first REDELIVERY onward. A consumer's first
// failure therefore sees no header at all — which is precisely when the retry
// path has to choose a strategy. Inferring "not a quorum queue" from its absence
// is what caused routing keys to be rewritten.
func TestDeliveryCountIsAbsentOnFirstDelivery(t *testing.T) {
	first := &Delivery{}
	if got := DeliveryCount(first); got != 0 {
		t.Errorf("DeliveryCount on a header-less delivery = %d, want 0", got)
	}

	// It must not be read as evidence about the queue type. Both a classic
	// queue and a quorum queue's FIRST delivery look exactly like this.
	if DeliveryCount(first) != DeliveryCount(&Delivery{}) {
		t.Error("a first delivery and an empty delivery are distinguishable, which they are not at the broker")
	}

	for _, tc := range []struct {
		name  string
		value any
		want  int
	}{
		{"int", 3, 3},
		{"int32", int32(4), 4},
		{"int64", int64(5), 5},
		{"an unexpected type is not guessed at", "7", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &Delivery{}
			d.Headers = map[string]any{"x-delivery-count": tc.value}
			if got := DeliveryCount(d); got != tc.want {
				t.Errorf("DeliveryCount = %d, want %d", got, tc.want)
			}
		})
	}
}
