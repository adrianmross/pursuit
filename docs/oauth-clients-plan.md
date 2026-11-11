# Third-party OAuth clients (ChatGPT, Claude connectors)

Goal: a hosted app such as a ChatGPT app or a Claude connector signs a user in to Pursuit and calls the API as them, without the user pasting a token. Both reach Pursuit through a remote MCP server, and MCP authorizes with OAuth 2.1: authorization code with PKCE, refresh tokens, discovery metadata, and clients that register themselves.

Findings below come from the vendors' own docs, fetched 2026-11-10: [Claude](https://claude.com/docs/connectors/building/authentication), [ChatGPT](https://developers.openai.com/apps-sdk/build/auth), and the [MCP authorization spec](https://modelcontextprotocol.io/specification/latest/basic/authorization).

Access tokens stay opaque `pst_` tokens. MCP treats them as opaque and the server checks the database on every request, so a JWT adds nothing here (see "Why not JWT" below).

## Where the code is today

`internal/service/oauth.go`, `internal/httpapi/oauth.go`, `internal/sqlstore/oauth.go`.

| Piece | State |
| --- | --- |
| Authorization code + PKCE (S256) | Yes, but redirect URIs must be `http://127.0.0.1:<port>/callback` or `[::1]`. No `https` redirects. |
| Device flow (RFC 8628) | Yes. |
| Client registry | None. `client_id` is any string up to 64 characters; empty becomes `pursuit-cli`. Nothing is stored per client. |
| Access tokens | Opaque `pst_`, named `cli`, no expiry, no `expires_in` in the response, not linked to a client. |
| Refresh tokens | None. `grant_type=refresh_token` is `unsupported_grant_type`. |
| Discovery metadata | None. No `.well-known` routes. |
| `resource` parameter (RFC 8707) | Ignored. |
| Scopes | `read`, `write`. `approve` is never granted over OAuth. Keep this. |
| Consent | Dashboard page `/authorize` calls `POST /v1/oauth/authorize`, gated by `humanApprover`. Reuse it. |

## Phases

Each phase is its own PR and leaves the CLI flow working.

### 1. Discovery metadata (built)
Served by `internal/httpapi/oauth_metadata.go`. It advertises only what works today: S256, `none` auth, `read`/`write`, the authorization-code and device grants. `registration_endpoint`, `client_id_metadata_document_supported`, `authorization_response_iss_parameter_supported`, `refresh_token` and `offline_access` are added by the phase that ships each, so the document never promises an endpoint that 404s. The list below is the end state.
- `GET /.well-known/oauth-authorization-server` (RFC 8414). Fields both clients check: `issuer`, `authorization_endpoint`, `token_endpoint`, `revocation_endpoint`, `registration_endpoint`, `code_challenge_methods_supported: ["S256"]` (both refuse the server without it), `token_endpoint_auth_methods_supported` including `"none"` (Claude's CIMD client is a public client), `client_id_metadata_document_supported: true`, `authorization_response_iss_parameter_supported: true`, `scopes_supported: ["read", "write", "offline_access"]`, `grant_types_supported`, `response_types_supported: ["code"]`. Claude requests `offline_access` when it is listed, which is how it gets a refresh token.
- `GET /.well-known/oauth-protected-resource` and the path-suffixed form (RFC 9728): `resource` must equal the MCP URL exactly as a user enters it, including any path (Claude); `authorization_servers` holds the same string as `issuer` (ChatGPT); Claude uses only the first entry.
- Issuer and base URL come from config for hosted, so a Host header cannot change them. Discovery must be reachable from Anthropic's egress range (160.79.104.0/21).
- Listed in `internal/spec/doc.go` and `api/openapi.json` (`make openapi`).

### 2. Client identity: CIMD first, DCR as fallback (built)
Deviations from the first draft: registration is `POST /v1/auth/register` (`/v1/oauth/*` is the signed-in dashboard API); the CIMD document is cached a fixed hour instead of reading `Cache-Control`; DCR clients may also use loopback redirects (Claude Code registers that way); an unregistered plain `client_id` keeps today's loopback-only behavior so existing local tools do not break, instead of being refused; client IDs may be up to 512 characters when they are URLs; CIMD URLs must use port 443. The consent page still says "on this computer" and does not yet show `client_name` or `unverified`; that is phase 3.
Both vendors prefer Client ID Metadata Documents, and the MCP spec now says servers SHOULD support CIMD and marks DCR deprecated (kept for compatibility). Claude's DCR registers a new client on every fresh connection, so open DCR fills the table; ChatGPT registers once per connection. Support both, CIMD as the main path.

- **CIMD:** `client_id` is an https URL (ChatGPT: `https://chatgpt.com/oauth/client.json` or `https://chatgpt.com/oauth/{callback_id}/client.json`). On first sight the server fetches the document and requires its `client_id` to equal the URL exactly; `client_name` and `redirect_uris` come from it. Needs a new outbound fetcher with SSRF protection: https only, no redirects, 5 second timeout, 64 KB cap, resolve the host and refuse private, loopback and link-local addresses, pin the resolved IP for the connection. Nothing in `internal/links` does this today, so it is new code. Cache the document (honor `Cache-Control`, cap at 1 hour) and record the client in `oauth_clients`.
- **DCR:** `POST /v1/oauth/register` (RFC 7591), JSON, public clients only (`token_endpoint_auth_method: none`), `application_type` required. Same redirect rules as below; per-IP rate limit, global cap, delete clients with no grant after 30 days.
- **Table:** `oauth_clients(id, name, redirect_uris, source, created_at, last_used_at)`; `source` is `cimd`, `dcr` or `builtin`. `pursuit-cli` stays built in and loopback-only.
- **Redirects:** https exactly as declared (known today: `https://claude.ai/api/mcp/auth_callback`; ChatGPT `https://chatgpt.com/connector_platform_oauth_redirect` or `https://chatgpt.com/connector/oauth/{callback_id}`). Claude Code declares `http://localhost/callback` and `http://127.0.0.1/callback` and expects the port to be ignored, so loopback matching ignores the port. See decision 1.
- `/authorize` refuses unknown clients instead of defaulting to `pursuit-cli`.

### 3. Authorize and consent for registered clients (built)
Shipped: `iss` on every authorization response and advertised in metadata; `resource` accepted on authorize (must be on this server's host), stored on the grant (migration `027`), and a token request naming a different resource gets `invalid_target`; the consent page shows the client name, redirect host and an unverified warning, and only forwards the known query parameters. Binding the resource to the access token waits for phase 4, where tokens gain a `resource` column.
- `checkAuthorize` loads the client, matches the redirect URI exactly, and stores the `resource` parameter on the grant (ChatGPT sends it on both the authorize and token requests; the MCP spec says to bind the token to it).
- Every authorization response, success or error, carries `iss` equal to the metadata `issuer` (RFC 9207). ChatGPT rejects the response if we advertise `authorization_response_iss_parameter_supported` and the value is missing or different; with it, ChatGPT can use its stable redirect URI.
- The consent page shows the client name, the redirect host, the scopes, and an "unverified app" line, because anyone can register. Scopes remain `read`/`write`; `approve` can never be granted to a registered client, enforced in `checkGrantable` and tested.
- Existing step-up applies: a stale session is asked for its passkey before consent (`humanApprover` already calls `requireFresh`).

### 4. Short-lived tokens and refresh rotation (built)
Shipped: registered clients get a 1 hour `pst_` access token plus a rotating `psr_` refresh token (30 days idle, 90 absolute); tokens carry `client_id`, `resource`, `expires_at` and `family_id` (migration `028`); `Authenticate` rejects expired tokens; reuse of a refresh token revokes the whole family; `DELETE /v1/tokens/{id}` and RFC 7009 revoke on either token kill the family; a refresh revokes the family's older access token so a connection has one live token; Settings has a "Connected apps" panel and the API tokens list hides app tokens. `offline_access` is accepted and ignored because registered clients always get a refresh token. Known gaps: two refreshes racing with the same token count as reuse (no grace window); a connection whose refresh chain died stays listed until disconnected. The resource check on use belongs to the MCP endpoint in phase 5.
- Migration: `api_tokens` gains `client_id` and `expires_at`; `Authenticate` rejects expired tokens. New table `oauth_refresh_tokens(hash, family_id, client_id, user_id, scopes, created_at, used_at, expires_at)`.
- The access token records its `resource`; the MCP endpoint rejects a token whose resource is not its own URL.
- Registered clients get a 1 hour access token and a refresh token (30 days idle, 90 days absolute, matching the dashboard session lifetimes). The token response gains `expires_in` and `refresh_token`.
- Claude refreshes on a 401 and up to 5 minutes before `expires_in`, waits up to 30 seconds for the refresh call, and expects `invalid_grant` for a dead refresh token and the new refresh token in the same response that retires the old one. `/token` must accept form-encoded bodies (it already does).
- `grant_type=refresh_token`: burn the presented token, issue a new access and refresh pair in the same family. A refresh token presented a second time revokes the whole family and its live access tokens (reuse detection, OAuth 2.1 section 4.3.1).
- `pursuit-cli` and device-flow tokens keep their current non-expiring behavior until we decide otherwise, so installed CLIs do not break.
- Settings: a "Connected apps" section, one row per client (name, scopes, last used), with Revoke, which kills the refresh family and access tokens. The tokens list already hides them under their client.
- Audit entries for registration, consent, refresh, reuse detection and revoke.

### 5. MCP endpoint (separate branch, after 1 to 4 land)
- `POST /mcp` (streamable HTTP) as a thin layer over the existing service methods. Unauthenticated or bad tokens get a real 401 (Claude ignores `WWW-Authenticate` on a 200) with `WWW-Authenticate: Bearer resource_metadata="...", scope="read write"`. ChatGPT's tool-level sign-in prompt also wants per-tool `securitySchemes` and an `_meta["mcp/www_authenticate"]` error; check the Apps SDK docs when we get here.
- Reverses the `AGENTS.md` line "Do not look for an MCP server"; update that file and `skills/pursuit/SKILL.md` in the same PR.
- Tools mirror the CLI: list, get, prepare, enqueue. Nothing that approves, rejects, or submits.

## Why not JWT

The MCP endpoint is in the same binary and checks the database on every request, so a signed token saves nothing and loses instant revocation, reuse detection and the Connected apps list. Revisit only if a separate service must verify tokens without calling Pursuit.

## Security notes
- Registration is open by design, so the damage limit is the consent screen and the scope cap, not trust in the client. Redirect matching is exact. No wildcard or prefix matching.
- Authorization codes stay single use, 60 seconds, bound to challenge, redirect URI and client.
- `state` stays required. Refuse the implicit grant and the password grant.
- Never log codes, refresh tokens or access tokens (the existing `logOAuth` rule).
- Refresh token hashes only in the database, like access tokens.

## Tests to leave behind
- Registration validation table (bad schemes, fragments, too many URIs, name length, rate limit).
- Redirect mismatch refused; unknown client refused; CLI loopback still works.
- Refresh happy path, expired refresh, replayed refresh revokes the family.
- Access token expiry enforced by `Authenticate`; registered client cannot obtain `approve`.
- Metadata documents match the routes actually served.

## Decisions from the research
1. **CIMD plus DCR, CIMD first.** Both ChatGPT and Claude support both. The earlier "which does each use" question is answered: neither requires DCR. One third-party guide says the ChatGPT App Directory requires DCR; OpenAI's own page says the builder chooses, so DCR is a fallback we ship anyway.
2. **Loopback redirects:** Claude Code declares `localhost` as well as `127.0.0.1`. Current code refuses `localhost`. Proposal: ignore the port, accept `127.0.0.1`, `[::1]` and `localhost` for CIMD clients only, keep the CLI's stricter rule. Decided: yes (2026-11-10).
3. **Scopes:** advertise `read`, `write` and `offline_access`. Claude asks for the metadata scopes unless the 401 names some, so the 401 should say `read`; the consent screen lets the user grant write.
4. **Hosted routing:** `vercel.json` rewrites every path to the Go function and serves the static dashboard from `public`, so `.well-known`, `/authorize` and the API share one origin there. I have not confirmed whether production runs this build or the Next proxy variant (`PURSUIT_API_URL`); confirm before phase 1.
5. **CLI tokens** keep their current behavior; revisit after phase 4.

## Still open
- Whether to accept `private_key_jwt` clients (both vendors can use it for CIMD). Public clients are enough to start.
- The exact set of tools the MCP endpoint exposes (phase 5).
