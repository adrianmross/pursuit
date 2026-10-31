package httpapi

import (
	"net/http"
	"regexp"
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
	if header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("index cache-control %q", header.Get("Cache-Control"))
	}

	// Hashed build output is cached for good; only the HTML revalidates.
	chunk := regexp.MustCompile(`/_next/static/[^"]+\.js`).FindString(page)
	if chunk == "" {
		t.Fatalf("index references no _next/static chunk")
	}
	code, _, header = e.do(t, http.MethodGet, chunk, "", nil, nil)
	if code != 200 || !strings.Contains(header.Get("Cache-Control"), "immutable") {
		t.Fatalf("chunk %s: %d cache-control %q", chunk, code, header.Get("Cache-Control"))
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

// /reference sends anyone signed out to sign in. That target has to be a
// real dashboard route: it once pointed at /login, which is the 404 page.
// Its Scalar bundle must name an exact version, or jsDelivr serves @latest.
func TestReferencePage(t *testing.T) {
	e := newEnv(t)
	code, body, _ := e.do(t, http.MethodGet, "/reference", "", nil, nil)
	if code != 200 {
		t.Fatalf("reference %d", code)
	}
	page := string(body)
	if !regexp.MustCompile(`@scalar/api-reference@\d+\.\d+\.\d+/[^"]+" integrity="sha384-`).MatchString(page) {
		t.Fatalf("scalar script is not pinned to an exact version with SRI")
	}
	m := regexp.MustCompile(`location\.replace\("(/[^"?]*)\?next=`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("reference page has no sign-in redirect")
	}
	if code, _, _ := e.do(t, http.MethodGet, m[1], "", nil, nil); code != 200 {
		t.Fatalf("sign-in redirect %s answers %d", m[1], code)
	}
}
