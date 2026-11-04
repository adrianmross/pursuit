package spec

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/sqlstore"
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

// x is a JSON object under construction.
type x = map[string]any

var pathParam = regexp.MustCompile(`\{([A-Za-z]+)\}`)

func build() []byte {
	paths := x{}
	g := schemas{}
	ids := map[string]string{}
	schemaFor := func(v any, request bool) x {
		if m, ok := v.(x); ok {
			return m
		}
		return g.of(v, request)
	}
	// add registers one operation. text is "Title. Details": the title is the
	// sidebar entry, the rest the description. extra sets in (request body: a Go
	// value, a schema, or a full requestBody with content), out or list (the
	// response: a Go value or schema; list wraps it in the page envelope), raw
	// (a non-JSON response media type), q (query parameter names), status
	// (default 200), cli (the equivalent pursuit command), and id (operationId
	// override). Anything else is copied onto the operation.
	add := func(path, method, text string, write bool, extra x) {
		title, desc := text, ""
		for i := 0; i+1 < len(text); i++ {
			if (text[i] == '.' || text[i] == ';') && text[i+1] == ' ' {
				title, desc = text[:i], text[i+2:]
				break
			}
		}
		op := x{"summary": upper(title)}
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

		var params []any
		for _, m := range pathParam.FindAllStringSubmatch(path, -1) {
			params = append(params, x{"name": m[1], "in": "path", "required": true, "schema": x{"type": "string"}})
		}
		qs, _ := extra["q"].([]string)
		for _, name := range qs {
			if _, ok := queryParams[name]; !ok {
				panic("spec: unknown query parameter " + name)
			}
			params = append(params, x{"$ref": "#/components/parameters/" + name})
		}
		if write {
			params = append(params, x{"$ref": "#/components/parameters/IdempotencyKey"}, x{"$ref": "#/components/parameters/DryRun"})
		}
		if len(params) > 0 {
			op["parameters"] = params
		}

		if in := extra["in"]; in != nil {
			if m, ok := in.(x); ok && m["content"] != nil {
				op["requestBody"] = m
			} else {
				op["requestBody"] = x{"required": true, "content": x{"application/json": x{"schema": schemaFor(in, true)}}}
			}
		}

		status, _ := extra["status"].(int)
		if status == 0 {
			status = 200
		}
		ok := x{"description": http.StatusText(status)}
		switch {
		case extra["out"] != nil:
			ok["content"] = x{"application/json": x{"schema": schemaFor(extra["out"], false)}}
		case extra["list"] != nil:
			ok["content"] = x{"application/json": x{"schema": x{
				"type":     "object",
				"required": []string{"data", "page"},
				"properties": x{
					"data": x{"type": "array", "items": schemaFor(extra["list"], false)},
					"page": g.of(domain.PageMeta{}, false),
				},
			}}}
		case extra["raw"] != nil:
			ok["content"] = x{extra["raw"].(string): x{"schema": x{"type": "string"}}}
		}
		op["responses"] = x{strconv.Itoa(status): ok, "default": errRef()}

		if cli, _ := extra["cli"].(string); cli != "" {
			op["x-codeSamples"] = []any{x{"lang": "shell", "label": "pursuit CLI", "source": cli}}
		}
		id, _ := extra["id"].(string)
		if id == "" {
			id = operationID(title)
		}
		if prev, dup := ids[id]; dup {
			panic("spec: operationId " + id + " is used by " + prev + " and " + method + " " + path)
		}
		ids[id] = method + " " + path
		op["operationId"] = id

		for k, v := range extra {
			switch k {
			case "in", "out", "list", "raw", "q", "status", "cli", "id":
			default:
				op[k] = v
			}
		}
		item, _ := paths[path].(x)
		if item == nil {
			item = x{}
		}
		item[method] = op
		paths[path] = item
	}
	// with copies base and sets more keys on it, so shared option sets stay unaliased.
	with := func(base x, kv ...any) x {
		out := x{}
		for k, v := range base {
			out[k] = v
		}
		for i := 0; i+1 < len(kv); i += 2 {
			out[kv[i].(string)] = kv[i+1]
		}
		return out
	}
	q := func(names ...string) []string { return names }
	str := x{"type": "string"}
	form := func(props x, required ...string) x {
		schema := x{"type": "object", "properties": props}
		if len(required) > 0 {
			schema["required"] = required
		}
		return x{"required": true, "content": x{"application/x-www-form-urlencoded": x{"schema": schema}}}
	}
	oauthResp := func(v any) x {
		return x{
			"200": x{"description": "OK", "content": x{"application/json": x{"schema": g.of(v, false)}}},
			"400": x{"description": "RFC 6749 error", "content": x{"application/json": x{"schema": x{"$ref": "#/components/schemas/OAuthError"}}}},
		}
	}
	upload := x{"required": true, "content": x{
		"multipart/form-data": x{"schema": x{"type": "object", "properties": x{
			"kind": str,
			"file": x{"type": "string", "contentMediaType": "application/octet-stream"},
		}}},
		"application/json": x{"schema": g.of(UploadInput{}, true)},
	}}
	text := func(media string) x {
		return x{"required": true, "content": x{media: x{"schema": str}}}
	}

	// Meta routes stay in the document for agents; x-internal hides them in /reference.
	hidden := x{"security": []any{}, "x-internal": true}
	add("/healthz", "get", "Liveness", false, with(hidden, "out", Status{}))
	add("/readyz", "get", "Readiness", false, with(hidden, "out", Status{}))
	add("/version", "get", "Version", false, with(hidden, "out", VersionInfo{}))
	add("/openapi.json", "get", "This document", false, with(hidden, "raw", "application/json", "id", "getOpenAPI"))
	add("/llms.txt", "get", "Short agent guide", false, with(hidden, "raw", "text/plain"))
	add("/auth.md", "get", "Agent auth guide: tokens, scopes, approval", false, with(hidden, "raw", "text/markdown", "id", "getAuthGuide"))
	add("/skill.md", "get", "Agent Skill for the pursuit CLI", false, with(hidden, "raw", "text/markdown", "id", "getSkill"))
	add("/reference", "get", "Interactive API reference page (HTML)", false, with(hidden, "raw", "text/html", "id", "getReference"))

	open := x{"security": []any{}}
	add("/v1/auth/device_authorization", "post", "Start a device login. RFC 8628 device authorization for `pursuit auth login --device`. scope is space or comma separated, each exactly read or write, default read write; anything else (approve, APPROVE, Read) is invalid_scope: OAuth never grants approve. client_id is at most 64 characters. limited to 10 starts a minute per client IP (429 slow_down). device codes last 10 minutes; poll /v1/auth/token no faster than interval. there is no verification_uri_complete: the human types the code", false, x{
		"security":    []any{},
		"requestBody": form(x{"client_id": str, "scope": str}),
		"responses":   oauthResp(service.DeviceAuthorization{}),
		"cli":         "pursuit auth login --device",
	})
	add("/v1/auth/token", "post", "Get an access token. OAuth2 token endpoint. grant_type=authorization_code takes code, code_verifier (PKCE S256), redirect_uri, client_id; codes are single-use and last 60 seconds. grant_type=urn:ietf:params:oauth:grant-type:device_code takes device_code, client_id and answers authorization_pending, slow_down, expired_token, or access_denied until approved. The access_token is a normal pst_ API token", false, x{
		"security": []any{},
		"requestBody": form(x{
			"grant_type": x{"type": "string", "enum": []string{"authorization_code", "urn:ietf:params:oauth:grant-type:device_code"}},
			"code":       str, "code_verifier": str, "redirect_uri": str, "device_code": str, "client_id": str,
		}, "grant_type"),
		"responses": oauthResp(service.OAuthToken{}),
		"cli":       "pursuit auth login",
	})
	add("/v1/auth/revoke", "post", "Revoke an OAuth token. RFC 7009 token revocation. holding the token is the authority; unknown tokens still return 200", false, x{
		"security":    []any{},
		"requestBody": form(x{"token": str}, "token"),
		"responses": oauthResp(struct {
			Revoked bool `json:"revoked"`
		}{}),
		"cli": "pursuit auth logout",
	})
	add("/v1/oauth/authorize", "get", "Check a browser login. Validate a loopback request for the dashboard /authorize page without deciding it. requires the approve scope. returns the request as the server normalized it; a scope other than read or write is 400 invalid_scope. response_type is accepted and ignored", false, x{
		"q": q("client_id", "redirect_uri", "code_challenge", "code_challenge_method", "state", "scope", "response_type"), "out": service.AuthorizeRequest{},
	})
	add("/v1/oauth/authorize", "post", "Approve a browser login. Loopback consent, called by the dashboard /authorize page that `pursuit auth login` opens. requires a signed-in human session with the approve scope (not the PURSUIT_TRUST_PROXY_AUTH owner), Content-Type application/json, and Sec-Fetch-Site same-origin or an Origin matching the host. writes an oauth.approve or oauth.deny audit entry. redirect_uri must be http://127.0.0.1:<port>/callback or http://[::1]:<port>/callback. redirect_to is the callback with code and state, or error=access_denied when approve is false", false, x{
		"in": service.AuthorizeInput{}, "out": Redirect{},
	})
	add("/v1/oauth/device", "get", "Get a device login. Look up a pending device login by user_code so the dashboard can show the requested scopes. requires the approve scope. requested_from is the IP that started the login", false, x{
		"q": q("user_code"), "out": service.DeviceRequest{},
	})
	add("/v1/oauth/device/approve", "post", "Approve a device login. Approve or deny a device login from the dashboard /device page. requires a signed-in human session with the approve scope (not the PURSUIT_TRUST_PROXY_AUTH owner), Content-Type application/json, and Sec-Fetch-Site same-origin or an Origin matching the host; the code alone approves nothing. writes an oauth.approve or oauth.deny audit entry. status is approved or denied", false, x{
		"in": DeviceDecision{}, "out": Status{},
	})
	add("/v1/auth/providers", "get", "List sign-in options. passkeys is true when PURSUIT_WEBAUTHN_RP_ID is set", false, with(open, "out", Providers{}))
	add("/v1/auth/{provider}/start", "get", "Start an OIDC sign-in. Redirects to the provider", false, with(open, "status", 302))
	add("/v1/auth/{provider}/callback", "get", "Finish an OIDC sign-in. Redirects to /#token=<secret>", false, with(open, "status", 302))
	add("/v1/auth/session", "post", "Sign in. Verify a token (pst_, or a Better Auth JWT exchanged for a pst_ login token) and store the pst_ token in the httpOnly pursuit_session cookie, then answer like GET /v1/me. requires Content-Type application/json and Sec-Fetch-Site same-origin or a matching Origin. agents send Authorization: Bearer instead", false, with(open, "in", SessionInput{}, "out", domain.Me{}))
	add("/v1/auth/session", "delete", "Sign out. Clear the pursuit_session cookie and revoke it when sign-in minted it (token name login). any other token stays valid; revoke it with POST /v1/auth/revoke. same-origin only", false, with(open, "status", 204))
	add("/v1/auth/passkey/login/begin", "post", "Start a passkey sign-in. WebAuthn, usernameless. body {}. publicKey is for PublicKeyCredential.parseRequestOptionsFromJSON with no allowCredentials, so nothing reveals which accounts exist. the challenge is single-use and lasts 5 minutes. 404 when passkeys are off (PURSUIT_WEBAUTHN_RP_ID unset). requires Content-Type application/json and Sec-Fetch-Site same-origin or a matching Origin. begin and finish together are limited to 20 a minute per client IP (429)", false, with(open, "out", WebAuthnOptions{}))
	add("/v1/auth/passkey/login/finish", "post", "Finish a passkey sign-in. body is the browser's PublicKeyCredential.toJSON(). verifies challenge, origin, RP ID, signature, user verification and sign count, then mints a pst_ login token (name login, read,write,approve) into the httpOnly pursuit_session cookie and answers like GET /v1/me. never creates a user: an unknown, removed or replayed passkey is 401. same-origin JSON only", false, with(open, "in", x{}, "out", domain.Me{}))
	add("/v1/passkeys", "get", "List passkeys. requires a signed-in human with approve (not a demo sandbox or the PURSUIT_TRUST_PROXY_AUTH owner)", false, x{"q": q("limit", "cursor"), "list": service.PasskeyView{}})
	add("/v1/passkeys/register/begin", "post", "Start adding a passkey. body {}. publicKey is for PublicKeyCredential.parseCreationOptionsFromJSON (resident key and user verification required, existing passkeys excluded). requires approve, not demo or proxy owner, and same-origin JSON", false, x{"out": WebAuthnOptions{}})
	add("/v1/passkeys/register/finish", "post", "Finish adding a passkey. credential is PublicKeyCredential.toJSON(). writes a passkey.create audit entry. same-origin JSON only", false, x{"in": PasskeyInput{}, "out": service.PasskeyView{}, "status": 201})
	add("/v1/passkeys/{id}", "delete", "Remove a passkey. It no longer signs in. sessions it started stay valid until revoked under /v1/tokens. writes a passkey.delete audit entry", true, x{"out": domain.Deleted{}})
	add("/v1/demo", "post", "Start a demo. Mint a token for a fresh, isolated demo sandbox when demo mode is on", false, with(open, "out", DemoSession{}, "status", 201))

	add("/v1/me", "get", "Get the current user. Returns the user and the token making the call", false, x{"q": q("fields"), "out": domain.Me{}, "cli": "pursuit auth status"})
	add("/v1/me", "patch", "Update the current user. Set display name or avatar (png, jpeg, or webp data URL under 256KB; empty clears it). email is accepted only from the PURSUIT_TRUST_PROXY_AUTH owner (GET /v1/me returns proxy: true)", true, x{"in": service.MePatch{}, "out": domain.User{}})
	add("/v1/tokens", "get", "List tokens", false, x{"q": q("limit", "cursor", "sort", "fields"), "list": domain.Token{}, "cli": "pursuit tokens list"})
	add("/v1/tokens", "post", "Create a token. The secret is returned once. a token can only grant scopes it holds", true, x{"in": service.TokenInput{}, "out": domain.IssuedToken{}, "status": 201, "cli": "pursuit tokens create --name agent-1 --scope read --scope write"})
	add("/v1/tokens/{id}", "delete", "Revoke a token", true, x{"out": domain.Token{}, "cli": "pursuit tokens revoke TOKEN_ID"})

	add("/v1/companies", "get", "List companies", false, x{"q": q("limit", "cursor", "sort", "q", "fields"), "list": domain.Company{}, "cli": "pursuit companies list"})
	add("/v1/companies", "post", "Create a company", true, x{"in": service.CompanyInput{}, "out": domain.Company{}, "status": 201, "cli": "pursuit companies create --name Example"})
	add("/v1/companies/{id}", "get", "Get a company", false, x{"q": q("fields"), "out": domain.Company{}, "cli": "pursuit companies get COMPANY_ID"})
	add("/v1/companies/{id}/favicon", "get", "Get a company favicon", false, x{"q": q("service"), "raw": "image/*"})
	add("/v1/companies/{id}", "patch", "Update a company. body.version is required", true, x{"in": service.CompanyPatch{}, "out": domain.Company{}, "cli": "pursuit companies update COMPANY_ID --website https://example.com"})
	add("/v1/companies/{id}", "delete", "Delete a company", true, x{"q": q("version"), "out": domain.Deleted{}, "cli": "pursuit companies delete COMPANY_ID"})

	add("/v1/roles", "get", "List roles", false, x{"q": q("limit", "cursor", "sort", "q", "company", "company_id", "fit", "source", "fields"), "list": domain.Role{}, "cli": "pursuit roles list --company Example"})
	add("/v1/roles", "post", "Create a role", true, x{"in": service.RoleInput{}, "out": domain.Role{}, "status": 201, "cli": "pursuit roles create --company Example --title Engineer"})
	add("/v1/roles/{id}", "get", "Get a role", false, x{"q": q("fields"), "out": domain.Role{}, "cli": "pursuit roles get ROLE_ID"})
	add("/v1/roles/{id}", "patch", "Update a role. body.version is required", true, x{"in": service.RolePatch{}, "out": domain.Role{}, "cli": "pursuit roles update ROLE_ID --salary '$150k'"})
	add("/v1/roles/{id}", "delete", "Delete a role", true, x{"q": q("version"), "out": domain.Deleted{}, "cli": "pursuit roles delete ROLE_ID"})

	add("/v1/applications", "get", "List applications", false, x{"q": q("limit", "cursor", "sort", "q", "status", "status_not", "company", "company_id", "fit", "source", "tag", "stale", "archived", "fields"), "list": domain.Application{}, "cli": "pursuit apps list --status to_apply --fields id,company,title,status"})
	add("/v1/applications/by-url", "get", "Find an application by URL. 404 when the page is not tracked", false, x{"q": q("url"), "out": domain.Application{}})
	add("/v1/applications/capture", "post", "Capture a posting. Saves the posting the user is viewing; dedupes by URL and creates status to_apply. created is false when the URL was already tracked", true, x{"in": service.ApplicationInput{}, "out": service.CaptureResult{}})
	add("/v1/applications", "post", "Create an application", true, x{"in": service.ApplicationInput{}, "out": domain.Application{}, "status": 201, "cli": "pursuit apps create --company Example --title Engineer --status to_apply"})
	add("/v1/applications/{id}", "get", "Get an application", false, x{"q": q("fields"), "out": domain.Application{}, "cli": "pursuit apps get APP_ID"})
	add("/v1/applications/{id}", "patch", "Update an application. body.version is required. status changes obey the approval rule", true, x{"in": service.ApplicationPatch{}, "out": domain.Application{}, "cli": "pursuit apps update APP_ID --next-step 'Schedule call'"})
	add("/v1/applications/{id}", "delete", "Delete an application", true, x{"q": q("version"), "out": domain.Deleted{}, "cli": "pursuit apps delete APP_ID"})
	add("/v1/applications/{id}/transition", "post", "Change status. A stage replaces the status and clears any outcome; waiting, needs_input, failed, rejected, withdrawn and ghosted are outcomes that keep the stage. approved requires the approve scope. submitting requires a stored approval", true, x{"in": service.TransitionInput{}, "out": domain.Application{}, "cli": "pursuit apps transition APP_ID --status interviewing"})
	add("/v1/applications/{id}/archive", "post", "Archive an application. body.version is required", true, x{"in": VersionInput{}, "out": domain.Application{}, "cli": "pursuit apps archive APP_ID"})
	add("/v1/applications/{id}/unarchive", "post", "Unarchive an application. body.version is required", true, x{"in": VersionInput{}, "out": domain.Application{}, "cli": "pursuit apps unarchive APP_ID"})
	add("/v1/applications/{id}/events", "get", "List an application's events", false, x{"q": q("limit", "cursor", "sort", "fields"), "list": domain.Event{}, "cli": "pursuit events --application APP_ID"})
	add("/v1/applications/{id}/events", "post", "Add an event. kind is email_received, interview_scheduled, or note", true, x{"in": service.EventInput{}, "out": domain.Event{}, "status": 201, "cli": "pursuit events add APP_ID --kind note --body 'Recruiter replied'"})

	add("/v1/applications/{id}/rounds", "get", "List rounds. Without limit it returns all", false, x{"q": q("limit", "cursor", "fields"), "list": domain.Interview{}, "cli": "pursuit rounds list APP_ID"})
	add("/v1/applications/{id}/rounds", "post", "Add a round. stage is prepared, applied, screening, interviewing, final, or offer and defaults to the current stage. prep holds what to rehearse", true, x{"in": service.InterviewInput{}, "out": domain.Interview{}, "status": 201, "cli": "pursuit rounds add APP_ID --kind 'System design' --when 2026-11-05"})
	add("/v1/rounds/{id}", "get", "Get a round", false, x{"q": q("fields"), "out": domain.Interview{}})
	add("/v1/rounds/{id}", "patch", "Update a round. body.version is required. done true or false marks it done", true, x{"in": service.InterviewPatch{}, "out": domain.Interview{}, "cli": "pursuit rounds done ROUND_ID --notes 'Went well'"})
	add("/v1/rounds/{id}", "delete", "Delete a round", true, x{"q": q("version"), "out": domain.Deleted{}, "cli": "pursuit rounds delete ROUND_ID"})
	// The /interviews aliases stay callable and in the document, but out of /reference.
	deprecated := x{"deprecated": true, "x-internal": true}
	add("/v1/applications/{id}/interviews", "get", "List rounds. Deprecated alias of GET /v1/applications/{id}/rounds", false, with(deprecated, "q", q("limit", "cursor", "fields"), "list", domain.Interview{}, "id", "listInterviews"))
	add("/v1/applications/{id}/interviews", "post", "Add a round. Deprecated alias of POST /v1/applications/{id}/rounds", true, with(deprecated, "in", service.InterviewInput{}, "out", domain.Interview{}, "status", 201, "id", "addInterview"))
	add("/v1/interviews/{id}", "get", "Get a round. Deprecated alias of GET /v1/rounds/{id}", false, with(deprecated, "q", q("fields"), "out", domain.Interview{}, "id", "getInterview"))
	add("/v1/interviews/{id}", "patch", "Update a round. Deprecated alias of PATCH /v1/rounds/{id}", true, with(deprecated, "in", service.InterviewPatch{}, "out", domain.Interview{}, "id", "updateInterview"))
	add("/v1/interviews/{id}", "delete", "Delete a round. Deprecated alias of DELETE /v1/rounds/{id}", true, with(deprecated, "q", q("version"), "out", domain.Deleted{}, "id", "deleteInterview"))

	add("/v1/applications/{id}/tasks", "get", "List tasks. An application's to-dos, the things waiting on you. without limit it returns all", false, x{"q": q("limit", "cursor", "fields"), "list": domain.Task{}, "cli": "pursuit tasks list APP_ID"})
	add("/v1/applications/{id}/tasks", "post", "Add a task. title is required. stage is to_apply, prepared, applied, screening, interviewing, final, or offer and defaults to the current stage. due_at is RFC 3339 or YYYY-MM-DD", true, x{"in": service.TaskInput{}, "out": domain.Task{}, "status": 201, "cli": "pursuit tasks add APP_ID --title 'Send references' --due 2026-11-05"})
	add("/v1/tasks/{id}", "get", "Get a task", false, x{"q": q("fields"), "out": domain.Task{}})
	add("/v1/tasks/{id}", "patch", "Update a task. body.version is required. done true or false marks it done", true, x{"in": service.TaskPatch{}, "out": domain.Task{}, "cli": "pursuit tasks done TASK_ID"})
	add("/v1/tasks/{id}", "delete", "Delete a task", true, x{"q": q("version"), "out": domain.Deleted{}, "cli": "pursuit tasks delete TASK_ID"})

	add("/v1/applications/{id}/tags", "post", "Attach a tag. By name or tag_id", true, x{"in": service.AttachTagInput{}, "out": domain.Application{}, "cli": "pursuit apps tag add APP_ID --name remote"})
	add("/v1/applications/{id}/tags/{tagID}", "delete", "Detach a tag", true, x{"out": domain.Application{}, "cli": "pursuit apps tag remove APP_ID TAG_ID"})
	add("/v1/applications/{id}/fields/{fieldID}", "post", "Set a custom field value", true, x{"in": service.SetFieldValueInput{}, "out": domain.Application{}, "cli": "pursuit apps set-field APP_ID FIELD_ID --value 'Remote OK'"})
	add("/v1/applications/{id}/prepare", "post", "Prepare an application. Pull questions, draft answers, and stop at awaiting_review or needs_input", true, x{"q": q("worker"), "out": service.PrepareResult{}, "cli": "pursuit apply APP_ID --prepare"})
	add("/v1/applications/{id}/enqueue", "post", "Enqueue work. kind is prepare or submit", true, x{"in": EnqueueInput{}, "out": service.WorkView{}, "cli": "pursuit apply APP_ID --async"})
	add("/v1/applications/{id}/artifacts", "get", "List artifacts. Metadata only. without limit it returns all", false, x{"q": q("limit", "cursor"), "list": service.ArtifactView{}, "cli": "pursuit artifacts list APP_ID"})
	add("/v1/applications/{id}/artifacts", "post", "Attach an artifact. kind is resume_pdf, cover_letter, answers_json, form_snapshot, or confirmation. send multipart (kind, file) or JSON with a text body", true, x{"in": upload, "out": service.ArtifactView{}, "status": 201, "cli": "pursuit artifacts add APP_ID --kind resume_pdf --file tailored.pdf"})
	add("/v1/applications/{id}/questions", "get", "List questions. Form questions and their answers. without limit it returns all", false, x{"q": q("limit", "cursor"), "list": service.QuestionView{}})
	add("/v1/applications/{id}/check-link", "post", "Check the posting", true, x{"out": service.LinkCheckView{}, "cli": "pursuit check-links APP_ID"})
	add("/v1/approvals", "post", "Approve applications. Requires the approve scope, marks awaiting_review as approved, and enqueues submit. it does not submit a form", true, x{"in": DecisionInput{}, "out": []domain.Application{}, "cli": "pursuit approve APP_ID"})
	add("/v1/approvals/reject", "post", "Reject applications. Declines prepared work and requires the approve scope. the application keeps its awaiting_review stage with outcome withdrawn and nothing is submitted", true, x{"in": DecisionInput{}, "out": []domain.Application{}, "cli": "pursuit reject APP_ID"})
	add("/v1/links", "get", "List postings. Every checked posting. attention=true returns closed or changed ones and cannot be combined with status or changed", false, x{"q": q("limit", "cursor", "sort", "link_status", "changed", "attention", "fields"), "list": domain.LinkHealth{}, "cli": "pursuit links --fields company,title,status,detail"})
	add("/v1/apply", "post", "Apply in bulk. Prepare every matching application now, or enqueue them with async", true, x{"in": ApplyInput{}, "out": service.BatchResult{}, "cli": "pursuit apply --batch status=to_apply --async"})
	add("/v1/batch", "post", "Run a batch. Up to 100 API requests in order as the caller. scopes apply per item, X-Dry-Run applies to every item, items are not atomic, and stop_on_error ends results at the first failure. Idempotency-Key goes on each item, not the batch", true, x{"in": BatchInput{}, "out": BatchOutput{}, "cli": "pursuit batch requests.json"})
	add("/v1/queue/claim", "post", "Claim work. Leases the next work item. work is null when the queue is empty. replicas use FOR UPDATE SKIP LOCKED on Postgres", true, x{"in": WorkerInput{}, "out": service.ClaimResult{}, "cli": "pursuit queue claim --worker agent-1"})
	add("/v1/queue/{id}/heartbeat", "post", "Heartbeat. Extend a lease before its 2 minutes run out", true, x{"in": WorkerInput{}, "out": service.WorkView{}, "cli": "pursuit queue heartbeat WORK_ID --worker agent-1"})
	add("/v1/queue/{id}/complete", "post", "Complete work. Finish leased work and set the resulting status: awaiting_review, needs_input, submitted, or failed", true, x{"in": CompleteInput{}, "out": domain.Application{}, "cli": "pursuit queue complete WORK_ID --worker agent-1 --status awaiting_review"})
	add("/v1/artifacts/{id}", "get", "Get an artifact. Includes the body", false, x{"out": ArtifactFile{}, "cli": "pursuit artifacts get ARTIFACT_ID"})
	add("/v1/profile", "get", "Get the applicant profile. Standing answers as a map, for the browser extension", false, x{"out": service.ProfileView{}})
	add("/v1/documents", "get", "List documents. kind is base_resume, cover_letter, or other. without limit it returns all", false, x{"q": q("limit", "cursor"), "list": service.DocumentView{}, "cli": "pursuit documents list"})
	add("/v1/documents", "post", "Upload a document. Multipart (kind, file) or JSON text. prepare attaches the newest base_resume", true, x{"in": upload, "out": service.DocumentView{}, "status": 201, "cli": "pursuit documents add --kind base_resume --file resume.pdf"})
	add("/v1/documents/{id}", "get", "Get a document. Includes the body; binary bodies are base64", false, x{"out": DocumentFile{}, "cli": "pursuit documents get DOCUMENT_ID"})
	add("/v1/documents/{id}", "delete", "Delete a document", true, x{"out": domain.Deleted{}, "cli": "pursuit documents delete DOCUMENT_ID"})
	add("/v1/standing", "get", "List standing answers. Without limit it returns all", false, x{"q": q("limit", "cursor"), "list": sqlstore.StandingAnswer{}, "cli": "pursuit standing list"})
	add("/v1/standing", "post", "Set a standing answer. Profile values also load from PURSUIT_PROFILE", true, x{"in": StandingInput{}, "out": sqlstore.StandingAnswer{}, "cli": "pursuit standing set --key work_authorization --value 'US citizen'"})
	add("/v1/standing/{key}", "delete", "Delete a standing answer. A PURSUIT_PROFILE value for the same key still applies", true, x{"out": domain.Deleted{}, "cli": "pursuit standing delete work_authorization"})
	add("/v1/caps", "get", "List company caps. Per-company application limits", false, x{"out": Caps{}, "cli": "pursuit caps"})
	add("/v1/links/check", "post", "Check all postings. Sweeps every posting. one replica holds the advisory lock", true, x{"in": SweepInput{}, "out": service.CheckReport{}, "cli": "pursuit check-links --all"})
	add("/v1/cron/links", "get", "Run the daily link check. For Vercel Cron. requires Authorization Bearer CRON_SECRET, not a Pursuit token", false, x{"x-internal": true, "out": service.CheckReport{}})
	add("/v1/webhooks", "get", "List webhooks. Without limit it returns all", false, x{"q": q("limit", "cursor"), "list": service.WebhookView{}, "cli": "pursuit webhooks list"})
	add("/v1/webhooks", "post", "Register a webhook. Every new event is posted to url; see Webhooks below for the payload", true, x{"in": WebhookInput{}, "out": service.WebhookView{}, "status": 201, "cli": "pursuit webhooks add https://example.com/hook"})
	add("/v1/webhooks/{id}", "delete", "Delete a webhook", true, x{"out": domain.Deleted{}, "cli": "pursuit webhooks delete WEBHOOK_ID"})
	add("/v1/events", "get", "List events. after plus wait_ms long-polls for the next event", false, x{"q": q("limit", "cursor", "sort", "application_id", "after", "wait_ms", "fields"), "list": domain.Event{}, "cli": "pursuit events --follow --timeout 25s"})
	add("/v1/tags", "get", "List tags", false, x{"q": q("limit", "cursor", "sort", "q", "fields"), "list": domain.Tag{}, "cli": "pursuit tags list"})
	add("/v1/tags", "post", "Create a tag", true, x{"in": service.TagInput{}, "out": domain.Tag{}, "status": 201, "cli": "pursuit tags create --name remote"})
	add("/v1/tags/{id}", "delete", "Delete a tag", true, x{"out": domain.Deleted{}, "cli": "pursuit tags delete TAG_ID"})
	add("/v1/fields", "get", "List custom fields", false, x{"q": q("limit", "cursor", "sort", "q", "fields"), "list": domain.CustomField{}, "cli": "pursuit fields list"})
	add("/v1/fields", "post", "Create a custom field. type is text, number, or date", true, x{"in": service.FieldInput{}, "out": domain.CustomField{}, "status": 201, "cli": "pursuit fields create --name 'Salary range' --type text"})
	add("/v1/fields/{id}", "delete", "Delete a custom field", true, x{"out": domain.Deleted{}, "cli": "pursuit fields delete FIELD_ID"})
	add("/v1/audit", "get", "List audit entries. Includes actor_name and metadata. q searches summary, action, entity, actor, and metadata. sort=created_at for oldest first", false, x{"q": q("limit", "cursor", "sort", "q", "since", "until", "actor", "action", "entity_type", "entity_id", "fields"), "list": domain.Audit{}, "cli": "pursuit audit list --entity-id APP_ID --sort created_at"})
	add("/v1/import/csv", "post", "Import CSV. The tracker CSV, as text/csv or a multipart file field. Prepared maps to awaiting_review. Applied stays applied. the same posting_url updates the existing application", true, x{"in": text("text/csv"), "out": service.ImportResult{}, "cli": "pursuit import csv applications.csv"})
	add("/v1/import/json", "post", "Import JSON. A JSON array of postings", true, x{"in": []ImportPosting{}, "out": service.ImportResult{}, "cli": "pursuit import json postings.json"})
	add("/v1/import/markdown", "post", "Import Markdown. Tables of roles become to_apply, deduped by URL", true, x{"in": text("text/markdown"), "out": service.ImportResult{}, "cli": "pursuit import markdown roles.md"})
	add("/v1/import/seed", "post", "Import the seed. Loads the committed seed data", true, x{"out": service.ImportResult{}, "cli": "pursuit import seed"})
	add("/v1/export/simplify", "get", "Export Simplify CSV. Simplify Job Tracker format", false, x{"raw": "text/csv", "cli": "pursuit export simplify -o simplify.csv"})
	add("/v1/export/json", "get", "Export JSON", false, x{"q": q("fields"), "out": service.ExportJSON{}, "cli": "pursuit export json -o pursuit.json"})

	params := x{
		"IdempotencyKey": x{"name": "Idempotency-Key", "in": "header", "description": "Replays the first successful response for the same key.", "schema": str},
		"DryRun":         x{"name": "X-Dry-Run", "in": "header", "description": "Validate and write nothing. The `dry_run=true` query does the same.", "schema": x{"type": "string", "enum": []string{"true"}}},
	}
	for name, p := range queryParams {
		params[name] = p
	}
	components := x{
		"securitySchemes": x{"bearerAuth": x{"type": "http", "scheme": "bearer"}},
		"parameters":      params,
		"schemas":         g,
	}
	g["OAuthError"] = x{
		"type": "object",
		"properties": x{
			"error":             x{"type": "string", "enum": []string{"invalid_request", "invalid_grant", "invalid_scope", "unsupported_grant_type", "authorization_pending", "slow_down", "expired_token", "access_denied", "server_error"}},
			"error_description": str,
		},
	}
	g["Error"] = x{
		"type":     "object",
		"required": []string{"error"},
		"properties": x{
			"error": x{"type": "object", "required": []string{"code", "message"}, "properties": x{
				"code": str, "message": str, "hint": str, "details": x{"type": "object"},
			}},
		},
	}
	doc := x{
		"openapi": "3.1.0",
		"info": x{
			"title":       "Pursuit",
			"version":     version.Version,
			"description": intro,
			"license":     x{"name": "MIT", "identifier": "MIT"},
		},
		"servers":     []any{x{"url": "/"}},
		"security":    []any{x{"bearerAuth": []any{}}},
		"tags":        tags,
		"x-tagGroups": tagGroups,
		"paths":       paths,
		"webhooks": x{
			"event": x{"post": x{
				"summary":     "Event",
				"operationId": "eventWebhook",
				"description": "Pursuit posts each new event to every registered webhook URL as JSON, with `User-Agent: pursuit-webhook`. Delivery is best effort: no retries and no ordering guarantee.",
				"parameters": []any{x{
					"name": "X-Pursuit-Secret", "in": "header",
					"description": "The secret the webhook was registered with, when it has one. Compare it before trusting the body.",
					"schema":      str,
				}},
				"requestBody": x{"required": true, "content": x{"application/json": x{"schema": g.of(domain.Event{}, false)}}},
				"responses":   x{"200": x{"description": "Any 2xx acknowledges the event."}},
			}},
		},
		"components": components,
	}
	b, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return b
}

func param(name, desc string, schema x) x {
	return x{"name": name, "in": "query", "description": desc, "schema": schema}
}

// queryParams are every query parameter a route accepts, by component name.
// The handlers' query(r, ...) allowlists in internal/httpapi are the source.
var queryParams = map[string]any{
	"limit":                 param("limit", "Page size, 1 to 200. Lists that return everything by default page only when it is set.", x{"type": "integer", "minimum": 1, "maximum": 200}),
	"cursor":                param("cursor", "`page.next_cursor` from the previous page, unchanged.", x{"type": "string"}),
	"sort":                  param("sort", "A field name, with an optional leading `-` for descending.", x{"type": "string"}),
	"q":                     param("q", "Text search.", x{"type": "string"}),
	"fields":                param("fields", "Comma-separated fields to return, such as `id,company,title`.", x{"type": "string"}),
	"version":               param("version", "The record's current version. `If-Match` works too.", x{"type": "integer"}),
	"status":                param("status", "Comma-separated statuses or outcomes to include. Repeatable.", x{"type": "string"}),
	"status_not":            param("status_not", "Comma-separated statuses or outcomes to exclude. Repeatable.", x{"type": "string"}),
	"company":               param("company", "Company name.", x{"type": "string"}),
	"company_id":            param("company_id", "Company ID.", x{"type": "string"}),
	"fit":                   param("fit", "strong, reach, or maybe.", x{"type": "string"}),
	"source":                param("source", "greenhouse, ashby, lever, workday, company_site, or other.", x{"type": "string"}),
	"tag":                   param("tag", "Tag name.", x{"type": "string"}),
	"stale":                 param("stale", "Only postings the link checker marked closed.", x{"type": "boolean"}),
	"archived":              param("archived", "List archived applications instead of the default view.", x{"type": "boolean"}),
	"url":                   param("url", "The posting URL.", x{"type": "string"}),
	"service":               param("service", "Set to use the favicon service instead of fetching the site.", x{"type": "string"}),
	"worker":                param("worker", "Worker ID recorded on the prepare.", x{"type": "string"}),
	"link_status":           param("status", "live, closed, or unknown.", x{"type": "string", "enum": []string{"live", "closed", "unknown"}}),
	"changed":               param("changed", "Only postings whose page changed, or did not.", x{"type": "boolean"}),
	"attention":             param("attention", "Only closed or changed postings.", x{"type": "boolean"}),
	"application_id":        param("application_id", "Only this application's events.", x{"type": "string"}),
	"after":                 param("after", "Event ID to start after.", x{"type": "string"}),
	"wait_ms":               param("wait_ms", "Wait up to this long for a new event, 0 to 60000.", x{"type": "integer", "minimum": 0, "maximum": 60000}),
	"since":                 param("since", "Date or RFC 3339 time, inclusive.", x{"type": "string"}),
	"until":                 param("until", "Date or RFC 3339 time, inclusive.", x{"type": "string"}),
	"actor":                 param("actor", "Actor name.", x{"type": "string"}),
	"action":                param("action", "Action, such as `application.update`.", x{"type": "string"}),
	"entity_type":           param("entity_type", "Entity type, such as `application`.", x{"type": "string"}),
	"entity_id":             param("entity_id", "Entity ID.", x{"type": "string"}),
	"client_id":             param("client_id", "OAuth client ID.", x{"type": "string"}),
	"redirect_uri":          param("redirect_uri", "Loopback callback URL.", x{"type": "string"}),
	"code_challenge":        param("code_challenge", "PKCE challenge.", x{"type": "string"}),
	"code_challenge_method": param("code_challenge_method", "S256.", x{"type": "string", "enum": []string{"S256"}}),
	"state":                 param("state", "Opaque state echoed to the callback.", x{"type": "string"}),
	"scope":                 param("scope", "read, write, or both.", x{"type": "string"}),
	"response_type":         param("response_type", "Accepted and ignored.", x{"type": "string"}),
	"user_code":             param("user_code", "The code the human typed.", x{"type": "string"}),
}

// operationID turns a title such as "List an application's events" into listApplicationsEvents.
func operationID(title string) string {
	var b strings.Builder
	for _, w := range strings.Fields(title) {
		w = strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, w)
		switch strings.ToLower(w) {
		case "", "a", "an", "the", "by":
			continue
		}
		if b.Len() == 0 {
			b.WriteString(strings.ToLower(w[:1]) + w[1:])
		} else {
			b.WriteString(strings.ToUpper(w[:1]) + w[1:])
		}
	}
	return b.String()
}

const intro = `The HTTP API behind the Pursuit dashboard and the ` + "`pursuit`" + ` CLI. Agents should prefer the CLI: ` + "`pursuit schema <command>`" + ` describes one command. The short agent guide is [/llms.txt](/llms.txt).

## Authentication

Send ` + "`Authorization: Bearer pst_...`" + `. A token holds the scopes ` + "`read`" + `, ` + "`write`" + `, and ` + "`approve`" + `. ` + "`write`" + ` does not include ` + "`approve`" + `, and only ` + "`approve`" + ` can move an application toward submission. Tokens, scopes, and OAuth login are in [/auth.md](/auth.md).

## Errors

Errors are JSON: ` + "`{\"error\": {\"code\", \"message\", \"hint\", \"details\"}}`" + `. 400 is validation, 401 a missing or bad token, 403 a missing scope, 404 not found, 409 a version or cap conflict.

## Writes

Every write accepts ` + "`Idempotency-Key`" + `, which replays a successful response, and ` + "`X-Dry-Run: true`" + ` (or ` + "`?dry_run=true`" + `), which validates and writes nothing. A dry run answers ` + "`{\"dry_run\": true, \"hint\", \"data\"}`" + ` instead of the response shown. Creates answer 201. Updates and deletes of versioned records take ` + "`version`" + ` in the body, the query, or ` + "`If-Match`" + `.

## Pagination

Lists answer ` + "`{\"data\": [...], \"page\": {\"limit\", \"has_more\", \"next_cursor\", \"total\"}}`" + `. They take ` + "`limit`" + ` (1-200) and ` + "`cursor`" + `; pass ` + "`page.next_cursor`" + ` back unchanged until ` + "`has_more`" + ` is false. ` + "`fields=id,company`" + ` trims each record to those fields. ` + "`POST /v1/batch`" + ` runs up to 100 requests in one call.

## Webhooks

A registered webhook receives every new event as a POST; see **Webhooks** at the end of this page for the payload.`

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
		return "Dashboard sign-in"
	case "oauth", "passkeys", "demo":
		return "Dashboard sign-in"
	}
	return upper(seg[0]) // not in tags yet: shows up on its own in the sidebar
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
