package rabbitmq

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ConsumeGuard is an opt-in check run by (*Client).NewConsumer before any
// channel is opened. It receives the broker hosts the client is configured for
// and returns a non-nil error to refuse creating consumers. No guard is
// installed by default; see WithConsumeGuard. Only NewConsumer is guarded:
// CreateChannel and the Admin API are not, and publishers are unaffected.
type ConsumeGuard func(hosts []string) error

// WithConsumeGuard installs a guard checked in (*Client).NewConsumer. Only
// NewConsumer is guarded; CreateChannel and the Admin API are not, and
// publishers are unaffected. The guard sees the union of URLs, URL and Hosts.
// A nil guard (the default) disables the check.
func WithConsumeGuard(g ConsumeGuard) Option {
	return func(config *clientConfig) error {
		config.ConsumeGuard = g
		return nil
	}
}

// checkConsumeGuard runs the configured guard, if any.
func (c *Client) checkConsumeGuard() error {
	if c.config == nil || c.config.ConsumeGuard == nil {
		return nil
	}
	if err := c.config.ConsumeGuard(guardHosts(c.config)); err != nil {
		return fmt.Errorf("consume guard refused: %w", err)
	}
	return nil
}

// guardHosts returns every broker the client may dial: the union of URLs, URL
// and Hosts (connect() tries them in that order), de-duplicated. Checking only
// one of them would let a local Hosts entry hide a shared broker in URL.
func guardHosts(config *clientConfig) []string {
	var hosts []string
	seen := map[string]bool{}
	add := func(h string) {
		if h != "" && !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}
	urls := append(append([]string{}, config.URLs...), config.URL)
	for _, raw := range urls {
		if raw == "" {
			continue
		}
		if u, err := url.Parse(raw); err == nil {
			add(u.Host)
		}
	}
	for _, h := range config.Hosts {
		add(h)
	}
	return hosts
}

// SharedBrokerRefusedError is returned by the guard from
// RefuseSharedBrokerOutsideKubernetes when consuming is refused.
type SharedBrokerRefusedError struct {
	// Hosts are the non-local hosts that triggered the refusal.
	Hosts []string
	// AllowEnv is the environment variable that overrides the refusal.
	AllowEnv string
}

func (e *SharedBrokerRefusedError) Error() string {
	return "refusing to consume: not running in Kubernetes and the broker is not local (" +
		strings.Join(e.Hosts, ",") + "); a local run would compete with the deployed consumers for shared queues. " +
		"Point at a local broker, or set " + e.AllowEnv + "=true to consume deliberately"
}

// RefuseSharedBrokerOutsideKubernetes returns a ConsumeGuard that refuses
// consuming from a non-local broker when the process is not running in
// Kubernetes, unless the allowEnv environment variable is "true"
// (case-insensitive, trimmed). Allowed when KUBERNETES_SERVICE_HOST is set or
// every host is local (localhost, 127.0.0.1, ::1, 0.0.0.0, host.docker.internal).
func RefuseSharedBrokerOutsideKubernetes(allowEnv string) ConsumeGuard {
	return func(hosts []string) error {
		return sharedConsumeDecision(hosts, allowEnv, os.Getenv)
	}
}

func sharedConsumeDecision(hosts []string, allowEnv string, getenv func(string) string) error {
	if getenv("KUBERNETES_SERVICE_HOST") != "" {
		return nil
	}
	shared := nonLocalHosts(hosts)
	if len(shared) == 0 {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(getenv(allowEnv)), "true") {
		return nil
	}
	return &SharedBrokerRefusedError{Hosts: shared, AllowEnv: allowEnv}
}

// nonLocalHosts returns the entries of hosts that are not local. Entries may be
// host, host:port, [v6] or [v6]:port; an empty list yields nothing.
func nonLocalHosts(hosts []string) []string {
	var shared []string
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		name := h
		if strings.HasPrefix(name, "[") {
			if end := strings.Index(name, "]"); end > 0 {
				name = name[1:end]
			}
		} else if strings.Count(name, ":") == 1 {
			name = name[:strings.Index(name, ":")]
		}
		switch strings.ToLower(name) {
		case "localhost", "127.0.0.1", "::1", "0.0.0.0", "host.docker.internal":
		default:
			shared = append(shared, h)
		}
	}
	return shared
}
