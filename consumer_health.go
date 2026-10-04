package rabbitmq

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// consumerResubscribeFailureLimit is the number of consecutive failed
// (re)subscription attempts at which a consumer counts as not consuming even
// though the retry loop is still spinning.
const consumerResubscribeFailureLimit = 5

// ConsumerHealth is a point-in-time, in-memory snapshot of a consumer's ability
// to receive messages, returned by Consumer.Health. Feed it to Assess.
//
// A Consumer consumes one queue at a time (Consume and ConsumeBatch both refuse
// to start while it is already consuming), so there is exactly one
// ConsumerHealth per Consumer. In channel-per-worker mode it aggregates the
// workers: Consuming is true while at least one worker is subscribed.
type ConsumerHealth struct {
	// Queue is the queue passed to Consume or ConsumeBatch; empty before either
	// was called.
	Queue string

	// Consuming is true while at least one subscription is live: Consume
	// succeeded and the delivery channel has not closed since.
	Consuming bool

	// ChannelOpen is true if at least one subscribed channel is open.
	ChannelOpen bool

	// ConnectionOpen is the client's Connected flag at snapshot time.
	ConnectionOpen bool

	// CancelledByBroker is true when the broker cancelled a subscription (basic.cancel,
	// e.g. the queue was deleted, or its node/leader went away) and it has not
	// been re-established yet. The channel and connection stay open in that case,
	// which is exactly why it needs its own flag.
	CancelledByBroker bool

	// Stopped is true once Consume/ConsumeBatch has returned (context cancelled,
	// stopped or closed). A stopped consumer is not a stalled one.
	Stopped bool

	// SubscribedAt is when the current set of subscriptions was first established.
	SubscribedAt time.Time

	// NotConsumingSince is when Consuming last became false (or when Consume was
	// called, if it never subscribed). Zero while consuming and before Consume.
	NotConsumingSince time.Time

	// LastDeliveryAt is when a delivery was last received. An old value is NOT a
	// fault by itself: an idle queue is legitimate.
	LastDeliveryAt time.Time

	// LastAckAt is when a delivery was last acknowledged successfully (for an
	// auto-ack consumer, when it was received).
	LastAckAt time.Time

	// InFlight is the number of deliveries currently being handled.
	InFlight int

	// ResubscribeFailures is the number of consecutive failed attempts to
	// (re)establish a subscription (no channel, QoS or Consume failure) since the
	// last success.
	ResubscribeFailures int

	// LastError is the most recent subscription failure.
	LastError   string
	LastErrorAt time.Time
}

// consumerHealth is the mutable state behind Consumer.Health. The zero value is
// ready to use. Lifecycle events take a short mutex; the per-message hot path
// only touches atomics.
type consumerHealth struct {
	mu       sync.Mutex
	queue    string
	begun    bool
	stopped  bool
	subs     map[int]*subscriptionState
	failures int
	lastErr  string
	lastErrT time.Time
	since    time.Time // NotConsumingSince
	subAt    time.Time

	lastDelivery atomic.Int64
	lastAck      atomic.Int64
	inFlight     atomic.Int64
}

// subscriptionState is one subscription: the shared channel (id 0) or one worker.
type subscriptionState struct {
	ch        *amqp.Channel
	active    bool
	cancelled bool
}

func (h *consumerHealth) sub(id int) *subscriptionState {
	if h.subs == nil {
		h.subs = make(map[int]*subscriptionState)
	}
	s := h.subs[id]
	if s == nil {
		s = &subscriptionState{}
		h.subs[id] = s
	}
	return s
}

func (h *consumerHealth) anyActiveLocked() bool {
	for _, s := range h.subs {
		if s.active {
			return true
		}
	}
	return false
}

// refreshSinceLocked keeps since in step with whether anything is subscribed.
func (h *consumerHealth) refreshSinceLocked() {
	if h.anyActiveLocked() {
		h.since = time.Time{}
	} else if h.since.IsZero() {
		h.since = time.Now()
	}
}

func (h *consumerHealth) begin(queue string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.queue = queue
	h.begun = true
	h.stopped = false
	h.subs = nil
	h.failures = 0
	h.subAt = time.Time{}
	h.since = time.Now()
}

// end records that Consume/ConsumeBatch returned. clean means it was asked to
// stop (context or stop signal); an unclean return (the delivery channel closed
// under a ConsumeBatch, a setup failure) is a fault and must stay visible to
// Assess instead of reading as an intentional stop.
func (h *consumerHealth) end(clean bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = clean
	for _, s := range h.subs {
		s.active = false
	}
	h.refreshSinceLocked()
}

// subscribed records a successful Consume on ch.
func (h *consumerHealth) subscribed(id int, ch *amqp.Channel) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sub(id)
	s.ch, s.active, s.cancelled = ch, true, false
	h.failures = 0
	if h.subAt.IsZero() {
		h.subAt = time.Now()
	}
	h.refreshSinceLocked()
}

// closed records that a subscription's delivery channel closed.
func (h *consumerHealth) closed(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sub(id).active = false
	h.subAt = time.Time{}
	h.refreshSinceLocked()
}

// cancelledByBroker records a basic.cancel received on ch. A notification from a
// channel that is no longer the subscription's current one is stale and ignored.
func (h *consumerHealth) cancelledByBroker(id int, ch *amqp.Channel) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sub(id)
	if s.ch != ch {
		return
	}
	s.active, s.cancelled = false, true
	h.subAt = time.Time{}
	h.refreshSinceLocked()
}

// failed records a failed attempt to (re)establish a subscription.
func (h *consumerHealth) failed(id int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sub(id)
	s.active = false
	h.failures++
	h.lastErr, h.lastErrT = err.Error(), time.Now()
	h.refreshSinceLocked()
}

func (h *consumerHealth) delivered(autoAck bool) {
	now := time.Now().UnixNano()
	h.lastDelivery.Store(now)
	if autoAck {
		h.lastAck.Store(now)
	}
}

func (h *consumerHealth) acked() {
	h.lastAck.Store(time.Now().UnixNano())
}

// watchCancel registers a basic.cancel listener on ch for subscription id. The
// listener must exist before Consume is called, or a cancel racing the start
// would be missed. The goroutine ends when amqp091 closes the listener channel,
// which it does when ch (or its connection) closes; every channel this package
// subscribes on is closed by its owner, so it cannot outlive its channel.
func (c *Consumer) watchCancel(ch *amqp.Channel, id int) {
	cancels := ch.NotifyCancel(make(chan string, 1))
	go func() {
		for range cancels {
			c.client.config.Logger.Warn("Consumer cancelled by broker",
				"queue", c.health.queueName())
			c.health.cancelledByBroker(id, ch)
		}
	}()
}

func (h *consumerHealth) queueName() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.queue
}

// Health returns a snapshot of the consumer's health. It never touches the
// broker, is safe to call at any frequency from any goroutine, and never blocks
// on message handling.
func (c *Consumer) Health() ConsumerHealth {
	h := &c.health
	out := ConsumerHealth{
		InFlight:       int(h.inFlight.Load()),
		ConnectionOpen: c.client.State().Connected,
	}
	if n := h.lastDelivery.Load(); n != 0 {
		out.LastDeliveryAt = time.Unix(0, n)
	}
	if n := h.lastAck.Load(); n != 0 {
		out.LastAckAt = time.Unix(0, n)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	out.Queue = h.queue
	out.Stopped = h.stopped
	out.SubscribedAt = h.subAt
	out.NotConsumingSince = h.since
	out.ResubscribeFailures = h.failures
	out.LastError, out.LastErrorAt = h.lastErr, h.lastErrT
	for _, s := range h.subs {
		if s.active {
			out.Consuming = true
			if s.ch != nil && !s.ch.IsClosed() {
				out.ChannelOpen = true
			}
		}
		if s.cancelled {
			out.CancelledByBroker = true
		}
	}
	return out
}

// ConsumerPolicy tunes ConsumerHealth.Assess. Zero values select the defaults.
type ConsumerPolicy struct {
	// StallAfter is how long a consumer must have been unable to consume, with the
	// connection healthy, before it counts as stalled. Default: 5m.
	StallAfter time.Duration

	// MinConnectionAge is how long the connection must have been up before a
	// Stalled verdict is allowed (restart-storm guard). Default: 2m.
	MinConnectionAge time.Duration
}

func (pol ConsumerPolicy) withDefaults() ConsumerPolicy {
	if pol.StallAfter <= 0 {
		pol.StallAfter = 5 * time.Minute
	}
	if pol.MinConnectionAge <= 0 {
		pol.MinConnectionAge = defaultMinConnectionAge
	}
	return pol
}

// Assess turns a ConsumerHealth snapshot into a verdict. It is pure; cs is
// Client.State() taken at about the same time.
//
// HealthStalled requires that the consumer be unable to consume, that is
// (!Consuming || CancelledByBroker || ResubscribeFailures >= 5), for at least
// pol.StallAfter WHILE the connection was healthy, and that the restart-storm
// guard hold now (cs.Connected, !cs.Reconnecting, connection at least
// pol.MinConnectionAge old). The hold time is measured from the later of
// NotConsumingSince and cs.ConnectedAt: a consumer gets a full StallAfter on a
// fresh connection to resubscribe before it is blamed, rather than being
// charged for the outage that preceded the reconnect.
//
// "No deliveries for a long time" is never a fault on its own, however old
// LastDeliveryAt is: an idle queue is legitimate. It is reported in the
// reason of an OK verdict only.
//
// A consumer that never started, or whose Consume has returned, is
// HealthDegraded, never Stalled: the first is a wiring question for readiness,
// the second an intentional stop, and neither is cured by a restart on a timer.
func (h ConsumerHealth) Assess(cs ClientState, pol ConsumerPolicy, now time.Time) (HealthLevel, string) {
	pol = pol.withDefaults()

	switch {
	case h.Stopped:
		return HealthDegraded, fmt.Sprintf("consumer of %q has stopped", h.Queue)
	case h.NotConsumingSince.IsZero() && !h.Consuming:
		return HealthDegraded, "consumer has not started consuming"
	}

	broken := !h.Consuming || h.CancelledByBroker || h.ResubscribeFailures >= consumerResubscribeFailureLimit
	if !broken {
		return HealthOK, fmt.Sprintf("consuming %q, %d in flight, last delivery %s, last ack %s",
			h.Queue, h.InFlight, ageString(h.LastDeliveryAt, now), ageString(h.LastAckAt, now))
	}

	// ResubscribeFailures >= limit with Consuming still true (another worker is
	// fine) has no NotConsumingSince; fall back to the subscription time.
	since := h.NotConsumingSince
	if since.IsZero() {
		since = h.SubscribedAt
	}
	held := now.Sub(latestOf(since, cs.ConnectedAt))
	state := fmt.Sprintf("consumer of %q cannot consume (consuming: %t, cancelled by broker: %t, %d consecutive resubscribe failures, last error %q)",
		h.Queue, h.Consuming, h.CancelledByBroker, h.ResubscribeFailures, h.LastError)

	if ok, why := connectionTrusted(cs, pol.MinConnectionAge, now); !ok {
		return HealthDegraded, fmt.Sprintf("%s; not declared stalled: %s", state, why)
	}
	if held < pol.StallAfter {
		return HealthDegraded, fmt.Sprintf("%s for %s (stalled after %s)", state, held.Round(time.Second), pol.StallAfter)
	}
	return HealthStalled, fmt.Sprintf("%s for %s with a healthy connection", state, held.Round(time.Second))
}
