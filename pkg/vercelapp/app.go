// Package vercelapp builds the HTTP handler used by the Vercel function.
// api/index.go is compiled outside this module, so it cannot import internal/.
// This package stays inside the module and is the only import that file needs.
package vercelapp

import (
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

	"github.com/pursuit/pursuit/internal/httpapi"
	"github.com/pursuit/pursuit/internal/links"
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
	trust := strings.TrimSpace(os.Getenv("PURSUIT_TRUST_PROXY_AUTH"))
	if trust != "" && !strings.EqualFold(trust, "vercel") {
		log.Warn("PURSUIT_TRUST_PROXY_AUTH ignored", "value", trust)
		trust = ""
	}
	if strings.EqualFold(trust, "vercel") {
		log.Warn("proxy auth is on", "hint", "PURSUIT_TRUST_PROXY_AUTH=vercel treats a request with no Pursuit token as the owner. Only use this behind Vercel Authentication.")
	}
	api := &httpapi.Server{Svc: svc, Log: log, TrustProxyAuth: trust}
	api.Ready.Store(true)
	booted = &runtime{handler: api.Handler()}
	return booted, nil
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
