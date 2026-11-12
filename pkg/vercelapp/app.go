// Package vercelapp builds the HTTP handler used by the Vercel function.
// api/index.go is compiled outside this module, so it cannot import internal/.
// This package stays inside the module and is the only import that file needs.
package vercelapp

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/pursuit/pursuit/internal/app"
	"github.com/pursuit/pursuit/internal/config"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

// Handler serves the embedded dashboard and the REST API.
func Handler(w http.ResponseWriter, r *http.Request) {
	rt, err := ensure(r.Context())
	if err != nil {
		writeUnavailable(w, err)
		return
	}
	rt.handler.ServeHTTP(w, withOriginalPath(r))
}

type runtime struct {
	handler http.Handler
}

var (
	bootMu sync.Mutex
	booted *runtime
)

func ensure(ctx context.Context) (*runtime, error) {
	bootMu.Lock()
	defer bootMu.Unlock()
	if booted != nil {
		return booted, nil
	}
	dsn, err := postgresURL()
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	store, err := sqlstore.Open(ctx, dsn)
	if err != nil {
		slog.Error("database", "err", err.Error())
		return nil, fmt.Errorf("postgres database is unavailable")
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(log)
	trust, why := trustFromEnv(os.Getenv)
	if why != "" {
		log.Warn("PURSUIT_TRUST_PROXY_AUTH ignored", "value", os.Getenv("PURSUIT_TRUST_PROXY_AUTH"), "reason", why)
	}
	if trust != "" {
		log.Warn("proxy auth is on", "hint", "PURSUIT_TRUST_PROXY_AUTH=vercel treats a request with no Pursuit token as the owner. Only use this behind Vercel Authentication.")
	}
	cfg := configFromEnv(os.Getenv)
	cfg.TrustProxyAuth = trust
	if cfg.DemoMode {
		log.Warn("demo mode is on", "hint", "every unauthenticated caller gets a fresh throwaway tenant; never set PURSUIT_DEMO_MODE on a real-data deployment")
	}
	api, stage, err := app.NewServer(ctx, store, cfg, log, cfg.ServerURL)
	if err != nil {
		store.Close()
		slog.Error(cmp.Or(stage, "wiring"), "err", err.Error())
		switch stage {
		case app.StagePasskeys:
			return nil, fmt.Errorf("passkey configuration failed")
		case app.StageOIDC:
			return nil, fmt.Errorf("oidc configuration failed")
		}
		return nil, fmt.Errorf("bootstrap failed")
	}
	api.WebAnalytics = strings.TrimSpace(os.Getenv("PURSUIT_CF_BEACON_TOKEN")) != ""
	api.OnFailure = sentryHook(log)
	api.ConnectOrigins = webConnectOrigins(os.Getenv)
	booted = &runtime{handler: api.Handler()}
	return booted, nil
}

// configFromEnv is the settings the function honors. It has no config file,
// data directory, profile file or link-check schedule.
func configFromEnv(getenv func(string) string) config.Config {
	get := func(k string) string { return strings.TrimSpace(getenv(k)) }
	on := func(k, v string) bool { return strings.EqualFold(get(k), v) }
	var origins []string
	for _, o := range strings.Split(getenv("PURSUIT_WEBAUTHN_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	return config.Config{
		ServerURL:          publicURL(getenv),
		OwnerName:          cmp.Or(get("PURSUIT_OWNER_NAME"), "Adrian"),
		OwnerEmail:         cmp.Or(get("PURSUIT_OWNER_EMAIL"), "adrian@localhost"),
		BootstrapName:      cmp.Or(get("PURSUIT_BOOTSTRAP_NAME"), "owner"),
		BootstrapToken:     get("PURSUIT_BOOTSTRAP_TOKEN"),
		TenantMode:         strings.ToLower(cmp.Or(get("PURSUIT_TENANT_MODE"), "single")),
		SessionSecret:      get("PURSUIT_SESSION_SECRET"),
		OIDCGoogleID:       getenv("PURSUIT_OIDC_GOOGLE_CLIENT_ID"),
		OIDCGoogleSecret:   getenv("PURSUIT_OIDC_GOOGLE_CLIENT_SECRET"),
		OIDCIssuerURL:      getenv("PURSUIT_OIDC_ISSUER_URL"),
		OIDCIssuerName:     get("PURSUIT_OIDC_ISSUER_NAME"),
		OIDCIssuerID:       getenv("PURSUIT_OIDC_ISSUER_CLIENT_ID"),
		OIDCIssuerSecret:   getenv("PURSUIT_OIDC_ISSUER_CLIENT_SECRET"),
		BetterAuthBaseURL:  get("PURSUIT_BETTERAUTH_BASE_URL"),
		BetterAuthIssuer:   getenv("PURSUIT_BETTERAUTH_ISSUER"),
		BetterAuthAudience: getenv("PURSUIT_BETTERAUTH_AUDIENCE"),
		HideTokenLogin:     on("PURSUIT_HIDE_TOKEN_LOGIN", "true"),
		DemoMode:           on("PURSUIT_DEMO_MODE", "true"),
		SignupsClosed:      on("PURSUIT_SIGNUPS", "closed"),
		NoDocs:             on("PURSUIT_NO_DOCS", "true"),
		WebAuthnRPID:       getenv("PURSUIT_WEBAUTHN_RP_ID"),
		WebAuthnOrigins:    origins,
	}
}

// trustFromEnv returns "vercel" only on a Vercel preview deployment, where
// deployment protection gates every request. Production refuses it: there,
// one unprotected path or a protection setting turned off would make every
// tokenless request the owner. why says why a set value was ignored.
func trustFromEnv(getenv func(string) string) (trust, why string) {
	v := strings.TrimSpace(getenv("PURSUIT_TRUST_PROXY_AUTH"))
	switch {
	case v == "":
		return "", ""
	case !strings.EqualFold(v, "vercel"):
		return "", `the only supported value is "vercel"`
	case getenv("VERCEL_ENV") != "preview":
		return "", "only honored on Vercel preview deployments (VERCEL_ENV=preview); production needs a real sign-in"
	}
	return "vercel", ""
}

// publicURL is PURSUIT_SERVER, else the URL Vercel says this deployment
// answers on: the project's production domain (the shortest custom domain
// once one is added) in production, the deployment's own URL elsewhere.
// Adding a custom domain therefore needs no env change here.
func publicURL(getenv func(string) string) string {
	if v := strings.TrimSpace(getenv("PURSUIT_SERVER")); v != "" {
		return strings.TrimRight(v, "/")
	}
	host := getenv("VERCEL_URL")
	if getenv("VERCEL_ENV") == "production" {
		host = cmp.Or(getenv("VERCEL_PROJECT_PRODUCTION_URL"), host)
	}
	if host == "" {
		return ""
	}
	return "https://" + host
}

func postgresURL() (string, error) {
	return postgresURLFrom(os.Getenv)
}

func postgresURLFrom(getenv func(string) string) (string, error) {
	for _, key := range []string{"DATABASE_URL", "POSTGRES_URL"} {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "postgres://") || strings.HasPrefix(v, "postgresql://") {
			return v, nil
		}
		return "", fmt.Errorf("%s must be a postgres or postgresql URL", key)
	}
	return "", fmt.Errorf("set DATABASE_URL or POSTGRES_URL to a postgres URL")
}

// withOriginalPath keeps the caller's path when a rewrite reports the function path.
func withOriginalPath(r *http.Request) *http.Request {
	if r == nil || r.URL == nil {
		return r
	}
	switch r.URL.Path {
	case "/api", "/api/", "/api/index", "/api/index.go":
	default:
		return r
	}
	for _, header := range []string{"X-Original-Url", "X-Forwarded-Uri", "X-Vercel-Original-Path"} {
		raw := strings.TrimSpace(r.Header.Get(header))
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Path == "" {
			continue
		}
		clone := r.Clone(r.Context())
		clone.URL.Path = u.Path
		if u.RawQuery != "" {
			clone.URL.RawQuery = u.RawQuery
		}
		return clone
	}
	return r
}

func writeUnavailable(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": "unavailable", "message": err.Error()},
	})
}
