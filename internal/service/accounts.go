package service

import (
	"strings"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/adrianmross/appkit/account"
)

// DefaultCLIClientID is the client_id of `pursuit auth login`.
const DefaultCLIClientID = "pursuit-cli"

// Accounts is appkit's account layer on Pursuit's tables. It serves the
// OAuth server (`pursuit auth login`, and the apps that register themselves
// with their refresh tokens) and passkeys. Sessions and tokens are still
// this package's own.
// ponytail: built per call, like sessions(), because tests set Passkeys and
// ClientDoc after the server exists; build it once when sessions move here.
func (s *Service) Accounts() *account.Accounts {
	return &account.Accounts{
		DB: s.DB.Core(), App: "pursuit", EnvPrefix: "PURSUIT_", TokenMarker: "pst_",
		Scopes: []string{"read", "write", "approve"}, IdleTTL: LoginIdleTTL, MaxTTL: LoginMaxTTL,
		Now: s.now, NewID: s.NewID,
		OAuth: &account.OAuth{
			ClientID: DefaultCLIClientID, ClientName: DefaultCLIClientID,
			// OAuth never grants approve: approving stays a dashboard action,
			// or an explicit PURSUIT_TOKEN a human minted on purpose.
			AllowScope: func(scope string) bool { return scope == "read" || scope == "write" },
			Clients: &account.OAuthClients{
				PublicURL: s.PublicURL, RefreshMarker: "psr_", IDMarker: "pc_", AnyLoopback: true, FetchDoc: s.ClientDoc,
			},
		},
		Passkeys: s.Passkeys,
	}
}

// NewPasskeys returns nil when rpID is empty: passkeys are off until
// PURSUIT_WEBAUTHN_RP_ID is set. A passkey is only added by a signed-in user
// and only signs that user in again, so PURSUIT_SIGNUPS has nothing to gate.
func NewPasskeys(rpID string, origins []string) (*account.Passkeys, error) {
	rpID = strings.TrimSpace(rpID)
	if rpID == "" {
		return nil, nil
	}
	// appkit reads the settings on first use; check them here so a bad one
	// stops the boot instead of the first sign-in.
	if _, err := webauthn.New(&webauthn.Config{RPID: rpID, RPDisplayName: "Pursuit", RPOrigins: origins}); err != nil {
		return nil, err
	}
	return &account.Passkeys{RPID: rpID, Origins: origins, DisplayName: "Pursuit"}, nil
}
