package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pursuit/pursuit/internal/httpapi"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

func TestSchemaHelpAndExitCodes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"schema", "apps", "list"}, &stdout, &stderr); code != 0 {
		t.Fatalf("schema %d %s", code, stderr.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["name"] != "pursuit apps list" {
		t.Fatalf("name %#v", doc["name"])
	}
	flags, _ := doc["flags"].([]any)
	if len(flags) == 0 {
		t.Fatal("expected flags for apps list")
	}
	blob := stdout.String()
	if strings.Contains(blob, "check-links") || strings.Contains(blob, "queue claim") {
		t.Fatalf("schema dumped other commands: %s", blob)
	}
	if _, ok := doc["children"]; ok && doc["children"] != nil {
		t.Fatalf("leaf schema included children %#v", doc["children"])
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"schema"}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	var index struct {
		Commands []map[string]string `json:"commands"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &index); err != nil {
		t.Fatal(stdout.String(), err)
	}
	found := false
	for _, item := range index.Commands {
		if item["name"] == "pursuit apps list" {
			found = true
		}
	}
	if !found {
		t.Fatal("schema index missing apps list")
	}

	stdout.Reset()
	if code := Run([]string{"apps", "list", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	help := stdout.String()
	if !strings.Contains(help, "--status") || len(help) > 2500 {
		t.Fatalf("help not short or missing status:\n%s", help)
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"apps", "list", "--dry-run"}, &stdout, &stderr); code != 2 {
		t.Fatalf("dry-run on list exited %d %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), `"code":"usage"`) {
		t.Fatalf("stderr %s", stderr.String())
	}

	stderr.Reset()
	if code := Run([]string{"apps", "get"}, &stdout, &stderr); code != 2 {
		t.Fatalf("missing id exited %d", code)
	}

	stdout.Reset()
	stderr.Reset()
	cfg := filepath.Join(t.TempDir(), "empty.json")
	if err := osWrite(cfg, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"--config", cfg, "--json", "version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("version %d %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"version"`) {
		t.Fatalf("version out %s", stdout.String())
	}

	db, err := sqlstore.Open(context.Background(), "sqlite://"+filepath.Join(t.TempDir(), "cli.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	svc := service.New(db)
	secret := "pst_cli_test"
	if _, _, err := svc.Bootstrap(context.Background(), "Adrian", "adrian@localhost", "owner", secret); err != nil {
		t.Fatal(err)
	}
	api := &httpapi.Server{Svc: svc, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	api.Ready.Store(true)
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	stdout.Reset()
	stderr.Reset()
	code := Run([]string{"--server", srv.URL, "--token", secret, "--config", cfg, "apps", "get", "missing"}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("missing app exited %d %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), `"code":"not_found"`) {
		t.Fatalf("stderr %s", stderr.String())
	}
}

func TestImportCSVDirectDatabase(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "apps.csv")
	body := strings.Join([]string{
		"id,company,role,location,salary,fit,status,applied_date,next_step,posting_url,notes,last_updated",
		"ext-1,Acme,Engineer,Remote,$100k,Strong,Applied,2026-01-02,Follow up,https://example.com/jobs/1,first note,2026-01-03",
		"ext-2,Acme,Engineer,Remote,$120k,Strong,Prepared,2026-01-02,Send portfolio,https://example.com/jobs/1,updated note,2026-01-04",
		"",
	}, "\n")
	if err := os.WriteFile(csvPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(cfg, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dsn := "sqlite://" + filepath.Join(dir, "pursuit.db")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--config", cfg, "--json", "import", "csv", csvPath, "--database-url", dsn}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("import %d %s %s", code, stdout.String(), stderr.String())
	}
	var imported map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if imported["applications_created"] != float64(1) || imported["applications_updated"] != float64(1) {
		t.Fatalf("counts %#v", imported)
	}
	token, _ := imported["bootstrap_token"].(string)
	if !strings.HasPrefix(token, "pst_") {
		t.Fatalf("bootstrap token %#v", imported["bootstrap_token"])
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"--config", cfg, "--json", "import", "csv", csvPath, "--database-url", dsn}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("reimport %d %s", code, stderr.String())
	}
	again := map[string]any{}
	if err := json.Unmarshal(stdout.Bytes(), &again); err != nil {
		t.Fatal(err)
	}
	if again["applications_created"] != float64(0) || again["applications_updated"] != float64(2) {
		t.Fatalf("reimport %#v", again)
	}
	if _, ok := again["bootstrap_token"]; ok {
		t.Fatal("bootstrap token was issued again")
	}
	store, err := sqlstore.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	user, err := store.FirstUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	apps, err := store.ListApplicationsForExport(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].Status != "awaiting_review" || apps[0].NextStep != "Send portfolio" || apps[0].Notes != "updated note" {
		t.Fatalf("stored %#v", apps)
	}

	untouched := filepath.Join(dir, "untouched.db")
	t.Setenv("DATABASE_URL", "sqlite://"+untouched)
	t.Setenv("PURSUIT_DATABASE_URL", "")
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"--config", cfg, "--server", "http://127.0.0.1:1", "import", "csv", csvPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("http import succeeded %s", stdout.String())
	}
	if _, err := os.Stat(untouched); err == nil {
		t.Fatal("import opened DATABASE_URL without --database-url")
	}
}

func osWrite(path string, b []byte) error {
	return os.WriteFile(path, b, 0o644)
}
