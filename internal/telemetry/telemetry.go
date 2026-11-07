// Package telemetry sends the anonymous install ping from `pursuit serve`.
// It is on unless the operator opts out, and it never carries user content.
package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultURL = "https://pursuit-telemetry.winter-tooth-73af.workers.dev/v1/ping"
	interval   = 24 * time.Hour
)

// Payload is everything that leaves the machine. Keep it small: the docs
// page lists exactly these fields.
type Payload struct {
	InstallID string `json:"install_id"`
	Version   string `json:"version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	DB        string `json:"db"`
	Docker    bool   `json:"docker"`
}

// Enabled reports whether the ping may be sent. Any of DO_NOT_TRACK or
// PURSUIT_TELEMETRY turns it off.
func Enabled(getenv func(string) string) bool {
	if truthy(getenv("DO_NOT_TRACK")) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(getenv("PURSUIT_TELEMETRY"))) {
	case "off", "false", "0", "no":
		return false
	}
	return true
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// InstallID returns a random id kept in dataDir. With no data dir it is
// new each start, so nothing persists.
func InstallID(dataDir string) string {
	path := ""
	if dataDir != "" {
		path = filepath.Join(dataDir, "telemetry-id")
		if b, err := os.ReadFile(path); err == nil {
			if id := strings.TrimSpace(string(b)); len(id) == 32 {
				return id
			}
		}
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	if path != "" {
		_ = os.WriteFile(path, []byte(id+"\n"), 0o600)
	}
	return id
}

// Run logs what it sends, then pings now and daily until ctx ends. A failed
// ping is only logged at debug level.
func Run(ctx context.Context, log *slog.Logger, p Payload, getenv func(string) string) {
	url := strings.TrimSpace(getenv("PURSUIT_TELEMETRY_URL"))
	if url == "" {
		url = defaultURL
	}
	b, _ := json.Marshal(p)
	log.Info("anonymous telemetry on", "sends", string(b), "to", url, "opt_out", "PURSUIT_TELEMETRY=off or DO_NOT_TRACK=1")
	client := &http.Client{Timeout: 5 * time.Second}
	for {
		if latest := ping(ctx, client, url, b); latest != "" && latest != p.Version {
			log.Info("a newer release is available", "latest", latest, "running", p.Version, "notes", "https://hirepursuit.app/docs/changelog")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// ping posts the payload and returns the "latest" version the endpoint
// answers with, or "" on any failure.
func ping(ctx context.Context, client *http.Client, url string, body []byte) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var out struct {
		Latest string `json:"latest"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&out)
	return out.Latest
}

// Docker reports whether the process runs in a container.
func Docker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// Base fills the fields every caller shares.
func Base(version, db, dataDir string) Payload {
	return Payload{InstallID: InstallID(dataDir), Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH, DB: db, Docker: Docker()}
}
