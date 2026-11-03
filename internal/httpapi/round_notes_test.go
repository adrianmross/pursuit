package httpapi

import (
	"net/http"
	"testing"
)

func TestRoundNotes(t *testing.T) {
	e := newEnv(t)
	code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{"company": "Snowflake", "title": "Engineer", "status": "screening"})
	if code != 201 {
		t.Fatal(app)
	}
	code, round := e.json(t, http.MethodPost, "/v1/applications/"+app["id"].(string)+"/rounds", e.token, map[string]any{"kind": "Recruiter call", "prep": "Read the 10-K."})
	if code != 201 || round["notes"] != "## Prep\nRead the 10-K." {
		t.Fatalf("prep lands under ## Prep: %d %#v", code, round)
	}
	path := "/v1/rounds/" + round["id"].(string)

	if code, bad := e.json(t, http.MethodPatch, path, e.token, map[string]any{"notes": "x"}); code != 400 {
		t.Fatalf("notes without version: %d %#v", code, bad)
	}
	code, round = e.json(t, http.MethodPatch, path, e.token, map[string]any{"set_question": map[string]any{"prompt": "Why Snowflake", "answer": "Scale."}})
	qs := round["questions"].([]any)
	if code != 200 || len(qs) != 1 || qs[0].(map[string]any)["prompt"] != "Why Snowflake?" {
		t.Fatalf("set_question creates: %d %#v", code, round)
	}
	code, round = e.json(t, http.MethodPatch, path, e.token, map[string]any{"set_question": map[string]any{"prompt": "why\nsnowflake?", "answer": "Query engine."}, "append": "Recruiter: Dana."})
	qs = round["questions"].([]any)
	if code != 200 || len(qs) != 1 || qs[0].(map[string]any)["answer"] != "Query engine." {
		t.Fatalf("set_question replaces, append stays out of the answer: %d %#v", code, round)
	}
	if code, stale := e.json(t, http.MethodPatch, path, e.token, map[string]any{"version": 1, "append": "late"}); code != 409 {
		t.Fatalf("stale version still checked: %d %#v", code, stale)
	}

	code, bank := e.json(t, http.MethodGet, "/v1/questions?q=query+engine", e.token, nil)
	rows := bank["data"].([]any)
	if code != 200 || len(rows) != 1 || rows[0].(map[string]any)["company"] != "Snowflake" || rows[0].(map[string]any)["round_id"] != round["id"] {
		t.Fatalf("bank: %d %#v", code, bank)
	}
	if code, bank = e.json(t, http.MethodGet, "/v1/questions?q=nothing", e.token, nil); code != 200 || len(bank["data"].([]any)) != 0 {
		t.Fatalf("bank filter: %d %#v", code, bank)
	}
}
