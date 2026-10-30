package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestDashboardAssets(t *testing.T) {
	e := newEnv(t)
	code, body, header := e.do(t, http.MethodGet, "/", "", nil, nil)
	if code != 200 {
		t.Fatalf("index %d %s", code, body)
	}
	if !strings.Contains(header.Get("Content-Type"), "text/html") {
		t.Fatalf("content type %q", header.Get("Content-Type"))
	}
	page := string(body)
	if !strings.Contains(page, `name="pursuit-dashboard"`) || !strings.Contains(page, `id="root"`) {
		t.Fatalf("index missing the SPA shell: %s", page)
	}
	if !strings.Contains(header.Get("Content-Security-Policy"), "script-src 'self'") {
		t.Fatalf("csp %q", header.Get("Content-Security-Policy"))
	}

	// Client-side routes have no file of their own — they fall back to the
	// SPA shell, which authenticates against the API in the browser.
	code, body, header = e.do(t, http.MethodGet, "/app/does-not-exist", "", nil, nil)
	if code != 200 || !strings.Contains(header.Get("Content-Type"), "text/html") || !strings.Contains(string(body), `id="root"`) {
		t.Fatalf("spa fallback route %d %s", code, body)
	}

	// A path with an extension that genuinely doesn't exist 404s instead of
	// silently serving the SPA shell.
	code, _, _ = e.do(t, http.MethodGet, "/missing-asset.js", "", nil, nil)
	if code != 404 {
		t.Fatalf("missing asset should 404, got %d", code)
	}

	code, _, _ = e.do(t, http.MethodGet, "/v1/applications", "", nil, nil)
	if code != 401 {
		t.Fatalf("api without token %d", code)
	}
	// An unknown /v1 path still requires auth rather than falling back to the
	// dashboard shell — auth runs before routing decides there's no handler.
	code, _, _ = e.do(t, http.MethodGet, "/v1/does-not-exist", "", nil, nil)
	if code != 401 {
		t.Fatalf("unknown v1 path without a token %d", code)
	}
}

func TestDocsServedAndToggle(t *testing.T) {
	e := newEnv(t)
	code, body, header := e.do(t, http.MethodGet, "/docs/cli", "", nil, nil)
	if code != 200 || !strings.Contains(header.Get("Content-Type"), "text/html") || strings.Contains(string(body), `id="root"`) {
		t.Fatalf("docs page %d %s", code, body)
	}
	if !strings.Contains(header.Get("Content-Security-Policy"), "'unsafe-inline'") {
		t.Fatalf("docs csp %q", header.Get("Content-Security-Policy"))
	}
	if code, _, _ = e.do(t, http.MethodGet, "/docs/nope", "", nil, nil); code != 404 {
		t.Fatalf("unknown docs page %d", code)
	}
	e.api.NoDocs = true
	if code, _, _ = e.do(t, http.MethodGet, "/docs/cli", "", nil, nil); code != 404 {
		t.Fatalf("docs with NoDocs %d", code)
	}
}
