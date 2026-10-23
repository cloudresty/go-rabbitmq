package rabbitmq

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type captureLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *captureLogger) log(msg string, f ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, msg+" "+fmt.Sprint(f...))
}
func (l *captureLogger) Debug(m string, f ...any) { l.log(m, f...) }
func (l *captureLogger) Info(m string, f ...any)  { l.log(m, f...) }
func (l *captureLogger) Warn(m string, f ...any)  { l.log(m, f...) }
func (l *captureLogger) Error(m string, f ...any) { l.log(m, f...) }

func TestRedactURL(t *testing.T) {
	got := redactURL("amqp://user:s3cretpw@host:5672/")
	if strings.Contains(got, "s3cretpw") || !strings.Contains(got, "user") || !strings.Contains(got, "host:5672") {
		t.Fatalf("got %q", got)
	}
	if got := redactURL("://bad\x7f"); strings.Contains(got, "bad") {
		t.Fatalf("unparseable leaked: %q", got)
	}
}

func TestConnectLogsNeverContainPassword(t *testing.T) {
	lg := &captureLogger{}
	cfg := &clientConfig{
		URL:         "amqp://user:s3cretpw@127.0.0.1:1/",
		URLs:        []string{"amqp://user:s3cretpw@127.0.0.1:1/"},
		Logger:      lg,
		Metrics:     NewNopMetrics(),
		DialTimeout: time.Second,
	}
	c := &Client{config: cfg}
	if err := c.connect(); err == nil {
		t.Fatal("expected dial failure")
	}
	if len(lg.lines) == 0 {
		t.Fatal("nothing logged")
	}
	for _, l := range lg.lines {
		if strings.Contains(l, "s3cretpw") {
			t.Fatalf("password logged: %s", l)
		}
	}
}
