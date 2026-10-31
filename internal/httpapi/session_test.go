package httpapi

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pursuit/pursuit/internal/betterauth"
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

// fakeBetterAuth serves a JWKS for one Ed25519 key and signs JWTs with it,
// so the real betterauth.Verifier checks them.
func fakeBetterAuth(t *testing.T) (*betterauth.Verifier, func(email string, verified bool) string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.RawURLEncoding.EncodeToString
	jwks := `{"keys":[{"kty":"OKP","crv":"Ed25519","alg":"EdDSA","use":"sig","kid":"k1","x":"` + b64(pub) + `"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jwks))
	}))
	t.Cleanup(srv.Close)
	sign := func(email string, verified bool) string {
		now := time.Now().Unix()
		claims, _ := json.Marshal(map[string]any{"iss": srv.URL, "sub": email, "email": email, "email_verified": verified, "name": "Neon User", "iat": now, "exp": now + 900})
		in := b64([]byte(`{"alg":"EdDSA","typ":"JWT","kid":"k1"}`)) + "." + b64(claims)
		return in + "." + b64(ed25519.Sign(priv, []byte(in)))
	}
	return betterauth.New(context.Background(), srv.URL, "", ""), sign
}

func TestSessionExchangesBetterAuthJWT(t *testing.T) {
	e := newEnv(t)
	verifier, sign := fakeBetterAuth(t)
	e.api.BetterAuth, e.api.TenantMode = verifier, "multi"
	same := map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "same-origin"}
	post := func(jwt string) (int, []byte, http.Header) {
		return e.do(t, http.MethodPost, "/v1/auth/session", "", []byte(`{"token":"`+jwt+`"}`), same)
	}

	jwt := sign("neon@example.com", true)
	code, b, h := post(jwt)
	if code != 200 || !strings.Contains(string(b), "neon@example.com") {
		t.Fatalf("sign-in %d %s", code, b)
	}
	set := h.Get("Set-Cookie")
	if !strings.HasPrefix(set, sessionCookie+"=pst_") || strings.Contains(set, jwt) {
		t.Fatalf("cookie should hold a pst_ token, not the JWT: %q", set)
	}
	cookie := map[string]string{"Cookie": strings.SplitN(set, ";", 2)[0], "Sec-Fetch-Site": "same-origin"}
	if code, b, _ := e.do(t, http.MethodGet, "/v1/me", "", nil, cookie); code != 200 || !strings.Contains(string(b), `"login"`) {
		t.Fatalf("me via exchanged cookie %d %s", code, b)
	}
	// Sign-out revokes the minted login token.
	if code, _, _ := e.do(t, http.MethodDelete, "/v1/auth/session", "", nil, cookie); code != 204 {
		t.Fatalf("sign-out %d", code)
	}
	if code, _, _ := e.do(t, http.MethodGet, "/v1/me", "", nil, cookie); code != 401 {
		t.Fatalf("login token should be revoked after sign-out, got %d", code)
	}

	if code, _, _ := post(sign("unverified@example.com", false)); code != 401 {
		t.Fatalf("unverified email %d", code)
	}
	e.api.SignupsClosed = true
	if code, b, _ := post(sign("stranger@example.com", true)); code != 403 || !strings.Contains(string(b), "paused") {
		t.Fatalf("signups closed, unknown email %d %s", code, b)
	}
	// An existing user still signs in while sign-ups are closed.
	if code, b, _ := post(sign("neon@example.com", true)); code != 200 {
		t.Fatalf("existing user, signups closed %d %s", code, b)
	}
	// A pst_ token is stored as is, and sign-out leaves it valid.
	code, _, h = post(e.token)
	if code != 200 || !strings.Contains(h.Get("Set-Cookie"), sessionCookie+"="+e.token) {
		t.Fatalf("pst_ passthrough %d %q", code, h.Get("Set-Cookie"))
	}
	owner := map[string]string{"Cookie": sessionCookie + "=" + e.token, "Sec-Fetch-Site": "same-origin"}
	if code, _, _ := e.do(t, http.MethodDelete, "/v1/auth/session", "", nil, owner); code != 204 {
		t.Fatalf("owner sign-out %d", code)
	}
	if code, _, _ := e.do(t, http.MethodGet, "/v1/me", e.token, nil, nil); code != 200 {
		t.Fatalf("owner token should survive sign-out, got %d", code)
	}
}
