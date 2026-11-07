package httpapi

import (
	"net/http"
	"testing"

	"github.com/pursuit/pursuit/internal/domain"
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

	// A verified email with no account: 403 "paused" while closed, the
	// usual 401 otherwise, and unrelated errors pass through.
	e.api.TenantMode, e.api.SignupsClosed = "multi", true
	if err := e.api.signupsPaused(domain.Unauthorized()); !domain.IsCode(err, "forbidden") {
		t.Fatalf("closed: got %v, want forbidden", err)
	}
	if err := e.api.signupsPaused(domain.Validation("x")); !domain.IsCode(err, "validation") {
		t.Fatalf("closed passes other errors through: got %v", err)
	}
	e.api.SignupsClosed = false
	if err := e.api.signupsPaused(domain.Unauthorized()); !domain.IsCode(err, "unauthorized") {
		t.Fatalf("open: got %v, want unauthorized", err)
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
