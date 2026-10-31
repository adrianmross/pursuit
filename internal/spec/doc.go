package spec

import (
	_ "embed"
	"encoding/json"

	"github.com/pursuit/pursuit/internal/version"
)

//go:embed llms.txt
var LLMS []byte

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
	add := func(path, method, summary string, write bool, extra map[string]any) {
		op := map[string]any{
			"summary":     summary,
			"operationId": method + "_" + slug(path),
			"responses": map[string]any{
				"200": map[string]any{"description": "JSON body"},
				"400": errRef(),
				"401": errRef(),
				"403": errRef(),
				"404": errRef(),
				"409": errRef(),
			},
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
		"/healthz": "liveness", "/readyz": "readiness", "/version": "version", "/openapi.json": "this document", "/llms.txt": "short agent guide",
	}
	for p, s := range pub {
		add(p, "get", s, false, map[string]any{"security": []any{}})
	}
	add("/v1/me", "get", "current user and token", false, nil)
	add("/v1/me", "patch", "set display name or avatar (png, jpeg, or webp data URL under 256KB; empty clears it)", true, nil)
	add("/v1/tokens", "get", "list tokens", false, nil)
	add("/v1/tokens", "post", "create a token; secret is returned once", true, nil)
	add("/v1/tokens/{id}", "delete", "revoke a token", true, nil)
	add("/v1/companies", "get", "list companies", false, nil)
	add("/v1/companies", "post", "create a company", true, nil)
	add("/v1/companies/{id}", "get", "get a company", false, nil)
	add("/v1/companies/{id}", "patch", "update a company; body.version is required", true, nil)
	add("/v1/companies/{id}", "delete", "delete a company; version query or If-Match", true, nil)
	add("/v1/roles", "get", "list roles", false, nil)
	add("/v1/roles", "post", "create a role", true, nil)
	add("/v1/roles/{id}", "get", "get a role", false, nil)
	add("/v1/roles/{id}", "patch", "update a role", true, nil)
	add("/v1/roles/{id}", "delete", "delete a role", true, nil)
	add("/v1/applications", "get", "list applications. status, company, fit, source, tag, q, stale, cursor, limit, sort, fields", false, nil)
	add("/v1/applications/by-url", "get", "lookup one application by posting URL. query url. 404 when the page is not tracked", false, nil)
	add("/v1/applications/capture", "post", "save the posting the user is viewing. dedupes by URL and creates status to_apply", true, nil)
	add("/v1/applications", "post", "create an application", true, nil)
	add("/v1/applications/{id}", "get", "get an application", false, nil)
	add("/v1/applications/{id}", "patch", "update an application. status changes obey the approval rule", true, nil)
	add("/v1/applications/{id}", "delete", "delete an application", true, nil)
	add("/v1/applications/{id}/transition", "post", "change status. a stage replaces the status and clears any outcome; waiting, needs_input, failed, rejected, withdrawn and ghosted are outcomes that keep the stage. approved requires the approve scope. submitting requires a stored approval", true, nil)
	add("/v1/applications/{id}/events", "get", "list events for an application", false, nil)
	add("/v1/applications/{id}/events", "post", "append email_received, interview_scheduled, or note", true, nil)
	add("/v1/applications/{id}/interviews", "get", "list interview rounds for an application. limit, cursor, fields; no limit returns all", false, nil)
	add("/v1/applications/{id}/interviews", "post", "add an interview round. stage is prepared, applied, screening, interviewing, final, or offer and defaults to the current stage. prep holds what to rehearse", true, nil)
	add("/v1/interviews/{id}", "get", "get an interview round", false, nil)
	add("/v1/interviews/{id}", "patch", "update an interview round; body.version is required. done true/false marks it done", true, nil)
	add("/v1/interviews/{id}", "delete", "delete an interview round; version query or If-Match", true, nil)
	add("/v1/applications/{id}/tags", "post", "attach a tag by name or tag_id", true, nil)
	add("/v1/applications/{id}/tags/{tagID}", "delete", "detach a tag", true, nil)
	add("/v1/applications/{id}/prepare", "post", "one-step prepare: pull questions, draft answers, stop at awaiting_review or needs_input", true, nil)
	add("/v1/applications/{id}/enqueue", "post", "enqueue prepare or submit work", true, nil)
	add("/v1/applications/{id}/artifacts", "get", "list artifact metadata. limit, cursor; no limit returns all", false, nil)
	add("/v1/applications/{id}/artifacts", "post", "attach resume_pdf, cover_letter, answers_json, form_snapshot, or confirmation", true, nil)
	add("/v1/applications/{id}/questions", "get", "list form questions and answers. limit, cursor; no limit returns all", false, nil)
	add("/v1/applications/{id}/check-link", "post", "re-check this posting", true, nil)
	add("/v1/approvals", "post", "human approval. requires the approve scope, marks awaiting_review as approved, and enqueues submit. it does not submit a form", true, nil)
	add("/v1/approvals/reject", "post", "decline a prepared application. requires the approve scope. the application keeps its awaiting_review stage with outcome withdrawn and nothing is submitted", true, nil)
	add("/v1/links", "get", "list checked postings. attention=true returns closed or changed. status is live, closed, or unknown", false, nil)
	add("/v1/apply", "post", "batch prepare or async enqueue", true, nil)
	add("/v1/batch", "post", "run up to 100 API requests in order as the caller. body {requests:[{method,path,body,idempotency_key}],stop_on_error}. returns {results:[{status,body}],failed}. scopes apply per item, X-Dry-Run applies to every item, items are not atomic, and stop_on_error ends results at the first failure. Idempotency-Key goes on each item, not the batch", true, nil)
	add("/v1/queue/claim", "post", "lease the next work item. replicas use FOR UPDATE SKIP LOCKED on Postgres", true, nil)
	add("/v1/queue/{id}/heartbeat", "post", "extend a lease", true, nil)
	add("/v1/queue/{id}/complete", "post", "finish leased work and set the resulting status", true, nil)
	add("/v1/artifacts/{id}", "get", "fetch an artifact body", false, nil)
	add("/v1/profile", "get", "applicant profile: standing answers as a map, for the browser extension", false, nil)
	add("/v1/documents", "get", "list user documents: base_resume, cover_letter, other. limit, cursor; no limit returns all", false, nil)
	add("/v1/documents", "post", "upload a document as multipart (kind, file) or JSON text. prepare attaches the newest base_resume", true, nil)
	add("/v1/documents/{id}", "get", "document with body; binary bodies are base64", false, nil)
	add("/v1/documents/{id}", "delete", "delete a document", true, nil)
	add("/v1/standing", "get", "list standing answers. limit, cursor; no limit returns all", false, nil)
	add("/v1/standing", "post", "set a standing answer. profile values also load from PURSUIT_PROFILE", true, nil)
	add("/v1/standing/{key}", "delete", "delete a standing answer. a PURSUIT_PROFILE value for the same key still applies", true, nil)
	add("/v1/caps", "get", "per-company application caps", false, nil)
	add("/v1/links/check", "post", "sweep postings. one replica holds the advisory lock", true, nil)
	add("/v1/cron/links", "get", "daily link sweep for Vercel Cron. requires Authorization Bearer CRON_SECRET, not a Pursuit token", false, nil)
	add("/v1/webhooks", "get", "list webhooks. limit, cursor; no limit returns all", false, nil)
	add("/v1/webhooks", "post", "register a webhook", true, nil)
	add("/v1/webhooks/{id}", "delete", "delete a webhook", true, nil)
	add("/v1/events", "get", "list events. after plus wait_ms long-polls", false, nil)
	add("/v1/tags", "get", "list tags", false, nil)
	add("/v1/tags", "post", "create a tag", true, nil)
	add("/v1/tags/{id}", "delete", "delete a tag", true, nil)
	add("/v1/audit", "get", "audit log, including actor_name and metadata. filter with q (summary, action, entity, actor, metadata), since and until (date or RFC 3339), actor, action, entity_type, entity_id. sort=created_at for oldest first", false, nil)
	add("/v1/import/csv", "post", "import the tracker CSV. Prepared maps to awaiting_review. Applied stays applied. The same posting_url updates the existing application", true, nil)
	add("/v1/import/json", "post", "import a JSON array of postings", true, nil)
	add("/v1/import/markdown", "post", "import markdown tables of roles as to_apply, deduped by URL", true, nil)
	add("/v1/import/seed", "post", "import the committed seed", true, nil)
	add("/v1/export/simplify", "get", "Simplify Job Tracker CSV", false, nil)
	add("/v1/export/json", "get", "JSON export", false, nil)

	doc := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "Pursuit",
			"version":     version.Version,
			"description": "Job-application tracker. Agents should prefer the pursuit CLI. Fetch this document when you need the HTTP shape; use `pursuit schema <command>` for one CLI command.",
		},
		"servers":  []any{map[string]any{"url": "/"}},
		"security": []any{map[string]any{"bearerAuth": []any{}}},
		"paths":    paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{"type": "http", "scheme": "bearer"},
			},
			"parameters": map[string]any{
				"IdempotencyKey": map[string]any{"name": "Idempotency-Key", "in": "header", "schema": map[string]any{"type": "string"}},
				"DryRun":         map[string]any{"name": "X-Dry-Run", "in": "header", "schema": map[string]any{"type": "string", "enum": []string{"true"}}},
			},
			"schemas": map[string]any{
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
