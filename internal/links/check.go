// Package links re-checks that a posting is still live.
// It prefers the public Greenhouse, Ashby, and Lever APIs and falls back
// to HTTP status plus a few closed-posting phrases.
package links

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
)

const maxBody = 1 << 20

type Result struct {
	Status     string
	Detail     string
	HTTPStatus int
	CheckedAt  time.Time
}

type Client struct {
	HTTP *http.Client
	Now  func() time.Time
	// Gap is the minimum delay between requests to the same host. Zero disables it.
	Gap time.Duration
	mu   sync.Mutex
	next map[string]time.Time
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Client) wait(ctx context.Context, host string) error {
	if c.Gap <= 0 || host == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.next == nil {
		c.next = map[string]time.Time{}
	}
	if t, ok := c.next[host]; ok {
		if d := t.Sub(c.now()); d > 0 {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	c.next[host] = c.now().Add(c.Gap)
	return nil
}

// Check classifies one posting URL.
func (c *Client) Check(ctx context.Context, raw string) Result {
	out := Result{Status: domain.LinkUnknown, CheckedAt: c.now().UTC()}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		out.Detail = "posting url is empty"
		return out
	}
	if board, id, ok := greenhouseRef(raw); ok {
		return c.checkGreenhouse(ctx, board, id, out)
	}
	if board, id, ok := ashbyRef(raw); ok {
		return c.checkAshby(ctx, board, id, out)
	}
	if site, id, ok := leverRef(raw); ok {
		return c.checkLever(ctx, site, id, out)
	}
	if host, tenant, site, jobPath, ok := workdayRef(raw); ok {
		return c.checkWorkday(ctx, host, tenant, site, jobPath, out)
	}
	if isLinkedIn(raw) {
		return c.checkLinkedIn(ctx, raw, out)
	}
	return c.checkHTTP(ctx, raw, "", out)
}

func (c *Client) checkGreenhouse(ctx context.Context, board, id string, out Result) Result {
	api := "https://boards-api.greenhouse.io/v1/boards/" + url.PathEscape(board) + "/jobs/" + url.PathEscape(id)
	status, final, body, err := c.get(ctx, api)
	out.HTTPStatus = status
	if err != nil {
		out.Detail = "greenhouse api: " + err.Error()
		return out
	}
	if status == http.StatusNotFound || status == http.StatusGone {
		out.Status = domain.LinkClosed
		out.Detail = "greenhouse job endpoint returned " + http.StatusText(status)
		return out
	}
	if status >= 200 && status < 300 {
		var payload struct {
			ID     int64  `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal(body, &payload)
		if payload.ID == 0 && !strings.Contains(string(body), `"title"`) {
			return c.checkHTTP(ctx, "", final, out)
		}
		out.Status = domain.LinkLive
		out.Detail = "greenhouse boards-api lists this job"
		return out
	}
	out.Detail = "greenhouse api status " + http.StatusText(status)
	return out
}

func (c *Client) checkAshby(ctx context.Context, board, id string, out Result) Result {
	api := "https://api.ashbyhq.com/posting-api/job-board/" + url.PathEscape(board)
	status, _, body, err := c.get(ctx, api)
	out.HTTPStatus = status
	if err != nil {
		out.Detail = "ashby api: " + err.Error()
		return out
	}
	if status == http.StatusNotFound || status == http.StatusGone {
		out.Status = domain.LinkClosed
		out.Detail = "ashby job board was not found"
		return out
	}
	if status < 200 || status >= 300 {
		out.Detail = "ashby api status " + http.StatusText(status)
		return out
	}
	var payload struct {
		Jobs []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			JobURL   string `json:"jobUrl"`
			IsListed bool   `json:"isListed"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		out.Detail = "ashby api returned unexpected json"
		return out
	}
	for _, job := range payload.Jobs {
		if strings.EqualFold(job.ID, id) || strings.Contains(job.JobURL, id) {
			if job.IsListed || job.Title != "" {
				out.Status = domain.LinkLive
				out.Detail = "ashby posting api lists this job"
				return out
			}
		}
	}
	out.Status = domain.LinkClosed
	out.Detail = "ashby posting api does not list this job"
	return out
}

func (c *Client) checkLever(ctx context.Context, site, id string, out Result) Result {
	api := "https://api.lever.co/v0/postings/" + url.PathEscape(site) + "/" + url.PathEscape(id)
	status, _, body, err := c.get(ctx, api)
	out.HTTPStatus = status
	if err != nil {
		out.Detail = "lever api: " + err.Error()
		return out
	}
	if status == http.StatusNotFound || status == http.StatusGone {
		out.Status = domain.LinkClosed
		out.Detail = "lever posting endpoint returned " + http.StatusText(status)
		return out
	}
	if status >= 200 && status < 300 && strings.Contains(string(body), `"text"`) {
		out.Status = domain.LinkLive
		out.Detail = "lever posting api lists this job"
		return out
	}
	out.Detail = "lever api status " + http.StatusText(status)
	return out
}

// checkWorkday uses the public CXS endpoint Workday-hosted career sites
// serve the job detail page from (no auth required). Falls back to the
// generic HTTP check if the response doesn't look like job JSON, the same
// way checkGreenhouse does.
func (c *Client) checkWorkday(ctx context.Context, host, tenant, site, jobPath string, out Result) Result {
	api := "https://" + host + "/wday/cxs/" + tenant + "/" + site + "/" + jobPath
	status, final, body, err := c.get(ctx, api)
	out.HTTPStatus = status
	if err != nil {
		out.Detail = "workday cxs api: " + err.Error()
		return out
	}
	if status == http.StatusNotFound || status == http.StatusGone {
		out.Status = domain.LinkClosed
		out.Detail = "workday cxs api returned " + http.StatusText(status)
		return out
	}
	if status >= 200 && status < 300 {
		var payload struct {
			JobPostingInfo struct {
				Title string `json:"title"`
			} `json:"jobPostingInfo"`
		}
		_ = json.Unmarshal(body, &payload)
		if payload.JobPostingInfo.Title == "" {
			return c.checkHTTP(ctx, "", final, out)
		}
		out.Status = domain.LinkLive
		out.Detail = "workday cxs api lists this job"
		return out
	}
	out.Detail = "workday cxs api status " + http.StatusText(status)
	return out
}

var linkedinClosedPhrases = []string{
	"no longer accepting applications",
	"this job is no longer available",
	"this job is no longer accepting applications",
}

// checkLinkedIn falls back to the generic HTTP check (LinkedIn has no public
// unauthenticated job-status API) but treats an anonymous-view login wall as
// LinkUnknown instead of guessing live or closed from a page with no job
// content, and recognizes LinkedIn's specific closed-posting phrasing.
func (c *Client) checkLinkedIn(ctx context.Context, raw string, out Result) Result {
	status, final, body, err := c.get(ctx, raw)
	out.HTTPStatus = status
	if err != nil {
		out.Detail = "linkedin: " + err.Error()
		return out
	}
	text := strings.ToLower(string(body))
	if strings.Contains(text, "authwall") || strings.Contains(text, "join now to see") {
		out.Detail = "linkedin requires sign-in to view this posting"
		return out
	}
	for _, phrase := range linkedinClosedPhrases {
		if strings.Contains(text, phrase) {
			out.Status = domain.LinkClosed
			out.Detail = "page says " + phrase
			return out
		}
	}
	return c.checkHTTP(ctx, "", final, out)
}

func (c *Client) checkHTTP(ctx context.Context, raw, already string, out Result) Result {
	if raw == "" && already == "" {
		return out
	}
	target := raw
	if target == "" {
		target = already
	}
	status, final, body, err := c.get(ctx, target)
	if already == "" {
		out.HTTPStatus = status
	}
	if err != nil {
		out.Detail = "http: " + err.Error()
		return out
	}
	if status == http.StatusNotFound || status == http.StatusGone {
		out.Status = domain.LinkClosed
		out.Detail = "posting returned " + http.StatusText(status)
		return out
	}
	text := strings.ToLower(string(body))
	for _, phrase := range []string{
		"no longer accepting",
		"no longer available",
		"position has been filled",
		"this position has been filled",
		"job is closed",
		"this job has expired",
		"posting is not available",
		"sorry, this job is no longer",
	} {
		if strings.Contains(text, phrase) {
			out.Status = domain.LinkClosed
			out.Detail = "page says " + phrase
			return out
		}
	}
	if redirectedToBoardRoot(target, final) {
		out.Status = domain.LinkClosed
		out.Detail = "redirected to the board root"
		return out
	}
	if status >= 200 && status < 300 {
		out.Status = domain.LinkLive
		out.Detail = "posting url returned " + http.StatusText(status)
		return out
	}
	out.Status = domain.LinkUnknown
	out.Detail = "unexpected http status " + http.StatusText(status)
	return out
}

func (c *Client) get(ctx context.Context, raw string) (int, string, []byte, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return 0, "", nil, err
	}
	if err := c.wait(ctx, u.Host); err != nil {
		return 0, "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return 0, "", nil, err
	}
	req.Header.Set("User-Agent", "pursuit/link-check")
	req.Header.Set("Accept", "application/json,text/html;q=0.9,*/*;q=0.8")
	resp, err := c.client().Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return resp.StatusCode, resp.Request.URL.String(), nil, err
	}
	final := raw
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	return resp.StatusCode, final, body, nil
}

var (
	ghPath     = regexp.MustCompile(`(?i)^/(?:[^/]+/)?([^/]+)/jobs/(\d+)`)
	ashbyPath  = regexp.MustCompile(`(?i)^/([^/]+)/([0-9a-f-]{8,})`)
	leverPath  = regexp.MustCompile(`(?i)^/([^/]+)/([0-9a-f-]{8,})`)
)

func greenhouseRef(raw string) (board, id string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	host := strings.ToLower(u.Host)
	if strings.Contains(host, "greenhouse.io") {
		if m := ghPath.FindStringSubmatch(u.Path); m != nil {
			return m[1], m[2], true
		}
	}
	if jid := u.Query().Get("gh_jid"); jid != "" && ghBoard(host, u.Path) != "" {
		return ghBoard(host, u.Path), jid, true
	}
	return "", "", false
}

func ghBoard(host, path string) string {
	switch {
	case strings.Contains(host, "databricks.com"):
		return "databricks"
	case strings.Contains(host, "elastic.co"):
		return "elastic"
	case strings.Contains(host, "waymo"):
		return "waymo"
	default:
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) > 0 && parts[0] != "" && parts[0] != "jobs" && parts[0] != "company" && parts[0] != "careers" {
			return parts[0]
		}
	}
	return ""
}

func ashbyRef(raw string) (board, id string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || !strings.Contains(strings.ToLower(u.Host), "ashbyhq.com") {
		return "", "", false
	}
	m := ashbyPath.FindStringSubmatch(u.Path)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func leverRef(raw string) (site, id string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || !strings.Contains(strings.ToLower(u.Host), "lever.co") {
		return "", "", false
	}
	m := leverPath.FindStringSubmatch(u.Path)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

var workdayPath = regexp.MustCompile(`(?i)^/(?:[a-z]{2}-[A-Z]{2}/)?([^/]+)/(job/.+)$`)

// workdayRef parses a Workday-hosted career site URL, e.g.
// https://databricks.wd1.myworkdayjobs.com/en-US/databricks/job/San-Francisco-CA/Software-Engineer_R12345
// tenant is the first host label; site and jobPath come from the URL path.
func workdayRef(raw string) (host, tenant, site, jobPath string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || !strings.Contains(strings.ToLower(u.Host), "myworkdayjobs.com") {
		return "", "", "", "", false
	}
	m := workdayPath.FindStringSubmatch(u.Path)
	if m == nil {
		return "", "", "", "", false
	}
	host = strings.ToLower(u.Host)
	tenant = strings.SplitN(host, ".", 2)[0]
	if tenant == "" {
		return "", "", "", "", false
	}
	return host, tenant, m[1], m[2], true
}

func isLinkedIn(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	return strings.Contains(host, "linkedin.com") && strings.Contains(u.Path, "/jobs/")
}

func redirectedToBoardRoot(original, final string) bool {
	if original == "" || final == "" || original == final {
		return false
	}
	from, err1 := url.Parse(original)
	to, err2 := url.Parse(final)
	if err1 != nil || err2 != nil {
		return false
	}
	if !strings.EqualFold(from.Host, to.Host) {
		return false
	}
	id := trailingID(from.Path)
	if id == "" {
		return false
	}
	toPath := strings.Trim(to.Path, "/")
	fromPath := strings.Trim(from.Path, "/")
	if toPath == "" || toPath == "jobs" || !strings.Contains(to.Path, id) && len(toPath) < len(fromPath) {
		return true
	}
	return false
}

func trailingID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if len(last) >= 6 {
		return last
	}
	return ""
}

// FormQuestion is one field discovered on an application form.
type FormQuestion struct {
	Prompt   string
	Required bool
}

// FetchForm loads application questions when the ATS exposes them.
// An empty slice with a nil error means the posting has no published questions.
func (c *Client) FetchForm(ctx context.Context, raw string) ([]FormQuestion, []byte, error) {
	board, id, ok := greenhouseRef(raw)
	if !ok {
		return nil, nil, nil
	}
	api := "https://boards-api.greenhouse.io/v1/boards/" + url.PathEscape(board) + "/jobs/" + url.PathEscape(id) + "?questions=true"
	status, _, body, err := c.get(ctx, api)
	if err != nil || status < 200 || status >= 300 {
		return nil, body, err
	}
	var payload struct {
		Questions []struct {
			Label    string `json:"label"`
			Required bool   `json:"required"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, body, nil
	}
	out := make([]FormQuestion, 0, len(payload.Questions))
	for _, q := range payload.Questions {
		label := strings.TrimSpace(q.Label)
		if label == "" {
			continue
		}
		out = append(out, FormQuestion{Prompt: label, Required: q.Required})
	}
	return out, body, nil
}
