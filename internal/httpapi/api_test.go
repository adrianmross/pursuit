package httpapi

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pursuit/pursuit/internal/betterauth"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/links"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

type env struct {
	srv   *httptest.Server
	svc   *service.Service
	api   *Server
	token string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db, err := sqlstore.Open(context.Background(), "sqlite://"+filepath.Join(t.TempDir(), "pursuit.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	svc := service.New(db)
	secret := "pst_test_owner_token_value"
	if _, _, err := svc.Bootstrap(context.Background(), "Adrian", "adrian@localhost", "owner", secret); err != nil {
		t.Fatal(err)
	}
	api := &Server{Svc: svc, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	api.Ready.Store(true)
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	return &env{srv: srv, svc: svc, api: api, token: secret}
}

func (e *env) do(t *testing.T, method, path, token string, body []byte, headers map[string]string) (int, []byte, http.Header) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, b, resp.Header
}

func (e *env) json(t *testing.T, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	headers := map[string]string{}
	if body != nil {
		headers["Content-Type"] = "application/json"
	}
	code, b, _ := e.do(t, method, path, token, raw, headers)
	var doc map[string]any
	if len(bytes.TrimSpace(b)) > 0 && (b[0] == '{' || b[0] == '[') {
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatalf("status %d body %s: %v", code, b, err)
		}
	}
	return code, doc
}

func errCode(doc map[string]any) string {
	err, _ := doc["error"].(map[string]any)
	code, _ := err["code"].(string)
	return code
}

func TestGoldenImportAndSimplifyExport(t *testing.T) {
	e := newEnv(t)
	raw, err := os.ReadFile("../../testdata/applications.csv")
	if err != nil {
		t.Fatal(err)
	}
	code, body, _ := e.do(t, http.MethodPost, "/v1/import/csv", e.token, raw, map[string]string{"Content-Type": "text/csv"})
	if code != 200 {
		t.Fatalf("import %d %s", code, body)
	}
	var imported map[string]any
	if err := json.Unmarshal(body, &imported); err != nil {
		t.Fatal(err)
	}
	if imported["applications_created"] != float64(2) {
		t.Fatalf("created %#v", imported["applications_created"])
	}
	code, csvBody, _ := e.do(t, http.MethodGet, "/v1/export/simplify", e.token, nil, nil)
	if code != 200 {
		t.Fatalf("export %d %s", code, csvBody)
	}
	got, err := csv.NewReader(bytes.NewReader(csvBody)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	wantFile, err := os.Open("../../testdata/simplify_import.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer wantFile.Close()
	want, err := csv.NewReader(wantFile).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("records %d vs %d", len(got), len(want))
	}
	for i := range want {
		if strings.Join(got[i], "\x1f") != strings.Join(want[i], "\x1f") {
			t.Fatalf("row %d\n got %#v\nwant %#v", i, got[i], want[i])
		}
	}
	code, body, _ = e.do(t, http.MethodPost, "/v1/import/csv", e.token, raw, map[string]string{"Content-Type": "text/csv"})
	if code != 200 {
		t.Fatalf("reimport %d %s", code, body)
	}
	if err := json.Unmarshal(body, &imported); err != nil {
		t.Fatal(err)
	}
	if imported["applications_created"] != float64(0) || imported["applications_updated"] != float64(2) {
		t.Fatalf("reimport %#v", imported)
	}
}

func TestSeedImportDedupesByURL(t *testing.T) {
	e := newEnv(t)
	code, doc := e.json(t, http.MethodPost, "/v1/import/seed", e.token, map[string]any{})
	if code != 200 {
		t.Fatalf("seed %d %#v", code, doc)
	}
	if doc["applications_created"].(float64) != 7 {
		t.Fatalf("expected example csv plus new markdown rows, got %#v", doc)
	}
	if doc["skipped"].(float64) != 1 {
		t.Fatalf("expected one URL dedupe skip, got %#v", doc)
	}
	code, exported, _ := e.do(t, http.MethodGet, "/v1/export/json", e.token, nil, nil)
	if code != 200 {
		t.Fatal(exported)
	}
	var payload struct {
		Applications []domain.Application `json:"applications"`
	}
	if err := json.Unmarshal(exported, &payload); err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	var applied, prepared, toApply int
	for _, app := range payload.Applications {
		if app.PostingURL != "" {
			seen[app.PostingURL]++
		}
		switch app.Status {
		case domain.StatusApplied:
			applied++
		case domain.StatusAwaitingReview:
			prepared++
		case domain.StatusToApply:
			toApply++
		}
	}
	for url, n := range seen {
		if n != 1 {
			t.Fatalf("url %s appears %d times", url, n)
		}
	}
	if applied != 2 || prepared != 1 || toApply != 4 {
		t.Fatalf("applied=%d awaiting_review=%d to_apply=%d", applied, prepared, toApply)
	}
	code, caps := e.json(t, http.MethodGet, "/v1/caps", e.token, nil)
	if code != 200 {
		t.Fatal(caps)
	}
	blob, _ := json.Marshal(caps)
	for _, name := range []string{"Coinbase", "OpenAI", "Block"} {
		if !strings.Contains(string(blob), name) {
			t.Fatalf("missing cap %s in %s", name, blob)
		}
	}
}

func TestAuthScopesDryRunIdempotencyAndCursors(t *testing.T) {
	e := newEnv(t)
	code, _ := e.json(t, http.MethodGet, "/v1/companies", "", nil)
	if code != 401 {
		t.Fatalf("anon %d", code)
	}
	code, issued := e.json(t, http.MethodPost, "/v1/tokens", e.token, map[string]any{"name": "reader", "scopes": []string{"read"}})
	if code != 201 {
		t.Fatal(issued)
	}
	readTok := issued["secret"].(string)
	code, denied := e.json(t, http.MethodPost, "/v1/companies", readTok, map[string]any{"name": "Nope"})
	if code != 403 || errCode(denied) != "forbidden" {
		t.Fatalf("read write %d %#v", code, denied)
	}

	reqBody := []byte(`{"name":"Dry Co"}`)
	code, body, _ := e.do(t, http.MethodPost, "/v1/companies", e.token, reqBody, map[string]string{
		"Content-Type": "application/json", "X-Dry-Run": "true",
	})
	if code != 200 || !bytes.Contains(body, []byte(`"dry_run":true`)) {
		t.Fatalf("dry %d %s", code, body)
	}
	code, listed := e.json(t, http.MethodGet, "/v1/companies", e.token, nil)
	if code != 200 || listed["page"].(map[string]any)["total"].(float64) != 0 {
		t.Fatalf("dry persisted %#v", listed)
	}

	code, replayBody, hdr := e.do(t, http.MethodPost, "/v1/companies", e.token, []byte(`{"name":"Idem Co"}`), map[string]string{
		"Content-Type": "application/json", "Idempotency-Key": "k1",
	})
	if code != 201 {
		t.Fatalf("first idem %d %s", code, replayBody)
	}
	code, again, hdr2 := e.do(t, http.MethodPost, "/v1/companies", e.token, []byte(`{"name":"Idem Co"}`), map[string]string{
		"Content-Type": "application/json", "Idempotency-Key": "k1",
	})
	if code != 201 || hdr2.Get("Idempotent-Replayed") == "" {
		t.Fatalf("replay %d header %q %s", code, hdr2.Get("Idempotent-Replayed"), again)
	}
	if !bytes.Equal(bytes.TrimSpace(replayBody), bytes.TrimSpace(again)) {
		t.Fatalf("replay body mismatch\n%s\n%s", replayBody, again)
	}
	_ = hdr
	code, mismatch, _ := e.do(t, http.MethodPost, "/v1/companies", e.token, []byte(`{"name":"Other Co"}`), map[string]string{
		"Content-Type": "application/json", "Idempotency-Key": "k1",
	})
	if code != 409 {
		t.Fatalf("mismatch %d %s", code, mismatch)
	}

	var wg sync.WaitGroup
	codes := make([]int, 2)
	bodies := make([][]byte, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], bodies[i], _ = e.do(t, http.MethodPost, "/v1/companies", e.token, []byte(`{"name":"Race Co"}`), map[string]string{
				"Content-Type": "application/json", "Idempotency-Key": "race",
			})
		}(i)
	}
	wg.Wait()
	if codes[0] != 201 || codes[1] != 201 || !bytes.Equal(bytes.TrimSpace(bodies[0]), bytes.TrimSpace(bodies[1])) {
		t.Fatalf("parallel idem %d %d\n%s\n%s", codes[0], codes[1], bodies[0], bodies[1])
	}

	for _, name := range []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo"} {
		code, doc := e.json(t, http.MethodPost, "/v1/companies", e.token, map[string]any{"name": name})
		if code != 201 {
			t.Fatal(doc)
		}
	}
	seen := map[string]int{}
	cursor := ""
	pages := 0
	for {
		pages++
		path := "/v1/companies?limit=2&sort=name"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		code, page := e.json(t, http.MethodGet, path, e.token, nil)
		if code != 200 {
			t.Fatal(page)
		}
		for _, item := range page["data"].([]any) {
			id := item.(map[string]any)["id"].(string)
			seen[id]++
		}
		meta := page["page"].(map[string]any)
		if meta["has_more"] != true {
			break
		}
		cursor = meta["next_cursor"].(string)
		if cursor == "" || pages > 10 {
			t.Fatal("cursor did not advance")
		}
	}
	if len(seen) < 5 {
		t.Fatalf("cursor walk saw %d", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("id %s seen %d", id, n)
		}
	}
	code, bad := e.json(t, http.MethodGet, "/v1/applications?fields=not_a_field", e.token, nil)
	if code != 400 || errCode(bad) != "validation" {
		t.Fatalf("fields %d %#v", code, bad)
	}
}

func TestTwoReplicasDoNotClobber(t *testing.T) {
	dir := t.TempDir()
	dsn := "sqlite://" + filepath.Join(dir, "shared.db")
	open := func() *env {
		t.Helper()
		db, err := sqlstore.Open(context.Background(), dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		svc := service.New(db)
		secret := "pst_shared_owner"
		if _, _, err := svc.Bootstrap(context.Background(), "Adrian", "adrian@localhost", "owner", secret); err != nil {
			t.Fatal(err)
		}
		api := &Server{Svc: svc, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
		api.Ready.Store(true)
		srv := httptest.NewServer(api.Handler())
		t.Cleanup(srv.Close)
		return &env{srv: srv, svc: svc, token: secret}
	}
	a := open()
	b := open()
	code, created := a.json(t, http.MethodPost, "/v1/companies", a.token, map[string]any{"name": "Shared"})
	if code != 201 {
		t.Fatal(created)
	}
	id := created["id"].(string)
	body := []byte(`{"version":1,"notes":"from-a"}`)
	var wg sync.WaitGroup
	type result struct {
		code int
		body []byte
	}
	out := make([]result, 2)
	servers := []*env{a, b}
	notes := []string{`{"version":1,"notes":"from-a"}`, `{"version":1,"notes":"from-b"}`}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			out[i].code, out[i].body, _ = servers[i].do(t, http.MethodPatch, "/v1/companies/"+id, a.token, []byte(notes[i]), map[string]string{"Content-Type": "application/json"})
		}(i)
	}
	wg.Wait()
	ok, conflict := 0, 0
	for _, r := range out {
		if r.code == 200 {
			ok++
		} else if r.code == 409 {
			conflict++
		} else {
			t.Fatalf("unexpected %d %s", r.code, r.body)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("want one 200 and one 409, got %+v", out)
	}
	code, got := a.json(t, http.MethodGet, "/v1/companies/"+id, a.token, nil)
	if code != 200 || got["version"].(float64) != 2 {
		t.Fatalf("final %#v", got)
	}
	_ = body
}

func TestQueueLeaseApproveAndCaps(t *testing.T) {
	e := newEnv(t)
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":111,"title":"Engineer","questions":[{"label":"Why us?","required":true},{"label":"Email","required":true}]}`))
	}))
	defer gh.Close()
	e.svc.Links = &links.Client{HTTP: &http.Client{Transport: rewriteTransport(gh.URL)}}

	code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "Example", "title": "Platform Engineer", "status": "to_apply",
		"posting_url": "https://boards.greenhouse.io/example/jobs/111",
	})
	if code != 201 {
		t.Fatal(app)
	}
	id := app["id"].(string)
	code, prepared := e.json(t, http.MethodPost, "/v1/applications/"+id+"/prepare", e.token, nil)
	if code != 200 {
		t.Fatalf("prepare %d %#v", code, prepared)
	}
	application := prepared["application"].(map[string]any)
	if application["outcome"] != domain.StatusNeedsInput || prepared["unknowns"].(float64) < 1 {
		t.Fatalf("expected needs_input, got %#v", prepared)
	}
	code, _ = e.json(t, http.MethodPost, "/v1/standing", e.token, map[string]any{"key": "why_us", "answer": "I build platforms."})
	if code != 200 {
		t.Fatal(code)
	}
	code, _ = e.json(t, http.MethodPost, "/v1/standing", e.token, map[string]any{"key": "email", "answer": "person@example.com"})
	if code != 200 {
		t.Fatal(code)
	}
	code, doc := e.json(t, http.MethodPost, "/v1/documents", e.token, map[string]any{"kind": "base_resume", "filename": "resume.txt", "content_type": "text/plain", "body": "Base resume"})
	if code != 201 {
		t.Fatalf("document %d %#v", code, doc)
	}
	code, prepared = e.json(t, http.MethodPost, "/v1/applications/"+id+"/prepare", e.token, nil)
	if code != 200 || prepared["application"].(map[string]any)["status"] != domain.StatusAwaitingReview {
		t.Fatalf("second prepare %#v", prepared)
	}
	code, arts := e.json(t, http.MethodGet, "/v1/applications/"+id+"/artifacts", e.token, nil)
	if code != 200 {
		t.Fatal(arts)
	}
	kinds := map[string]bool{}
	for _, item := range arts["data"].([]any) {
		kinds[item.(map[string]any)["kind"].(string)] = true
	}
	if !kinds["form_snapshot"] || !kinds["answers_json"] || !kinds["resume_pdf"] {
		t.Fatalf("artifacts %#v", arts)
	}

	code, reader := e.json(t, http.MethodPost, "/v1/tokens", e.token, map[string]any{"name": "agent", "scopes": []string{"write"}})
	if code != 201 {
		t.Fatal(reader)
	}
	writeTok := reader["secret"].(string)
	code, denied := e.json(t, http.MethodPost, "/v1/approvals", writeTok, map[string]any{"ids": []string{id}})
	if code != 403 {
		t.Fatalf("agent approve %d %#v", code, denied)
	}
	ver := int(prepared["application"].(map[string]any)["version"].(float64))
	code, jump := e.json(t, http.MethodPost, "/v1/applications/"+id+"/transition", writeTok, map[string]any{
		"version": ver, "status": "submitting",
	})
	if code != 409 || !strings.Contains(jump["error"].(map[string]any)["message"].(string), "approval") {
		t.Fatalf("self-submit %d %#v", code, jump)
	}
	code, raw, _ := e.do(t, http.MethodPost, "/v1/approvals", e.token, []byte(`{"ids":["`+id+`"],"note":"send it"}`), map[string]string{"Content-Type": "application/json"})
	if code != 200 || !bytes.Contains(raw, []byte(`"status":"approved"`)) {
		t.Fatalf("approve %d %s", code, raw)
	}
	code, raw, _ = e.do(t, http.MethodPost, "/v1/approvals", e.token, []byte(`{"ids":["`+id+`"],"note":"again"}`), map[string]string{"Content-Type": "application/json"})
	if code == 200 {
		t.Fatalf("double approve %s", raw)
	}

	code, claim := e.json(t, http.MethodPost, "/v1/queue/claim", writeTok, map[string]any{"worker": "agent-1"})
	if code != 200 || claim["work"] == nil {
		t.Fatalf("claim %#v", claim)
	}
	work := claim["work"].(map[string]any)
	if work["kind"] != "submit" || work["application_id"] != id {
		t.Fatalf("work %#v", work)
	}
	code, done := e.json(t, http.MethodPost, "/v1/queue/"+work["id"].(string)+"/complete", writeTok, map[string]any{
		"worker": "agent-1", "status": "submitted", "note": "confirmation received",
	})
	if code != 200 || done["status"] != domain.StatusSubmitted {
		t.Fatalf("complete %#v", done)
	}

	// Concurrent claims lease each job once.
	var ids []string
	for i := 0; i < 4; i++ {
		code, created := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
			"company": "QueueCo", "title": "Role", "status": "to_apply",
			"posting_url": "https://example.com/jobs/" + string(rune('a'+i)),
		})
		if code != 201 {
			t.Fatal(created)
		}
		appID := created["id"].(string)
		ids = append(ids, appID)
		code, queued := e.json(t, http.MethodPost, "/v1/applications/"+appID+"/enqueue", e.token, map[string]any{"kind": "prepare"})
		if code != 200 {
			t.Fatalf("enqueue %#v", queued)
		}
	}
	var mu sync.Mutex
	leased := map[string]bool{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			code, doc := e.json(t, http.MethodPost, "/v1/queue/claim", e.token, map[string]any{"worker": "w"})
			if code != 200 {
				t.Errorf("claim %d %#v", code, doc)
				return
			}
			if doc["work"] == nil {
				return
			}
			mu.Lock()
			leased[doc["work"].(map[string]any)["id"].(string)] = true
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if len(leased) != 4 {
		t.Fatalf("leased %d of 4", len(leased))
	}

	code, capped := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "Coinbase", "title": "One", "status": "to_apply",
	})
	if code != 201 {
		t.Fatal(capped)
	}
	for _, title := range []string{"Two", "Three"} {
		code, doc := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
			"company": "Coinbase", "title": title, "status": "to_apply",
		})
		if code != 201 {
			t.Fatalf("%s %#v", title, doc)
		}
	}
	code, blocked := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "Coinbase", "title": "Four", "status": "to_apply",
	})
	if code != 409 || errCode(blocked) != "conflict" {
		t.Fatalf("cap %d %#v", code, blocked)
	}
	_ = ids
}

func TestLinkCheckStaleAndWebhook(t *testing.T) {
	e := newEnv(t)
	var hits int
	var secret string
	hookHit := make(chan struct{}, 1)
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "pursuit-webhook" {
			secret = r.Header.Get("X-Pursuit-Secret")
			select {
			case hookHit <- struct{}{}:
			default:
			}
			w.WriteHeader(204)
			return
		}
		hits++
		_, _ = w.Write([]byte("<html>this job is no longer accepting applications</html>"))
	}))
	defer site.Close()
	e.svc.Links = &links.Client{HTTP: site.Client(), Gap: 0}
	e.svc.Hooks = func(userID string, ev domain.Event) {
		e.svc.DeliverWebhooks(context.Background(), userID, ev)
	}
	code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "ClosedCo", "title": "Old role", "status": "to_apply",
		"posting_url": site.URL + "/jobs/old",
	})
	if code != 201 {
		t.Fatal(app)
	}
	id := app["id"].(string)
	code, hook := e.json(t, http.MethodPost, "/v1/webhooks", e.token, map[string]any{"url": site.URL + "/hook", "secret": "s3"})
	if code != 201 {
		t.Fatal(hook)
	}
	code, checked := e.json(t, http.MethodPost, "/v1/applications/"+id+"/check-link", e.token, nil)
	if code != 200 || checked["status"] != "closed" || checked["changed"] != true {
		t.Fatalf("check %#v", checked)
	}
	code, stale := e.json(t, http.MethodGet, "/v1/applications?stale=true", e.token, nil)
	if code != 200 || stale["page"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("stale %#v", stale)
	}
	item := stale["data"].([]any)[0].(map[string]any)
	if item["link_status"] != "closed" || !strings.Contains(item["next_step"].(string), "Posting closed") {
		t.Fatalf("marked %#v", item)
	}
	if item["status"] != domain.StatusToApply {
		t.Fatalf("pipeline status changed %#v", item["status"])
	}
	select {
	case <-hookHit:
	case <-time.After(2 * time.Second):
		t.Fatal("webhook was not delivered")
	}
	if secret != "s3" {
		t.Fatalf("secret %q", secret)
	}

	blocked := make(chan struct{})
	entered := make(chan struct{})
	var once sync.Once
	e.svc.Links = &links.Client{HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		once.Do(func() { close(entered) })
		<-blocked
		return &http.Response{
			StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")),
			Header: make(http.Header), Request: r,
		}, nil
	})}, Gap: 0}
	code, second := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "LiveCo", "title": "Still open", "status": "to_apply",
		"posting_url": "https://example.com/jobs/live",
	})
	if code != 201 {
		t.Fatal(second)
	}
	done := make(chan int, 1)
	go func() {
		code, _, _ := e.do(t, http.MethodPost, "/v1/links/check", e.token, []byte(`{"owner":"replica-a"}`), map[string]string{"Content-Type": "application/json"})
		done <- code
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("link sweep did not start a check")
	}
	code, busy := e.json(t, http.MethodPost, "/v1/links/check", e.token, map[string]any{"owner": "replica-b"})
	close(blocked)
	if code != 409 {
		t.Fatalf("second sweep %d %#v", code, busy)
	}
	if got := <-done; got != 200 {
		t.Fatalf("first sweep %d", got)
	}
	_ = hits
}

func TestRejectDoesNotApproveOrSubmit(t *testing.T) {
	e := newEnv(t)
	code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "ReviewCo", "title": "Prepared role", "status": "to_apply", "notes": "draft answers",
	})
	if code != 201 {
		t.Fatal(app)
	}
	id := app["id"].(string)
	ver := int(app["version"].(float64))
	code, moved := e.json(t, http.MethodPost, "/v1/applications/"+id+"/transition", e.token, map[string]any{
		"version": ver, "status": "awaiting_review",
	})
	if code != 200 || moved["status"] != domain.StatusAwaitingReview {
		t.Fatalf("queue %d %#v", code, moved)
	}

	code, other := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "ReviewCo", "title": "Still drafting", "status": "to_apply",
	})
	if code != 201 {
		t.Fatal(other)
	}
	code, early := e.json(t, http.MethodPost, "/v1/approvals/reject", e.token, map[string]any{"ids": []string{other["id"].(string)}})
	if code != 400 || errCode(early) != "validation" {
		t.Fatalf("early reject %d %#v", code, early)
	}

	for _, scopes := range [][]string{{"read"}, {"write"}} {
		code, issued := e.json(t, http.MethodPost, "/v1/tokens", e.token, map[string]any{"name": scopes[0], "scopes": scopes})
		if code != 201 {
			t.Fatal(issued)
		}
		code, denied := e.json(t, http.MethodPost, "/v1/approvals/reject", issued["secret"].(string), map[string]any{"ids": []string{id}})
		if code != 403 || errCode(denied) != "forbidden" {
			t.Fatalf("scope %v reject %d %#v", scopes, code, denied)
		}
	}

	code, raw, _ := e.do(t, http.MethodPost, "/v1/approvals/reject", e.token, []byte(`{"ids":["`+id+`"],"note":"hold off"}`), map[string]string{"Content-Type": "application/json"})
	if code != 200 || !bytes.Contains(raw, []byte(`"outcome":"withdrawn"`)) || bytes.Contains(raw, []byte(`"status":"approved"`)) || bytes.Contains(raw, []byte(`"status":"submitting"`)) || bytes.Contains(raw, []byte(`"status":"submitted"`)) {
		t.Fatalf("reject %d %s", code, raw)
	}
	code, got := e.json(t, http.MethodGet, "/v1/applications/"+id, e.token, nil)
	if code != 200 || got["outcome"] != domain.StatusWithdrawn || got["status"] != domain.StatusAwaitingReview {
		t.Fatalf("stored %#v", got)
	}
	code, events := e.json(t, http.MethodGet, "/v1/applications/"+id+"/events", e.token, nil)
	if code != 200 {
		t.Fatal(events)
	}
	found := false
	for _, item := range events["data"].([]any) {
		ev := item.(map[string]any)
		if ev["kind"] == domain.EventApproval && ev["title"] == "Approval rejected" && ev["to_status"] == domain.StatusWithdrawn && ev["body"] == "hold off" {
			found = true
		}
	}
	if !found {
		t.Fatalf("events %#v", events)
	}
	ver = int(got["version"].(float64))
	code, jump := e.json(t, http.MethodPost, "/v1/applications/"+id+"/transition", e.token, map[string]any{
		"version": ver, "status": "submitting",
	})
	if code != 409 || errCode(jump) != "conflict" {
		t.Fatalf("submit after reject %d %#v", code, jump)
	}
	code, claim := e.json(t, http.MethodPost, "/v1/queue/claim", e.token, map[string]any{"worker": "agent-1"})
	if code != 200 || claim["work"] != nil {
		t.Fatalf("reject enqueued work %#v", claim)
	}
	code, again := e.json(t, http.MethodPost, "/v1/approvals/reject", e.token, map[string]any{"ids": []string{id}, "note": "again"})
	if code != 400 {
		t.Fatalf("second reject %d %#v", code, again)
	}
}

func TestLinkHealthList(t *testing.T) {
	e := newEnv(t)
	page := "<html>open role</html>"
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(page))
	}))
	defer site.Close()
	e.svc.Links = &links.Client{HTTP: site.Client(), Gap: 0}

	code, _ := e.json(t, http.MethodGet, "/v1/links?attention=true", "", nil)
	if code != 401 {
		t.Fatalf("anon %d", code)
	}
	code, issued := e.json(t, http.MethodPost, "/v1/tokens", e.token, map[string]any{"name": "reader", "scopes": []string{"read"}})
	if code != 201 {
		t.Fatal(issued)
	}
	readTok := issued["secret"].(string)

	page = "<html>this job is no longer accepting applications</html>"
	deadID := createChecked(t, e, site.URL+"/jobs/dead", "DeadCo", "Old role")
	page = "<html>open role</html>"
	liveID := createChecked(t, e, site.URL+"/jobs/live", "LiveCo", "Open role")

	code, attention := e.json(t, http.MethodGet, "/v1/links?attention=true", readTok, nil)
	if code != 200 || attention["page"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("attention %#v", attention)
	}
	row := attention["data"].([]any)[0].(map[string]any)
	if row["application_id"] != deadID || row["status"] != "closed" || row["changed"] != false || !strings.Contains(row["detail"].(string), "no longer accepting") {
		t.Fatalf("dead row %#v", row)
	}
	code, live := e.json(t, http.MethodGet, "/v1/links?status=live", e.token, nil)
	if code != 200 || live["page"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("live %#v", live)
	}
	if live["data"].([]any)[0].(map[string]any)["application_id"] != liveID {
		t.Fatalf("live row %#v", live)
	}

	page = "<html>this job is no longer accepting applications</html>"
	code, checked := e.json(t, http.MethodPost, "/v1/applications/"+liveID+"/check-link", e.token, nil)
	if code != 200 || checked["status"] != "closed" || checked["changed"] != true {
		t.Fatalf("recheck %#v", checked)
	}
	code, attention = e.json(t, http.MethodGet, "/v1/links?attention=true&sort=company", e.token, nil)
	if code != 200 || attention["page"].(map[string]any)["total"].(float64) != 2 {
		t.Fatalf("both %#v", attention)
	}
	var sawChanged bool
	for _, item := range attention["data"].([]any) {
		row := item.(map[string]any)
		if row["application_id"] == liveID {
			if row["changed"] != true || row["status"] != "closed" {
				t.Fatalf("changed row %#v", row)
			}
			sawChanged = true
		}
	}
	if !sawChanged {
		t.Fatalf("missing changed row %#v", attention)
	}
	code, bad := e.json(t, http.MethodGet, "/v1/links?attention=true&status=live", e.token, nil)
	if code != 400 || errCode(bad) != "validation" {
		t.Fatalf("combined %d %#v", code, bad)
	}
	code, bad = e.json(t, http.MethodGet, "/v1/links?status=nope", e.token, nil)
	if code != 400 {
		t.Fatalf("bad status %d %#v", code, bad)
	}
}

func TestTrustProxyAuthAndCron(t *testing.T) {
	e := newEnv(t)
	code, _ := e.json(t, http.MethodGet, "/v1/me", "", nil)
	if code != 401 {
		t.Fatalf("default anon %d", code)
	}
	e.api.TrustProxyAuth = "on"
	code, _ = e.json(t, http.MethodGet, "/v1/me", "", nil)
	if code != 401 {
		t.Fatalf("non-vercel proxy %d", code)
	}
	e.api.TrustProxyAuth = "vercel"
	code, me := e.json(t, http.MethodGet, "/v1/me", "", nil)
	if code != 200 {
		t.Fatalf("proxy me %d %#v", code, me)
	}
	tok, _ := me["token"].(map[string]any)
	if tok["name"] != "Adrian" {
		t.Fatalf("proxy token %#v", tok)
	}
	scopes := map[string]bool{}
	for _, s := range tok["scopes"].([]any) {
		scopes[s.(string)] = true
	}
	if !scopes["read"] || !scopes["write"] || !scopes["approve"] {
		t.Fatalf("proxy scopes %#v", tok["scopes"])
	}
	code, issued := e.json(t, http.MethodPost, "/v1/tokens", e.token, map[string]any{"name": "reader", "scopes": []string{"read"}})
	if code != 201 {
		t.Fatal(issued)
	}
	readTok := issued["secret"].(string)
	code, asReader := e.json(t, http.MethodGet, "/v1/me", readTok, nil)
	if code != 200 {
		t.Fatal(asReader)
	}
	readerScopes, _ := asReader["token"].(map[string]any)["scopes"].([]any)
	if len(readerScopes) != 1 || readerScopes[0] != "read" {
		t.Fatalf("token should win over proxy %#v", asReader["token"])
	}
	code, denied := e.json(t, http.MethodPost, "/v1/companies", readTok, map[string]any{"name": "Nope"})
	if code != 403 {
		t.Fatalf("read token write %d %#v", code, denied)
	}

	if err := e.api.Svc.DB.CreateUser(context.Background(), domain.User{ID: "u2", Name: "Second", Email: "second@localhost", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	code, _ = e.json(t, http.MethodGet, "/v1/me", "", nil)
	if code != 401 {
		t.Fatalf("proxy auth with two users %d", code)
	}

	code, _ = e.json(t, http.MethodGet, "/v1/cron/links", "", nil)
	if code != 401 {
		t.Fatalf("cron without secret %d", code)
	}
	code, _ = e.json(t, http.MethodGet, "/v1/cron/links", e.token, nil)
	if code != 401 {
		t.Fatalf("pursuit token must not authorize cron %d", code)
	}
	e.api.CronSecret = "cron-secret-value"
	code, _ = e.json(t, http.MethodGet, "/v1/cron/links", "nope", nil)
	if code != 401 {
		t.Fatalf("wrong cron secret %d", code)
	}
	code, report := e.json(t, http.MethodGet, "/v1/cron/links", "cron-secret-value", nil)
	if code != 200 || report["checked"] != float64(0) {
		t.Fatalf("cron %d %#v", code, report)
	}
}

func TestLoginIdentityTenantGating(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	if secret, err := e.svc.LoginIdentity(ctx, "adrian@localhost", "Adrian", false); err != nil || secret == "" {
		t.Fatalf("single-tenant owner login: %v %q", err, secret)
	}
	if _, err := e.svc.LoginIdentity(ctx, "stranger@example.com", "Stranger", false); err == nil {
		t.Fatal("single-tenant mode should refuse an unknown email")
	}
	secret, err := e.svc.LoginIdentity(ctx, "second@example.com", "Second", true)
	if err != nil || secret == "" {
		t.Fatalf("multi-tenant first login should provision a user: %v %q", err, secret)
	}
	u, err := e.svc.DB.FindUserByEmail(ctx, "second@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if secret2, err := e.svc.LoginIdentity(ctx, "second@example.com", "Second", true); err != nil || secret2 == "" {
		t.Fatalf("second login for the same user: %v %q", err, secret2)
	}
	if u2, err := e.svc.DB.FindUserByEmail(ctx, "second@example.com"); err != nil || u2.ID != u.ID {
		t.Fatalf("second login should reuse the same tenant, got %+v vs %+v (err %v)", u2, u, err)
	}
}

func TestResolveTenantUser(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	if _, err := e.svc.ResolveTenantUser(ctx, "third@example.com", "Third", false); err == nil {
		t.Fatal("allowCreate=false should refuse an unknown email")
	}
	actor, err := e.svc.ResolveTenantUser(ctx, "third@example.com", "Third", true)
	if err != nil || actor.UserID == "" {
		t.Fatalf("first resolve should provision a user: %v %+v", err, actor)
	}
	if !actor.Can("approve") {
		t.Fatalf("resolved actor should have full scopes, got %v", actor.Scopes)
	}
	actor2, err := e.svc.ResolveTenantUser(ctx, "third@example.com", "Third", true)
	if err != nil || actor2.UserID != actor.UserID {
		t.Fatalf("second resolve should reuse the same tenant: %v %+v vs %+v", err, actor2, actor)
	}
}

func TestAuthenticateSkipsBetterAuthOutsideMultiTenant(t *testing.T) {
	e := newEnv(t)
	e.api.BetterAuth = betterauth.New(context.Background(), "https://example.invalid", "", "")
	// TenantMode defaults to "" (single), so a JWT-shaped bearer token
	// must still fall through to Pursuit's own check and fail cleanly,
	// never reaching the network for JWKS verification.
	code, _ := e.json(t, http.MethodGet, "/v1/me", "a.b.c", nil)
	if code != 401 {
		t.Fatalf("expected 401 for a JWT-shaped token in single-tenant mode, got %d", code)
	}
}

func TestImportSamePostingURLUpdates(t *testing.T) {
	e := newEnv(t)
	csv := strings.Join([]string{
		"id,company,role,location,salary,fit,status,applied_date,next_step,posting_url,notes,last_updated",
		"ext-1,Acme,Engineer,Remote,$100k,Strong,Applied,2026-01-02,Follow up,https://example.com/jobs/1,first note,2026-01-03",
		"ext-2,Acme,Engineer,Remote,$120k,Strong,Prepared,2026-01-02,Send portfolio,https://example.com/jobs/1,updated note,2026-01-04",
		"ext-3,Beta,Designer,NYC,,Maybe,INTERVIEWING,2026-02-01,Onsite,https://example.com/jobs/2,loop notes,2026-02-01",
	}, "\n")
	code, body, _ := e.do(t, http.MethodPost, "/v1/import/csv", e.token, []byte(csv), map[string]string{"Content-Type": "text/csv"})
	if code != 200 {
		t.Fatalf("import %d %s", code, body)
	}
	var imported map[string]any
	if err := json.Unmarshal(body, &imported); err != nil {
		t.Fatal(err)
	}
	if imported["applications_created"] != float64(2) || imported["applications_updated"] != float64(1) {
		t.Fatalf("counts %#v", imported)
	}
	code, raw, _ := e.do(t, http.MethodGet, "/v1/export/json", e.token, nil, nil)
	if code != 200 {
		t.Fatalf("export %d %s", code, raw)
	}
	var exported struct {
		Applications []domain.Application `json:"applications"`
	}
	if err := json.Unmarshal(raw, &exported); err != nil {
		t.Fatal(err)
	}
	if len(exported.Applications) != 2 {
		t.Fatalf("apps %d", len(exported.Applications))
	}
	byURL := map[string]domain.Application{}
	for _, app := range exported.Applications {
		byURL[app.PostingURL] = app
	}
	acme := byURL["https://example.com/jobs/1"]
	if acme.Status != domain.StatusAwaitingReview || acme.NextStep != "Send portfolio" || acme.Notes != "updated note" {
		t.Fatalf("acme %#v", acme)
	}
	beta := byURL["https://example.com/jobs/2"]
	if beta.Status != domain.StatusInterviewing || beta.NextStep != "Onsite" || beta.Notes != "loop notes" {
		t.Fatalf("beta %#v", beta)
	}
	code, body, _ = e.do(t, http.MethodPost, "/v1/import/csv", e.token, []byte(csv), map[string]string{"Content-Type": "text/csv"})
	if code != 200 {
		t.Fatalf("reimport %d %s", code, body)
	}
	if err := json.Unmarshal(body, &imported); err != nil {
		t.Fatal(err)
	}
	if imported["applications_created"] != float64(0) || imported["applications_updated"] != float64(3) {
		t.Fatalf("reimport %#v", imported)
	}
	code, raw, _ = e.do(t, http.MethodGet, "/v1/export/json", e.token, nil, nil)
	if code != 200 {
		t.Fatalf("export after reimport %d %s", code, raw)
	}
	if err := json.Unmarshal(raw, &exported); err != nil {
		t.Fatal(err)
	}
	if len(exported.Applications) != 2 {
		t.Fatalf("apps after reimport %d", len(exported.Applications))
	}
	byURL = map[string]domain.Application{}
	for _, app := range exported.Applications {
		byURL[app.PostingURL] = app
	}
	acme = byURL["https://example.com/jobs/1"]
	if acme.Status != domain.StatusAwaitingReview || acme.NextStep != "Send portfolio" || acme.Notes != "updated note" {
		t.Fatalf("acme after reimport %#v", acme)
	}
}

func createChecked(t *testing.T, e *env, postingURL, company, title string) string {
	t.Helper()
	code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": company, "title": title, "status": "to_apply", "posting_url": postingURL,
	})
	if code != 201 {
		t.Fatal(app)
	}
	id := app["id"].(string)
	code, checked := e.json(t, http.MethodPost, "/v1/applications/"+id+"/check-link", e.token, nil)
	if code != 200 {
		t.Fatalf("check %s %#v", postingURL, checked)
	}
	return id
}

func TestStatusExclusion(t *testing.T) {
	e := newEnv(t)
	for _, s := range []struct{ company, status string }{
		{"A Co", "to_apply"}, {"B Co", "applied"}, {"C Co", "rejected"},
	} {
		code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
			"company": s.company, "title": "Eng", "status": s.status,
		})
		if code != 201 {
			t.Fatal(app)
		}
	}
	code, all := e.json(t, http.MethodGet, "/v1/applications", e.token, nil)
	if code != 200 || all["page"].(map[string]any)["total"].(float64) != 3 {
		t.Fatalf("baseline: %d %#v", code, all)
	}
	code, excluded := e.json(t, http.MethodGet, "/v1/applications?status_not=rejected", e.token, nil)
	if code != 200 || excluded["page"].(map[string]any)["total"].(float64) != 2 {
		t.Fatalf("excluding rejected: %d %#v", code, excluded)
	}
	for _, row := range excluded["data"].([]any) {
		if row.(map[string]any)["status"] == "rejected" {
			t.Fatalf("rejected should not appear: %#v", excluded)
		}
	}
	code, both := e.json(t, http.MethodGet, "/v1/applications?status_not=rejected&status_not=applied", e.token, nil)
	if code != 200 || both["page"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("excluding two statuses: %d %#v", code, both)
	}
}

func TestCustomFields(t *testing.T) {
	e := newEnv(t)
	code, field := e.json(t, http.MethodPost, "/v1/fields", e.token, map[string]any{"name": "SalaryRange", "type": "number"})
	if code != 201 || field["type"] != "number" {
		t.Fatalf("create field: %d %#v", code, field)
	}
	fieldID := field["id"].(string)

	code, bad := e.json(t, http.MethodPost, "/v1/fields", e.token, map[string]any{"name": "Bad", "type": "currency"})
	if code != 400 {
		t.Fatalf("invalid type should be rejected: %d %#v", code, bad)
	}

	code, high := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{"company": "High Co", "title": "Eng", "status": "to_apply"})
	if code != 201 {
		t.Fatal(high)
	}
	code, low := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{"company": "Low Co", "title": "Eng", "status": "to_apply"})
	if code != 201 {
		t.Fatal(low)
	}
	highID, lowID := high["id"].(string), low["id"].(string)

	code, setHigh := e.json(t, http.MethodPost, "/v1/applications/"+highID+"/fields/"+fieldID, e.token, map[string]any{"value": "200000"})
	if code != 200 || setHigh["custom_fields"].(map[string]any)["SalaryRange"] != "200000" {
		t.Fatalf("set high: %d %#v", code, setHigh)
	}
	code, setLow := e.json(t, http.MethodPost, "/v1/applications/"+lowID+"/fields/"+fieldID, e.token, map[string]any{"value": "100000"})
	if code != 200 {
		t.Fatalf("set low: %d %#v", code, setLow)
	}
	code, badValue := e.json(t, http.MethodPost, "/v1/applications/"+lowID+"/fields/"+fieldID, e.token, map[string]any{"value": "not-a-number"})
	if code != 400 {
		t.Fatalf("non-numeric value should be rejected: %d %#v", code, badValue)
	}

	code, asc := e.json(t, http.MethodGet, "/v1/applications?sort=field:SalaryRange", e.token, nil)
	if code != 200 {
		t.Fatalf("sort asc: %d %#v", code, asc)
	}
	ascData := asc["data"].([]any)
	if len(ascData) != 2 || ascData[0].(map[string]any)["id"] != lowID || ascData[1].(map[string]any)["id"] != highID {
		t.Fatalf("expected low then high ascending: %#v", ascData)
	}

	code, desc := e.json(t, http.MethodGet, "/v1/applications?sort=-field:SalaryRange", e.token, nil)
	if code != 200 {
		t.Fatalf("sort desc: %d %#v", code, desc)
	}
	descData := desc["data"].([]any)
	if len(descData) != 2 || descData[0].(map[string]any)["id"] != highID {
		t.Fatalf("expected high first descending: %#v", descData)
	}

	code, deleted := e.json(t, http.MethodDelete, "/v1/fields/"+fieldID, e.token, nil)
	if code != 200 {
		t.Fatalf("delete field: %d %#v", code, deleted)
	}
	code, after := e.json(t, http.MethodGet, "/v1/applications/"+highID, e.token, nil)
	if code != 200 || after["custom_fields"] != nil {
		t.Fatalf("deleting a field should clear its values: %d %#v", code, after)
	}
}

func TestArchiveAndUnarchive(t *testing.T) {
	e := newEnv(t)
	code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "Example", "title": "Engineer", "status": "to_apply",
	})
	if code != 201 {
		t.Fatal(app)
	}
	id := app["id"].(string)
	ver := int(app["version"].(float64))

	code, list := e.json(t, http.MethodGet, "/v1/applications", e.token, nil)
	if code != 200 || list["page"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("default list before archive: %d %#v", code, list)
	}

	code, archived := e.json(t, http.MethodPost, "/v1/applications/"+id+"/archive", e.token, map[string]any{"version": ver})
	if code != 200 || archived["archived_at"] == nil {
		t.Fatalf("archive: %d %#v", code, archived)
	}
	if archived["status"] != "to_apply" {
		t.Fatalf("archive should not change status: %#v", archived)
	}

	code, list = e.json(t, http.MethodGet, "/v1/applications", e.token, nil)
	if code != 200 || list["page"].(map[string]any)["total"].(float64) != 0 {
		t.Fatalf("default list should exclude archived: %d %#v", code, list)
	}
	code, list = e.json(t, http.MethodGet, "/v1/applications?archived=true", e.token, nil)
	if code != 200 || list["page"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("archived list should include it: %d %#v", code, list)
	}

	ver = int(archived["version"].(float64))
	code, stale := e.json(t, http.MethodPost, "/v1/applications/"+id+"/archive", e.token, map[string]any{"version": ver - 1})
	if code != 409 {
		t.Fatalf("stale version should conflict: %d %#v", code, stale)
	}

	code, unarchived := e.json(t, http.MethodPost, "/v1/applications/"+id+"/unarchive", e.token, map[string]any{"version": ver})
	if code != 200 || unarchived["archived_at"] != nil {
		t.Fatalf("unarchive: %d %#v", code, unarchived)
	}
	code, list = e.json(t, http.MethodGet, "/v1/applications", e.token, nil)
	if code != 200 || list["page"].(map[string]any)["total"].(float64) != 1 {
		t.Fatalf("default list should include it again: %d %#v", code, list)
	}
}

func TestCompanyFavicon(t *testing.T) {
	e := newEnv(t)
	png := "\x89PNG\r\n\x1a\nlogo"
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><head><link rel="shortcut icon" href="/static/logo.png"></head></html>`))
		case "/static/logo.png", "/custom.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte(png))
		case "/favicon.ico":
			// A bot challenge: 200, but HTML.
			_, _ = w.Write([]byte("<html>just a moment</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer gh.Close()
	e.svc.HTTP = &http.Client{Transport: rewriteTransport(gh.URL)}

	code, company := e.json(t, http.MethodPost, "/v1/companies", e.token, map[string]any{
		"name": "Acme", "website": "https://acme.example.com",
	})
	if code != 201 {
		t.Fatal(company)
	}
	id := company["id"].(string)

	// The HTML decoy at /favicon.ico is skipped; the page-declared icon wins.
	code, body, _ := e.do(t, http.MethodGet, "/v1/companies/"+id+"/favicon", e.token, nil, nil)
	if code != 200 || string(body) != png {
		t.Fatalf("favicon: %d %q", code, body)
	}

	// An icon_url override is tried first, and rejects a non-url.
	code, bad := e.json(t, http.MethodPatch, "/v1/companies/"+id, e.token, map[string]any{"version": 1, "icon_url": "nope"})
	if code != 400 {
		t.Fatalf("bad icon_url: %d %#v", code, bad)
	}
	code, updated := e.json(t, http.MethodPatch, "/v1/companies/"+id, e.token, map[string]any{"version": 1, "icon_url": "https://cdn.example.com/custom.png"})
	if code != 200 || updated["icon_url"] != "https://cdn.example.com/custom.png" {
		t.Fatalf("set icon_url: %d %#v", code, updated)
	}
	code, body, _ = e.do(t, http.MethodGet, "/v1/companies/"+id+"/favicon", e.token, nil, nil)
	if code != 200 || string(body) != png {
		t.Fatalf("override favicon: %d %q", code, body)
	}

	// service=1 only adds a last-resort candidate; it never replaces a working icon.
	code, body, _ = e.do(t, http.MethodGet, "/v1/companies/"+id+"/favicon?service=1", e.token, nil, nil)
	if code != 200 || string(body) != png {
		t.Fatalf("service favicon: %d %q", code, body)
	}

	code, noWebsite := e.json(t, http.MethodPost, "/v1/companies", e.token, map[string]any{"name": "NoSite"})
	if code != 201 {
		t.Fatal(noWebsite)
	}
	code, _, _ = e.do(t, http.MethodGet, "/v1/companies/"+noWebsite["id"].(string)+"/favicon", e.token, nil, nil)
	if code != 404 {
		t.Fatalf("favicon without website should 404: %d", code)
	}
}

func TestFaviconServiceDefault(t *testing.T) {
	for _, c := range []struct {
		tenant, cfg, param string
		want               bool
	}{
		{"single", "", "", false},
		{"multi", "", "", true},
		{"multi", "off", "", false},
		{"single", "on", "", true},
		{"multi", "", "0", false},
		{"single", "", "1", true},
	} {
		s := &Server{TenantMode: c.tenant, FaviconService: c.cfg}
		if got := s.useFaviconService(c.param); got != c.want {
			t.Errorf("%+v: got %v", c, got)
		}
	}
}

type rewriteRT struct{ base string }

func rewriteTransport(base string) http.RoundTripper { return rewriteRT{base} }

func (r rewriteRT) RoundTrip(req *http.Request) (*http.Response, error) {
	next, err := http.NewRequestWithContext(req.Context(), req.Method, r.base+req.URL.RequestURI(), nil)
	if err != nil {
		return nil, err
	}
	next.Header = req.Header
	return http.DefaultTransport.RoundTrip(next)
}

type fnRT struct {
	fn func(*http.Request) (*http.Response, error)
}

func roundTrip(fn func(*http.Request) (*http.Response, error)) http.RoundTripper {
	return fnRT{fn}
}

func (f fnRT) RoundTrip(r *http.Request) (*http.Response, error) { return f.fn(r) }

func TestSmallListsPage(t *testing.T) {
	e := newEnv(t)
	for _, k := range []string{"a", "b", "c"} {
		if code, doc := e.json(t, http.MethodPost, "/v1/standing", e.token, map[string]any{"key": k, "answer": k}); code != 200 {
			t.Fatalf("standing %d %#v", code, doc)
		}
	}
	code, doc := e.json(t, http.MethodGet, "/v1/standing", e.token, nil)
	if code != 200 || len(doc["data"].([]any)) != 3 || doc["page"].(map[string]any)["has_more"] != false {
		t.Fatalf("no limit returns all %d %#v", code, doc)
	}
	var seen int
	path := "/v1/standing?limit=2"
	for pages := 0; ; pages++ {
		code, doc = e.json(t, http.MethodGet, path, e.token, nil)
		if code != 200 || pages > 2 {
			t.Fatalf("page %d %#v", code, doc)
		}
		seen += len(doc["data"].([]any))
		page := doc["page"].(map[string]any)
		if page["has_more"] != true {
			break
		}
		path = "/v1/standing?limit=2&cursor=" + page["next_cursor"].(string)
	}
	if seen != 3 {
		t.Fatalf("paged %d items", seen)
	}
	if code, _ = e.json(t, http.MethodGet, "/v1/standing?fields=key", e.token, nil); code != 400 {
		t.Fatalf("fields on standing %d", code)
	}
	if code, _ = e.json(t, http.MethodGet, "/v1/standing?limit=500", e.token, nil); code != 400 {
		t.Fatalf("limit 500 %d", code)
	}
}

func TestBatchRunsEachItemAsTheCaller(t *testing.T) {
	e := newEnv(t)
	code, issued := e.json(t, http.MethodPost, "/v1/tokens", e.token, map[string]any{"name": "agent", "scopes": []string{"read", "write"}})
	if code != 201 {
		t.Fatal(issued)
	}
	agent := issued["secret"].(string)
	batch := map[string]any{"requests": []map[string]any{
		{"method": "POST", "path": "/v1/companies", "body": map[string]any{"name": "Batch A"}, "idempotency_key": "batch-a"},
		{"method": "POST", "path": "/v1/companies", "body": map[string]any{"name": "Batch B"}},
		{"method": "POST", "path": "/v1/approvals", "body": map[string]any{"ids": []string{"x"}}},
		{"method": "POST", "path": "/v1/batch", "body": map[string]any{"requests": []any{}}},
		{"method": "GET", "path": "/v1/companies?q=Batch&fields=name"},
	}}
	code, doc := e.json(t, http.MethodPost, "/v1/batch", agent, batch)
	if code != 200 {
		t.Fatalf("batch %d %#v", code, doc)
	}
	results := doc["results"].([]any)
	for i, want := range []float64{201, 201, 403, 400, 200} {
		if got := results[i].(map[string]any)["status"].(float64); got != want {
			t.Fatalf("item %d status %v want %v: %#v", i, got, want, results[i])
		}
	}
	if doc["failed"].(float64) != 2 {
		t.Fatalf("failed %#v", doc["failed"])
	}
	if list := results[4].(map[string]any)["body"].(map[string]any)["data"].([]any); len(list) != 2 {
		t.Fatalf("companies %#v", list)
	}

	// Replaying the keyed item does not create a second company.
	code, doc = e.json(t, http.MethodPost, "/v1/batch", agent, map[string]any{"requests": []map[string]any{
		{"method": "POST", "path": "/v1/companies", "body": map[string]any{"name": "Batch A"}, "idempotency_key": "batch-a"},
	}})
	if code != 200 || doc["results"].([]any)[0].(map[string]any)["status"].(float64) != 201 {
		t.Fatalf("replay %d %#v", code, doc)
	}
	if code, doc = e.json(t, http.MethodGet, "/v1/companies?q=Batch", agent, nil); len(doc["data"].([]any)) != 2 {
		t.Fatalf("after replay %d %#v", code, doc)
	}

	if code, doc = e.json(t, http.MethodPost, "/v1/batch", agent, map[string]any{"requests": []any{}}); code != 400 {
		t.Fatalf("empty batch %d %#v", code, doc)
	}
	raw, _ := json.Marshal(batch)
	if code, _, _ := e.do(t, http.MethodPost, "/v1/batch", agent, raw, map[string]string{"Idempotency-Key": "whole"}); code != 400 {
		t.Fatalf("outer idempotency key %d", code)
	}
}

func TestUpdateMe(t *testing.T) {
	e := newEnv(t)
	png := "data:image/png;base64,iVBORw0KGgo="
	code, user := e.json(t, http.MethodPatch, "/v1/me", e.token, map[string]any{"name": "Ada", "avatar": png})
	if code != 200 || user["name"] != "Ada" || user["avatar"] != png {
		t.Fatalf("patch me %d %#v", code, user)
	}
	code, _ = e.json(t, http.MethodPatch, "/v1/me", e.token, map[string]any{"avatar": "data:image/svg+xml;base64,PHN2Zz4="})
	if code < 400 || code >= 500 {
		t.Fatalf("svg avatar accepted %d", code)
	}
	code, me := e.json(t, http.MethodGet, "/v1/me", e.token, nil)
	if code != 200 || me["user"].(map[string]any)["name"] != "Ada" {
		t.Fatalf("me %#v", me)
	}
}

func TestDeleteStanding(t *testing.T) {
	e := newEnv(t)
	if code, _ := e.json(t, http.MethodPost, "/v1/standing", e.token, map[string]any{"key": "Why us?", "answer": "x"}); code != 200 {
		t.Fatal(code)
	}
	if code, out := e.json(t, http.MethodDelete, "/v1/standing/why_us", e.token, nil); code != 200 {
		t.Fatalf("delete %d %#v", code, out)
	}
	if code, _ := e.json(t, http.MethodDelete, "/v1/standing/why_us", e.token, nil); code != 404 {
		t.Fatalf("second delete %d", code)
	}
}

func TestTokensMintOnlyScopesTheyHold(t *testing.T) {
	e := newEnv(t)
	mint := func(token string, scopes ...string) (int, map[string]any) {
		return e.json(t, http.MethodPost, "/v1/tokens", token, map[string]any{"name": "t", "scopes": scopes})
	}
	// The bootstrap owner token is the human holding approve.
	code, doc := mint(e.token, "read", "write", "approve")
	if code != 201 {
		t.Fatalf("owner approve %d %#v", code, doc)
	}
	_, doc = mint(e.token, "read", "write")
	writer := doc["secret"].(string)
	_, doc = mint(e.token, "read")
	reader := doc["secret"].(string)

	for _, c := range []struct {
		token  string
		scopes []string
		want   int
	}{
		{writer, []string{"approve"}, 403},
		{writer, []string{"write", "approve"}, 403},
		{reader, []string{"write"}, 403},
		{writer, []string{"write"}, 201},
		{writer, []string{"read"}, 201},
	} {
		code, doc := mint(c.token, c.scopes...)
		if code != c.want || (c.want == 403 && errCode(doc) != "forbidden") {
			t.Fatalf("%v: %d %#v", c.scopes, code, doc)
		}
	}

	// The PURSUIT_TRUST_PROXY_AUTH owner cannot mint approve either, the
	// same as it cannot consent to OAuth logins. (Proxy auth needs exactly
	// one user, so this runs before the demo tenant exists.)
	e.api.TrustProxyAuth = "vercel"
	if code, doc := mint("", "approve"); code != 403 {
		t.Fatalf("proxy approve %d %#v", code, doc)
	}
	if code, doc := mint("", "write"); code != 201 {
		t.Fatalf("proxy write %d %#v", code, doc)
	}
	e.api.TrustProxyAuth = ""

	// A demo visitor holds approve on its throwaway tenant but is not a
	// human approver, so it cannot mint an approve token.
	demo, _, err := e.svc.CreateDemoSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if code, doc := mint(demo, "approve"); code != 403 {
		t.Fatalf("demo approve %d %#v", code, doc)
	}
	if code, doc := mint(demo, "write"); code != 201 {
		t.Fatalf("demo write %d %#v", code, doc)
	}
}

func TestAuditSearchAndTimeFilters(t *testing.T) {
	e := newEnv(t)
	code, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{
		"company": "TraceCo", "title": "Traced role", "status": "to_apply",
	})
	if code != 201 {
		t.Fatal(app)
	}
	id := app["id"].(string)

	count := func(query string) int {
		t.Helper()
		code, page := e.json(t, http.MethodGet, "/v1/audit?"+query, e.token, nil)
		if code != 200 {
			t.Fatalf("%s: %d %#v", query, code, page)
		}
		return len(page["data"].([]any))
	}
	if count("q=traceco") == 0 || count("q=TRACED") == 0 {
		t.Fatal("q should match the summary and metadata case-insensitively")
	}
	if count("q=no-such-text") != 0 {
		t.Fatal("q with no match should return nothing")
	}
	if count("entity_id="+id) == 0 {
		t.Fatal("entity_id should trace one application")
	}
	if count("since=2999-01-01") != 0 || count("until=2000-01-01") != 0 {
		t.Fatal("time bounds should exclude everything")
	}
	if count("since=2000-01-01&until=2999-01-01") == 0 {
		t.Fatal("wide time bounds should include everything")
	}
	if code, bad := e.json(t, http.MethodGet, "/v1/audit?since=yesterday", e.token, nil); code != 400 || errCode(bad) != "validation" {
		t.Fatalf("bad since %d %#v", code, bad)
	}
}
