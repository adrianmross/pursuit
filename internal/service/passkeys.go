package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

// Passkeys: WebAuthn sign-in for existing users (go-webauthn does every
// check: challenge, origin, RP ID, signature, flags). A passkey is only
// added by a signed-in user and only signs that user in again; login never
// creates a user, so PURSUIT_SIGNUPS has nothing to gate. The password
// (Neon Auth) or token stays the recovery path.
//
// go-webauthn is pre-1.0; it is imported here and nowhere else.

// passkeyChallengeTTL bounds a begin -> finish ceremony.
const passkeyChallengeTTL = 5 * time.Minute

var b64 = base64.RawURLEncoding

// NewWebAuthn returns nil when rpID is empty: passkeys are off until
// PURSUIT_WEBAUTHN_RP_ID is set.
func NewWebAuthn(rpID string, origins []string) (*webauthn.WebAuthn, error) {
	rpID = strings.TrimSpace(rpID)
	if rpID == "" {
		return nil, nil
	}
	yes := true
	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: "Pursuit",
		RPOrigins:     origins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: &yes,
			UserVerification:   protocol.VerificationRequired,
		},
	})
}

// PasskeyView is a passkey as the API shows it. The key itself never leaves the server.
type PasskeyView struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	// Synced is the backup state flag: the passkey is in a cloud keychain
	// (iCloud, Google, 1Password) rather than bound to one device.
	Synced bool `json:"synced"`
}

type passkeyUser struct {
	handle []byte
	user   domain.User
	creds  []webauthn.Credential
	rows   []sqlstore.WebAuthnCredential
}

func (u *passkeyUser) WebAuthnID() []byte                         { return u.handle }
func (u *passkeyUser) WebAuthnName() string                       { return u.user.Email }
func (u *passkeyUser) WebAuthnDisplayName() string                { return u.user.Name }
func (u *passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func passkeysOff() error {
	return &domain.Error{Code: "not_found", Message: "passkeys are not enabled on this server", Hint: "the owner sets PURSUIT_WEBAUTHN_RP_ID to turn them on", Status: 404}
}

func passkeyRefused() error {
	return &domain.Error{Code: "unauthorized", Message: "passkey not accepted", Hint: "start again; sign in with your password or token if this keeps failing", Status: 401}
}

// passkeyOwner is who may manage passkeys: a signed-in human, the same
// test humanApprover applies. Demo sandboxes and the proxy owner have no
// account of their own to attach one to.
func (s *Service) passkeyOwner(ctx context.Context) (domain.Actor, error) {
	if s.WebAuthn == nil {
		return domain.Actor{}, passkeysOff()
	}
	a, err := s.actor(ctx, false)
	if err != nil {
		return a, err
	}
	if !a.Can("approve") || a.Demo || a.Proxy {
		return domain.Actor{}, domain.Forbidden("approve").WithHint("manage passkeys from the dashboard while signed in as yourself")
	}
	return a, nil
}

// loadPasskeyUser reads a user and their credentials. With create, a user
// without a WebAuthn handle gets one.
func (s *Service) loadPasskeyUser(ctx context.Context, userID string, create bool) (*passkeyUser, error) {
	user, err := s.DB.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	handle, err := s.DB.WebAuthnHandle(ctx, userID)
	if err != nil {
		return nil, err
	}
	if handle == "" && create {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return nil, err
		}
		if err := s.DB.SetWebAuthnHandle(ctx, userID, b64.EncodeToString(raw)); err != nil {
			return nil, err
		}
		if handle, err = s.DB.WebAuthnHandle(ctx, userID); err != nil {
			return nil, err
		}
	}
	h, err := b64.DecodeString(handle)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.ListWebAuthnCredentials(ctx, userID)
	if err != nil {
		return nil, err
	}
	u := &passkeyUser{handle: h, user: user, rows: rows}
	for _, r := range rows {
		var c webauthn.Credential
		if err := json.Unmarshal(r.Credential, &c); err != nil {
			return nil, err
		}
		u.creds = append(u.creds, c)
	}
	return u, nil
}

func (s *Service) saveChallenge(ctx context.Context, kind, userID string, sess *webauthn.SessionData) error {
	now := s.now()
	_ = s.DB.DeleteExpiredWebAuthnChallenges(ctx, domain.FormatTime(now))
	raw, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.DB.CreateWebAuthnChallenge(ctx, sess.Challenge, kind, userID, raw, domain.FormatTime(now.Add(passkeyChallengeTTL)))
}

// takeChallenge spends a challenge. A replayed, expired or unknown one is refused.
func (s *Service) takeChallenge(ctx context.Context, kind, challenge string) (string, webauthn.SessionData, error) {
	var sess webauthn.SessionData
	userID, raw, ok, err := s.DB.UseWebAuthnChallenge(ctx, challenge, kind, domain.FormatTime(s.now()))
	if err != nil {
		return "", sess, err
	}
	if !ok {
		return "", sess, &domain.Error{Code: "unauthorized", Message: "this passkey request expired or was already used", Hint: "start again", Status: 401}
	}
	if err := json.Unmarshal(raw, &sess); err != nil {
		return "", sess, err
	}
	return userID, sess, nil
}

// BeginPasskeyRegistration returns the options for navigator.credentials.create.
func (s *Service) BeginPasskeyRegistration(ctx context.Context) (*protocol.CredentialCreation, error) {
	a, err := s.passkeyOwner(ctx)
	if err != nil {
		return nil, err
	}
	u, err := s.loadPasskeyUser(ctx, a.UserID, true)
	if err != nil {
		return nil, err
	}
	creation, sess, err := s.WebAuthn.BeginRegistration(u, webauthn.WithExclusions(webauthn.Credentials(u.creds).CredentialDescriptors()))
	if err != nil {
		return nil, err
	}
	return creation, s.saveChallenge(ctx, "register", a.UserID, sess)
}

// FinishPasskeyRegistration verifies the authenticator's response and stores the passkey.
func (s *Service) FinishPasskeyRegistration(ctx context.Context, name string, response json.RawMessage) (PasskeyView, error) {
	a, err := s.passkeyOwner(ctx)
	if err != nil {
		return PasskeyView{}, err
	}
	name, err = domain.RequireText("name", name, 80)
	if err != nil {
		return PasskeyView{}, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return PasskeyView{}, domain.Validation("credential is not a WebAuthn registration response").WithHint("send the browser's PublicKeyCredential.toJSON() as credential")
	}
	userID, sess, err := s.takeChallenge(ctx, "register", parsed.Response.CollectedClientData.Challenge)
	if err != nil {
		return PasskeyView{}, err
	}
	if userID != a.UserID {
		return PasskeyView{}, passkeyRefused()
	}
	u, err := s.loadPasskeyUser(ctx, a.UserID, false)
	if err != nil {
		return PasskeyView{}, err
	}
	cred, err := s.WebAuthn.CreateCredential(u, sess, parsed)
	if err != nil {
		return PasskeyView{}, passkeyRefused()
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return PasskeyView{}, err
	}
	now := s.now()
	row := sqlstore.WebAuthnCredential{ID: b64.EncodeToString(cred.ID), UserID: a.UserID, Name: name, Credential: raw, CreatedAt: domain.FormatTime(now)}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.CreateWebAuthnCredential(ctx, row); err != nil {
			return err
		}
		return s.audit(ctx, a, "passkey.create", "passkey", row.ID, "added passkey "+name, map[string]any{"synced": cred.Flags.BackupState})
	})
	if err != nil {
		return PasskeyView{}, err
	}
	return PasskeyView{ID: row.ID, Name: name, CreatedAt: now, Synced: cred.Flags.BackupState}, nil
}

// BeginPasskeyLogin returns options for navigator.credentials.get with no
// allow list: the authenticator offers whichever passkeys it holds for this
// RP, so the server never learns or reveals which accounts exist.
func (s *Service) BeginPasskeyLogin(ctx context.Context) (*protocol.CredentialAssertion, error) {
	if s.WebAuthn == nil {
		return nil, passkeysOff()
	}
	assertion, sess, err := s.WebAuthn.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, err
	}
	return assertion, s.saveChallenge(ctx, "login", "", sess)
}

// FinishPasskeyLogin verifies an assertion and mints a dashboard login
// token for the passkey's owner. An unknown passkey, or one whose user is
// gone, is refused; nothing is created.
func (s *Service) FinishPasskeyLogin(ctx context.Context, response json.RawMessage) (string, error) {
	if s.WebAuthn == nil {
		return "", passkeysOff()
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return "", passkeyRefused()
	}
	_, sess, err := s.takeChallenge(ctx, "login", parsed.Response.CollectedClientData.Challenge)
	if err != nil {
		return "", err
	}
	var found *passkeyUser
	handler := func(_, userHandle []byte) (webauthn.User, error) {
		userID, err := s.DB.UserIDByWebAuthnHandle(ctx, b64.EncodeToString(userHandle))
		if err != nil {
			return nil, err
		}
		found, err = s.loadPasskeyUser(ctx, userID, false)
		return found, err
	}
	_, cred, err := s.WebAuthn.ValidatePasskeyLogin(handler, sess, parsed)
	if err != nil {
		return "", passkeyRefused()
	}
	// The counter went backwards: two copies of the key may exist. Synced
	// passkeys report 0 every time and never trip this.
	if cred.Authenticator.CloneWarning {
		return "", &domain.Error{Code: "unauthorized", Message: "passkey not accepted: its signature counter went backwards", Hint: "sign in another way, then remove this passkey in Settings and add it again", Status: 401}
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return "", err
	}
	id := b64.EncodeToString(cred.ID)
	var secret string
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.UpdateWebAuthnCredential(ctx, found.user.ID, id, raw, domain.FormatTime(s.now())); err != nil {
			return err
		}
		secret, err = s.mintLogin(ctx, found.user.ID, "passkey login token", map[string]any{"via": "passkey", "passkey_id": id})
		return err
	})
	return secret, err
}

func (s *Service) ListPasskeys(ctx context.Context) ([]PasskeyView, error) {
	a, err := s.passkeyOwner(ctx)
	if err != nil {
		return nil, err
	}
	u, err := s.loadPasskeyUser(ctx, a.UserID, false)
	if err != nil {
		return nil, err
	}
	out := make([]PasskeyView, len(u.rows))
	for i, r := range u.rows {
		created, _ := domain.ParseTime(r.CreatedAt)
		out[i] = PasskeyView{ID: r.ID, Name: r.Name, CreatedAt: created, Synced: u.creds[i].Flags.BackupState}
		if t, err := domain.ParseTime(r.LastUsedAt); err == nil {
			out[i].LastUsedAt = &t
		}
	}
	return out, nil
}

// DeletePasskey removes one passkey. Sessions it already started stay
// signed in; revoke those under API tokens.
func (s *Service) DeletePasskey(ctx context.Context, id string) (domain.Deleted, error) {
	a, err := s.passkeyOwner(ctx)
	if err != nil {
		return domain.Deleted{}, err
	}
	err = s.DB.WithTx(ctx, func(ctx context.Context) error {
		if err := s.DB.DeleteWebAuthnCredential(ctx, a.UserID, id); err != nil {
			return err
		}
		return s.audit(ctx, a, "passkey.delete", "passkey", id, "removed a passkey", nil)
	})
	if err != nil {
		return domain.Deleted{}, err
	}
	return domain.Deleted{ID: id, Deleted: true}, nil
}
