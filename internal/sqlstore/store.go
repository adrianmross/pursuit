// Package sqlstore persists Pursuit in Postgres or SQLite.
// SQLite is the zero-dependency daemon default. Postgres is the multi-replica target.
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/pursuit/pursuit/internal/domain"
)

type ctxKey int

const (
	keyTx ctxKey = iota + 1
	keyConn
	keyDry
)

// ErrIdempotencyRace means another request claimed the key first.
var ErrIdempotencyRace = errors.New("idempotency race")

type Store struct {
	db      *sql.DB
	dialect string
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	db, dialect, err := openDB(databaseURL)
	if err != nil {
		return nil, err
	}
	if err := Migrate(ctx, db, dialect); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, dialect: dialect}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Dialect() string { return s.dialect }

func (s *Store) Ping(ctx context.Context) error {
	var n int
	return s.queryRow(ctx, "SELECT 1").Scan(&n)
}

func WithDryRun(ctx context.Context) context.Context {
	return context.WithValue(ctx, keyDry, true)
}

func IsDryRun(ctx context.Context) bool {
	v, _ := ctx.Value(keyDry).(bool)
	return v
}

func (s *Store) WithTx(ctx context.Context, fn func(context.Context) error) error {
	if ctx.Value(keyTx) != nil || ctx.Value(keyConn) != nil {
		return fn(ctx)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	err = fn(context.WithValue(ctx, keyTx, tx))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if IsDryRun(ctx) {
		return tx.Rollback()
	}
	return tx.Commit()
}

// WithLock holds the migration/bootstrap lock for fn.
// On SQLite the callback runs inside an immediate transaction on one connection.
func (s *Store) WithLock(ctx context.Context, fn func(context.Context) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if s.dialect == "postgres" {
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
			return err
		}
		defer conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockKey)
		return fn(context.WithValue(ctx, keyConn, conn))
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	err = fn(context.WithValue(ctx, keyConn, conn))
	if err != nil {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		return err
	}
	_, err = conn.ExecContext(ctx, "COMMIT")
	return err
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *Store) ex(ctx context.Context) execer {
	if tx, ok := ctx.Value(keyTx).(*sql.Tx); ok && tx != nil {
		return tx
	}
	if c, ok := ctx.Value(keyConn).(*sql.Conn); ok && c != nil {
		return c
	}
	return s.db
}

func (s *Store) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.ex(ctx).ExecContext(ctx, rebind(s.dialect, query), args...)
}

func (s *Store) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.ex(ctx).QueryContext(ctx, rebind(s.dialect, query), args...)
}

func (s *Store) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.ex(ctx).QueryRowContext(ctx, rebind(s.dialect, query), args...)
}

func rebind(dialect, query string) string {
	if dialect != "postgres" {
		return query
	}
	var b strings.Builder
	n := 1
	for _, r := range query {
		if r == '?' {
			b.WriteByte('$')
			b.WriteString(fmt.Sprint(n))
			n++
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func openDB(raw string) (*sql.DB, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		path, err := defaultSQLitePath("")
		if err != nil {
			return nil, "", err
		}
		raw = "sqlite://" + path
	}
	if strings.HasPrefix(raw, "postgres://") || strings.HasPrefix(raw, "postgresql://") {
		db, err := sql.Open("pgx", raw)
		if err != nil {
			return nil, "", err
		}
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(30 * time.Minute)
		return db, "postgres", nil
	}
	path, err := sqlitePath(raw)
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, "", err
	}
	dsn := sqliteDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, "", err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	return db, "sqlite", nil
}

func defaultSQLitePath(dataDir string) (string, error) {
	if dataDir == "" {
		if v := strings.TrimSpace(os.Getenv("PURSUIT_DATA_DIR")); v != "" {
			dataDir = v
		}
	}
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataDir = filepath.Join(home, ".local", "share", "pursuit")
	}
	return filepath.Join(dataDir, "pursuit.db"), nil
}

func sqlitePath(raw string) (string, error) {
	path := raw
	switch {
	case strings.HasPrefix(raw, "sqlite://"):
		path = strings.TrimPrefix(raw, "sqlite://")
	case strings.HasPrefix(raw, "sqlite:"):
		path = strings.TrimPrefix(raw, "sqlite:")
	case strings.HasPrefix(raw, "file:"):
		u, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		path = u.Path
	default:
		return "", fmt.Errorf("unsupported database url %q", raw)
	}
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func sqliteDSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_txlock", "immediate")
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: q.Encode()}
	return u.String()
}

func mapWriteErr(err error) error {
	if err == nil {
		return nil
	}
	if isUnique(err) {
		return domain.Conflict("unique constraint violated", "reuse the existing resource or choose a different value")
	}
	if isFK(err) {
		return domain.Conflict("resource is still referenced", "delete dependent roles or applications first")
	}
	return err
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

func isFK(err error) bool {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "foreign key")
}

func formatTime(t time.Time) string { return domain.FormatTime(t) }

func parseTime(s string) (time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, nil
	}
	return domain.ParseTime(s)
}

func optTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid || strings.TrimSpace(s.String) == "" {
		return nil, nil
	}
	t, err := parseTime(s.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func optInt(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func nullInt(n *int64) any {
	if n == nil {
		return nil
	}
	return *n
}

func nullString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

type where struct {
	clauses []string
	args    []any
}

func (w *where) add(clause string, args ...any) {
	w.clauses = append(w.clauses, clause)
	w.args = append(w.args, args...)
}

func (w *where) sql() string {
	if len(w.clauses) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(w.clauses, " AND ")
}

func likeContains(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return "%" + strings.ToLower(s) + "%"
}

func resolveSort(cols map[string]string, sort, def string) (expr string, desc bool, canon string, err error) {
	if strings.TrimSpace(sort) == "" {
		sort = def
	}
	desc = strings.HasPrefix(sort, "-")
	name := strings.TrimPrefix(sort, "-")
	expr, ok := cols[name]
	if !ok || name == "" {
		return "", false, "", domain.Validation("invalid sort %q", sort).WithHint("sort is a field name, with an optional leading - for descending")
	}
	if desc {
		return expr, true, "-" + name, nil
	}
	return expr, false, name, nil
}

func cursorSQL(expr, idExpr string, desc bool) string {
	op := ">"
	if desc {
		op = "<"
	}
	return fmt.Sprintf("(%s %s ? OR (%s = ? AND %s %s ?))", expr, op, expr, idExpr, op)
}

func orderSQL(expr, idExpr string, desc bool) string {
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	return fmt.Sprintf("ORDER BY %s %s, %s %s", expr, dir, idExpr, dir)
}

func sortSpec(kind string) (map[string]string, string, error) {
	switch kind {
	case "company":
		return map[string]string{"name": "name", "updated_at": "updated_at", "created_at": "created_at"}, "name", nil
	case "role":
		return map[string]string{"title": "r.title", "company": "c.name", "updated_at": "r.updated_at", "created_at": "r.created_at"}, "title", nil
	case "application":
		return map[string]string{
			"updated_at": "a.updated_at", "created_at": "a.created_at",
			"applied_date": "COALESCE(a.applied_date, '')", "company": "c.name", "status": "a.status", "title": "r.title",
		}, "-updated_at", nil
	case "link":
		return map[string]string{
			"checked_at": "pc.checked_at", "company": "c.name", "status": "pc.status",
		}, "-checked_at", nil
	case "event":
		return map[string]string{"occurred_at": "e.occurred_at", "created_at": "e.created_at"}, "-occurred_at", nil
	case "audit":
		return map[string]string{"created_at": "created_at"}, "-created_at", nil
	case "tag":
		return map[string]string{"name": "name", "created_at": "created_at"}, "name", nil
	case "field":
		return map[string]string{"name": "name", "created_at": "created_at"}, "name", nil
	case "token":
		return map[string]string{"created_at": "created_at", "name": "name"}, "-created_at", nil
	default:
		return nil, "", fmt.Errorf("unknown sort kind %s", kind)
	}
}

// PrepareSort resolves a public sort name and checks an optional cursor.
func PrepareSort(kind, sort string, cur *domain.Cursor) (expr string, desc bool, canon string, err error) {
	cols, def, err := sortSpec(kind)
	if err != nil {
		return "", false, "", err
	}
	expr, desc, canon, err = resolveSort(cols, sort, def)
	if err != nil {
		return "", false, "", err
	}
	if cur != nil && cur.ID != "" {
		if err := domain.CheckCursorSort(*cur, canon); err != nil {
			return "", false, "", err
		}
	}
	return expr, desc, canon, nil
}

func changed(res sql.Result, probe func() (int, error), entity string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		return nil
	}
	cur, err := probe()
	if err != nil {
		return err
	}
	return domain.VersionConflict(entity, cur)
}
