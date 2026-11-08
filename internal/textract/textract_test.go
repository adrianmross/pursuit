package textract

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// The fixtures were made on macOS from the same three lines: cupsfilter for
// the PDF, textutil for the Word file.
func TestText(t *testing.T) {
	for _, c := range []struct{ file, ct string }{
		{"resume.pdf", "application/pdf"},
		{"resume.docx", docxType},
		{"resume.docx", ""}, // by extension
	} {
		raw, err := os.ReadFile("testdata/" + c.file)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Text(c.ct, c.file, raw)
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		for _, want := range []string{"Jordan Example", "Platform engineer", "developer tools"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: missing %q in %q", c.file, want, got)
			}
		}
	}
	if got, _ := Text("text/markdown", "a.md", []byte("# Hi")); got != "# Hi" {
		t.Fatalf("text: %q", got)
	}
	if _, err := Text("image/png", "a.png", []byte{1}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("png: %v", err)
	}
	if _, err := Text("application/pdf", "a.pdf", []byte("%PDF-1.4 broken")); err == nil {
		t.Fatal("broken PDF should fail, not panic")
	}
}
