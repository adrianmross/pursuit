package apiclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBypassOnlyWithTokenOrOAuth(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("x-vercel-protection-bypass")
	}))
	defer srv.Close()

	for _, tc := range []struct {
		token, path, want string
	}{
		{"pst_x", "/v1/applications", "secret"},
		{"", "/v1/auth/token", "secret"},
		// No bearer behind trust-proxy auth would be the owner: never bypass.
		{"", "/v1/applications", ""},
	} {
		got = "unset"
		c := &Client{BaseURL: srv.URL, Token: tc.token, Bypass: "secret"}
		if _, err := c.Do(nil, "GET", tc.path, nil, nil, "", Options{}); err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("token=%q path=%s: bypass %q, want %q", tc.token, tc.path, got, tc.want)
		}
	}
}
