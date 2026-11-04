package service

import (
	"strings"
	"testing"
)

func TestChunks(t *testing.T) {
	if got := chunks("  short  "); len(got) != 1 || got[0] != "short" {
		t.Fatalf("short: %q", got)
	}
	para := strings.Repeat("word ", 300) // 1500 chars
	long := para + "\n\n" + para + "\n\n" + strings.Repeat("x", 4500)
	got := chunks(long)
	total := 0
	for _, c := range got {
		if len(c) > chunkSize {
			t.Fatalf("chunk of %d > %d", len(c), chunkSize)
		}
		total += len(strings.ReplaceAll(strings.ReplaceAll(c, " ", ""), "\n", ""))
	}
	if want := len(strings.ReplaceAll(strings.ReplaceAll(long, " ", ""), "\n", "")); total != want {
		t.Fatalf("lost text: %d of %d", total, want)
	}
	if len(got) < 4 {
		t.Fatalf("want paragraphs kept apart and the long run split, got %d chunks", len(got))
	}
}
