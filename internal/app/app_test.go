package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pursuit/pursuit/internal/config"
	"github.com/pursuit/pursuit/internal/sqlstore"
	"github.com/pursuit/pursuit/internal/sqlstore/sqlstoretest"
)

// NewServer on an empty database creates the owner's token and returns a
// server that accepts it; an OIDC provider without a session secret fails at the OIDC stage.
func TestNewServer(t *testing.T) {
	ctx := context.Background()
	store, err := sqlstore.Open(ctx, sqlstoretest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := config.Config{ServerURL: "http://127.0.0.1:8787", BootstrapToken: "pst_test_owner_token_0123456789"}
	api, stage, err := NewServer(ctx, store, cfg, slog.New(slog.DiscardHandler), "")
	if err != nil {
		t.Fatalf("stage %q: %v", stage, err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.BootstrapToken)
	rec := httptest.NewRecorder()
	api.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET /v1/me = %d %s", rec.Code, rec.Body)
	}

	cfg.OIDCGoogleID, cfg.OIDCGoogleSecret = "id", "secret"
	if _, stage, err := NewServer(ctx, store, cfg, slog.New(slog.DiscardHandler), ""); err == nil || stage != StageOIDC {
		t.Fatalf("OIDC without a session secret: stage %q err %v", stage, err)
	}
}
