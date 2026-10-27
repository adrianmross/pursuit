// Package config loads Pursuit settings from flags, environment, and an optional JSON file.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr             string        `json:"addr,omitempty"`
	DatabaseURL      string        `json:"database_url,omitempty"`
	LogLevel         string        `json:"log_level,omitempty"`
	LogFormat        string        `json:"log_format,omitempty"`
	OwnerName        string        `json:"owner_name,omitempty"`
	OwnerEmail       string        `json:"owner_email,omitempty"`
	BootstrapToken   string        `json:"bootstrap_token,omitempty"`
	BootstrapName    string        `json:"bootstrap_name,omitempty"`
	CORSOrigins      []string      `json:"cors_origins,omitempty"`
	ServerURL        string        `json:"server,omitempty"`
	Token            string        `json:"token,omitempty"`
	DataDir          string        `json:"data_dir,omitempty"`
	ShutdownTimeout  time.Duration `json:"-"`
	ShutdownSeconds  int           `json:"shutdown_timeout_seconds,omitempty"`
	CheckInterval    time.Duration `json:"-"`
	CheckSeconds     int           `json:"check_interval_seconds,omitempty"`
	CheckJitter      float64       `json:"check_jitter,omitempty"`
	CheckGap         time.Duration `json:"-"`
	CheckGapMS       int           `json:"check_gap_ms,omitempty"`
	ProfilePath      string        `json:"profile,omitempty"`
	ResumeFile       string        `json:"resume_file,omitempty"`
	ContactEmail     string        `json:"contact_email,omitempty"`
}

func Defaults() Config {
	return Config{
		Addr:            ":8787",
		LogLevel:        "info",
		LogFormat:       "json",
		OwnerName:       "Adrian",
		OwnerEmail:      "adrian@localhost",
		BootstrapName:   "owner",
		ServerURL:       "http://127.0.0.1:8787",
		ShutdownTimeout: 15 * time.Second,
		CheckInterval:   time.Hour,
		CheckJitter:     0.1,
		CheckGap:        250 * time.Millisecond,
	}
}

// Load reads an optional JSON file, then applies environment variables.
// Flag overrides happen in the CLI after Load returns.
func Load(explicitPath string) (Config, error) {
	cfg := Defaults()
	path, explicit, err := resolvePath(explicitPath)
	if err != nil {
		return cfg, err
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			if explicit || !os.IsNotExist(err) {
				return cfg, fmt.Errorf("read config %s: %w", path, err)
			}
		} else {
			dec := json.NewDecoder(strings.NewReader(string(b)))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&cfg); err != nil {
				return Config{}, fmt.Errorf("parse config %s: %w", path, err)
			}
			if cfg.ShutdownSeconds > 0 {
				cfg.ShutdownTimeout = time.Duration(cfg.ShutdownSeconds) * time.Second
			}
			if cfg.CheckSeconds > 0 {
				cfg.CheckInterval = time.Duration(cfg.CheckSeconds) * time.Second
			}
			if cfg.CheckGapMS > 0 {
				cfg.CheckGap = time.Duration(cfg.CheckGapMS) * time.Millisecond
			}
		}
	}
	applyEnv(&cfg)
	fill(&cfg)
	return cfg, nil
}

func fill(cfg *Config) {
	d := Defaults()
	if cfg.Addr == "" {
		cfg.Addr = d.Addr
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = d.LogLevel
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = d.LogFormat
	}
	if cfg.OwnerName == "" {
		cfg.OwnerName = d.OwnerName
	}
	if cfg.OwnerEmail == "" {
		cfg.OwnerEmail = d.OwnerEmail
	}
	if cfg.BootstrapName == "" {
		cfg.BootstrapName = d.BootstrapName
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = d.ServerURL
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = d.ShutdownTimeout
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = d.CheckInterval
	}
	if cfg.CheckJitter < 0 || cfg.CheckJitter > 1 {
		cfg.CheckJitter = d.CheckJitter
	}
	if cfg.CheckGap < 0 {
		cfg.CheckGap = d.CheckGap
	}
}

func applyEnv(cfg *Config) {
	set := func(dst *string, keys ...string) {
		for _, k := range keys {
			if v := strings.TrimSpace(os.Getenv(k)); v != "" {
				*dst = v
				return
			}
		}
	}
	set(&cfg.Addr, "PURSUIT_ADDR")
	set(&cfg.DatabaseURL, "PURSUIT_DATABASE_URL", "DATABASE_URL")
	set(&cfg.LogLevel, "PURSUIT_LOG_LEVEL")
	set(&cfg.LogFormat, "PURSUIT_LOG_FORMAT")
	set(&cfg.OwnerName, "PURSUIT_OWNER_NAME")
	set(&cfg.OwnerEmail, "PURSUIT_OWNER_EMAIL")
	set(&cfg.BootstrapToken, "PURSUIT_BOOTSTRAP_TOKEN")
	set(&cfg.BootstrapName, "PURSUIT_BOOTSTRAP_NAME")
	set(&cfg.ServerURL, "PURSUIT_SERVER")
	set(&cfg.Token, "PURSUIT_TOKEN")
	set(&cfg.DataDir, "PURSUIT_DATA_DIR")
	set(&cfg.ProfilePath, "PURSUIT_PROFILE")
	set(&cfg.ResumeFile, "PURSUIT_RESUME_FILE")
	set(&cfg.ContactEmail, "PURSUIT_CONTACT_EMAIL")
	if v := strings.TrimSpace(os.Getenv("PURSUIT_CORS_ORIGINS")); v != "" {
		cfg.CORSOrigins = splitComma(v)
	}
	if v := strings.TrimSpace(os.Getenv("PURSUIT_SHUTDOWN_TIMEOUT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ShutdownTimeout = time.Duration(n) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("PURSUIT_CHECK_INTERVAL")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.CheckInterval = time.Duration(n) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("PURSUIT_CHECK_JITTER")); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n >= 0 && n <= 1 {
			cfg.CheckJitter = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("PURSUIT_CHECK_GAP_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.CheckGap = time.Duration(n) * time.Millisecond
		}
	}
}

func resolvePath(explicit string) (string, bool, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, true, nil
	}
	if v := strings.TrimSpace(os.Getenv("PURSUIT_CONFIG")); v != "" {
		return v, true, nil
	}
	candidates := []string{"pursuit.json"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "pursuit", "config.json"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, false, nil
		}
	}
	return "", false, nil
}

func splitComma(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
