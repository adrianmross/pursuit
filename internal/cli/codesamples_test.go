package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/pursuit/pursuit/internal/spec"
)

// TestOpenAPICodeSamples keeps the pursuit commands shown in the API
// reference real: each names a command and only flags that command takes.
func TestOpenAPICodeSamples(t *testing.T) {
	var doc struct {
		Paths map[string]map[string]struct {
			Samples []struct {
				Source string `json:"source"`
			} `json:"x-codeSamples"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(spec.Document(), &doc); err != nil {
		t.Fatal(err)
	}
	global := map[string]bool{"json": true, "text": true, "dry-run": true, "all-pages": true, "server": true, "token": true, "config": true, "idempotency-key": true, "fields": true}
	n := 0
	for path, ops := range doc.Paths {
		for method, op := range ops {
			for _, s := range op.Samples {
				n++
				words := strings.Fields(s.Source)
				if len(words) < 2 || words[0] != "pursuit" {
					t.Errorf("%s %s: sample %q is not a pursuit command", method, path, s.Source)
					continue
				}
				// The command is the longest prefix pursuit schema knows.
				var cmd *Command
				for i := len(words); i > 1 && cmd == nil; i-- {
					var out bytes.Buffer
					if Run(append([]string{"schema"}, words[1:i]...), &out, io.Discard) == 0 {
						cmd = &Command{}
						if err := json.Unmarshal(out.Bytes(), cmd); err != nil {
							t.Fatal(err)
						}
					}
				}
				if cmd == nil {
					t.Errorf("%s %s: %q names no pursuit command", method, path, s.Source)
					continue
				}
				flags := map[string]bool{}
				for _, f := range cmd.Flags {
					flags[f.Name] = true
				}
				for _, w := range words {
					name, _, _ := strings.Cut(strings.TrimPrefix(w, "--"), "=")
					if strings.HasPrefix(w, "--") && !flags[name] && !global[name] {
						t.Errorf("%s %s: %q passes --%s, which %s does not take", method, path, s.Source, name, cmd.Name)
					}
				}
			}
		}
	}
	if n < 50 {
		t.Fatalf("found only %d code samples; did x-codeSamples move?", n)
	}
}
