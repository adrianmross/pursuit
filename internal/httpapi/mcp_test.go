package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// rpc posts one JSON-RPC message to /mcp and returns the status, the decoded
// body and the response headers.
func (e *env) rpc(t *testing.T, token string, id any, method string, params any) (int, map[string]any, http.Header) {
	t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id != nil {
		msg["id"] = id
	}
	raw, _ := json.Marshal(msg)
	code, b, h := e.do(t, "POST", "/mcp", token, raw, map[string]string{"Content-Type": "application/json"})
	var doc map[string]any
	_ = json.Unmarshal(b, &doc)
	return code, doc, h
}

func (e *env) callTool(t *testing.T, token, name string, args map[string]any) map[string]any {
	t.Helper()
	code, doc, _ := e.rpc(t, token, 1, "tools/call", map[string]any{"name": name, "arguments": args})
	if code != 200 || doc["error"] != nil {
		t.Fatalf("tools/call %s: %d %v", name, code, doc)
	}
	return doc["result"].(map[string]any)
}

func TestMCPRequiresSignIn(t *testing.T) {
	e := newEnv(t)
	for name, token := range map[string]string{"none": "", "bad": "pst_nope"} {
		code, _, h := e.rpc(t, token, 1, "tools/list", nil)
		ch := h.Get("WWW-Authenticate")
		if code != 401 || !strings.HasPrefix(ch, "Bearer ") || !strings.Contains(ch, `resource_metadata="`+e.srv.URL+`/.well-known/oauth-protected-resource/mcp"`) {
			t.Errorf("%s: %d %q", name, code, ch)
		}
		if name == "bad" && !strings.Contains(ch, `error="invalid_token"`) {
			t.Errorf("bad token challenge: %q", ch)
		}
	}
	if code, _, h := e.do(t, "GET", "/mcp", e.token, nil, nil); code != 405 || h.Get("Allow") != "POST" {
		t.Errorf("GET /mcp: %d %q", code, h.Get("Allow"))
	}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	if code, _, _ := e.do(t, "POST", "/mcp", e.token, raw, map[string]string{"Origin": "https://evil.example.com"}); code != 403 {
		t.Errorf("foreign origin: %d", code)
	}
}

func TestMCPProtocol(t *testing.T) {
	e := newEnv(t)
	_, doc, _ := e.rpc(t, e.token, 1, "initialize", map[string]any{"protocolVersion": "2025-06-18"})
	res := doc["result"].(map[string]any)
	if res["protocolVersion"] != "2025-06-18" || res["serverInfo"].(map[string]any)["name"] != "pursuit" {
		t.Errorf("initialize: %v", doc)
	}
	_, doc, _ = e.rpc(t, e.token, 2, "initialize", map[string]any{"protocolVersion": "1999-01-01"})
	if doc["result"].(map[string]any)["protocolVersion"] != mcpVersions[0] {
		t.Errorf("unknown version should get the newest: %v", doc)
	}
	if code, _, _ := e.rpc(t, e.token, nil, "notifications/initialized", nil); code != 202 {
		t.Errorf("notification: %d", code)
	}
	if _, doc, _ := e.rpc(t, e.token, 3, "nope", nil); doc["error"].(map[string]any)["code"] != float64(-32601) {
		t.Errorf("unknown method: %v", doc)
	}
	if _, doc, _ := e.rpc(t, e.token, 4, "ping", nil); doc["id"] != float64(4) || doc["error"] != nil {
		t.Errorf("ping: %v", doc)
	}

	// The tools never approve, reject or submit, and each names the scope it needs.
	_, doc, _ = e.rpc(t, e.token, 5, "tools/list", nil)
	var names []string
	for _, it := range doc["result"].(map[string]any)["tools"].([]any) {
		tool := it.(map[string]any)
		names = append(names, tool["name"].(string))
		if tool["securitySchemes"] == nil || tool["_meta"].(map[string]any)["securitySchemes"] == nil || tool["inputSchema"] == nil {
			t.Errorf("tool %v is missing securitySchemes or inputSchema", tool["name"])
		}
		for _, bad := range []string{"approve", "reject", "submit", "transition"} {
			if strings.Contains(tool["name"].(string), bad) {
				t.Errorf("tool %s must not exist", tool["name"])
			}
		}
	}
	if len(names) != 5 {
		t.Errorf("tools = %v", names)
	}
}

func TestMCPTools(t *testing.T) {
	e := newEnv(t)
	res := e.callTool(t, e.token, "capture_posting", map[string]any{"posting_url": "https://example.com/jobs/1", "company": "Example", "title": "Engineer"})
	app := res["structuredContent"].(map[string]any)["application"].(map[string]any)
	id := app["id"].(string)
	if res["isError"] == true || res["structuredContent"].(map[string]any)["created"] != true {
		t.Fatalf("capture: %v", res)
	}
	if again := e.callTool(t, e.token, "capture_posting", map[string]any{"posting_url": "https://example.com/jobs/1", "company": "Example", "title": "Engineer"}); again["structuredContent"].(map[string]any)["created"] != false {
		t.Errorf("same posting should not create a second application: %v", again)
	}

	list := e.callTool(t, e.token, "list_applications", map[string]any{"q": "Engineer", "limit": 5})
	if data := list["structuredContent"].(map[string]any)["data"].([]any); len(data) != 1 {
		t.Errorf("list: %v", list)
	}
	if got := e.callTool(t, e.token, "get_application", map[string]any{"id": id}); got["structuredContent"].(map[string]any)["company"] != "Example" {
		t.Errorf("get: %v", got)
	}
	if prep := e.callTool(t, e.token, "prepare_application", map[string]any{"id": id}); prep["isError"] == true {
		t.Errorf("prepare: %v", prep)
	}
	if q := e.callTool(t, e.token, "enqueue_prepare", map[string]any{"id": id}); q["isError"] == true {
		t.Errorf("enqueue: %v", q)
	}

	// Failures are results the model can read, not protocol errors.
	for _, c := range []struct {
		tool string
		args map[string]any
	}{{"get_application", map[string]any{"id": "nope"}}, {"list_applications", map[string]any{"limit": 500}}, {"capture_posting", map[string]any{}}} {
		r := e.callTool(t, e.token, c.tool, c.args)
		if r["isError"] != true || r["content"].([]any)[0].(map[string]any)["text"] == "" {
			t.Errorf("%s %v: %v", c.tool, c.args, r)
		}
	}
	if _, doc, _ := e.rpc(t, e.token, 9, "tools/call", map[string]any{"name": "approve_application"}); doc["error"] == nil {
		t.Errorf("unknown tool should be a protocol error: %v", doc)
	}
}

func TestMCPScopes(t *testing.T) {
	e := newEnv(t)
	read := mintToken(t, e, "read")
	if r := e.callTool(t, read, "list_applications", nil); r["isError"] == true {
		t.Errorf("read token listing: %v", r)
	}
	r := e.callTool(t, read, "capture_posting", map[string]any{"posting_url": "https://example.com/jobs/2", "company": "X", "title": "Y"})
	meta, _ := r["_meta"].(map[string]any)
	ch, _ := meta["mcp/www_authenticate"].([]any)
	if r["isError"] != true || len(ch) != 1 || !strings.Contains(ch[0].(string), `error="insufficient_scope"`) || !strings.Contains(ch[0].(string), "error_description=") {
		t.Errorf("read token writing: %v", r)
	}
}

func TestMCPWithOAuthToken(t *testing.T) {
	e := newEnv(t)
	connectFor := func(resource string) string {
		_, reg := e.register(t, `{"client_name":"ChatGPT","redirect_uris":["https://chatgpt.com/cb"]}`)
		id := reg["client_id"].(string)
		b := authzFor(id, "https://chatgpt.com/cb", true)
		b["scope"], b["resource"] = "read write", resource
		code, out := e.consent(t, "/v1/oauth/authorize", e.token, b)
		if code != 200 {
			t.Fatalf("authorize: %d %v", code, out)
		}
		u, _ := url.Parse(out["redirect_to"].(string))
		status, tok := e.form(t, "/v1/auth/token", url.Values{
			"grant_type": {"authorization_code"}, "code": {u.Query().Get("code")}, "code_verifier": {verifier},
			"redirect_uri": {"https://chatgpt.com/cb"}, "client_id": {id}, "resource": {resource},
		})
		if status != 200 {
			t.Fatalf("token: %d %v", status, tok)
		}
		return tok["access_token"].(string)
	}

	good := connectFor(e.srv.URL + "/mcp")
	if r := e.callTool(t, good, "capture_posting", map[string]any{"posting_url": "https://example.com/jobs/3", "company": "A", "title": "B"}); r["isError"] == true {
		t.Errorf("token for /mcp: %v", r)
	}
	// A token issued for another resource on this host is not accepted here.
	if code, _, h := e.rpc(t, connectFor(e.srv.URL+"/other"), 1, "tools/list", nil); code != 401 || !strings.Contains(h.Get("WWW-Authenticate"), "invalid_token") {
		t.Errorf("token for another resource: %d %q", code, h.Get("WWW-Authenticate"))
	}
}
