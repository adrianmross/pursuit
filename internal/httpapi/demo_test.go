package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

func newDemoEnv(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := sqlstore.Open(context.Background(), "sqlite://"+filepath.Join(t.TempDir(), "pursuit.db"))
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

func meUserID(t *testing.T, srv *httptest.Server, jar http.CookieJar) string {
	t.Helper()
	client := &http.Client{Jar: jar}
	resp, err := client.Get(srv.URL + "/v1/me")
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
	return me.User.ID
}

func TestDemoModeMintsAndReusesSession(t *testing.T) {
	srv := newDemoEnv(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	first := meUserID(t, srv, jar)
	if first == "" {
		t.Fatal("no user id minted")
	}
	// Same jar (cookie kept) must resolve to the same tenant on a second call.
	second := meUserID(t, srv, jar)
	if second != first {
		t.Fatalf("session not reused: %s != %s", first, second)
	}

	// A different visitor (no cookie jar) must get an isolated tenant.
	otherJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	other := meUserID(t, srv, otherJar)
	if other == first {
		t.Fatal("two anonymous visitors shared the same tenant")
	}
}

func TestDemoModeRateLimitsMinting(t *testing.T) {
	srv := newDemoEnv(t)
	limit := newDemoLimiter().limit

	var last int
	for i := 0; i <= limit; i++ {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		client := &http.Client{Jar: jar}
		resp, err := client.Get(srv.URL + "/v1/me")
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
