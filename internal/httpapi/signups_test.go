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
