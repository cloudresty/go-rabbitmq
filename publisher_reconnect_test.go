package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Integration tests for the delivery-assurance confirm channel surviving a
// broker-side connection loss. They need a RabbitMQ with the management plugin
// (the CI service container already is one): the broker is reached on
// localhost:5672 like every other integration test here, and connections are
// killed through the management HTTP API (override with RABBITMQ_TEST_MGMT_URL).

// mgmtHTTP never keeps connections alive, so management-API calls leave no idle
// transport goroutines behind to blur the goroutine-leak measurement.
var mgmtHTTP = &http.Client{
	Timeout:   10 * time.Second,
	Transport: &http.Transport{DisableKeepAlives: true},
}

func mgmtURL() string {
	if v := os.Getenv("RABBITMQ_TEST_MGMT_URL"); v != "" {
		return v
	}
	return "http://localhost:15672"
}

type mgmtConn struct {
	Name             string `json:"name"`
	ClientProperties struct {
		ConnectionName string `json:"connection_name"`
	} `json:"client_properties"`
}

// brokerConnections lists the broker's connections opened with connectionName.
func brokerConnections(t *testing.T, connectionName string) ([]string, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, mgmtURL()+"/api/connections", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("guest", "guest")
	resp, err := mgmtHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("management API: %s", resp.Status)
	}
	var conns []mgmtConn
	if err := json.NewDecoder(resp.Body).Decode(&conns); err != nil {
		return nil, err
	}
	var names []string
	for _, c := range conns {
		if c.ClientProperties.ConnectionName == connectionName {
			names = append(names, c.Name)
		}
	}
	return names, nil
}

// killBrokerConnection force-closes every broker connection opened with
// connectionName and returns once the client has re-established a new one.
func killBrokerConnection(t *testing.T, connectionName string) {
	t.Helper()

	var victims []string
	deadline := time.Now().Add(10 * time.Second)
	for len(victims) == 0 {
		var err error
		victims, err = brokerConnections(t, connectionName)
		if err != nil {
			t.Skipf("RabbitMQ management API not available: %v", err)
		}
		if len(victims) == 0 {
			if time.Now().After(deadline) {
				t.Fatalf("no broker connection named %q found", connectionName)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	killed := map[string]bool{}
	for _, name := range victims {
		req, err := http.NewRequest(http.MethodDelete, mgmtURL()+"/api/connections/"+url.PathEscape(name), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.SetBasicAuth("guest", "guest")
		req.Header.Set("X-Reason", "go-rabbitmq reconnect test")
		resp, err := mgmtHTTP.Do(req)
		if err != nil {
			t.Fatalf("closing connection %q: %v", name, err)
		}
		_ = resp.Body.Close()
		killed[name] = true
	}

	// Wait for the client's connection monitor to dial a replacement.
	deadline = time.Now().Add(20 * time.Second)
	for {
		now, err := brokerConnections(t, connectionName)
		if err == nil {
			for _, name := range now {
				if !killed[name] {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("client did not reconnect after connection %v was closed", victims)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// awaitWindow is how long a test waits for an expected callback. It must stay
// below deliveryTimeout so that a delivery timeout can never pass as success.
const (
	awaitWindow     = 10 * time.Second
	deliveryTimeout = 20 * time.Second
)

type outcomeRecorder struct {
	mu sync.Mutex
	ch map[string]chan deliveryResult
}

type deliveryResult struct {
	outcome DeliveryOutcome
	errMsg  string
}

func newOutcomeRecorder() *outcomeRecorder {
	return &outcomeRecorder{ch: make(map[string]chan deliveryResult)}
}

func (r *outcomeRecorder) chanFor(id string) chan deliveryResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.ch[id]
	if !ok {
		c = make(chan deliveryResult, 8)
		r.ch[id] = c
	}
	return c
}

func (r *outcomeRecorder) callback(id string, outcome DeliveryOutcome, errMsg string) {
	r.chanFor(id) <- deliveryResult{outcome, errMsg}
}

// await waits for the (single) callback of id.
func (r *outcomeRecorder) await(t *testing.T, id string, within time.Duration) deliveryResult {
	t.Helper()
	select {
	case res := <-r.chanFor(id):
		return res
	case <-time.After(within):
		t.Fatalf("no delivery callback for %q within %v", id, within)
		return deliveryResult{}
	}
}

type reconnectFixture struct {
	client     *Client
	publisher  *Publisher
	rec        *outcomeRecorder
	connName   string
	exchange   string
	routingKey string
}

// newReconnectFixture builds a client + delivery-assurance publisher over a
// routable exchange/queue pair. Delivery timeout is deliberately longer than
// every "promptly" assertion below, so a timeout can never pass as success.
func newReconnectFixture(t *testing.T) *reconnectFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	f := &reconnectFixture{
		connName:   "reconnect-test-" + suffix,
		exchange:   "reconnect-test-ex-" + suffix,
		routingKey: "rk",
		rec:        newOutcomeRecorder(),
	}

	client, err := NewClient(
		WithHosts("localhost:5672"),
		WithCredentials("guest", "guest"),
		WithConnectionName(f.connName),
		WithReconnectDelay(200*time.Millisecond),
	)
	if err != nil {
		t.Skip("RabbitMQ not available for testing")
	}
	f.client = client
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	admin := client.Admin()
	if err := admin.DeclareExchange(ctx, f.exchange, ExchangeTypeDirect); err != nil {
		t.Fatalf("declare exchange: %v", err)
	}
	queue, err := admin.DeclareQueue(ctx, "reconnect-test-q-"+suffix)
	if err != nil {
		t.Fatalf("declare queue: %v", err)
	}
	if err := admin.BindQueue(ctx, queue.Name, f.exchange, f.routingKey); err != nil {
		t.Fatalf("bind queue: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.DeleteQueue(context.Background(), queue.Name)
		_ = admin.DeleteExchange(context.Background(), f.exchange)
	})

	publisher, err := client.NewPublisher(
		WithDeliveryAssurance(),
		WithDefaultDeliveryCallback(f.rec.callback),
		WithDeliveryTimeout(deliveryTimeout),
	)
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	f.publisher = publisher
	t.Cleanup(func() { _ = publisher.Close() })
	return f
}

func (f *reconnectFixture) publish(t *testing.T, id string) {
	t.Helper()
	err := f.publisher.PublishWithDeliveryAssurance(context.Background(), f.exchange, f.routingKey,
		NewMessage([]byte("payload "+id)), DeliveryOptions{MessageID: id, Mandatory: true})
	if err != nil {
		t.Fatalf("publish %q: %v", id, err)
	}
}

// TestDeliveryAssurance_ConfirmsSurviveReconnect is the regression test for the
// 2026-10-04 production incident: after the broker dropped the connection, the
// publisher reopened its confirm channel but nothing read the new channel's
// confirms, so every delivery callback reported a timeout until restart.
func TestDeliveryAssurance_ConfirmsSurviveReconnect(t *testing.T) {
	f := newReconnectFixture(t)

	f.publish(t, "before-1")
	if res := f.rec.await(t, "before-1", awaitWindow); res.outcome != DeliverySuccess {
		t.Fatalf("before reconnect: got %s (%s), want success", res.outcome, res.errMsg)
	}

	killBrokerConnection(t, f.connName)

	// Well under the 8s delivery timeout: a timeout outcome cannot sneak in.
	f.publish(t, "after-1")
	if res := f.rec.await(t, "after-1", awaitWindow); res.outcome != DeliverySuccess {
		t.Fatalf("after reconnect: got %s (%s), want success", res.outcome, res.errMsg)
	}

	// And it keeps working, not just once.
	f.publish(t, "after-2")
	if res := f.rec.await(t, "after-2", awaitWindow); res.outcome != DeliverySuccess {
		t.Fatalf("second publish after reconnect: got %s (%s), want success", res.outcome, res.errMsg)
	}
}

// TestDeliveryAssurance_TagsStayAlignedAfterEarlyReturns checks that publishes
// which return before reaching the wire do not shift the library's delivery
// tags against the broker's. Previously the tag was allocated before the
// duplicate-MessageID check and before the publish could fail, so the next real
// publish was filed under a tag the broker never used and timed out.
func TestDeliveryAssurance_TagsStayAlignedAfterEarlyReturns(t *testing.T) {
	f := newReconnectFixture(t)

	// An in-flight message that stays pending (its pending entry is registered on
	// the current generation, as a real publish would, but never confirmed), so a
	// second publish with the same ID is a deterministic duplicate.
	gen := f.publisher.currentConfirmGeneration()
	held := &pendingMessage{MessageID: "held", PublishedAt: time.Now(), key: deliveryKey(gen.id, 999999), generation: gen.id}
	if !f.publisher.pendingMessages.Store("held", held.key, held) {
		t.Fatal("could not register the held message")
	}
	defer f.publisher.pendingMessages.Delete("held", held.key)

	var err error
	for i := 0; i < 3; i++ {
		err = f.publisher.PublishWithDeliveryAssurance(context.Background(), f.exchange, f.routingKey,
			NewMessage([]byte("dup")), DeliveryOptions{MessageID: "held", Mandatory: true})
		if err == nil {
			t.Fatal("duplicate MessageID was accepted")
		}
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for i := 0; i < 3; i++ {
		err = f.publisher.PublishWithDeliveryAssurance(cancelled, f.exchange, f.routingKey,
			NewMessage([]byte("cancelled")), DeliveryOptions{MessageID: fmt.Sprintf("cancelled-%d", i), Mandatory: true})
		if err == nil {
			t.Fatal("publish with a cancelled context succeeded")
		}
	}

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("normal-%d", i)
		f.publish(t, id)
		res := f.rec.await(t, id, awaitWindow)
		if res.outcome != DeliverySuccess {
			t.Fatalf("%s: got %s (%s), want success", id, res.outcome, res.errMsg)
		}
	}

	stats := f.publisher.GetDeliveryStats()
	if stats.TotalTimedOut != 0 {
		t.Errorf("TotalTimedOut = %d, want 0", stats.TotalTimedOut)
	}
}

// TestDeliveryAssurance_PendingSettledWhenChannelReplaced checks that messages
// still pending on a channel that dies are settled with a non-success outcome
// right away, instead of lingering until their delivery timeout.
func TestDeliveryAssurance_PendingSettledWhenChannelReplaced(t *testing.T) {
	f := newReconnectFixture(t)
	p := f.publisher

	// An in-flight message whose confirm can never arrive: registered on the
	// current channel generation as a real publish would, but never sent.
	gen := p.currentConfirmGeneration()
	const id = "orphaned"
	pending := &pendingMessage{
		MessageID:   id,
		PublishedAt: time.Now(),
		Callback:    f.rec.callback,
		key:         deliveryKey(gen.id, 4242),
		brokerTag:   4242,
		generation:  gen.id,
	}
	pending.TimeoutTimer = time.AfterFunc(time.Minute, func() {})
	defer pending.TimeoutTimer.Stop()
	if !p.pendingMessages.Store(id, pending.key, pending) {
		t.Fatal("could not register pending message")
	}

	killBrokerConnection(t, f.connName)

	res := f.rec.await(t, id, awaitWindow)
	if res.outcome == DeliverySuccess || res.outcome == DeliveryTimeout {
		t.Fatalf("orphaned message settled as %s (%s), want a prompt non-success, non-timeout outcome", res.outcome, res.errMsg)
	}
	if res.errMsg == "" {
		t.Error("orphaned message settled without an explanation")
	}
	if got := p.GetDeliveryStats().TotalOrphaned; got != 1 {
		t.Errorf("TotalOrphaned = %d, want 1", got)
	}
}

// TestDeliveryAssurance_NoGoroutineLeakAcrossRefreshes drives several
// connection losses and checks that reader goroutines of dead channels exit,
// and that Close reclaims the rest.
func TestDeliveryAssurance_NoGoroutineLeakAcrossRefreshes(t *testing.T) {
	f := newReconnectFixture(t)

	// One full kill/reconnect cycle first, so lazily started machinery (the
	// reconnected client's goroutines) is part of the baseline.
	f.publish(t, "warm-up")
	f.rec.await(t, "warm-up", awaitWindow)
	killBrokerConnection(t, f.connName)
	f.publish(t, "warm-up-2")
	f.rec.await(t, "warm-up-2", awaitWindow)
	steady := settledGoroutines()

	const (
		cycles    = 5
		tolerance = 4 // a leak of one reader pair per refresh would add 2*cycles = 10
	)
	for i := 0; i < cycles; i++ {
		killBrokerConnection(t, f.connName)
		id := fmt.Sprintf("cycle-%d", i)
		f.publish(t, id)
		if res := f.rec.await(t, id, awaitWindow); res.outcome != DeliverySuccess {
			t.Fatalf("cycle %d: got %s (%s), want success", i, res.outcome, res.errMsg)
		}
	}

	after := settledGoroutines()
	if after > steady+tolerance {
		t.Fatalf("goroutines grew from %d to %d across %d refreshes", steady, after, cycles)
	}

	if err := f.publisher.Close(); err != nil {
		t.Fatalf("close publisher: %v", err)
	}
	if closed := settledGoroutines(); closed > steady+tolerance {
		t.Errorf("goroutines after Close = %d, want at most %d (steady %d)", closed, steady+tolerance, steady)
	}
}

// TestDeliveryAssurance_FailureReasonIsReported publishes to an exchange that
// does not exist: the broker closes the channel with 404 NOT_FOUND. The message
// is orphaned, and the reason the broker gave must reach the callback; the
// publisher must then recover on its own.
func TestDeliveryAssurance_FailureReasonIsReported(t *testing.T) {
	f := newReconnectFixture(t)

	err := f.publisher.PublishWithDeliveryAssurance(context.Background(), "no-such-exchange-"+f.exchange, "rk",
		NewMessage([]byte("lost")), DeliveryOptions{MessageID: "to-nowhere", Mandatory: true})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	res := f.rec.await(t, "to-nowhere", awaitWindow)
	if res.outcome != DeliveryFailed {
		t.Fatalf("got %s (%s), want failed", res.outcome, res.errMsg)
	}
	if !strings.Contains(res.errMsg, "404") || !strings.Contains(res.errMsg, "NOT_FOUND") {
		t.Errorf("error %q does not carry the broker's close reason", res.errMsg)
	}
	if !strings.Contains(res.errMsg, "unknown") {
		t.Errorf("error %q does not say the delivery state is unknown", res.errMsg)
	}

	f.publish(t, "after-404")
	if r := f.rec.await(t, "after-404", awaitWindow); r.outcome != DeliverySuccess {
		t.Fatalf("after 404: got %s (%s), want success", r.outcome, r.errMsg)
	}
}

// TestDeliveryAssurance_CallbackMayRepublishSameID: the pending entry is removed
// before the callback runs, so a callback can republish under the same
// MessageID, and each publish still gets exactly one callback.
func TestDeliveryAssurance_CallbackMayRepublishSameID(t *testing.T) {
	f := newReconnectFixture(t)

	var calls atomic.Int32
	republishErr := make(chan error, 1)
	second := make(chan DeliveryOutcome, 2)

	var cb DeliveryCallback
	cb = func(id string, outcome DeliveryOutcome, _ string) {
		if calls.Add(1) > 1 {
			second <- outcome
			return
		}
		republishErr <- f.publisher.PublishWithDeliveryAssurance(context.Background(), f.exchange, f.routingKey,
			NewMessage([]byte("again")), DeliveryOptions{MessageID: id, Mandatory: true, Callback: cb})
	}

	err := f.publisher.PublishWithDeliveryAssurance(context.Background(), f.exchange, f.routingKey,
		NewMessage([]byte("once")), DeliveryOptions{MessageID: "same-id", Mandatory: true, Callback: cb})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case err := <-republishErr:
		if err != nil {
			t.Fatalf("republish from the callback was rejected: %v", err)
		}
	case <-time.After(awaitWindow):
		t.Fatal("first callback never ran")
	}
	select {
	case o := <-second:
		if o != DeliverySuccess {
			t.Fatalf("second delivery: got %s, want success", o)
		}
	case <-time.After(awaitWindow):
		t.Fatal("second callback never ran")
	}

	time.Sleep(500 * time.Millisecond)
	if got := calls.Load(); got != 2 {
		t.Errorf("callbacks = %d, want exactly 2 (one per publish)", got)
	}
}

// TestDeliveryAssurance_CloseDoesNotQueueBehindBlockedPublish simulates a publish
// stuck in a socket write while holding the publisher's internal lock: only
// closing its channel releases it. Close must close the channel first and so
// return promptly, rather than wait for a lock only it can release.
func TestDeliveryAssurance_CloseDoesNotQueueBehindBlockedPublish(t *testing.T) {
	f := newReconnectFixture(t)
	p := f.publisher
	gen := p.currentConfirmGeneration()

	p.publishMu.Lock()
	go func() {
		defer p.publishMu.Unlock()
		for !gen.ch.IsClosed() { // released only by the channel being closed
			time.Sleep(10 * time.Millisecond)
		}
	}()

	done := make(chan error, 1)
	go func() { done <- p.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not return while a publish was blocked holding the publish lock")
	}

	// Idempotent.
	if err := p.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func newUnitPublisher() *Publisher {
	c := &Client{config: &clientConfig{Logger: NewNopLogger(), Metrics: NewNopMetrics()}}
	return &Publisher{
		client:                   c,
		config:                   &publisherConfig{},
		deliveryAssuranceEnabled: true,
		pendingMessages:          newShardedPendingMap(),
		shutdownChan:             make(chan struct{}),
	}
}

// TestFailOrphans_DoesNotNeedPublishMu: orphans must be settled even while a
// publish (blocked on the wire) or a refresh holds publishMu, which is exactly
// the situation during an outage. Needs no broker.
func TestFailOrphans_DoesNotNeedPublishMu(t *testing.T) {
	p := newUnitPublisher()

	gen := &confirmGeneration{id: 1, returnsDone: make(chan struct{}), closes: make(chan *amqp.Error, 1)}
	close(gen.returnsDone)
	gen.closes <- &amqp.Error{Code: 404, Reason: "NOT_FOUND - no exchange 'x'"}

	got := make(chan string, 1)
	pending := &pendingMessage{
		MessageID:   "orphan",
		PublishedAt: time.Now(),
		key:         deliveryKey(1, 1),
		brokerTag:   1,
		generation:  1,
		Callback: func(_ string, outcome DeliveryOutcome, msg string) {
			got <- fmt.Sprintf("%s|%s", outcome, msg)
		},
	}
	p.pendingMessages.Store("orphan", pending.key, pending)

	p.publishMu.Lock() // a publish or refresh is in progress and not finishing
	defer p.publishMu.Unlock()

	go p.failOrphans(gen)

	select {
	case v := <-got:
		if !strings.HasPrefix(v, "failed|") || !strings.Contains(v, "404") {
			t.Errorf("unexpected settlement %q", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("orphan not settled while publishMu was held")
	}
	if n := p.GetDeliveryStats().TotalOrphaned; n != 1 {
		t.Errorf("TotalOrphaned = %d, want 1", n)
	}
}

// TestRekeyPending covers the defensive resynchronisation when the channel's tag
// differs from the one predicted: the entry must move to the real tag and the
// old key must stop resolving.
func TestRekeyPending(t *testing.T) {
	p := newUnitPublisher()
	pending := &pendingMessage{MessageID: "m", key: deliveryKey(3, 5), brokerTag: 5, generation: 3}
	p.pendingMessages.Store("m", pending.key, pending)

	p.rekeyPending(pending, 3, 6, time.Minute)
	defer pending.TimeoutTimer.Stop()

	if _, ok := p.pendingMessages.LoadByDeliveryTag(deliveryKey(3, 5)); ok {
		t.Error("old key still resolves")
	}
	if got, ok := p.pendingMessages.LoadByDeliveryTag(deliveryKey(3, 6)); !ok || got != pending {
		t.Error("pending not reachable under the real tag")
	}
	if pending.key != deliveryKey(3, 6) || pending.brokerTag != 6 {
		t.Errorf("pending fields not updated: key=%d brokerTag=%d", pending.key, pending.brokerTag)
	}
	if _, ok := p.pendingMessages.LoadByMessageID("m"); !ok {
		t.Error("message id index lost the entry")
	}
}

// settledGoroutines returns the goroutine count once it has stopped changing.
func settledGoroutines() int {
	prev := -1
	for i := 0; i < 50; i++ {
		runtime.GC()
		n := runtime.NumGoroutine()
		if n == prev {
			return n
		}
		prev = n
		time.Sleep(100 * time.Millisecond)
	}
	return prev
}

// TestDeliveryKey_NoCollisionAcrossGenerations pins the property that makes tag
// reuse after a channel replacement harmless: the broker restarts delivery tags
// at 1 on every channel, and entries of an old channel must neither be matched
// by, nor deleted on behalf of, a message on its replacement.
func TestDeliveryKey_NoCollisionAcrossGenerations(t *testing.T) {
	m := newShardedPendingMap()
	oldMsg := &pendingMessage{MessageID: "old"}
	newMsg := &pendingMessage{MessageID: "new"}

	oldKey := deliveryKey(1, 1)
	newKey := deliveryKey(2, 1) // same broker tag, next channel generation
	if oldKey == newKey {
		t.Fatalf("keys for tag 1 of generations 1 and 2 collide: %d", oldKey)
	}

	if !m.Store("old", oldKey, oldMsg) || !m.Store("new", newKey, newMsg) {
		t.Fatal("Store rejected a message")
	}
	if got, _ := m.LoadByDeliveryTag(oldKey); got != oldMsg {
		t.Error("old key resolved to the wrong message")
	}
	if got, _ := m.LoadByDeliveryTag(newKey); got != newMsg {
		t.Error("new key resolved to the wrong message")
	}

	// Late cleanup of the old entry must leave the new one alone.
	m.Delete("old", oldKey)
	if got, ok := m.LoadByDeliveryTag(newKey); !ok || got != newMsg {
		t.Error("deleting the old channel's entry removed the new channel's entry")
	}
}

// countingRecorder records every callback for one message id, so a test can
// assert both what was reported and that nothing else was.
type countingRecorder struct {
	mu   sync.Mutex
	seen []deliveryResult
}

func (c *countingRecorder) callback(_ string, outcome DeliveryOutcome, msg string) {
	c.mu.Lock()
	c.seen = append(c.seen, deliveryResult{outcome, msg})
	c.mu.Unlock()
}

func (c *countingRecorder) results() []deliveryResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]deliveryResult(nil), c.seen...)
}

// TestDeliveryAssurance_FailedSendThenRetryYieldsOneCallback covers the retry
// loop reusing nothing from a failed attempt, and the single-signal contract when
// a sweep lands while the send is in flight. The first attempt registers, is
// "swept" (as failOrphans would) and then fails because its channel is gone: the
// call must return no error, the second attempt must be tracked from scratch and
// confirmed, and the message must get exactly one callback, a success. With a
// reused pendingMessage the second attempt's confirm is ignored and there is no
// callback at all; with an unguarded sweep the first attempt also reports Failed.
func TestDeliveryAssurance_FailedSendThenRetryYieldsOneCallback(t *testing.T) {
	f := newReconnectFixture(t)
	p := f.publisher
	rec := &countingRecorder{}

	var once sync.Once
	p.testHookAfterRegister = func(pending *pendingMessage) {
		once.Do(func() {
			// The channel dies and its sweep runs while this send is in flight.
			_ = p.currentConfirmGeneration().ch.Close()
			p.settleOrphan(pending, " (channel closed: test)")
		})
	}

	err := p.PublishWithDeliveryAssurance(context.Background(), f.exchange, f.routingKey,
		NewMessage([]byte("retried")), DeliveryOptions{MessageID: "retried", Mandatory: true, Callback: rec.callback})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	deadline := time.Now().Add(awaitWindow)
	for len(rec.results()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(time.Second) // room for a (wrong) second callback
	got := rec.results()
	if len(got) != 1 || got[0].outcome != DeliverySuccess {
		t.Fatalf("callbacks = %+v, want exactly one success", got)
	}
	if _, stuck := p.pendingMessages.LoadByMessageID("retried"); stuck {
		t.Error("message left in the pending map after its callback")
	}
}

// TestDeliveryAssurance_SweepDuringSendAfterSuccessfulSend: the sweep lands while
// the send is in flight but the send itself succeeds. The publisher settles the
// message itself: nil error and exactly one Failed callback with the reason.
func TestDeliveryAssurance_SweepDuringSendAfterSuccessfulSend(t *testing.T) {
	f := newReconnectFixture(t)
	p := f.publisher
	rec := &countingRecorder{}

	var once sync.Once
	p.testHookAfterRegister = func(pending *pendingMessage) {
		once.Do(func() { p.settleOrphan(pending, " (channel closed: test)") })
	}

	err := p.PublishWithDeliveryAssurance(context.Background(), f.exchange, f.routingKey,
		NewMessage([]byte("swept")), DeliveryOptions{MessageID: "swept", Mandatory: true, Callback: rec.callback})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	time.Sleep(2 * time.Second)
	got := rec.results()
	// The real broker also confirms the message; whichever finishes it, the
	// contract is one callback.
	if len(got) != 1 {
		t.Fatalf("callbacks = %+v, want exactly one", got)
	}
}

// TestRekeyPending_SettledEntryNotResurrected: an entry that already has its
// outcome must not be re-stored under a new key.
func TestRekeyPending_SettledEntryNotResurrected(t *testing.T) {
	p := newUnitPublisher()
	pending := &pendingMessage{MessageID: "m", key: deliveryKey(3, 5), brokerTag: 5, generation: 3, CallbackFired: true}

	p.rekeyPending(pending, 3, 6, time.Minute)

	if _, ok := p.pendingMessages.LoadByDeliveryTag(deliveryKey(3, 6)); ok {
		t.Error("settled entry was stored under a new key")
	}
	if pending.key != deliveryKey(3, 5) {
		t.Error("settled entry was re-keyed")
	}
}
