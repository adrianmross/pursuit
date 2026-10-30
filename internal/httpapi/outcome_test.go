package httpapi

import (
	"net/http"
	"testing"
)

func TestOutcomeOverlaysStage(t *testing.T) {
	e := newEnv(t)
	code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "Example", "title": "Engineer", "status": "interviewing",
	})
	if code != 201 || app["stage"].(float64) != 4 {
		t.Fatalf("create %d %#v", code, app)
	}
	id, ver := app["id"].(string), int(app["version"].(float64))

	code, rejected := e.json(t, http.MethodPost, "/v1/applications/"+id+"/transition", e.token, map[string]any{"version": ver, "status": "rejected"})
	if code != 200 || rejected["status"] != "interviewing" || rejected["outcome"] != "rejected" || rejected["stage"].(float64) != 4 {
		t.Fatalf("reject kept stage: %d %#v", code, rejected)
	}
	ver = int(rejected["version"].(float64))

	for filter, want := range map[string]float64{"status=rejected": 1, "status=interviewing": 0, "status_not=interviewing": 1} {
		code, list := e.json(t, http.MethodGet, "/v1/applications?"+filter, e.token, nil)
		if code != 200 || list["page"].(map[string]any)["total"].(float64) != want {
			t.Fatalf("%s: %d %#v", filter, code, list)
		}
	}

	code, same := e.json(t, http.MethodPost, "/v1/applications/"+id+"/transition", e.token, map[string]any{"version": ver, "status": "rejected"})
	if code != 400 {
		t.Fatalf("repeat outcome should be unchanged: %d %#v", code, same)
	}

	code, back := e.json(t, http.MethodPost, "/v1/applications/"+id+"/transition", e.token, map[string]any{"version": ver, "status": "interviewing"})
	if code != 200 || back["outcome"] != "" {
		t.Fatalf("stage clears outcome: %d %#v", code, back)
	}
	ver = int(back["version"].(float64))

	// waiting overlays without hiding the application from its stage.
	code, waiting := e.json(t, http.MethodPost, "/v1/applications/"+id+"/transition", e.token, map[string]any{"version": ver, "status": "waiting"})
	if code != 200 || waiting["outcome"] != "waiting" || waiting["status"] != "interviewing" {
		t.Fatalf("waiting: %d %#v", code, waiting)
	}
	code, list := e.json(t, http.MethodGet, "/v1/applications?status=interviewing", e.token, nil)
	if code != 200 || list["page"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("waiting stays under its stage: %d %#v", code, list)
	}
}

func TestInterviews(t *testing.T) {
	e := newEnv(t)
	code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "Example", "title": "Engineer", "status": "screening",
	})
	if code != 201 {
		t.Fatal(app)
	}
	id := app["id"].(string)

	code, first := e.json(t, http.MethodPost, "/v1/applications/"+id+"/interviews", e.token, map[string]any{"kind": "Recruiter call", "prep": "Why this company"})
	if code != 201 || first["stage"] != "screening" {
		t.Fatalf("add defaults to the current stage: %d %#v", code, first)
	}
	code, second := e.json(t, http.MethodPost, "/v1/applications/"+id+"/interviews", e.token, map[string]any{"kind": "Hiring manager", "scheduled_at": "2026-11-05"})
	if code != 201 || second["scheduled_at"] == nil {
		t.Fatalf("add scheduled: %d %#v", code, second)
	}
	if code, bad := e.json(t, http.MethodPost, "/v1/applications/"+id+"/interviews", e.token, map[string]any{"stage": "rejected"}); code != 400 {
		t.Fatalf("an outcome is not a stage: %d %#v", code, bad)
	}
	// Every stage can hold rounds, several at once.
	code, applied := e.json(t, http.MethodPost, "/v1/applications/"+id+"/interviews", e.token, map[string]any{"stage": "Prepared", "kind": "Resume review"})
	if code != 201 || applied["stage"] != "awaiting_review" {
		t.Fatalf("prepared round: %d %#v", code, applied)
	}

	code, done := e.json(t, http.MethodPatch, "/v1/interviews/"+first["id"].(string), e.token, map[string]any{"version": first["version"], "done": true, "notes": "went well"})
	if code != 200 || done["done_at"] == nil || done["notes"] != "went well" {
		t.Fatalf("done: %d %#v", code, done)
	}

	code, got := e.json(t, http.MethodGet, "/v1/applications/"+id, e.token, nil)
	counts := got["interviews"].(map[string]any)
	screening, prepared := counts["3"].(map[string]any), counts["1"].(map[string]any)
	if code != 200 || screening["done"].(float64) != 1 || screening["total"].(float64) != 2 || prepared["total"].(float64) != 1 {
		t.Fatalf("per-stage fill: %d %#v", code, got)
	}

	code, del := e.json(t, http.MethodDelete, "/v1/interviews/"+second["id"].(string)+"?version=1", e.token, nil)
	if code != 200 || del["deleted"] != true {
		t.Fatalf("delete: %d %#v", code, del)
	}
	code, got = e.json(t, http.MethodGet, "/v1/applications/"+id, e.token, nil)
	if got["interviews"].(map[string]any)["3"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("after delete: %d %#v", code, got)
	}
}
