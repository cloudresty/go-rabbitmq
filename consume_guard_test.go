package rabbitmq

import (
	"errors"
	"strings"
	"testing"
)

func TestSharedConsumeDecision(t *testing.T) {
	const allow = "MY_ALLOW"
	tests := []struct {
		name    string
		hosts   []string
		env     map[string]string
		wantErr []string // nil = allowed; else the refused hosts
	}{
		{"in kubernetes", []string{"rabbit.example.com:5672"}, map[string]string{"KUBERNETES_SERVICE_HOST": "10.0.0.1"}, nil},
		{"local hosts", []string{"localhost:5672", "127.0.0.1", "[::1]:5672", "::1", "0.0.0.0:5672", "Host.Docker.Internal"}, nil, nil},
		{"empty hosts", nil, nil, nil},
		{"mixed list", []string{"localhost:5672", "rabbit.example.com:5672"}, nil, []string{"rabbit.example.com:5672"}},
		{"shared refused", []string{"rabbit.example.com"}, nil, []string{"rabbit.example.com"}},
		{"allow true", []string{"rabbit.example.com"}, map[string]string{allow: "true"}, nil},
		{"allow TRUE", []string{"rabbit.example.com"}, map[string]string{allow: "TRUE"}, nil},
		{"allow padded", []string{"rabbit.example.com"}, map[string]string{allow: " true "}, nil},
		{"allow other value", []string{"rabbit.example.com"}, map[string]string{allow: "yes"}, []string{"rabbit.example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sharedConsumeDecision(tt.hosts, allow, func(k string) string { return tt.env[k] })
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("want allowed, got %v", err)
				}
				return
			}
			var re *SharedBrokerRefusedError
			if !errors.As(err, &re) {
				t.Fatalf("want *SharedBrokerRefusedError, got %v", err)
			}
			if strings.Join(re.Hosts, ",") != strings.Join(tt.wantErr, ",") {
				t.Fatalf("hosts = %v, want %v", re.Hosts, tt.wantErr)
			}
			if !strings.Contains(err.Error(), allow) {
				t.Fatalf("message %q does not name %s", err, allow)
			}
		})
	}
}

func recordingGuard(got *[]string, err error) ConsumeGuard {
	return func(hosts []string) error {
		*got = hosts
		return err
	}
}

func TestNewConsumerGuard(t *testing.T) {
	stub := errors.New("nope")

	t.Run("guard error surfaces", func(t *testing.T) {
		var seen []string
		cfg := &clientConfig{Hosts: []string{"a:5672", "b:5672"}}
		if err := WithConsumeGuard(recordingGuard(&seen, stub))(cfg); err != nil {
			t.Fatal(err)
		}
		_, err := (&Client{config: cfg}).NewConsumer()
		if !errors.Is(err, stub) || !strings.Contains(err.Error(), "consume guard refused") {
			t.Fatalf("err = %v", err)
		}
		if strings.Join(seen, ",") != "a:5672,b:5672" {
			t.Fatalf("hosts = %v", seen)
		}
	})

	t.Run("url fallback", func(t *testing.T) {
		var seen []string
		cfg := &clientConfig{URL: "amqp://u:p@single.example.com:5672/"}
		_ = WithConsumeGuard(recordingGuard(&seen, stub))(cfg)
		_, _ = (&Client{config: cfg}).NewConsumer()
		if len(seen) != 1 || seen[0] != "single.example.com:5672" {
			t.Fatalf("hosts = %v", seen)
		}
	})

	t.Run("urls fallback preferred over url", func(t *testing.T) {
		var seen []string
		cfg := &clientConfig{URL: "amqp://x/", URLs: []string{"amqp://u:p@h1:5672/", "amqps://h2/"}}
		_ = WithConsumeGuard(recordingGuard(&seen, stub))(cfg)
		_, _ = (&Client{config: cfg}).NewConsumer()
		if strings.Join(seen, ",") != "h1:5672,h2" {
			t.Fatalf("hosts = %v", seen)
		}
	})

	t.Run("no option means no guard", func(t *testing.T) {
		cfg := &clientConfig{Hosts: []string{"a"}}
		if cfg.ConsumeGuard != nil {
			t.Fatal("guard set by default")
		}
		if err := (&Client{config: cfg}).checkConsumeGuard(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("FromEnvWithPrefix hosts reach the guard", func(t *testing.T) {
		t.Setenv("GUARDTEST_RABBITMQ_HOSTS", "env1.example.com:5672,env2.example.com:5672")
		var seen []string
		cfg := &clientConfig{}
		if err := FromEnvWithPrefix("GUARDTEST_")(cfg); err != nil {
			t.Fatal(err)
		}
		_ = WithConsumeGuard(recordingGuard(&seen, stub))(cfg)
		_, err := (&Client{config: cfg}).NewConsumer()
		if !errors.Is(err, stub) {
			t.Fatalf("err = %v", err)
		}
		if strings.Join(seen, ",") != "env1.example.com:5672,env2.example.com:5672" {
			t.Fatalf("hosts = %v", seen)
		}
	})
}
