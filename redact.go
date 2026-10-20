package rabbitmq

import "net/url"

// redactURL returns raw with the userinfo password masked, safe for logs.
// An unparseable value is replaced entirely, as it may still hold a secret.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<unparseable url>"
	}
	return u.Redacted()
}
