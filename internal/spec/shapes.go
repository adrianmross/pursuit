package spec

import (
	"encoding/json"

	"github.com/pursuit/pursuit/internal/sqlstore"
)

// Bodies the handlers in internal/httpapi declare inline. They mirror those
// anonymous structs and maps field for field; keep them in step.

type VersionInput struct {
	Version int `json:"version"`
}

type EnqueueInput struct {
	Kind string `json:"kind"`
}

type DecisionInput struct {
	IDs  []string `json:"ids"`
	Note string   `json:"note"`
}

type ApplyInput struct {
	Prepare bool   `json:"prepare"`
	Async   bool   `json:"async"`
	Status  string `json:"status"`
	Company string `json:"company"`
	Stale   bool   `json:"stale"`
	Q       string `json:"q"`
}

type BatchInput struct {
	Requests []struct {
		Method         string          `json:"method"`
		Path           string          `json:"path"`
		Body           json.RawMessage `json:"body"`
		IdempotencyKey string          `json:"idempotency_key"`
	} `json:"requests"`
	StopOnError bool `json:"stop_on_error"`
}

type BatchOutput struct {
	Results []struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body,omitempty"`
	} `json:"results"`
	Failed int `json:"failed"`
}

type WorkerInput struct {
	Worker string `json:"worker"`
}

type CompleteInput struct {
	Worker string `json:"worker"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

type UploadInput struct {
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Body        string `json:"body"`
}

type DocumentFile struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Label       string `json:"label"`
	Primary     bool   `json:"primary"`
	Body        string `json:"body"`
	CreatedAt   string `json:"created_at"`
}

type ArtifactFile struct {
	ID            string `json:"id"`
	ApplicationID string `json:"application_id"`
	Kind          string `json:"kind"`
	Filename      string `json:"filename"`
	ContentType   string `json:"content_type"`
	Body          string `json:"body"`
	CreatedAt     string `json:"created_at"`
}

type StandingInput struct {
	Key    string `json:"key"`
	Answer string `json:"answer"`
}

type SweepInput struct {
	Owner string `json:"owner"`
}

type WebhookInput struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

type Caps struct {
	Data []sqlstore.CompanyCap `json:"data"`
}

type ImportPosting struct {
	ID          string `json:"id"`
	Company     string `json:"company"`
	Title       string `json:"title"`
	Role        string `json:"role"`
	Location    string `json:"location"`
	Salary      string `json:"salary"`
	Fit         string `json:"fit"`
	Status      string `json:"status"`
	URL         string `json:"url"`
	PostingURL  string `json:"posting_url"`
	Notes       string `json:"notes"`
	NextStep    string `json:"next_step"`
	AppliedDate string `json:"applied_date"`
}

type SessionInput struct {
	Token string `json:"token"`
}

type DemoSession struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

type Providers struct {
	TenantMode string `json:"tenant_mode"`
	Providers  []struct {
		Name  string `json:"name"`
		Label string `json:"label"`
	} `json:"providers"`
	AllowTokenLogin bool `json:"allow_token_login"`
	DemoAvailable   bool `json:"demo_available"`
	SignupsOpen     bool `json:"signups_open"`
	Passkeys        bool `json:"passkeys"`
}

type DeviceDecision struct {
	UserCode string `json:"user_code"`
	Approve  bool   `json:"approve"`
}

type Redirect struct {
	RedirectTo string `json:"redirect_to"`
}

type Status struct {
	Status string `json:"status"`
}

type VersionInfo struct {
	Version        string `json:"version"`
	FaviconService string `json:"favicon_service"`
}

type PasskeyInput struct {
	Name       string          `json:"name"`
	Credential json.RawMessage `json:"credential"`
}

// WebAuthnOptions is the {publicKey} options object, passed through to the browser.
type WebAuthnOptions struct {
	PublicKey json.RawMessage `json:"publicKey"`
}
