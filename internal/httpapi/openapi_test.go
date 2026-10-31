package httpapi

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/pursuit/pursuit/internal/spec"
)

// TestOpenAPICoversRoutes fails when a route registered in Handler is
// missing from the served OpenAPI document.
func TestOpenAPICoversRoutes(t *testing.T) {
	src, err := os.ReadFile("http.go")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(spec.Document(), &doc); err != nil {
		t.Fatal(err)
	}
	routes := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/[^"]*)"`).FindAllStringSubmatch(string(src), -1)
	if len(routes) < 50 {
		t.Fatalf("found only %d routes in http.go; did the registration style change?", len(routes))
	}
	for _, m := range routes {
		method, path := strings.ToLower(m[1]), m[2]
		if path == "/" { // dashboard SPA shell, not an API route
			continue
		}
		if _, ok := doc.Paths[path][method]; !ok {
			t.Errorf("%s %s is routed but missing from internal/spec", m[1], path)
		}
	}
}
