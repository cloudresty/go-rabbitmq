package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
)

// Integration tests for the delivery-assurance confirm channel surviving a
// broker-side connection loss. They need a RabbitMQ with the management plugin
// (the CI service container already is one): the broker is reached on
// localhost:5672 like every other integration test here, and connections are
// killed through the management HTTP API (override with RABBITMQ_TEST_MGMT_URL).

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
	resp, err := http.DefaultClient.Do(req)
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
		resp, err := http.DefaultClient.Do(req)
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
		WithDeliveryTimeout(8*time.Second),
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
	if res := f.rec.await(t, "before-1", 3*time.Second); res.outcome != DeliverySuccess {
		t.Fatalf("before reconnect: got %s (%s), want success", res.outcome, res.errMsg)
	}

	killBrokerConnection(t, f.connName)

	// Well under the 8s delivery timeout: a timeout outcome cannot sneak in.
	f.publish(t, "after-1")
	if res := f.rec.await(t, "after-1", 3*time.Second); res.outcome != DeliverySuccess {
		t.Fatalf("after reconnect: got %s (%s), want success", res.outcome, res.errMsg)
	}

	// And it keeps working, not just once.
	f.publish(t, "after-2")
	if res := f.rec.await(t, "after-2", 3*time.Second); res.outcome != DeliverySuccess {
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

	// Keep "held" pending by blocking its callback: the pending entry is only
	// removed after the callback returns, so a second publish with the same ID is
	// a deterministic duplicate.
	release := make(chan struct{})
	held := make(chan DeliveryOutcome, 1)
	err := f.publisher.PublishWithDeliveryAssurance(context.Background(), f.exchange, f.routingKey,
		NewMessage([]byte("held")), DeliveryOptions{
			MessageID: "held",
			Mandatory: true,
			Callback: func(_ string, outcome DeliveryOutcome, _ string) {
				held <- outcome
				<-release
			},
		})
	if err != nil {
		t.Fatalf("publish held: %v", err)
	}
	select {
	case o := <-held:
		if o != DeliverySuccess {
			t.Fatalf("held: got %s, want success", o)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("held message never confirmed")
	}

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
		res := f.rec.await(t, id, 3*time.Second)
		if res.outcome != DeliverySuccess {
			t.Fatalf("%s: got %s (%s), want success", id, res.outcome, res.errMsg)
		}
	}
	close(release)

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
		DeliveryTag: deliveryKey(gen.id, 4242),
		generation:  gen.id,
	}
	pending.TimeoutTimer = time.AfterFunc(time.Minute, func() {})
	defer pending.TimeoutTimer.Stop()
	if !p.pendingMessages.Store(id, pending.DeliveryTag, pending) {
		t.Fatal("could not register pending message")
	}

	killBrokerConnection(t, f.connName)

	res := f.rec.await(t, id, 3*time.Second)
	if res.outcome == DeliverySuccess || res.outcome == DeliveryTimeout {
		t.Fatalf("orphaned message settled as %s (%s), want a prompt non-success, non-timeout outcome", res.outcome, res.errMsg)
	}
	if res.errMsg == "" {
		t.Error("orphaned message settled without an explanation")
	}
}

// TestDeliveryAssurance_NoGoroutineLeakAcrossRefreshes drives several
// connection losses and checks that reader goroutines of dead channels exit,
// and that Close reclaims the rest.
func TestDeliveryAssurance_NoGoroutineLeakAcrossRefreshes(t *testing.T) {
	f := newReconnectFixture(t)

	f.publish(t, "warm-up")
	f.rec.await(t, "warm-up", 3*time.Second)
	steady := settledGoroutines()

	const cycles = 5
	for i := 0; i < cycles; i++ {
		killBrokerConnection(t, f.connName)
		id := fmt.Sprintf("cycle-%d", i)
		f.publish(t, id)
		if res := f.rec.await(t, id, 3*time.Second); res.outcome != DeliverySuccess {
			t.Fatalf("cycle %d: got %s (%s), want success", i, res.outcome, res.errMsg)
		}
	}

	after := settledGoroutines()
	// A leak of one reader pair per refresh would add 2*cycles goroutines.
	if after > steady+3 {
		t.Fatalf("goroutines grew from %d to %d across %d refreshes", steady, after, cycles)
	}

	if err := f.publisher.Close(); err != nil {
		t.Fatalf("close publisher: %v", err)
	}
	closed := settledGoroutines()
	if closed > after-2 {
		t.Errorf("Close did not stop the readers: %d goroutines before, %d after", after, closed)
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
