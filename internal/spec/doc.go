package spec

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/pursuit/pursuit/internal/version"
	"github.com/pursuit/pursuit/skills"
)

// LLMS is the agent guide served at /llms.txt. The root llms.txt points here.
//
//go:embed llms.txt
var LLMS []byte

// Auth is the agent auth guide served at /auth.md. The root AUTH.md points here.
//
//go:embed auth.md
var Auth []byte

// Skill is the Agent Skill served at /skill.md. It lives at skills/pursuit/SKILL.md
// so the skills CLI can install it from the repo.
var Skill = skills.Pursuit

// Reference is the standalone Scalar API-reference page served at
// /reference. It renders the same-origin /openapi.json, so it works
// identically self-hosted or behind api.<domain> in hosted mode.
//
//go:embed reference.html
var Reference []byte

// Document is the OpenAPI description served at /openapi.json.
func Document() []byte {
	if len(cached) == 0 {
		cached = build()
	}
	out := make([]byte, len(cached))
	copy(out, cached)
	if out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return out
}

var cached []byte

func build() []byte {
	paths := map[string]any{}
	// text is "Title. Details": the title is the sidebar entry, the rest the description.
	add := func(path, method, text string, write bool, extra map[string]any) {
		title, desc := text, ""
		for i := 0; i+1 < len(text); i++ {
			if (text[i] == '.' || text[i] == ';') && text[i+1] == ' ' {
				title, desc = text[:i], text[i+2:]
				break
			}
		}
		op := map[string]any{
			"summary":     upper(title),
			"operationId": method + "_" + slug(path),
			"responses": map[string]any{
				"200":     map[string]any{"description": "JSON body"},
				"default": errRef(),
			},
		}
		if tag := tagOf(path); tag != "" {
			op["tags"] = []string{tag}
		}
		if desc != "" {
			sentences := strings.Split(desc, ". ")
			for i, s := range sentences {
				sentences[i] = upper(s)
			}
			op["description"] = strings.Join(sentences, ". ") + "."
		}
		if write {
			op["parameters"] = []any{map[string]any{"$ref": "#/components/parameters/IdempotencyKey"}, map[string]any{"$ref": "#/components/parameters/DryRun"}}
		}
		for k, v := range extra {
			op[k] = v
		}
		item, _ := paths[path].(map[string]any)
		if item == nil {
			item = map[string]any{}
		}
		item[method] = op
		paths[path] = item
	}
	pub := map[string]string{
		"/healthz": "liveness", "/readyz": "readiness", "/version": "version", "/openapi.json": "this document", "/llms.txt": "short agent guide", "/auth.md": "agent auth guide: tokens, scopes, approval", "/skill.md": "Agent Skill for the pursuit CLI", "/reference": "interactive API reference page (HTML)",
	}
	// Meta routes stay in the document for agents; x-internal hides them in /reference.
	hidden := map[string]any{"security": []any{}, "x-internal": true}
	for p, s := range pub {
		add(p, "get", s, false, hidden)
	}
	form := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"required": true, "content": map[string]any{"application/x-www-form-urlencoded": map[string]any{
			"schema": map[string]any{"type": "object", "properties": props, "required": required},
		}}}
	}
	str := map[string]any{"type": "string"}
	oauthResp := func(desc string, props map[string]any) map[string]any {
		return map[string]any{
			"200": map[string]any{"description": desc, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": props}}}},
			"400": map[string]any{"description": "RFC 6749 error", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/OAuthError"}}}},
		}
	}
	add("/v1/auth/device_authorization", "post", "Start a device login. RFC 8628 device authorization for pursuit auth login --device. scope is space or comma separated, each exactly read or write, default read write; anything else (approve, APPROVE, Read) is invalid_scope: OAuth never grants approve. client_id is at most 64 characters. limited to 10 starts a minute per client IP (429 slow_down). device codes last 10 minutes; poll /v1/auth/token no faster than interval. there is no verification_uri_complete: the human types the code", false, map[string]any{
		"security":    []any{},
		"requestBody": form(map[string]any{"client_id": str, "scope": str}),
		"responses": oauthResp("device authorization", map[string]any{
			"device_code": str, "user_code": str, "verification_uri": str,
			"expires_in": map[string]any{"type": "integer"}, "interval": map[string]any{"type": "integer"},
		}),
	})
	add("/v1/auth/token", "post", "Get an access token. OAuth2 token endpoint. grant_type=authorization_code takes code, code_verifier (PKCE S256), redirect_uri, client_id; codes are single-use and last 60 seconds. grant_type=urn:ietf:params:oauth:grant-type:device_code takes device_code, client_id and answers authorization_pending, slow_down, expired_token, or access_denied until approved. The access_token is a normal pst_ API token", false, map[string]any{
		"security": []any{},
		"requestBody": form(map[string]any{
			"grant_type": map[string]any{"type": "string", "enum": []string{"authorization_code", "urn:ietf:params:oauth:grant-type:device_code"}},
			"code":       str, "code_verifier": str, "redirect_uri": str, "device_code": str, "client_id": str,
		}, "grant_type"),
		"responses": oauthResp("token", map[string]any{"access_token": str, "token_type": str, "scope": str}),
	})
	add("/v1/auth/revoke", "post", "Revoke a token. RFC 7009 token revocation. holding the token is the authority; unknown tokens still return 200. `pursuit auth logout` calls this", false, map[string]any{
		"security":    []any{},
		"requestBody": form(map[string]any{"token": str}, "token"),
		"responses":   oauthResp("revoked", map[string]any{"revoked": map[string]any{"type": "boolean"}}),
	})
	add("/v1/oauth/authorize", "get", "Check a browser login. Validate a loopback request for the dashboard /authorize page without deciding it. query client_id, redirect_uri, code_challenge, code_challenge_method, state, scope (response_type is accepted and ignored). requires the approve scope. returns {client_id, scopes, redirect_uri} as the server normalized them; a scope other than read or write is 400 invalid_scope", false, nil)
	add("/v1/oauth/authorize", "post", "Approve a browser login. Loopback consent, called by the dashboard /authorize page that pursuit auth login opens. requires a signed-in human session with the approve scope (not the PURSUIT_TRUST_PROXY_AUTH owner), Content-Type application/json, and Sec-Fetch-Site same-origin or an Origin matching the host. writes an oauth.approve or oauth.deny audit entry. body {client_id, redirect_uri, code_challenge, code_challenge_method:S256, state, scope, approve}. redirect_uri must be http://127.0.0.1:<port>/callback or http://[::1]:<port>/callback. returns {redirect_to}: the callback with code and state, or error=access_denied when approve is false", false, nil)
	add("/v1/oauth/device", "get", "Get a device login. Look up a pending device login by user_code (query) so the dashboard can show the requested scopes. requires the approve scope. returns {user_code, client_id, scopes, status, created_at, expires_at, requested_from} (requested_from is the IP that started the login)", false, nil)
	add("/v1/oauth/device/approve", "post", "Approve a device login. Approve or deny a device login from the dashboard /device page. body {user_code, approve}. requires a signed-in human session with the approve scope (not the PURSUIT_TRUST_PROXY_AUTH owner), Content-Type application/json, and Sec-Fetch-Site same-origin or an Origin matching the host; the code alone approves nothing. writes an oauth.approve or oauth.deny audit entry. returns {status: approved|denied}", false, nil)
	open := map[string]any{"security": []any{}}
	add("/v1/auth/providers", "get", "List sign-in options. Returns tenant_mode, providers, allow_token_login, demo_available, signups_open, passkeys (true when PURSUIT_WEBAUTHN_RP_ID is set)", false, open)
	add("/v1/auth/{provider}/start", "get", "Start an OIDC sign-in. Redirects to the provider", false, open)
	add("/v1/auth/{provider}/callback", "get", "Finish an OIDC sign-in. Redirects to /#token=<secret>", false, open)
	add("/v1/auth/session", "post", "Sign in. Verify a token (pst_, or a Better Auth JWT exchanged for a pst_ login token) and store the pst_ token in the httpOnly pursuit_session cookie, then answer like GET /v1/me. requires Content-Type application/json and Sec-Fetch-Site same-origin or a matching Origin. body {token}. agents send Authorization: Bearer instead", false, open)
	add("/v1/auth/session", "delete", "Sign out. Clear the pursuit_session cookie and revoke it when sign-in minted it (token name login). any other token stays valid; revoke it with POST /v1/auth/revoke. same-origin only; returns 204", false, open)
	add("/v1/auth/passkey/login/begin", "post", "start a passkey sign-in (WebAuthn, usernameless). body {}. returns {publicKey} for PublicKeyCredential.parseRequestOptionsFromJSON with no allowCredentials, so nothing reveals which accounts exist. the challenge is single-use and lasts 5 minutes. 404 when passkeys are off (PURSUIT_WEBAUTHN_RP_ID unset). requires Content-Type application/json and Sec-Fetch-Site same-origin or a matching Origin. begin and finish together are limited to 20 a minute per client IP (429)", false, open)
	add("/v1/auth/passkey/login/finish", "post", "finish a passkey sign-in. body is the browser's PublicKeyCredential.toJSON(). verifies challenge, origin, RP ID, signature, user verification and sign count, then mints a pst_ login token (name login, read,write,approve) into the httpOnly pursuit_session cookie and answers like GET /v1/me. never creates a user: an unknown, removed or replayed passkey is 401. same-origin JSON only", false, open)
	add("/v1/passkeys", "get", "List passkeys. Returns {id, name, created_at, last_used_at, synced}. requires a signed-in human with approve (not a demo sandbox or the PURSUIT_TRUST_PROXY_AUTH owner)", false, nil)
	add("/v1/passkeys/register/begin", "post", "start adding a passkey to the signed-in account. body {}. returns {publicKey} for PublicKeyCredential.parseCreationOptionsFromJSON (resident key and user verification required, existing passkeys excluded). requires approve, not demo or proxy owner, and same-origin JSON", false, nil)
	add("/v1/passkeys/register/finish", "post", "finish adding a passkey. body {name, credential} where credential is PublicKeyCredential.toJSON(). writes a passkey.create audit entry. returns the passkey (201). same-origin JSON only", false, nil)
	add("/v1/passkeys/{id}", "delete", "remove a passkey; it no longer signs in. sessions it started stay valid until revoked under /v1/tokens. writes a passkey.delete audit entry", true, nil)
	add("/v1/demo", "post", "Start a demo. Mint a token for a fresh, isolated demo sandbox when demo mode is on", false, open)
	add("/v1/me", "get", "Get the current user. Returns the user and the token making the call", false, nil)
	add("/v1/me", "patch", "Update the current user. Set display name or avatar (png, jpeg, or webp data URL under 256KB; empty clears it). email is accepted only from the PURSUIT_TRUST_PROXY_AUTH owner (GET /v1/me returns proxy: true)", true, nil)
	add("/v1/tokens", "get", "list tokens", false, nil)
	add("/v1/tokens", "post", "create a token; secret is returned once", true, nil)
	add("/v1/tokens/{id}", "delete", "revoke a token", true, nil)
	add("/v1/companies", "get", "list companies", false, nil)
	add("/v1/companies", "post", "create a company", true, nil)
	add("/v1/companies/{id}", "get", "get a company", false, nil)
	add("/v1/companies/{id}/favicon", "get", "Get a company favicon", false, nil)
	add("/v1/companies/{id}", "patch", "update a company; body.version is required", true, nil)
	add("/v1/companies/{id}", "delete", "delete a company; version query or If-Match", true, nil)
	add("/v1/roles", "get", "list roles", false, nil)
	add("/v1/roles", "post", "create a role", true, nil)
	add("/v1/roles/{id}", "get", "get a role", false, nil)
	add("/v1/roles/{id}", "patch", "update a role", true, nil)
	add("/v1/roles/{id}", "delete", "delete a role", true, nil)
	add("/v1/applications", "get", "List applications. Filters: status, company, fit, source, tag, q, stale, cursor, limit, sort, fields", false, nil)
	add("/v1/applications/by-url", "get", "Find an application by URL. Query url. 404 when the page is not tracked", false, nil)
	add("/v1/applications/capture", "post", "Capture a posting. Saves the posting the user is viewing; dedupes by URL and creates status to_apply", true, nil)
	add("/v1/applications", "post", "create an application", true, nil)
	add("/v1/applications/{id}", "get", "get an application", false, nil)
	add("/v1/applications/{id}", "patch", "update an application. status changes obey the approval rule", true, nil)
	add("/v1/applications/{id}", "delete", "delete an application", true, nil)
	add("/v1/applications/{id}/transition", "post", "Change status. A stage replaces the status and clears any outcome; waiting, needs_input, failed, rejected, withdrawn and ghosted are outcomes that keep the stage. approved requires the approve scope. submitting requires a stored approval", true, nil)
	add("/v1/applications/{id}/archive", "post", "archive an application; body.version is required", true, nil)
	add("/v1/applications/{id}/unarchive", "post", "unarchive an application; body.version is required", true, nil)
	add("/v1/applications/{id}/events", "get", "list events for an application", false, nil)
	add("/v1/applications/{id}/events", "post", "Add an event. Append email_received, interview_scheduled, or note", true, nil)
	add("/v1/applications/{id}/rounds", "get", "list rounds for an application. limit, cursor, fields; no limit returns all", false, nil)
	add("/v1/applications/{id}/rounds", "post", "add a round. stage is prepared, applied, screening, interviewing, final, or offer and defaults to the current stage. prep holds what to rehearse", true, nil)
	add("/v1/rounds/{id}", "get", "get a round", false, nil)
	add("/v1/rounds/{id}", "patch", "update a round; body.version is required. done true/false marks it done", true, nil)
	add("/v1/rounds/{id}", "delete", "delete a round; version query or If-Match", true, nil)
	deprecated := map[string]any{"deprecated": true}
	add("/v1/applications/{id}/interviews", "get", "List rounds. Deprecated alias of GET /v1/applications/{id}/rounds", false, deprecated)
	add("/v1/applications/{id}/interviews", "post", "Add a round. Deprecated alias of POST /v1/applications/{id}/rounds", true, deprecated)
	add("/v1/interviews/{id}", "get", "Get a round. Deprecated alias of GET /v1/rounds/{id}", false, deprecated)
	add("/v1/interviews/{id}", "patch", "Update a round. Deprecated alias of PATCH /v1/rounds/{id}", true, deprecated)
	add("/v1/interviews/{id}", "delete", "Delete a round. Deprecated alias of DELETE /v1/rounds/{id}", true, deprecated)
	add("/v1/applications/{id}/tasks", "get", "List tasks. An application's to-dos, the things waiting on you. limit, cursor, fields; no limit returns all", false, nil)
	add("/v1/applications/{id}/tasks", "post", "add a task. title is required. stage is to_apply, prepared, applied, screening, interviewing, final, or offer and defaults to the current stage. due_at is RFC3339 or YYYY-MM-DD", true, nil)
	add("/v1/tasks/{id}", "get", "get a task", false, nil)
	add("/v1/tasks/{id}", "patch", "update a task; body.version is required. done true/false marks it done", true, nil)
	add("/v1/tasks/{id}", "delete", "delete a task; version query or If-Match", true, nil)
	add("/v1/applications/{id}/tags", "post", "Attach a tag. By name or tag_id", true, nil)
	add("/v1/applications/{id}/tags/{tagID}", "delete", "detach a tag", true, nil)
	add("/v1/applications/{id}/fields/{fieldID}", "post", "Set a custom field value", true, nil)
	add("/v1/applications/{id}/prepare", "post", "Prepare an application. Pull questions, draft answers, and stop at awaiting_review or needs_input", true, nil)
	add("/v1/applications/{id}/enqueue", "post", "Enqueue work. Prepare or submit", true, nil)
	add("/v1/applications/{id}/artifacts", "get", "List artifacts. Metadata only. limit, cursor; no limit returns all", false, nil)
	add("/v1/applications/{id}/artifacts", "post", "Attach an artifact. Kind is resume_pdf, cover_letter, answers_json, form_snapshot, or confirmation", true, nil)
	add("/v1/applications/{id}/questions", "get", "List questions. Form questions and their answers. limit, cursor; no limit returns all", false, nil)
	add("/v1/applications/{id}/check-link", "post", "Check the posting", true, nil)
	add("/v1/approvals", "post", "Approve an application. Requires the approve scope, marks awaiting_review as approved, and enqueues submit. it does not submit a form", true, nil)
	add("/v1/approvals/reject", "post", "Reject an application. Declines prepared work and requires the approve scope. the application keeps its awaiting_review stage with outcome withdrawn and nothing is submitted", true, nil)
	add("/v1/links", "get", "List postings. Every checked posting. attention=true returns closed or changed. status is live, closed, or unknown", false, nil)
	add("/v1/apply", "post", "Apply in bulk. Batch prepare or async enqueue", true, nil)
	add("/v1/batch", "post", "Run a batch. Up to 100 API requests in order as the caller. body {requests:[{method,path,body,idempotency_key}],stop_on_error}. returns {results:[{status,body}],failed}. scopes apply per item, X-Dry-Run applies to every item, items are not atomic, and stop_on_error ends results at the first failure. Idempotency-Key goes on each item, not the batch", true, nil)
	add("/v1/queue/claim", "post", "Claim work. Leases the next work item. replicas use FOR UPDATE SKIP LOCKED on Postgres", true, nil)
	add("/v1/queue/{id}/heartbeat", "post", "Heartbeat. Extend a lease", true, nil)
	add("/v1/queue/{id}/complete", "post", "Complete work. Finish leased work and set the resulting status", true, nil)
	add("/v1/artifacts/{id}", "get", "Get an artifact. Returns the body", false, nil)
	add("/v1/profile", "get", "Get the applicant profile. Standing answers as a map, for the browser extension", false, nil)
	add("/v1/documents", "get", "List documents. Kinds are base_resume, cover_letter, other. limit, cursor; no limit returns all", false, nil)
	add("/v1/documents", "post", "Upload a document. Multipart (kind, file) or JSON text. prepare attaches the newest base_resume", true, nil)
	add("/v1/documents/{id}", "get", "Get a document. Includes the body; binary bodies are base64", false, nil)
	add("/v1/documents/{id}", "delete", "delete a document", true, nil)
	add("/v1/standing", "get", "list standing answers. limit, cursor; no limit returns all", false, nil)
	add("/v1/standing", "post", "set a standing answer. profile values also load from PURSUIT_PROFILE", true, nil)
	add("/v1/standing/{key}", "delete", "delete a standing answer. a PURSUIT_PROFILE value for the same key still applies", true, nil)
	add("/v1/caps", "get", "List company caps. Per-company application limits", false, nil)
	add("/v1/links/check", "post", "Check all postings. Sweeps every posting. one replica holds the advisory lock", true, nil)
	add("/v1/cron/links", "get", "Run the daily link check. For Vercel Cron. requires Authorization Bearer CRON_SECRET, not a Pursuit token", false, map[string]any{"x-internal": true})
	add("/v1/webhooks", "get", "list webhooks. limit, cursor; no limit returns all", false, nil)
	add("/v1/webhooks", "post", "register a webhook", true, nil)
	add("/v1/webhooks/{id}", "delete", "delete a webhook", true, nil)
	add("/v1/events", "get", "List events. after plus wait_ms long-polls", false, nil)
	add("/v1/tags", "get", "list tags", false, nil)
	add("/v1/tags", "post", "create a tag", true, nil)
	add("/v1/tags/{id}", "delete", "delete a tag", true, nil)
	add("/v1/fields", "get", "list custom fields. limit, cursor, sort, q, fields", false, nil)
	add("/v1/fields", "post", "create a custom field", true, nil)
	add("/v1/fields/{id}", "delete", "delete a custom field. dry_run query is also accepted", true, nil)
	add("/v1/audit", "get", "List audit entries. Includes actor_name and metadata. filter with q (summary, action, entity, actor, metadata), since and until (date or RFC 3339), actor, action, entity_type, entity_id. sort=created_at for oldest first", false, nil)
	add("/v1/import/csv", "post", "Import CSV. The tracker CSV. Prepared maps to awaiting_review. Applied stays applied. The same posting_url updates the existing application", true, nil)
	add("/v1/import/json", "post", "Import JSON. A JSON array of postings", true, nil)
	add("/v1/import/markdown", "post", "Import Markdown. Tables of roles become to_apply, deduped by URL", true, nil)
	add("/v1/import/seed", "post", "Import the seed. Loads the committed seed data", true, nil)
	add("/v1/export/simplify", "get", "Export Simplify CSV. Simplify Job Tracker format", false, nil)
	add("/v1/export/json", "get", "Export JSON", false, nil)

	doc := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "Pursuit",
			"version":     version.Version,
			"description": intro,
		},
		"servers":     []any{map[string]any{"url": "/"}},
		"security":    []any{map[string]any{"bearerAuth": []any{}}},
		"tags":        tags,
		"x-tagGroups": tagGroups,
		"paths":       paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{"type": "http", "scheme": "bearer"},
			},
			"parameters": map[string]any{
				"IdempotencyKey": map[string]any{"name": "Idempotency-Key", "in": "header", "schema": map[string]any{"type": "string"}},
				"DryRun":         map[string]any{"name": "X-Dry-Run", "in": "header", "schema": map[string]any{"type": "string", "enum": []string{"true"}}},
			},
			"schemas": map[string]any{
				"OAuthError": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"error":             map[string]any{"type": "string", "enum": []string{"invalid_request", "invalid_grant", "invalid_scope", "unsupported_grant_type", "authorization_pending", "slow_down", "expired_token", "access_denied", "server_error"}},
						"error_description": map[string]any{"type": "string"},
					},
				},
				"Error": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"error": map[string]any{"type": "object", "properties": map[string]any{
							"code": map[string]any{"type": "string"}, "message": map[string]any{"type": "string"},
							"hint": map[string]any{"type": "string"}, "details": map[string]any{"type": "object"},
						}},
					},
				},
			},
		},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return b
}

const intro = `The HTTP API behind the Pursuit dashboard and the ` + "`pursuit`" + ` CLI. Agents should prefer the CLI: ` + "`pursuit schema <command>`" + ` describes one command. The short agent guide is [/llms.txt](/llms.txt).

## Authentication

Send ` + "`Authorization: Bearer pst_...`" + `. A token holds the scopes ` + "`read`" + `, ` + "`write`" + `, and ` + "`approve`" + `. ` + "`write`" + ` does not include ` + "`approve`" + `, and only ` + "`approve`" + ` can move an application toward submission. Tokens, scopes, and OAuth login are in [/auth.md](/auth.md).

## Errors

Errors are JSON: ` + "`{\"error\": {\"code\", \"message\", \"hint\", \"details\"}}`" + `. 400 is validation, 401 a missing or bad token, 403 a missing scope, 404 not found, 409 a version or cap conflict.

## Writes

Every write accepts ` + "`Idempotency-Key`" + `, which replays a successful response, and ` + "`X-Dry-Run: true`" + `, which validates and writes nothing. Updates and deletes of versioned records take ` + "`version`" + ` in the body, the query, or ` + "`If-Match`" + `.

## Pagination

Lists take ` + "`limit`" + ` (1-200) and ` + "`cursor`" + `, and return ` + "`next_cursor`" + ` until the last page. ` + "`POST /v1/batch`" + ` runs up to 100 requests in one call.`

// tags is the sidebar order. tagOf maps each path to one of them.
var tags = []any{
	map[string]any{"name": "Applications", "description": "A posting you are tracking or applying to, and its status."},
	map[string]any{"name": "Companies"},
	map[string]any{"name": "Roles"},
	map[string]any{"name": "Rounds", "description": "Interview rounds, per stage. `/interviews` routes are deprecated aliases."},
	map[string]any{"name": "Tasks", "description": "To-dos, per stage."},
	map[string]any{"name": "Events", "description": "The activity feed. `GET /v1/events` long-polls with `after` and `wait_ms`."},
	map[string]any{"name": "Tags"},
	map[string]any{"name": "Custom fields"},
	map[string]any{"name": "Applying", "description": "Prepare drafts answers and stops at `awaiting_review`. A human approves; then a worker claims the submit work."},
	map[string]any{"name": "Queue", "description": "Workers lease work, heartbeat before the 2 minute lease expires, and complete it."},
	map[string]any{"name": "Artifacts", "description": "Files attached to one application: tailored résumé, cover letter, form snapshots."},
	map[string]any{"name": "Documents", "description": "Your files, such as the base résumé that prepare attaches."},
	map[string]any{"name": "Standing answers", "description": "Answers reused to fill every form."},
	map[string]any{"name": "Postings", "description": "Link checks for closed or changed postings."},
	map[string]any{"name": "Batch"},
	map[string]any{"name": "Import and export"},
	map[string]any{"name": "Webhooks"},
	map[string]any{"name": "Audit log", "description": "Who changed what and when."},
	map[string]any{"name": "Account", "description": "The current user and API tokens."},
	map[string]any{"name": "OAuth", "description": "Login for the CLI: `pursuit auth login`, with or without `--device`. OAuth never grants `approve`."},
	map[string]any{"name": "Dashboard sign-in", "description": "Browser-only routes the dashboard calls. They require same-origin requests."},
}

var tagGroups = []any{
	map[string]any{"name": "Tracking", "tags": []string{"Applications", "Companies", "Roles", "Rounds", "Tasks", "Events", "Tags", "Custom fields"}},
	map[string]any{"name": "Applying", "tags": []string{"Applying", "Queue", "Artifacts", "Documents", "Standing answers", "Postings"}},
	map[string]any{"name": "Data", "tags": []string{"Batch", "Import and export", "Webhooks", "Audit log"}},
	map[string]any{"name": "Auth", "tags": []string{"Account", "OAuth", "Dashboard sign-in"}},
}

func tagOf(path string) string {
	seg := strings.Split(strings.TrimPrefix(path, "/v1/"), "/")
	if !strings.HasPrefix(path, "/v1/") {
		return "" // meta routes, hidden from /reference
	}
	if len(seg) >= 3 && seg[0] == "applications" {
		switch seg[2] {
		case "rounds", "interviews", "tasks", "events", "tags", "artifacts":
			seg = seg[2:]
		case "fields":
			return "Custom fields"
		case "prepare", "enqueue", "questions":
			return "Applying"
		case "check-link":
			return "Postings"
		}
	}
	switch seg[0] {
	case "interviews", "rounds":
		return "Rounds"
	case "tasks", "events", "tags", "artifacts", "documents", "companies", "roles", "applications", "webhooks":
		return upper(seg[0])
	case "fields":
		return "Custom fields"
	case "approvals", "apply", "caps":
		return "Applying"
	case "queue":
		return "Queue"
	case "standing", "profile":
		return "Standing answers"
	case "links", "cron":
		return "Postings"
	case "batch":
		return "Batch"
	case "import", "export":
		return "Import and export"
	case "audit":
		return "Audit log"
	case "me", "tokens":
		return "Account"
	case "auth":
		switch seg[1] {
		case "device_authorization", "token", "revoke":
			return "OAuth"
		}
	}
	return "Dashboard sign-in"
}

// upper capitalizes s unless it opens on an identifier such as body.version.
func upper(s string) string {
	word, _, _ := strings.Cut(s, " ")
	if s == "" || strings.ContainsAny(word, "._/{") {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func errRef() map[string]any {
	return map[string]any{"description": "error", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Error"}}}}
}

func slug(path string) string {
	out := make([]byte, 0, len(path))
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '/' || c == '{' || c == '}' {
			out = append(out, '_')
			continue
		}
		out = append(out, c)
	}
	return string(out)
}
