package rabbitmq

import (
	"errors"
	"fmt"
	"time"
)

// DeliveryHealth is a point-in-time, in-memory snapshot of a publisher's
// delivery-assurance machinery, returned by Publisher.DeliveryHealth. Feed it to
// Assess (with the previous snapshot and Client.State) to get a verdict.
type DeliveryHealth struct {
	// Enabled is true when the publisher was created with delivery assurance.
	// Everything else is zero when it is false.
	Enabled bool

	// GenerationAlive is true while the current confirm channel is open and its
	// listeners are attached. It is false between a channel dying and the
	// replacement being installed (normally well under a second once the
	// connection is back).
	GenerationAlive bool

	// ReadersRunning is true while both the confirm reader and the return reader
	// of the current generation are running. A live channel without its readers
	// is the wedge that made every publish end in a confirmation timeout.
	ReadersRunning bool

	// Generation is the id of the current confirm channel; it increments each time
	// the channel is replaced.
	Generation uint64

	// GenerationSince is when the current generation was installed.
	GenerationSince time.Time

	// StartedAt is when delivery assurance was initialised.
	StartedAt time.Time

	// Pending is the number of messages awaiting a confirmation outcome.
	Pending int64

	// OldestPendingAge is the age of the oldest pending message. It costs one pass
	// over the pending map, which is bounded by the delivery timeout.
	OldestPendingAge time.Duration

	// ConfirmTimeout is the longest delivery timeout in force: the publisher's
	// default, or a larger per-message override currently pending.
	ConfirmTimeout time.Duration

	// LastPublishAt is when a message was last handed to the broker.
	LastPublishAt time.Time

	// LastConfirmAt is when a confirm frame (ack or nack) or a return was last read
	// from the broker. It is deliberately broader than a successful confirmation:
	// a nack or an unroutable return also proves the broker is answering and the
	// readers are alive, which is the property a stall verdict needs.
	LastConfirmAt time.Time

	// FailuresSinceConfirm is the number of timeouts plus orphans on the current
	// generation since LastConfirmAt. It resets on any broker response and when
	// the confirm channel is replaced.
	FailuresSinceConfirm int64

	TotalConfirmed, TotalTimedOut, TotalOrphaned, TotalReturned, TotalNacked int64

	// LastError describes the most recent timeout, orphan or keeper failure.
	LastError   string
	LastErrorAt time.Time
}

// DeliveryHealth returns a snapshot of the publisher's delivery health. It never
// touches the broker, takes no lock a publish holds across I/O, and is safe to
// call at any frequency from any goroutine. For a publisher without delivery
// assurance it returns the zero value.
func (p *Publisher) DeliveryHealth() DeliveryHealth {
	if !p.deliveryAssuranceEnabled {
		return DeliveryHealth{}
	}

	h := DeliveryHealth{
		Enabled:        true,
		StartedAt:      p.startedAt,
		ConfirmTimeout: p.config.deliveryTimeout,
		Pending:        p.pendingMessages.Count(),
	}
	h.LastPublishAt = p.atOffset(p.lastPublishOff.Load())
	h.LastConfirmAt = p.atOffset(p.lastResponseOff.Load())
	h.FailuresSinceConfirm = p.failuresSinceResponse.Load()

	if gen := p.confirmGen.Load(); gen != nil {
		gen.mu.Lock()
		dead := gen.dead
		gen.mu.Unlock()
		h.Generation = gen.id
		h.GenerationSince = gen.since
		h.GenerationAlive = !dead && !gen.ch.IsClosed()
		h.ReadersRunning = gen.readers.Load() == 2
	}

	// Collect first and inspect afterwards: rekeyPending takes pending.mu and then a
	// shard lock, so taking pending.mu inside Range (shard lock held) would invert
	// that order. Entries already claimed (CallbackFired) have an outcome and are
	// only waiting for their cleanup goroutine; they are not "pending" for health.
	var entries []*pendingMessage
	p.pendingMessages.Range(func(_ string, pending *pendingMessage) bool {
		entries = append(entries, pending)
		return true
	})
	now := time.Now()
	for _, pending := range entries {
		pending.mu.Lock()
		claimed := pending.CallbackFired
		publishedAt, timeout := pending.PublishedAt, pending.timeout
		pending.mu.Unlock()
		if claimed {
			continue
		}
		if age := now.Sub(publishedAt); age > h.OldestPendingAge {
			h.OldestPendingAge = age
		}
		if timeout > h.ConfirmTimeout {
			h.ConfirmTimeout = timeout
		}
	}

	p.statsMutex.RLock()
	h.TotalConfirmed = p.stats.TotalConfirmed
	h.TotalTimedOut = p.stats.TotalTimedOut
	h.TotalOrphaned = p.stats.TotalOrphaned
	h.TotalReturned = p.stats.TotalReturned
	h.TotalNacked = p.stats.TotalNacked
	h.LastError = p.lastErr
	h.LastErrorAt = p.lastErrAt
	p.statsMutex.RUnlock()

	return h
}

// noteResponse records that the broker answered (confirm frame or return).
func (p *Publisher) noteResponse() {
	p.lastResponseOff.Store(p.sinceStart())
	p.failuresSinceResponse.Store(0)
}

// sinceStart is a monotonic offset from startedAt, never 0 (0 means "never").
func (p *Publisher) sinceStart() int64 {
	return max(1, int64(time.Since(p.startedAt)))
}

// atOffset converts a sinceStart offset back to a time (zero for 0).
func (p *Publisher) atOffset(off int64) time.Time {
	if off == 0 {
		return time.Time{}
	}
	return p.startedAt.Add(time.Duration(off))
}

// noteFailureLocked records a timeout or orphan. Must hold statsMutex. A failure
// belonging to a superseded generation is remembered as LastError but does not
// count towards the current generation's failure streak.
func (p *Publisher) noteFailureLocked(generation uint64, reason string) {
	p.lastErr = reason
	p.lastErrAt = time.Now()
	if gen := p.confirmGen.Load(); gen == nil || gen.id == generation {
		p.failuresSinceResponse.Add(1)
	}
}

func (p *Publisher) noteKeeperError(err error) {
	p.statsMutex.Lock()
	p.lastErr = err.Error()
	p.lastErrAt = time.Now()
	p.statsMutex.Unlock()
}

// Confirm-keeper timing. Variables, not constants, so tests can shrink them.
var (
	keeperMinBackoff  = 250 * time.Millisecond
	keeperMaxBackoff  = 30 * time.Second
	keeperPollOffline = time.Second
)

// runConfirmKeeper replaces the confirm channel as soon as it dies, instead of
// waiting for the next publish to notice. An idle publisher otherwise sits on a
// dead channel indefinitely, and a health snapshot taken then would report a
// dead generation (and the first publish after the outage would pay for the
// refresh and its retry delay).
//
// It waits on gen.returnsDone rather than gen.closes. Both fire when the channel
// dies (amqp091 closes every listener together), but closes is a single-consumer
// channel whose one reader, failOrphans, uses the close reason to explain the
// orphans it settles; a second receiver here would steal that reason. returnsDone
// is closed, never sent on, so any number of goroutines can observe it.
//
// It never refreshes while the client is disconnected or reconnecting (it polls
// the client's state instead of blocking on the connection lock the reconnect
// loop holds), backs off exponentially on failure, and exits on Close. The lazy
// refresh in the publish path stays as a fallback; both go through
// refreshConfirmChannel, which is a no-op for a generation that was already
// replaced, so they cannot install two channels for one failure.
func (p *Publisher) runConfirmKeeper() {
	defer p.shutdownWg.Done()

	// Without AutoReconnect the client never replaces a lost connection, so there
	// is nothing for the keeper to wait for or refresh against.
	if !p.client.config.AutoReconnect {
		return
	}
	p.keeperActive.Store(true)
	defer p.keeperActive.Store(false)

	for {
		gen := p.confirmGen.Load()
		select {
		case <-p.shutdownChan:
			return
		case <-gen.returnsDone:
		}

		backoff := keeperMinBackoff
		for p.confirmGen.Load() == gen {
			if p.closed.Load() {
				return
			}

			if cs := p.client.State(); !cs.Connected || cs.Reconnecting {
				if !p.keeperSleep(keeperPollOffline) {
					return
				}
				continue
			}

			err := p.refreshConfirmChannelOpt(gen, true)
			if err == nil {
				break
			}
			if p.closed.Load() {
				return
			}
			if errors.Is(err, errRefreshBusy) {
				// Another refresher, or a dial holding the connection lock: not
				// a failure, just look again shortly.
				if !p.keeperSleep(keeperMinBackoff) {
					return
				}
				continue
			}
			p.noteKeeperError(fmt.Errorf("refreshing confirm channel: %w", err))
			p.client.config.Logger.Warn("Confirm channel keeper could not refresh the channel",
				"error", err.Error(),
				"retry_in", backoff)
			if !p.keeperSleep(backoff) {
				return
			}
			backoff = min(backoff*2, keeperMaxBackoff)
		}
	}
}

// keeperSleep waits d and reports false if the publisher or client shut down first.
func (p *Publisher) keeperSleep(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-p.shutdownChan:
		return false
	case <-p.client.closeCh:
		return false
	case <-t.C:
		return true
	}
}

// DeliveryPolicy tunes DeliveryHealth.Assess. Zero values select the defaults.
type DeliveryPolicy struct {
	// StallAfter is how long publishes must have gone unanswered before the
	// publisher counts as stalled. Default: max(3 x ConfirmTimeout, 5m).
	StallAfter time.Duration

	// MinConnectionAge is how long the connection must have been up before a
	// Stalled verdict is allowed (restart-storm guard), and how long GaveUp must
	// have held to be Stalled. Default: 2m.
	MinConnectionAge time.Duration

	// MinFailures is how many timeouts or orphans, with no broker response
	// in between, are needed before Stalled. It keeps one slow confirm on a
	// sporadic publisher from ever qualifying. Default: 3.
	MinFailures int64
}

func (pol DeliveryPolicy) withDefaults(confirmTimeout time.Duration) DeliveryPolicy {
	if pol.StallAfter <= 0 {
		pol.StallAfter = max(3*confirmTimeout, 5*time.Minute)
	}
	if pol.MinConnectionAge <= 0 {
		pol.MinConnectionAge = defaultMinConnectionAge
	}
	if pol.MinFailures <= 0 {
		pol.MinFailures = 3
	}
	return pol
}

// pendingSlack is how far past its timeout the oldest pending message may be
// before the timeout timer itself is suspected (timer fire + callback hand-off).
const pendingSlack = 5 * time.Second

// Assess turns a DeliveryHealth snapshot into a verdict. It is pure: all inputs
// are arguments, so it is table-testable and takes the clock as a parameter.
// prev is the snapshot from the previous evaluation (the zero value on the first
// call, which disables the "moved since prev" Degraded signal); cs is Client.State() taken at about the same time.
//
// HealthStalled requires ALL of:
//
//   - delivery assurance is enabled;
//   - the restart-storm guard holds: cs.Connected, !cs.Reconnecting and the
//     connection is at least pol.MinConnectionAge old. Without it a broker outage
//     would be a stall in every pod at once, and restarting them cures nothing;
//   - at least pol.MinFailures timeouts/orphans on the current confirm channel
//     with no broker response since;
//   - publishing is ongoing: the last publish is within pol.StallAfter and
//     after the last response;
//   - the broker has been silent for pol.StallAfter, measured from the latest
//     of the last response, the start of the current confirm channel, the
//     start of the current connection and the publisher's creation. Failures on a
//     channel that has since been replaced, or on a connection that has since
//     been re-established, therefore never count against the new one.
//
// "Some confirms flow, most time out" is deliberately NOT Stalled. A reader
// wedge is all-or-nothing for a generation (nothing is ever read), so any
// confirm, nack or return inside the window proves the readers work and the
// problem is the broker or the network, which a restart does not cure. Such a
// publisher is HealthDegraded, with the failure/success counts in the reason.
// This is why the criterion is "no response for the window" and not the literal
// "TotalConfirmed == prev.TotalConfirmed", which would also be satisfied by a
// quiet interval of a perfectly healthy publisher, and would be defeated by a
// single return or nack.
//
// A connection the broker has blocked (memory or disk alarm, ClientState.Blocked)
// also stops confirms while the client is connected, and a restart cannot cure
// it. It is therefore part of the guard (never Stalled while blocked) and the
// silence clock restarts at ClientState.UnblockedAt.
//
// One verdict bypasses the guard on purpose: if the operator capped reconnection
// (MaxReconnectAttempts > 0) and the cap was exhausted (ClientState.GaveUp) for at
// least MinConnectionAge, the result is Stalled, because the client will not
// recover by itself and a restart is the cure. Disabled delivery assurance does
// not hide it.
//
// HealthDegraded is returned when failures moved since prev, the confirm channel
// is dead or its readers are not both running, the oldest pending message
// outlived its timeout (the timer itself is wedged), or the Stalled criteria
// are met except for a guard (the reason names it).
func (h DeliveryHealth) Assess(prev DeliveryHealth, cs ClientState, pol DeliveryPolicy, now time.Time) (HealthLevel, string) {
	pol = pol.withDefaults(h.ConfirmTimeout)
	if lvl, reason, ok := gaveUpVerdict(cs, pol.MinConnectionAge, now); ok {
		return lvl, reason
	}
	if !h.Enabled {
		return HealthOK, "delivery assurance not enabled"
	}

	ref := latestOf(h.LastConfirmAt, h.GenerationSince, h.StartedAt, cs.ConnectedAt, cs.UnblockedAt)
	silentFor := now.Sub(ref)
	failing := h.FailuresSinceConfirm >= pol.MinFailures
	publishing := h.LastPublishAt.After(ref) && now.Sub(h.LastPublishAt) < pol.StallAfter
	candidate := failing && publishing && silentFor >= pol.StallAfter

	if candidate {
		ok, why := connectionTrusted(cs, pol.MinConnectionAge, now)
		if !ok {
			return HealthDegraded, fmt.Sprintf(
				"confirms look stalled (no broker response for %s, %d consecutive failures) but not declared: %s",
				silentFor.Round(time.Second), h.FailuresSinceConfirm, why)
		}
		return HealthStalled, fmt.Sprintf(
			"no broker response for %s while publishing (last publish %s ago): %d consecutive timeouts/orphans, %d confirmed in total, confirm channel generation %d, readers running: %t",
			silentFor.Round(time.Second), now.Sub(h.LastPublishAt).Round(time.Second),
			h.FailuresSinceConfirm, h.TotalConfirmed, h.Generation, h.ReadersRunning)
	}

	failuresMoved := (h.TotalTimedOut + h.TotalOrphaned) - (prev.TotalTimedOut + prev.TotalOrphaned)
	responses := (h.TotalConfirmed + h.TotalReturned + h.TotalNacked) - (prev.TotalConfirmed + prev.TotalReturned + prev.TotalNacked)

	switch {
	case !h.GenerationAlive:
		return HealthDegraded, fmt.Sprintf("confirm channel (generation %d) is not alive; being replaced", h.Generation)
	case !h.ReadersRunning:
		return HealthDegraded, fmt.Sprintf("confirm/return readers of generation %d are not both running", h.Generation)
	case h.OldestPendingAge > h.ConfirmTimeout+pendingSlack:
		return HealthDegraded, fmt.Sprintf("oldest pending message is %s old, past its %s timeout (timeout timer not firing, or its send is blocked on the wire)",
			h.OldestPendingAge.Round(time.Second), h.ConfirmTimeout)
	case prev.Enabled && failuresMoved > 0:
		return HealthDegraded, fmt.Sprintf("%d delivery timeouts/orphans since previous check (%d confirmed/returned/nacked in the same period); %d consecutive since last broker response",
			failuresMoved, responses, h.FailuresSinceConfirm)
	}
	return HealthOK, fmt.Sprintf("%d pending, %d confirmed, last broker response %s", h.Pending, h.TotalConfirmed, ageString(h.LastConfirmAt, now))
}

func ageString(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return now.Sub(t).Round(time.Second).String() + " ago"
}
