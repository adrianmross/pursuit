package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// TestTagsAreTypedOperations pins what a route registered with Huma answers:
// "$schema" on object bodies, the usual error envelope, and ?fields, dry runs
// and Idempotency-Key as on the plain handlers.
func TestTagsAreTypedOperations(t *testing.T) {
	e := newEnv(t)
	post := func(path, body string, h map[string]string) (int, string, http.Header) {
		t.Helper()
		code, b, hdr := e.do(t, "POST", path, e.token, []byte(body), h)
		return code, strings.TrimSpace(string(b)), hdr
	}
	code, body, _ := post("/v1/tags", `{"name":"remote"}`, nil)
	if code != 201 || !strings.Contains(body, `"$schema":"`) || !strings.Contains(body, `/schemas/Tag.json"`) || !strings.Contains(body, `"name":"remote"`) {
		t.Fatalf("create: %d %s", code, body)
	}
	if code, body, _ := post("/v1/tags?fields=name", `{"name":"onsite"}`, nil); code != 201 || body != `{"name":"onsite"}` {
		t.Errorf("projected create: %d %s", code, body)
	}
	if code, body, _ := post("/v1/tags?dry_run=true", `{"name":"dry"}`, nil); code != 200 || !strings.Contains(body, `"dry_run":true`) || !strings.Contains(body, `"name":"dry"`) {
		t.Errorf("dry run: %d %s", code, body)
	}
	key := map[string]string{"Idempotency-Key": "k1"}
	_, first, _ := post("/v1/tags", `{"name":"hybrid"}`, key)
	if code, again, h := post("/v1/tags", `{"name":"hybrid"}`, key); code != 201 || again != first || h.Get("Idempotent-Replayed") != "true" {
		t.Errorf("replay: %d %s, first %s", code, again, first)
	}
	if code, body, _ := post("/v1/tags", `{"name":"other"}`, key); code != 409 || !strings.Contains(body, `"idempotency_mismatch"`) {
		t.Errorf("reused key: %d %s", code, body)
	}
	for name, bad := range map[string]string{"empty name": `{"name":""}`, "unknown field": `{"name":"a","nope":1}`, "long name": `{"name":"` + strings.Repeat("a", 81) + `"}`} {
		if code, body, _ := post("/v1/tags", bad, nil); code != 400 || !strings.HasPrefix(body, `{"error":{"code":"validation",`) {
			t.Errorf("%s: %d %s", name, code, body)
		}
	}
	// What tags cannot say stays in the service: a name of only spaces.
	if code, body, _ := post("/v1/tags", `{"name":"  "}`, nil); code != 400 || !strings.Contains(body, `"validation"`) {
		t.Errorf("blank name: %d %s", code, body)
	}

	code, list := e.json(t, "GET", "/v1/tags?sort=name&limit=2", e.token, nil)
	if code != 200 || len(list["data"].([]any)) != 2 || !strings.HasSuffix(list["$schema"].(string), "/schemas/PageTag.json") {
		t.Fatalf("list: %d %v", code, list)
	}
	next := list["page"].(map[string]any)["next_cursor"].(string)
	if code, page := e.json(t, "GET", "/v1/tags?sort=name&limit=2&fields=name&cursor="+next, e.token, nil); code != 200 || len(page["data"].([]any)) != 1 || len(page["data"].([]any)[0].(map[string]any)) != 1 {
		t.Errorf("second page: %d %v", code, page)
	}
	if code, _ := e.json(t, "GET", "/v1/tags?nope=1", e.token, nil); code != 400 {
		t.Errorf("unknown query parameter: %d", code)
	}
	if code, _ := e.json(t, "GET", "/v1/tags", "", nil); code != 401 {
		t.Errorf("no token: %d", code)
	}
	id := list["data"].([]any)[0].(map[string]any)["id"].(string)
	if code, del := e.json(t, "DELETE", "/v1/tags/"+id, e.token, nil); code != 200 || del["deleted"] != true {
		t.Errorf("delete: %d %v", code, del)
	}
	if code, _ := e.json(t, "DELETE", "/v1/tags/"+id, e.token, nil); code != 404 {
		t.Errorf("delete again: %d", code)
	}
	// The schema a body names is served without a token.
	if code, schema := e.json(t, "GET", "/schemas/Tag.json", "", nil); code != 200 || schema["type"] != "object" {
		t.Errorf("schema: %d %v", code, schema)
	}
}
