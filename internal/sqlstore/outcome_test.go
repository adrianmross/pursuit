package sqlstore

import (
	"context"
	"testing"
)

// Migration 006 moves outcomes off the status column and restores the stage
// the application was in from the status change that led into the outcome.
func TestMigrateOutcomeBackfill(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.db.ExecContext(ctx, rebind(s.dialect, q), args...); err != nil {
			t.Fatal(err)
		}
	}
	ts := "2026-01-01T00:00:00Z"
	exec(`INSERT INTO users (id, name, email, created_at) VALUES ('u','A','a@localhost',?)`, ts)
	exec(`INSERT INTO companies (id, user_id, name, created_at, updated_at) VALUES ('c','u','Co',?,?)`, ts, ts)
	exec(`INSERT INTO roles (id, user_id, company_id, title, created_at, updated_at) VALUES ('r','u','c','T',?,?)`, ts, ts)
	for id, status := range map[string]string{"a1": "rejected", "a2": "ghosted"} {
		exec(`INSERT INTO applications (id, user_id, role_id, status, applied_date, created_at, updated_at) VALUES (?, 'u','r',?, '2026-01-01', ?, ?)`, id, status, ts, ts)
	}
	exec(`INSERT INTO events (id, user_id, application_id, kind, from_status, to_status, occurred_at, created_at) VALUES ('e1','u','a1','status_change','interviewing','rejected',?,?)`, ts, ts)

	exec(migrations[5].stmts[1])
	for id, want := range map[string][2]string{"a1": {"interviewing", "rejected"}, "a2": {"applied", "ghosted"}} {
		var status, outcome string
		if err := s.db.QueryRowContext(ctx, rebind(s.dialect, `SELECT status, outcome FROM applications WHERE id=?`), id).Scan(&status, &outcome); err != nil {
			t.Fatal(err)
		}
		if status != want[0] || outcome != want[1] {
			t.Fatalf("%s: got %s/%s want %s/%s", id, status, outcome, want[0], want[1])
		}
	}
}
