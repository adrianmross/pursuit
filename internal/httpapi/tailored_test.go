package httpapi

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestTailoredDocuments(t *testing.T) {
	e := newEnv(t)
	e.svc.PremiumOn = true
	main := addText(t, e, "base_resume", "resume.txt", "Jordan Example\nBuilt developer tools in Go.")
	backend := addText(t, e, "base_resume", "backend.txt", "Jordan Example\nBuilt distributed systems in Go.")
	_, app := e.json(t, http.MethodPost, "/v1/applications", e.token, map[string]any{"company": "Example", "title": "Engineer"})
	id := app["id"].(string)
	use := func(doc string) (int, map[string]any) {
		return e.json(t, http.MethodPost, "/v1/applications/"+id+"/documents", e.token, map[string]any{"document_id": doc})
	}

	// The main résumé is free; another version and the comparison need the feature.
	if code, out := use(main); code != 201 || out["kind"] != "resume_pdf" || out["source_document_id"] != main {
		t.Fatalf("use main %d %#v", code, out)
	}
	if code, out := use(backend); code != 403 || out["error"].(map[string]any)["code"] != "feature_unavailable" {
		t.Fatalf("variant without the feature %d %#v", code, out)
	}
	if code, _ := e.json(t, http.MethodGet, "/v1/applications/"+id+"/compare", e.token, nil); code != 403 {
		t.Fatalf("compare without the feature %d", code)
	}
	e.json(t, http.MethodPatch, "/v1/features/tailored_documents", e.token, map[string]any{"enabled": true})

	code, cmp := e.json(t, http.MethodGet, "/v1/applications/"+id+"/compare?kind=resume_pdf", e.token, nil)
	if code != 200 || cmp["same"] != true || cmp["current"].(map[string]any)["source"] != "default" {
		t.Fatalf("compare default %d %#v", code, cmp)
	}
	if code, _ := use(backend); code != 201 {
		t.Fatalf("use variant %d", code)
	}
	_, cmp = e.json(t, http.MethodGet, "/v1/applications/"+id+"/compare?kind=resume_pdf", e.token, nil)
	cur := cmp["current"].(map[string]any)
	if cmp["same"] != false || cur["source"] != "version" || !strings.Contains(cur["text"].(string), "distributed systems") {
		t.Fatalf("compare variant %#v", cmp)
	}

	// A tailored PDF uploaded to the application is read as text too.
	pdf, err := os.ReadFile("../textract/testdata/resume.pdf")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("kind", "resume_pdf")
	fw, _ := mw.CreateFormFile("file", "tailored.pdf")
	_, _ = fw.Write(pdf)
	_ = mw.Close()
	if code, raw, _ := e.do(t, http.MethodPost, "/v1/applications/"+id+"/artifacts", e.token, buf.Bytes(), map[string]string{"Content-Type": mw.FormDataContentType()}); code != 201 {
		t.Fatalf("upload %d %s", code, raw)
	}
	_, cmp = e.json(t, http.MethodGet, "/v1/applications/"+id+"/compare", e.token, nil)
	cur = cmp["current"].(map[string]any)
	if cur["source"] != "upload" || !strings.Contains(cur["text"].(string), "Platform engineer") {
		t.Fatalf("compare upload %#v", cmp)
	}
	if code, _ := e.json(t, http.MethodGet, "/v1/applications/"+id+"/compare?kind=cover_letter", e.token, nil); code != 404 {
		t.Fatalf("no cover letter on the application: %d", code)
	}
}
