package sqlstore

import (
	"context"
	"database/sql"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
)

// sweepLockKey is distinct from the migration advisory lock.
const sweepLockKey int64 = 594132718233452

// postgresSkipLocked is the replica-safe queue claim clause.
const postgresSkipLocked = "FOR UPDATE SKIP LOCKED"

type WorkItem struct {
	ID            string
	UserID        string
	ApplicationID string
	Kind          string
	State         string
	LeaseOwner    string
	LeaseUntil    string
	Attempts      int
	CreatedAt     string
	UpdatedAt     string
}

type Artifact struct {
	ID            string
	UserID        string
	ApplicationID string
	Kind          string
	Filename      string
	ContentType   string
	Body          string
	CreatedAt     string
}

type Question struct {
	ID            string
	UserID        string
	ApplicationID string
	Prompt        string
	Answer        string
	Required      bool
	Unknown       bool
	Source        string
	CreatedAt     string
}

type StandingAnswer struct {
	ID        string `json:"id"`
	UserID    string `json:"-"`
	Key       string `json:"key"`
	Answer    string `json:"answer"`
	UpdatedAt string `json:"updated_at"`
}

type Approval struct {
	ID            string
	UserID        string
	ApplicationID string
	TokenID       string
	ActorName     string
	Note          string
	CreatedAt     string
}

type CompanyCap struct {
	ID         string `json:"id"`
	UserID     string `json:"-"`
	Company    string `json:"company"`
	Max        int    `json:"max"`
	WindowDays int    `json:"window_days"`
	Mode       string `json:"mode"`
}

type PostingCheck struct {
	RoleID     string
	UserID     string
	Status     string
	Detail     string
	HTTPStatus int
	CheckedAt  string
}

type Webhook struct {
	ID        string
	UserID    string
	URL       string
	Secret    string
	CreatedAt string
}

func (s *Store) GetApplicationByPostingURL(ctx context.Context, userID, postingURL string) (domain.Application, error) {
	a, err := scanApp(s.queryRow(ctx, appSelect()+` WHERE a.user_id=? AND r.posting_url=? ORDER BY a.created_at, a.id LIMIT 1`, userID, postingURL))
	if err == sql.ErrNoRows {
		return domain.Application{}, domain.NotFound("application")
	}
	if err != nil {
		return domain.Application{}, err
	}
	names, err := s.TagNames(ctx, []string{a.ID})
	if err != nil {
		return domain.Application{}, err
	}
	if tags := names[a.ID]; tags != nil {
		a.Tags = tags
	}
	return a, nil
}

func (s *Store) FindRoleByPostingURL(ctx context.Context, userID, postingURL string) (domain.Role, error) {
	r, err := scanRole(s.queryRow(ctx, roleSelect()+` WHERE r.user_id=? AND r.posting_url=? LIMIT 1`, userID, postingURL))
	if err == sql.ErrNoRows {
		return domain.Role{}, domain.NotFound("role")
	}
	return r, err
}

func (s *Store) EnqueueWork(ctx context.Context, item WorkItem) (WorkItem, error) {
	var existing WorkItem
	err := scanWork(s.queryRow(ctx, `SELECT id, user_id, application_id, kind, state, lease_owner, lease_until, attempts, created_at, updated_at FROM work_items WHERE user_id=? AND application_id=? AND kind=? AND state IN ('queued','leased') ORDER BY created_at LIMIT 1`, item.UserID, item.ApplicationID, item.Kind), &existing)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return WorkItem{}, err
	}
	if _, err := s.exec(ctx, `INSERT INTO work_items (id, user_id, application_id, kind, state, lease_owner, lease_until, attempts, created_at, updated_at) VALUES (?, ?, ?, ?, 'queued', '', NULL, 0, ?, ?)`,
		item.ID, item.UserID, item.ApplicationID, item.Kind, item.CreatedAt, item.UpdatedAt); err != nil {
		return WorkItem{}, err
	}
	item.State = "queued"
	return item, nil
}

func scanWork(row scanner, item *WorkItem) error {
	var until sql.NullString
	err := row.Scan(&item.ID, &item.UserID, &item.ApplicationID, &item.Kind, &item.State, &item.LeaseOwner, &until, &item.Attempts, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return err
	}
	if until.Valid {
		item.LeaseUntil = until.String
	}
	return nil
}

const workCols = `id, user_id, application_id, kind, state, lease_owner, lease_until, attempts, created_at, updated_at`

// ClaimWork leases the oldest queued or expired work item.
// Postgres uses FOR UPDATE SKIP LOCKED so replicas do not block on each other.
// SQLite writers already serialize on an immediate transaction, so one UPDATE
// claims a single row.
func (s *Store) ClaimWork(ctx context.Context, userID, worker, now, until string) (WorkItem, bool, error) {
	if s.dialect == "postgres" {
		return s.claimWorkPostgres(ctx, userID, worker, now, until)
	}
	row := s.queryRow(ctx, `UPDATE work_items SET state='leased', lease_owner=?, lease_until=?, attempts=attempts+1, updated_at=?
		WHERE id = (
			SELECT id FROM work_items
			WHERE user_id=? AND (state='queued' OR (state='leased' AND (lease_until IS NULL OR lease_until='' OR lease_until < ?)))
			ORDER BY created_at, id
			LIMIT 1
		)
		RETURNING `+workCols, worker, until, now, userID, now)
	var item WorkItem
	err := scanWork(row, &item)
	if err == sql.ErrNoRows {
		return WorkItem{}, false, nil
	}
	if err != nil {
		return WorkItem{}, false, err
	}
	return item, true, nil
}

func (s *Store) claimWorkPostgres(ctx context.Context, userID, worker, now, until string) (WorkItem, bool, error) {
	var id string
	err := s.queryRow(ctx, `SELECT id FROM work_items
		WHERE user_id=? AND (state='queued' OR (state='leased' AND (lease_until IS NULL OR lease_until='' OR lease_until < ?)))
		ORDER BY created_at, id
		LIMIT 1
		`+postgresSkipLocked, userID, now).Scan(&id)
	if err == sql.ErrNoRows {
		return WorkItem{}, false, nil
	}
	if err != nil {
		return WorkItem{}, false, err
	}
	var item WorkItem
	err = scanWork(s.queryRow(ctx, `UPDATE work_items SET state='leased', lease_owner=?, lease_until=?, attempts=attempts+1, updated_at=? WHERE id=? RETURNING `+workCols,
		worker, until, now, id), &item)
	if err != nil {
		return WorkItem{}, false, err
	}
	return item, true, nil
}

func (s *Store) HeartbeatWork(ctx context.Context, userID, id, worker, now, until string) (WorkItem, error) {
	var item WorkItem
	err := scanWork(s.queryRow(ctx, `UPDATE work_items SET lease_until=?, updated_at=?
		WHERE id=? AND user_id=? AND lease_owner=? AND state='leased' AND lease_until IS NOT NULL AND lease_until >= ?
		RETURNING `+workCols, until, now, id, userID, worker, now), &item)
	if err == sql.ErrNoRows {
		return WorkItem{}, domain.Conflict("lease is not held by this worker", "claim the job again; the previous lease expired or belongs to someone else")
	}
	return item, err
}

func (s *Store) FinishWork(ctx context.Context, userID, id, worker, now string) error {
	res, err := s.exec(ctx, `UPDATE work_items SET state='done', lease_owner='', lease_until=NULL, updated_at=?
		WHERE id=? AND user_id=? AND lease_owner=? AND state='leased' AND lease_until IS NOT NULL AND lease_until >= ?`, now, id, userID, worker, now)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.Conflict("lease is not held by this worker", "claim the job again before completing it")
	}
	return nil
}

func (s *Store) GetWork(ctx context.Context, userID, id string) (WorkItem, error) {
	var item WorkItem
	err := scanWork(s.queryRow(ctx, `SELECT `+workCols+` FROM work_items WHERE user_id=? AND id=?`, userID, id), &item)
	if err == sql.ErrNoRows {
		return WorkItem{}, domain.NotFound("work item")
	}
	return item, err
}

func (s *Store) CreateApproval(ctx context.Context, a Approval) error {
	_, err := s.exec(ctx, `INSERT INTO approvals (id, user_id, application_id, token_id, actor_name, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.UserID, a.ApplicationID, a.TokenID, a.ActorName, a.Note, a.CreatedAt)
	return err
}

func (s *Store) HasApproval(ctx context.Context, userID, applicationID string) (bool, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM approvals WHERE user_id=? AND application_id=?`, userID, applicationID).Scan(&n)
	return n > 0, err
}

func (s *Store) CreateArtifact(ctx context.Context, a Artifact) error {
	_, err := s.exec(ctx, `INSERT INTO artifacts (id, user_id, application_id, kind, filename, content_type, body, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.UserID, a.ApplicationID, a.Kind, a.Filename, a.ContentType, a.Body, a.CreatedAt)
	return err
}

func (s *Store) ListArtifacts(ctx context.Context, userID, applicationID string) ([]Artifact, error) {
	rows, err := s.query(ctx, `SELECT id, user_id, application_id, kind, filename, content_type, created_at FROM artifacts WHERE user_id=? AND application_id=? ORDER BY created_at, id`, userID, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Artifact
	for rows.Next() {
		var a Artifact
		if err := rows.Scan(&a.ID, &a.UserID, &a.ApplicationID, &a.Kind, &a.Filename, &a.ContentType, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if out == nil {
		out = []Artifact{}
	}
	return out, rows.Err()
}

func (s *Store) GetArtifact(ctx context.Context, userID, id string) (Artifact, error) {
	var a Artifact
	err := s.queryRow(ctx, `SELECT id, user_id, application_id, kind, filename, content_type, body, created_at FROM artifacts WHERE user_id=? AND id=?`, userID, id).
		Scan(&a.ID, &a.UserID, &a.ApplicationID, &a.Kind, &a.Filename, &a.ContentType, &a.Body, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return Artifact{}, domain.NotFound("artifact")
	}
	return a, err
}

func (s *Store) ReplaceQuestions(ctx context.Context, userID, applicationID string, qs []Question) error {
	if _, err := s.exec(ctx, `DELETE FROM questions WHERE user_id=? AND application_id=?`, userID, applicationID); err != nil {
		return err
	}
	for _, q := range qs {
		req, unk := 0, 0
		if q.Required {
			req = 1
		}
		if q.Unknown {
			unk = 1
		}
		if _, err := s.exec(ctx, `INSERT INTO questions (id, user_id, application_id, prompt, answer, required, unknown, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			q.ID, userID, applicationID, q.Prompt, q.Answer, req, unk, q.Source, q.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListQuestions(ctx context.Context, userID, applicationID string) ([]Question, error) {
	rows, err := s.query(ctx, `SELECT id, user_id, application_id, prompt, answer, required, unknown, source, created_at FROM questions WHERE user_id=? AND application_id=? ORDER BY created_at, id`, userID, applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Question
	for rows.Next() {
		var q Question
		var req, unk int
		if err := rows.Scan(&q.ID, &q.UserID, &q.ApplicationID, &q.Prompt, &q.Answer, &req, &unk, &q.Source, &q.CreatedAt); err != nil {
			return nil, err
		}
		q.Required, q.Unknown = req == 1, unk == 1
		out = append(out, q)
	}
	if out == nil {
		out = []Question{}
	}
	return out, rows.Err()
}

func (s *Store) UpsertStanding(ctx context.Context, a StandingAnswer) error {
	_, err := s.exec(ctx, `INSERT INTO standing_answers (id, user_id, answer_key, answer, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, answer_key) DO UPDATE SET answer=excluded.answer, updated_at=excluded.updated_at`,
		a.ID, a.UserID, a.Key, a.Answer, a.UpdatedAt)
	return err
}

func (s *Store) ListStanding(ctx context.Context, userID string) ([]StandingAnswer, error) {
	rows, err := s.query(ctx, `SELECT id, user_id, answer_key, answer, updated_at FROM standing_answers WHERE user_id=? ORDER BY answer_key`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StandingAnswer
	for rows.Next() {
		var a StandingAnswer
		if err := rows.Scan(&a.ID, &a.UserID, &a.Key, &a.Answer, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if out == nil {
		out = []StandingAnswer{}
	}
	return out, rows.Err()
}

func (s *Store) EnsureCap(ctx context.Context, c CompanyCap) error {
	_, err := s.exec(ctx, `INSERT INTO company_caps (id, user_id, company, max_count, window_days, mode) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, company) DO NOTHING`, c.ID, c.UserID, c.Company, c.Max, c.WindowDays, c.Mode)
	return err
}

func (s *Store) ListCaps(ctx context.Context, userID string) ([]CompanyCap, error) {
	rows, err := s.query(ctx, `SELECT id, user_id, company, max_count, window_days, mode FROM company_caps WHERE user_id=? ORDER BY company`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CompanyCap
	for rows.Next() {
		var c CompanyCap
		if err := rows.Scan(&c.ID, &c.UserID, &c.Company, &c.Max, &c.WindowDays, &c.Mode); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if out == nil {
		out = []CompanyCap{}
	}
	return out, rows.Err()
}

func (s *Store) CountCap(ctx context.Context, userID, company, since, excludeID, mode string) (int, error) {
	q := `SELECT COUNT(*) FROM applications a
		JOIN roles r ON r.id = a.role_id
		JOIN companies c ON c.id = r.company_id
		WHERE a.user_id=? AND lower(c.name)=lower(?) AND a.created_at>=? AND a.id<>?`
	args := []any{userID, company, since, excludeID}
	switch mode {
	case "submitted":
		q += ` AND a.status IN ('applied','submitted','submitting','screening','interviewing','offer')`
	default:
		q += ` AND a.status NOT IN ('rejected','withdrawn','ghosted','failed','saved')`
	}
	var n int
	err := s.queryRow(ctx, q, args...).Scan(&n)
	return n, err
}

func (s *Store) UpsertPostingCheck(ctx context.Context, c PostingCheck) error {
	_, err := s.exec(ctx, `INSERT INTO posting_checks (role_id, user_id, status, detail, http_status, checked_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(role_id) DO UPDATE SET status=excluded.status, detail=excluded.detail, http_status=excluded.http_status, checked_at=excluded.checked_at, user_id=excluded.user_id`,
		c.RoleID, c.UserID, c.Status, c.Detail, c.HTTPStatus, c.CheckedAt)
	return err
}

func (s *Store) GetPostingCheck(ctx context.Context, roleID string) (PostingCheck, bool, error) {
	var c PostingCheck
	err := s.queryRow(ctx, `SELECT role_id, user_id, status, detail, http_status, checked_at FROM posting_checks WHERE role_id=?`, roleID).
		Scan(&c.RoleID, &c.UserID, &c.Status, &c.Detail, &c.HTTPStatus, &c.CheckedAt)
	if err == sql.ErrNoRows {
		return PostingCheck{}, false, nil
	}
	return c, err == nil, err
}

func (s *Store) ListRoleIDsForCheck(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.query(ctx, `SELECT DISTINCT r.id FROM roles r
		JOIN applications a ON a.role_id = r.id
		WHERE r.user_id=? AND a.status IN ('to_apply','awaiting_review','preparing','saved')
		AND r.posting_url<>''
		ORDER BY r.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) ApplicationsForRole(ctx context.Context, userID, roleID string) ([]domain.Application, error) {
	rows, err := s.query(ctx, appSelect()+` WHERE a.user_id=? AND a.role_id=? ORDER BY a.created_at, a.id`, userID, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Application
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CreateWebhook(ctx context.Context, w Webhook) error {
	_, err := s.exec(ctx, `INSERT INTO webhooks (id, user_id, url, secret, created_at) VALUES (?, ?, ?, ?, ?)`, w.ID, w.UserID, w.URL, w.Secret, w.CreatedAt)
	return err
}

func (s *Store) ListWebhooks(ctx context.Context, userID string) ([]Webhook, error) {
	rows, err := s.query(ctx, `SELECT id, user_id, url, secret, created_at FROM webhooks WHERE user_id=? ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Webhook
	for rows.Next() {
		var w Webhook
		if err := rows.Scan(&w.ID, &w.UserID, &w.URL, &w.Secret, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	if out == nil {
		out = []Webhook{}
	}
	return out, rows.Err()
}

func (s *Store) DeleteWebhook(ctx context.Context, userID, id string) error {
	res, err := s.exec(ctx, `DELETE FROM webhooks WHERE user_id=? AND id=?`, userID, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.NotFound("webhook")
	}
	return nil
}

func (s *Store) GetEvent(ctx context.Context, userID, id string) (domain.Event, error) {
	e, err := scanEvent(s.queryRow(ctx, `SELECT e.id, e.user_id, e.application_id, e.kind, e.from_status, e.to_status, e.title, e.body, e.occurred_at, e.created_at, e.actor_token_id, e.actor_name FROM events e JOIN applications a ON a.id = e.application_id WHERE a.user_id=? AND e.id=?`, userID, id))
	if err == sql.ErrNoRows {
		return domain.Event{}, domain.NotFound("event")
	}
	return e, err
}

// Leader is a sweep lock held by one replica.
type Leader struct {
	store    *Store
	conn     *sql.Conn
	name     string
	owner    string
	advisory bool
}

// AcquireLeader takes the link-sweep lock. Postgres also takes an advisory
// lock so a second replica fails immediately instead of waiting.
func (s *Store) AcquireLeader(ctx context.Context, name, owner string, now, until time.Time) (*Leader, bool, error) {
	if s.dialect == "postgres" {
		return s.acquireLeaderPostgres(ctx, name, owner, now, until)
	}
	var ok bool
	err := s.WithLock(ctx, func(ctx context.Context) error {
		res, err := s.exec(ctx, `INSERT INTO leader_leases (name, owner, lease_until) VALUES (?, ?, ?)
			ON CONFLICT(name) DO UPDATE SET owner=excluded.owner, lease_until=excluded.lease_until
			WHERE leader_leases.lease_until < ? OR leader_leases.owner = excluded.owner`,
			name, owner, domain.FormatTime(until), domain.FormatTime(now))
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		ok = err == nil && n == 1
		return err
	})
	if err != nil || !ok {
		return nil, false, err
	}
	return &Leader{store: s, name: name, owner: owner}, true, nil
}

func (s *Store) acquireLeaderPostgres(ctx context.Context, name, owner string, now, until time.Time) (*Leader, bool, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	var locked bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", sweepLockKey).Scan(&locked); err != nil {
		conn.Close()
		return nil, false, err
	}
	if !locked {
		conn.Close()
		return nil, false, nil
	}
	res, err := conn.ExecContext(ctx, `INSERT INTO leader_leases (name, owner, lease_until) VALUES ($1, $2, $3)
		ON CONFLICT(name) DO UPDATE SET owner=EXCLUDED.owner, lease_until=EXCLUDED.lease_until
		WHERE leader_leases.lease_until < $4 OR leader_leases.owner = EXCLUDED.owner`,
		name, owner, domain.FormatTime(until), domain.FormatTime(now))
	if err != nil {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", sweepLockKey)
		conn.Close()
		return nil, false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", sweepLockKey)
		conn.Close()
		return nil, false, nil
	}
	return &Leader{store: s, conn: conn, name: name, owner: owner, advisory: true}, true, nil
}

func (l *Leader) Renew(ctx context.Context, until time.Time) error {
	q := `UPDATE leader_leases SET lease_until=? WHERE name=? AND owner=?`
	args := []any{domain.FormatTime(until), l.name, l.owner}
	if l.conn != nil {
		_, err := l.conn.ExecContext(ctx, rebind(l.store.dialect, q), args...)
		return err
	}
	_, err := l.store.exec(ctx, q, args...)
	return err
}

func (l *Leader) Release() {
	if l == nil {
		return
	}
	ctx := context.Background()
	q := `UPDATE leader_leases SET lease_until=? WHERE name=? AND owner=?`
	past := domain.FormatTime(time.Unix(0, 0).UTC())
	if l.conn != nil {
		_, _ = l.conn.ExecContext(ctx, rebind(l.store.dialect, q), past, l.name, l.owner)
		if l.advisory {
			_, _ = l.conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", sweepLockKey)
		}
		l.conn.Close()
		return
	}
	_, _ = l.store.exec(ctx, q, past, l.name, l.owner)
}
