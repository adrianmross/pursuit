package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestOAuthMetadata(t *testing.T) {
	e := newEnv(t)
	get := func(path string) map[string]any {
		t.Helper()
		code, b, h := e.do(t, "GET", path, "", nil, nil)
		if code != 200 {
			t.Fatalf("GET %s: %d %s", path, code, b)
		}
		if h.Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("GET %s: no CORS header", path)
		}
		var doc map[string]any
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatal(err)
		}
		return doc
	}

	as := get("/.well-known/oauth-authorization-server")
	iss := e.srv.URL
	if as["issuer"] != iss || as["token_endpoint"] != iss+"/v1/auth/token" || as["authorization_endpoint"] != iss+"/authorize" {
		t.Errorf("server metadata: %v", as)
	}
	if m, _ := as["code_challenge_methods_supported"].([]any); len(m) != 1 || m[0] != "S256" {
		t.Errorf("code_challenge_methods_supported: %v", m)
	}
	// Every advertised endpoint must be a route that answers.
	for _, k := range []string{"token_endpoint", "revocation_endpoint", "device_authorization_endpoint"} {
		path := as[k].(string)[len(iss):]
		if code, _, _ := e.do(t, http.MethodPost, path, "", nil, nil); code == 404 || code == 405 {
			t.Errorf("%s %s answered %d", k, path, code)
		}
	}

	rs := get("/.well-known/oauth-protected-resource")
	if rs["resource"] != iss || rs["authorization_servers"].([]any)[0] != iss {
		t.Errorf("resource metadata: %v", rs)
	}
	if rs := get("/.well-known/oauth-protected-resource/mcp"); rs["resource"] != iss+"/mcp" {
		t.Errorf("path-suffixed resource: %v", rs["resource"])
	}
}
