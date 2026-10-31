package spec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestOpenAPIFile keeps api/openapi.json equal to the served document.
// UPDATE_OPENAPI=1 go test ./internal/spec rewrites it (make openapi).
func TestOpenAPIFile(t *testing.T) {
	path := filepath.Join("..", "..", "api", "openapi.json")
	var want bytes.Buffer
	if err := json.Indent(&want, Document(), "", "  "); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE_OPENAPI") != "" {
		if err := os.WriteFile(path, want.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Compare parsed JSON so release-please's version rewrite can't trip on layout.
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatalf("api/openapi.json: %v", err)
	}
	_ = json.Unmarshal(want.Bytes(), &b)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if !bytes.Equal(ja, jb) {
		t.Fatal("api/openapi.json is stale; run make openapi")
	}
}
