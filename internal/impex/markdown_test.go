package impex

import (
	"bytes"
	"os"
	"testing"
)

func TestMarkdownBatches(t *testing.T) {
	b3, err := os.ReadFile("../../testdata/seed/batch3.md")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ParseMarkdownTables(bytes.NewReader(b3))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("batch3 rows=%d, want the table plus one bullet", len(rows))
	}
	b4, err := os.ReadFile("../../testdata/seed/batch4.md")
	if err != nil {
		t.Fatal(err)
	}
	rows4, err := ParseMarkdownTables(bytes.NewReader(b4))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows4) != 2 {
		for _, row := range rows4 {
			t.Log(row.Company, "|", row.Title, "|", row.URL)
		}
		t.Fatalf("batch4 rows=%d", len(rows4))
	}
	for _, row := range append(rows, rows4...) {
		if row.URL == "" || row.Company == "" || row.Title == "" {
			t.Fatalf("incomplete row %+v", row)
		}
	}
}
