package rabbitmq

import (
	"context"
	"errors"
	"time"
)

// Redelivery backoff.
//
// A consumer that returns RejectError{Requeue: true} on a retryable failure gets
// its message back IMMEDIATELY. During a sustained downstream outage — a
// database, an API, a model provider — that tight loop burns through a quorum
// queue's delivery-limit (RabbitMQ 4.x default: 20) in milliseconds, so the
// message dead-letters long before the downstream has any chance to recover.
// Retrying without pacing mostly just reaches the same destination faster.
//
// RequeueWithBackoff spaces redeliveries over minutes instead. With the values
// below, exhausting 20 deliveries takes roughly eight minutes.
//
// This began as an rmqbackoff package inside search-intelligence-service, was
// copied into five more services as they hit the same problem, and lives here so
// there is one copy to fix rather than six to keep in step.

const (
	// backoffBase is the delay before the first requeue. It doubles on each
	// subsequent redelivery, up to backoffMax.
	backoffBase = 1 * time.Second

	// backoffMax caps the per-redelivery delay. Keep it comfortably under any
	// configured MessageTimeout, or the wait can trip the per-message deadline
	// by itself.
	backoffMax = 30 * time.Second
)

// RequeueWithBackoff waits a bounded, exponentially-growing delay keyed to the
// broker-tracked delivery count, then returns a RejectError that requeues.
//
// The wait is context-aware: on shutdown, or when a per-message deadline fires,
// it stops waiting and requeues immediately rather than holding the worker.
//
// cause is the failure that triggered the retry and becomes RejectError.Cause,
// which this library logs — so it must be non-nil, and a nil is replaced rather
// than panicking inside a failure path.
//
// Use it in place of returning RejectError{Requeue: true} directly:
//
//	if err := doWork(ctx, msg); err != nil {
//	    return rabbitmq.RequeueWithBackoff(ctx, delivery, err)
//	}
func RequeueWithBackoff(ctx context.Context, delivery *Delivery, cause error) error {
	if d := BackoffForDeliveryCount(DeliveryCount(delivery)); d > 0 {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
	}
	if cause == nil {
		cause = errors.New("retryable delivery requeue")
	}
	return &RejectError{Requeue: true, Cause: cause}
}

// BackoffForDeliveryCount returns the delay for a given delivery count:
// backoffBase * 2^(count-1), capped at backoffMax. Counts below 1 yield
// backoffBase.
//
// Pure and exported so the schedule can be asserted directly, without a broker.
func BackoffForDeliveryCount(count int) time.Duration {
	if count < 1 {
		count = 1
	}
	d := backoffBase
	for i := 1; i < count; i++ {
		d *= 2
		if d >= backoffMax {
			return backoffMax
		}
	}
	return d
}

// DeliveryCount reads the broker-tracked x-delivery-count header.
//
// QUORUM QUEUES ONLY, and ABSENT ON A FIRST DELIVERY — the header appears from
// the first redelivery onward. A missing or unexpected value is reported as 0,
// which yields the base delay, so a first failure waits the shortest time. Do
// not use this to decide whether a queue is quorum: see WithQueueType for why
// that inference is unsound.
func DeliveryCount(delivery *Delivery) int {
	if delivery == nil || delivery.Headers == nil {
		return 0
	}
	switch n := delivery.Headers["x-delivery-count"].(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	default:
		return 0
	}
}
