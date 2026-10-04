package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Integration tests for the functional-health API against a real broker (the
// same RabbitMQ + management plugin the reconnect tests use).

// eventually polls cond every 10ms until it holds or d elapses.
func eventually(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %v waiting for %s", d, what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestHealth_PublisherChannelRecoversWithoutAnyPublish: after the broker kills
// the connection, the confirm generation is observed dead, then replaced by the
// proactive keeper while NOTHING is published.
func TestHealth_PublisherChannelRecoversWithoutAnyPublish(t *testing.T) {
	f := newReconnectFixture(t)

	f.publish(t, "warm-up")
	if res := f.rec.await(t, "warm-up", awaitWindow); res.outcome != DeliverySuccess {
		t.Fatalf("warm-up: %s (%s)", res.outcome, res.errMsg)
	}

	before := f.publisher.DeliveryHealth()
	if !before.Enabled || !before.GenerationAlive || !before.ReadersRunning {
		t.Fatalf("healthy publisher reported unhealthy: %+v", before)
	}
	if before.LastConfirmAt.IsZero() || before.LastPublishAt.IsZero() || before.TotalConfirmed != 1 {
		t.Fatalf("activity not reflected: %+v", before)
	}
	if cs := f.client.State(); !cs.Connected || cs.Reconnecting || cs.Reconnects != 0 || cs.ConnectedAt.IsZero() {
		t.Fatalf("client state before: %+v", cs)
	}
	published := f.publisher.GetDeliveryStats().TotalPublished

	var sawDeadGen, sawDisconnected, sawReconnecting atomic.Bool
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if h := f.publisher.DeliveryHealth(); !h.GenerationAlive {
				sawDeadGen.Store(true)
			}
			cs := f.client.State()
			if !cs.Connected {
				sawDisconnected.Store(true)
			}
			if cs.Reconnecting {
				sawReconnecting.Store(true)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	killBrokerConnection(t, f.connName)

	// Recovery with no publish: a new, alive generation with running readers.
	eventually(t, 20*time.Second, "keeper to replace the confirm channel", func() bool {
		h := f.publisher.DeliveryHealth()
		return h.GenerationAlive && h.ReadersRunning && h.Generation > before.Generation
	})
	close(stop)
	wg.Wait()

	if !sawDeadGen.Load() {
		t.Error("never observed GenerationAlive=false during the outage")
	}
	if !sawDisconnected.Load() {
		t.Error("never observed Connected=false during the outage")
	}
	if !sawReconnecting.Load() {
		t.Log("Reconnecting window was too short to sample (not an error)")
	}
	if got := f.publisher.GetDeliveryStats().TotalPublished; got != published {
		t.Fatalf("a publish happened during recovery (%d -> %d); the lazy path may have done the work", published, got)
	}
	cs := f.client.State()
	if !cs.Connected || cs.Reconnects != 1 || cs.LastDisconnectAt.IsZero() || cs.LastError == "" {
		t.Fatalf("client state after: %+v", cs)
	}

	// And it works: a publish is confirmed, with no timeout recorded.
	f.publish(t, "after")
	if res := f.rec.await(t, "after", awaitWindow); res.outcome != DeliverySuccess {
		t.Fatalf("after recovery: %s (%s)", res.outcome, res.errMsg)
	}
	h := f.publisher.DeliveryHealth()
	if h.TotalTimedOut != 0 || h.FailuresSinceConfirm != 0 {
		t.Fatalf("failures recorded: %+v", h)
	}
	if lvl, reason := h.Assess(before, f.client.State(), DeliveryPolicy{}, time.Now()); lvl != HealthOK {
		t.Fatalf("Assess after recovery = %s (%s)", lvl, reason)
	}
}

// TestHealth_AssessNeverStalledWhileReconnecting samples the real publisher and
// client through a connection kill and recovery. Every snapshot is Assessed with
// a hair-trigger policy and stall evidence forced on, so the restart-storm guard
// is the only thing between the snapshot and Stalled; it must hold at every
// sample, and Stalled must still be reachable once the connection is trusted
// (otherwise the test proves nothing).
func TestHealth_AssessNeverStalledWhileReconnecting(t *testing.T) {
	f := newReconnectFixture(t)
	f.publish(t, "warm-up")
	f.rec.await(t, "warm-up", awaitWindow)

	pol := DeliveryPolicy{StallAfter: time.Millisecond, MinConnectionAge: time.Nanosecond, MinFailures: 1}
	var (
		mu                                         sync.Mutex
		samples, reconnectingSamples, stalledCount int
		violation                                  string
	)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var prev DeliveryHealth
		for {
			select {
			case <-stop:
				return
			default:
			}
			cs := f.client.State()
			h := f.publisher.DeliveryHealth()
			now := time.Now().Add(time.Second)
			h.FailuresSinceConfirm = 10
			h.LastConfirmAt = time.Time{}
			h.LastPublishAt = now.Add(-time.Microsecond)
			lvl, reason := h.Assess(prev, cs, pol, now)

			mu.Lock()
			samples++
			if cs.Reconnecting {
				reconnectingSamples++
			}
			if lvl == HealthStalled {
				stalledCount++
				if !cs.Connected || cs.Reconnecting {
					violation = fmt.Sprintf("Stalled with %+v: %s", cs, reason)
				}
			}
			mu.Unlock()
			prev = h
			time.Sleep(time.Millisecond)
		}
	}()

	killBrokerConnection(t, f.connName)
	time.Sleep(500 * time.Millisecond) // sample the settled connection too
	close(stop)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if violation != "" {
		t.Fatal(violation)
	}
	if stalledCount == 0 {
		t.Fatal("Stalled was never reachable, even on a healthy connection")
	}
	t.Logf("%d samples, %d while Reconnecting, %d Stalled (all on a trusted connection)", samples, reconnectingSamples, stalledCount)
}

type consumerFixture struct {
	client *Client
	queue  string
}

func newConsumerFixture(t *testing.T) *consumerFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	client, err := NewClient(
		WithHosts("localhost:5672"),
		WithCredentials("guest", "guest"),
		WithConnectionName("health-consumer-"+suffix),
		WithReconnectDelay(200*time.Millisecond),
		WithoutTopologyValidation(), // a deleted queue must stay deleted
	)
	if err != nil {
		t.Skip("RabbitMQ not available for testing")
	}
	t.Cleanup(func() { _ = client.Close() })
	q, err := client.Admin().DeclareQueue(context.Background(), "health-consumer-q-"+suffix)
	if err != nil {
		t.Fatalf("declare queue: %v", err)
	}
	t.Cleanup(func() { _ = client.Admin().DeleteQueue(context.Background(), q.Name) })
	return &consumerFixture{client: client, queue: q.Name}
}

// TestHealth_ConsumerSeesBrokerCancel deletes the consumed queue. The broker
// cancels the consumer (basic.cancel) while the channel and connection stay
// open; Health must show that, and Assess must escalate it to Stalled.
func TestHealth_ConsumerSeesBrokerCancel(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []ConsumerOption
	}{
		{"shared channel", []ConsumerOption{WithConcurrency(2)}},
		{"channel per worker", []ConsumerOption{WithConcurrency(2), WithChannelPerWorker()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			f := newConsumerFixture(t)
			consumer, err := f.client.NewConsumer(mode.opts...)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = consumer.Close() })

			if h := consumer.Health(); h.Consuming || h.Queue != "" {
				t.Fatalf("before Consume: %+v", h)
			}

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			var handled atomic.Int32
			go func() {
				done <- consumer.Consume(ctx, f.queue, func(_ context.Context, _ *Delivery) error {
					handled.Add(1)
					return nil
				})
			}()
			t.Cleanup(func() { cancel(); <-done })

			eventually(t, 10*time.Second, "consumer to subscribe", func() bool { return consumer.Health().Consuming })
			h := consumer.Health()
			if h.Queue != f.queue || !h.ChannelOpen || !h.ConnectionOpen || h.CancelledByBroker || h.SubscribedAt.IsZero() || !h.NotConsumingSince.IsZero() {
				t.Fatalf("healthy consumer: %+v", h)
			}
			if lvl, reason := h.Assess(f.client.State(), ConsumerPolicy{}, time.Now()); lvl != HealthOK {
				t.Fatalf("Assess healthy = %s (%s)", lvl, reason)
			}

			// A delivery and its ack are reflected.
			pub, err := f.client.NewPublisher()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = pub.Close() })
			if err := pub.Publish(context.Background(), "", f.queue, NewMessage([]byte("hi"))); err != nil {
				t.Fatal(err)
			}
			eventually(t, 5*time.Second, "delivery and ack to be recorded", func() bool {
				h := consumer.Health()
				return handled.Load() == 1 && !h.LastDeliveryAt.IsZero() && !h.LastAckAt.IsZero() && h.InFlight == 0
			})

			// Delete the queue under the consumer.
			if err := f.client.Admin().DeleteQueue(context.Background(), f.queue); err != nil {
				t.Fatalf("delete queue: %v", err)
			}
			eventually(t, 10*time.Second, "CancelledByBroker and !Consuming", func() bool {
				h := consumer.Health()
				return h.CancelledByBroker && !h.Consuming
			})
			eventually(t, 10*time.Second, "resubscribe failures to accumulate", func() bool {
				return consumer.Health().ResubscribeFailures >= 1
			})
			h = consumer.Health()
			// The broker may also drop the connection when a quorum consumer is
			// resubscribed mid-delete, so ConnectionOpen is not asserted here.
			if h.NotConsumingSince.IsZero() || h.LastError == "" || h.LastErrorAt.IsZero() {
				t.Fatalf("after cancel: %+v", h)
			}

			// Default policy: degraded, because the window (5m) is far from met.
			if lvl, reason := h.Assess(f.client.State(), ConsumerPolicy{}, time.Now()); lvl != HealthDegraded {
				t.Fatalf("Assess (default policy) = %s (%s), want degraded", lvl, reason)
			}
			// Short window: the same fact becomes Stalled once held long enough.
			pol := ConsumerPolicy{StallAfter: time.Second, MinConnectionAge: time.Millisecond}
			eventually(t, 10*time.Second, "Assess to escalate to Stalled", func() bool {
				lvl, _ := consumer.Health().Assess(f.client.State(), pol, time.Now())
				return lvl == HealthStalled
			})
		})
	}
}
