package dashboard

import "testing"

func TestContentTypeServesVideo(t *testing.T) {
	for name, want := range map[string]string{"a/b.mp4": "video/mp4", "a/b.webm": "video/webm", "a/b.html": "text/html; charset=utf-8"} {
		if got := contentType(name); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}
