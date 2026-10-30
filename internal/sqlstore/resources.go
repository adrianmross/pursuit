package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
)

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, u domain.User) error {
	_, err := s.exec(ctx, `INSERT INTO users (id, name, email, created_at) VALUES (?, ?, ?, ?)`,
		u.ID, u.Name, u.Email, formatTime(u.CreatedAt))
	return mapWriteErr(err)
}

func (s *Store) FirstUser(ctx context.Context) (domain.User, error) {
	var u domain.User
	var created string
	err := s.queryRow(ctx, `SELECT id, name, email, created_at FROM users ORDER BY created_at LIMIT 1`).Scan(&u.ID, &u.Name, &u.Email, &created)
	if err == sql.ErrNoRows {
		return domain.User{}, domain.NotFound("user")
	}
	if err != nil {
		return domain.User{}, err
	}
	u.CreatedAt, err = parseTime(created)
	return u, err
}

func (s *Store) FindUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	var created string
	err := s.queryRow(ctx, `SELECT id, name, email, created_at FROM users WHERE lower(email)=lower(?)`, email).
		Scan(&u.ID, &u.Name, &u.Email, &created)
	if err == sql.ErrNoRows {
		return domain.User{}, domain.NotFound("user")
	}
	if err != nil {
		return domain.User{}, err
	}
	u.CreatedAt, err = parseTime(created)
	return u, err
}

func (s *Store) GetUser(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	var created string
	err := s.queryRow(ctx, `SELECT id, name, email, created_at FROM users WHERE id=?`, id).
		Scan(&u.ID, &u.Name, &u.Email, &created)
	if err == sql.ErrNoRows {
		return domain.User{}, domain.NotFound("user")
	}
	if err != nil {
		return domain.User{}, err
	}
	u.CreatedAt, err = parseTime(created)
	return u, err
}

func (s *Store) CountTokens(ctx context.Context) (int, error) {
	var n int
	err := s.queryRow(ctx, `SELECT COUNT(*) FROM api_tokens`).Scan(&n)
	return n, err
}

func (s *Store) CreateToken(ctx context.Context, t domain.Token) error {
	_, err := s.exec(ctx, `INSERT INTO api_tokens (id, user_id, name, token_hash, token_prefix, scopes, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.UserID, t.Name, t.Hash, t.Prefix, strings.Join(t.Scopes, ","), formatTime(t.CreatedAt))
	return mapWriteErr(err)
}

func (s *Store) GetTokenByHash(ctx context.Context, hash string) (domain.Token, error) {
	row := s.queryRow(ctx, `SELECT id, user_id, name, token_hash, token_prefix, scopes, created_at, last_used_at, revoked_at FROM api_tokens WHERE token_hash=?`, hash)
	t, err := scanToken(row)
	if err == sql.ErrNoRows {
		return domain.Token{}, domain.NotFound("token")
	}
	return t, err
}

func (s *Store) GetToken(ctx context.Context, userID, id string) (domain.Token, error) {
	row := s.queryRow(ctx, `SELECT id, user_id, name, token_hash, token_prefix, scopes, created_at, last_used_at, revoked_at FROM api_tokens WHERE user_id=? AND id=?`, userID, id)
	t, err := scanToken(row)
	if err == sql.ErrNoRows {
		return domain.Token{}, domain.NotFound("token")
	}
	return t, err
}

func scanToken(row scanner) (domain.Token, error) {
	var t domain.Token
	var scopes, created string
	var last, revoked sql.NullString
	err := row.Scan(&t.ID, &t.UserID, &t.Name, &t.Hash, &t.Prefix, &scopes, &created, &last, &revoked)
	if err != nil {
		return domain.Token{}, err
	}
	if scopes != "" {
		t.Scopes = strings.Split(scopes, ",")
	} else {
		t.Scopes = []string{}
	}
	t.CreatedAt, err = parseTime(created)
	if err != nil {
		return domain.Token{}, err
	}
	t.LastUsedAt, err = optTime(last)
	if err != nil {
		return domain.Token{}, err
	}
	t.RevokedAt, err = optTime(revoked)
	return t, err
}

type scanner interface{ Scan(dest ...any) error }

func (s *Store) ListTokens(ctx context.Context, userID string, q domain.ListQuery) ([]domain.Token, int, error) {
	expr, desc, _, err := PrepareSort("token", q.Sort, q.Cursor)
	if err != nil {
		return nil, 0, err
	}
	w := where{}
	w.add("user_id=?", userID)
	if q.Cursor != nil && q.Cursor.ID != "" {
		w.add(cursorSQL(expr, "id", desc), q.Cursor.K, q.Cursor.K, q.Cursor.ID)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 51
	}
	rows, err := s.query(ctx, `SELECT id, user_id, name, token_hash, token_prefix, scopes, created_at, last_used_at, revoked_at FROM api_tokens `+w.sql()+` `+orderSQL(expr, "id", desc)+` LIMIT ?`, append(w.args, limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.Token
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.queryRow(ctx, `SELECT COUNT(*) FROM api_tokens WHERE user_id=?`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.Token{}
	}
	return out, total, nil
}

func (s *Store) RevokeToken(ctx context.Context, userID, id, at string) (bool, error) {
	res, err := s.exec(ctx, `UPDATE api_tokens SET revoked_at=? WHERE user_id=? AND id=? AND revoked_at IS NULL`, at, userID, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (s *Store) TouchToken(ctx context.Context, id, at, staleBefore string) error {
	_, err := s.exec(ctx, `UPDATE api_tokens SET last_used_at=? WHERE id=? AND (last_used_at IS NULL OR last_used_at < ?)`, at, id, staleBefore)
	return err
}

func (s *Store) CreateCompany(ctx context.Context, c domain.Company) error {
	_, err := s.exec(ctx, `INSERT INTO companies (id, user_id, name, website, notes, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.UserID, c.Name, c.Website, c.Notes, c.Version, formatTime(c.CreatedAt), formatTime(c.UpdatedAt))
	return mapWriteErr(err)
}

func (s *Store) GetCompany(ctx context.Context, userID, id string) (domain.Company, error) {
	c, err := scanCompany(s.queryRow(ctx, `SELECT id, user_id, name, website, notes, version, created_at, updated_at FROM companies WHERE user_id=? AND id=?`, userID, id))
	if err == sql.ErrNoRows {
		return domain.Company{}, domain.NotFound("company")
	}
	return c, err
}

func (s *Store) GetCompanyByName(ctx context.Context, userID, name string) (domain.Company, error) {
	c, err := scanCompany(s.queryRow(ctx, `SELECT id, user_id, name, website, notes, version, created_at, updated_at FROM companies WHERE user_id=? AND lower(name)=lower(?)`, userID, name))
	if err == sql.ErrNoRows {
		return domain.Company{}, domain.NotFound("company")
	}
	return c, err
}

func scanCompany(row scanner) (domain.Company, error) {
	var c domain.Company
	var created, updated string
	err := row.Scan(&c.ID, &c.UserID, &c.Name, &c.Website, &c.Notes, &c.Version, &created, &updated)
	if err != nil {
		return domain.Company{}, err
	}
	if c.CreatedAt, err = parseTime(created); err != nil {
		return domain.Company{}, err
	}
	c.UpdatedAt, err = parseTime(updated)
	return c, err
}

func (s *Store) ListCompanies(ctx context.Context, userID string, q domain.ListQuery) ([]domain.Company, int, error) {
	expr, desc, _, err := PrepareSort("company", q.Sort, q.Cursor)
	if err != nil {
		return nil, 0, err
	}
	w := where{}
	w.add("user_id=?", userID)
	countArgs := []any{userID}
	if q.Q != "" {
		p := likeContains(q.Q)
		clause := `(lower(name) LIKE ? ESCAPE '\' OR lower(notes) LIKE ? ESCAPE '\')`
		w.add(clause, p, p)
		countArgs = append(countArgs, p, p)
	}
	countWhere := w.sql()
	if q.Cursor != nil && q.Cursor.ID != "" {
		w.add(cursorSQL(expr, "id", desc), q.Cursor.K, q.Cursor.K, q.Cursor.ID)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 51
	}
	rows, err := s.query(ctx, `SELECT id, user_id, name, website, notes, version, created_at, updated_at FROM companies `+w.sql()+` `+orderSQL(expr, "id", desc)+` LIMIT ?`, append(w.args, limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.Company
	for rows.Next() {
		c, err := scanCompany(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.queryRow(ctx, `SELECT COUNT(*) FROM companies `+countWhere, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.Company{}
	}
	return out, total, nil
}

func (s *Store) UpdateCompany(ctx context.Context, c domain.Company) error {
	res, err := s.exec(ctx, `UPDATE companies SET name=?, website=?, notes=?, version=version+1, updated_at=? WHERE id=? AND user_id=? AND version=?`,
		c.Name, c.Website, c.Notes, formatTime(c.UpdatedAt), c.ID, c.UserID, c.Version)
	if err != nil {
		return mapWriteErr(err)
	}
	return changed(res, func() (int, error) {
		cur, err := s.GetCompany(ctx, c.UserID, c.ID)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}, "company")
}

func (s *Store) DeleteCompany(ctx context.Context, userID, id string, version int) error {
	res, err := s.exec(ctx, `DELETE FROM companies WHERE id=? AND user_id=? AND version=?`, id, userID, version)
	if err != nil {
		return mapWriteErr(err)
	}
	return changed(res, func() (int, error) {
		cur, err := s.GetCompany(ctx, userID, id)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}, "company")
}

func (s *Store) CreateRole(ctx context.Context, r domain.Role) error {
	_, err := s.exec(ctx, `INSERT INTO roles (id, user_id, company_id, title, location, salary_text, salary_min_cents, salary_max_cents, fit, posting_url, source, job_type, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.UserID, r.CompanyID, r.Title, r.Location, r.Salary, nullInt(r.SalaryMinCents), nullInt(r.SalaryMaxCents), r.Fit, r.PostingURL, r.Source, r.JobType, r.Version, formatTime(r.CreatedAt), formatTime(r.UpdatedAt))
	return mapWriteErr(err)
}

func roleSelect() string {
	return `SELECT r.id, r.user_id, r.company_id, c.name, r.title, r.location, r.salary_text, r.salary_min_cents, r.salary_max_cents, r.fit, r.posting_url, r.source, r.job_type, r.version, r.created_at, r.updated_at FROM roles r JOIN companies c ON c.id = r.company_id`
}

func scanRole(row scanner) (domain.Role, error) {
	var r domain.Role
	var min, max sql.NullInt64
	var created, updated string
	err := row.Scan(&r.ID, &r.UserID, &r.CompanyID, &r.Company, &r.Title, &r.Location, &r.Salary, &min, &max, &r.Fit, &r.PostingURL, &r.Source, &r.JobType, &r.Version, &created, &updated)
	if err != nil {
		return domain.Role{}, err
	}
	r.SalaryMinCents = optInt(min)
	r.SalaryMaxCents = optInt(max)
	if r.CreatedAt, err = parseTime(created); err != nil {
		return domain.Role{}, err
	}
	r.UpdatedAt, err = parseTime(updated)
	return r, err
}

func (s *Store) GetRole(ctx context.Context, userID, id string) (domain.Role, error) {
	r, err := scanRole(s.queryRow(ctx, roleSelect()+` WHERE r.user_id=? AND r.id=?`, userID, id))
	if err == sql.ErrNoRows {
		return domain.Role{}, domain.NotFound("role")
	}
	return r, err
}

func (s *Store) FindRoleByURL(ctx context.Context, userID, companyID, postingURL string) (domain.Role, error) {
	r, err := scanRole(s.queryRow(ctx, roleSelect()+` WHERE r.user_id=? AND r.company_id=? AND r.posting_url=?`, userID, companyID, postingURL))
	if err == sql.ErrNoRows {
		return domain.Role{}, domain.NotFound("role")
	}
	return r, err
}

func (s *Store) FindRoleByTitle(ctx context.Context, userID, companyID, title string) (domain.Role, error) {
	r, err := scanRole(s.queryRow(ctx, roleSelect()+` WHERE r.user_id=? AND r.company_id=? AND lower(r.title)=lower(?)`, userID, companyID, title))
	if err == sql.ErrNoRows {
		return domain.Role{}, domain.NotFound("role")
	}
	return r, err
}

func (s *Store) ListRoles(ctx context.Context, userID string, q domain.ListQuery) ([]domain.Role, int, error) {
	expr, desc, _, err := PrepareSort("role", q.Sort, q.Cursor)
	if err != nil {
		return nil, 0, err
	}
	w := where{}
	w.add("r.user_id=?", userID)
	if q.CompanyID != "" {
		w.add("r.company_id=?", q.CompanyID)
	}
	if q.Company != "" {
		w.add("lower(c.name)=lower(?)", q.Company)
	}
	if q.Fit != "" {
		w.add("r.fit=?", q.Fit)
	}
	if q.Source != "" {
		w.add("r.source=?", q.Source)
	}
	if q.Q != "" {
		p := likeContains(q.Q)
		w.add(`(lower(r.title) LIKE ? ESCAPE '\' OR lower(c.name) LIKE ? ESCAPE '\' OR lower(r.location) LIKE ? ESCAPE '\')`, p, p, p)
	}
	countSQL := `SELECT COUNT(*) FROM roles r JOIN companies c ON c.id = r.company_id ` + w.sql()
	countArgs := append([]any{}, w.args...)
	if q.Cursor != nil && q.Cursor.ID != "" {
		w.add(cursorSQL(expr, "r.id", desc), q.Cursor.K, q.Cursor.K, q.Cursor.ID)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 51
	}
	rows, err := s.query(ctx, roleSelect()+` `+w.sql()+` `+orderSQL(expr, "r.id", desc)+` LIMIT ?`, append(w.args, limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.Role
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.queryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.Role{}
	}
	return out, total, nil
}

func (s *Store) UpdateRole(ctx context.Context, r domain.Role) error {
	res, err := s.exec(ctx, `UPDATE roles SET title=?, location=?, salary_text=?, salary_min_cents=?, salary_max_cents=?, fit=?, posting_url=?, source=?, job_type=?, version=version+1, updated_at=? WHERE id=? AND user_id=? AND version=?`,
		r.Title, r.Location, r.Salary, nullInt(r.SalaryMinCents), nullInt(r.SalaryMaxCents), r.Fit, r.PostingURL, r.Source, r.JobType, formatTime(r.UpdatedAt), r.ID, r.UserID, r.Version)
	if err != nil {
		return mapWriteErr(err)
	}
	return changed(res, func() (int, error) {
		cur, err := s.GetRole(ctx, r.UserID, r.ID)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}, "role")
}

func (s *Store) DeleteRole(ctx context.Context, userID, id string, version int) error {
	res, err := s.exec(ctx, `DELETE FROM roles WHERE id=? AND user_id=? AND version=?`, id, userID, version)
	if err != nil {
		return mapWriteErr(err)
	}
	return changed(res, func() (int, error) {
		cur, err := s.GetRole(ctx, userID, id)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}, "role")
}

// effectiveStatus is what a row reads as: its outcome when that replaces the
// stage (needs_input, failed, rejected, withdrawn, ghosted), else its stage.
// waiting only overlays, so a waiting row still filters under its stage.
const effectiveStatus = `CASE WHEN a.outcome IN ('', 'waiting') THEN a.status ELSE a.outcome END`

// statusIn filters on effectiveStatus; "waiting" matches the overlay itself.
func statusIn(list []string, not bool) (string, []any) {
	ph := strings.TrimRight(strings.Repeat("?,", len(list)), ",")
	args := make([]any, len(list))
	waiting := false
	for i, st := range list {
		args[i] = st
		waiting = waiting || st == domain.StatusWaiting
	}
	if not {
		cond := effectiveStatus + " NOT IN (" + ph + ")"
		if waiting {
			cond += " AND a.outcome<>'waiting'"
		}
		return cond, args
	}
	cond := effectiveStatus + " IN (" + ph + ")"
	if waiting {
		cond = "(" + cond + " OR a.outcome='waiting')"
	}
	return cond, args
}

// attachInterviews fills Interviews with the done/total of each stage that has
// rounds, keyed by stage number: the segments of the mark that fill partly.
func (s *Store) attachInterviews(ctx context.Context, apps []domain.Application) error {
	if len(apps) == 0 {
		return nil
	}
	ph := strings.TrimRight(strings.Repeat("?,", len(apps)), ",")
	args := make([]any, len(apps))
	for i, a := range apps {
		args[i] = a.ID
	}
	rows, err := s.query(ctx, `SELECT application_id, stage, COUNT(*), COUNT(CASE WHEN done_at<>'' THEN 1 END) FROM interviews WHERE application_id IN (`+ph+`) GROUP BY application_id, stage`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	byApp := map[string]map[int]domain.InterviewCount{}
	for rows.Next() {
		var appID, stage string
		var c domain.InterviewCount
		if err := rows.Scan(&appID, &stage, &c.Total, &c.Done); err != nil {
			return err
		}
		if byApp[appID] == nil {
			byApp[appID] = map[int]domain.InterviewCount{}
		}
		n := domain.StageOf(stage)
		sum := byApp[appID][n]
		byApp[appID][n] = domain.InterviewCount{Done: sum.Done + c.Done, Total: sum.Total + c.Total}
	}
	for i := range apps {
		apps[i].Interviews = byApp[apps[i].ID]
	}
	return rows.Err()
}

func appSelect() string {
	return `SELECT a.id, a.user_id, a.role_id, a.status, a.outcome, a.applied_date, a.next_step, a.notes, a.resume_version, a.resume_file_ref, a.contact_email, a.external_id, a.archived_at, a.version, a.created_at, a.updated_at, r.title, r.location, r.salary_text, r.salary_min_cents, r.salary_max_cents, r.fit, r.posting_url, r.source, r.job_type, r.company_id, c.name, COALESCE(pc.status, ''), pc.checked_at FROM applications a JOIN roles r ON r.id = a.role_id JOIN companies c ON c.id = r.company_id LEFT JOIN posting_checks pc ON pc.role_id = r.id`
}

func scanApp(row scanner) (domain.Application, error) {
	var a domain.Application
	var applied sql.NullString
	var min, max sql.NullInt64
	var created, updated, archived string
	var checked sql.NullString
	err := row.Scan(&a.ID, &a.UserID, &a.RoleID, &a.Status, &a.Outcome, &applied, &a.NextStep, &a.Notes, &a.ResumeVersion, &a.ResumeFileRef, &a.ContactEmail, &a.ExternalID, &archived, &a.Version, &created, &updated, &a.Title, &a.Location, &a.Salary, &min, &max, &a.Fit, &a.PostingURL, &a.Source, &a.JobType, &a.CompanyID, &a.Company, &a.LinkStatus, &checked)
	if err != nil {
		return domain.Application{}, err
	}
	if applied.Valid && applied.String != "" {
		d := applied.String
		a.AppliedDate = &d
	}
	a.SalaryMinCents = optInt(min)
	a.SalaryMaxCents = optInt(max)
	a.Stage = domain.StageOf(a.Status)
	a.Tags = []string{}
	if checked.Valid && checked.String != "" {
		if t, perr := parseTime(checked.String); perr == nil {
			a.LinkCheckedAt = &t
		}
	}
	if archived != "" {
		if t, perr := parseTime(archived); perr == nil {
			a.ArchivedAt = &t
		}
	}
	if a.CreatedAt, err = parseTime(created); err != nil {
		return domain.Application{}, err
	}
	a.UpdatedAt, err = parseTime(updated)
	return a, err
}

func (s *Store) CreateApplication(ctx context.Context, a domain.Application) error {
	// An outcome has no stage of its own; a new row lands on applied when it
	// has an applied date, otherwise on to_apply.
	if domain.IsOutcome(a.Status) {
		a.Outcome, a.Status = a.Status, domain.StatusToApply
		if a.AppliedDateValue() != "" {
			a.Status = domain.StatusApplied
		}
	}
	_, err := s.exec(ctx, `INSERT INTO applications (id, user_id, role_id, status, outcome, applied_date, next_step, notes, resume_version, resume_file_ref, contact_email, external_id, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.UserID, a.RoleID, a.Status, a.Outcome, nullString(a.AppliedDateValue()), a.NextStep, a.Notes, a.ResumeVersion, a.ResumeFileRef, a.ContactEmail, a.ExternalID, a.Version, formatTime(a.CreatedAt), formatTime(a.UpdatedAt))
	return mapWriteErr(err)
}

func (s *Store) GetApplication(ctx context.Context, userID, id string) (domain.Application, error) {
	a, err := scanApp(s.queryRow(ctx, appSelect()+` WHERE a.user_id=? AND a.id=?`, userID, id))
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
	fields, err := s.FieldValues(ctx, []string{a.ID})
	if err != nil {
		return domain.Application{}, err
	}
	if v := fields[a.ID]; len(v) > 0 {
		a.CustomFields = v
	}
	one := []domain.Application{a}
	if err := s.attachInterviews(ctx, one); err != nil {
		return domain.Application{}, err
	}
	return one[0], nil
}

func (s *Store) GetApplicationByExternalID(ctx context.Context, userID, externalID string) (domain.Application, error) {
	a, err := scanApp(s.queryRow(ctx, appSelect()+` WHERE a.user_id=? AND a.external_id=?`, userID, externalID))
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

// fieldSortName returns the field name and descending flag for a
// "field:<name>" or "-field:<name>" sort, or ok=false for any other sort.
func fieldSortName(sort string) (name string, desc bool, ok bool) {
	desc = strings.HasPrefix(sort, "-")
	rest := strings.TrimPrefix(sort, "-")
	name, ok = strings.CutPrefix(rest, "field:")
	return name, desc, ok
}

func (s *Store) ListApplications(ctx context.Context, userID string, q domain.ListQuery) ([]domain.Application, int, error) {
	if name, desc, ok := fieldSortName(q.Sort); ok {
		return s.listApplicationsByField(ctx, userID, q, name, desc)
	}
	expr, desc, _, err := PrepareSort("application", q.Sort, q.Cursor)
	if err != nil {
		return nil, 0, err
	}
	w := where{}
	w.add("a.user_id=?", userID)
	if len(q.Status) > 0 {
		cond, args := statusIn(q.Status, false)
		w.add(cond, args...)
	}
	if len(q.StatusExclude) > 0 {
		cond, args := statusIn(q.StatusExclude, true)
		w.add(cond, args...)
	}
	if q.CompanyID != "" {
		w.add("r.company_id=?", q.CompanyID)
	}
	if q.Company != "" {
		w.add("lower(c.name)=lower(?)", q.Company)
	}
	if q.Fit != "" {
		w.add("r.fit=?", q.Fit)
	}
	if q.Source != "" {
		w.add("r.source=?", q.Source)
	}
	if q.Tag != "" {
		w.add(`EXISTS (SELECT 1 FROM application_tags at JOIN tags t ON t.id = at.tag_id WHERE at.application_id = a.id AND lower(t.name)=lower(?))`, q.Tag)
	}
	if q.Stale {
		w.add("pc.status=?", domain.LinkClosed)
	}
	if q.Archived {
		w.add("a.archived_at<>''")
	} else {
		w.add("a.archived_at=''")
	}
	if q.Q != "" {
		p := likeContains(q.Q)
		w.add(`(lower(c.name) LIKE ? ESCAPE '\' OR lower(r.title) LIKE ? ESCAPE '\' OR lower(a.notes) LIKE ? ESCAPE '\' OR lower(r.location) LIKE ? ESCAPE '\' OR lower(a.next_step) LIKE ? ESCAPE '\')`, p, p, p, p, p)
	}
	countSQL := `SELECT COUNT(*) FROM applications a JOIN roles r ON r.id = a.role_id JOIN companies c ON c.id = r.company_id LEFT JOIN posting_checks pc ON pc.role_id = r.id ` + w.sql()
	countArgs := append([]any{}, w.args...)
	if q.Cursor != nil && q.Cursor.ID != "" {
		w.add(cursorSQL(expr, "a.id", desc), q.Cursor.K, q.Cursor.K, q.Cursor.ID)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 51
	}
	rows, err := s.query(ctx, appSelect()+` `+w.sql()+` `+orderSQL(expr, "a.id", desc)+` LIMIT ?`, append(w.args, limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.Application
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	ids := make([]string, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	names, err := s.TagNames(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range out {
		if tags := names[out[i].ID]; tags != nil {
			out[i].Tags = tags
		}
	}
	fields, err := s.FieldValues(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range out {
		if v := fields[out[i].ID]; len(v) > 0 {
			out[i].CustomFields = v
		}
	}
	if err := s.attachInterviews(ctx, out); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.queryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.Application{}
	}
	return out, total, nil
}

// listApplicationsByField sorts by a custom field's value instead of a
// built-in column. It shares the same filters as ListApplications but,
// unlike it, has no cursor/keyset pagination — a field-sorted list is a
// single page (LIMIT only), since keyset pagination over an arbitrary
// joined column isn't worth the complexity here.
func (s *Store) listApplicationsByField(ctx context.Context, userID string, q domain.ListQuery, fieldName string, desc bool) ([]domain.Application, int, error) {
	field, err := s.GetFieldByName(ctx, userID, fieldName)
	if err != nil {
		return nil, 0, err
	}
	orderCol := "v.value"
	if field.Type == domain.FieldNumber {
		orderCol = "v.value_number"
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}

	w := where{}
	w.add("a.user_id=?", userID)
	if len(q.Status) > 0 {
		cond, args := statusIn(q.Status, false)
		w.add(cond, args...)
	}
	if len(q.StatusExclude) > 0 {
		cond, args := statusIn(q.StatusExclude, true)
		w.add(cond, args...)
	}
	if q.CompanyID != "" {
		w.add("r.company_id=?", q.CompanyID)
	}
	if q.Company != "" {
		w.add("lower(c.name)=lower(?)", q.Company)
	}
	if q.Fit != "" {
		w.add("r.fit=?", q.Fit)
	}
	if q.Source != "" {
		w.add("r.source=?", q.Source)
	}
	if q.Tag != "" {
		w.add(`EXISTS (SELECT 1 FROM application_tags at JOIN tags t ON t.id = at.tag_id WHERE at.application_id = a.id AND lower(t.name)=lower(?))`, q.Tag)
	}
	if q.Stale {
		w.add("pc.status=?", domain.LinkClosed)
	}
	if q.Archived {
		w.add("a.archived_at<>''")
	} else {
		w.add("a.archived_at=''")
	}
	if q.Q != "" {
		p := likeContains(q.Q)
		w.add(`(lower(c.name) LIKE ? ESCAPE '\' OR lower(r.title) LIKE ? ESCAPE '\' OR lower(a.notes) LIKE ? ESCAPE '\' OR lower(r.location) LIKE ? ESCAPE '\' OR lower(a.next_step) LIKE ? ESCAPE '\')`, p, p, p, p, p)
	}

	joined := `applications a JOIN roles r ON r.id = a.role_id JOIN companies c ON c.id = r.company_id LEFT JOIN posting_checks pc ON pc.role_id = r.id LEFT JOIN custom_field_values v ON v.application_id = a.id AND v.field_id = ?`
	joinArgs := append([]any{field.ID}, w.args...)
	countSQL := `SELECT COUNT(*) FROM ` + joined + ` ` + w.sql()
	limit := q.Limit
	if limit <= 0 {
		limit = 51
	}
	selectSQL := `SELECT a.id, a.user_id, a.role_id, a.status, a.outcome, a.applied_date, a.next_step, a.notes, a.resume_version, a.resume_file_ref, a.contact_email, a.external_id, a.archived_at, a.version, a.created_at, a.updated_at, r.title, r.location, r.salary_text, r.salary_min_cents, r.salary_max_cents, r.fit, r.posting_url, r.source, r.job_type, r.company_id, c.name, COALESCE(pc.status, ''), pc.checked_at FROM ` +
		joined + ` ` + w.sql() + ` ORDER BY ` + orderCol + ` IS NULL, ` + orderCol + ` ` + dir + `, a.id LIMIT ?`
	rows, err := s.query(ctx, selectSQL, append(joinArgs, limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.Application
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	ids := make([]string, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	names, err := s.TagNames(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	fields, err := s.FieldValues(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range out {
		if tags := names[out[i].ID]; tags != nil {
			out[i].Tags = tags
		}
		if v := fields[out[i].ID]; len(v) > 0 {
			out[i].CustomFields = v
		}
	}
	if err := s.attachInterviews(ctx, out); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.queryRow(ctx, countSQL, joinArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.Application{}
	}
	return out, total, nil
}

func (s *Store) ListApplicationsForExport(ctx context.Context, userID string) ([]domain.Application, error) {
	rows, err := s.query(ctx, appSelect()+` WHERE a.user_id=? ORDER BY a.created_at ASC, a.id ASC`, userID)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]string, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	names, err := s.TagNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if tags := names[out[i].ID]; tags != nil {
			out[i].Tags = tags
		}
	}
	if out == nil {
		out = []domain.Application{}
	}
	return out, nil
}

func (s *Store) UpdateApplication(ctx context.Context, a domain.Application) error {
	// An outcome keeps the stored stage; a stage replaces it and clears the outcome.
	outcome := ""
	if domain.IsOutcome(a.Status) {
		outcome = a.Status
	}
	res, err := s.exec(ctx, `UPDATE applications SET status=CASE WHEN ?='' THEN ? ELSE status END, outcome=?, applied_date=?, next_step=?, notes=?, resume_version=?, resume_file_ref=?, contact_email=?, version=version+1, updated_at=? WHERE id=? AND user_id=? AND version=?`,
		outcome, a.Status, outcome, nullString(a.AppliedDateValue()), a.NextStep, a.Notes, a.ResumeVersion, a.ResumeFileRef, a.ContactEmail, formatTime(a.UpdatedAt), a.ID, a.UserID, a.Version)
	if err != nil {
		return mapWriteErr(err)
	}
	return changed(res, func() (int, error) {
		cur, err := s.GetApplication(ctx, a.UserID, a.ID)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}, "application")
}

// SetApplicationArchived sets or clears archived_at. at is ignored when
// archived is false.
func (s *Store) SetApplicationArchived(ctx context.Context, userID, id string, version int, archived bool, at time.Time) error {
	value := ""
	if archived {
		value = formatTime(at)
	}
	res, err := s.exec(ctx, `UPDATE applications SET archived_at=?, version=version+1, updated_at=? WHERE id=? AND user_id=? AND version=?`,
		value, formatTime(at), id, userID, version)
	if err != nil {
		return mapWriteErr(err)
	}
	return changed(res, func() (int, error) {
		cur, err := s.GetApplication(ctx, userID, id)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}, "application")
}

func (s *Store) DeleteApplication(ctx context.Context, userID, id string, version int) error {
	res, err := s.exec(ctx, `DELETE FROM applications WHERE id=? AND user_id=? AND version=?`, id, userID, version)
	if err != nil {
		return mapWriteErr(err)
	}
	return changed(res, func() (int, error) {
		cur, err := s.GetApplication(ctx, userID, id)
		if err != nil {
			return 0, err
		}
		return cur.Version, nil
	}, "application")
}

func (s *Store) CreateEvent(ctx context.Context, e domain.Event) error {
	_, err := s.exec(ctx, `INSERT INTO events (id, user_id, application_id, kind, from_status, to_status, title, body, occurred_at, created_at, actor_token_id, actor_name) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.UserID, e.ApplicationID, e.Kind, e.FromStatus, e.ToStatus, e.Title, e.Body, formatTime(e.OccurredAt), formatTime(e.CreatedAt), e.Actor.TokenID, e.Actor.Name)
	return err
}

func (s *Store) ListEvents(ctx context.Context, userID string, q domain.ListQuery) ([]domain.Event, int, error) {
	expr, desc, _, err := PrepareSort("event", q.Sort, q.Cursor)
	if err != nil {
		return nil, 0, err
	}
	w := where{}
	w.add("a.user_id=?", userID)
	if q.ApplicationID != "" {
		w.add("e.application_id=?", q.ApplicationID)
	}
	if q.AfterTime != "" && q.AfterID != "" {
		w.add("(e.occurred_at > ? OR (e.occurred_at = ? AND e.id > ?))", q.AfterTime, q.AfterTime, q.AfterID)
	}
	countSQL := `SELECT COUNT(*) FROM events e JOIN applications a ON a.id = e.application_id ` + w.sql()
	countArgs := append([]any{}, w.args...)
	if q.Cursor != nil && q.Cursor.ID != "" {
		w.add(cursorSQL(expr, "e.id", desc), q.Cursor.K, q.Cursor.K, q.Cursor.ID)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 51
	}
	rows, err := s.query(ctx, `SELECT e.id, e.user_id, e.application_id, e.kind, e.from_status, e.to_status, e.title, e.body, e.occurred_at, e.created_at, e.actor_token_id, e.actor_name FROM events e JOIN applications a ON a.id = e.application_id `+w.sql()+` `+orderSQL(expr, "e.id", desc)+` LIMIT ?`, append(w.args, limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.queryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.Event{}
	}
	return out, total, nil
}

func scanEvent(row scanner) (domain.Event, error) {
	var e domain.Event
	var occurred, created string
	err := row.Scan(&e.ID, &e.UserID, &e.ApplicationID, &e.Kind, &e.FromStatus, &e.ToStatus, &e.Title, &e.Body, &occurred, &created, &e.Actor.TokenID, &e.Actor.Name)
	if err != nil {
		return domain.Event{}, err
	}
	if e.OccurredAt, err = parseTime(occurred); err != nil {
		return domain.Event{}, err
	}
	e.CreatedAt, err = parseTime(created)
	return e, err
}

func (s *Store) CreateTag(ctx context.Context, t domain.Tag) error {
	_, err := s.exec(ctx, `INSERT INTO tags (id, user_id, name, created_at) VALUES (?, ?, ?, ?)`, t.ID, t.UserID, t.Name, formatTime(t.CreatedAt))
	return mapWriteErr(err)
}

func scanTag(row scanner) (domain.Tag, error) {
	var t domain.Tag
	var created string
	err := row.Scan(&t.ID, &t.UserID, &t.Name, &created)
	if err != nil {
		return domain.Tag{}, err
	}
	t.CreatedAt, err = parseTime(created)
	return t, err
}

func (s *Store) GetTag(ctx context.Context, userID, id string) (domain.Tag, error) {
	t, err := scanTag(s.queryRow(ctx, `SELECT id, user_id, name, created_at FROM tags WHERE user_id=? AND id=?`, userID, id))
	if err == sql.ErrNoRows {
		return domain.Tag{}, domain.NotFound("tag")
	}
	return t, err
}

func (s *Store) GetTagByName(ctx context.Context, userID, name string) (domain.Tag, error) {
	t, err := scanTag(s.queryRow(ctx, `SELECT id, user_id, name, created_at FROM tags WHERE user_id=? AND lower(name)=lower(?)`, userID, name))
	if err == sql.ErrNoRows {
		return domain.Tag{}, domain.NotFound("tag")
	}
	return t, err
}

func (s *Store) ListTags(ctx context.Context, userID string, q domain.ListQuery) ([]domain.Tag, int, error) {
	expr, desc, _, err := PrepareSort("tag", q.Sort, q.Cursor)
	if err != nil {
		return nil, 0, err
	}
	w := where{}
	w.add("user_id=?", userID)
	countArgs := []any{userID}
	if q.Q != "" {
		p := likeContains(q.Q)
		w.add(`lower(name) LIKE ? ESCAPE '\'`, p)
		countArgs = append(countArgs, p)
	}
	countWhere := w.sql()
	if q.Cursor != nil && q.Cursor.ID != "" {
		w.add(cursorSQL(expr, "id", desc), q.Cursor.K, q.Cursor.K, q.Cursor.ID)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 51
	}
	rows, err := s.query(ctx, `SELECT id, user_id, name, created_at FROM tags `+w.sql()+` `+orderSQL(expr, "id", desc)+` LIMIT ?`, append(w.args, limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.Tag
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.queryRow(ctx, `SELECT COUNT(*) FROM tags `+countWhere, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.Tag{}
	}
	return out, total, nil
}

func (s *Store) DeleteTag(ctx context.Context, userID, id string) error {
	res, err := s.exec(ctx, `DELETE FROM tags WHERE user_id=? AND id=?`, userID, id)
	if err != nil {
		return mapWriteErr(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.NotFound("tag")
	}
	return nil
}

func (s *Store) AttachTag(ctx context.Context, appID, tagID string) error {
	_, err := s.exec(ctx, `INSERT INTO application_tags (application_id, tag_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, appID, tagID)
	return err
}

func (s *Store) DetachTag(ctx context.Context, appID, tagID string) error {
	res, err := s.exec(ctx, `DELETE FROM application_tags WHERE application_id=? AND tag_id=?`, appID, tagID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.NotFound("application tag")
	}
	return nil
}

func (s *Store) CreateField(ctx context.Context, f domain.CustomField) error {
	_, err := s.exec(ctx, `INSERT INTO custom_fields (id, user_id, name, type, created_at) VALUES (?, ?, ?, ?, ?)`,
		f.ID, f.UserID, f.Name, f.Type, formatTime(f.CreatedAt))
	return mapWriteErr(err)
}

func scanField(row scanner) (domain.CustomField, error) {
	var f domain.CustomField
	var created string
	err := row.Scan(&f.ID, &f.UserID, &f.Name, &f.Type, &created)
	if err != nil {
		return domain.CustomField{}, err
	}
	f.CreatedAt, err = parseTime(created)
	return f, err
}

func (s *Store) GetField(ctx context.Context, userID, id string) (domain.CustomField, error) {
	f, err := scanField(s.queryRow(ctx, `SELECT id, user_id, name, type, created_at FROM custom_fields WHERE user_id=? AND id=?`, userID, id))
	if err == sql.ErrNoRows {
		return domain.CustomField{}, domain.NotFound("field")
	}
	return f, err
}

func (s *Store) GetFieldByName(ctx context.Context, userID, name string) (domain.CustomField, error) {
	f, err := scanField(s.queryRow(ctx, `SELECT id, user_id, name, type, created_at FROM custom_fields WHERE user_id=? AND lower(name)=lower(?)`, userID, name))
	if err == sql.ErrNoRows {
		return domain.CustomField{}, domain.NotFound("field")
	}
	return f, err
}

func (s *Store) ListFields(ctx context.Context, userID string, q domain.ListQuery) ([]domain.CustomField, int, error) {
	expr, desc, _, err := PrepareSort("field", q.Sort, q.Cursor)
	if err != nil {
		return nil, 0, err
	}
	w := where{}
	w.add("user_id=?", userID)
	countArgs := []any{userID}
	if q.Q != "" {
		p := likeContains(q.Q)
		w.add(`lower(name) LIKE ? ESCAPE '\'`, p)
		countArgs = append(countArgs, p)
	}
	countWhere := w.sql()
	if q.Cursor != nil && q.Cursor.ID != "" {
		w.add(cursorSQL(expr, "id", desc), q.Cursor.K, q.Cursor.K, q.Cursor.ID)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 51
	}
	rows, err := s.query(ctx, `SELECT id, user_id, name, type, created_at FROM custom_fields `+w.sql()+` `+orderSQL(expr, "id", desc)+` LIMIT ?`, append(w.args, limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.CustomField
	for rows.Next() {
		f, err := scanField(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.queryRow(ctx, `SELECT COUNT(*) FROM custom_fields `+countWhere, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.CustomField{}
	}
	return out, total, nil
}

func (s *Store) DeleteField(ctx context.Context, userID, id string) error {
	res, err := s.exec(ctx, `DELETE FROM custom_fields WHERE user_id=? AND id=?`, userID, id)
	if err != nil {
		return mapWriteErr(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.NotFound("field")
	}
	return nil
}

// SetFieldValue upserts a field's value for an application. valueNumber is
// used only for numeric-typed fields, to make sorting on them cheap; pass
// nil for text and date fields.
func (s *Store) SetFieldValue(ctx context.Context, appID, fieldID, value string, valueNumber *float64) error {
	_, err := s.exec(ctx, `INSERT INTO custom_field_values (application_id, field_id, value, value_number) VALUES (?, ?, ?, ?)
		ON CONFLICT (application_id, field_id) DO UPDATE SET value=excluded.value, value_number=excluded.value_number`,
		appID, fieldID, value, valueNumber)
	return mapWriteErr(err)
}

// FieldValues batches a name->value map per application, the same pattern
// TagNames uses for tags.
func (s *Store) FieldValues(ctx context.Context, appIDs []string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if len(appIDs) == 0 {
		return out, nil
	}
	ph := strings.TrimRight(strings.Repeat("?,", len(appIDs)), ",")
	args := make([]any, len(appIDs))
	for i, id := range appIDs {
		args[i] = id
	}
	rows, err := s.query(ctx, `SELECT v.application_id, f.name, v.value FROM custom_field_values v JOIN custom_fields f ON f.id = v.field_id WHERE v.application_id IN (`+ph+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var appID, name, value string
		if err := rows.Scan(&appID, &name, &value); err != nil {
			return nil, err
		}
		if out[appID] == nil {
			out[appID] = map[string]string{}
		}
		out[appID][name] = value
	}
	return out, rows.Err()
}

func (s *Store) TagNames(ctx context.Context, appIDs []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(appIDs) == 0 {
		return out, nil
	}
	ph := strings.TrimRight(strings.Repeat("?,", len(appIDs)), ",")
	args := make([]any, len(appIDs))
	for i, id := range appIDs {
		args[i] = id
	}
	rows, err := s.query(ctx, `SELECT at.application_id, t.name FROM application_tags at JOIN tags t ON t.id = at.tag_id WHERE at.application_id IN (`+ph+`) ORDER BY t.name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = append(out[id], name)
	}
	return out, rows.Err()
}

func (s *Store) CreateAudit(ctx context.Context, a domain.Audit) error {
	meta := ""
	if len(a.Metadata) > 0 {
		meta = string(a.Metadata)
	}
	_, err := s.exec(ctx, `INSERT INTO audit_log (id, user_id, token_id, actor_name, action, entity_type, entity_id, summary, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.UserID, a.TokenID, a.ActorName, a.Action, a.EntityType, a.EntityID, a.Summary, meta, formatTime(a.CreatedAt))
	return err
}

func (s *Store) ListAudit(ctx context.Context, userID string, q domain.ListQuery) ([]domain.Audit, int, error) {
	expr, desc, _, err := PrepareSort("audit", q.Sort, q.Cursor)
	if err != nil {
		return nil, 0, err
	}
	w := where{}
	w.add("user_id=?", userID)
	if q.Actor != "" {
		w.add("lower(actor_name)=lower(?)", q.Actor)
	}
	if q.Action != "" {
		w.add("action=?", q.Action)
	}
	if q.EntityType != "" {
		w.add("entity_type=?", q.EntityType)
	}
	if q.EntityID != "" {
		w.add("entity_id=?", q.EntityID)
	}
	if q.Since != "" {
		w.add("created_at>=?", q.Since)
	}
	if q.Until != "" {
		w.add("created_at<=?", q.Until)
	}
	if q.Q != "" {
		p := likeContains(q.Q)
		w.add(`(lower(summary) LIKE ? ESCAPE '\' OR lower(action) LIKE ? ESCAPE '\' OR lower(entity_type) LIKE ? ESCAPE '\' OR lower(entity_id) LIKE ? ESCAPE '\' OR lower(actor_name) LIKE ? ESCAPE '\' OR lower(metadata) LIKE ? ESCAPE '\')`, p, p, p, p, p, p)
	}
	countWhere := w.sql()
	countArgs := append([]any{}, w.args...)
	if q.Cursor != nil && q.Cursor.ID != "" {
		w.add(cursorSQL(expr, "id", desc), q.Cursor.K, q.Cursor.K, q.Cursor.ID)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 51
	}
	rows, err := s.query(ctx, `SELECT id, user_id, token_id, actor_name, action, entity_type, entity_id, summary, metadata, created_at FROM audit_log `+w.sql()+` `+orderSQL(expr, "id", desc)+` LIMIT ?`, append(w.args, limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.Audit
	for rows.Next() {
		var a domain.Audit
		var meta, created string
		if err := rows.Scan(&a.ID, &a.UserID, &a.TokenID, &a.ActorName, &a.Action, &a.EntityType, &a.EntityID, &a.Summary, &meta, &created); err != nil {
			return nil, 0, err
		}
		if meta != "" {
			a.Metadata = json.RawMessage(meta)
		}
		var perr error
		if a.CreatedAt, perr = parseTime(created); perr != nil {
			return nil, 0, perr
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.queryRow(ctx, `SELECT COUNT(*) FROM audit_log `+countWhere, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []domain.Audit{}
	}
	return out, total, nil
}

type IdemRecord struct {
	TokenID      string
	UserID       string
	Key          string
	Method       string
	Path         string
	RequestHash  string
	StatusCode   int
	ResponseBody string
	CreatedAt    string
}

func (s *Store) GetIdempotency(ctx context.Context, tokenID, key string) (IdemRecord, bool, error) {
	var rec IdemRecord
	err := s.queryRow(ctx, `SELECT token_id, user_id, idem_key, method, path, request_hash, status_code, response_body, created_at FROM idempotency_keys WHERE token_id=? AND idem_key=?`, tokenID, key).
		Scan(&rec.TokenID, &rec.UserID, &rec.Key, &rec.Method, &rec.Path, &rec.RequestHash, &rec.StatusCode, &rec.ResponseBody, &rec.CreatedAt)
	if err == sql.ErrNoRows {
		return IdemRecord{}, false, nil
	}
	if err != nil {
		return IdemRecord{}, false, err
	}
	return rec, true, nil
}

func (s *Store) InsertIdempotency(ctx context.Context, rec IdemRecord) error {
	_, err := s.exec(ctx, `INSERT INTO idempotency_keys (token_id, user_id, idem_key, method, path, request_hash, status_code, response_body, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.TokenID, rec.UserID, rec.Key, rec.Method, rec.Path, rec.RequestHash, rec.StatusCode, rec.ResponseBody, rec.CreatedAt)
	if err != nil && isUnique(err) {
		return ErrIdempotencyRace
	}
	return err
}

func (s *Store) FinishIdempotency(ctx context.Context, tokenID, key string, status int, body string) error {
	_, err := s.exec(ctx, `UPDATE idempotency_keys SET status_code=?, response_body=? WHERE token_id=? AND idem_key=?`, status, body, tokenID, key)
	return err
}
