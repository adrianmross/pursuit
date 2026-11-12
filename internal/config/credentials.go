package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrianmross/tokenkeep"
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

var keyring = tokenkeep.Keyring{Service: "pursuit"}

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
	return tokenkeep.Backend()
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
		tok, err := keyring.Read(c.Storage, ServerKey(server))
		if errors.Is(err, tokenkeep.ErrNotFound) {
			return Credential{}, false, nil
		}
		if err != nil {
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
		if keyring.Write(backend, key, c.Token) == nil {
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
	// A keyring item that can't be deleted (locked, access denied) is an
	// error, and the file entry stays so the two remain in step.
	if c.Storage != "file" {
		if err := keyring.Delete(c.Storage, key); err != nil {
			return false, err
		}
	}
	delete(cf.Servers, key)
	return true, writeCredentials(path, cf)
}
