package httpapi

import (
	"net/http"
	"testing"
)

func TestSignupsToggle(t *testing.T) {
	e := newEnv(t)
	for _, c := range []struct {
		mode   string
		closed bool
		open   bool
	}{
		{"multi", false, true},
		{"multi", true, false},
		{"single", false, false},
	} {
		e.api.TenantMode, e.api.SignupsClosed = c.mode, c.closed
		code, doc := e.json(t, http.MethodGet, "/v1/auth/providers", "", nil)
		if code != 200 || doc["signups_open"] != c.open {
			t.Fatalf("%s closed=%v: %d signups_open=%v, want %v", c.mode, c.closed, code, doc["signups_open"], c.open)
		}
	}
}

// Pasting an API token to sign in is for self-hosting: multi-tenant never offers it.
func TestTokenLoginOnlyWhenSelfHosted(t *testing.T) {
	e := newEnv(t)
	allowed := func() any {
		_, doc := e.json(t, http.MethodGet, "/v1/auth/providers", "", nil)
		return doc["allow_token_login"]
	}
	if allowed() != true {
		t.Fatalf("single-tenant should offer token login")
	}
	e.api.HideTokenLogin = true
	if allowed() != false {
		t.Fatalf("PURSUIT_HIDE_TOKEN_LOGIN should hide it")
	}
	e.api.HideTokenLogin = false
	e.api.TenantMode = "multi"
	if allowed() != false {
		t.Fatalf("multi-tenant should hide it")
	}
}
