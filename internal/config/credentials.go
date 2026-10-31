package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Credential is what `pursuit auth login` keeps for one server. The token
// itself lives in the OS keyring when one is available (macOS Keychain via
// `security`, Linux Secret Service via `secret-tool`); otherwise it is in
// the credentials file, which is always mode 0600.
type Credential struct {
	Token     string   `json:"token,omitempty"`
	Storage   string   `json:"storage"`
	Method    string   `json:"method,omitempty"`
	Scopes    []string `json:"scopes,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
}

type credentialsFile struct {
	Servers map[string]Credential `json:"servers"`
}

const keyringService = "pursuit"

// CredentialsPath is ~/.config/pursuit/credentials.json, beside config.json.
// PURSUIT_CREDENTIALS overrides it.
func CredentialsPath() (string, error) {
	if v := strings.TrimSpace(os.Getenv("PURSUIT_CREDENTIALS")); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "pursuit", "credentials.json"), nil
}

// ServerKey normalizes a server URL so http://h:1/ and http://h:1 share a credential.
func ServerKey(server string) string {
	return strings.TrimRight(strings.TrimSpace(server), "/")
}

// KeyringBackend names the OS keyring in use, or "" for the file.
// PURSUIT_AUTH_SECRET_STORE=file forces the file.
func KeyringBackend() string {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("PURSUIT_AUTH_SECRET_STORE"))); v == "file" {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("security"); err == nil {
			return "macos-keychain"
		}
	case "linux":
		if _, err := exec.LookPath("secret-tool"); err == nil {
			return "secret-service"
		}
	}
	return ""
}

func readCredentials() (credentialsFile, string, error) {
	cf := credentialsFile{Servers: map[string]Credential{}}
	path, err := CredentialsPath()
	if err != nil {
		return cf, "", err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cf, path, nil
	}
	if err != nil {
		return cf, path, err
	}
	if err := json.Unmarshal(b, &cf); err != nil {
		return cf, path, err
	}
	if cf.Servers == nil {
		cf.Servers = map[string]Credential{}
	}
	return cf, path, nil
}

func writeCredentials(path string, cf credentialsFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".credentials-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// LoadCredential returns the stored credential for server, with Token
// filled from the keyring when that is where it lives.
func LoadCredential(server string) (Credential, bool, error) {
	cf, _, err := readCredentials()
	if err != nil {
		return Credential{}, false, err
	}
	c, ok := cf.Servers[ServerKey(server)]
	if !ok {
		return Credential{}, false, nil
	}
	if c.Storage != "file" {
		tok, err := keyringRead(c.Storage, ServerKey(server))
		if err != nil || tok == "" {
			return Credential{}, false, err
		}
		c.Token = tok
	}
	return c, c.Token != "", nil
}

// SaveCredential stores c for server: token in the keyring if possible,
// else in the 0600 file. It returns the storage used.
func SaveCredential(server string, c Credential) (string, error) {
	key := ServerKey(server)
	cf, path, err := readCredentials()
	if err != nil {
		return "", err
	}
	c.Storage = "file"
	if backend := KeyringBackend(); backend != "" {
		if keyringWrite(backend, key, c.Token) == nil {
			c.Storage = backend
			c.Token = ""
		}
	}
	cf.Servers[key] = c
	return c.Storage, writeCredentials(path, cf)
}

// DeleteCredential removes server's credential from the keyring and file.
func DeleteCredential(server string) (bool, error) {
	key := ServerKey(server)
	cf, path, err := readCredentials()
	if err != nil {
		return false, err
	}
	c, ok := cf.Servers[key]
	if !ok {
		return false, nil
	}
	if c.Storage != "file" {
		keyringDelete(c.Storage, key)
	}
	delete(cf.Servers, key)
	return true, writeCredentials(path, cf)
}

func keyringWrite(backend, account, token string) error {
	switch backend {
	case "macos-keychain":
		// -U updates an existing item. security has no stdin form for -w.
		return exec.Command("security", "add-generic-password", "-a", account, "-s", keyringService, "-w", token, "-U").Run()
	case "secret-service":
		cmd := exec.Command("secret-tool", "store", "--label", "pursuit token for "+account, "service", keyringService, "account", account)
		cmd.Stdin = strings.NewReader(token)
		return cmd.Run()
	}
	return errors.New("no keyring")
}

func keyringRead(backend, account string) (string, error) {
	var cmd *exec.Cmd
	switch backend {
	case "macos-keychain":
		cmd = exec.Command("security", "find-generic-password", "-a", account, "-s", keyringService, "-w")
	case "secret-service":
		cmd = exec.Command("secret-tool", "lookup", "service", keyringService, "account", account)
	default:
		return "", errors.New("unknown credential storage " + backend)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", nil // missing item: treat as logged out
	}
	return string(bytes.TrimSpace(out)), nil
}

func keyringDelete(backend, account string) {
	switch backend {
	case "macos-keychain":
		_ = exec.Command("security", "delete-generic-password", "-a", account, "-s", keyringService).Run()
	case "secret-service":
		_ = exec.Command("secret-tool", "clear", "service", keyringService, "account", account).Run()
	}
}
