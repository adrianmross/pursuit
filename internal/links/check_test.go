package links

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pursuit/pursuit/internal/domain"
)

func TestLinkFixtures(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/boards/acme/jobs/111", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":111,"title":"Engineer"}`))
	})
	mux.HandleFunc("/v1/boards/acme/jobs/222", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/posting-api/job-board/openai", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jobs":[{"id":"abc12345-aaaa-bbbb-cccc-ddddeeeeffff","title":"IAM","jobUrl":"https://jobs.ashbyhq.com/openai/abc12345-aaaa-bbbb-cccc-ddddeeeeffff","isListed":true}]}`))
	})
	mux.HandleFunc("/v0/postings/zoox/deadbeef-dead-beef-dead-beefdeadbeef", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/careers/closed", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>no longer accepting applications</html>"))
	})
	mux.HandleFunc("/boards/root", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("board"))
	})
	mux.HandleFunc("/boards/root/jobs/999999", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/boards/root", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := &Client{HTTP: &http.Client{Transport: rewrite(srv.URL)}}

	live := c.Check(context.Background(), "https://job-boards.greenhouse.io/acme/jobs/111")
	if live.Status != domain.LinkLive {
		t.Fatalf("greenhouse live: %+v", live)
	}
	closed := c.Check(context.Background(), "https://boards.greenhouse.io/acme/jobs/222")
	if closed.Status != domain.LinkClosed {
		t.Fatalf("greenhouse closed: %+v", closed)
	}
	ash := c.Check(context.Background(), "https://jobs.ashbyhq.com/openai/abc12345-aaaa-bbbb-cccc-ddddeeeeffff")
	if ash.Status != domain.LinkLive {
		t.Fatalf("ashby: %+v", ash)
	}
	missing := c.Check(context.Background(), "https://jobs.ashbyhq.com/openai/ffffffffffffffffffffffffffffffff")
	if missing.Status != domain.LinkClosed {
		t.Fatalf("ashby missing: %+v", missing)
	}
	lever := c.Check(context.Background(), "https://jobs.lever.co/zoox/deadbeef-dead-beef-dead-beefdeadbeef")
	if lever.Status != domain.LinkClosed {
		t.Fatalf("lever: %+v", lever)
	}
	phrase := c.Check(context.Background(), srv.URL+"/careers/closed")
	if phrase.Status != domain.LinkClosed || !strings.Contains(phrase.Detail, "no longer accepting") {
		t.Fatalf("phrase: %+v", phrase)
	}
	redir := c.Check(context.Background(), srv.URL+"/boards/root/jobs/999999")
	if redir.Status != domain.LinkClosed {
		t.Fatalf("redirect: %+v", redir)
	}
}

type rewriteRoundTripper struct{ base string }

func rewrite(base string) http.RoundTripper { return rewriteRoundTripper{base} }

func (r rewriteRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	next, err := http.NewRequestWithContext(req.Context(), req.Method, r.base+req.URL.RequestURI(), nil)
	if err != nil {
		return nil, err
	}
	next.Header = req.Header
	return http.DefaultTransport.RoundTrip(next)
}
