package sqlstore

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
)

const migrationsGolden = "testdata/migrations.txt"

// A migration that has merged may already have run somewhere, and a database
// never re-runs a version it has recorded, so editing one silently forks the
// schema. Change the schema with a new migration instead. New versions are
// appended to the golden file with UPDATE_MIGRATIONS=1; recorded lines are
// never rewritten.
func TestMigrationsAreAppendOnly(t *testing.T) {
	raw, err := os.ReadFile(migrationsGolden)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	recorded := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if v, sum, ok := strings.Cut(line, " "); ok {
			recorded[v] = sum
		}
	}
	seen := map[string]bool{}
	var appended []string
	for i, m := range migrations {
		if i > 0 && m.version <= migrations[i-1].version {
			t.Errorf("migration %s comes after %s: versions must increase", m.version, migrations[i-1].version)
		}
		seen[m.version] = true
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(m.stmts, "\x00"))))
		want, ok := recorded[m.version]
		switch {
		case ok && want != sum:
			t.Errorf("migration %s changed after it merged; add a new migration instead", m.version)
		case !ok && os.Getenv("UPDATE_MIGRATIONS") == "1":
			appended = append(appended, m.version+" "+sum+"\n")
		case !ok:
			t.Errorf("migration %s is not in %s; run UPDATE_MIGRATIONS=1 go test ./internal/sqlstore/ -run TestMigrationsAreAppendOnly", m.version, migrationsGolden)
		}
	}
	for v := range recorded {
		if !seen[v] {
			t.Errorf("migration %s was removed", v)
		}
	}
	if len(appended) > 0 {
		f, err := os.OpenFile(migrationsGolden, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(strings.Join(appended, "")); err != nil {
			t.Fatal(err)
		}
	}
}
