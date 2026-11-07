package httpapi

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/pursuit/pursuit/internal/blobstore"
)

func (e *env) docs(t *testing.T) map[string]map[string]any {
	t.Helper()
	code, list := e.json(t, http.MethodGet, "/v1/documents", e.token, nil)
	if code != 200 {
		t.Fatal(list)
	}
	out := map[string]map[string]any{}
	for _, d := range list["data"].([]any) {
		m := d.(map[string]any)
		out[m["id"].(string)] = m
	}
	return out
}

func addText(t *testing.T, e *env, kind, name, body string) string {
	t.Helper()
	code, d := e.json(t, http.MethodPost, "/v1/documents", e.token, map[string]any{"kind": kind, "filename": name, "content_type": "text/plain", "body": body})
	if code != 201 {
		t.Fatalf("add %s: %d %#v", name, code, d)
	}
	return d["id"].(string)
}

// One main résumé: the first, until another is chosen or it's deleted.
func TestDocumentPrimary(t *testing.T) {
	e := newEnv(t)
	first := addText(t, e, "base_resume", "one.txt", "one")
	second := addText(t, e, "base_resume", "two.txt", "two")
	other := addText(t, e, "other", "notes.txt", "x")
	d := e.docs(t)
	if d[first]["primary"] != true || d[second]["primary"] != false || d[other]["primary"] != false {
		t.Fatalf("after upload: %v %v %v", d[first]["primary"], d[second]["primary"], d[other]["primary"])
	}
	if code, out := e.json(t, http.MethodPatch, "/v1/documents/"+second, e.token, map[string]any{"primary": true, "label": "Backend"}); code != 200 || out["label"] != "Backend" {
		t.Fatalf("set primary %d %#v", code, out)
	}
	if d = e.docs(t); d[first]["primary"] != false || d[second]["primary"] != true {
		t.Fatalf("after switch: %v %v", d[first]["primary"], d[second]["primary"])
	}
	if code, _ := e.json(t, http.MethodPatch, "/v1/documents/"+other, e.token, map[string]any{"primary": true}); code != 400 {
		t.Fatalf("other kind has no main version: %d", code)
	}
	if code, _ := e.json(t, http.MethodDelete, "/v1/documents/"+second, e.token, nil); code != 200 {
		t.Fatal(code)
	}
	if d = e.docs(t); d[first]["primary"] != true {
		t.Fatalf("deleting the main one should promote the other: %v", d[first])
	}

	// A text document is edited in place; a binary one is replaced by upload.
	if code, out := e.json(t, http.MethodPatch, "/v1/documents/"+first, e.token, map[string]any{"body": "edited"}); code != 200 || out["size"].(float64) != 6 {
		t.Fatalf("edit text %d %#v", code, out)
	}
	if _, got := e.json(t, http.MethodGet, "/v1/documents/"+first, e.token, nil); got["body"] != "edited" {
		t.Fatalf("body %#v", got["body"])
	}
}

// With a bucket, new files go there and come back the same; delete removes them.
func TestDocumentS3(t *testing.T) {
	e := newEnv(t)
	blobs := &blobstore.Memory{}
	e.svc.Blobs = blobs
	pdf := append([]byte("%PDF-1.7\n"), 0xff, 0x00, 0x10)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("kind", "base_resume")
	_ = mw.WriteField("label", "Main")
	fw, _ := mw.CreateFormFile("file", "resume.pdf")
	_, _ = fw.Write(pdf)
	_ = mw.Close()
	code, raw, _ := e.do(t, http.MethodPost, "/v1/documents", e.token, buf.Bytes(), map[string]string{"Content-Type": mw.FormDataContentType()})
	if code != 201 || blobs.Len() != 1 {
		t.Fatalf("upload %d %s blobs=%d", code, raw, blobs.Len())
	}
	var id string
	for k, d := range e.docs(t) {
		if d["storage"] != "s3" || d["label"] != "Main" || d["primary"] != true {
			t.Fatalf("stored %#v", d)
		}
		id = k
	}
	_, got := e.json(t, http.MethodGet, "/v1/documents/"+id, e.token, nil)
	if b, err := base64.StdEncoding.DecodeString(got["body"].(string)); err != nil || !bytes.Equal(b, pdf) {
		t.Fatalf("round trip %q %v", b, err)
	}
	if code, _ := e.json(t, http.MethodPatch, "/v1/documents/"+id, e.token, map[string]any{"body": "text"}); code != 400 {
		t.Fatalf("editing a PDF in place: %d", code)
	}
	if code, _ := e.json(t, http.MethodDelete, "/v1/documents/"+id, e.token, nil); code != 200 || blobs.Len() != 0 {
		t.Fatalf("delete %d blobs=%d", code, blobs.Len())
	}
}
