package handler

import (
	"os"
	"strings"
	"testing"
)

func TestEntrypointDoesNotImportInternal(t *testing.T) {
	b, err := os.ReadFile("index.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "internal/") {
		t.Fatalf("api/index.go imports an internal package:\n%s", b)
	}
	if !strings.Contains(string(b), "github.com/pursuit/pursuit/pkg/vercelapp") {
		t.Fatalf("api/index.go should call pkg/vercelapp:\n%s", b)
	}
}
