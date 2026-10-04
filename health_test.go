package rabbitmq

import (
	"errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

var healthNow = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) time.Time { return healthNow.Add(-d) }

// healthyClient is connected, settled and not reconnecting.
func healthyClient() ClientState {
	return ClientState{Connected: true, ConnectedAt: ago(time.Hour)}
}

// wedgedPublisher satisfies every Stalled criterion under the default policy:
// publishing continues, five consecutive timeouts, nothing from the broker for
// 30 minutes, connection and confirm channel an hour old.
func wedgedPublisher() DeliveryHealth {
	return DeliveryHealth{
		Enabled: true, GenerationAlive: true, ReadersRunning: true,
		Generation: 3, GenerationSince: ago(time.Hour), StartedAt: ago(time.Hour),
		ConfirmTimeout: 30 * time.Second,
		LastPublishAt:  ago(time.Minute), LastConfirmAt: ago(30 * time.Minute),
		FailuresSinceConfirm: 5, TotalConfirmed: 100, TotalTimedOut: 5,
	}
}

func TestHealthLevelString(t *testing.T) {
	for lvl, want := range map[HealthLevel]string{HealthOK: "ok", HealthDegraded: "degraded", HealthStalled: "stalled", 9: "HealthLevel(9)"} {
		if got := lvl.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(lvl), got, want)
		}
	}
}

func TestDeliveryHealthAssess(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(h *DeliveryHealth, cs *ClientState)
		prev       func(h DeliveryHealth) DeliveryHealth
		pol        DeliveryPolicy
		want       HealthLevel
		reasonPart string
	}{
		{name: "wedge with healthy connection is stalled", want: HealthStalled, reasonPart: "5 consecutive"},
		{name: "readers dead is still a wedge", mutate: func(h *DeliveryHealth, _ *ClientState) { h.ReadersRunning = false }, want: HealthStalled},
		{name: "delivery assurance disabled", mutate: func(h *DeliveryHealth, _ *ClientState) { *h = DeliveryHealth{} }, want: HealthOK},

		// Each restart-storm guard on its own.
		{name: "guard: not connected", mutate: func(_ *DeliveryHealth, cs *ClientState) { cs.Connected = false }, want: HealthDegraded, reasonPart: "not connected"},
		{name: "guard: reconnecting", mutate: func(_ *DeliveryHealth, cs *ClientState) { cs.Reconnecting = true }, want: HealthDegraded, reasonPart: "reconnecting"},
		{
			name:   "guard: connection younger than MinConnectionAge",
			mutate: func(_ *DeliveryHealth, cs *ClientState) { cs.ConnectedAt = ago(6 * time.Minute) },
			pol:    DeliveryPolicy{MinConnectionAge: 10 * time.Minute}, want: HealthDegraded, reasonPart: "only 6m0s old",
		},
		{name: "guard: connection blocked by broker", mutate: func(_ *DeliveryHealth, cs *ClientState) {
			cs.Blocked, cs.BlockedReason, cs.BlockedAt = true, "low on memory", ago(time.Hour)
		}, want: HealthDegraded, reasonPart: "blocked by broker: low on memory"},
		{name: "unblock restarts the silence clock", mutate: func(_ *DeliveryHealth, cs *ClientState) {
			cs.BlockedAt, cs.UnblockedAt = ago(time.Hour), ago(time.Minute)
		}, want: HealthOK},
		{name: "unblocked long ago does not shield a wedge", mutate: func(_ *DeliveryHealth, cs *ClientState) {
			cs.BlockedAt, cs.UnblockedAt = ago(50*time.Minute), ago(40*time.Minute)
		}, want: HealthStalled},
		{name: "gave up reconnecting is stalled even though not connected", mutate: func(_ *DeliveryHealth, cs *ClientState) {
			cs.Connected, cs.GaveUp, cs.GaveUpAt = false, true, ago(10*time.Minute)
		}, want: HealthStalled, reasonPart: "gave up reconnecting"},
		{name: "gave up reconnecting even while another round is reconnecting", mutate: func(_ *DeliveryHealth, cs *ClientState) {
			cs.Connected, cs.Reconnecting, cs.GaveUp, cs.GaveUpAt = false, true, true, ago(10*time.Minute)
		}, want: HealthStalled},
		{name: "gave up only just now", mutate: func(_ *DeliveryHealth, cs *ClientState) {
			cs.Connected, cs.GaveUp, cs.GaveUpAt = false, true, ago(30*time.Second)
		}, want: HealthDegraded, reasonPart: "exhausted its reconnect attempts"},
		{name: "gave up with delivery assurance disabled", mutate: func(h *DeliveryHealth, cs *ClientState) {
			*h = DeliveryHealth{}
			cs.Connected, cs.GaveUp, cs.GaveUpAt = false, true, ago(10*time.Minute)
		}, want: HealthStalled},
		{name: "connection reset restarts the silence clock", mutate: func(_ *DeliveryHealth, cs *ClientState) { cs.ConnectedAt = ago(time.Minute) }, want: HealthOK},
		{name: "new confirm channel restarts the silence clock", mutate: func(h *DeliveryHealth, _ *ClientState) { h.GenerationSince = ago(time.Minute) }, want: HealthOK},

		// Evidence criteria.
		{name: "window not met", mutate: func(h *DeliveryHealth, _ *ClientState) { h.LastConfirmAt = ago(2 * time.Minute) }, want: HealthOK},
		{
			name:   "window not met but counters moved since prev",
			mutate: func(h *DeliveryHealth, _ *ClientState) { h.LastConfirmAt = ago(2 * time.Minute) },
			prev:   func(h DeliveryHealth) DeliveryHealth { h.TotalTimedOut = 1; return h },
			want:   HealthDegraded, reasonPart: "4 delivery timeouts",
		},
		{name: "not publishing any more", mutate: func(h *DeliveryHealth, _ *ClientState) { h.LastPublishAt = ago(10 * time.Minute) }, want: HealthOK},
		{name: "last publish predates last response", mutate: func(h *DeliveryHealth, _ *ClientState) { h.LastPublishAt = ago(time.Hour) }, want: HealthOK},
		{name: "never published", mutate: func(h *DeliveryHealth, _ *ClientState) { h.LastPublishAt = time.Time{} }, want: HealthOK},
		{name: "fewer failures than MinFailures", mutate: func(h *DeliveryHealth, _ *ClientState) { h.FailuresSinceConfirm = 2 }, want: HealthOK},
		{name: "MinFailures lowered", mutate: func(h *DeliveryHealth, _ *ClientState) { h.FailuresSinceConfirm = 1 }, pol: DeliveryPolicy{MinFailures: 1}, want: HealthStalled},
		{name: "never confirmed, started long ago", mutate: func(h *DeliveryHealth, _ *ClientState) { h.LastConfirmAt = time.Time{}; h.TotalConfirmed = 0 }, want: HealthStalled},
		{name: "never confirmed, started just now", mutate: func(h *DeliveryHealth, _ *ClientState) {
			h.LastConfirmAt = time.Time{}
			h.StartedAt, h.GenerationSince = ago(time.Minute), ago(time.Minute)
		}, want: HealthOK},

		// Partial failure: confirms keep flowing while most time out.
		{
			name: "partial confirms: recent confirm, failures present",
			mutate: func(h *DeliveryHealth, _ *ClientState) {
				h.LastConfirmAt = ago(20 * time.Second)
				h.FailuresSinceConfirm = 0
				h.TotalTimedOut, h.TotalConfirmed = 95, 105
			},
			prev: func(h DeliveryHealth) DeliveryHealth { h.TotalTimedOut, h.TotalConfirmed = 5, 100; return h },
			want: HealthDegraded, reasonPart: "90 delivery timeouts/orphans since previous check (5 confirmed",
		},
		{
			name:   "partial confirms: failure streak but a confirm inside the window",
			mutate: func(h *DeliveryHealth, _ *ClientState) { h.LastConfirmAt = ago(4 * time.Minute) },
			want:   HealthOK,
		},
		{
			name:   "partial confirms: returns and nacks count as responses",
			mutate: func(h *DeliveryHealth, _ *ClientState) { h.LastConfirmAt = ago(time.Minute); h.TotalReturned = 7 },
			want:   HealthOK,
		},

		// Policy defaults.
		{name: "default StallAfter scales with confirm timeout: not yet", mutate: func(h *DeliveryHealth, _ *ClientState) {
			h.ConfirmTimeout = 3 * time.Minute // 3x = 9m
			h.LastConfirmAt = ago(6 * time.Minute)
		}, want: HealthOK},
		{name: "default StallAfter scales with confirm timeout: now", mutate: func(h *DeliveryHealth, _ *ClientState) {
			h.ConfirmTimeout = 3 * time.Minute
			h.LastConfirmAt = ago(10 * time.Minute)
		}, want: HealthStalled},
		{name: "explicit StallAfter", mutate: func(h *DeliveryHealth, _ *ClientState) {
			h.LastConfirmAt, h.LastPublishAt = ago(2*time.Minute), ago(10*time.Second)
		}, pol: DeliveryPolicy{StallAfter: time.Minute}, want: HealthStalled},

		// Degraded structure signals.
		{name: "dead confirm channel", mutate: func(h *DeliveryHealth, _ *ClientState) {
			h.GenerationAlive = false
			h.LastConfirmAt = ago(time.Minute)
		}, want: HealthDegraded, reasonPart: "not alive"},
		{name: "readers not running, no stall evidence", mutate: func(h *DeliveryHealth, _ *ClientState) {
			h.ReadersRunning = false
			h.LastConfirmAt = ago(time.Minute)
		}, want: HealthDegraded, reasonPart: "readers"},
		{name: "oldest pending outlived its timeout", mutate: func(h *DeliveryHealth, _ *ClientState) {
			h.LastConfirmAt = ago(time.Minute)
			h.OldestPendingAge = 90 * time.Second
		}, want: HealthDegraded, reasonPart: "timeout timer"},
		{name: "oldest pending within slack of its timeout", mutate: func(h *DeliveryHealth, _ *ClientState) {
			h.LastConfirmAt = ago(time.Minute)
			h.OldestPendingAge = 33 * time.Second
		}, want: HealthOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, cs := wedgedPublisher(), healthyClient()
			if tc.mutate != nil {
				tc.mutate(&h, &cs)
			}
			prev := h
			if tc.prev != nil {
				prev = tc.prev(h)
			}
			got, reason := h.Assess(prev, cs, tc.pol, healthNow)
			if got != tc.want {
				t.Fatalf("Assess = %s (%q), want %s", got, reason, tc.want)
			}
			if tc.reasonPart != "" && !strings.Contains(reason, tc.reasonPart) {
				t.Fatalf("reason %q does not contain %q", reason, tc.reasonPart)
			}
			if reason == "" {
				t.Fatal("empty reason")
			}
		})
	}
}

// TestDeliveryAssessNeverStalledWithoutTrustedConnection sweeps a grid of
// snapshots engineered to maximise stall evidence and asserts the invariant the
// whole API exists to keep: no Stalled verdict unless the client is connected,
// not reconnecting, and connected for at least MinConnectionAge.
func TestDeliveryAssessNeverStalledWithoutTrustedConnection(t *testing.T) {
	stalledSeen := false
	for _, connected := range []bool{false, true} {
		for _, reconnecting := range []bool{false, true} {
			for _, blocked := range []bool{false, true} {
				for _, connAge := range []time.Duration{0, time.Second, time.Minute, 119 * time.Second, 2 * time.Minute, time.Hour} {
					for _, readers := range []bool{false, true} {
						for _, alive := range []bool{false, true} {
							h := wedgedPublisher()
							h.ReadersRunning, h.GenerationAlive = readers, alive
							h.LastPublishAt = ago(10 * time.Second)
							cs := ClientState{Connected: connected, Reconnecting: reconnecting, Blocked: blocked, ConnectedAt: ago(connAge)}
							lvl, reason := h.Assess(DeliveryHealth{}, cs, DeliveryPolicy{StallAfter: time.Minute}, healthNow)
							if lvl != HealthStalled {
								continue
							}
							stalledSeen = true
							if !connected || reconnecting || blocked || connAge < defaultMinConnectionAge {
								t.Fatalf("Stalled with connected=%t reconnecting=%t blocked=%t age=%s: %s", connected, reconnecting, blocked, connAge, reason)
							}
						}
					}
				}
			}
		}
	}
	if !stalledSeen {
		t.Fatal("grid never produced Stalled; the invariant was not exercised")
	}
}

func healthyConsumer() ConsumerHealth {
	return ConsumerHealth{
		Queue: "q", Consuming: true, ChannelOpen: true, ConnectionOpen: true,
		SubscribedAt: ago(time.Hour), LastDeliveryAt: ago(40 * time.Minute), LastAckAt: ago(40 * time.Minute),
	}
}

func TestConsumerHealthAssess(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(h *ConsumerHealth, cs *ClientState)
		pol        ConsumerPolicy
		want       HealthLevel
		reasonPart string
	}{
		{name: "idle queue is healthy however old the last delivery", want: HealthOK, reasonPart: "last delivery 40m0s ago"},
		{name: "never delivered anything", mutate: func(h *ConsumerHealth, _ *ClientState) { h.LastDeliveryAt, h.LastAckAt = time.Time{}, time.Time{} }, want: HealthOK},
		{name: "not started", mutate: func(h *ConsumerHealth, _ *ClientState) { *h = ConsumerHealth{} }, want: HealthDegraded, reasonPart: "not started"},
		{name: "stopped on purpose", mutate: func(h *ConsumerHealth, _ *ClientState) { h.Consuming, h.Stopped = false, true }, want: HealthDegraded, reasonPart: "stopped"},

		{name: "not consuming for 10m, connection healthy: stalled", mutate: func(h *ConsumerHealth, _ *ClientState) {
			h.Consuming, h.NotConsumingSince = false, ago(10*time.Minute)
		}, want: HealthStalled},
		{name: "cancelled by broker for 10m: stalled", mutate: func(h *ConsumerHealth, _ *ClientState) {
			h.Consuming, h.CancelledByBroker, h.NotConsumingSince = false, true, ago(10*time.Minute)
			h.LastError = "NOT_FOUND"
		}, want: HealthStalled, reasonPart: "cancelled by broker: true"},
		{name: "resubscribe failures at the limit", mutate: func(h *ConsumerHealth, _ *ClientState) {
			h.Consuming, h.NotConsumingSince, h.ResubscribeFailures = false, ago(10*time.Minute), 5
		}, want: HealthStalled, reasonPart: "5 consecutive resubscribe failures"},
		{name: "1 of N failing >= 5 while the others consume is degraded, never stalled", mutate: func(h *ConsumerHealth, _ *ClientState) {
			h.ActiveSubscriptions, h.ResubscribeFailures, h.FailingSubscriptions = 4, 7, 1
		}, want: HealthDegraded, reasonPart: "partially broken"},
		{name: "1 of N cancelled by broker is degraded, never stalled, however old the subscription", mutate: func(h *ConsumerHealth, _ *ClientState) {
			h.ActiveSubscriptions, h.CancelledByBroker = 4, true
			h.SubscribedAt = ago(100 * time.Hour)
		}, want: HealthDegraded, reasonPart: "partially broken"},
		{name: "all N cancelled: degraded until StallAfter has elapsed", mutate: func(h *ConsumerHealth, _ *ClientState) {
			h.Consuming, h.ActiveSubscriptions, h.CancelledByBroker = false, 0, true
			h.NotConsumingSince = ago(4 * time.Minute)
		}, want: HealthDegraded, reasonPart: "stalled after 5m0s"},
		{name: "all N cancelled: stalled once StallAfter has elapsed", mutate: func(h *ConsumerHealth, _ *ClientState) {
			h.Consuming, h.ActiveSubscriptions, h.CancelledByBroker = false, 0, true
			h.NotConsumingSince = ago(6 * time.Minute)
		}, want: HealthStalled},
		{name: "not consuming with a zero hold clock is never stalled", mutate: func(h *ConsumerHealth, _ *ClientState) {
			h.Consuming, h.NotConsumingSince, h.SubscribedAt = false, time.Time{}, ago(100*time.Hour)
		}, want: HealthDegraded},
		{name: "guard: connection blocked", mutate: func(h *ConsumerHealth, cs *ClientState) {
			h.Consuming, h.NotConsumingSince = false, ago(time.Hour)
			cs.Blocked, cs.BlockedReason = true, "disk free space low"
		}, want: HealthDegraded, reasonPart: "blocked by broker: disk free space low"},
		{name: "unblock restarts the hold clock", mutate: func(h *ConsumerHealth, cs *ClientState) {
			h.Consuming, h.NotConsumingSince = false, ago(time.Hour)
			cs.UnblockedAt = ago(time.Minute)
		}, want: HealthDegraded, reasonPart: "stalled after"},
		{name: "gave up reconnecting is stalled", mutate: func(h *ConsumerHealth, cs *ClientState) {
			h.Consuming, h.NotConsumingSince = false, ago(time.Hour)
			cs.Connected, cs.GaveUp, cs.GaveUpAt = false, true, ago(5*time.Minute)
		}, want: HealthStalled, reasonPart: "gave up"},
		{name: "gave up only just now", mutate: func(h *ConsumerHealth, cs *ClientState) {
			cs.Connected, cs.GaveUp, cs.GaveUpAt = false, true, ago(10*time.Second)
		}, want: HealthDegraded},
		{name: "four failures alone are not broken", mutate: func(h *ConsumerHealth, _ *ClientState) { h.ResubscribeFailures = 4 }, want: HealthOK},

		{name: "window not met", mutate: func(h *ConsumerHealth, _ *ClientState) {
			h.Consuming, h.NotConsumingSince = false, ago(4*time.Minute)
		}, want: HealthDegraded, reasonPart: "stalled after 5m0s"},
		{name: "explicit StallAfter", mutate: func(h *ConsumerHealth, _ *ClientState) {
			h.Consuming, h.NotConsumingSince = false, ago(2*time.Minute)
		}, pol: ConsumerPolicy{StallAfter: time.Minute}, want: HealthStalled},

		{name: "guard: not connected", mutate: func(h *ConsumerHealth, cs *ClientState) {
			h.Consuming, h.NotConsumingSince = false, ago(time.Hour)
			cs.Connected = false
		}, want: HealthDegraded, reasonPart: "not connected"},
		{name: "guard: reconnecting", mutate: func(h *ConsumerHealth, cs *ClientState) {
			h.Consuming, h.NotConsumingSince = false, ago(time.Hour)
			cs.Reconnecting = true
		}, want: HealthDegraded, reasonPart: "reconnecting"},
		{name: "guard: connection too young", mutate: func(h *ConsumerHealth, cs *ClientState) {
			h.Consuming, h.NotConsumingSince = false, ago(time.Hour)
			cs.ConnectedAt = ago(time.Minute)
		}, want: HealthDegraded, reasonPart: "only 1m0s old"},
		{name: "outage before reconnect is not charged to the consumer", mutate: func(h *ConsumerHealth, cs *ClientState) {
			h.Consuming, h.NotConsumingSince = false, ago(time.Hour)
			cs.ConnectedAt = ago(3 * time.Minute) // trusted (>= 2m) but only 3m of healthy connection
		}, want: HealthDegraded, reasonPart: "stalled after"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, cs := healthyConsumer(), healthyClient()
			if tc.mutate != nil {
				tc.mutate(&h, &cs)
			}
			got, reason := h.Assess(cs, tc.pol, healthNow)
			if got != tc.want {
				t.Fatalf("Assess = %s (%q), want %s", got, reason, tc.want)
			}
			if tc.reasonPart != "" && !strings.Contains(reason, tc.reasonPart) {
				t.Fatalf("reason %q does not contain %q", reason, tc.reasonPart)
			}
		})
	}
}

func TestConsumerAssessNeverStalledWithoutTrustedConnection(t *testing.T) {
	stalledSeen := false
	for _, connected := range []bool{false, true} {
		for _, reconnecting := range []bool{false, true} {
			for _, blocked := range []bool{false, true} {
				for _, connAge := range []time.Duration{0, time.Minute, 119 * time.Second, 10 * time.Minute, time.Hour} {
					h := healthyConsumer()
					h.Consuming, h.CancelledByBroker, h.NotConsumingSince = false, true, ago(2*time.Hour)
					cs := ClientState{Connected: connected, Reconnecting: reconnecting, Blocked: blocked, ConnectedAt: ago(connAge)}
					lvl, reason := h.Assess(cs, ConsumerPolicy{StallAfter: time.Minute}, healthNow)
					if lvl != HealthStalled {
						continue
					}
					stalledSeen = true
					if !connected || reconnecting || blocked || connAge < defaultMinConnectionAge {
						t.Fatalf("Stalled with connected=%t reconnecting=%t blocked=%t age=%s: %s", connected, reconnecting, blocked, connAge, reason)
					}
				}
			}
		}
	}
	if !stalledSeen {
		t.Fatal("grid never produced Stalled")
	}
}

// TestConsumerHealthStateMachine drives the lifecycle events the consume loops
// emit and checks the snapshot after each, with a fake clock so the time-based
// fields can be asserted exactly.
func TestConsumerHealthStateMachine(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Consumer{client: &Client{}}
		snap := func() ConsumerHealth {
			h := &c.health
			// Health() reads the client's state and channel liveness; reproduce
			// its aggregation without a real client.
			h.mu.Lock()
			defer h.mu.Unlock()
			out := ConsumerHealth{Queue: h.queue, Stopped: h.stopped, NotConsumingSince: h.since, SubscribedAt: h.subAt}
			for _, s := range h.subs {
				out.Consuming = out.Consuming || s.active
				out.CancelledByBroker = out.CancelledByBroker || s.cancelled
				out.ResubscribeFailures = max(out.ResubscribeFailures, s.failures)
				if s.failures >= consumerResubscribeFailureLimit {
					out.FailingSubscriptions++
				}
			}
			return out
		}

		if got := snap(); got.Consuming || !got.NotConsumingSince.IsZero() {
			t.Fatalf("before begin: %+v", got)
		}

		start := time.Now()
		c.health.begin("q")
		if got := snap(); got.Consuming || !got.NotConsumingSince.Equal(start) || got.Queue != "q" {
			t.Fatalf("after begin: %+v", got)
		}

		time.Sleep(10 * time.Second)
		c.health.failed(0, errors.New("no channel"))
		c.health.failed(0, errors.New("no channel"))
		if got := snap(); got.ResubscribeFailures != 2 || !got.NotConsumingSince.Equal(start) {
			t.Fatalf("after failures: %+v", got)
		}

		time.Sleep(10 * time.Second)
		c.health.subscribed(0, nil)
		got := snap()
		if !got.Consuming || got.ResubscribeFailures != 0 || !got.NotConsumingSince.IsZero() || !got.SubscribedAt.Equal(time.Now()) {
			t.Fatalf("after subscribe: %+v", got)
		}

		// Two workers: one cancelled keeps Consuming true but is reported.
		c.health.subscribed(1, nil)
		c.health.cancelledByBroker(1, nil)
		got = snap()
		if !got.Consuming || !got.CancelledByBroker {
			t.Fatalf("one worker cancelled: %+v", got)
		}

		// Everything goes: NotConsumingSince starts now, not at begin.
		time.Sleep(time.Minute)
		c.health.cancelledByBroker(0, nil)
		cancelAt := time.Now()
		got = snap()
		if got.Consuming || !got.CancelledByBroker || !got.NotConsumingSince.Equal(cancelAt) {
			t.Fatalf("all cancelled: %+v", got)
		}

		// A later subscribe clears the flag and the clock.
		c.health.subscribed(0, nil)
		c.health.subscribed(1, nil)
		got = snap()
		if got.CancelledByBroker || !got.Consuming || !got.NotConsumingSince.IsZero() {
			t.Fatalf("resubscribed: %+v", got)
		}

		// A stale cancel (different channel than the subscription's) is ignored.
		c.health.mu.Lock()
		c.health.subs[0].ch = nil
		c.health.mu.Unlock()
		c.health.cancelledByBroker(0, new(amqp.Channel))
		if got := snap(); got.CancelledByBroker {
			t.Fatalf("stale cancel applied: %+v", got)
		}

		c.health.end(true)
		if got := snap(); got.Consuming || !got.Stopped {
			t.Fatalf("after clean end: %+v", got)
		}
		c.health.begin("q")
		c.health.end(false)
		if got := snap(); got.Stopped {
			t.Fatalf("unclean end must not read as a stop: %+v", got)
		}
	})
}

// TestConsumerHealthPerSubscriptionFailures: each subscription (worker) owns its
// failure streak. One worker's success must not zero another's streak, and one
// worker's streak must not condemn a consumer whose other workers are consuming.
func TestConsumerHealthPerSubscriptionFailures(t *testing.T) {
	c := &Consumer{client: &Client{}}
	c.health.begin("q")
	c.health.subscribed(0, nil)
	for range consumerResubscribeFailureLimit {
		c.health.failed(1, errors.New("no channel"))
	}
	c.health.subscribed(2, nil) // another worker succeeding must not reset worker 1

	h := c.Health()
	if !h.Consuming || h.ActiveSubscriptions != 2 || h.ResubscribeFailures != 5 || h.FailingSubscriptions != 1 {
		t.Fatalf("snapshot: %+v", h)
	}
	cs := healthyClient()
	cs.ConnectedAt = healthNow.Add(-24 * time.Hour)
	h.SubscribedAt = healthNow.Add(-24 * time.Hour)
	if lvl, reason := h.Assess(cs, ConsumerPolicy{}, healthNow); lvl != HealthDegraded {
		t.Fatalf("1-of-3 failing: %s (%s), want degraded", lvl, reason)
	}

	c.health.subscribed(1, nil) // worker 1 recovers: its own streak resets
	if h := c.Health(); h.ResubscribeFailures != 0 || h.FailingSubscriptions != 0 {
		t.Fatalf("after recovery: %+v", h)
	}
}

// TestClientGivesUpAfterMaxReconnectAttempts drives the real reconnect loop
// against a dead port with a capped attempt count.
func TestClientGivesUpAfterMaxReconnectAttempts(t *testing.T) {
	c := &Client{
		closeCh: make(chan struct{}),
		config: &clientConfig{
			URL:                  "amqp://guest:guest@127.0.0.1:1/",
			DialTimeout:          200 * time.Millisecond,
			ReconnectDelay:       time.Millisecond,
			MaxReconnectAttempts: 3,
			Logger:               NewNopLogger(),
			Metrics:              NewNopMetrics(),
		},
	}
	c.handleReconnection()

	cs := c.State()
	if !cs.GaveUp || cs.GaveUpAt.IsZero() || cs.Reconnecting || cs.Connected || cs.LastError == "" {
		t.Fatalf("state after exhausting attempts: %+v", cs)
	}

	// A second exhausted round (the monitor re-enters) keeps the original time.
	first := cs.GaveUpAt
	c.handleReconnection()
	if got := c.State().GaveUpAt; !got.Equal(first) {
		t.Fatalf("GaveUpAt moved from %v to %v", first, got)
	}

	late := first.Add(5 * time.Minute)
	if lvl, reason := (DeliveryHealth{}).Assess(DeliveryHealth{}, c.State(), DeliveryPolicy{}, late); lvl != HealthStalled {
		t.Fatalf("publisher Assess = %s (%s), want stalled", lvl, reason)
	}
	if lvl, reason := (ConsumerHealth{}).Assess(c.State(), ConsumerPolicy{}, late); lvl != HealthStalled {
		t.Fatalf("consumer Assess = %s (%s), want stalled", lvl, reason)
	}
	if lvl, _ := (ConsumerHealth{}).Assess(c.State(), ConsumerPolicy{}, first.Add(time.Second)); lvl == HealthStalled {
		t.Fatal("stalled the moment the client gave up")
	}
}
