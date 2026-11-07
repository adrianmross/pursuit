package httpapi

import (
	"net/http"
	"strings"
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
	if code, bad := e.json(t, http.MethodPost, "/v1/applications/"+app["id"].(string)+"/rounds", e.token, map[string]any{"stage": "offer"}); code != 400 {
		t.Fatalf("an offer has no rounds: %d %#v", code, bad)
	}

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
	if round["in_notes"] != true {
		t.Fatalf("rounds start in notes: %#v", round)
	}
	if code, round = e.json(t, http.MethodPatch, path, e.token, map[string]any{"in_notes": false}); code != 200 || round["in_notes"] != false {
		t.Fatalf("in_notes off: %d %#v", code, round)
	}
	if code, stale := e.json(t, http.MethodPatch, path, e.token, map[string]any{"version": 1, "append": "late"}); code != 409 {
		t.Fatalf("stale version still checked: %d %#v", code, stale)
	}

	// Compiled notes leave out a round with in_notes false, then take it back.
	code, compiled := e.json(t, http.MethodGet, "/v1/applications/"+app["id"].(string)+"/notes", e.token, nil)
	if code != 200 || compiled["left_out"].(float64) != 1 || strings.Contains(compiled["markdown"].(string), "Recruiter call") {
		t.Fatalf("compiled without the round: %d %#v", code, compiled)
	}
	if code, _ = e.json(t, http.MethodPatch, path, e.token, map[string]any{"in_notes": true}); code != 200 {
		t.Fatal(code)
	}
	_, compiled = e.json(t, http.MethodGet, "/v1/applications/"+app["id"].(string)+"/notes", e.token, nil)
	if md := compiled["markdown"].(string); !strings.Contains(md, "## Recruiter call · Screening") || !strings.Contains(md, "### Prep") || !strings.Contains(md, "#### Why Snowflake?") {
		t.Fatalf("compiled with the round:\n%s", md)
	}

	// A task can belong to no stage.
	code, task := e.json(t, http.MethodPost, "/v1/applications/"+app["id"].(string)+"/tasks", e.token, map[string]any{"title": "Ask about team match", "stage": "anytime"})
	if code != 201 || task["stage"] != "anytime" {
		t.Fatalf("anytime task: %d %#v", code, task)
	}
	_, got := e.json(t, http.MethodGet, "/v1/applications/"+app["id"].(string), e.token, nil)
	if tasks := got["tasks"].(map[string]any); tasks["-1"] == nil || tasks["0"] != nil {
		t.Fatalf("anytime counts apart from Apply: %#v", tasks)
	}

	// A task on a round takes the round's stage, and loses the round when it goes.
	code, other := e.json(t, http.MethodPost, "/v1/applications/"+app["id"].(string)+"/rounds", e.token, map[string]any{"stage": "interviewing", "kind": "System design"})
	if code != 201 {
		t.Fatal(other)
	}
	code, rt := e.json(t, http.MethodPost, "/v1/applications/"+app["id"].(string)+"/tasks", e.token, map[string]any{"title": "Practice a design", "round_id": other["id"]})
	if code != 201 || rt["round_id"] != other["id"] || rt["stage"] != "interviewing" {
		t.Fatalf("round task: %d %#v", code, rt)
	}
	if code, _ = e.json(t, http.MethodDelete, "/v1/rounds/"+other["id"].(string)+"?version=1", e.token, nil); code != 200 {
		t.Fatal(code)
	}
	if _, rt = e.json(t, http.MethodGet, "/v1/tasks/"+rt["id"].(string), e.token, nil); rt["round_id"] != nil || rt["stage"] != "interviewing" {
		t.Fatalf("round gone, task stays on the stage: %#v", rt)
	}

	// An application can name its own stages and skip some; statuses stay.
	appPath := "/v1/applications/" + app["id"].(string)
	_, cur := e.json(t, http.MethodGet, appPath, e.token, nil)
	code, planned := e.json(t, http.MethodPatch, appPath, e.token, map[string]any{"version": cur["version"], "stage_plan": map[string]any{"labels": map[string]any{"Screening": "Phone screen"}, "off": []any{"final", "final"}}})
	plan, _ := planned["stage_plan"].(map[string]any)
	if code != 200 || plan == nil || plan["labels"].(map[string]any)["screening"] != "Phone screen" || len(plan["off"].([]any)) != 1 || planned["status"] != "screening" {
		t.Fatalf("stage plan: %d %#v", code, planned)
	}
	if code, bad := e.json(t, http.MethodPatch, appPath, e.token, map[string]any{"version": planned["version"], "stage_plan": map[string]any{"off": []any{"offer"}}}); code != 400 {
		t.Fatalf("offer is not a round stage: %d %#v", code, bad)
	}
	if code, planned = e.json(t, http.MethodPatch, appPath, e.token, map[string]any{"version": planned["version"], "stage_plan": map[string]any{}}); code != 200 || planned["stage_plan"] != nil {
		t.Fatalf("empty plan resets: %d %#v", code, planned)
	}

	// Notes files of their own, in the compiled notes.
	code, file := e.json(t, http.MethodPost, appPath+"/notes/files", e.token, map[string]any{"name": "Salary research", "body": "Levels say 250."})
	if code != 201 || file["name"] != "Salary research" {
		t.Fatalf("add file: %d %#v", code, file)
	}
	if code, file = e.json(t, http.MethodPatch, "/v1/notes/"+file["id"].(string), e.token, map[string]any{"set_question": map[string]any{"prompt": "What band", "answer": "L5"}}); code != 200 || len(file["questions"].([]any)) != 1 {
		t.Fatalf("file question: %d %#v", code, file)
	}
	if _, compiled = e.json(t, http.MethodGet, appPath+"/notes", e.token, nil); !strings.Contains(compiled["markdown"].(string), "## Salary research") {
		t.Fatalf("compiled has the file:\n%s", compiled["markdown"])
	}
	if code, _ = e.json(t, http.MethodDelete, "/v1/notes/"+file["id"].(string)+"?version=2", e.token, nil); code != 200 {
		t.Fatalf("delete file: %d", code)
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
