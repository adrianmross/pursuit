// Package httpapi is the REST adapter over service.Service.
package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/pursuit/pursuit/internal/dashboard"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/impex"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/spec"
	"github.com/pursuit/pursuit/internal/sqlstore"
	"github.com/pursuit/pursuit/internal/version"
)

type Server struct {
	Svc   *service.Service
	Log   *slog.Logger
	CORS  []string
	Ready atomic.Bool
	// TrustProxyAuth is "vercel" only when a platform in front of this
	// process already authenticates every caller. Empty keeps bearer tokens required.
	TrustProxyAuth string
	// CronSecret overrides CRON_SECRET for the link-check cron. Empty reads the environment.
	CronSecret string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /version", s.version)
	mux.HandleFunc("GET /openapi.json", s.openapi)
	mux.HandleFunc("GET /llms.txt", s.llms)
	mux.HandleFunc("GET /", s.dashboard)

	mux.HandleFunc("GET /v1/me", s.me)
	mux.HandleFunc("POST /v1/tokens", s.createToken)
	mux.HandleFunc("GET /v1/tokens", s.listTokens)
	mux.HandleFunc("DELETE /v1/tokens/{id}", s.revokeToken)

	mux.HandleFunc("GET /v1/companies", s.listCompanies)
	mux.HandleFunc("POST /v1/companies", s.createCompany)
	mux.HandleFunc("GET /v1/companies/{id}", s.getCompany)
	mux.HandleFunc("PATCH /v1/companies/{id}", s.updateCompany)
	mux.HandleFunc("DELETE /v1/companies/{id}", s.deleteCompany)

	mux.HandleFunc("GET /v1/roles", s.listRoles)
	mux.HandleFunc("POST /v1/roles", s.createRole)
	mux.HandleFunc("GET /v1/roles/{id}", s.getRole)
	mux.HandleFunc("PATCH /v1/roles/{id}", s.updateRole)
	mux.HandleFunc("DELETE /v1/roles/{id}", s.deleteRole)

	mux.HandleFunc("GET /v1/applications", s.listApps)
	mux.HandleFunc("GET /v1/applications/by-url", s.appByURL)
	mux.HandleFunc("POST /v1/applications/capture", s.captureApp)
	mux.HandleFunc("POST /v1/applications", s.createApp)
	mux.HandleFunc("GET /v1/applications/{id}", s.getApp)
	mux.HandleFunc("PATCH /v1/applications/{id}", s.updateApp)
	mux.HandleFunc("DELETE /v1/applications/{id}", s.deleteApp)
	mux.HandleFunc("POST /v1/applications/{id}/transition", s.transition)
	mux.HandleFunc("POST /v1/applications/{id}/archive", s.archiveApp)
	mux.HandleFunc("POST /v1/applications/{id}/unarchive", s.unarchiveApp)
	mux.HandleFunc("GET /v1/applications/{id}/events", s.listAppEvents)
	mux.HandleFunc("POST /v1/applications/{id}/events", s.addEvent)
	mux.HandleFunc("POST /v1/applications/{id}/tags", s.attachTag)
	mux.HandleFunc("DELETE /v1/applications/{id}/tags/{tagID}", s.detachTag)
	mux.HandleFunc("POST /v1/applications/{id}/fields/{fieldID}", s.setFieldValue)
	mux.HandleFunc("POST /v1/applications/{id}/prepare", s.prepare)
	mux.HandleFunc("POST /v1/applications/{id}/enqueue", s.enqueue)
	mux.HandleFunc("GET /v1/applications/{id}/artifacts", s.listArtifacts)
	mux.HandleFunc("POST /v1/applications/{id}/artifacts", s.addArtifact)
	mux.HandleFunc("GET /v1/applications/{id}/questions", s.listQuestions)
	mux.HandleFunc("POST /v1/applications/{id}/check-link", s.checkOne)

	mux.HandleFunc("POST /v1/approvals", s.approve)
	mux.HandleFunc("POST /v1/approvals/reject", s.reject)
	mux.HandleFunc("GET /v1/links", s.listLinks)
	mux.HandleFunc("POST /v1/apply", s.applyBatch)
	mux.HandleFunc("POST /v1/queue/claim", s.claim)
	mux.HandleFunc("POST /v1/queue/{id}/heartbeat", s.heartbeat)
	mux.HandleFunc("POST /v1/queue/{id}/complete", s.complete)

	mux.HandleFunc("GET /v1/artifacts/{id}", s.getArtifact)
	mux.HandleFunc("GET /v1/profile", s.profile)
	mux.HandleFunc("GET /v1/standing", s.listStanding)
	mux.HandleFunc("POST /v1/standing", s.setStanding)
	mux.HandleFunc("GET /v1/caps", s.caps)
	mux.HandleFunc("POST /v1/links/check", s.checkAll)
	mux.HandleFunc("GET /v1/cron/links", s.cronLinks)
	mux.HandleFunc("GET /v1/webhooks", s.listHooks)
	mux.HandleFunc("POST /v1/webhooks", s.createHook)
	mux.HandleFunc("DELETE /v1/webhooks/{id}", s.deleteHook)

	mux.HandleFunc("GET /v1/events", s.listEvents)
	mux.HandleFunc("GET /v1/tags", s.listTags)
	mux.HandleFunc("POST /v1/tags", s.createTag)
	mux.HandleFunc("DELETE /v1/tags/{id}", s.deleteTag)
	mux.HandleFunc("GET /v1/fields", s.listFields)
	mux.HandleFunc("POST /v1/fields", s.createField)
	mux.HandleFunc("DELETE /v1/fields/{id}", s.deleteField)
	mux.HandleFunc("GET /v1/audit", s.listAudit)

	mux.HandleFunc("POST /v1/import/csv", s.importCSV)
	mux.HandleFunc("POST /v1/import/json", s.importJSON)
	mux.HandleFunc("POST /v1/import/markdown", s.importMarkdown)
	mux.HandleFunc("POST /v1/import/seed", s.importSeed)
	mux.HandleFunc("GET /v1/export/simplify", s.exportSimplify)
	mux.HandleFunc("GET /v1/export/json", s.exportJSON)

	return s.wrap(mux)
}

func (s *Server) log() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

func (s *Server) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		rec := &statusRec{ResponseWriter: w, code: 200}
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				s.log().Error("panic", "err", rec, "request_id", id)
				writeErr(w, domain.Conflict("internal error", "retry the request").WithDetails(nil))
				// status already maybe written; best effort
				_ = rec
			}
			s.log().Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.code, "ms", time.Since(start).Milliseconds(), "request_id", id)
		}()
		if origin := r.Header.Get("Origin"); origin != "" && s.allowOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Dry-Run, If-Match")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Expose-Headers", "Idempotent-Replayed, X-Request-Id")
			w.Header().Set("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		ctx := r.Context()
		if isCron(r.URL.Path) {
			if !s.cronAuthorized(r) {
				writeErr(w, domain.Unauthorized())
				return
			}
			next.ServeHTTP(rec, r.WithContext(ctx))
			return
		}
		if !isPublic(r.URL.Path) {
			if raw, ok := bearerToken(r.Header.Get("Authorization")); ok {
				actor, err := s.Svc.Authenticate(ctx, raw)
				if err != nil {
					writeErr(w, err)
					return
				}
				ctx = domain.WithActor(ctx, actor)
			} else if s.trustVercel() {
				actor, err := s.proxyOwner(ctx)
				if err != nil {
					writeErr(w, err)
					return
				}
				ctx = domain.WithActor(ctx, actor)
			} else {
				writeErr(w, domain.Unauthorized())
				return
			}
		}
		next.ServeHTTP(rec, r.WithContext(ctx))
	})
}

// isPublic skips auth for anything that isn't a /v1 API call: health/version
// endpoints and every dashboard asset and client-side route (e.g. /app/:id),
// which render the SPA shell and authenticate in the browser via the API.
func isPublic(path string) bool {
	return !strings.HasPrefix(path, "/v1/")
}

func isCron(path string) bool {
	return path == "/v1/cron/links"
}

func bearerToken(header string) (string, bool) {
	header = strings.TrimSpace(header)
	const prefix = "bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	raw := strings.TrimSpace(header[len(prefix):])
	if raw == "" {
		return "", false
	}
	return raw, true
}

// trustVercel is off unless PURSUIT_TRUST_PROXY_AUTH is exactly "vercel".
// That mode is only safe when deployment protection already gates the process.
func (s *Server) trustVercel() bool {
	return strings.EqualFold(strings.TrimSpace(s.TrustProxyAuth), "vercel")
}

func (s *Server) proxyOwner(ctx context.Context) (domain.Actor, error) {
	user, err := s.Svc.DB.FirstUser(ctx)
	if err != nil {
		return domain.Actor{}, err
	}
	return domain.Actor{UserID: user.ID, Name: "vercel", Scopes: []string{"read", "write", "approve"}}, nil
}

func (s *Server) cronSecret() string {
	if strings.TrimSpace(s.CronSecret) != "" {
		return s.CronSecret
	}
	return strings.TrimSpace(os.Getenv("CRON_SECRET"))
}

func (s *Server) cronAuthorized(r *http.Request) bool {
	secret := s.cronSecret()
	if secret == "" {
		return false
	}
	got, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(secret)) == 1
}

func (s *Server) allowOrigin(origin string) bool {
	for _, o := range s.CORS {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" || strings.EqualFold(o, origin) {
			return true
		}
		if strings.HasSuffix(o, "*") {
			prefix := strings.TrimSuffix(o, "*")
			if prefix != "" && strings.HasPrefix(strings.ToLower(origin), strings.ToLower(prefix)) {
				return true
			}
		}
	}
	return false
}

type statusRec struct {
	http.ResponseWriter
	code int
}

func (s *statusRec) WriteHeader(code int) {
	s.code = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if !s.Ready.Load() {
		writeErr(w, &domain.Error{Code: "unavailable", Message: "not ready", Hint: "wait until bootstrap finishes", Status: 503})
		return
	}
	if err := s.Svc.DB.Ping(r.Context()); err != nil {
		writeErr(w, &domain.Error{Code: "unavailable", Message: "database unreachable", Hint: "check PURSUIT_DATABASE_URL", Status: 503})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ready"})
}

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"version": version.Version})
}

func (s *Server) openapi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, _ = w.Write(spec.Document())
}

func (s *Server) llms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write(spec.LLMS)
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/v1/") {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/")
	body, contentType, err := dashboard.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(contentType, "text/html") {
		// style-src needs 'unsafe-inline': Radix positions popovers/menus with
		// inline style attributes (Floating UI), not a stylesheet.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; img-src 'self' data:; font-src 'self'; base-uri 'self'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func query(r *http.Request, allowed ...string) (map[string][]string, error) {
	allow := map[string]bool{}
	for _, k := range allowed {
		allow[k] = true
	}
	var unknown []string
	for k := range r.URL.Query() {
		if !allow[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		return nil, domain.Validation("unknown query parameter %s", strings.Join(unknown, ", ")).WithHint("allowed: " + strings.Join(allowed, ", "))
	}
	return r.URL.Query(), nil
}

func fieldsOf(q map[string][]string) []string {
	if q == nil {
		return nil
	}
	raw := strings.TrimSpace(strings.Join(q["fields"], ","))
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func listQuery(q map[string][]string) (domain.ListQuery, error) {
	var out domain.ListQuery
	if v := first(q, "limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return out, domain.Validation("invalid limit").WithHint("limit is an integer from 1 to 200")
		}
		out.Limit = n
	}
	if v := first(q, "cursor"); v != "" {
		c, err := domain.DecodeCursor(v)
		if err != nil {
			return out, err
		}
		out.Cursor = &c
	}
	out.Sort = first(q, "sort")
	out.Q = first(q, "q")
	out.Company = first(q, "company")
	out.CompanyID = first(q, "company_id")
	out.Fit = first(q, "fit")
	out.Source = first(q, "source")
	out.Tag = first(q, "tag")
	out.ApplicationID = first(q, "application_id")
	out.Actor = first(q, "actor")
	out.Action = first(q, "action")
	out.EntityType = first(q, "entity_type")
	out.EntityID = first(q, "entity_id")
	out.AfterID = first(q, "after")
	for _, raw := range q["status"] {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			st, err := domain.ParseStatus(part)
			if err != nil {
				return out, err
			}
			out.Status = append(out.Status, st)
		}
	}
	switch strings.ToLower(first(q, "stale")) {
	case "", "0", "false":
	case "1", "true":
		out.Stale = true
	default:
		return out, domain.Validation("invalid stale").WithHint("stale is true or false")
	}
	switch strings.ToLower(first(q, "archived")) {
	case "", "0", "false":
	case "1", "true":
		out.Archived = true
	default:
		return out, domain.Validation("invalid archived").WithHint("archived is true or false")
	}
	return out, nil
}

func first(q map[string][]string, k string) string {
	if len(q[k]) == 0 {
		return ""
	}
	return q[k][0]
}

func (s *Server) read(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	limit := int64(1 << 20)
	if strings.Contains(r.URL.Path, "/import/") || strings.Contains(r.URL.Path, "/artifacts") {
		limit = 10 << 20
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		writeErr(w, domain.Validation("read body: %s", err.Error()))
		return nil, false
	}
	if int64(len(b)) > limit {
		writeErr(w, domain.Validation("request body is too large").WithHint("JSON bodies are limited to 1MB and imports to 10MB"))
		return nil, false
	}
	return b, true
}

func decode(body []byte, dst any) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return domain.Validation("json body is required").WithHint("see /openapi.json for the request shape")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return domain.Validation("invalid json: %s", err.Error()).WithHint("see /openapi.json for the request shape")
	}
	return nil
}

func dryRun(r *http.Request) bool {
	if strings.EqualFold(r.Header.Get("X-Dry-Run"), "true") || r.Header.Get("X-Dry-Run") == "1" {
		return true
	}
	v := strings.ToLower(r.URL.Query().Get("dry_run"))
	return v == "1" || v == "true"
}

var errBusy = errors.New("idempotency in progress")

func (s *Server) mutate(w http.ResponseWriter, r *http.Request, status int, allowed []string, body []byte, fn func(ctx context.Context) (any, error)) {
	fields := fieldsOf(r.URL.Query())
	if err := domain.ValidateFields(fields, allowed); err != nil && len(fields) > 0 {
		if len(allowed) == 0 {
			writeErr(w, domain.Validation("fields are not supported on this response"))
			return
		}
		writeErr(w, err)
		return
	}
	ctx := r.Context()
	if dryRun(r) {
		obj, err := fn(sqlstore.WithDryRun(ctx))
		if err != nil {
			writeErr(w, err)
			return
		}
		payload, err := project(obj, fields)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"dry_run": true, "hint": "no changes were written", "data": payload})
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		obj, err := fn(ctx)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeProjected(w, status, obj, fields)
		return
	}
	if len(key) > 200 {
		writeErr(w, domain.Validation("idempotency key is too long").WithHint("keep Idempotency-Key under 200 characters"))
		return
	}
	actor, _ := domain.ActorFrom(ctx)
	hash := requestHash(r.Method, r.URL.Path, r.URL.RawQuery, body)
	var replay []byte
	var replayCode int
	var replayed bool
	for attempt := 0; attempt < 8; attempt++ {
		err := s.Svc.DB.WithTx(ctx, func(ctx context.Context) error {
			rec, found, err := s.Svc.DB.GetIdempotency(ctx, actor.TokenID, key)
			if err != nil {
				return err
			}
			if found {
				if rec.RequestHash != hash {
					return domain.IdempotencyMismatch()
				}
				if rec.StatusCode == 0 {
					return errBusy
				}
				replay = []byte(rec.ResponseBody)
				replayCode = rec.StatusCode
				replayed = true
				return nil
			}
			now := domain.FormatTime(time.Now().UTC())
			if err := s.Svc.DB.InsertIdempotency(ctx, sqlstore.IdemRecord{
				TokenID: actor.TokenID, UserID: actor.UserID, Key: key, Method: r.Method,
				Path: r.URL.Path, RequestHash: hash, StatusCode: 0, ResponseBody: "", CreatedAt: now,
			}); err != nil {
				return err
			}
			obj, err := fn(ctx)
			if err != nil {
				return err
			}
			b, err := marshal(obj)
			if err != nil {
				return err
			}
			b, err = projectBytes(b, fields)
			if err != nil {
				return err
			}
			if err := s.Svc.DB.FinishIdempotency(ctx, actor.TokenID, key, status, string(b)); err != nil {
				return err
			}
			replay = b
			replayCode = status
			replayed = false
			return nil
		})
		if errors.Is(err, sqlstore.ErrIdempotencyRace) || errors.Is(err, errBusy) {
			time.Sleep(time.Duration(attempt+1) * 20 * time.Millisecond)
			continue
		}
		if err != nil {
			writeErr(w, err)
			return
		}
		if replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeRaw(w, replayCode, replay)
		return
	}
	writeErr(w, domain.Conflict("idempotency key is in progress", "retry this request shortly"))
}

func requestHash(method, path, rawQuery string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write([]byte(rawQuery))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func writeProjected(w http.ResponseWriter, status int, v any, fields []string) {
	b, err := marshal(v)
	if err != nil {
		writeErr(w, err)
		return
	}
	b, err = projectBytes(b, fields)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeRaw(w, status, b)
}

func project(v any, fields []string) (any, error) {
	b, err := marshal(v)
	if err != nil {
		return nil, err
	}
	b, err = projectBytes(b, fields)
	if err != nil {
		return nil, err
	}
	var out any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func projectBytes(b []byte, fields []string) ([]byte, error) {
	if len(fields) == 0 {
		return b, nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	v = projectValue(v, fields)
	return marshal(v)
}

func projectValue(v any, fields []string) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	if data, ok := m["data"].([]any); ok {
		if _, has := m["page"]; has {
			for i, item := range data {
				if im, ok := item.(map[string]any); ok {
					data[i] = pick(im, fields)
				}
			}
			m["data"] = data
			return m
		}
	}
	return pick(m, fields)
}

func pick(m map[string]any, fields []string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		if v, ok := m[f]; ok {
			out[f] = v
		}
	}
	return out
}

func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := marshal(v)
	if err != nil {
		http.Error(w, `{"error":{"code":"internal","message":"encode failed"}}`, 500)
		return
	}
	writeRaw(w, status, b)
}

func writeRaw(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(b, '\n'))
}

func writeErr(w http.ResponseWriter, err error) {
	de, ok := domain.As(err)
	if !ok {
		de = &domain.Error{Code: "internal", Message: "internal error", Hint: "check server logs", Status: 500}
	}
	if de.Status == 0 {
		de.Status = 500
	}
	writeJSON(w, de.Status, map[string]any{"error": de})
}

func versionParam(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("version"))
	if raw == "" {
		raw = strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), `"`)
	}
	if raw == "" {
		return 0, domain.Validation("version is required").WithHint("pass version as a query parameter or If-Match header")
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, domain.Validation("invalid version").WithHint("version is a positive integer from the latest get")
	}
	return n, nil
}

func csvBody(r *http.Request, body []byte) ([]byte, error) {
	ct := r.Header.Get("Content-Type")
	media, params, err := mime.ParseMediaType(ct)
	if err != nil || !strings.HasPrefix(media, "multipart/") {
		return body, nil
	}
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, domain.Validation("invalid multipart body").WithHint("send text/csv or a multipart file field named file")
		}
		if part.FormName() == "file" || part.FileName() != "" {
			b, err := io.ReadAll(io.LimitReader(part, 10<<20))
			if err != nil {
				return nil, err
			}
			return b, nil
		}
	}
	return nil, domain.Validation("multipart body is missing file").WithHint("use a file field named file")
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "fields"); err != nil {
		writeErr(w, err)
		return
	}
	me, err := s.Svc.Me(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProjected(w, 200, me, fieldsOf(r.URL.Query()))
}

func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run", "fields"); err != nil {
		writeErr(w, err)
		return
	}
	body, ok := s.read(w, r)
	if !ok {
		return
	}
	s.mutate(w, r, 201, nil, body, func(ctx context.Context) (any, error) {
		var in service.TokenInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.CreateToken(ctx, in)
	})
}

func (s *Server) listTokens(w http.ResponseWriter, r *http.Request) {
	s.page(w, r, []string{"limit", "cursor", "sort", "fields"}, domain.TokenFields, func(ctx context.Context, q domain.ListQuery) (any, error) {
		return s.Svc.ListTokens(ctx, q)
	})
}

func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	id := r.PathValue("id")
	s.mutate(w, r, 200, nil, nil, func(ctx context.Context) (any, error) {
		return s.Svc.RevokeToken(ctx, id)
	})
}

func (s *Server) page(w http.ResponseWriter, r *http.Request, allow []string, fieldAllow []string, load func(context.Context, domain.ListQuery) (any, error)) {
	q, err := query(r, allow...)
	if err != nil {
		writeErr(w, err)
		return
	}
	lq, err := listQuery(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	fields := fieldsOf(q)
	if err := domain.ValidateFields(fields, fieldAllow); err != nil {
		writeErr(w, err)
		return
	}
	obj, err := load(r.Context(), lq)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProjected(w, 200, obj, fields)
}

func (s *Server) listCompanies(w http.ResponseWriter, r *http.Request) {
	s.page(w, r, []string{"limit", "cursor", "sort", "q", "fields"}, domain.CompanyFields, func(ctx context.Context, q domain.ListQuery) (any, error) {
		return s.Svc.ListCompanies(ctx, q)
	})
}

func (s *Server) createCompany(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 201, domain.CompanyFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.CompanyInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.CreateCompany(ctx, in)
	})
}

func (s *Server) getCompany(w http.ResponseWriter, r *http.Request) {
	s.getOne(w, r, domain.CompanyFields, func(ctx context.Context, id string) (any, error) {
		return s.Svc.GetCompany(ctx, id)
	})
}

func (s *Server) updateCompany(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 200, domain.CompanyFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.CompanyPatch
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.UpdateCompany(ctx, id, in)
	})
}

func (s *Server) deleteCompany(w http.ResponseWriter, r *http.Request) {
	s.deleteOne(w, r, func(ctx context.Context, id string, version int) (any, error) {
		return s.Svc.DeleteCompany(ctx, id, version)
	})
}

func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) {
	s.page(w, r, []string{"limit", "cursor", "sort", "q", "company", "company_id", "fit", "source", "fields"}, domain.RoleFields, func(ctx context.Context, q domain.ListQuery) (any, error) {
		return s.Svc.ListRoles(ctx, q)
	})
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 201, domain.RoleFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.RoleInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.CreateRole(ctx, in)
	})
}

func (s *Server) getRole(w http.ResponseWriter, r *http.Request) {
	s.getOne(w, r, domain.RoleFields, func(ctx context.Context, id string) (any, error) {
		return s.Svc.GetRole(ctx, id)
	})
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 200, domain.RoleFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.RolePatch
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.UpdateRole(ctx, id, in)
	})
}

func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request) {
	s.deleteOne(w, r, func(ctx context.Context, id string, version int) (any, error) {
		return s.Svc.DeleteRole(ctx, id, version)
	})
}

func (s *Server) profile(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r); err != nil {
		writeErr(w, err)
		return
	}
	view, err := s.Svc.ApplicantProfile(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, view)
}

func (s *Server) appByURL(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "url"); err != nil {
		writeErr(w, err)
		return
	}
	app, err := s.Svc.ApplicationByPostingURL(r.Context(), r.URL.Query().Get("url"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, app)
}

func (s *Server) captureApp(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in service.ApplicationInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.CapturePosting(ctx, in)
	})
}

func (s *Server) listApps(w http.ResponseWriter, r *http.Request) {
	s.page(w, r, []string{"limit", "cursor", "sort", "q", "status", "company", "company_id", "fit", "source", "tag", "stale", "archived", "fields"}, domain.AppFields, func(ctx context.Context, q domain.ListQuery) (any, error) {
		return s.Svc.ListApplications(ctx, q)
	})
}

func (s *Server) createApp(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 201, domain.AppFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.ApplicationInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.CreateApplication(ctx, in)
	})
}

func (s *Server) getApp(w http.ResponseWriter, r *http.Request) {
	s.getOne(w, r, domain.AppFields, func(ctx context.Context, id string) (any, error) {
		return s.Svc.GetApplication(ctx, id)
	})
}

func (s *Server) updateApp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 200, domain.AppFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.ApplicationPatch
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.UpdateApplication(ctx, id, in)
	})
}

func (s *Server) deleteApp(w http.ResponseWriter, r *http.Request) {
	s.deleteOne(w, r, func(ctx context.Context, id string, version int) (any, error) {
		return s.Svc.DeleteApplication(ctx, id, version)
	})
}

func (s *Server) transition(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 200, domain.AppFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.TransitionInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.Transition(ctx, id, in)
	})
}

type archiveInput struct {
	Version int `json:"version"`
}

func (s *Server) archiveApp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 200, domain.AppFields, func(ctx context.Context, body []byte) (any, error) {
		var in archiveInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.ArchiveApplication(ctx, id, in.Version)
	})
}

func (s *Server) unarchiveApp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 200, domain.AppFields, func(ctx context.Context, body []byte) (any, error) {
		var in archiveInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.UnarchiveApplication(ctx, id, in.Version)
	})
}

func (s *Server) listAppEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.page(w, r, []string{"limit", "cursor", "sort", "fields"}, domain.EventFields, func(ctx context.Context, q domain.ListQuery) (any, error) {
		q.ApplicationID = id
		return s.Svc.ListEvents(ctx, q)
	})
}

func (s *Server) addEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 201, domain.EventFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.EventInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.AddEvent(ctx, id, in)
	})
}

func (s *Server) attachTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 200, domain.AppFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.AttachTagInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.AttachTag(ctx, id, in)
	})
}

func (s *Server) detachTag(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	id, tagID := r.PathValue("id"), r.PathValue("tagID")
	s.mutate(w, r, 200, domain.AppFields, nil, func(ctx context.Context) (any, error) {
		return s.Svc.DetachTag(ctx, id, tagID)
	})
}

func (s *Server) listTags(w http.ResponseWriter, r *http.Request) {
	s.page(w, r, []string{"limit", "cursor", "sort", "q", "fields"}, domain.TagFields, func(ctx context.Context, q domain.ListQuery) (any, error) {
		return s.Svc.ListTags(ctx, q)
	})
}

func (s *Server) createTag(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 201, domain.TagFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.TagInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.CreateTag(ctx, in)
	})
}

func (s *Server) deleteTag(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	id := r.PathValue("id")
	s.mutate(w, r, 200, nil, nil, func(ctx context.Context) (any, error) {
		return s.Svc.DeleteTag(ctx, id)
	})
}

func (s *Server) listFields(w http.ResponseWriter, r *http.Request) {
	s.page(w, r, []string{"limit", "cursor", "sort", "q", "fields"}, domain.CustomFieldFields, func(ctx context.Context, q domain.ListQuery) (any, error) {
		return s.Svc.ListFields(ctx, q)
	})
}

func (s *Server) createField(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 201, domain.CustomFieldFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.FieldInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.CreateField(ctx, in)
	})
}

func (s *Server) deleteField(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	id := r.PathValue("id")
	s.mutate(w, r, 200, nil, nil, func(ctx context.Context) (any, error) {
		return s.Svc.DeleteField(ctx, id)
	})
}

func (s *Server) setFieldValue(w http.ResponseWriter, r *http.Request) {
	id, fieldID := r.PathValue("id"), r.PathValue("fieldID")
	s.jsonMut(w, r, 200, domain.AppFields, func(ctx context.Context, body []byte) (any, error) {
		var in service.SetFieldValueInput
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.SetFieldValue(ctx, id, fieldID, in)
	})
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	q, err := query(r, "limit", "cursor", "sort", "application_id", "after", "wait_ms", "fields")
	if err != nil {
		writeErr(w, err)
		return
	}
	lq, err := listQuery(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	fields := fieldsOf(q)
	if err := domain.ValidateFields(fields, domain.EventFields); err != nil {
		writeErr(w, err)
		return
	}
	wait := time.Duration(0)
	if raw := first(q, "wait_ms"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeErr(w, domain.Validation("invalid wait_ms").WithHint("wait_ms is a non-negative integer up to 60000"))
			return
		}
		wait = time.Duration(n) * time.Millisecond
	}
	page, err := s.Svc.ListEventsFollow(r.Context(), lq, wait)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProjected(w, 200, page, fields)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	s.page(w, r, []string{"limit", "cursor", "sort", "actor", "action", "entity_type", "entity_id", "fields"}, domain.AuditFields, func(ctx context.Context, q domain.ListQuery) (any, error) {
		return s.Svc.ListAudit(ctx, q)
	})
}

func (s *Server) importCSV(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	body, ok := s.read(w, r)
	if !ok {
		return
	}
	payload, err := csvBody(r, body)
	if err != nil {
		writeErr(w, err)
		return
	}
	s.mutate(w, r, 200, nil, payload, func(ctx context.Context) (any, error) {
		return s.Svc.ImportCSV(ctx, bytes.NewReader(payload))
	})
}

func (s *Server) importJSON(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	body, ok := s.read(w, r)
	if !ok {
		return
	}
	s.mutate(w, r, 200, nil, body, func(ctx context.Context) (any, error) {
		rows, err := impex.ParsePostingsJSON(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		return s.Svc.ImportPostings(ctx, rows)
	})
}

func (s *Server) importMarkdown(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	body, ok := s.read(w, r)
	if !ok {
		return
	}
	s.mutate(w, r, 200, nil, body, func(ctx context.Context) (any, error) {
		rows, err := impex.ParseMarkdownTables(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		return s.Svc.ImportPostings(ctx, rows)
	})
}

func (s *Server) importSeed(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	s.mutate(w, r, 200, nil, []byte("seed"), func(ctx context.Context) (any, error) {
		return s.Svc.ImportSeed(ctx)
	})
}

func (s *Server) exportSimplify(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r); err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.Svc.ExportSimplify(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write(b)
}

func (s *Server) exportJSON(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "fields"); err != nil {
		writeErr(w, err)
		return
	}
	doc, err := s.Svc.ExportJSON(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProjected(w, 200, doc, nil)
}

func (s *Server) prepare(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run", "worker"); err != nil {
		writeErr(w, err)
		return
	}
	id := r.PathValue("id")
	worker := r.URL.Query().Get("worker")
	s.mutate(w, r, 200, nil, []byte(id+worker), func(ctx context.Context) (any, error) {
		return s.Svc.Prepare(ctx, id, worker)
	})
}

func (s *Server) enqueue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			Kind string `json:"kind"`
		}
		if len(bytes.TrimSpace(body)) > 0 {
			if err := decode(body, &in); err != nil {
				return nil, err
			}
		}
		if in.Kind == "" {
			in.Kind = "prepare"
		}
		return s.Svc.Enqueue(ctx, id, in.Kind)
	})
}

func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			IDs  []string `json:"ids"`
			Note string   `json:"note"`
		}
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.Approve(ctx, in.IDs, in.Note)
	})
}

func (s *Server) reject(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			IDs  []string `json:"ids"`
			Note string   `json:"note"`
		}
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.Reject(ctx, in.IDs, in.Note)
	})
}

func (s *Server) listLinks(w http.ResponseWriter, r *http.Request) {
	raw, err := query(r, "limit", "cursor", "sort", "status", "changed", "attention", "fields")
	if err != nil {
		writeErr(w, err)
		return
	}
	base := map[string][]string{}
	for _, k := range []string{"limit", "cursor", "sort"} {
		if v, ok := raw[k]; ok {
			base[k] = v
		}
	}
	lq, err := listQuery(base)
	if err != nil {
		writeErr(w, err)
		return
	}
	status := strings.ToLower(strings.TrimSpace(first(raw, "status")))
	switch status {
	case "", domain.LinkLive, domain.LinkClosed, domain.LinkUnknown:
		lq.LinkStatus = status
	default:
		writeErr(w, domain.Validation("invalid link status %q", first(raw, "status")).WithHint("status is live, closed, or unknown"))
		return
	}
	attention, err := parseBoolFlag(first(raw, "attention"), "attention")
	if err != nil {
		writeErr(w, err)
		return
	}
	var changed *bool
	if rawChanged := first(raw, "changed"); rawChanged != "" {
		v, err := parseBoolFlag(rawChanged, "changed")
		if err != nil {
			writeErr(w, err)
			return
		}
		changed = &v
	}
	if attention && (status != "" || changed != nil) {
		writeErr(w, domain.Validation("attention cannot be combined with status or changed").WithHint("pass attention=true, or filter by status and changed"))
		return
	}
	lq.Attention = attention
	lq.Changed = changed
	fields := fieldsOf(raw)
	if err := domain.ValidateFields(fields, domain.LinkFields); err != nil {
		writeErr(w, err)
		return
	}
	page, err := s.Svc.ListLinks(r.Context(), lq)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProjected(w, 200, page, fields)
}

func parseBoolFlag(v, name string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false":
		return false, nil
	case "1", "true":
		return true, nil
	default:
		return false, domain.Validation("invalid %s", name).WithHint(name + " is true or false")
	}
}

func (s *Server) applyBatch(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			Prepare bool   `json:"prepare"`
			Async   bool   `json:"async"`
			Status  string `json:"status"`
			Company string `json:"company"`
			Stale   bool   `json:"stale"`
			Q       string `json:"q"`
		}
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		mode := "async"
		if in.Prepare {
			mode = "prepare"
		}
		if in.Prepare && in.Async {
			return nil, domain.Validation("choose prepare or async").WithHint("pass only one of prepare or async")
		}
		q := domain.ListQuery{Company: in.Company, Q: in.Q, Stale: in.Stale}
		if in.Status != "" {
			st, err := domain.ParseStatus(in.Status)
			if err != nil {
				return nil, err
			}
			q.Status = []string{st}
		}
		return s.Svc.ApplyBatch(ctx, q, mode)
	})
}

func (s *Server) claim(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			Worker string `json:"worker"`
		}
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.Claim(ctx, in.Worker)
	})
}

func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			Worker string `json:"worker"`
		}
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.Heartbeat(ctx, id, in.Worker)
	})
}

func (s *Server) complete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			Worker string `json:"worker"`
			Status string `json:"status"`
			Note   string `json:"note"`
		}
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.CompleteWork(ctx, id, in.Worker, in.Status, in.Note)
	})
}

func (s *Server) listArtifacts(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r); err != nil {
		writeErr(w, err)
		return
	}
	rows, err := s.Svc.ListArtifacts(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": rows})
}

func (s *Server) addArtifact(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	body, ok := s.read(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	ct := r.Header.Get("Content-Type")
	s.mutate(w, r, 201, nil, body, func(ctx context.Context) (any, error) {
		media, params, err := mime.ParseMediaType(ct)
		if err == nil && strings.HasPrefix(media, "multipart/") {
			mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
			var kind, filename, contentType string
			var payload []byte
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					return nil, domain.Validation("invalid multipart body")
				}
				b, _ := io.ReadAll(io.LimitReader(part, 6<<20))
				switch part.FormName() {
				case "kind":
					kind = string(b)
				case "file":
					filename = part.FileName()
					contentType = part.Header.Get("Content-Type")
					payload = b
				}
			}
			return s.Svc.AddArtifact(ctx, id, kind, filename, contentType, payload)
		}
		var in struct {
			Kind        string `json:"kind"`
			Filename    string `json:"filename"`
			ContentType string `json:"content_type"`
			Body        string `json:"body"`
		}
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.AddArtifact(ctx, id, in.Kind, in.Filename, in.ContentType, []byte(in.Body))
	})
}

func (s *Server) getArtifact(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r); err != nil {
		writeErr(w, err)
		return
	}
	art, err := s.Svc.GetArtifact(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"id": art.ID, "application_id": art.ApplicationID, "kind": art.Kind,
		"filename": art.Filename, "content_type": art.ContentType, "body": art.Body, "created_at": art.CreatedAt,
	})
}

func (s *Server) listQuestions(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r); err != nil {
		writeErr(w, err)
		return
	}
	rows, err := s.Svc.ListQuestions(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if rows == nil {
		rows = []service.QuestionView{}
	}
	writeJSON(w, 200, map[string]any{"data": rows})
}

func (s *Server) listStanding(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r); err != nil {
		writeErr(w, err)
		return
	}
	rows, err := s.Svc.ListStanding(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": rows})
}

func (s *Server) setStanding(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			Key    string `json:"key"`
			Answer string `json:"answer"`
		}
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.SetStanding(ctx, in.Key, in.Answer)
	})
}

func (s *Server) caps(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r); err != nil {
		writeErr(w, err)
		return
	}
	rows, err := s.Svc.ListCaps(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": rows})
}

func (s *Server) checkOne(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	id := r.PathValue("id")
	s.mutate(w, r, 200, nil, []byte(id), func(ctx context.Context) (any, error) {
		return s.Svc.CheckApplicationLink(ctx, id)
	})
}

func (s *Server) cronLinks(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r); err != nil {
		writeErr(w, err)
		return
	}
	report, err := s.Svc.SweepAsOwner(r.Context(), "vercel-cron")
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, report)
}

func (s *Server) checkAll(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 200, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			Owner string `json:"owner"`
		}
		if len(bytes.TrimSpace(body)) > 0 {
			if err := decode(body, &in); err != nil {
				return nil, err
			}
		}
		if in.Owner == "" {
			if a, ok := domain.ActorFrom(ctx); ok {
				in.Owner = a.Name
			}
		}
		return s.Svc.SweepOnce(ctx, in.Owner)
	})
}

func (s *Server) listHooks(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r); err != nil {
		writeErr(w, err)
		return
	}
	rows, err := s.Svc.ListWebhooks(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": rows})
}

func (s *Server) createHook(w http.ResponseWriter, r *http.Request) {
	s.jsonMut(w, r, 201, nil, func(ctx context.Context, body []byte) (any, error) {
		var in struct {
			URL    string `json:"url"`
			Secret string `json:"secret"`
		}
		if err := decode(body, &in); err != nil {
			return nil, err
		}
		return s.Svc.CreateWebhook(ctx, in.URL, in.Secret)
	})
}

func (s *Server) deleteHook(w http.ResponseWriter, r *http.Request) {
	if _, err := query(r, "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	id := r.PathValue("id")
	s.mutate(w, r, 200, nil, nil, func(ctx context.Context) (any, error) {
		return s.Svc.DeleteWebhook(ctx, id)
	})
}

func (s *Server) jsonMut(w http.ResponseWriter, r *http.Request, status int, allowed []string, fn func(context.Context, []byte) (any, error)) {
	if _, err := query(r, "dry_run", "fields"); err != nil {
		writeErr(w, err)
		return
	}
	body, ok := s.read(w, r)
	if !ok {
		return
	}
	s.mutate(w, r, status, allowed, body, func(ctx context.Context) (any, error) {
		return fn(ctx, body)
	})
}

func (s *Server) getOne(w http.ResponseWriter, r *http.Request, allowed []string, load func(context.Context, string) (any, error)) {
	if _, err := query(r, "fields"); err != nil {
		writeErr(w, err)
		return
	}
	fields := fieldsOf(r.URL.Query())
	if err := domain.ValidateFields(fields, allowed); err != nil {
		writeErr(w, err)
		return
	}
	obj, err := load(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeProjected(w, 200, obj, fields)
}

func (s *Server) deleteOne(w http.ResponseWriter, r *http.Request, fn func(context.Context, string, int) (any, error)) {
	if _, err := query(r, "version", "dry_run"); err != nil {
		writeErr(w, err)
		return
	}
	version, err := versionParam(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	id := r.PathValue("id")
	s.mutate(w, r, 200, nil, []byte(strconv.Itoa(version)), func(ctx context.Context) (any, error) {
		return fn(ctx, id, version)
	})
}
