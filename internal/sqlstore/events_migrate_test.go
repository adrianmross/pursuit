package sqlstore

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/adrianmross/appkit/sqldb"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore/sqlstoretest"
)

// Migration 030 rebuilds events around a subject. Events written under the
// old schema must read back exactly as they were.
func TestMigrateEventsToSubject(t *testing.T) {
	ctx := context.Background()
	s := openBefore(t, "030")
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.db.ExecContext(ctx, rebind(s.dialect, q), args...); err != nil {
			t.Fatal(err)
		}
	}
	ts := "2026-01-01T00:00:00Z"
	for _, u := range []string{"u", "v"} {
		exec(`INSERT INTO users (id, name, email, created_at) VALUES (?, 'A', ?, ?)`, u, u+"@localhost", ts)
		exec(`INSERT INTO companies (id, user_id, name, created_at, updated_at) VALUES (?, ?, 'Co', ?, ?)`, "c"+u, u, ts, ts)
		exec(`INSERT INTO roles (id, user_id, company_id, title, created_at, updated_at) VALUES (?, ?, ?, 'T', ?, ?)`, "r"+u, u, "c"+u, ts, ts)
	}
	for id, u := range map[string]string{"a1": "u", "a2": "u", "b1": "v"} {
		exec(`INSERT INTO applications (id, user_id, role_id, status, created_at, updated_at) VALUES (?, ?, ?, 'applied', ?, ?)`, id, u, "r"+u, ts, ts)
	}
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	want := []domain.Event{
		{ID: "e1", UserID: "u", ApplicationID: "a1", Kind: "status_change", FromStatus: "applied", ToStatus: "interviewing", Title: "Status changed",
			OccurredAt: at("2026-01-02T00:00:00Z"), CreatedAt: at("2026-01-02T00:00:01Z"), Actor: domain.ActorRef{TokenID: "t1", Name: "Ada"}},
		{ID: "e2", UserID: "u", ApplicationID: "a1", Kind: "note", Title: `He said "hi"`, Body: "line one\nback\\slash {\"x\":1}",
			OccurredAt: at("2026-01-03T00:00:00Z"), CreatedAt: at("2026-01-03T00:00:00Z")},
		{ID: "e3", UserID: "u", ApplicationID: "a2", Kind: "link_check", ToStatus: `od"d\`, Title: "Posting closed",
			OccurredAt: at("2026-01-04T00:00:00Z"), CreatedAt: at("2026-01-04T00:00:00Z")},
		{ID: "e4", UserID: "v", ApplicationID: "b1", Kind: "status_change", FromStatus: "to_apply", ToStatus: "applied",
			OccurredAt: at("2026-01-05T00:00:00Z"), CreatedAt: at("2026-01-05T00:00:00Z")},
	}
	for _, e := range want {
		exec(`INSERT INTO events (id, user_id, application_id, kind, from_status, to_status, title, body, occurred_at, created_at, actor_token_id, actor_name) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID, e.UserID, e.ApplicationID, e.Kind, e.FromStatus, e.ToStatus, e.Title, e.Body, formatTime(e.OccurredAt), formatTime(e.CreatedAt), e.Actor.TokenID, e.Actor.Name)
	}

	if err := Migrate(ctx, s.db, s.dialect); err != nil {
		t.Fatal(err)
	}

	for _, w := range want {
		got, err := s.GetEvent(ctx, w.UserID, w.ID)
		if err != nil {
			t.Fatalf("%s: %v", w.ID, err)
		}
		if !reflect.DeepEqual(got, w) {
			t.Fatalf("%s:\n got %+v\nwant %+v", w.ID, got, w)
		}
	}
	if _, err := s.GetEvent(ctx, "v", "e1"); !domain.IsCode(err, "not_found") {
		t.Fatalf("another user's event: %v", err)
	}
	list, total, err := s.ListEvents(ctx, "u", domain.ListQuery{Sort: "occurred_at"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || !reflect.DeepEqual(list, want[:3]) {
		t.Fatalf("list: total %d, %+v", total, list)
	}
	list, total, err = s.ListEvents(ctx, "u", domain.ListQuery{Sort: "occurred_at", ApplicationID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || !reflect.DeepEqual(list, want[:2]) {
		t.Fatalf("application list: total %d, %+v", total, list)
	}

	// A new event round-trips, and deleting an application still takes its
	// events with it now that no foreign key does.
	n := domain.Event{ID: "e5", UserID: "u", ApplicationID: "a2", Kind: "status_change", FromStatus: "applied", ToStatus: "offer",
		OccurredAt: at("2026-01-06T00:00:00Z"), CreatedAt: at("2026-01-06T00:00:00Z")}
	if err := s.CreateEvent(ctx, n); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetEvent(ctx, "u", "e5"); err != nil || !reflect.DeepEqual(got, n) {
		t.Fatalf("new event: %+v %v", got, err)
	}
	app, err := s.GetApplication(ctx, "u", "a1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteApplication(ctx, "u", "a1", app.Version); err != nil {
		t.Fatal(err)
	}
	list, total, err = s.ListEvents(ctx, "u", domain.ListQuery{Sort: "occurred_at"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 || list[0].ID != "e3" || list[1].ID != "e5" {
		t.Fatalf("after delete: total %d, %+v", total, list)
	}
}

// openBefore opens a database migrated up to, but not including, version.
func openBefore(t *testing.T, version string) *Store {
	t.Helper()
	core, err := sqldb.Open(sqlstoretest.DSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { core.SQL.Close() })
	core.LockKey = advisoryLockKey
	n := 0
	for n < len(migrations) && migrations[n].version != version {
		n++
	}
	if n == len(migrations) {
		t.Fatalf("no migration %s", version)
	}
	if err := migrateTo(context.Background(), core.SQL, core.Dialect, migrations[:n]); err != nil {
		t.Fatal(err)
	}
	return &Store{core: core, db: core.SQL, dialect: core.Dialect}
}
