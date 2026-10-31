package httpapi

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pursuit/pursuit/internal/auth"
	"github.com/pursuit/pursuit/internal/service"
)

// clock is a settable Service.Now for expiry and slow_down tests.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func withClock(e *env) *clock {
	c := &clock{t: time.Now().UTC()}
	e.svc.Now = c.now
	return c
}

func (e *env) form(t *testing.T, path string, vals url.Values) (int, map[string]any) {
	t.Helper()
	code, b, h := e.do(t, "POST", path, "", []byte(vals.Encode()), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if path == "/v1/auth/token" && h.Get("Cache-Control") != "no-store" {
		t.Fatalf("token response must be no-store, got %q", h.Get("Cache-Control"))
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("status %d body %s: %v", code, b, err)
	}
	return code, doc
}

// consent is a same-origin JSON POST, the way the dashboard page sends it.
func (e *env) consent(t *testing.T, path, token string, body any) (int, map[string]any) {
	t.Helper()
	return e.consentWith(t, path, token, body, map[string]string{"Content-Type": "application/json", "Origin": e.srv.URL})
}

func (e *env) consentWith(t *testing.T, path, token string, body any, headers map[string]string) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	code, b, _ := e.do(t, "POST", path, token, raw, headers)
	var doc map[string]any
	_ = json.Unmarshal(b, &doc)
	return code, doc
}

func oauthError(doc map[string]any) string {
	s, _ := doc["error"].(string)
	return s
}

// meOf returns /v1/me for a token: its scopes as JSON and its user id.
func meOf(t *testing.T, e *env, token string) (scopes, userID, tokenID string) {
	t.Helper()
	code, doc := e.json(t, "GET", "/v1/me", token, nil)
	if code != 200 {
		t.Fatalf("me: %d %v", code, doc)
	}
	tok, _ := doc["token"].(map[string]any)
	user, _ := doc["user"].(map[string]any)
	raw, _ := json.Marshal(tok["scopes"])
	return string(raw), user["id"].(string), tok["id"].(string)
}

func scopesOf(t *testing.T, e *env, token string) string {
	s, _, _ := meOf(t, e, token)
	return s
}

const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk_and_some_more"
const loopback = "http://127.0.0.1:53111/callback"

func authzBody(redirect, scope string, approve bool) map[string]any {
	return map[string]any{
		"client_id": "pursuit-cli", "redirect_uri": redirect, "code_challenge": auth.PKCEChallenge(verifier),
		"code_challenge_method": "S256", "state": "st-123", "scope": scope, "approve": approve,
	}
}

func authorize(t *testing.T, e *env, redirect, scope string, approve bool) (int, map[string]any) {
	t.Helper()
	return e.consent(t, "/v1/oauth/authorize", e.token, authzBody(redirect, scope, approve))
}

func codeFrom(t *testing.T, doc map[string]any) string {
	t.Helper()
	to, _ := doc["redirect_to"].(string)
	u, err := url.Parse(to)
	if err != nil || u.Query().Get("state") != "st-123" {
		t.Fatalf("bad redirect_to %q (%v)", to, doc)
	}
	return u.Query().Get("code")
}

func exchange(t *testing.T, e *env, code, v, redir string) (int, map[string]any) {
	t.Helper()
	return e.form(t, "/v1/auth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {v},
		"redirect_uri": {redir}, "client_id": {"pursuit-cli"},
	})
}

func mintToken(t *testing.T, e *env, scopes string) string {
	t.Helper()
	code, doc := e.json(t, "POST", "/v1/tokens", e.token, map[string]any{"name": "agent", "scopes": strings.Split(scopes, ",")})
	if code != 201 {
		t.Fatalf("mint: %d %v", code, doc)
	}
	return doc["secret"].(string)
}

func TestValidLoopbackRedirect(t *testing.T) {
	for _, tc := range []struct {
		uri string
		ok  bool
	}{
		{"http://127.0.0.1:5000/callback", true},
		{"http://[::1]:5000/callback", true},
		{"http://127.0.0.1:1/callback", true},
		{"http://localhost:5000/callback", false},
		{"https://127.0.0.1:5000/callback", false},
		{"http://127.0.0.1/callback", false},
		{"http://127.0.0.1:5000/other", false},
		{"http://127.0.0.1:5000/callback/", false},
		{"http://127.0.0.1:5000/callback?x=1", false},
		{"http://127.0.0.1:5000/callback#f", false},
		{"http://user@127.0.0.1:5000/callback", false},
		{"http://127.0.0.1.evil.example:80/callback", false},
		{"http://127.0.0.2:5000/callback", false},
		{"http://evil.example/callback", false},
		{"//127.0.0.1:5000/callback", false},
		{"", false},
	} {
		if got := service.ValidLoopbackRedirect(tc.uri) == nil; got != tc.ok {
			t.Errorf("%q: ok=%v, want %v", tc.uri, got, tc.ok)
		}
	}
}

func TestOAuthLoopbackFlow(t *testing.T) {
	e := newEnv(t)

	t.Run("validate step returns the server's reading", func(t *testing.T) {
		q := url.Values{"client_id": {"pursuit-cli"}, "redirect_uri": {loopback}, "code_challenge": {auth.PKCEChallenge(verifier)},
			"code_challenge_method": {"S256"}, "state": {"s"}, "scope": {"write"}, "response_type": {"code"}}
		code, doc := e.json(t, "GET", "/v1/oauth/authorize?"+q.Encode(), e.token, nil)
		if raw, _ := json.Marshal(doc["scopes"]); code != 200 || string(raw) != `["read","write"]` || doc["client_id"] != "pursuit-cli" || doc["redirect_uri"] != loopback {
			t.Fatalf("check: %d %v", code, doc)
		}
	})

	t.Run("happy path grants read and write", func(t *testing.T) {
		code, doc := authorize(t, e, loopback, "", true)
		if code != 200 {
			t.Fatalf("authorize: %d %v", code, doc)
		}
		status, tok := exchange(t, e, codeFrom(t, doc), verifier, loopback)
		if status != 200 || tok["token_type"] != "Bearer" || tok["scope"] != "read write" {
			t.Fatalf("exchange: %d %v", status, tok)
		}
		if got := scopesOf(t, e, tok["access_token"].(string)); got != `["read","write"]` {
			t.Fatalf("default scopes = %s", got)
		}
	})

	t.Run("approve and non-exact scopes are invalid_scope", func(t *testing.T) {
		for _, sc := range []string{"approve", "APPROVE", "Read", "read write approve", "read,admin"} {
			code, doc := authorize(t, e, loopback, sc, true)
			if code != 400 || errCode(doc) != "invalid_scope" {
				t.Fatalf("POST scope %q: %d %v", sc, code, doc)
			}
			q := url.Values{"redirect_uri": {loopback}, "code_challenge": {auth.PKCEChallenge(verifier)}, "code_challenge_method": {"S256"}, "state": {"s"}, "scope": {sc}}
			code, doc = e.json(t, "GET", "/v1/oauth/authorize?"+q.Encode(), e.token, nil)
			if code != 400 || errCode(doc) != "invalid_scope" {
				t.Fatalf("GET scope %q: %d %v", sc, code, doc)
			}
		}
	})

	t.Run("replayed code is rejected", func(t *testing.T) {
		_, doc := authorize(t, e, loopback, "read", true)
		c := codeFrom(t, doc)
		if status, _ := exchange(t, e, c, verifier, loopback); status != 200 {
			t.Fatal("first exchange failed")
		}
		status, doc := exchange(t, e, c, verifier, loopback)
		if status != 400 || oauthError(doc) != "invalid_grant" {
			t.Fatalf("replay: %d %v", status, doc)
		}
	})

	t.Run("concurrent exchange mints exactly one token", func(t *testing.T) {
		_, doc := authorize(t, e, loopback, "", true)
		c := codeFrom(t, doc)
		var wg sync.WaitGroup
		var mu sync.Mutex
		wins := 0
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if status, _ := exchange(t, e, c, verifier, loopback); status == 200 {
					mu.Lock()
					wins++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if wins != 1 {
			t.Fatalf("%d exchanges succeeded, want 1", wins)
		}
	})

	t.Run("wrong verifier is rejected and burns the code", func(t *testing.T) {
		_, doc := authorize(t, e, loopback, "", true)
		c := codeFrom(t, doc)
		status, out := exchange(t, e, c, strings.Repeat("x", 43), loopback)
		if status != 400 || oauthError(out) != "invalid_grant" {
			t.Fatalf("wrong verifier: %d %v", status, out)
		}
		if status, _ := exchange(t, e, c, verifier, loopback); status != 400 {
			t.Fatal("code should be spent after a bad verifier")
		}
	})

	t.Run("mismatched redirect_uri at exchange is rejected", func(t *testing.T) {
		_, doc := authorize(t, e, loopback, "", true)
		status, out := exchange(t, e, codeFrom(t, doc), verifier, "http://127.0.0.1:9/callback")
		if status != 400 || oauthError(out) != "invalid_grant" {
			t.Fatalf("redirect mismatch: %d %v", status, out)
		}
	})

	t.Run("non-loopback redirects are refused before any code exists", func(t *testing.T) {
		for _, bad := range []string{"http://evil.example/callback", "http://localhost:5000/callback"} {
			code, doc := authorize(t, e, bad, "", true)
			if code != 400 || doc["redirect_to"] != nil {
				t.Fatalf("%s: %d %v", bad, code, doc)
			}
		}
	})

	t.Run("deny redirects with access_denied and no code", func(t *testing.T) {
		code, doc := authorize(t, e, loopback, "", false)
		to, _ := doc["redirect_to"].(string)
		u, _ := url.Parse(to)
		if code != 200 || u.Query().Get("error") != "access_denied" || u.Query().Get("code") != "" {
			t.Fatalf("deny: %d %v", code, doc)
		}
	})

	t.Run("plain PKCE and long client_id are refused", func(t *testing.T) {
		body := authzBody(loopback, "", true)
		body["code_challenge_method"] = "plain"
		if code, _ := e.consent(t, "/v1/oauth/authorize", e.token, body); code != 400 {
			t.Fatalf("plain: %d", code)
		}
		body = authzBody(loopback, "", true)
		body["client_id"] = strings.Repeat("c", 65)
		if code, _ := e.consent(t, "/v1/oauth/authorize", e.token, body); code != 400 {
			t.Fatalf("long client_id: %d", code)
		}
	})

	t.Run("code expires after about a minute", func(t *testing.T) {
		c := withClock(e)
		_, doc := authorize(t, e, loopback, "", true)
		c.add(61 * time.Second)
		status, out := exchange(t, e, codeFrom(t, doc), verifier, loopback)
		if status != 400 || oauthError(out) != "invalid_grant" {
			t.Fatalf("expired code: %d %v", status, out)
		}
	})

	t.Run("agents, anonymous callers, and narrower approvers cannot consent", func(t *testing.T) {
		if code, _ := e.consent(t, "/v1/oauth/authorize", mintToken(t, e, "read,write"), authzBody(loopback, "", true)); code != 403 {
			t.Fatalf("agent consent: %d", code)
		}
		if code, _ := e.consent(t, "/v1/oauth/authorize", "", authzBody(loopback, "", true)); code != 401 {
			t.Fatalf("anonymous consent: %d", code)
		}
		// read+approve, no write: may approve a read login, not a write one.
		narrow := mintToken(t, e, "read,approve")
		if code, doc := e.consent(t, "/v1/oauth/authorize", narrow, authzBody(loopback, "read write", true)); code != 403 {
			t.Fatalf("narrow approver granting write: %d %v", code, doc)
		}
		if code, _ := e.consent(t, "/v1/oauth/authorize", narrow, authzBody(loopback, "read", true)); code != 200 {
			t.Fatalf("narrow approver granting read: %d", code)
		}
	})
}

func TestOAuthConsentCSRF(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/v1/oauth/authorize", "/v1/oauth/device/approve"} {
		body := authzBody(loopback, "", true)
		if path == "/v1/oauth/device/approve" {
			body = map[string]any{"user_code": "BCDF-GHJK", "approve": true}
		}
		for name, h := range map[string]map[string]string{
			"text/plain":       {"Content-Type": "text/plain", "Origin": e.srv.URL},
			"form":             {"Content-Type": "application/x-www-form-urlencoded", "Sec-Fetch-Site": "same-origin"},
			"cross-origin":     {"Content-Type": "application/json", "Origin": "https://evil.example"},
			"cross-site fetch": {"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site"},
			"no origin":        {"Content-Type": "application/json"},
		} {
			if code, doc := e.consentWith(t, path, e.token, body, h); code != 400 && code != 403 {
				t.Fatalf("%s %s: %d %v", path, name, code, doc)
			}
		}
		h := map[string]string{"Content-Type": "application/json; charset=utf-8", "Sec-Fetch-Site": "same-origin"}
		if code, doc := e.consentWith(t, path, e.token, body, h); code == 403 {
			t.Fatalf("%s same-origin fetch refused: %v", path, doc)
		}
	}
}

func TestOAuthConsentRefusesProxyOwner(t *testing.T) {
	e := newEnv(t)
	e.api.TrustProxyAuth = "vercel"
	if code, doc := e.consent(t, "/v1/oauth/authorize", "", authzBody(loopback, "", true)); code != 403 {
		t.Fatalf("proxy owner authorize: %d %v", code, doc)
	}
	_, d := e.form(t, "/v1/auth/device_authorization", url.Values{})
	if code, doc := e.consent(t, "/v1/oauth/device/approve", "", map[string]any{"user_code": d["user_code"], "approve": true}); code != 403 {
		t.Fatalf("proxy owner device approve: %d %v", code, doc)
	}
}

func TestOAuthDeviceFlow(t *testing.T) {
	e := newEnv(t)
	c := withClock(e)
	start := func(scope string) map[string]any {
		t.Helper()
		code, doc := e.form(t, "/v1/auth/device_authorization", url.Values{"client_id": {"pursuit-cli"}, "scope": {scope}})
		if code != 200 || doc["interval"].(float64) != 5 || doc["expires_in"].(float64) != 600 {
			t.Fatalf("device_authorization: %d %v", code, doc)
		}
		if !strings.HasSuffix(doc["verification_uri"].(string), "/device") || doc["verification_uri_complete"] != nil {
			t.Fatalf("verification uris %v", doc)
		}
		return doc
	}
	poll := func(device string) (int, map[string]any) {
		t.Helper()
		return e.form(t, "/v1/auth/token", url.Values{"grant_type": {service.DeviceCodeGrantType}, "device_code": {device}, "client_id": {"pursuit-cli"}})
	}
	decide := func(token, userCode string, approve bool) int {
		t.Helper()
		code, _ := e.consent(t, "/v1/oauth/device/approve", token, map[string]any{"user_code": userCode, "approve": approve})
		return code
	}

	t.Run("approve and non-exact scopes are invalid_scope", func(t *testing.T) {
		for _, sc := range []string{"approve", "APPROVE", "Read", "read approve"} {
			code, doc := e.form(t, "/v1/auth/device_authorization", url.Values{"scope": {sc}})
			if code != 400 || oauthError(doc) != "invalid_scope" {
				t.Fatalf("scope %q: %d %v", sc, code, doc)
			}
		}
		if _, doc := e.form(t, "/v1/auth/device_authorization", url.Values{"client_id": {strings.Repeat("c", 65)}}); oauthError(doc) != "invalid_request" {
			t.Fatalf("long client_id: %v", doc)
		}
	})

	t.Run("happy path with pending, slow_down, approval by a second user", func(t *testing.T) {
		d := start("")
		device, userCode := d["device_code"].(string), d["user_code"].(string)
		if status, doc := poll(device); status != 400 || oauthError(doc) != "authorization_pending" {
			t.Fatalf("first poll: %d %v", status, doc)
		}
		if status, doc := poll(device); status != 400 || oauthError(doc) != "slow_down" {
			t.Fatalf("fast poll: %d %v", status, doc)
		}
		c.add(6 * time.Second)
		if _, doc := poll(device); oauthError(doc) != "slow_down" {
			t.Fatalf("poll inside the raised interval: %v", doc)
		}
		// A different human approves; the token must belong to them.
		other, err := e.svc.LoginIdentity(context.Background(), "other@example.com", "Other", true)
		if err != nil {
			t.Fatal(err)
		}
		_, otherUser, otherTok := meOf(t, e, other)
		code, info := e.json(t, "GET", "/v1/oauth/device?user_code="+url.QueryEscape(strings.ToLower(userCode)), other, nil)
		if code != 200 || info["status"] != "pending" || info["created_at"] == "" {
			t.Fatalf("lookup: %d %v", code, info)
		}
		if decide(other, userCode, true) != 200 {
			t.Fatal("approve failed")
		}
		if code := decide(other, userCode, true); code != 409 {
			t.Fatalf("second decision: %d", code)
		}
		c.add(16 * time.Second)
		status, tok := poll(device)
		if status != 200 {
			t.Fatalf("approved poll: %d %v", status, tok)
		}
		scopes, user, _ := meOf(t, e, tok["access_token"].(string))
		if scopes != `["read","write"]` || user != otherUser {
			t.Fatalf("device token scopes %s user %s, want approver %s", scopes, user, otherUser)
		}
		// Audit: who approved, and the issued token names the approver's token.
		_, a := e.json(t, "GET", "/v1/audit?action=oauth.approve", other, nil)
		if rows, _ := a["data"].([]any); len(rows) != 1 || rows[0].(map[string]any)["token_id"] != otherTok {
			t.Fatalf("oauth.approve audit: %v", a)
		}
		_, a = e.json(t, "GET", "/v1/audit?action=token.create&q=cli+login", other, nil)
		raw, _ := json.Marshal(a["data"])
		if !strings.Contains(string(raw), otherTok) {
			t.Fatalf("token.create metadata lacks approver_token_id %s: %s", otherTok, raw)
		}
		c.add(16 * time.Second)
		if _, doc := poll(device); oauthError(doc) != "invalid_grant" {
			t.Fatalf("device code reused: %v", doc)
		}
	})

	t.Run("denied, and audited", func(t *testing.T) {
		d := start("read")
		if code := decide(e.token, d["user_code"].(string), false); code != 200 {
			t.Fatalf("deny: %d", code)
		}
		if _, doc := poll(d["device_code"].(string)); oauthError(doc) != "access_denied" {
			t.Fatalf("denied poll: %v", doc)
		}
		_, a := e.json(t, "GET", "/v1/audit?action=oauth.deny", e.token, nil)
		if rows, _ := a["data"].([]any); len(rows) != 1 {
			t.Fatalf("oauth.deny audit: %v", a)
		}
	})

	t.Run("expired", func(t *testing.T) {
		d := start("")
		c.add(10*time.Minute + time.Second)
		if _, doc := poll(d["device_code"].(string)); oauthError(doc) != "expired_token" {
			t.Fatalf("expired poll: %v", doc)
		}
		if code := decide(e.token, d["user_code"].(string), true); code != 404 {
			t.Fatalf("approving an expired code: %d", code)
		}
	})

	t.Run("the device code alone approves nothing; narrower approvers get 403", func(t *testing.T) {
		d := start("")
		userCode := d["user_code"].(string)
		if code := decide("", userCode, true); code != 401 {
			t.Fatalf("anonymous approve: %d", code)
		}
		if code := decide(d["device_code"].(string), userCode, true); code != 401 {
			t.Fatalf("device code as bearer: %d", code)
		}
		if code := decide(mintToken(t, e, "read,write"), userCode, true); code != 403 {
			t.Fatalf("agent token approve: %d", code)
		}
		if code := decide(mintToken(t, e, "read,approve"), userCode, true); code != 403 {
			t.Fatalf("approver without write granting write: %d", code)
		}
	})
}

func TestOAuthDeviceMisc(t *testing.T) {
	e := newEnv(t)
	if _, doc := e.form(t, "/v1/auth/token", url.Values{"grant_type": {service.DeviceCodeGrantType}, "device_code": {"nope"}}); oauthError(doc) != "invalid_grant" {
		t.Fatalf("unknown: %v", doc)
	}
	if _, doc := e.form(t, "/v1/auth/token", url.Values{"grant_type": {"password"}}); oauthError(doc) != "unsupported_grant_type" {
		t.Fatalf("grant type: %v", doc)
	}
	_, d := e.form(t, "/v1/auth/device_authorization", url.Values{})
	if _, err := e.svc.DB.GetOAuthGrantByCode(context.Background(), "device", d["device_code"].(string)); err == nil {
		t.Fatal("raw device code found in the store")
	}
	if _, err := e.svc.DB.GetOAuthGrantByCode(context.Background(), "device", auth.Hash(d["device_code"].(string))); err != nil {
		t.Fatal(err)
	}
	// Per-IP limit: 10 starts a minute, this was the first.
	for i := 0; i < 9; i++ {
		if code, _ := e.form(t, "/v1/auth/device_authorization", url.Values{}); code != 200 {
			t.Fatalf("start %d: %d", i+2, code)
		}
	}
	if code, doc := e.form(t, "/v1/auth/device_authorization", url.Values{}); code != 429 {
		t.Fatalf("11th start: %d %v", code, doc)
	}
	_, _, h := e.do(t, "GET", "/device", "", nil, nil)
	if !strings.Contains(h.Get("Content-Security-Policy"), "frame-ancestors 'none'") || h.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("/device framing headers: %q %q", h.Get("Content-Security-Policy"), h.Get("X-Frame-Options"))
	}
}

func TestOAuthRevoke(t *testing.T) {
	e := newEnv(t)
	tok := mintToken(t, e, "read")
	if code, _ := e.form(t, "/v1/auth/revoke", url.Values{"token": {tok}}); code != 200 {
		t.Fatal("revoke failed")
	}
	if code, _ := e.json(t, "GET", "/v1/me", tok, nil); code != 401 {
		t.Fatalf("revoked token still works: %d", code)
	}
	if code, _ := e.form(t, "/v1/auth/revoke", url.Values{"token": {"pst_unknown"}}); code != 200 {
		t.Fatal("unknown token revoke should be 200")
	}
}
