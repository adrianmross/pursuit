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
	if !strings.Contains(page, `<title>Hire Pursuit</title>`) || !strings.Contains(page, "self.__next_f") {
		t.Fatalf("index missing the dashboard shell: %s", page)
	}
	if !strings.Contains(header.Get("Content-Security-Policy"), "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("csp %q", header.Get("Content-Security-Policy"))
	}

	// A path with no file of its own (every real route is its own
	// prerendered file) gets the themed 404 page, not the landing page.
	code, body, header = e.do(t, http.MethodGet, "/pipeline", "", nil, nil)
	if code != 404 || !strings.Contains(header.Get("Content-Type"), "text/html") || !strings.Contains(string(body), "You rolled a 404.") {
		t.Fatalf("unknown route %d %s", code, body)
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
	if code != 200 || !strings.Contains(header.Get("Content-Type"), "text/html") || !strings.Contains(string(body), "Hire Pursuit docs") {
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
