package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/sqlstore"
	"github.com/pursuit/pursuit/internal/sqlstore/sqlstoretest"
)

func newDemoEnv(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := sqlstore.Open(context.Background(), sqlstoretest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	svc := service.New(db)
	api := &Server{Svc: svc, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), DemoMode: true}
	api.Ready.Store(true)
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func startDemo(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	resp, err := http.Post(srv.URL+"/v1/demo", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /v1/demo: status %d", resp.StatusCode)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Token == "" {
		t.Fatal("no token minted")
	}
	return body.Token
}

func meViaToken(t *testing.T, srv *httptest.Server, token string) domain.Me {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET /v1/me: status %d", resp.StatusCode)
	}
	var me domain.Me
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		t.Fatal(err)
	}
	return me
}

func TestDemoModeDisabledHidesTheEndpoint(t *testing.T) {
	db, err := sqlstore.Open(context.Background(), sqlstoretest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	api := &Server{Svc: service.New(db), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	api.Ready.Store(true)
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/v1/demo", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("expected demo mode off to 404, got %d", resp.StatusCode)
	}
}

func TestDemoModeMintsIsolatedSandboxAndCanReplay(t *testing.T) {
	srv := newDemoEnv(t)

	tokenA := startDemo(t, srv)
	meA := meViaToken(t, srv, tokenA)
	if meA.User.ID == "" {
		t.Fatal("no user id minted")
	}
	if meA.Token.Name != "demo" {
		t.Fatalf("expected a demo-named token, got %q", meA.Token.Name)
	}

	// "Replay tutorial": minting again abandons the old sandbox for a
	// brand-new, isolated one — it must not reuse or touch the first.
	tokenB := startDemo(t, srv)
	meB := meViaToken(t, srv, tokenB)
	if meB.User.ID == meA.User.ID {
		t.Fatal("replaying the tutorial reused the previous sandbox")
	}

	// The old token must still work — it wasn't revoked by the new mint.
	meAAgain := meViaToken(t, srv, tokenA)
	if meAAgain.User.ID != meA.User.ID {
		t.Fatal("the first sandbox's token stopped working after a replay")
	}
}

func TestDemoModeRateLimitsMinting(t *testing.T) {
	srv := newDemoEnv(t)
	limit := newDemoLimiter().limit

	var last int
	for i := 0; i <= limit; i++ {
		resp, err := http.Post(srv.URL+"/v1/demo", "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		last = resp.StatusCode
		resp.Body.Close()
	}
	if last != 429 {
		t.Fatalf("expected the mint past the limit to be rate-limited, got %d", last)
	}
}
