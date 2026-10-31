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
	"time"

	"github.com/pursuit/pursuit/internal/betterauth"
	"github.com/pursuit/pursuit/internal/httpapi"
	"github.com/pursuit/pursuit/internal/links"
	"github.com/pursuit/pursuit/internal/oidcauth"
	"github.com/pursuit/pursuit/internal/service"
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
	svc := service.New(store)
	svc.HTTP = &http.Client{Timeout: 20 * time.Second}
	svc.Links = &links.Client{HTTP: svc.HTTP, Now: svc.Now}
	owner := env("PURSUIT_OWNER_NAME", "Adrian")
	email := env("PURSUIT_OWNER_EMAIL", "adrian@localhost")
	tokenName := env("PURSUIT_BOOTSTRAP_NAME", "owner")
	supplied := strings.TrimSpace(os.Getenv("PURSUIT_BOOTSTRAP_TOKEN"))
	secret, created, err := svc.Bootstrap(ctx, owner, email, tokenName, supplied)
	if err != nil {
		store.Close()
		slog.Error("bootstrap", "err", err.Error())
		return nil, fmt.Errorf("bootstrap failed")
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if created && supplied == "" {
		log.Info("bootstrap token created", "token", secret, "hint", "save this token; it will not be shown again")
	}
	trust, why := trustFromEnv(os.Getenv)
	if why != "" {
		log.Warn("PURSUIT_TRUST_PROXY_AUTH ignored", "value", os.Getenv("PURSUIT_TRUST_PROXY_AUTH"), "reason", why)
	}
	if trust != "" {
		log.Warn("proxy auth is on", "hint", "PURSUIT_TRUST_PROXY_AUTH=vercel treats a request with no Pursuit token as the owner. Only use this behind Vercel Authentication.")
	}
	tenantMode := strings.ToLower(env("PURSUIT_TENANT_MODE", "single"))
	oidcMgr, err := buildOIDC(ctx, log)
	if err != nil {
		store.Close()
		slog.Error("oidc", "err", err.Error())
		return nil, fmt.Errorf("oidc configuration failed")
	}
	var betterAuthVerifier *betterauth.Verifier
	var betterAuthOrigin string
	if baseURL := strings.TrimSpace(os.Getenv("PURSUIT_BETTERAUTH_BASE_URL")); baseURL != "" {
		betterAuthVerifier = betterauth.New(ctx, baseURL, os.Getenv("PURSUIT_BETTERAUTH_ISSUER"), os.Getenv("PURSUIT_BETTERAUTH_AUDIENCE"))
		betterAuthOrigin = betterauth.Origin(baseURL)
	}
	hideTokenLogin := strings.EqualFold(strings.TrimSpace(os.Getenv("PURSUIT_HIDE_TOKEN_LOGIN")), "true")
	demoMode := strings.EqualFold(strings.TrimSpace(os.Getenv("PURSUIT_DEMO_MODE")), "true")
	if demoMode {
		log.Warn("demo mode is on", "hint", "every unauthenticated caller gets a fresh throwaway tenant; never set PURSUIT_DEMO_MODE on a real-data deployment")
	}
	api := &httpapi.Server{Svc: svc, Log: log, TrustProxyAuth: trust, TenantMode: tenantMode, OIDC: oidcMgr, BetterAuth: betterAuthVerifier, BetterAuthOrigin: betterAuthOrigin, HideTokenLogin: hideTokenLogin, SignupsClosed: strings.EqualFold(strings.TrimSpace(os.Getenv("PURSUIT_SIGNUPS")), "closed"), DemoMode: demoMode, NoDocs: strings.EqualFold(strings.TrimSpace(os.Getenv("PURSUIT_NO_DOCS")), "true")}
	api.Ready.Store(true)
	booted = &runtime{handler: api.Handler()}
	return booted, nil
}

// buildOIDC returns nil when no provider is configured.
func buildOIDC(ctx context.Context, log *slog.Logger) (*oidcauth.Manager, error) {
	var specs []oidcauth.Spec
	if id, secret := os.Getenv("PURSUIT_OIDC_GOOGLE_CLIENT_ID"), os.Getenv("PURSUIT_OIDC_GOOGLE_CLIENT_SECRET"); id != "" && secret != "" {
		specs = append(specs, oidcauth.Spec{Name: "google", Label: "Google", IssuerURL: "https://accounts.google.com", ClientID: id, ClientSecret: secret})
	}
	if issuer, id, secret := os.Getenv("PURSUIT_OIDC_ISSUER_URL"), os.Getenv("PURSUIT_OIDC_ISSUER_CLIENT_ID"), os.Getenv("PURSUIT_OIDC_ISSUER_CLIENT_SECRET"); issuer != "" && id != "" && secret != "" {
		specs = append(specs, oidcauth.Spec{Name: "custom", Label: env("PURSUIT_OIDC_ISSUER_NAME", "SSO"), IssuerURL: issuer, ClientID: id, ClientSecret: secret})
	}
	if len(specs) == 0 {
		return nil, nil
	}
	secret := strings.TrimSpace(os.Getenv("PURSUIT_SESSION_SECRET"))
	if secret == "" {
		return nil, fmt.Errorf("PURSUIT_SESSION_SECRET is required when an OIDC provider is configured")
	}
	base := publicURL(os.Getenv)
	if base == "" {
		return nil, fmt.Errorf("PURSUIT_SERVER (the deployment's public URL) is required when an OIDC provider is configured")
	}
	redirect := func(name string) string { return base + "/v1/auth/" + name + "/callback" }
	return oidcauth.New(ctx, []byte(secret), specs, redirect)
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

func env(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
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
