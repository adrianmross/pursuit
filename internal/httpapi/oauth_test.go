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

func oauthError(doc map[string]any) string {
	s, _ := doc["error"].(string)
	return s
}

// scopesOf asks /v1/me which scopes a freshly minted token carries.
func scopesOf(t *testing.T, e *env, token string) string {
	t.Helper()
	code, doc := e.json(t, "GET", "/v1/me", token, nil)
	if code != 200 {
		t.Fatalf("me with minted token: %d %v", code, doc)
	}
	tok, _ := doc["token"].(map[string]any)
	raw, _ := json.Marshal(tok["scopes"])
	return string(raw)
}

const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk_and_some_more"

func authorize(t *testing.T, e *env, redirect, scope string, approve bool) (int, map[string]any) {
	t.Helper()
	return e.json(t, "POST", "/v1/oauth/authorize", e.token, map[string]any{
		"client_id": "pursuit-cli", "redirect_uri": redirect, "code_challenge": auth.PKCEChallenge(verifier),
		"code_challenge_method": "S256", "state": "st-123", "scope": scope, "approve": approve,
	})
}

func codeFrom(t *testing.T, doc map[string]any) string {
	t.Helper()
	to, _ := doc["redirect_to"].(string)
	u, err := url.Parse(to)
	if err != nil || u.Query().Get("state") != "st-123" {
		t.Fatalf("bad redirect_to %q", to)
	}
	return u.Query().Get("code")
}

func TestOAuthLoopbackFlow(t *testing.T) {
	e := newEnv(t)
	const redirect = "http://127.0.0.1:53111/callback"
	exchange := func(code, v, redir string) (int, map[string]any) {
		return e.form(t, "/v1/auth/token", url.Values{
			"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {v},
			"redirect_uri": {redir}, "client_id": {"pursuit-cli"},
		})
	}

	t.Run("happy path grants read and write, not approve", func(t *testing.T) {
		code, doc := authorize(t, e, redirect, "", true)
		if code != 200 {
			t.Fatalf("authorize: %d %v", code, doc)
		}
		status, tok := exchange(codeFrom(t, doc), verifier, redirect)
		if status != 200 || tok["token_type"] != "Bearer" || tok["scope"] != "read write" {
			t.Fatalf("exchange: %d %v", status, tok)
		}
		if got := scopesOf(t, e, tok["access_token"].(string)); got != `["read","write"]` {
			t.Fatalf("default scopes = %s", got)
		}
	})

	t.Run("replayed code is rejected", func(t *testing.T) {
		_, doc := authorize(t, e, redirect, "read", true)
		c := codeFrom(t, doc)
		if status, _ := exchange(c, verifier, redirect); status != 200 {
			t.Fatal("first exchange failed")
		}
		status, doc := exchange(c, verifier, redirect)
		if status != 400 || oauthError(doc) != "invalid_grant" {
			t.Fatalf("replay: %d %v", status, doc)
		}
	})

	t.Run("wrong verifier is rejected and burns the code", func(t *testing.T) {
		_, doc := authorize(t, e, redirect, "", true)
		c := codeFrom(t, doc)
		status, out := exchange(c, strings.Repeat("x", 43), redirect)
		if status != 400 || oauthError(out) != "invalid_grant" {
			t.Fatalf("wrong verifier: %d %v", status, out)
		}
		if status, _ := exchange(c, verifier, redirect); status != 400 {
			t.Fatal("code should be spent after a bad verifier")
		}
	})

	t.Run("mismatched redirect_uri at exchange is rejected", func(t *testing.T) {
		_, doc := authorize(t, e, redirect, "", true)
		status, out := exchange(codeFrom(t, doc), verifier, "http://127.0.0.1:9/callback")
		if status != 400 || oauthError(out) != "invalid_grant" {
			t.Fatalf("redirect mismatch: %d %v", status, out)
		}
	})

	t.Run("non-loopback redirects are refused before any code exists", func(t *testing.T) {
		for _, bad := range []string{
			"http://evil.example/callback", "http://localhost:5000/callback", "https://127.0.0.1:5000/callback",
			"http://127.0.0.1:5000/other", "http://127.0.0.1/callback", "http://127.0.0.1.evil.example:80/callback",
			"http://user@127.0.0.1:5000/callback", "http://127.0.0.1:5000/callback?x=1",
		} {
			code, doc := authorize(t, e, bad, "", true)
			if code != 400 || doc["redirect_to"] != nil {
				t.Fatalf("%s: %d %v", bad, code, doc)
			}
		}
		if code, _ := authorize(t, e, "http://[::1]:5000/callback", "", true); code != 200 {
			t.Fatal("[::1] loopback should be accepted")
		}
	})

	t.Run("deny redirects with access_denied and no code", func(t *testing.T) {
		code, doc := authorize(t, e, redirect, "", false)
		u, _ := url.Parse(doc["redirect_to"].(string))
		if code != 200 || u.Query().Get("error") != "access_denied" || u.Query().Get("code") != "" {
			t.Fatalf("deny: %d %v", code, doc)
		}
	})

	t.Run("plain PKCE is refused", func(t *testing.T) {
		code, _ := e.json(t, "POST", "/v1/oauth/authorize", e.token, map[string]any{
			"redirect_uri": redirect, "code_challenge": verifier, "code_challenge_method": "plain", "state": "s", "approve": true,
		})
		if code != 400 {
			t.Fatalf("plain: %d", code)
		}
	})

	t.Run("code expires after about a minute", func(t *testing.T) {
		c := withClock(e)
		_, doc := authorize(t, e, redirect, "", true)
		c.add(61 * time.Second)
		status, out := exchange(codeFrom(t, doc), verifier, redirect)
		if status != 400 || oauthError(out) != "invalid_grant" {
			t.Fatalf("expired code: %d %v", status, out)
		}
	})

	t.Run("approve scope only when asked and held", func(t *testing.T) {
		_, doc := authorize(t, e, redirect, "read write approve", true)
		_, tok := exchange(codeFrom(t, doc), verifier, redirect)
		if got := scopesOf(t, e, tok["access_token"].(string)); got != `["read","write","approve"]` {
			t.Fatalf("explicit approve = %s", got)
		}
	})

	t.Run("an agent token without approve cannot consent", func(t *testing.T) {
		agent := mintToken(t, e, "read,write")
		code, doc := e.json(t, "POST", "/v1/oauth/authorize", agent, map[string]any{
			"redirect_uri": redirect, "code_challenge": auth.PKCEChallenge(verifier), "code_challenge_method": "S256", "state": "s", "approve": true,
		})
		if code != 403 {
			t.Fatalf("agent consent: %d %v", code, doc)
		}
		if code, _ := e.json(t, "POST", "/v1/oauth/authorize", "", map[string]any{"approve": true}); code != 401 {
			t.Fatalf("anonymous consent: %d", code)
		}
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

func TestOAuthDeviceFlow(t *testing.T) {
	e := newEnv(t)
	c := withClock(e)
	start := func(scope string) map[string]any {
		t.Helper()
		code, doc := e.form(t, "/v1/auth/device_authorization", url.Values{"client_id": {"pursuit-cli"}, "scope": {scope}})
		if code != 200 || doc["interval"].(float64) != 5 || doc["expires_in"].(float64) != 600 {
			t.Fatalf("device_authorization: %d %v", code, doc)
		}
		if !strings.HasSuffix(doc["verification_uri"].(string), "/device") {
			t.Fatalf("verification_uri %v", doc["verification_uri"])
		}
		return doc
	}
	poll := func(device string) (int, map[string]any) {
		t.Helper()
		return e.form(t, "/v1/auth/token", url.Values{"grant_type": {service.DeviceCodeGrantType}, "device_code": {device}, "client_id": {"pursuit-cli"}})
	}
	decide := func(token, userCode string, approve bool) int {
		t.Helper()
		code, _ := e.json(t, "POST", "/v1/oauth/device/approve", token, map[string]any{"user_code": userCode, "approve": approve})
		return code
	}

	t.Run("happy path with pending, slow_down, approval", func(t *testing.T) {
		d := start("")
		device, userCode := d["device_code"].(string), d["user_code"].(string)
		if status, doc := poll(device); status != 400 || oauthError(doc) != "authorization_pending" {
			t.Fatalf("first poll: %d %v", status, doc)
		}
		if status, doc := poll(device); status != 400 || oauthError(doc) != "slow_down" {
			t.Fatalf("fast poll: %d %v", status, doc)
		}
		// slow_down raised the interval to 10s; 6s later is still too fast.
		c.add(6 * time.Second)
		if _, doc := poll(device); oauthError(doc) != "slow_down" {
			t.Fatalf("poll inside the raised interval: %v", doc)
		}
		code, info := e.json(t, "GET", "/v1/oauth/device?user_code="+url.QueryEscape(strings.ToLower(userCode)), e.token, nil)
		if code != 200 || info["status"] != "pending" {
			t.Fatalf("lookup: %d %v", code, info)
		}
		if raw, _ := json.Marshal(info["scopes"]); string(raw) != `["read","write"]` {
			t.Fatalf("lookup scopes %s", raw)
		}
		if code := decide(e.token, userCode, true); code != 200 {
			t.Fatalf("approve: %d", code)
		}
		if code := decide(e.token, userCode, true); code != 404 && code != 409 {
			t.Fatalf("second decision should fail, got %d", code)
		}
		c.add(16 * time.Second)
		status, tok := poll(device)
		if status != 200 {
			t.Fatalf("approved poll: %d %v", status, tok)
		}
		if got := scopesOf(t, e, tok["access_token"].(string)); got != `["read","write"]` {
			t.Fatalf("device default scopes = %s", got)
		}
		c.add(16 * time.Second)
		if _, doc := poll(device); oauthError(doc) != "invalid_grant" {
			t.Fatalf("device code reused: %v", doc)
		}
	})

	t.Run("denied", func(t *testing.T) {
		d := start("read")
		if code := decide(e.token, d["user_code"].(string), false); code != 200 {
			t.Fatalf("deny: %d", code)
		}
		if _, doc := poll(d["device_code"].(string)); oauthError(doc) != "access_denied" {
			t.Fatalf("denied poll: %v", doc)
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

	t.Run("the device code alone approves nothing", func(t *testing.T) {
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
	})

	t.Run("unknown device code and grant type", func(t *testing.T) {
		if _, doc := poll("nope"); oauthError(doc) != "invalid_grant" {
			t.Fatalf("unknown: %v", doc)
		}
		if _, doc := e.form(t, "/v1/auth/token", url.Values{"grant_type": {"password"}}); oauthError(doc) != "unsupported_grant_type" {
			t.Fatalf("grant type: %v", doc)
		}
	})

	t.Run("device codes are stored hashed", func(t *testing.T) {
		d := start("")
		if _, err := e.svc.DB.GetOAuthGrantByCode(context.Background(), "device", d["device_code"].(string)); err == nil {
			t.Fatal("raw device code found in the store")
		}
		if _, err := e.svc.DB.GetOAuthGrantByCode(context.Background(), "device", auth.Hash(d["device_code"].(string))); err != nil {
			t.Fatal(err)
		}
	})
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
