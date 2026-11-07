package vercelapp

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/pursuit/pursuit/internal/version"
)

// sentryHook reports panics and 5xx responses when PURSUIT_SENTRY_DSN is set.
// It returns nil otherwise, so self-hosted and preview builds send nothing.
// Events carry the route, status, request id and stack, never headers,
// cookies, query strings or bodies.
func sentryHook(log *slog.Logger) func(*http.Request, string, int, any) {
	dsn := strings.TrimSpace(os.Getenv("PURSUIT_SENTRY_DSN"))
	if dsn == "" {
		return nil
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Release:     "pursuit@" + version.Version,
		Environment: sentryEnv(),
		BeforeSend: func(ev *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			if ev.Request != nil {
				ev.Request = &sentry.Request{Method: ev.Request.Method, URL: ev.Request.URL}
			}
			ev.User = sentry.User{}
			ev.ServerName = ""
			return ev
		},
	})
	if err != nil {
		log.Warn("sentry disabled", "err", err.Error())
		return nil
	}
	return func(r *http.Request, requestID string, status int, v any) {
		hub := sentry.CurrentHub().Clone()
		hub.WithScope(func(sc *sentry.Scope) {
			sc.SetTag("request_id", requestID)
			sc.SetTag("status", fmt.Sprint(status))
			sc.SetTag("route", r.Method+" "+r.URL.Path)
			if v != nil {
				hub.Recover(v)
				return
			}
			hub.CaptureMessage(fmt.Sprintf("%s %s returned %d", r.Method, r.URL.Path, status))
		})
		// Serverless instances freeze after the response, so send before that.
		hub.Flush(2 * time.Second)
	}
}

// webConnectOrigins returns the origins the dashboard's browser telemetry
// posts to, for the CSP's connect-src. Empty when both are unset.
func webConnectOrigins(getenv func(string) string) []string {
	var out []string
	if u, err := url.Parse(strings.TrimSpace(getenv("PURSUIT_WEB_SENTRY_DSN"))); err == nil && u.Host != "" {
		out = append(out, u.Scheme+"://"+u.Host)
	}
	if strings.TrimSpace(getenv("PURSUIT_POSTHOG_KEY")) != "" {
		host := strings.TrimRight(strings.TrimSpace(getenv("PURSUIT_POSTHOG_HOST")), "/")
		if host == "" {
			host = "https://us.i.posthog.com"
		}
		out = append(out, host)
	}
	return out
}

func sentryEnv() string {
	if v := strings.TrimSpace(os.Getenv("VERCEL_ENV")); v != "" {
		return v
	}
	return "production"
}
