// Package domain holds Pursuit entities, validation, and values shared by every transport.
package domain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TimeLayout is the fixed-width UTC timestamp stored in the database.
// Lexicographic order matches chronological order.
const TimeLayout = "2006-01-02T15:04:05.000000000Z"

func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(TimeLayout)
}

func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	for _, layout := range []string{TimeLayout, time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q", s)
}

func ParseDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return "", Validation("invalid date %q", s).WithHint("use YYYY-MM-DD")
	}
	return t.Format("2006-01-02"), nil
}

// Status pipeline. Apply work moves to_apply → preparing → awaiting_review →
// approved → submitting → submitted, and can stop in needs_input or failed.
// Historical tracker values (applied, screening, …) stay valid.
const (
	StatusSaved          = "saved"
	StatusToApply        = "to_apply"
	StatusPreparing      = "preparing"
	StatusAwaitingReview = "awaiting_review"
	StatusApproved       = "approved"
	StatusSubmitting     = "submitting"
	StatusSubmitted      = "submitted"
	StatusNeedsInput     = "needs_input"
	StatusFailed         = "failed"
	StatusInProgress     = "in_progress"
	StatusApplied        = "applied"
	StatusScreening      = "screening"
	StatusInterviewing   = "interviewing"
	StatusOffer          = "offer"
	StatusRejected       = "rejected"
	StatusWithdrawn      = "withdrawn"
	StatusGhosted        = "ghosted"
)

var Statuses = []string{
	StatusSaved, StatusToApply, StatusPreparing, StatusAwaitingReview, StatusApproved,
	StatusSubmitting, StatusSubmitted, StatusNeedsInput, StatusFailed, StatusInProgress,
	StatusApplied, StatusScreening, StatusInterviewing, StatusOffer, StatusRejected,
	StatusWithdrawn, StatusGhosted,
}

func ParseStatus(s string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(s))
	n = strings.ReplaceAll(n, "_", " ")
	n = strings.ReplaceAll(n, "-", " ")
	n = strings.Join(strings.Fields(n), " ")
	switch n {
	case "saved":
		return StatusSaved, nil
	case "to apply":
		return StatusToApply, nil
	case "preparing":
		return StatusPreparing, nil
	case "prepared", "awaiting review":
		return StatusAwaitingReview, nil
	case "approved":
		return StatusApproved, nil
	case "submitting":
		return StatusSubmitting, nil
	case "submitted":
		return StatusSubmitted, nil
	case "needs input":
		return StatusNeedsInput, nil
	case "failed":
		return StatusFailed, nil
	case "in progress":
		return StatusInProgress, nil
	case "applied":
		return StatusApplied, nil
	case "screening":
		return StatusScreening, nil
	case "interviewing":
		return StatusInterviewing, nil
	case "offer":
		return StatusOffer, nil
	case "rejected":
		return StatusRejected, nil
	case "withdrawn":
		return StatusWithdrawn, nil
	case "ghosted":
		return StatusGhosted, nil
	default:
		return "", Validation("invalid status %q", s).WithHint("status must be one of: " + strings.Join(statusLabels(), ", "))
	}
}

func statusLabels() []string {
	out := make([]string, len(Statuses))
	for i, s := range Statuses {
		out[i] = StatusLabel(s)
	}
	return out
}

func StatusLabel(s string) string {
	switch s {
	case StatusSaved:
		return "Saved"
	case StatusToApply:
		return "To apply"
	case StatusPreparing:
		return "Preparing"
	case StatusAwaitingReview:
		return "Awaiting review"
	case StatusApproved:
		return "Approved"
	case StatusSubmitting:
		return "Submitting"
	case StatusSubmitted:
		return "Submitted"
	case StatusNeedsInput:
		return "Needs input"
	case StatusFailed:
		return "Failed"
	case StatusInProgress:
		return "In progress"
	case StatusApplied:
		return "Applied"
	case StatusScreening:
		return "Screening"
	case StatusInterviewing:
		return "Interviewing"
	case StatusOffer:
		return "Offer"
	case StatusRejected:
		return "Rejected"
	case StatusWithdrawn:
		return "Withdrawn"
	case StatusGhosted:
		return "Ghosted"
	default:
		return s
	}
}

// SimplifyStatus maps a Pursuit status onto the values used by Simplify's importer.
func SimplifyStatus(status string) string {
	switch status {
	case StatusSaved, StatusToApply, StatusInProgress, StatusPreparing, StatusAwaitingReview,
		StatusApproved, StatusNeedsInput, StatusFailed:
		return "Saved"
	case StatusApplied, StatusSubmitting, StatusSubmitted:
		return "Applied"
	case StatusScreening:
		return "Screening"
	case StatusInterviewing:
		return "Interviewing"
	case StatusOffer:
		return "Offer"
	case StatusRejected:
		return "Rejected"
	case StatusWithdrawn:
		return "Withdrawn"
	case StatusGhosted:
		return "Ghosted"
	default:
		return StatusLabel(status)
	}
}

// CountsAsSubmitted reports statuses that consume a "submitted" company cap slot.
func CountsAsSubmitted(status string) bool {
	switch status {
	case StatusApplied, StatusSubmitted, StatusSubmitting, StatusScreening, StatusInterviewing, StatusOffer:
		return true
	default:
		return false
	}
}

// IsTerminalStatus reports outcomes that no longer count as an active application.
func IsTerminalStatus(status string) bool {
	switch status {
	case StatusRejected, StatusWithdrawn, StatusGhosted, StatusFailed:
		return true
	default:
		return false
	}
}

const (
	LinkLive    = "live"
	LinkClosed  = "closed"
	LinkUnknown = "unknown"
)

func ParseFit(s string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(s))
	switch n {
	case "", "strong", "reach", "maybe":
		return n, nil
	default:
		return "", Validation("invalid fit %q", s).WithHint("fit is strong, reach, or maybe")
	}
}

func FitLabel(fit string) string {
	switch fit {
	case "strong":
		return "Strong"
	case "reach":
		return "Reach"
	case "maybe":
		return "Maybe"
	case "":
		return ""
	default:
		return strings.ToUpper(fit[:1]) + fit[1:]
	}
}

const (
	SourceGreenhouse  = "greenhouse"
	SourceAshby       = "ashby"
	SourceLever       = "lever"
	SourceWorkday     = "workday"
	SourceCompanySite = "company_site"
	SourceOther       = "other"
)

func ParseSource(s string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(s))
	n = strings.ReplaceAll(n, "-", "_")
	n = strings.ReplaceAll(n, " ", "_")
	switch n {
	case "", SourceGreenhouse, SourceAshby, SourceLever, SourceWorkday, SourceCompanySite, SourceOther:
		return n, nil
	case "company", "companysite", "company_website":
		return SourceCompanySite, nil
	default:
		return "", Validation("invalid source %q", s).WithHint("source is greenhouse, ashby, lever, workday, company_site, or other")
	}
}

func InferSource(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return SourceOther
	}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, "greenhouse.io") || u.Query().Get("gh_jid") != "" {
		return SourceGreenhouse
	}
	if strings.Contains(host, "ashbyhq.com") {
		return SourceAshby
	}
	if strings.Contains(host, "lever.co") {
		return SourceLever
	}
	if strings.Contains(host, "myworkdayjobs.com") || strings.Contains(host, "workday.com") {
		return SourceWorkday
	}
	return SourceCompanySite
}

func ParseURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", Validation("invalid url %q", s).WithHint("use an absolute http or https url")
	}
	return s, nil
}

// CanonicalPostingURL drops the fragment and common tracking parameters so the
// same posting saved from a browser matches the URL already in the tracker.
// A URL with nothing to drop is returned unchanged.
func CanonicalPostingURL(s string) (string, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", Validation("invalid url %q", s).WithHint("use an absolute http or https url")
	}
	changed := u.Fragment != ""
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "utm_") || lk == "trk" || lk == "trackingid" || lk == "refid" || lk == "lipi" {
			q.Del(k)
			changed = true
		}
	}
	if !changed {
		return raw, nil
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func Origin(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func ParseEmail(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if !strings.Contains(s, "@") || strings.ContainsAny(s, " \t") {
		return "", Validation("invalid email %q", s).WithHint("contact_email should look like name@example.com")
	}
	return s, nil
}

var moneyRe = regexp.MustCompile(`\$(\d+(?:\.\d+)?)([Kk])?`)

// ParseSalary extracts min and max annual compensation in cents from a free-text range.
// "Not listed" and unparseable text return nil bounds; callers keep the raw string.
func ParseSalary(raw string) (min, max *int64) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "not listed") || strings.EqualFold(raw, "n/a") {
		return nil, nil
	}
	matches := moneyRe.FindAllStringSubmatch(raw, -1)
	vals := make([]int64, 0, len(matches))
	for _, m := range matches {
		f, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		if m[2] != "" {
			f *= 1000
		}
		vals = append(vals, int64(math.Round(f*100)))
	}
	if len(vals) == 0 {
		return nil, nil
	}
	lo, hi := vals[0], vals[0]
	if len(vals) > 1 {
		hi = vals[1]
	}
	if hi < lo {
		lo, hi = hi, lo
	}
	return &lo, &hi
}

// SimplifyNotes builds the Notes cell Simplify's importer expects.
func SimplifyNotes(fit, salary, status, appliedDate, nextStep, notes string) string {
	label := FitLabel(fit)
	if label == "" {
		label = "Unspecified"
	}
	if strings.TrimSpace(salary) == "" {
		salary = "Not listed"
	}
	parts := []string{label + " fit", "salary " + salary}
	if strings.TrimSpace(appliedDate) != "" {
		parts = append(parts, "applied "+strings.TrimSpace(appliedDate))
	} else {
		parts = append(parts, "status: "+StatusLabel(status))
	}
	if strings.TrimSpace(nextStep) != "" {
		parts = append(parts, "next: "+strings.TrimSpace(nextStep))
	}
	if strings.TrimSpace(notes) != "" {
		parts = append(parts, notes)
	}
	return strings.Join(parts, "; ")
}

const (
	EventStatusChange       = "status_change"
	EventEmailReceived      = "email_received"
	EventInterviewScheduled = "interview_scheduled"
	EventNote               = "note"
	EventImported           = "imported"
	EventLinkCheck          = "link_check"
	EventApproval           = "approval"
)

func ParseEventKind(s string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(s))
	n = strings.ReplaceAll(n, "-", "_")
	n = strings.ReplaceAll(n, " ", "_")
	switch n {
	case EventEmailReceived, EventInterviewScheduled, EventNote:
		return n, nil
	case EventStatusChange, EventImported, EventLinkCheck, EventApproval:
		return "", Validation("cannot append event kind %q", s).WithHint("use the transition endpoint for status changes; append email_received, interview_scheduled, or note")
	default:
		return "", Validation("invalid event kind %q", s).WithHint("kind is email_received, interview_scheduled, or note")
	}
}

func DefaultEventTitle(kind string) string {
	switch kind {
	case EventEmailReceived:
		return "Email received"
	case EventInterviewScheduled:
		return "Interview scheduled"
	case EventNote:
		return "Note"
	case EventImported:
		return "Imported from CSV"
	case EventStatusChange:
		return "Status changed"
	default:
		return kind
	}
}

// Actor is the token that authorized a request.
type Actor struct {
	UserID  string
	TokenID string
	Name    string
	Scopes  []string
}

func (a Actor) Can(scope string) bool {
	for _, s := range a.Scopes {
		if s == scope || (s == "write" && scope == "read") {
			return true
		}
	}
	return false
}

type actorKey struct{}

func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	return a, ok
}

type ActorRef struct {
	TokenID string `json:"token_id,omitempty"`
	Name    string `json:"name,omitempty"`
}

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Token struct {
	ID         string     `json:"id"`
	UserID     string     `json:"-"`
	Name       string     `json:"name"`
	Hash       string     `json:"-"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

type IssuedToken struct {
	Token  Token  `json:"token"`
	Secret string `json:"secret"`
}

type Me struct {
	User  User  `json:"user"`
	Token Token `json:"token"`
}

type Company struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	Name      string    `json:"name"`
	Website   string    `json:"website"`
	Notes     string    `json:"notes"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Role struct {
	ID             string    `json:"id"`
	UserID         string    `json:"-"`
	CompanyID      string    `json:"company_id"`
	Company        string    `json:"company"`
	Title          string    `json:"title"`
	Location       string    `json:"location"`
	Salary         string    `json:"salary"`
	SalaryMinCents *int64    `json:"salary_min_cents,omitempty"`
	SalaryMaxCents *int64    `json:"salary_max_cents,omitempty"`
	Fit            string    `json:"fit"`
	PostingURL     string    `json:"posting_url"`
	Source         string    `json:"source"`
	JobType        string    `json:"job_type"`
	Version        int       `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Application struct {
	ID             string     `json:"id"`
	UserID         string     `json:"-"`
	CompanyID      string     `json:"company_id"`
	Company        string     `json:"company"`
	RoleID         string     `json:"role_id"`
	Title          string     `json:"title"`
	Location       string     `json:"location"`
	Salary         string     `json:"salary"`
	SalaryMinCents *int64     `json:"salary_min_cents,omitempty"`
	SalaryMaxCents *int64     `json:"salary_max_cents,omitempty"`
	Fit            string     `json:"fit"`
	Status         string     `json:"status"`
	AppliedDate    *string    `json:"applied_date"`
	NextStep       string     `json:"next_step"`
	Notes          string     `json:"notes"`
	PostingURL     string     `json:"posting_url"`
	Source         string     `json:"source"`
	JobType        string     `json:"job_type"`
	ResumeVersion  string     `json:"resume_version"`
	ResumeFileRef  string     `json:"resume_file_ref"`
	ContactEmail   string     `json:"contact_email"`
	ExternalID     string     `json:"external_id,omitempty"`
	Tags           []string   `json:"tags"`
	LinkStatus     string     `json:"link_status,omitempty"`
	LinkCheckedAt  *time.Time `json:"link_checked_at,omitempty"`
	Version        int        `json:"version"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (a Application) AppliedDateValue() string {
	if a.AppliedDate == nil {
		return ""
	}
	return *a.AppliedDate
}

type Event struct {
	ID            string    `json:"id"`
	UserID        string    `json:"-"`
	ApplicationID string    `json:"application_id"`
	Kind          string    `json:"kind"`
	FromStatus    string    `json:"from_status,omitempty"`
	ToStatus      string    `json:"to_status,omitempty"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	OccurredAt    time.Time `json:"occurred_at"`
	CreatedAt     time.Time `json:"created_at"`
	Actor         ActorRef  `json:"actor"`
}

type Tag struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Audit struct {
	ID         string          `json:"id"`
	UserID     string          `json:"-"`
	TokenID    string          `json:"token_id"`
	ActorName  string          `json:"actor_name"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Summary    string          `json:"summary"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

type PageMeta struct {
	Limit      int    `json:"limit"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
	Total      int    `json:"total"`
}

type Page[T any] struct {
	Data []T      `json:"data"`
	Page PageMeta `json:"page"`
}

type Deleted struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type ListQuery struct {
	Limit         int
	Sort          string
	Cursor        *Cursor
	Q             string
	Status        []string
	Company       string
	CompanyID     string
	Fit           string
	Source        string
	Tag           string
	ApplicationID string
	Actor         string
	Action        string
	EntityType    string
	EntityID      string
	Stale         bool
	AfterID       string
	AfterTime     string
	// Link listing. Attention means closed or changed since the previous check.
	LinkStatus string
	Changed    *bool
	Attention  bool
}

type Cursor struct {
	V    int    `json:"v"`
	Sort string `json:"sort"`
	K    string `json:"k"`
	ID   string `json:"id"`
}

func EncodeCursor(sort, key, id string) string {
	b, _ := json.Marshal(Cursor{V: 1, Sort: sort, K: key, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodeCursor(s string) (Cursor, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Cursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return Cursor{}, Validation("invalid cursor").WithHint("pass next_cursor from the previous page unchanged")
		}
	}
	var c Cursor
	if err := json.Unmarshal(raw, &c); err != nil || c.V != 1 || c.ID == "" {
		return Cursor{}, Validation("invalid cursor").WithHint("pass next_cursor from the previous page unchanged")
	}
	return c, nil
}

func CheckCursorSort(c Cursor, sort string) error {
	if c.ID == "" {
		return nil
	}
	if c.Sort != sort {
		return Validation("cursor does not match sort %q", sort).WithHint("reuse the cursor only with the same sort, or omit the cursor")
	}
	return nil
}

var (
	CompanyFields = []string{"id", "name", "website", "notes", "version", "created_at", "updated_at"}
	RoleFields    = []string{"id", "company_id", "company", "title", "location", "salary", "salary_min_cents", "salary_max_cents", "fit", "posting_url", "source", "job_type", "version", "created_at", "updated_at"}
	AppFields     = []string{"id", "company_id", "company", "role_id", "title", "location", "salary", "salary_min_cents", "salary_max_cents", "fit", "status", "applied_date", "next_step", "notes", "posting_url", "source", "job_type", "resume_version", "resume_file_ref", "contact_email", "external_id", "tags", "link_status", "link_checked_at", "version", "created_at", "updated_at"}
	EventFields   = []string{"id", "application_id", "kind", "from_status", "to_status", "title", "body", "occurred_at", "created_at", "actor"}
	TagFields     = []string{"id", "name", "created_at"}
	TokenFields   = []string{"id", "name", "prefix", "scopes", "created_at", "last_used_at", "revoked_at"}
	AuditFields   = []string{"id", "token_id", "actor_name", "action", "entity_type", "entity_id", "summary", "metadata", "created_at"}
	LinkFields    = []string{"application_id", "role_id", "company", "title", "posting_url", "status", "detail", "http_status", "changed", "checked_at"}
)

// LinkHealth is one posting the link checker has already classified.
type LinkHealth struct {
	ApplicationID string `json:"application_id"`
	RoleID        string `json:"role_id"`
	Company       string `json:"company"`
	Title         string `json:"title"`
	PostingURL    string `json:"posting_url"`
	Status        string `json:"status"`
	Detail        string `json:"detail"`
	HTTPStatus    int    `json:"http_status"`
	Changed       bool   `json:"changed"`
	CheckedAt     string `json:"checked_at"`
}

func ValidateFields(requested, allowed []string) error {
	allow := map[string]bool{}
	for _, f := range allowed {
		allow[f] = true
	}
	var unknown []string
	for _, f := range requested {
		if !allow[f] {
			unknown = append(unknown, f)
		}
	}
	if len(unknown) > 0 {
		return Validation("unknown field %s", strings.Join(unknown, ", ")).WithHint("allowed fields: " + strings.Join(allowed, ", "))
	}
	return nil
}

// Error is a structured failure safe to show to an agent.
type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Hint    string         `json:"hint,omitempty"`
	Details map[string]any `json:"details,omitempty"`
	Status  int            `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

func (e *Error) WithHint(h string) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Hint = h
	return &cp
}

func (e *Error) WithDetails(d map[string]any) *Error {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Details = d
	return &cp
}

func Validation(format string, args ...any) *Error {
	return &Error{Code: "validation", Message: fmt.Sprintf(format, args...), Status: 400, Hint: "see /openapi.json for the request shape"}
}

func NotFound(entity string) *Error {
	return &Error{Code: "not_found", Message: entity + " not found", Hint: "check the id with the matching list command", Status: 404}
}

func Conflict(message, hint string) *Error {
	return &Error{Code: "conflict", Message: message, Hint: hint, Status: 409}
}

func VersionConflict(entity string, current int) *Error {
	return Conflict(entity+" was modified", "fetch the current "+entity+" and retry with its version").WithDetails(map[string]any{"current_version": current})
}

func Unauthorized() *Error {
	return &Error{Code: "unauthorized", Message: "invalid or revoked token", Hint: "set Authorization: Bearer to a pursuit token, or pass --token / PURSUIT_TOKEN", Status: 401}
}

func Forbidden(scope string) *Error {
	return &Error{Code: "forbidden", Message: "token is missing the " + scope + " scope", Hint: "create a token with --scope " + scope, Status: 403}
}

func CapExceeded(company string, used, max, windowDays int) *Error {
	return Conflict(
		fmt.Sprintf("%s application cap reached (%d/%d in %d days)", company, used, max, windowDays),
		"wait for the window to pass, withdraw an application, or raise the cap",
	).WithDetails(map[string]any{"company": company, "used": used, "max": max, "window_days": windowDays})
}

func IdempotencyMismatch() *Error {
	return &Error{Code: "idempotency_mismatch", Message: "idempotency key was reused with a different request", Hint: "reuse a key only with an identical request, or send a new key", Status: 409}
}

func IsCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e != nil && e.Code == code
}

func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) && e != nil {
		return e, true
	}
	return nil, false
}

func RequireText(name, value string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", Validation("%s is required", name).WithHint("set " + name)
	}
	if max > 0 && len(value) > max {
		return "", Validation("%s is too long", name).WithHint(fmt.Sprintf("keep %s under %d characters", name, max))
	}
	return value, nil
}

func BoundText(name, value string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if max > 0 && len(value) > max {
		return "", Validation("%s is too long", name).WithHint(fmt.Sprintf("keep %s under %d characters", name, max))
	}
	return value, nil
}
