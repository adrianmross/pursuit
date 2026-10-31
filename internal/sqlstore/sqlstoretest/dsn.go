// Package sqlstoretest picks the database a test runs against.
package sqlstoretest

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// DSN returns a fresh SQLite file, or a fresh schema on the Postgres server in
// PURSUIT_TEST_DATABASE_URL when that is set, so every test starts empty.
func DSN(t testing.TB) string {
	t.Helper()
	base := os.Getenv("PURSUIT_TEST_DATABASE_URL")
	if base == "" {
		return "sqlite://" + filepath.Join(t.TempDir(), "pursuit.db")
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	schema := "t_" + hex.EncodeToString(b)
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db, err := sql.Open("pgx", base)
		if err != nil {
			return
		}
		defer db.Close()
		_, _ = db.Exec("DROP SCHEMA " + schema + " CASCADE")
	})
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}
