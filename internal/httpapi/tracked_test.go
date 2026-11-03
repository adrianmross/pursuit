package httpapi

import (
	"net/http"
	"testing"
)

func TestTrackedBy(t *testing.T) {
	e := newEnv(t)
	add := func(company, title, status string) map[string]any {
		code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{"company": company, "title": title, "status": status})
		if code != 201 {
			t.Fatalf("create %d %#v", code, app)
		}
		return app
	}
	patch := func(app map[string]any, body map[string]any) (int, map[string]any) {
		body["version"] = app["version"]
		return e.json(t, http.MethodPatch, "/v1/applications/"+app["id"].(string), e.token, body)
	}
	primary := add("Snowflake", "Software Engineer", "screening")
	child := add("Snowflake", "Data Engineer", "to_apply")
	other := add("Example", "Engineer", "to_apply")

	if code, bad := patch(other, map[string]any{"tracked_by": primary["id"]}); code != 400 {
		t.Fatalf("another company: %d %#v", code, bad)
	}
	if code, bad := patch(primary, map[string]any{"tracked_by": primary["id"]}); code != 400 {
		t.Fatalf("itself: %d %#v", code, bad)
	}
	code, child := patch(child, map[string]any{"tracked_by": primary["id"]})
	if code != 200 || child["tracked_by"] != primary["id"] {
		t.Fatalf("track: %d %#v", code, child)
	}
	if code, bad := patch(primary, map[string]any{"tracked_by": child["id"]}); code != 400 {
		t.Fatalf("chain: %d %#v", code, bad)
	}
	if code, bad := e.json(t, http.MethodPost, "/v1/applications/"+child["id"].(string)+"/prepare", e.token, map[string]any{}); code != 409 {
		t.Fatalf("prepare a tracked application: %d %#v", code, bad)
	}

	// Not considered for one posting: an outcome on that row only, and a
	// later edit keeps it.
	code, child = e.json(t, http.MethodPost, "/v1/applications/"+child["id"].(string)+"/transition", e.token, map[string]any{"version": child["version"], "status": "rejected"})
	if code != 200 || child["outcome"] != "rejected" || child["tracked_by"] != primary["id"] {
		t.Fatalf("reject child: %d %#v", code, child)
	}
	code, child = patch(child, map[string]any{"notes": "other team"})
	if code != 200 || child["outcome"] != "rejected" {
		t.Fatalf("edit keeps outcome: %d %#v", code, child)
	}

	code, child = patch(child, map[string]any{"tracked_by": ""})
	if code != 200 || child["tracked_by"] != nil {
		t.Fatalf("untrack: %d %#v", code, child)
	}
}
