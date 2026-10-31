package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pursuit/pursuit/internal/config"
	"github.com/pursuit/pursuit/internal/httpapi"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

type authEnv struct {
	srv   *httptest.Server
	owner string
	base  []string
	dir   string
	// now is the server's clock; the device test's sleep stub advances it.
	now time.Time
}

// newAuthEnv isolates HOME, forces file credential storage (no Keychain
// writes from tests), and clears env that would change the login path.
func newAuthEnv(t *testing.T) *authEnv {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("PURSUIT_CREDENTIALS", "")
	t.Setenv("PURSUIT_AUTH_SECRET_STORE", "file")
	t.Setenv("PURSUIT_TOKEN", "")
	t.Setenv("PURSUIT_SERVER", "")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_TTY", "")
	t.Setenv("DISPLAY", ":0")
	db, err := sqlstore.Open(context.Background(), "sqlite://"+filepath.Join(dir, "cli.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	svc := service.New(db)
	a := &authEnv{now: time.Now().UTC()}
	svc.Now = func() time.Time { return a.now }
	owner := "pst_cli_auth_owner"
	if _, _, err := svc.Bootstrap(context.Background(), "Adrian", "adrian@localhost", "owner", owner); err != nil {
		t.Fatal(err)
	}
	api := &httpapi.Server{Svc: svc, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	api.Ready.Store(true)
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(srv.Close)
	cfg := filepath.Join(dir, "empty.json")
	if err := osWrite(cfg, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	a.srv, a.owner, a.dir = srv, owner, dir
	a.base = []string{"--server", srv.URL, "--config", cfg, "--json"}
	return a
}

func (a *authEnv) run(t *testing.T, args ...string) (int, map[string]any, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append(append([]string{}, a.base...), args...), &stdout, &stderr)
	var doc map[string]any
	_ = json.Unmarshal(stdout.Bytes(), &doc)
	return code, doc, stderr.String()
}

// approveAs plays the human in the dashboard: POST the consent with the
// owner's session token, then follow redirect_to to the CLI's callback.
func (a *authEnv) approveAs(t *testing.T, authURL string) error {
	u, err := url.Parse(authURL)
	if err != nil || u.Path != "/authorize" {
		t.Fatalf("authorize URL %q", authURL)
	}
	q := u.Query()
	body, _ := json.Marshal(map[string]any{
		"client_id": q.Get("client_id"), "redirect_uri": q.Get("redirect_uri"), "code_challenge": q.Get("code_challenge"),
		"code_challenge_method": q.Get("code_challenge_method"), "state": q.Get("state"), "scope": q.Get("scope"), "approve": true,
	})
	req, _ := http.NewRequest("POST", a.srv.URL+"/v1/oauth/authorize", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+a.owner)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", a.srv.URL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		RedirectTo string `json:"redirect_to"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.RedirectTo == "" {
		t.Fatalf("consent: %d %v", resp.StatusCode, err)
	}
	cb, err := http.Get(out.RedirectTo)
	if err != nil {
		return err
	}
	cb.Body.Close()
	return nil
}

func TestAuthLoginPrecedenceAndFileMode(t *testing.T) {
	a := newAuthEnv(t)
	openBrowser = func(u string) error { return a.approveAs(t, u) }
	t.Cleanup(func() { openBrowser = defaultOpenBrowser })

	code, doc, stderr := a.run(t, "auth", "login")
	if code != 0 {
		t.Fatalf("login %d %s", code, stderr)
	}
	if raw, _ := json.Marshal(doc["scopes"]); string(raw) != `["read","write"]` || doc["auth_method"] != "browser" {
		t.Fatalf("login result %v", doc)
	}

	path := filepath.Join(a.dir, ".config", "pursuit", "credentials.json")
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("credentials mode %o, want 600", st.Mode().Perm())
	}
	if dst, _ := os.Stat(filepath.Dir(path)); dst.Mode().Perm() != 0o700 {
		t.Fatalf("credentials dir mode %o, want 700", dst.Mode().Perm())
	}
	stored, ok, err := config.LoadCredential(a.srv.URL + "/")
	if err != nil || !ok || !strings.HasPrefix(stored.Token, "pst_") || stored.Storage != "file" {
		t.Fatalf("stored credential %+v %v %v", stored, ok, err)
	}

	// The stored token is used when nothing else is set.
	code, doc, stderr = a.run(t, "auth", "status")
	if code != 0 || doc["source"] != "file" {
		t.Fatalf("status from store: %d %v %s", code, doc, stderr)
	}
	if raw, _ := json.Marshal(doc["scopes"]); string(raw) != `["read","write"]` {
		t.Fatalf("stored token scopes %s", raw)
	}
	// --token beats the stored credential.
	code, doc, _ = a.run(t, "--token", a.owner, "auth", "status")
	if code != 0 || doc["source"] != "flag" {
		t.Fatalf("status with --token: %d %v", code, doc)
	}
	if raw, _ := json.Marshal(doc["scopes"]); !strings.Contains(string(raw), "approve") {
		t.Fatalf("--token did not win: %s", raw)
	}
	// PURSUIT_TOKEN beats it too.
	t.Setenv("PURSUIT_TOKEN", a.owner)
	if code, doc, _ = a.run(t, "auth", "status"); code != 0 || doc["source"] != "env" {
		t.Fatalf("status with PURSUIT_TOKEN: %d %v", code, doc)
	}
	t.Setenv("PURSUIT_TOKEN", "")

	// Logout revokes server-side and deletes the local copy.
	code, doc, stderr = a.run(t, "auth", "logout")
	if code != 0 || doc["revoked"] != true || doc["removed"] != true {
		t.Fatalf("logout %d %v %s", code, doc, stderr)
	}
	if _, ok, _ := config.LoadCredential(a.srv.URL); ok {
		t.Fatal("credential still stored after logout")
	}
	if me, err := whoami(a.srv.URL, stored.Token); err == nil {
		t.Fatalf("revoked token still works: %+v", me)
	}
	if code, _, _ = a.run(t, "auth", "status"); code != 5 {
		t.Fatalf("status after logout exited %d, want 5", code)
	}
}

func TestAuthLoginDevice(t *testing.T) {
	a := newAuthEnv(t)
	var stderr bytes.Buffer
	userCode := regexp.MustCompile(`(?m)^  ([A-Z]{4}-[A-Z]{4})$`)
	polls := 0
	sleep = func(d time.Duration) {
		a.now = a.now.Add(d)
		polls++
		if polls != 2 {
			return
		}
		m := userCode.FindStringSubmatch(stderr.String())
		if m == nil {
			t.Fatalf("no user code printed:\n%s", stderr.String())
		}
		body, _ := json.Marshal(map[string]any{"user_code": m[1], "approve": true})
		req, _ := http.NewRequest("POST", a.srv.URL+"/v1/oauth/device/approve", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+a.owner)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("approve: %v %v", resp, err)
		}
		resp.Body.Close()
	}
	t.Cleanup(func() { sleep = time.Sleep })

	var stdout bytes.Buffer
	code := Run(append(append([]string{}, a.base...), "auth", "login", "--device", "--scope", "read"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("device login %d %s", code, stderr.String())
	}
	var doc map[string]any
	_ = json.Unmarshal(stdout.Bytes(), &doc)
	if raw, _ := json.Marshal(doc["scopes"]); string(raw) != `["read"]` || doc["auth_method"] != "device" {
		t.Fatalf("device result %v", doc)
	}
	if polls < 2 {
		t.Fatalf("expected polling, got %d", polls)
	}
}

func TestAuthLoginRefusesApproveScope(t *testing.T) {
	a := newAuthEnv(t)
	openBrowser = func(string) error { t.Fatal("must not reach the browser"); return nil }
	t.Cleanup(func() { openBrowser = defaultOpenBrowser })
	for _, sc := range []string{"approve", "read,approve", "APPROVE", "Read"} {
		if code, _, stderr := a.run(t, "auth", "login", "--scope", sc); code != 2 {
			t.Fatalf("--scope %s exited %d: %s", sc, code, stderr)
		}
	}
}

// An unreadable stored credential is an error, not a silent logout.
func TestStoredCredentialErrorsSurface(t *testing.T) {
	a := newAuthEnv(t)
	path := filepath.Join(a.dir, ".config", "pursuit", "credentials.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// A keyring backend this machine can't read stands in for a locked keychain.
	body := `{"servers":{"` + a.srv.URL + `":{"storage":"locked-keyring"}}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := a.run(t, "auth", "status")
	if code != 5 || !strings.Contains(stderr, "stored credential unavailable") {
		t.Fatalf("status with unreadable credential: %d %s", code, stderr)
	}
	code, _, stderr = a.run(t, "auth", "logout")
	if code != 1 || !strings.Contains(stderr, "nothing was revoked or removed") {
		t.Fatalf("logout with unreadable credential: %d %s", code, stderr)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "locked-keyring") {
		t.Fatal("logout removed a credential it could not delete")
	}
	if code, _, _ := a.run(t, "--token", a.owner, "auth", "status"); code != 0 {
		t.Fatal("--token should still work past an unreadable stored credential")
	}
}

func TestAuthSchema(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"schema", "auth", "login"}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	if !strings.Contains(stdout.String(), `"device"`) || !strings.Contains(stdout.String(), `"scope"`) {
		t.Fatalf("schema %s", stdout.String())
	}
}
