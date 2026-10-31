package cli

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	osexec "os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/apiclient"
	"github.com/pursuit/pursuit/internal/auth"
	"github.com/pursuit/pursuit/internal/config"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
)

const (
	cliClientID      = "pursuit-cli"
	deviceGrantType  = "urn:ietf:params:oauth:grant-type:device_code"
	defaultCLIScopes = "read,write"
)

// Test seams. openBrowser returns an error when no opener could start.
var (
	openBrowser  = defaultOpenBrowser
	sleep        = time.Sleep
	loginTimeout = 5 * time.Minute
)

func cmdAuth() *Command {
	return &Command{Name: "auth", Summary: "sign the CLI in to a server", Children: []*Command{
		{
			Name: "login", Summary: "sign in through the dashboard and store a token for this server",
			Usage:   "pursuit auth login [--server URL] [--device] [--scope read,write]",
			Example: "pursuit auth login --server https://pursuit.example.com",
			Flags: []Flag{
				{Name: "device", Type: "bool", Help: "use a device code instead of a browser callback (default when there is no display or over SSH)"},
				{Name: "scope", Type: "string", Help: "read, write, or read,write (default); approve is never granted to the CLI"},
			},
			Run: runAuthLogin,
		},
		{Name: "logout", Summary: "revoke and delete the stored token for this server", Usage: "pursuit auth logout [--server URL]", Run: runAuthLogout},
		{Name: "status", Summary: "show server, identity, scopes, and where the token came from", Usage: "pursuit auth status [--server URL]", Run: runAuthStatus},
	}}
}

// tokenSource reports where cfg.Token came from, before stored credentials
// are consulted. PURSUIT_TOKEN and --token always win over a stored login.
func tokenSource(g globals, cfg config.Config) string {
	switch {
	case g.Token != "":
		return "flag"
	case strings.TrimSpace(os.Getenv("PURSUIT_TOKEN")) != "":
		return "env"
	case cfg.Token != "":
		return "config"
	}
	return "none"
}

func runAuthLogin(e *exec) error {
	server := config.ServerKey(e.cfg.ServerURL)
	raw := e.flag("scope")
	if raw == "" {
		raw = defaultCLIScopes
	}
	if strings.Contains(strings.ToLower(raw), "approve") {
		return &exitError{code: 2, message: "pursuit auth login never grants approve", hint: "approve in the dashboard, or have a human mint an approve token and pass it as PURSUIT_TOKEN"}
	}
	scopes, err := service.ParseOAuthScopes(raw)
	if err != nil {
		return &exitError{code: 2, message: "invalid --scope " + raw, hint: "use read, write, or read,write (lowercase)"}
	}
	scope := strings.Join(scopes, " ")
	method := "browser"
	if e.flag("device") == "true" || headless() {
		method = "device"
	}
	var token string
	if method == "browser" {
		token, err = loopbackLogin(e, server, scope)
	} else {
		token, err = deviceLogin(e, server, scope)
	}
	if err != nil {
		return err
	}
	me, err := whoami(e.cfg.VercelBypass, server, token)
	if err != nil {
		return err
	}
	storage, err := config.SaveCredential(server, config.Credential{
		Token: token, Method: method, Scopes: me.Token.Scopes, CreatedAt: domain.FormatTime(time.Now().UTC()),
	})
	if err != nil {
		return &exitError{code: 1, message: "save credentials: " + err.Error(), hint: "check permissions on ~/.config/pursuit"}
	}
	path, _ := config.CredentialsPath()
	preferred := config.KeyringBackend()
	if preferred == "" {
		preferred = "file"
	}
	fmt.Fprintln(e.stderr, "Logged in to "+server+" as "+me.User.Email+".")
	return e.printValue(map[string]any{
		"status": "logged_in", "server": server, "auth_method": method,
		"user":   map[string]string{"id": me.User.ID, "name": me.User.Name, "email": me.User.Email},
		"scopes": me.Token.Scopes, "token_prefix": me.Token.Prefix,
		"token_storage":    map[string]string{"stored": storage, "preferred": preferred},
		"credentials_path": path,
	})
}

func runAuthLogout(e *exec) error {
	server := config.ServerKey(e.cfg.ServerURL)
	c, ok, err := config.LoadCredential(server)
	if err != nil {
		return &exitError{code: 1, message: "cannot read the stored token, so nothing was revoked or removed: " + err.Error(), hint: "unlock the keychain and run pursuit auth logout again"}
	}
	revoked := false
	if ok {
		status, _, err := postForm(e.cfg.VercelBypass, server, "/v1/auth/revoke", url.Values{"token": {c.Token}})
		revoked = err == nil && status == 200
		if !revoked {
			fmt.Fprintln(e.stderr, "Could not revoke the token on "+server+"; deleting it locally anyway.")
		}
	}
	removed, err := config.DeleteCredential(server)
	if err != nil {
		return &exitError{code: 1, message: fmt.Sprintf("revoked=%v, but the stored credential was not removed: %s", revoked, err.Error()), hint: "unlock the keychain and run pursuit auth logout again"}
	}
	out := map[string]any{"status": "logged_out", "server": server, "revoked": revoked, "removed": removed}
	if src := tokenSource(e.g, e.cfg); src == "env" || src == "flag" {
		out["note"] = "a token from " + src + " is still set; it was not changed"
	}
	return e.printValue(out)
}

func runAuthStatus(e *exec) error {
	server := config.ServerKey(e.cfg.ServerURL)
	if e.client.Token == "" {
		return &exitError{code: 5, message: "not logged in to " + server, hint: "run pursuit auth login, or set PURSUIT_TOKEN"}
	}
	me, err := whoami(e.cfg.VercelBypass, server, e.client.Token)
	if err != nil {
		return err
	}
	return e.printValue(map[string]any{
		"logged_in": true, "server": server, "source": e.tokenSource,
		"user":   map[string]string{"id": me.User.ID, "name": me.User.Name, "email": me.User.Email},
		"token":  map[string]any{"id": me.Token.ID, "name": me.Token.Name, "prefix": me.Token.Prefix},
		"scopes": me.Token.Scopes,
	})
}

func (e *exec) printValue(v any) error {
	b, err := e.jsonBody(v)
	if err != nil {
		return err
	}
	return e.print(b, "application/json")
}

func whoami(bypass, server, token string) (domain.Me, error) {
	c := &apiclient.Client{BaseURL: server, Token: token, Bypass: bypass}
	resp, err := c.Do(nil, "GET", "/v1/me", nil, nil, "", apiclient.Options{})
	if err != nil {
		return domain.Me{}, &exitError{code: 1, message: strings.TrimPrefix(err.Error(), "unavailable: ")}
	}
	if resp.Status >= 400 {
		return domain.Me{}, &exitError{code: exitStatus(resp.Status), raw: resp.Body}
	}
	var me domain.Me
	if err := json.Unmarshal(resp.Body, &me); err != nil {
		return domain.Me{}, &exitError{code: 1, message: "unexpected /v1/me response"}
	}
	return me, nil
}

// postForm calls a public OAuth endpoint without any bearer token.
func postForm(bypass, server, path string, vals url.Values) (int, map[string]any, error) {
	c := &apiclient.Client{BaseURL: server, Bypass: bypass}
	resp, err := c.Do(nil, "POST", path, nil, []byte(vals.Encode()), "application/x-www-form-urlencoded", apiclient.Options{})
	if err != nil {
		return 0, nil, &exitError{code: 1, message: strings.TrimPrefix(err.Error(), "unavailable: "), hint: "check --server or PURSUIT_SERVER"}
	}
	var doc map[string]any
	_ = json.Unmarshal(resp.Body, &doc)
	return resp.Status, doc, nil
}

func str(doc map[string]any, k string) string {
	s, _ := doc[k].(string)
	return s
}

func tokenFrom(status int, doc map[string]any, flow string) (string, error) {
	if status == 200 && str(doc, "access_token") != "" {
		return str(doc, "access_token"), nil
	}
	msg := flow + " login failed"
	if code := str(doc, "error"); code != "" {
		msg += ": " + code
		if d := str(doc, "error_description"); d != "" {
			msg += " (" + d + ")"
		}
	} else {
		msg += fmt.Sprintf(": HTTP %d", status)
	}
	return "", &exitError{code: 5, message: msg, hint: "run pursuit auth login again"}
}

// loopbackLogin is RFC 8252 authorization code + PKCE with a one-shot
// listener on 127.0.0.1. The dashboard's /authorize page does sign-in and
// consent, then sends the browser back to our /callback.
func loopbackLogin(e *exec, server, scope string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", &exitError{code: 1, message: "listen on 127.0.0.1: " + err.Error(), hint: "use pursuit auth login --device"}
	}
	redirect := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)
	verifier, err := auth.RandomURLToken(48)
	if err != nil {
		return "", err
	}
	state, err := auth.RandomURLToken(24)
	if err != nil {
		return "", err
	}
	authURL := server + "/authorize?" + url.Values{
		"response_type": {"code"}, "client_id": {cliClientID}, "redirect_uri": {redirect},
		"code_challenge": {auth.PKCEChallenge(verifier)}, "code_challenge_method": {"S256"},
		"state": {state}, "scope": {scope},
	}.Encode()

	type result struct{ code, errCode string }
	done := make(chan result, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/callback" || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		res := result{code: q.Get("code"), errCode: q.Get("error")}
		msg := "Pursuit CLI is signed in. You can close this tab."
		if res.code == "" {
			msg = "Sign-in was not completed (" + res.errCode + "). You can close this tab."
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><title>Pursuit</title><p style=\"font:16px system-ui;margin:3rem\">%s</p>", html.EscapeString(msg))
		select {
		case done <- res:
		default:
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	if err := openBrowser(authURL); err != nil {
		fmt.Fprintln(e.stderr, "Could not open a browser. Open this URL to sign in:")
	} else {
		fmt.Fprintln(e.stderr, "Opened your browser. If it did not open, visit:")
	}
	fmt.Fprintln(e.stderr, "  "+authURL)
	fmt.Fprintln(e.stderr, "Waiting for the browser on "+redirect+" (or run pursuit auth login --device) ...")

	var res result
	select {
	case res = <-done:
	case <-time.After(loginTimeout):
		return "", &exitError{code: 5, message: "timed out waiting for the browser", hint: "run pursuit auth login --device"}
	}
	if res.code == "" {
		code := res.errCode
		if code == "" {
			code = "no code returned"
		}
		return "", &exitError{code: 5, message: "browser login failed: " + code, hint: "run pursuit auth login again"}
	}
	status, doc, err := postForm(e.cfg.VercelBypass, server, "/v1/auth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {res.code}, "code_verifier": {verifier},
		"redirect_uri": {redirect}, "client_id": {cliClientID},
	})
	if err != nil {
		return "", err
	}
	return tokenFrom(status, doc, "browser")
}

// deviceLogin is RFC 8628: show a code, the human approves it in the
// dashboard, and we poll until the server answers.
func deviceLogin(e *exec, server, scope string) (string, error) {
	status, d, err := postForm(e.cfg.VercelBypass, server, "/v1/auth/device_authorization", url.Values{"client_id": {cliClientID}, "scope": {scope}})
	if err != nil {
		return "", err
	}
	deviceCode := str(d, "device_code")
	if status != 200 || deviceCode == "" {
		_, err := tokenFrom(status, d, "device")
		return "", err
	}
	interval := 5
	if n, ok := d["interval"].(float64); ok && n > 0 {
		interval = int(n)
	}
	expires := 600
	if n, ok := d["expires_in"].(float64); ok && n > 0 {
		expires = int(n)
	}
	fmt.Fprintln(e.stderr, "Open "+str(d, "verification_uri")+" while signed in to the dashboard and enter:")
	fmt.Fprintln(e.stderr, "  "+str(d, "user_code"))
	fmt.Fprintln(e.stderr, "Waiting for approval ...")
	deadline := time.Now().Add(time.Duration(expires) * time.Second)
	for time.Now().Before(deadline) {
		sleep(time.Duration(interval) * time.Second)
		status, doc, err := postForm(e.cfg.VercelBypass, server, "/v1/auth/token", url.Values{"grant_type": {deviceGrantType}, "device_code": {deviceCode}, "client_id": {cliClientID}})
		if err != nil {
			return "", err
		}
		switch str(doc, "error") {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5
			continue
		case "expired_token":
			return "", &exitError{code: 5, message: "the device code expired before it was approved", hint: "run pursuit auth login --device again"}
		case "access_denied":
			return "", &exitError{code: 5, message: "the login was denied in the dashboard", hint: "run pursuit auth login --device again to retry"}
		}
		return tokenFrom(status, doc, "device")
	}
	return "", &exitError{code: 5, message: "the device code expired before it was approved", hint: "run pursuit auth login --device again"}
}

// headless guesses that a browser on this machine can't reach us: an SSH
// session, or Linux with no display server.
func headless() bool {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return true
	}
	return runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == ""
}

func defaultOpenBrowser(u string) error {
	var cmd *osexec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = osexec.Command("open", u)
	case "windows":
		cmd = osexec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = osexec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
