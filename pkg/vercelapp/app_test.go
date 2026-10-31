package vercelapp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostgresURLOrder(t *testing.T) {
	got, err := postgresURLFrom(func(key string) string {
		switch key {
		case "DATABASE_URL":
			return "postgres://db/main"
		case "POSTGRES_URL":
			return "postgres://db/other"
		default:
			return ""
		}
	})
	if err != nil || got != "postgres://db/main" {
		t.Fatalf("prefer DATABASE_URL: %q %v", got, err)
	}

	got, err = postgresURLFrom(func(key string) string {
		if key == "POSTGRES_URL" {
			return "postgresql://db/neon"
		}
		return ""
	})
	if err != nil || got != "postgresql://db/neon" {
		t.Fatalf("fallback POSTGRES_URL: %q %v", got, err)
	}

	_, err = postgresURLFrom(func(key string) string {
		if key == "DATABASE_URL" {
			return "sqlite:///tmp/pursuit.db"
		}
		if key == "POSTGRES_URL" {
			return "postgres://db/other"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("sqlite should not fall through: %v", err)
	}

	_, err = postgresURLFrom(func(string) string { return "" })
	if err == nil {
		t.Fatal("expected a missing URL error")
	}
}

func TestHandlerRejectsSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no.db")
	t.Setenv("DATABASE_URL", "sqlite://"+path)
	t.Setenv("POSTGRES_URL", "postgres://db/should-not-connect")
	rec := httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "should-not-connect") {
		t.Fatalf("body leaked the fallback URL: %s", rec.Body.String())
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("sqlite file was created")
	}
}

func TestTrustFromEnv(t *testing.T) {
	for _, c := range []struct {
		trust, vercelEnv, want string
		ignored                bool
	}{
		{"", "preview", "", false},
		{"vercel", "preview", "vercel", false},
		{"VERCEL", "preview", "vercel", false},
		{"vercel", "production", "", true},
		{"vercel", "", "", true},
		{"on", "preview", "", true},
	} {
		got, why := trustFromEnv(func(k string) string {
			return map[string]string{"PURSUIT_TRUST_PROXY_AUTH": c.trust, "VERCEL_ENV": c.vercelEnv}[k]
		})
		if got != c.want || (why != "") != c.ignored {
			t.Errorf("trust=%q env=%q: got %q why %q", c.trust, c.vercelEnv, got, why)
		}
	}
}

func TestPublicURL(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"PURSUIT_SERVER": "https://pursuit.example/", "VERCEL_ENV": "production", "VERCEL_PROJECT_PRODUCTION_URL": "x.vercel.app"}, "https://pursuit.example"},
		{map[string]string{"VERCEL_ENV": "production", "VERCEL_PROJECT_PRODUCTION_URL": "pursuit.example", "VERCEL_URL": "pursuit-abc.vercel.app"}, "https://pursuit.example"},
		{map[string]string{"VERCEL_ENV": "preview", "VERCEL_PROJECT_PRODUCTION_URL": "pursuit.example", "VERCEL_URL": "pursuit-abc.vercel.app"}, "https://pursuit-abc.vercel.app"},
		{map[string]string{}, ""},
	} {
		if got := publicURL(func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("%v: got %q want %q", c.env, got, c.want)
		}
	}
}

func TestOriginalPathHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/index.go", nil)
	req.Header.Set("X-Original-Url", "/v1/applications?status=applied")
	got := withOriginalPath(req)
	if got.URL.Path != "/v1/applications" || got.URL.RawQuery != "status=applied" {
		t.Fatalf("path %s query %s", got.URL.Path, got.URL.RawQuery)
	}
	plain := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	if withOriginalPath(plain).URL.Path != "/healthz" {
		t.Fatal("rewrote a normal path")
	}
}
