package httpapi

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"

	"github.com/pursuit/pursuit/internal/service"
)

// softKey is a hand-rolled platform authenticator: one P-256 passkey,
// "none" attestation, user presence and verification always set.
type softKey struct {
	t       *testing.T
	key     *ecdsa.PrivateKey
	id      []byte
	handle  []byte
	counter uint32
	rpID    string
	origin  string
}

var raw64 = base64.RawURLEncoding

func newSoftKey(t *testing.T) *softKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	return &softKey{t: t, key: key, id: id, rpID: "localhost", origin: "http://localhost"}
}

func (k *softKey) authData(flags byte, attested bool) []byte {
	rp := sha256.Sum256([]byte(k.rpID))
	out := append(rp[:], flags)
	out = binary.BigEndian.AppendUint32(out, k.counter)
	if attested {
		pub, err := k.key.PublicKey.Bytes() // 0x04 || X || Y
		if err != nil {
			k.t.Fatal(err)
		}
		cose, err := webauthncbor.Marshal(webauthncose.EC2PublicKeyData{
			PublicKeyData: webauthncose.PublicKeyData{KeyType: 2, Algorithm: -7},
			Curve:         1, XCoord: pub[1:33], YCoord: pub[33:],
		})
		if err != nil {
			k.t.Fatal(err)
		}
		out = append(out, make([]byte, 16)...) // AAGUID
		out = binary.BigEndian.AppendUint16(out, uint16(len(k.id)))
		out = append(out, k.id...)
		out = append(out, cose...)
	}
	return out
}

func (k *softKey) clientData(kind string, options []byte) []byte {
	var opts struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
			User      struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(options, &opts); err != nil || opts.PublicKey.Challenge == "" {
		k.t.Fatalf("options %s: %v", options, err)
	}
	if opts.PublicKey.User.ID != "" {
		h, err := raw64.DecodeString(opts.PublicKey.User.ID)
		if err != nil {
			k.t.Fatal(err)
		}
		k.handle = h
	}
	b, _ := json.Marshal(map[string]any{"type": kind, "challenge": opts.PublicKey.Challenge, "origin": k.origin, "crossOrigin": false})
	return b
}

// create answers navigator.credentials.create for the given begin options.
func (k *softKey) create(options []byte) json.RawMessage {
	cdj := k.clientData("webauthn.create", options)
	att, err := webauthncbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": k.authData(0x45, true)})
	if err != nil {
		k.t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{
		"id": raw64.EncodeToString(k.id), "rawId": raw64.EncodeToString(k.id), "type": "public-key",
		"response":               map[string]any{"clientDataJSON": raw64.EncodeToString(cdj), "attestationObject": raw64.EncodeToString(att), "transports": []string{"internal"}},
		"clientExtensionResults": map[string]any{}, "authenticatorAttachment": "platform",
	})
	return b
}

// get answers navigator.credentials.get, bumping the signature counter.
func (k *softKey) get(options []byte) json.RawMessage {
	k.counter++
	cdj := k.clientData("webauthn.get", options)
	ad := k.authData(0x05, false)
	cdh := sha256.Sum256(cdj)
	digest := sha256.Sum256(append(append([]byte{}, ad...), cdh[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, k.key, digest[:])
	if err != nil {
		k.t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{
		"id": raw64.EncodeToString(k.id), "rawId": raw64.EncodeToString(k.id), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON": raw64.EncodeToString(cdj), "authenticatorData": raw64.EncodeToString(ad),
			"signature": raw64.EncodeToString(sig), "userHandle": raw64.EncodeToString(k.handle),
		},
		"clientExtensionResults": map[string]any{},
	})
	return b
}

var sameJSON = map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "same-origin"}

func passkeyEnv(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	w, err := service.NewWebAuthn("localhost", []string{"http://localhost"})
	if err != nil {
		t.Fatal(err)
	}
	e.svc.WebAuthn = w
	return e
}

func (e *env) post(t *testing.T, path, token string, body []byte) (int, []byte, http.Header) {
	t.Helper()
	return e.do(t, http.MethodPost, path, token, body, sameJSON)
}

func (e *env) registerPasskey(t *testing.T, k *softKey, name string) string {
	t.Helper()
	code, opts, _ := e.post(t, "/v1/passkeys/register/begin", e.token, []byte(`{}`))
	if code != 200 {
		t.Fatalf("register begin %d %s", code, opts)
	}
	body, _ := json.Marshal(map[string]any{"name": name, "credential": k.create(opts)})
	code, b, _ := e.post(t, "/v1/passkeys/register/finish", e.token, body)
	if code != 201 {
		t.Fatalf("register finish %d %s", code, b)
	}
	var out struct{ ID string }
	_ = json.Unmarshal(b, &out)
	return out.ID
}

func (e *env) loginBegin(t *testing.T) []byte {
	t.Helper()
	code, opts, _ := e.post(t, "/v1/auth/passkey/login/begin", "", []byte(`{}`))
	if code != 200 {
		t.Fatalf("login begin %d %s", code, opts)
	}
	return opts
}

func TestPasskeysDisabledWithoutRPID(t *testing.T) {
	e := newEnv(t)
	if _, doc := e.json(t, http.MethodGet, "/v1/auth/providers", "", nil); doc["passkeys"] != false {
		t.Fatalf("providers %v", doc)
	}
	if code, _, _ := e.post(t, "/v1/auth/passkey/login/begin", "", []byte(`{}`)); code != 404 {
		t.Fatalf("login begin while off %d", code)
	}
	if code, _ := e.json(t, http.MethodGet, "/v1/passkeys", e.token, nil); code != 404 {
		t.Fatalf("list while off %d", code)
	}
	if w, err := service.NewWebAuthn("", []string{"http://localhost"}); w != nil || err != nil {
		t.Fatalf("empty RP ID should be off: %v %v", w, err)
	}
}

func TestPasskeyRegisterAndLogin(t *testing.T) {
	e := passkeyEnv(t)
	if _, doc := e.json(t, http.MethodGet, "/v1/auth/providers", "", nil); doc["passkeys"] != true {
		t.Fatalf("providers %v", doc)
	}
	k := newSoftKey(t)

	// Registration needs a signed-in human and same-origin JSON.
	if code, _, _ := e.post(t, "/v1/passkeys/register/begin", "", []byte(`{}`)); code != 401 {
		t.Fatalf("anonymous register %d", code)
	}
	if code, _, _ := e.do(t, http.MethodPost, "/v1/passkeys/register/begin", e.token, []byte(`{}`), map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site"}); code != 403 {
		t.Fatalf("cross-site register %d", code)
	}
	id := e.registerPasskey(t, k, "Test key")
	code, list := e.json(t, http.MethodGet, "/v1/passkeys", e.token, nil)
	items, _ := list["data"].([]any)
	if code != 200 || len(items) != 1 || items[0].(map[string]any)["name"] != "Test key" || items[0].(map[string]any)["id"] != id {
		t.Fatalf("list %d %v", code, list)
	}

	// Login: usernameless, sets the session cookie with a login token.
	opts := e.loginBegin(t)
	if strings.Contains(string(opts), "allowCredentials") {
		t.Fatalf("discoverable login must not list credentials: %s", opts)
	}
	assertion := k.get(opts)
	code, b, h := e.post(t, "/v1/auth/passkey/login/finish", "", assertion)
	if code != 200 || !strings.Contains(h.Get("Set-Cookie"), sessionCookie+"=pst_") || !strings.Contains(h.Get("Set-Cookie"), "HttpOnly") {
		t.Fatalf("login finish %d %s %q", code, b, h.Get("Set-Cookie"))
	}
	var me struct {
		Token struct{ Name string } `json:"token"`
		User  struct{ Email string } `json:"user"`
	}
	_ = json.Unmarshal(b, &me)
	if me.Token.Name != service.LoginTokenName || me.User.Email != "adrian@localhost" {
		t.Fatalf("me %s", b)
	}
	cookie := strings.SplitN(h.Get("Set-Cookie"), ";", 2)[0]
	if code, _, _ := e.do(t, http.MethodGet, "/v1/me", "", nil, map[string]string{"Cookie": cookie}); code != 200 {
		t.Fatalf("cookie from passkey login %d", code)
	}
	if code, list := e.json(t, http.MethodGet, "/v1/passkeys", e.token, nil); code != 200 || list["data"].([]any)[0].(map[string]any)["last_used_at"] == nil {
		t.Fatalf("last_used_at not set: %v", list)
	}

	// Replay: the same signed response again. The challenge is spent.
	if code, b, _ := e.post(t, "/v1/auth/passkey/login/finish", "", assertion); code != 401 {
		t.Fatalf("replayed login %d %s", code, b)
	}

	// Sign-count regression: a response whose counter didn't advance.
	k.counter--
	if code, b, _ := e.post(t, "/v1/auth/passkey/login/finish", "", k.get(e.loginBegin(t))); code != 401 {
		t.Fatalf("counter regression %d %s", code, b)
	}

	// Wrong origin: signed by the right key, collected on another site.
	k.origin = "https://evil.example"
	if code, b, _ := e.post(t, "/v1/auth/passkey/login/finish", "", k.get(e.loginBegin(t))); code != 401 {
		t.Fatalf("wrong origin %d %s", code, b)
	}
	k.origin = "http://localhost"

	// Expired challenge.
	opts = e.loginBegin(t)
	e.svc.Now = func() time.Time { return time.Now().Add(6 * time.Minute) }
	if code, b, _ := e.post(t, "/v1/auth/passkey/login/finish", "", k.get(opts)); code != 401 {
		t.Fatalf("expired challenge %d %s", code, b)
	}
	e.svc.Now = time.Now

	// Still fine with a fresh challenge.
	if code, b, _ := e.post(t, "/v1/auth/passkey/login/finish", "", k.get(e.loginBegin(t))); code != 200 {
		t.Fatalf("login again %d %s", code, b)
	}

	// Unknown credential: a key that was never registered, for this user's handle and for a random one.
	stranger := newSoftKey(t)
	stranger.handle = k.handle
	if code, b, _ := e.post(t, "/v1/auth/passkey/login/finish", "", stranger.get(e.loginBegin(t))); code != 401 {
		t.Fatalf("unregistered key %d %s", code, b)
	}
	stranger.handle = []byte("no-such-user-handle")
	if code, b, _ := e.post(t, "/v1/auth/passkey/login/finish", "", stranger.get(e.loginBegin(t))); code != 401 {
		t.Fatalf("unknown handle %d %s", code, b)
	}

	// Delete, then the passkey no longer signs in.
	if code, b, _ := e.do(t, http.MethodDelete, "/v1/passkeys/"+id, e.token, nil, nil); code != 200 {
		t.Fatalf("delete %d %s", code, b)
	}
	if code, _, _ := e.do(t, http.MethodDelete, "/v1/passkeys/"+id, e.token, nil, nil); code != 404 {
		t.Fatalf("delete again %d", code)
	}
	if code, b, _ := e.post(t, "/v1/auth/passkey/login/finish", "", k.get(e.loginBegin(t))); code != 401 {
		t.Fatalf("deleted key %d %s", code, b)
	}
}

func TestPasskeyRegisterReplayAndScope(t *testing.T) {
	e := passkeyEnv(t)
	k := newSoftKey(t)
	_, opts, _ := e.post(t, "/v1/passkeys/register/begin", e.token, []byte(`{}`))
	body, _ := json.Marshal(map[string]any{"name": "Key", "credential": k.create(opts)})
	if code, b, _ := e.post(t, "/v1/passkeys/register/finish", e.token, body); code != 201 {
		t.Fatalf("finish %d %s", code, b)
	}
	if code, b, _ := e.post(t, "/v1/passkeys/register/finish", e.token, body); code != 401 {
		t.Fatalf("replayed registration %d %s", code, b)
	}

	// Wrong origin on registration.
	k2 := newSoftKey(t)
	k2.origin = "https://evil.example"
	_, opts, _ = e.post(t, "/v1/passkeys/register/begin", e.token, []byte(`{}`))
	body, _ = json.Marshal(map[string]any{"name": "Key", "credential": k2.create(opts)})
	if code, b, _ := e.post(t, "/v1/passkeys/register/finish", e.token, body); code != 401 {
		t.Fatalf("wrong-origin registration %d %s", code, b)
	}

	// An agent token without approve can't add one.
	code, doc := e.json(t, http.MethodPost, "/v1/tokens", e.token, map[string]any{"name": "agent", "scopes": []string{"write"}})
	if code != 201 {
		t.Fatalf("token %d %v", code, doc)
	}
	agent, _ := doc["secret"].(string)
	if code, _, _ := e.post(t, "/v1/passkeys/register/begin", agent, []byte(`{}`)); code != 403 {
		t.Fatalf("agent register %d", code)
	}
}
