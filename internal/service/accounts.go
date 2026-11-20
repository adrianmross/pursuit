package service

import (
	"github.com/adrianmross/appkit/account"
)

// DefaultCLIClientID is the client_id of `pursuit auth login`.
const DefaultCLIClientID = "pursuit-cli"

// Accounts is appkit's account layer on Pursuit's tables. It serves the
// OAuth server: `pursuit auth login`, and the apps that register themselves
// (a ChatGPT app, a Claude connector) with their refresh tokens. Sessions,
// tokens and passkeys are still this package's own.
// ponytail: built per call, like sessions(), because tests set WebAuthn and
// ClientDoc after the server exists; build it once when passkeys move here.
func (s *Service) Accounts() *account.Accounts {
	a := &account.Accounts{
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
	}
	if s.WebAuthn != nil {
		// Only so approving a login asks a user who has a passkey to have
		// used it recently; the passkey routes are still Pursuit's.
		a.Passkeys = &account.Passkeys{}
	}
	return a
}
