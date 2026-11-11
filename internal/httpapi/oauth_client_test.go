package httpapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pursuit/pursuit/internal/auth"
	"github.com/pursuit/pursuit/internal/service"
)

const cimdID = "https://app.example.com/oauth/client.json"

func clientDoc(id string, redirects ...string) []byte {
	b, _ := json.Marshal(map[string]any{"client_id": id, "client_name": "Example App", "redirect_uris": redirects, "token_endpoint_auth_method": "none"})
	return b
}

func authzFor(clientID, redirect string, approve bool) map[string]any {
	b := authzBody(redirect, "read", approve)
	b["client_id"] = clientID
	return b
}

func (e *env) register(t *testing.T, body string) (int, map[string]any) {
	t.Helper()
	code, b, _ := e.do(t, "POST", "/v1/auth/register", "", []byte(body), map[string]string{"Content-Type": "application/json"})
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("register %d %s: %v", code, b, err)
	}
	return code, doc
}

func TestOAuthClientCIMD(t *testing.T) {
	e := newEnv(t)
	clk := withClock(e)
	fetches := 0
	doc := clientDoc(cimdID, "https://app.example.com/callback", "http://localhost/callback")
	e.svc.ClientDoc = func(_ context.Context, u string) ([]byte, error) {
		fetches++
		if u != cimdID {
			t.Errorf("fetched %q", u)
		}
		return doc, nil
	}

	// A registered https redirect works end to end, with the client's query kept.
	code, out := e.consent(t, "/v1/oauth/authorize", e.token, authzFor(cimdID, "https://app.example.com/callback", true))
	if code != 200 {
		t.Fatalf("authorize: %d %v", code, out)
	}
	to, _ := url.Parse(out["redirect_to"].(string))
	if to.Host != "app.example.com" || to.Query().Get("code") == "" || to.Query().Get("state") != "st-123" {
		t.Fatalf("redirect_to %v", out["redirect_to"])
	}
	status, tok := e.form(t, "/v1/auth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {to.Query().Get("code")}, "code_verifier": {verifier},
		"redirect_uri": {"https://app.example.com/callback"}, "client_id": {cimdID},
	})
	if status != 200 || tok["access_token"] == "" || tok["scope"] != "read" {
		t.Fatalf("token: %d %v", status, tok)
	}

	// The consent check names the client and marks it unverified.
	q := url.Values{"client_id": {cimdID}, "redirect_uri": {"https://app.example.com/callback"}, "code_challenge": {auth.PKCEChallenge(verifier)},
		"code_challenge_method": {"S256"}, "state": {"s"}, "scope": {"read"}}
	status, chk := e.json(t, "GET", "/v1/oauth/authorize?"+q.Encode(), e.token, nil)
	if status != 200 || chk["client_name"] != "Example App" || chk["unverified"] != true {
		t.Fatalf("check: %d %v", status, chk)
	}
	if fetches != 1 {
		t.Errorf("document fetched %d times; the second request should use the cache", fetches)
	}

	// Loopback redirects ignore the port.
	if code, out := e.consent(t, "/v1/oauth/authorize", e.token, authzFor(cimdID, "http://localhost:49152/callback", true)); code != 200 {
		t.Errorf("loopback with port: %d %v", code, out)
	}
	// Anything not declared is refused.
	for _, bad := range []string{"https://evil.example.com/callback", "https://app.example.com/other", "http://127.0.0.1:5000/callback", "https://app.example.com/callback#x"} {
		if code, out := e.consent(t, "/v1/oauth/authorize", e.token, authzFor(cimdID, bad, true)); code != 400 {
			t.Errorf("%s: %d %v", bad, code, out)
		}
	}

	// The document is fetched again after an hour, and a changed one applies.
	clk.add(2 * time.Hour)
	doc = clientDoc(cimdID, "https://app.example.com/new")
	if code, out := e.consent(t, "/v1/oauth/authorize", e.token, authzFor(cimdID, "https://app.example.com/new", true)); code != 200 {
		t.Errorf("after refresh: %d %v", code, out)
	}
	if fetches != 2 {
		t.Errorf("fetches = %d, want 2", fetches)
	}
}

func TestOAuthClientCIMDRefused(t *testing.T) {
	e := newEnv(t)
	for name, tc := range map[string]struct {
		id  string
		doc []byte
	}{
		"id mismatch":      {cimdID, clientDoc("https://other.example.com/c.json", "https://app.example.com/cb")},
		"no redirect uris": {cimdID, clientDoc(cimdID)},
		"custom scheme":    {cimdID, clientDoc(cimdID, "myapp://cb")},
		"plain http":       {cimdID, clientDoc(cimdID, "http://app.example.com/cb")},
		"not json":         {cimdID, []byte("<html>")},
		"confidential":     {cimdID, []byte(`{"client_id":"` + cimdID + `","redirect_uris":["https://app.example.com/cb"],"token_endpoint_auth_method":"client_secret_basic"}`)},
		"no path":          {"https://app.example.com", nil},
		"dot segment":      {"https://app.example.com/a/../c.json", nil},
		"credentials":      {"https://u:p@app.example.com/c.json", nil},
		"other port":       {"https://app.example.com:8443/c.json", nil},
		"fragment":         {"https://app.example.com/c.json#f", nil},
	} {
		e.svc.ClientDoc = func(context.Context, string) ([]byte, error) { return tc.doc, nil }
		if code, out := e.consent(t, "/v1/oauth/authorize", e.token, authzFor(tc.id, "https://app.example.com/cb", true)); code != 400 {
			t.Errorf("%s: %d %v", name, code, out)
		}
	}
}

func TestOAuthClientRegistration(t *testing.T) {
	e := newEnv(t)
	code, doc := e.register(t, `{"client_name":"Claude","redirect_uris":["https://claude.ai/api/mcp/auth_callback"],"grant_types":["authorization_code","refresh_token"],"response_types":["code"],"token_endpoint_auth_method":"none","scope":"read"}`)
	id, _ := doc["client_id"].(string)
	if code != 201 || !strings.HasPrefix(id, "pc_") || doc["token_endpoint_auth_method"] != "none" {
		t.Fatalf("register: %d %v", code, doc)
	}
	if code, out := e.consent(t, "/v1/oauth/authorize", e.token, authzFor(id, "https://claude.ai/api/mcp/auth_callback", true)); code != 200 {
		t.Fatalf("authorize with registered client: %d %v", code, out)
	}
	if code, out := e.consent(t, "/v1/oauth/authorize", e.token, authzFor(id, "https://claude.ai/other", true)); code != 400 {
		t.Errorf("unregistered redirect: %d %v", code, out)
	}

	for name, body := range map[string]string{
		"no redirect":       `{"client_name":"x"}`,
		"custom scheme":     `{"redirect_uris":["myapp://cb"]}`,
		"http non-loopback": `{"redirect_uris":["http://example.com/cb"]}`,
		"fragment":          `{"redirect_uris":["https://example.com/cb#f"]}`,
		"too many":          `{"redirect_uris":["https://a.com/1","https://a.com/2","https://a.com/3","https://a.com/4","https://a.com/5","https://a.com/6"]}`,
	} {
		if code, doc := e.register(t, body); code != 400 || oauthError(doc) != "invalid_redirect_uri" {
			t.Errorf("%s: %d %v", name, code, doc)
		}
	}
	for name, body := range map[string]string{
		"secret client": `{"redirect_uris":["https://a.com/cb"],"token_endpoint_auth_method":"client_secret_post"}`,
		"implicit":      `{"redirect_uris":["https://a.com/cb"],"response_types":["token"]}`,
		"password":      `{"redirect_uris":["https://a.com/cb"],"grant_types":["password"]}`,
		"not json":      `nope`,
	} {
		if code, doc := e.register(t, body); code != 400 || oauthError(doc) != "invalid_client_metadata" {
			t.Errorf("%s: %d %v", name, code, doc)
		}
	}
	if code, _, _ := e.do(t, "POST", "/v1/auth/register", "", []byte(`{}`), map[string]string{"Content-Type": "text/plain"}); code != 400 {
		t.Errorf("non-JSON content type: %d", code)
	}

	e.api.registerLimiterFor().seen = map[string][]time.Time{}
	var last int
	for i := 0; i < 25; i++ {
		last, _ = e.register(t, `{"redirect_uris":["https://a.com/cb"]}`)
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("25 registrations in a minute: last status %d", last)
	}
}

func TestOAuthLegacyClientsKeepLoopback(t *testing.T) {
	e := newEnv(t)
	body := authzFor("some-local-tool", loopback, true)
	if code, out := e.consent(t, "/v1/oauth/authorize", e.token, body); code != 200 {
		t.Fatalf("unregistered plain client over loopback: %d %v", code, out)
	}
	body["redirect_uri"] = "https://example.com/callback"
	if code, out := e.consent(t, "/v1/oauth/authorize", e.token, body); code != 400 {
		t.Errorf("unregistered plain client over https: %d %v", code, out)
	}
}

func TestFetchClientDocRefusesPrivateAddresses(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("{}")) }))
	srv.TLS = &tls.Config{}
	srv.StartTLS()
	defer srv.Close()
	if _, err := service.FetchClientDoc(context.Background(), srv.URL+"/c.json"); err == nil {
		t.Fatal("fetched a loopback address")
	}
	for _, ip := range []string{"127.0.0.1", "::1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "fd00::1", "::ffff:10.0.0.1", "224.0.0.1"} {
		if !service.BlockedIP(netip.MustParseAddr(ip)) {
			t.Errorf("%s should be blocked", ip)
		}
	}
	for _, ip := range []string{"8.8.8.8", "160.79.104.1", "2606:4700::1111"} {
		if service.BlockedIP(netip.MustParseAddr(ip)) {
			t.Errorf("%s should be allowed", ip)
		}
	}
}
