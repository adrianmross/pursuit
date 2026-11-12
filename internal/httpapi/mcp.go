package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/version"
)

// POST /mcp is a Model Context Protocol server (Streamable HTTP, stateless:
// every request is one JSON-RPC message answered with one JSON body). It is
// how claude.ai connectors and ChatGPT apps reach Pursuit; agents with a
// shell keep using the CLI. It is a thin layer over the same service methods
// as the REST API, so scopes, audit and limits are the same, and it exposes
// nothing that approves, rejects or submits.
//
// Tokens come from the OAuth flow in oauth.go. A bad or missing token is a
// real 401 with a WWW-Authenticate challenge pointing at the resource
// metadata, which is what starts the sign-in in both clients. A token bound
// to another resource is refused.

const mcpPath = "/mcp"

// mcpVersions are the protocol revisions understood, newest first. The
// client's own is echoed when it is one of them.
var mcpVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

func (s *Server) registerMCP(mux *http.ServeMux) {
	mux.HandleFunc("POST "+mcpPath, s.mcp)
	// No server-initiated stream and no sessions to end.
	mux.HandleFunc("GET "+mcpPath, mcpNoStream)
	mux.HandleFunc("DELETE "+mcpPath, mcpNoStream)
}

func mcpNoStream(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Allow", "POST")
	http.Error(w, "this MCP server answers POST only", http.StatusMethodNotAllowed)
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// toolResult is a tools/call result. Failures the model can act on are
// results with IsError set, not protocol errors.
type toolResult struct {
	Content           []map[string]string `json:"content"`
	StructuredContent json.RawMessage     `json:"structuredContent,omitempty"`
	IsError           bool                `json:"isError,omitempty"`
	Meta              map[string]any      `json:"_meta,omitempty"`
}

type mcpTool struct {
	name, title, description string
	schema                   map[string]any
	readOnly                 bool
	// scope is what the caller needs: read or write.
	scope string
	run   func(ctx context.Context, args json.RawMessage) (any, error)
}

func obj(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

var (
	strProp = func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	idProp  = strProp("The application id, from list_applications.")
)

func (s *Server) mcpTools() []mcpTool {
	svc := s.Svc
	return []mcpTool{{
		name: "list_applications", title: "List applications", readOnly: true, scope: "read",
		description: "List the user's job applications, newest first. Filter with q (text search) and status.",
		schema: obj(map[string]any{
			"q":      strProp("Text to search for in company, title and notes."),
			"status": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Only these pipeline statuses, for example to_apply, applied, screening."},
			"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "description": "How many to return. Default 25."},
		}),
		run: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct {
				Q      string   `json:"q"`
				Status []string `json:"status"`
				Limit  int      `json:"limit"`
			}
			if err := decodeArgs(raw, &in); err != nil {
				return nil, err
			}
			if in.Limit == 0 {
				in.Limit = 25
			}
			if in.Limit < 1 || in.Limit > 50 {
				return nil, domain.Validation("limit is an integer from 1 to 50")
			}
			return svc.ListApplications(ctx, domain.ListQuery{Limit: in.Limit, Q: in.Q, Status: in.Status})
		},
	}, {
		name: "get_application", title: "Get an application", readOnly: true, scope: "read",
		description: "Get one application with its status, role, company, notes and next step.",
		schema:      obj(map[string]any{"id": idProp}, "id"),
		run: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ ID string }
			if err := decodeArgs(raw, &in); err != nil {
				return nil, err
			}
			return svc.GetApplication(ctx, in.ID)
		},
	}, {
		name: "capture_posting", title: "Save a job posting", scope: "write",
		description: "Save a job posting as an application to apply to. The same posting_url returns the existing application instead of a second one.",
		schema: obj(map[string]any{
			"posting_url": strProp("The URL of the job posting."),
			"company":     strProp("Company name."),
			"title":       strProp("Job title."),
			"location":    strProp("Where the job is, if known."),
			"notes":       strProp("Anything worth remembering about it."),
		}, "posting_url", "company", "title"),
		run: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct {
				PostingURL string `json:"posting_url"`
				Company    string `json:"company"`
				Title      string `json:"title"`
				Location   string `json:"location"`
				Notes      string `json:"notes"`
			}
			if err := decodeArgs(raw, &in); err != nil {
				return nil, err
			}
			return svc.CapturePosting(ctx, serviceApplicationInput(in.PostingURL, in.Company, in.Title, in.Location, in.Notes))
		},
	}, {
		name: "prepare_application", title: "Prepare an application", scope: "write",
		description: "Prepare an application: fill in what Pursuit already knows and list the questions it cannot answer yet. It does not submit anything; a human approves in the dashboard first.",
		schema:      obj(map[string]any{"id": idProp}, "id"),
		run: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ ID string }
			if err := decodeArgs(raw, &in); err != nil {
				return nil, err
			}
			return svc.Prepare(ctx, in.ID, "mcp")
		},
	}, {
		name: "enqueue_prepare", title: "Queue an application for preparing", scope: "write",
		description: "Put an application on the prepare queue for a worker to pick up later, instead of preparing it now.",
		schema:      obj(map[string]any{"id": idProp}, "id"),
		run: func(ctx context.Context, raw json.RawMessage) (any, error) {
			var in struct{ ID string }
			if err := decodeArgs(raw, &in); err != nil {
				return nil, err
			}
			return svc.Enqueue(ctx, in.ID, "prepare")
		},
	}}
}

// decodeArgs reads tool arguments, ignoring members a client adds.
func decodeArgs(raw json.RawMessage, dst any) error {
	if len(raw) == 0 || string(raw) == "null" {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return domain.Validation("arguments are not valid: %s", err.Error())
	}
	return nil
}

func (s *Server) mcp(w http.ResponseWriter, r *http.Request) {
	base := s.oauthIssuer(r)
	resource := base + mcpPath
	metadata := base + "/.well-known/oauth-protected-resource" + mcpPath
	challenge := func(extra string) string {
		return `Bearer resource_metadata="` + metadata + `", scope="read write"` + extra
	}
	// A browser page on another origin must not be able to drive this.
	if o := r.Header.Get("Origin"); o != "" && !s.allowOrigin(o) && !sameOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	raw, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		w.Header().Set("WWW-Authenticate", challenge(""))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "error_description": "sign in to use this server"})
		return
	}
	actor, err := s.Svc.Authenticate(r.Context(), raw)
	if err != nil || (actor.Resource != "" && actor.Resource != resource) {
		w.Header().Set("WWW-Authenticate", challenge(`, error="invalid_token"`))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_token", "error_description": "the token is invalid, expired or for another resource"})
		return
	}
	// Per token, so one connected app cannot starve the service for the rest.
	if !s.allow(r.Context(), ruleMCP, actor.TokenID) {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited", "error_description": "too many requests from this app; wait a minute"})
		return
	}
	ctx := domain.WithActor(r.Context(), actor)

	var req rpcRequest
	body := http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(body)
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error: send one JSON-RPC object"}})
		return
	}
	reply := func(result any) { writeJSON(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}) }
	fail := func(code int, msg string) {
		writeJSON(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: code, Message: msg}})
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		fail(-32600, "invalid request")
		return
	}
	if len(req.ID) == 0 { // a notification gets no answer
		w.WriteHeader(http.StatusAccepted)
		return
	}
	tools := s.mcpTools()
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		v := mcpVersions[0]
		for _, known := range mcpVersions {
			if known == p.ProtocolVersion {
				v = known
			}
		}
		reply(map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "pursuit", "title": "Pursuit", "version": version.Version},
			"instructions":    "Pursuit tracks job applications. List and read them, save postings, and prepare applications. Approving and submitting happen in the Pursuit dashboard by the user, never here.",
		})
	case "ping":
		reply(map[string]any{})
	case "tools/list":
		list := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			scopes := []map[string]any{{"type": "oauth2", "scopes": []string{t.scope}}}
			list = append(list, map[string]any{
				"name": t.name, "title": t.title, "description": t.description, "inputSchema": t.schema,
				"annotations": map[string]any{
					"readOnlyHint": t.readOnly, "destructiveHint": false, "idempotentHint": t.readOnly, "openWorldHint": false,
				},
				// ChatGPT reads these per tool; the _meta copy is for clients that only read _meta.
				"securitySchemes": scopes,
				"_meta":           map[string]any{"securitySchemes": scopes},
			})
		}
		reply(map[string]any{"tools": list})
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			fail(-32602, "invalid params")
			return
		}
		for _, t := range tools {
			if t.name == p.Name {
				reply(s.mcpCall(ctx, t, p.Arguments, actor, challenge))
				return
			}
		}
		fail(-32602, "unknown tool "+p.Name)
	default:
		fail(-32601, "method not found: "+req.Method)
	}
}

func (s *Server) mcpCall(ctx context.Context, t mcpTool, args json.RawMessage, actor domain.Actor, challenge func(string) string) toolResult {
	text := func(msg string) []map[string]string { return []map[string]string{{"type": "text", "text": msg}} }
	if !actor.Can(t.scope) {
		desc := "this app was not granted " + t.scope + " access; connect it again and allow it"
		return toolResult{
			Content: text("Insufficient scope: " + desc), IsError: true,
			Meta: map[string]any{"mcp/www_authenticate": []string{challenge(`, error="insufficient_scope", error_description="` + desc + `"`)}},
		}
	}
	out, err := t.run(ctx, args)
	if err != nil {
		var de *domain.Error
		msg := "something went wrong; try again"
		if errors.As(err, &de) {
			msg = de.Message
			if de.Hint != "" {
				msg += " (" + de.Hint + ")"
			}
		} else {
			s.log().Error("mcp tool", "tool", t.name, "err", err.Error())
		}
		return toolResult{Content: text(msg), IsError: true}
	}
	b, err := marshal(out)
	if err != nil {
		return toolResult{Content: text(fmt.Sprintf("could not encode the result: %v", err)), IsError: true}
	}
	res := toolResult{Content: text(string(b))}
	if strings.HasPrefix(string(b), "{") { // structuredContent must be an object
		res.StructuredContent = b
	}
	return res
}

func serviceApplicationInput(postingURL, company, title, location, notes string) service.ApplicationInput {
	return service.ApplicationInput{PostingURL: postingURL, Company: company, Title: title, Location: location, Notes: notes}
}
