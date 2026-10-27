package httpapi

import (
	"net/http"
	"net/url"
	"testing"
)

func TestCaptureLookupProfileAndExtensionCORS(t *testing.T) {
	e := newEnv(t)
	origin := "chrome-extension://pursuittest"
	code, _, hdr := e.do(t, http.MethodOptions, "/v1/profile", "", nil, map[string]string{
		"Origin": origin, "Access-Control-Request-Method": "GET",
	})
	if code == 204 || hdr.Get("Access-Control-Allow-Origin") == origin {
		t.Fatal("extension origin was allowed before it was configured")
	}
	e.api.CORS = []string{"chrome-extension://*", "moz-extension://*"}
	code, _, hdr = e.do(t, http.MethodOptions, "/v1/profile", "", nil, map[string]string{
		"Origin": origin, "Access-Control-Request-Method": "GET",
	})
	if code != 204 || hdr.Get("Access-Control-Allow-Origin") != origin {
		t.Fatalf("preflight %d allow %q", code, hdr.Get("Access-Control-Allow-Origin"))
	}

	code, profile := e.json(t, http.MethodGet, "/v1/profile", e.token, nil)
	if code != 200 {
		t.Fatal(profile)
	}
	code, _ = e.json(t, http.MethodPost, "/v1/standing", e.token, map[string]any{"key": "email", "answer": "person@example.com"})
	if code != 200 {
		t.Fatal(code)
	}
	code, profile = e.json(t, http.MethodGet, "/v1/profile", e.token, nil)
	if code != 200 || profile["answers"].(map[string]any)["email"] != "person@example.com" {
		t.Fatalf("profile %#v", profile)
	}

	page := "https://job-boards.greenhouse.io/acme/jobs/111?utm_source=linkedin"
	code, saved := e.json(t, http.MethodPost, "/v1/applications/capture", e.token, map[string]any{
		"company": "Acme", "title": "Engineer", "location": "Remote", "salary": "$100K",
		"posting_url": page,
	})
	if code != 200 || saved["created"] != true {
		t.Fatalf("capture %#v", saved)
	}
	app := saved["application"].(map[string]any)
	if app["status"] != "to_apply" || app["posting_url"] != "https://job-boards.greenhouse.io/acme/jobs/111" {
		t.Fatalf("saved %#v", app)
	}
	code, again := e.json(t, http.MethodPost, "/v1/applications/capture", e.token, map[string]any{
		"company": "Acme", "title": "Engineer", "posting_url": "https://job-boards.greenhouse.io/acme/jobs/111",
	})
	if code != 200 || again["created"] != false || again["application"].(map[string]any)["id"] != app["id"] {
		t.Fatalf("dedupe %#v", again)
	}
	code, found := e.json(t, http.MethodGet, "/v1/applications/by-url?url="+url.QueryEscape(page), e.token, nil)
	if code != 200 || found["id"] != app["id"] {
		t.Fatalf("lookup %d %#v", code, found)
	}
	code, missing := e.json(t, http.MethodGet, "/v1/applications/by-url?url="+url.QueryEscape("https://example.com/jobs/nope"), e.token, nil)
	if code != 404 || errCode(missing) != "not_found" {
		t.Fatalf("missing %d %#v", code, missing)
	}
}
