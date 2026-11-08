package httpapi

import (
	"context"
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/pursuit/pursuit/internal/embed"
	"github.com/pursuit/pursuit/internal/links"
)

// wordsEmbedder is a stand-in for a real model: a bag of words hashed into
// 64 dimensions, so texts sharing words score close. It counts what it embeds.
type wordsEmbedder struct {
	mu    sync.Mutex
	texts []string
}

func (f *wordsEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.mu.Lock()
	f.texts = append(f.texts, texts...)
	f.mu.Unlock()
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v := make([]float32, 64)
		for _, w := range strings.FieldsFunc(strings.ToLower(t), func(r rune) bool { return !(r >= 'a' && r <= 'z') }) {
			if len(w) < 3 {
				continue
			}
			h := fnv.New32a()
			_, _ = h.Write([]byte(w))
			v[h.Sum32()%64]++
		}
		out[i] = embed.Normalize(v)
	}
	return out, nil
}

func (f *wordsEmbedder) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.texts)
}

func similar(t *testing.T, e *env, q string) (int, []map[string]any) {
	t.Helper()
	code, out := e.json(t, http.MethodGet, "/v1/answers/similar?q="+url.QueryEscape(q), e.token, nil)
	var rows []map[string]any
	if data, ok := out["data"].([]any); ok {
		for _, d := range data {
			rows = append(rows, d.(map[string]any))
		}
	}
	return code, rows
}

func TestSmartSearch(t *testing.T) {
	e := newEnv(t)
	e.svc.PremiumOn = true
	fake := &wordsEmbedder{}
	e.svc.Embedder = fake
	for k, v := range map[string]string{
		"what_are_your_salary_expectations": "Around 180k base, flexible on equity.",
		"why_do_you_want_to_work_here":      "I like building developer tools.",
	} {
		if code, _ := e.json(t, http.MethodPost, "/v1/standing", e.token, map[string]any{"key": k, "answer": v}); code != 200 {
			t.Fatal(code)
		}
	}
	if code, _ := similar(t, e, "salary"); code != 403 {
		t.Fatalf("experimental and off: %d", code)
	}
	if code, _ := e.json(t, http.MethodPatch, "/v1/features/smart_search", e.token, map[string]any{"enabled": true}); code != 200 {
		t.Fatal(code)
	}
	code, rows := similar(t, e, "expected salary expectations")
	if code != 200 || len(rows) == 0 || rows[0]["source_id"] != "what_are_your_salary_expectations" || rows[0]["answer"] != "Around 180k base, flexible on equity." {
		t.Fatalf("ranking %d %#v", code, rows)
	}
	indexed := fake.count()

	// Searching again embeds only the query: nothing changed.
	similar(t, e, "salary")
	if got := fake.count() - indexed; got != 1 {
		t.Fatalf("unchanged sources re-embedded: %d texts", got)
	}
	// A changed answer is re-embedded alone; a deleted one drops out.
	before := fake.count()
	e.json(t, http.MethodPost, "/v1/standing", e.token, map[string]any{"key": "why_do_you_want_to_work_here", "answer": "Developer tools, and the team."})
	e.json(t, http.MethodDelete, "/v1/standing/what_are_your_salary_expectations", e.token, nil)
	_, rows = similar(t, e, "salary expectations")
	if got := fake.count() - before; got != 2 {
		t.Fatalf("want the changed answer plus the query embedded, got %d", got)
	}
	for _, r := range rows {
		if r["source_id"] == "what_are_your_salary_expectations" {
			t.Fatalf("deleted answer still found: %#v", rows)
		}
	}
}

// Prepare suggests the closest saved answer but leaves the question for a human.
func TestPrepareSuggestsAnswer(t *testing.T) {
	e := newEnv(t)
	e.svc.PremiumOn = true
	e.svc.Embedder = &wordsEmbedder{}
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"title":"Engineer","questions":[{"label":"What are your salary expectations for the role?","required":true}]}`))
	}))
	defer gh.Close()
	e.svc.Links = &links.Client{HTTP: &http.Client{Transport: rewriteTransport(gh.URL)}}
	e.json(t, http.MethodPatch, "/v1/features/smart_search", e.token, map[string]any{"enabled": true})
	e.json(t, http.MethodPost, "/v1/standing", e.token, map[string]any{"key": "what_are_your_salary_expectations", "answer": "Around 180k."})
	_, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "Example", "title": "Engineer", "status": "to_apply", "posting_url": "https://boards.greenhouse.io/example/jobs/7",
	})
	id := app["id"].(string)
	code, prepared := e.json(t, http.MethodPost, "/v1/applications/"+id+"/prepare", e.token, nil)
	if code != 200 || prepared["unknowns"].(float64) != 1 {
		t.Fatalf("prepare %d %#v", code, prepared)
	}
	q := prepared["questions"].([]any)[0].(map[string]any)
	if q["unknown"] != true || q["answer"] != "" || q["suggested_answer"] != "Around 180k." || q["suggested_source"] != "what_are_your_salary_expectations" {
		t.Fatalf("question %#v", q)
	}
	_, listed := e.json(t, http.MethodGet, "/v1/applications/"+id+"/questions", e.token, nil)
	if got := listed["data"].([]any)[0].(map[string]any)["suggested_answer"]; got != "Around 180k." {
		t.Fatalf("suggestion not stored: %#v", listed)
	}
}

func TestSmartSearchNotForDemo(t *testing.T) {
	e := newEnv(t)
	e.svc.PremiumOn = true
	e.svc.Embedder = &wordsEmbedder{}
	secret, _, err := e.svc.CreateDemoSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, out := e.json(t, http.MethodGet, "/v1/features", secret, nil)
	for _, f := range out["data"].([]any) {
		if m := f.(map[string]any); m["key"] == "smart_search" && m["configured"] != false {
			t.Fatalf("demo gets a paid API: %#v", m)
		}
	}
}
