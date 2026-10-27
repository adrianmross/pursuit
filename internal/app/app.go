// Package app runs the Pursuit daemon.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pursuit/pursuit/internal/config"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/httpapi"
	"github.com/pursuit/pursuit/internal/links"
	"github.com/pursuit/pursuit/internal/oidcauth"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

func Run(ctx context.Context, cfg config.Config) error {
	log := newLogger(cfg)
	dbURL := cfg.DatabaseURL
	if dbURL == "" && cfg.DataDir != "" {
		dbURL = "sqlite://" + filepath.Join(cfg.DataDir, "pursuit.db")
	}
	store, err := sqlstore.Open(ctx, dbURL)
	if err != nil {
		return err
	}
	defer store.Close()
	log.Info("database", "dialect", store.Dialect(), "url", redact(dbURL))

	profile, err := loadProfile(cfg.ProfilePath)
	if err != nil {
		return err
	}
	svc := service.New(store)
	svc.Profile = profile
	svc.ResumeFile = cfg.ResumeFile
	if cfg.ContactEmail != "" {
		if svc.Profile == nil {
			svc.Profile = map[string]string{}
		}
		if _, ok := svc.Profile["email"]; !ok {
			svc.Profile["email"] = cfg.ContactEmail
		}
	}
	svc.HTTP = &http.Client{Timeout: 20 * time.Second}
	svc.Links = &links.Client{HTTP: svc.HTTP, Now: svc.Now, Gap: cfg.CheckGap}
	svc.Hooks = func(userID string, ev domain.Event) {
		go svc.DeliverWebhooks(context.Background(), userID, ev)
	}

	secret, created, err := svc.Bootstrap(ctx, cfg.OwnerName, cfg.OwnerEmail, cfg.BootstrapName, cfg.BootstrapToken)
	if err != nil {
		return err
	}
	if created && strings.TrimSpace(cfg.BootstrapToken) == "" {
		log.Info("bootstrap token created", "token", secret, "hint", "save this token; it will not be shown again")
	}

	if v := strings.TrimSpace(cfg.TrustProxyAuth); v != "" && !strings.EqualFold(v, "vercel") {
		log.Warn("PURSUIT_TRUST_PROXY_AUTH ignored", "value", v)
	}
	if strings.EqualFold(strings.TrimSpace(cfg.TrustProxyAuth), "vercel") {
		log.Warn("proxy auth is on", "hint", "PURSUIT_TRUST_PROXY_AUTH=vercel treats a request with no Pursuit token as the owner. Only use this behind deployment protection.")
	}
	oidcMgr, err := buildOIDC(ctx, cfg)
	if err != nil {
		return err
	}
	api := &httpapi.Server{Svc: svc, Log: log, CORS: cfg.CORSOrigins, TrustProxyAuth: cfg.TrustProxyAuth, TenantMode: cfg.TenantMode, OIDC: oidcMgr}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	api.Ready.Store(true)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.Addr)

	sweepCtx, sweepCancel := context.WithCancel(ctx)
	defer sweepCancel()
	go runSweep(sweepCtx, svc, log, cfg)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	select {
	case <-ctx.Done():
	case <-sig:
	case err := <-errCh:
		api.Ready.Store(false)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
	api.Ready.Store(false)
	sweepCancel()
	sh, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	err = srv.Shutdown(sh)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// buildOIDC returns nil when no provider is configured.
func buildOIDC(ctx context.Context, cfg config.Config) (*oidcauth.Manager, error) {
	var specs []oidcauth.Spec
	if cfg.OIDCGoogleID != "" && cfg.OIDCGoogleSecret != "" {
		specs = append(specs, oidcauth.Spec{Name: "google", Label: "Google", IssuerURL: "https://accounts.google.com", ClientID: cfg.OIDCGoogleID, ClientSecret: cfg.OIDCGoogleSecret})
	}
	if cfg.OIDCIssuerURL != "" && cfg.OIDCIssuerID != "" && cfg.OIDCIssuerSecret != "" {
		label := cfg.OIDCIssuerName
		if label == "" {
			label = "SSO"
		}
		specs = append(specs, oidcauth.Spec{Name: "custom", Label: label, IssuerURL: cfg.OIDCIssuerURL, ClientID: cfg.OIDCIssuerID, ClientSecret: cfg.OIDCIssuerSecret})
	}
	if len(specs) == 0 {
		return nil, nil
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("PURSUIT_SESSION_SECRET is required when an OIDC provider is configured")
	}
	base := strings.TrimRight(cfg.ServerURL, "/")
	redirect := func(name string) string { return base + "/v1/auth/" + name + "/callback" }
	return oidcauth.New(ctx, []byte(cfg.SessionSecret), specs, redirect)
}

func runSweep(ctx context.Context, svc *service.Service, log *slog.Logger, cfg config.Config) {
	owner, _ := os.Hostname()
	if owner == "" {
		owner = "pursuit"
	}
	for {
		wait := cfg.CheckInterval
		if cfg.CheckJitter > 0 {
			wait += time.Duration(rand.Float64() * cfg.CheckJitter * float64(cfg.CheckInterval))
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		report, err := svc.SweepAsOwner(ctx, owner)
		if err != nil {
			if domain.IsCode(err, "conflict") {
				log.Info("link sweep skipped", "reason", "lock held")
				continue
			}
			log.Error("link sweep", "err", err.Error())
			continue
		}
		log.Info("link sweep", "checked", report.Checked)
	}
}

func loadProfile(path string) (map[string]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Answers map[string]string `json:"answers"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return doc.Answers, nil
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	if strings.ToLower(cfg.LogFormat) == "text" {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}

func redact(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	return u.Redacted()
}
