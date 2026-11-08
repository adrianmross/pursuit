package vercelapp

import (
	"reflect"
	"testing"
)

func TestWebConnectOrigins(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cases := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{"unset", nil, nil},
		{"sentry only", map[string]string{"PURSUIT_WEB_SENTRY_DSN": "https://key@o1.ingest.us.sentry.io/2"}, []string{"https://o1.ingest.us.sentry.io"}},
		{"posthog default host", map[string]string{"PURSUIT_POSTHOG_KEY": "phc_x"}, []string{"https://us.i.posthog.com"}},
		{"posthog host without key", map[string]string{"PURSUIT_POSTHOG_HOST": "https://eu.i.posthog.com"}, nil},
		{"both", map[string]string{"PURSUIT_WEB_SENTRY_DSN": "https://key@o1.ingest.us.sentry.io/2", "PURSUIT_POSTHOG_KEY": "phc_x", "PURSUIT_POSTHOG_HOST": "https://eu.i.posthog.com/"}, []string{"https://o1.ingest.us.sentry.io", "https://eu.i.posthog.com"}},
	}
	for _, c := range cases {
		if got := webConnectOrigins(env(c.env)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
