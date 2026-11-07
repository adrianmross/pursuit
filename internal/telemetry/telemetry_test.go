package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestEnabled(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want bool
	}{
		{nil, true},
		{map[string]string{"DO_NOT_TRACK": "1"}, false},
		{map[string]string{"DO_NOT_TRACK": "0"}, true},
		{map[string]string{"PURSUIT_TELEMETRY": "off"}, false},
		{map[string]string{"PURSUIT_TELEMETRY": "FALSE"}, false},
		{map[string]string{"PURSUIT_TELEMETRY": "on"}, true},
	}
	for _, c := range cases {
		if got := Enabled(env(c.env)); got != c.want {
			t.Errorf("Enabled(%v) = %v, want %v", c.env, got, c.want)
		}
	}
}

func TestInstallIDPersists(t *testing.T) {
	dir := t.TempDir()
	a, b := InstallID(dir), InstallID(dir)
	if len(a) != 32 || a != b {
		t.Fatalf("id not stable: %q vs %q", a, b)
	}
	if InstallID("") == InstallID("") {
		t.Fatal("ephemeral ids should differ")
	}
}

func TestPingSendsPayloadAndReadsLatest(t *testing.T) {
	var got Payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"latest":"9.9.9"}`))
	}))
	defer srv.Close()
	body, _ := json.Marshal(Payload{InstallID: "x", Version: "1.0.0", DB: "sqlite"})
	if latest := ping(context.Background(), srv.Client(), srv.URL, body); latest != "9.9.9" {
		t.Fatalf("latest = %q", latest)
	}
	if got.Version != "1.0.0" || got.DB != "sqlite" {
		t.Fatalf("payload = %+v", got)
	}
}

func TestPingFailureIsQuiet(t *testing.T) {
	if latest := ping(context.Background(), http.DefaultClient, "http://127.0.0.1:1", nil); latest != "" {
		t.Fatalf("latest = %q", latest)
	}
}
