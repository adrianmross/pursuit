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
	if !strings.Contains(page, `name="pursuit-dashboard"`) || !strings.Contains(page, "/dashboard.js") || !strings.Contains(page, "/dashboard.css") {
		t.Fatalf("index missing assets: %s", page)
	}
	if !strings.Contains(header.Get("Content-Security-Policy"), "script-src 'self'") {
		t.Fatalf("csp %q", header.Get("Content-Security-Policy"))
	}

	code, body, header = e.do(t, http.MethodGet, "/dashboard.css", "", nil, nil)
	if code != 200 || !strings.Contains(header.Get("Content-Type"), "text/css") || !strings.Contains(string(body), "--bg") {
		t.Fatalf("css %d %s", code, body)
	}
	code, body, header = e.do(t, http.MethodGet, "/dashboard.js", "", nil, nil)
	script := string(body)
	if code != 200 || !strings.Contains(header.Get("Content-Type"), "javascript") || !strings.Contains(script, "pursuit.token") {
		t.Fatalf("js %d %s", code, body)
	}
	if !strings.Contains(script, "/v1/approvals/reject") || !strings.Contains(script, "does not submit the form") {
		t.Fatal("dashboard script is missing the approval actions")
	}
	if strings.Contains(script, "/v1/queue/claim") || strings.Contains(script, "/v1/queue/") {
		t.Fatal("dashboard script claims queue work")
	}

	code, _, _ = e.do(t, http.MethodGet, "/v1/applications", "", nil, nil)
	if code != 401 {
		t.Fatalf("api without token %d", code)
	}
	code, missing, _ := e.do(t, http.MethodGet, "/missing-asset", "", nil, nil)
	if code != 401 {
		t.Fatalf("unknown path should stay on the API auth path, got %d %s", code, missing)
	}
}
