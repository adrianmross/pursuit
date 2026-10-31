package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestSessionCookie(t *testing.T) {
	e := newEnv(t)
	same := map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "same-origin"}
	body := []byte(`{"token":"` + e.token + `"}`)

	if code, _, _ := e.do(t, http.MethodPost, "/v1/auth/session", "", body, map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site"}); code != 403 {
		t.Fatalf("cross-site sign-in %d", code)
	}
	if code, _, _ := e.do(t, http.MethodPost, "/v1/auth/session", "", []byte(`{"token":"pst_nope"}`), same); code != 401 {
		t.Fatalf("bad token sign-in %d", code)
	}
	code, b, h := e.do(t, http.MethodPost, "/v1/auth/session", "", body, same)
	if code != 200 || !strings.Contains(string(b), `"token"`) {
		t.Fatalf("sign-in %d %s", code, b)
	}
	set := h.Get("Set-Cookie")
	if !strings.Contains(set, sessionCookie+"="+e.token) || !strings.Contains(set, "HttpOnly") || !strings.Contains(set, "SameSite=Lax") {
		t.Fatalf("cookie %q", set)
	}
	cookie := map[string]string{"Cookie": sessionCookie + "=" + e.token}

	if code, _, _ := e.do(t, http.MethodGet, "/v1/me", "", nil, cookie); code != 200 {
		t.Fatalf("me via cookie %d", code)
	}
	write := []byte(`{"name":"Acme"}`)
	if code, _, _ := e.do(t, http.MethodPost, "/v1/companies", "", write, map[string]string{"Cookie": cookie["Cookie"], "Content-Type": "application/json"}); code != 403 {
		t.Fatalf("cookie write without origin %d", code)
	}
	if code, _, _ := e.do(t, http.MethodPost, "/v1/companies", "", write, map[string]string{"Cookie": cookie["Cookie"], "Content-Type": "application/json", "Origin": "https://evil.example"}); code != 403 {
		t.Fatalf("cookie write from another origin %d", code)
	}
	if code, b, _ := e.do(t, http.MethodPost, "/v1/companies", "", write, map[string]string{"Cookie": cookie["Cookie"], "Content-Type": "application/json", "Sec-Fetch-Site": "same-origin"}); code != 201 {
		t.Fatalf("same-origin cookie write %d %s", code, b)
	}
	// A bearer token is not a browser credential, so no origin check.
	if code, _, _ := e.do(t, http.MethodPost, "/v1/companies", e.token, []byte(`{"name":"Bearer Co"}`), map[string]string{"Content-Type": "application/json"}); code != 201 {
		t.Fatalf("bearer write %d", code)
	}

	// A stale cookie is a 401 that also clears it.
	code, _, h = e.do(t, http.MethodGet, "/v1/me", "", nil, map[string]string{"Cookie": sessionCookie + "=pst_revoked"})
	if code != 401 || !strings.Contains(h.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("stale cookie %d %q", code, h.Get("Set-Cookie"))
	}

	if code, _, _ := e.do(t, http.MethodDelete, "/v1/auth/session", "", nil, map[string]string{"Origin": "https://evil.example"}); code != 403 {
		t.Fatalf("cross-site sign-out %d", code)
	}
	code, _, h = e.do(t, http.MethodDelete, "/v1/auth/session", "", nil, map[string]string{"Sec-Fetch-Site": "same-origin"})
	if code != 204 || !strings.Contains(h.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("sign-out %d %q", code, h.Get("Set-Cookie"))
	}
}

func TestProxyOwnerWritesNeedSameOrigin(t *testing.T) {
	e := newEnv(t)
	e.api.TrustProxyAuth = "vercel"
	write := []byte(`{"name":"Acme"}`)
	if code, _, _ := e.do(t, http.MethodGet, "/v1/me", "", nil, nil); code != 200 {
		t.Fatalf("proxy read %d", code)
	}
	if code, _, _ := e.do(t, http.MethodPost, "/v1/companies", "", write, map[string]string{"Content-Type": "text/plain", "Sec-Fetch-Site": "cross-site"}); code != 403 {
		t.Fatalf("cross-site proxy write %d", code)
	}
	if code, _, _ := e.do(t, http.MethodPost, "/v1/companies", "", write, map[string]string{"Content-Type": "application/json"}); code != 403 {
		t.Fatalf("proxy write without origin %d", code)
	}
	if code, b, _ := e.do(t, http.MethodPost, "/v1/companies", "", write, map[string]string{"Content-Type": "application/json", "Origin": e.srv.URL}); code != 201 {
		t.Fatalf("same-origin proxy write %d %s", code, b)
	}
}
