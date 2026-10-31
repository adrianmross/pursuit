# Passkeys plan: passkey sign-in for the hosted dashboard

**Status: built (option b).** Native WebAuthn in the Go server with `go-webauthn` v0.18.2, decisions D1 to D7 taken as recommended, with these differences from the plan below: registration and management live under `/v1/passkeys` (`/v1/auth/*` is unauthenticated), the challenge row is keyed by the challenge itself, the credential is stored as go-webauthn's JSON record rather than one column per field, a passkey login token is named `login` (so sign-out revokes it; the audit entry records `via: passkey`), and rename, conditional UI and "sign out other pursuit sessions" are not built yet. Owner recovery SQL is in `internal/spec/auth.md`.

The original plan follows. Before this, the dashboard at `https://hirepursuit.app` signs in through Neon's Managed Better Auth (Neon Auth, project `broad-poetry-45560961`) with email and password, email OTP verification, and password reset by code. Managed Neon Auth does not offer passkeys, so passkeys need either a different auth server, pursuit's own WebAuthn, or waiting.

Facts about Neon, Better Auth and the Go library were checked on 2026-10-31. Each one has a link. These change often, so re-check before building a phase.

## How sign-in works today

- **Hosting.** One Go function (`api/index.go`) serves the API and the statically exported Next.js dashboard. `vercel.json` rewrites every path to it. There is no Node runtime in the Vercel project; `scripts/vercel-build.sh` builds `web-app` with `build:self-hosted` and embeds the output.
- **Browser side.** `web-app/lib/betterauth.ts` builds `createAuthClient` from `better-auth/client` with `jwtClient()` and `emailOTPClient()` against `NEXT_PUBLIC_NEON_AUTH_URL`. `SignInForm.tsx` calls `signIn.email` / `signUp.email`, `emailOtp.verifyEmail`, `emailOtp.requestPasswordReset` / `resetPassword`, then `authClient.token()` for a JWT. `AccountPanel.tsx` uses `changeEmail`, `changePassword` and `revokeOtherSessions`.
- **Server side.** `Server.authenticate` (`internal/httpapi/http.go`) sends any non-`pst_` three-part token to `internal/betterauth`, which checks the EdDSA signature against `<NEON_AUTH_URL>/.well-known/jwks.json`, the issuer (the Neon Auth origin), and optionally the audience. It refuses the token unless `email_verified` (or `emailVerified`) is true. `Service.ResolveTenantUser` then finds the pursuit user by email, creating one only when `signupsOpen()` (multi-tenant and not `PURSUIT_SIGNUPS=closed`).
- **Session.** `POST /v1/auth/session` (same-origin JSON only) verifies the token and stores it as-is in the httpOnly, `SameSite=Lax`, 30-day `pursuit_session` cookie. For Neon Auth that token is the JWT itself, and Neon's JWTs "expire in 15 minutes" ([Neon JWT plugin](https://neon.com/docs/auth/guides/plugins/jwt.md)). The dashboard calls `authClient.token()` only in `finishSignIn`, so a Neon Auth dashboard session lasts about 15 minutes in practice. The generic OIDC callback (`authCallback`) does it differently: `Service.LoginIdentity` mints a `pst_` token named `login` with `read,write,approve` and puts that in the cookie.
- **Identity mapping.** Pursuit keeps its own `users` table (keyed by email) and `api_tokens`. Neon Auth's users, accounts and sessions live in the `neon_auth` schema of the same Neon database ([branching authentication](https://neon.com/docs/auth/branching-authentication.md)). Nothing in pursuit references a Neon Auth user id. The two meet only at the verified email.
- **CLI.** `pursuit auth login` uses `/authorize` (loopback + PKCE) or `/device` (RFC 8628). Consent is `POST /v1/oauth/authorize` or `POST /v1/oauth/device/approve` from a same-origin dashboard session that holds `approve`. The CLI never sees a Better Auth token.

## What the vendors offer

### Neon Managed Better Auth

- **No passkeys.** The plugin table lists Admin, Email OTP, JWT, Magic Link, OAuth Provider (beta), Organization (partial), Open API and Phone Number. "You don't install or configure Better Auth plugins directly." ([plugins](https://neon.com/docs/auth/guides/plugins.md)). `neon neon-auth plugins list` shows no passkey plugin, and the live auth server returns 404 for `/passkey/*`.
- **Roadmap.** "Managed Better Auth is generally available." Under plugins, "MFA support: Coming soon" and "Other plugins: Based on demand". Passkeys and WebAuthn are not mentioned, and there are no dates ([roadmap](https://neon.com/docs/auth/roadmap.md)). Neither term appears in the docs index ([llms.txt](https://neon.com/docs/llms.txt), [auth index](https://neon.com/docs/auth/llms.txt)).
- **Self-managed Better Auth** is Neon's documented escape hatch: "your app runs its own Better Auth server, with Lakebase Postgres as the auth database", only "when a required feature falls outside Managed Auth". It runs on an existing host that serves `/api/auth` (for example Vercel route handlers) or on Neon Functions. You cannot add plugins by changing only the client, because Managed Auth stays the backend. On leaving Managed Auth there is "no documented universal import from `neon_auth`". It says to inventory users, credentials, sessions and foreign keys first, and "Do not promise drop-in session continuity." ([self-managed reference](https://neon.com/docs/ai/skills/neon-auth/references/self-managed.md)).

### Better Auth passkey plugin

- Package `@better-auth/passkey`. The server plugin is `passkey()` with `rpID`, `rpName` and `origin`. The client plugin is `passkeyClient` from `@better-auth/passkey/client`. It is built on SimpleWebAuthn and adds a `passkey` table created by `migrate` / `generate` ([docs](https://www.better-auth.com/docs/plugins/passkey)).
- Client calls: `passkey.addPasskey`, `signIn.passkey` (with `autoFill: true` for conditional UI and `autocomplete="username webauthn"`), and `listUserPasskeys` / `deletePasskey` / `updatePasskey`.
- `registration.requireSession` defaults to `true`. Passkey-first sign-up (`false`) needs a `resolveUser` hook.
- **Not confirmed by the docs page** (verify before building option a): the exact `passkey` table columns, and whether a `signIn.passkey` session can be turned into a JWT through `jwtClient().token()` the same way an email sign-in can.

### go-webauthn

- `github.com/go-webauthn/webauthn` v0.18.2, published 2026-09-19. BSD-3-Clause. Pre-1.0, so the API can break. Supports Go 1.25 to 1.27 (pursuit is on 1.26). A descendant of `duo-labs/webauthn` ([repo](https://github.com/go-webauthn/webauthn), [package](https://pkg.go.dev/github.com/go-webauthn/webauthn/webauthn)).
- API: `webauthn.New(&Config{RPID, RPDisplayName, RPOrigins})`, `BeginRegistration` / `FinishRegistration(user, session, r)`, and for usernameless passkeys `BeginDiscoverableLogin` / `BeginDiscoverableMediatedLogin` (conditional UI) with `FinishPasskeyLogin(handler, session, r)`. The handler maps `(rawID, userHandle)` to a `webauthn.User`. Begin returns a `SessionData` (challenge, user id, expiry) that the caller must keep until Finish.

## Options

### (a) Run Better Auth ourselves with the passkey plugin

Replace Neon's managed server with our own Better Auth (email/password, email OTP, JWT, passkey) as a Node function on Vercel or a Neon Function, against the same Neon Postgres.

- **For:** one auth system with passkeys, 2FA and every other upstream plugin. The dashboard keeps the `better-auth/client` API it already uses. The Go verifier only needs a new JWKS URL and issuer.
- **New runtime.** The project is one Go function today. A Node auth server is either a second runtime in this Vercel project (route split in `vercel.json`, which `scripts/vercel-build.sh` calls "a separate, bigger reconfiguration"), a second Vercel project on a subdomain such as `auth.hirepursuit.app`, or a Neon Function. All three are new things to deploy, monitor and patch.
- **Same-site constraints.** The WebAuthn ceremony runs on `https://hirepursuit.app`, so the RP ID is `hirepursuit.app` wherever the server lives. Better Auth also keeps its session and WebAuthn challenge in cookies. On a Neon Functions domain those cookies are third-party, and browsers block them more each year. In practice the server must be on `hirepursuit.app/api/auth` or `auth.hirepursuit.app`.
- **Migration is ours.** No documented import from `neon_auth`, and Neon says not to promise session continuity. We would copy users, accounts and password hashes into Better Auth's schema and check the hash format matches, or ask everyone to reset their password. All current Neon Auth sessions end at cutover. That costs less than it sounds, because the cookie already expires within about 15 minutes.
- **We inherit what Neon runs today:** OTP and reset email delivery (a mail provider and templates), the JWKS signing keys and their rotation, rate limits, and auth-server uptime. An outage of our auth server is a sign-in outage.
- **Effort:** about 6 to 10 days plus cutover, and the auth server stays ours to run.

### (b) Native WebAuthn in the Go server

Keep Neon Auth for email/password, verification and reset. Add WebAuthn to the Go API with `go-webauthn`. A passkey signs in an existing pursuit user and mints a pursuit session directly, the way the OIDC callback already does.

- **For:** no new runtime and no auth-data migration. Credentials hang off pursuit's own `users.id`, so they survive a Neon Auth change, and they work for self-hosted `pursuit serve` too. It reuses the existing session cookie, same-origin checks, `LoginIdentity` token minting and audit log. One new Go dependency.
- **Against:** we own a security-sensitive flow: challenge storage, sign-count checks, credential management UI. It does not give Neon Auth itself MFA, so password sign-in stays as strong as the password plus email OTP. Two places hold a user's credentials (Neon for the password, pursuit for passkeys), so "remove this account's sign-in methods" touches both.
- **Effort:** about 5 to 7 days (see the phases below).

### (c) Wait for Neon

- **For:** zero work. When it ships it is part of the managed service.
- **Against:** no signal it is coming. The roadmap names MFA, not passkeys, and has no dates. Even if Neon's MFA uses WebAuthn, it would be a second factor after the password, not passwordless sign-in. Waiting costs nothing to reverse, so it fits as the default only if passkeys are not wanted soon.

## Recommendation: (b), native WebAuthn in Go, passkeys as an extra sign-in method

Option (b) is the smallest change that ships passwordless sign-in and leaves nothing new to host. (a) is the right move only if pursuit wants several Better Auth plugins Neon lacks, and then it is a migration project, not a passkey project. File a passkey request with Neon either way (D6). If Neon ships first-party passkeys later, (b) stays as is or is retired, with no data migration in either direction.

### Mapping onto pursuit

**Data.** One migration in `internal/sqlstore/migrate.go`, for SQLite and Postgres:

- `webauthn_credentials`: `id` (credential ID, base64url, primary key), `user_id` → `users.id`, `public_key`, `sign_count`, `aaguid`, `transports`, `backup_eligible`, `backup_state`, `attestation_type`, `name` (user label, for example "MacBook"), `created_at`, `last_used_at`. Index on `user_id`.
- `webauthn_challenges`: `id`, `kind` (`register` or `login`), `user_id` (empty for discoverable login), `session_json` (the `SessionData`), `expires_at` (5 minutes), `used_at`. It has the same shape and single-use rule as `oauth_grants`. Vercel instances are stateless, so the challenge cannot live in memory. A signed cookie would also work, but the database row makes single use easy and matches the OAuth codes.

The WebAuthn user handle is a random 32-byte value stored on `users` (`webauthn_user_handle`), not the email and not `users.id`, so the authenticator never holds anything that identifies the person.

**Endpoints** in `internal/httpapi` (new `passkeys.go`), service methods in `internal/service`:

| route | auth | does |
|---|---|---|
| `POST /v1/auth/passkey/register/begin` | dashboard session with `approve` | `BeginRegistration` with `ResidentKey: required`, `UserVerification: required`, existing credentials excluded |
| `POST /v1/auth/passkey/register/finish` | same | `FinishRegistration`, store the credential, audit `passkey.create` |
| `POST /v1/auth/passkey/login/begin` | none | `BeginDiscoverableMediatedLogin` (conditional UI) or `BeginDiscoverableLogin` (button) |
| `POST /v1/auth/passkey/login/finish` | none | `FinishPasskeyLogin`, update `sign_count` and `last_used_at`, mint a `login` token, set `pursuit_session`, answer like `GET /v1/me` |
| `GET /v1/passkeys`, `PATCH`/`DELETE /v1/passkeys/{id}` | `approve` | list, rename and remove, audit `passkey.delete` |

Every route goes through `sameOriginJSON`, like `POST /v1/auth/session`. Login begin and finish get a per-IP limiter like `deviceLimiterFor`. `/v1/auth/providers` gains `passkeys: true` when an RP ID is configured, so the sign-in page knows to offer the button. The routes go into `internal/spec/doc.go` so `/openapi.json` and `llms.txt` stay complete.

**Dashboard.** `SignInForm.tsx` gets a "Sign in with a passkey" button and `autocomplete="username webauthn"` on the email field, with conditional mediation started on mount when `PublicKeyCredential.isConditionalMediationAvailable()` is true. `AccountPanel.tsx` gets a Passkeys section (add, rename, remove, last used). The browser calls are `navigator.credentials.create/get` with the JSON from begin. `PublicKeyCredential.parseCreationOptionsFromJSON` covers current browsers. `@simplewebauthn/browser` is the fallback if Safari's support is still uneven (D4).

### RP ID and origin

- `RPID: "hirepursuit.app"`, `RPDisplayName: "Pursuit"`, `RPOrigins: ["https://hirepursuit.app"]`. The RP ID is the registrable domain, so a later `app.hirepursuit.app` or `www.` also works if added to `RPOrigins`. A passkey made for `hirepursuit.app` never works on another domain, so this is a one-way choice (D3).
- Config: `PURSUIT_WEBAUTHN_RP_ID` and `PURSUIT_WEBAUTHN_ORIGINS` (comma-separated), read in `internal/config` and `pkg/vercelapp`. Passkeys are off when the RP ID is unset. That keeps preview deployments (`*.vercel.app`, behind deployment protection) and self-hosted installs off until configured.
- Local dev: `localhost` is a valid RP ID with `http://localhost:<port>` origins. Split-origin dev (`PURSUIT_API_URL` set, `COOKIE_SESSION` false) has no cookie session, so passkeys are tested against same-origin `pursuit serve`.
- CSP already allows `connect-src 'self'`. WebAuthn needs no CSP change. Set `Permissions-Policy: publickey-credentials-get=(self), publickey-credentials-create=(self)` with the existing security headers.

### Existing users and sessions

- **No migration.** Neon Auth users, passwords and sessions stay where they are. A pursuit user adds a passkey from Settings while signed in, and their next sign-in can use it. Users without a passkey see no change.
- **Sessions in flight are untouched.** Passkey login writes the same `pursuit_session` cookie. A user already signed in with a Neon JWT keeps that cookie until it expires.

### `/v1/auth/session`, `email_verified`, `PURSUIT_SIGNUPS`, and the CLI

- **Session cookie.** Passkey login does not go through `POST /v1/auth/session`. Like `authCallback`, it mints a `login` token (`read,write,approve`) and calls `setSessionCookie` directly, so the browser never holds the secret. Name the token `passkey login` and record the credential id in the audit metadata, so Settings, Tokens shows where a session came from. `DELETE /v1/auth/session` signs out the same way. This means passkey sessions last the cookie's 30 days, while Neon JWT sessions last about 15 minutes. That matches the OIDC path, but it is a real difference (D5).
- **`email_verified`.** Registration needs a signed-in pursuit session, and that session came from a verified Neon JWT, a verified OIDC identity, or a `pst_` token. So a passkey only ever attaches to an account whose email was already proven. Login resolves a credential to `users.id` directly and checks no email. Refuse registration from demo tenants and from the `PURSUIT_TRUST_PROXY_AUTH=vercel` owner, like the OAuth consent does.
- **`PURSUIT_SIGNUPS`.** Passkey sign-in never creates a user. A credential with no user, or a deleted user, answers 401. So `PURSUIT_SIGNUPS=closed` has nothing to gate, and existing users keep signing in with passkeys while sign-ups are closed. That matches the toggle's intent. No passkey-first sign-up (D2).
- **CLI OAuth and device flow.** No change. `/authorize` and `/device` need a dashboard session with `approve`, and a passkey session has it. The new routes never issue tokens to the CLI, and OAuth still caps CLI grants at `read,write`. Passkeys make approving a CLI login faster (passkey, then approve). They do not put passkeys in the CLI.
- **Neon-side controls.** `revokeOtherSessions` in `AccountPanel.tsx` signs out Neon sessions only. It cannot see pursuit `login` tokens. The Passkeys section should sit next to a "sign out other pursuit sessions" action that revokes the user's other `login` tokens, or the Neon button should be relabelled so it does not over-promise.

### Account recovery

- Passkeys are added on top of the password, never instead of it. A user who loses every passkey signs in with email and password, or resets the password by emailed code, then removes the lost passkey in Settings. Neon Auth stays the recovery path, so there is no new recovery code.
- A user who still has a session but loses a device removes that device's passkey in Settings. Removing a passkey does not revoke the sessions it created. "Sign out other pursuit sessions" does that.
- Owner override: if someone loses both password and passkeys, the owner deletes the user's rows in `webauthn_credentials` and the user goes through Neon's password reset. It needs no tool at first; write the SQL down in `AUTH.md`.
- Not planned: passkey-only accounts with no password (D2), and recovery codes. Both add a recovery path Neon cannot back up.

### Phases

**Phase 0: verify (0.5 day, no code).** Re-check the Neon roadmap. Confirm `go-webauthn`'s latest version and its discoverable-login example. Check `parseCreationOptionsFromJSON` support in current Safari, Chrome and Firefox. Decide D1 to D5.

**Phase 1: server (2 to 3 days).** Migration, store methods, `go-webauthn` config from env, the six routes, challenge expiry and single use, per-IP limiting on login, audit entries, spec entries. Tests reuse the httpapi test env with a software authenticator: `go-webauthn` ships test helpers for this, otherwise use a small fixture built with `crypto/ecdsa` and `fxamacker/cbor` as a test-only dependency. Cover: register, login, unknown credential, replayed challenge, expired challenge, wrong origin, sign-count regression, cross-site request refused, demo and proxy-owner refused.

**Phase 2: dashboard (1.5 to 2 days).** Settings Passkeys section, sign-in button, conditional UI, error states (cancelled, no passkey on this device, not supported). Manual check on macOS Safari (iCloud Keychain), Chrome with Google Password Manager, a hardware key, and an iPhone through the cross-device QR flow.

**Phase 3: ship (0.5 day).** Set `PURSUIT_WEBAUTHN_RP_ID=hirepursuit.app` and `PURSUIT_WEBAUTHN_ORIGINS=https://hirepursuit.app` on production only. Document the feature and the owner recovery SQL in `AUTH.md` and the docs site. Changelog fragment for the user-facing change.

**Phase 4, optional: session hygiene (1 day).** "Sign out other pursuit sessions", and passkey step-up for sensitive actions (creating an `approve` token, approving a CLI login) once a user has a passkey.

Total: about 5 to 7 days, including Phase 4.

### Risks

- **Getting WebAuthn wrong.** Origin and RP ID checks, challenge single use and sign-count handling are where hand-rolled WebAuthn breaks. Leave all verification to `go-webauthn` and keep the pursuit code to storage and routing. Test the failure cases listed in Phase 1.
- **Pre-1.0 dependency.** `go-webauthn` can make breaking changes. Pin it, and keep it inside one file in `internal/httpapi` or a small `internal/webauthn` package, so an upgrade touches one place.
- **Synced passkeys.** Most passkeys sync through iCloud, Google or 1Password, so a compromised cloud account carries the passkey with it. That is the normal passkey model and still stronger than a password. Record `backup_state` so a later policy could require device-bound keys for `approve` step-up.
- **Long-lived sessions.** A passkey login yields a 30-day cookie holding a non-expiring `pst_` token. Revoking it from Settings, Tokens works today. Session expiry is D5.
- **Two credential stores.** Deleting a Neon Auth user does not delete their passkeys. Deleting the pursuit user must cascade to `webauthn_credentials`, and the owner-side user deletion path needs a test for that.
- **Enumeration.** Discoverable login never takes an email, so begin and finish reveal nothing about which accounts exist. Keep it that way: no "passkey for this email?" lookup endpoint.

## Decisions for the owner

- **D1. Option.** (b) native Go WebAuthn, (a) self-run Better Auth, or (c) wait. Recommended: (b), plus a feature request to Neon.
- **D2. Passkey-only and passkey-first accounts?** Allow sign-up with just a passkey, or deleting the password once a passkey exists? Recommended: no. Both bypass Neon's email verification and leave no recovery path.
- **D3. RP ID.** `hirepursuit.app` is permanent for every passkey registered against it. Confirm that is the long-term domain, and whether `www.` or an `app.` subdomain should be in `RPOrigins` from the start.
- **D4. Browser helper.** Native `PublicKeyCredential.parse*FromJSON` with no new npm dependency, or `@simplewebauthn/browser` for broader browser support. Recommended: native, falling back to the library only if Phase 0 finds a gap.
- **D5. Session lifetime.** Passkey (and OIDC) sessions mint a `pst_` that never expires behind a 30-day cookie, and Neon JWT sessions end after about 15 minutes. Should `login` tokens get an expiry, and should the Neon path exchange its JWT for a `login` token too, so all three behave the same? That is a separate change, but it should be settled before passkeys widen the gap.
- **D6. Self-hosted.** Ship passkeys for `pursuit serve` installs too (they only need `PURSUIT_WEBAUTHN_RP_ID`), or hosted only? Recommended: both, because the code is the same. Separately, file a passkey request with Neon so option (c) stays possible.
- **D7. New dependency.** `go-webauthn` takes `go.mod` from five direct dependencies to six, plus its transitive ones (CBOR, JWT for the metadata service). Accept, or hand-roll the subset? Recommended: accept. Hand-rolled WebAuthn verification is not worth the risk.
