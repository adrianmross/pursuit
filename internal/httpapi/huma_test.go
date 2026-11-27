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

// TestSliceListsAndSmallResources covers the lists that are loaded whole
// (webhooks, standing answers) and the bodies whose rules are struct tags.
func TestSliceListsAndSmallResources(t *testing.T) {
	e := newEnv(t)
	post := func(path, body string) (int, string) {
		t.Helper()
		code, b, _ := e.do(t, "POST", path, e.token, []byte(body), nil)
		return code, strings.TrimSpace(string(b))
	}
	for _, k := range []string{"a", "b", "c"} {
		if code, body := post("/v1/standing", `{"key":"`+k+`","answer":"x"}`); code != 200 || !strings.Contains(body, `"key":"`+k+`"`) {
			t.Fatalf("set standing: %d %s", code, body)
		}
	}
	code, all := e.json(t, "GET", "/v1/standing", e.token, nil)
	if code != 200 || len(all["data"].([]any)) != 3 || all["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("whole list: %d %v", code, all)
	}
	code, first := e.json(t, "GET", "/v1/standing?limit=2", e.token, nil)
	next, _ := first["page"].(map[string]any)["next_cursor"].(string)
	if code != 200 || len(first["data"].([]any)) != 2 || next == "" {
		t.Fatalf("first page: %d %v", code, first)
	}
	if code, rest := e.json(t, "GET", "/v1/standing?limit=2&cursor="+next, e.token, nil); code != 200 || len(rest["data"].([]any)) != 1 {
		t.Errorf("second page: %d %v", code, rest)
	}
	for _, bad := range []string{"?cursor=nope", "?sort=key", "?fields=key", "?limit=201"} {
		if code, _ := e.json(t, "GET", "/v1/standing"+bad, e.token, nil); code != 400 {
			t.Errorf("standing%s: %d", bad, code)
		}
	}
	if code, del := e.json(t, "DELETE", "/v1/standing/a", e.token, nil); code != 200 || del["deleted"] != true {
		t.Errorf("delete standing: %d %v", code, del)
	}

	// An optional field may be left out; a wrong one is refused by its tag.
	if code, body := post("/v1/webhooks", `{"url":"https://example.com/hook"}`); code != 201 || !strings.Contains(body, `"url":"https://example.com/hook"`) {
		t.Errorf("webhook without secret: %d %s", code, body)
	}
	if code, list := e.json(t, "GET", "/v1/webhooks", e.token, nil); code != 200 || len(list["data"].([]any)) != 1 {
		t.Errorf("webhooks: %d %v", code, list)
	}
	if code, body := post("/v1/fields", `{"name":"Level","type":"text"}`); code != 201 || !strings.Contains(body, `"type":"text"`) {
		t.Errorf("field: %d %s", code, body)
	}
	for name, bad := range map[string][2]string{
		"field type":    {"/v1/fields", `{"name":"Level","type":"colour"}`},
		"field no type": {"/v1/fields", `{"name":"Level"}`},
		"webhook url":   {"/v1/webhooks", `{}`},
		"empty answer":  {"/v1/standing", `{"key":"k","answer":""}`},
	} {
		if code, body := post(bad[0], bad[1]); code != 400 || !strings.HasPrefix(body, `{"error":{"code":"validation",`) {
			t.Errorf("%s: %d %s", name, code, body)
		}
	}
}
