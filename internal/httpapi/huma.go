package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	"github.com/adrianmross/appkit/humax"
	"github.com/adrianmross/appkit/paging"
	"github.com/danielgtaylor/huma/v2"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/spec"
	"github.com/pursuit/pursuit/internal/sqlstore"
	"github.com/pursuit/pursuit/internal/version"
)

// registerAPI adds the routes described as typed Huma operations. Their
// request shapes, validation and OpenAPI entries come from the Go types;
// the rest of http.go is being moved here one resource at a time.
func (s *Server) registerAPI(mux *http.ServeMux) {
	// Document serves both kinds of route, so Huma's own /openapi.json is off.
	s.api = humax.New(mux, "Pursuit", version.Version, func(c *huma.Config) { c.OpenAPIPath = "" })
	s.registerTags()
	s.registerFields()
	s.registerWebhooks()
	s.registerStanding()
}

// The inputs most operations share: a JSON body, a record named in the path,
// or both.
type (
	bodyIn[B any] struct {
		humax.Write
		Body B
	}
	idWrite struct {
		humax.Write
		ID string `path:"id"`
	}
)

// sliceIn is the query of a list that is loaded whole: it returns every row
// unless limit is set, and pages by offset.
type sliceIn struct {
	humax.Fields
	Limit  int    `query:"limit" minimum:"1" maximum:"200" doc:"Page size, 1 to 200. Without it the list returns every row."`
	Cursor string `query:"cursor" doc:"page.next_cursor from the previous page, unchanged."`
}

// pageSlice is the page of rows that in asks for.
// ponytail: httpx.PageSlice's paging, repeated; move both onto one function
// in appkit's paging when another app needs it.
func pageSlice[T any](in *sliceIn, rows []T, err error) (domain.Page[T], error) {
	if err != nil {
		return domain.Page[T]{}, err
	}
	offset := 0
	if in.Cursor != "" {
		c, err := paging.DecodeCursor(in.Cursor)
		if err == nil {
			err = paging.CheckCursorSort(c, "offset")
		}
		if err != nil {
			return domain.Page[T]{}, err
		}
		if offset, err = strconv.Atoi(c.ID); err != nil || offset < 0 {
			return domain.Page[T]{}, domain.Validation("invalid cursor").WithHint("pass next_cursor from the previous page unchanged")
		}
	}
	total := len(rows)
	rows = rows[min(offset, total):]
	limit := in.Limit
	if limit == 0 {
		limit = len(rows)
	}
	out := domain.Page[T]{Data: rows[:min(limit, len(rows))], Page: domain.PageMeta{Limit: limit, Total: total}}
	if out.Data == nil {
		out.Data = []T{}
	}
	if next := offset + len(out.Data); next < total {
		out.Page.HasMore = true
		out.Page.NextCursor = paging.EncodeCursor("offset", "", strconv.Itoa(next))
	}
	return out, nil
}

// listIn is the query of a list that sorts and searches.
type listIn struct {
	humax.Fields
	Limit  int    `query:"limit" minimum:"1" maximum:"200" doc:"Page size, 1 to 200. Lists that return everything by default page only when it is set."`
	Cursor string `query:"cursor" doc:"page.next_cursor from the previous page, unchanged."`
	Sort   string `query:"sort" doc:"A field name, with an optional leading - for descending."`
	Q      string `query:"q" doc:"Text search."`
}

func (l *listIn) query() (domain.ListQuery, error) {
	q := domain.ListQuery{Limit: l.Limit, Sort: l.Sort, Q: l.Q}
	if l.Cursor != "" {
		c, err := paging.DecodeCursor(l.Cursor)
		if err != nil {
			return q, err
		}
		q.Cursor = &c
	}
	return q, nil
}

func (s *Server) registerTags() {
	humax.Get(s.api, spec.Operation("GET", "/v1/tags", "List tags", spec.Op{CLI: "pursuit tags list"}), domain.TagFields,
		func(ctx context.Context, in *listIn) (domain.Page[domain.Tag], error) {
			q, err := in.query()
			if err != nil {
				return domain.Page[domain.Tag]{}, err
			}
			return s.Svc.ListTags(ctx, q)
		})
	humax.Mutate(s.api, s.Svc.DB, spec.Operation("POST", "/v1/tags", "Create a tag", spec.Op{Status: 201, CLI: "pursuit tags create --name remote"}), domain.TagFields,
		func(ctx context.Context, in *struct {
			humax.Write
			Body service.TagInput
		}) (domain.Tag, error) {
			return s.Svc.CreateTag(ctx, in.Body)
		})
	humax.Mutate(s.api, s.Svc.DB, spec.Operation("DELETE", "/v1/tags/{id}", "Delete a tag", spec.Op{CLI: "pursuit tags delete TAG_ID"}), nil,
		func(ctx context.Context, in *struct {
			humax.Write
			ID string `path:"id"`
		}) (domain.Deleted, error) {
			return s.Svc.DeleteTag(ctx, in.ID)
		})
}

func (s *Server) registerFields() {
	humax.Get(s.api, spec.Operation("GET", "/v1/fields", "List custom fields", spec.Op{CLI: "pursuit fields list"}), domain.CustomFieldFields,
		func(ctx context.Context, in *listIn) (domain.Page[domain.CustomField], error) {
			q, err := in.query()
			if err != nil {
				return domain.Page[domain.CustomField]{}, err
			}
			return s.Svc.ListFields(ctx, q)
		})
	humax.Mutate(s.api, s.Svc.DB, spec.Operation("POST", "/v1/fields", "Create a custom field. type is text, number, or date", spec.Op{Status: 201, CLI: "pursuit fields create --name 'Salary range' --type text"}), domain.CustomFieldFields,
		func(ctx context.Context, in *bodyIn[service.FieldInput]) (domain.CustomField, error) {
			return s.Svc.CreateField(ctx, in.Body)
		})
	humax.Mutate(s.api, s.Svc.DB, spec.Operation("DELETE", "/v1/fields/{id}", "Delete a custom field", spec.Op{CLI: "pursuit fields delete FIELD_ID"}), nil,
		func(ctx context.Context, in *idWrite) (domain.Deleted, error) {
			return s.Svc.DeleteField(ctx, in.ID)
		})
}

// WebhookInput registers a webhook.
type WebhookInput struct {
	URL    string `json:"url" minLength:"1" doc:"Where each new event is posted."`
	Secret string `json:"secret" required:"false" maxLength:"200" doc:"Signs each delivery when set."`
}

func (s *Server) registerWebhooks() {
	humax.Get(s.api, spec.Operation("GET", "/v1/webhooks", "List webhooks. Without limit it returns all", spec.Op{CLI: "pursuit webhooks list"}), nil,
		func(ctx context.Context, in *sliceIn) (domain.Page[service.WebhookView], error) {
			rows, err := s.Svc.ListWebhooks(ctx)
			return pageSlice(in, rows, err)
		})
	humax.Mutate(s.api, s.Svc.DB, spec.Operation("POST", "/v1/webhooks", "Register a webhook. Every new event is posted to url; see Webhooks below for the payload", spec.Op{Status: 201, CLI: "pursuit webhooks add https://example.com/hook"}), nil,
		func(ctx context.Context, in *bodyIn[WebhookInput]) (service.WebhookView, error) {
			return s.Svc.CreateWebhook(ctx, in.Body.URL, in.Body.Secret)
		})
	humax.Mutate(s.api, s.Svc.DB, spec.Operation("DELETE", "/v1/webhooks/{id}", "Delete a webhook", spec.Op{CLI: "pursuit webhooks delete WEBHOOK_ID"}), nil,
		func(ctx context.Context, in *idWrite) (domain.Deleted, error) {
			return s.Svc.DeleteWebhook(ctx, in.ID)
		})
}

// StandingInput sets a standing answer.
type StandingInput struct {
	Key    string `json:"key" minLength:"1" doc:"A word such as work_authorization or email."`
	Answer string `json:"answer" minLength:"1" maxLength:"20000"`
}

func (s *Server) registerStanding() {
	humax.Get(s.api, spec.Operation("GET", "/v1/standing", "List standing answers. Without limit it returns all", spec.Op{CLI: "pursuit standing list"}), nil,
		func(ctx context.Context, in *sliceIn) (domain.Page[sqlstore.StandingAnswer], error) {
			rows, err := s.Svc.ListStanding(ctx)
			return pageSlice(in, rows, err)
		})
	humax.Mutate(s.api, s.Svc.DB, spec.Operation("POST", "/v1/standing", "Set a standing answer. Profile values also load from PURSUIT_PROFILE", spec.Op{CLI: "pursuit standing set --key work_authorization --value 'US citizen'"}), nil,
		func(ctx context.Context, in *bodyIn[StandingInput]) (sqlstore.StandingAnswer, error) {
			return s.Svc.SetStanding(ctx, in.Body.Key, in.Body.Answer)
		})
	humax.Mutate(s.api, s.Svc.DB, spec.Operation("DELETE", "/v1/standing/{key}", "Delete a standing answer. A PURSUIT_PROFILE value for the same key still applies", spec.Op{CLI: "pursuit standing delete work_authorization"}), nil,
		func(ctx context.Context, in *struct {
			humax.Write
			Key string `path:"key"`
		}) (domain.Deleted, error) {
			return s.Svc.DeleteStanding(ctx, in.Key)
		})
}

// Document is the OpenAPI description served at /openapi.json: the routes
// internal/spec lists, plus the ones registered as Huma operations.
// ponytail: two sources merged while routes move to Huma; when the last one
// has, this is s.api.OpenAPI() and internal/spec's builder goes.
func Document() []byte { return document() }

var document func() []byte

func init() { document = sync.OnceValue(buildDocument) }

func buildDocument() []byte {
	s := &Server{Svc: &service.Service{}}
	s.Handler()
	var doc, typed map[string]any
	if err := json.Unmarshal(spec.Document(), &doc); err != nil {
		panic(err)
	}
	b, err := json.Marshal(s.api.OpenAPI())
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(b, &typed); err != nil {
		panic(err)
	}
	paths := doc["paths"].(map[string]any)
	ids := map[string]bool{}
	for _, item := range paths {
		for _, op := range item.(map[string]any) {
			ids[op.(map[string]any)["operationId"].(string)] = true
		}
	}
	typedPaths, _ := typed["paths"].(map[string]any)
	for path, item := range typedPaths {
		into, _ := paths[path].(map[string]any)
		if into == nil {
			into = map[string]any{}
			paths[path] = into
		}
		for method, op := range item.(map[string]any) {
			id, _ := op.(map[string]any)["operationId"].(string)
			if into[method] != nil || ids[id] {
				panic("openapi: " + method + " " + path + " (" + id + ") is described twice")
			}
			ids[id] = true
			into[method] = op
		}
	}
	// A type both sources describe takes Huma's schema.
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	if c, _ := typed["components"].(map[string]any); c != nil {
		for name, schema := range c["schemas"].(map[string]any) {
			schemas[name] = schema
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return append(out, '\n')
}
