package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPTrustsForwardedForOnlyOnVercel(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/auth/device_authorization", nil)
	r.RemoteAddr = "192.0.2.7:4321"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")

	t.Setenv("VERCEL", "")
	if got := clientIP(r); got != "192.0.2.7" {
		t.Fatalf("self-hosted: got %q, want RemoteAddr host", got)
	}
	t.Setenv("VERCEL", "1")
	if got := clientIP(r); got != "203.0.113.9" {
		t.Fatalf("vercel: got %q, want first X-Forwarded-For hop", got)
	}
}
