package rabbitmq

import (
	"fmt"
	"time"
)

// HealthLevel is the verdict of an Assess call.
//
// The levels are deliberately about FUNCTION, not connectivity: a client can be
// connected and still be unable to do its job (a publisher whose confirms never
// arrive, a consumer the broker silently cancelled). Map them onto probes as:
//
//   - HealthStalled: a liveness-FAIL candidate. It is only ever returned while
//     the client itself is connected, so a restart can actually cure it.
//   - HealthDegraded: report in readiness detail; do not restart on it.
//   - HealthOK: nothing to report.
type HealthLevel int

const (
	// HealthOK means nothing is wrong that the assessed signals can see.
	HealthOK HealthLevel = iota
	// HealthDegraded means something looks wrong but is not provably a wedge
	// (counters moving without enough evidence, a dead channel being replaced,
	// a guard such as "connection too young" withholding a harder verdict).
	HealthDegraded
	// HealthStalled means a functional wedge is provable while the connection is
	// healthy: a restart of this process would plausibly cure it.
	HealthStalled
)

// String returns "ok", "degraded" or "stalled".
func (l HealthLevel) String() string {
	switch l {
	case HealthOK:
		return "ok"
	case HealthDegraded:
		return "degraded"
	case HealthStalled:
		return "stalled"
	default:
		return fmt.Sprintf("HealthLevel(%d)", int(l))
	}
}

// defaultMinConnectionAge is how long a connection must have been up before a
// functional wedge may be declared against it.
const defaultMinConnectionAge = 2 * time.Minute

// connectionTrusted is the restart-storm guard shared by every Stalled verdict.
//
// A restart cures an in-process wedge; it never cures a broker outage. When the
// broker rolls or the network drops, every publisher and consumer in the fleet
// sees confirms stop and consumers cancel at the same moment, and a verdict that
// fired then would restart the whole fleet into the same outage. So a functional
// verdict is withheld unless the client reports that it is connected, is not in
// the middle of reconnecting, and has been connected for at least minAge (the
// state right after a reconnect is still settling: channels re-opening, the
// first publishes in flight).
//
// A connection the broker has blocked (memory or disk alarm) is not trusted
// either: confirms stop and publishes stall while the client is nominally
// connected, and a restart cures nothing. Callers also restart their silence
// clock at ClientState.UnblockedAt.
//
// It returns false and the name of the first guard that failed.
func connectionTrusted(cs ClientState, minAge time.Duration, now time.Time) (bool, string) {
	switch {
	case !cs.Connected:
		return false, "client is not connected"
	case cs.Reconnecting:
		return false, "client is reconnecting"
	case cs.Blocked:
		return false, fmt.Sprintf("connection blocked by broker: %s", cs.BlockedReason)
	case now.Sub(cs.ConnectedAt) < minAge:
		return false, fmt.Sprintf("connection is only %s old (minimum %s)", now.Sub(cs.ConnectedAt).Round(time.Second), minAge)
	}
	return true, ""
}

func latestOf(ts ...time.Time) time.Time {
	var latest time.Time
	for _, t := range ts {
		if t.After(latest) {
			latest = t
		}
	}
	return latest
}

// gaveUpVerdict is the one Stalled verdict that deliberately bypasses the
// connection guards. When the operator capped reconnection (MaxReconnectAttempts
// > 0) and the cap was exhausted, the process is, by its own configuration, not
// going to recover on its own, and a restart is the cure. Held for minAge so a
// cap that is hit and immediately recovered from never triggers it.
//
// This trades the restart-storm guard for the cap: a broker outage longer than
// MaxReconnectAttempts x ReconnectDelay makes every capped client Stalled.
// Leave MaxReconnectAttempts at 0 (unlimited) if that is not wanted.
func gaveUpVerdict(cs ClientState, minAge time.Duration, now time.Time) (HealthLevel, string, bool) {
	if !cs.GaveUp {
		return HealthOK, "", false
	}
	held := now.Sub(cs.GaveUpAt)
	if held < minAge {
		return HealthDegraded, fmt.Sprintf("client exhausted its reconnect attempts %s ago (stalled after %s)", held.Round(time.Second), minAge), true
	}
	return HealthStalled, fmt.Sprintf("client gave up reconnecting %s ago (MaxReconnectAttempts exhausted, last error %q); a restart is the cure", held.Round(time.Second), cs.LastError), true
}
