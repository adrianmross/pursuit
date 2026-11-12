// Package sqlstore persists Pursuit in Postgres or SQLite.
// SQLite is the zero-dependency daemon default. Postgres is the multi-replica target.
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/platform/sqldb"
)

// ErrIdempotencyRace means another request claimed the key first.
var ErrIdempotencyRace = errors.New("idempotency race")

// Store is Pursuit's tables on a sqldb.DB. db and dialect repeat the core's
// fields for the queries that branch on them.
type Store struct {
	core    *sqldb.DB
	db      *sql.DB
	dialect string
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		path, err := defaultSQLitePath("")
		if err != nil {
			return nil, err
		}
		databaseURL = "sqlite://" + path
	}
	core, err := sqldb.Open(databaseURL)
	if err != nil {
		return nil, err
	}
	core.LockKey = advisoryLockKey
	if err := Migrate(ctx, core.SQL, core.Dialect); err != nil {
		core.SQL.Close()
		return nil, err
	}
	return &Store{core: core, db: core.SQL, dialect: core.Dialect}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Dialect() string { return s.dialect }

func (s *Store) Ping(ctx context.Context) error { return s.core.Ping(ctx) }

var (
	WithDryRun = sqldb.WithDryRun
	IsDryRun   = sqldb.IsDryRun
	rebind     = sqldb.Rebind
	isUnique   = sqldb.IsUnique
	isFK       = sqldb.IsFK
)

func (s *Store) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return s.core.WithTx(ctx, fn)
}

// WithLock holds the migration/bootstrap lock for fn.
// Both dialects run the callback inside one transaction.
func (s *Store) WithLock(ctx context.Context, fn func(context.Context) error) error {
	return s.core.WithLock(ctx, fn)
}

func (s *Store) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.core.Exec(ctx, query, args...)
}

func (s *Store) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.core.Query(ctx, query, args...)
}

func (s *Store) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.core.QueryRow(ctx, query, args...)
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
