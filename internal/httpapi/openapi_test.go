package httpapi

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/pursuit/pursuit/internal/spec"
)

// TestOpenAPICoversRoutes fails when a route registered in Handler is
// missing from the served OpenAPI document.
func TestOpenAPICoversRoutes(t *testing.T) {
	src, err := os.ReadFile("http.go")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(spec.Document(), &doc); err != nil {
		t.Fatal(err)
	}
	routes := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/[^"]*)"`).FindAllStringSubmatch(string(src), -1)
	if len(routes) < 50 {
		t.Fatalf("found only %d routes in http.go; did the registration style change?", len(routes))
	}
	for _, m := range routes {
		method, path := strings.ToLower(m[1]), m[2]
		if path == "/" { // dashboard SPA shell, not an API route
			continue
		}
		if _, ok := doc.Paths[path][method]; !ok {
			t.Errorf("%s %s is routed but missing from internal/spec", m[1], path)
		}
	}
}

// TestOpenAPIResponsesMatch sends real requests and checks each response
// against the schema the document declares for it: every required field is
// there, and every field is in the schema.
func TestOpenAPIResponsesMatch(t *testing.T) {
	e := newEnv(t)
	var doc map[string]any
	if err := json.Unmarshal(spec.Document(), &doc); err != nil {
		t.Fatal(err)
	}
	send := func(method, path string, body any) map[string]any {
		t.Helper()
		code, out := e.json(t, method, path, e.token, body)
		if code >= 300 {
			t.Fatalf("%s %s: %d %v", method, path, code, out)
		}
		return out
	}
	first := func(path string) string {
		t.Helper()
		return send("GET", path, nil)["data"].([]any)[0].(map[string]any)["id"].(string)
	}
	id := send("POST", "/v1/applications", map[string]any{"company": "Example", "title": "Engineer", "posting_url": "https://example.com/jobs/1"})["id"].(string)
	round := send("POST", "/v1/applications/"+id+"/rounds", map[string]any{"kind": "Phone screen", "stage": "applied"})["id"].(string)
	task := send("POST", "/v1/applications/"+id+"/tasks", map[string]any{"title": "Send references"})["id"].(string)
	send("POST", "/v1/applications/"+id+"/tags", map[string]any{"name": "remote"})
	send("POST", "/v1/applications/"+id+"/events", map[string]any{"kind": "note", "body": "hello"})
	send("POST", "/v1/standing", map[string]any{"key": "work_authorization", "answer": "yes"})
	send("POST", "/v1/fields", map[string]any{"name": "Salary range", "type": "text"})
	send("POST", "/v1/webhooks", map[string]any{"url": "https://example.com/hook"})
	document := send("POST", "/v1/documents", map[string]any{"kind": "base_resume", "filename": "r.txt", "content_type": "text/plain", "body": "resume"})["id"].(string)

	for _, c := range []struct{ method, route, path string }{
		{"GET", "/v1/me", "/v1/me"},
		{"GET", "/v1/tokens", "/v1/tokens"},
		{"GET", "/v1/companies", "/v1/companies"},
		{"GET", "/v1/companies/{id}", "/v1/companies/" + first("/v1/companies")},
		{"GET", "/v1/roles", "/v1/roles"},
		{"GET", "/v1/roles/{id}", "/v1/roles/" + first("/v1/roles")},
		{"GET", "/v1/applications", "/v1/applications"},
		{"GET", "/v1/applications/{id}", "/v1/applications/" + id},
		{"GET", "/v1/applications/{id}/events", "/v1/applications/" + id + "/events"},
		{"GET", "/v1/applications/{id}/rounds", "/v1/applications/" + id + "/rounds"},
		{"GET", "/v1/rounds/{id}", "/v1/rounds/" + round},
		{"GET", "/v1/applications/{id}/tasks", "/v1/applications/" + id + "/tasks"},
		{"GET", "/v1/tasks/{id}", "/v1/tasks/" + task},
		{"GET", "/v1/applications/{id}/artifacts", "/v1/applications/" + id + "/artifacts"},
		{"GET", "/v1/applications/{id}/questions", "/v1/applications/" + id + "/questions"},
		{"GET", "/v1/events", "/v1/events"},
		{"GET", "/v1/tags", "/v1/tags"},
		{"GET", "/v1/fields", "/v1/fields"},
		{"GET", "/v1/audit", "/v1/audit"},
		{"GET", "/v1/links", "/v1/links"},
		{"GET", "/v1/standing", "/v1/standing"},
		{"GET", "/v1/profile", "/v1/profile"},
		{"GET", "/v1/documents", "/v1/documents"},
		{"GET", "/v1/documents/{id}", "/v1/documents/" + document},
		{"GET", "/v1/webhooks", "/v1/webhooks"},
		{"GET", "/v1/caps", "/v1/caps"},
		{"GET", "/v1/export/json", "/v1/export/json"},
		{"GET", "/v1/auth/providers", "/v1/auth/providers"},
		{"POST", "/v1/queue/claim", "/v1/queue/claim"},
	} {
		var body []byte
		if c.method == "POST" {
			body = []byte(`{"worker":"w"}`)
		}
		code, b, _ := e.do(t, c.method, c.path, e.token, body, map[string]string{"Content-Type": "application/json"})
		schema, _ := lookup(doc, "paths", c.route, strings.ToLower(c.method), "responses", strconv.Itoa(code), "content", "application/json", "schema").(map[string]any)
		if schema == nil {
			t.Errorf("%s %s answered %d, which the document has no JSON schema for: %s", c.method, c.route, code, b)
			continue
		}
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatalf("%s %s: %v", c.method, c.path, err)
		}
		for _, problem := range shapeErrors(doc, schema, v, c.method+" "+c.route) {
			t.Error(problem)
		}
	}
}

func lookup(v any, keys ...string) any {
	for _, k := range keys {
		m, _ := v.(map[string]any)
		v = m[k]
	}
	return v
}

// shapeErrors checks v against the subset of JSON Schema internal/spec emits.
func shapeErrors(doc, schema map[string]any, v any, at string) []string {
	if ref, ok := schema["$ref"].(string); ok {
		s, _ := lookup(doc, strings.Split(strings.TrimPrefix(ref, "#/"), "/")...).(map[string]any)
		return shapeErrors(doc, s, v, at)
	}
	if options, ok := schema["anyOf"].([]any); ok {
		var last []string
		for _, s := range options {
			if last = shapeErrors(doc, s.(map[string]any), v, at); len(last) == 0 {
				return nil
			}
		}
		return last
	}
	types := map[string]bool{}
	switch t := schema["type"].(type) {
	case string:
		types[t] = true
	case []any:
		for _, s := range t {
			types[s.(string)] = true
		}
	}
	if len(types) == 0 {
		return nil
	}
	var got string
	switch n := v.(type) {
	case nil:
		got = "null"
	case bool:
		got = "boolean"
	case string:
		got = "string"
	case float64:
		got = "number"
		if types["integer"] && n == float64(int64(n)) {
			got = "integer"
		}
	case []any:
		got = "array"
	case map[string]any:
		got = "object"
	}
	if !types[got] {
		return []string{fmt.Sprintf("%s: got %s, schema says %v", at, got, schema["type"])}
	}
	var out []string
	switch val := v.(type) {
	case []any:
		if items, ok := schema["items"].(map[string]any); ok {
			for i, item := range val {
				out = append(out, shapeErrors(doc, items, item, fmt.Sprintf("%s[%d]", at, i))...)
			}
		}
	case map[string]any:
		props, _ := schema["properties"].(map[string]any)
		extra, _ := schema["additionalProperties"].(map[string]any)
		required, _ := schema["required"].([]any)
		for _, r := range required {
			if _, ok := val[r.(string)]; !ok {
				out = append(out, fmt.Sprintf("%s: required field %s is missing", at, r))
			}
		}
		for k, field := range val {
			if s, ok := props[k].(map[string]any); ok {
				out = append(out, shapeErrors(doc, s, field, at+"."+k)...)
			} else if extra != nil {
				out = append(out, shapeErrors(doc, extra, field, at+"."+k)...)
			} else if props != nil {
				out = append(out, fmt.Sprintf("%s: field %s is not in the schema", at, k))
			}
		}
	}
	return out
}
