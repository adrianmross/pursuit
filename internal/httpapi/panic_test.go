package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPanicIsInternalError(t *testing.T) {
	failures := 0
	s := &Server{OnFailure: func(_ *http.Request, _ string, status int, _ any) {
		failures++
		if status != http.StatusInternalServerError {
			t.Errorf("OnFailure status = %d, want 500", status)
		}
	}}
	h := s.wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), `"code":"internal"`) {
		t.Errorf("got %d %s, want 500 with code internal", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "boom") {
		t.Errorf("panic value leaked to the client: %s", w.Body.String())
	}
	if failures != 1 {
		t.Errorf("OnFailure ran %d times, want 1", failures)
	}
}
