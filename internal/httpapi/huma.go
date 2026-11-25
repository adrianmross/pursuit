package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/adrianmross/appkit/humax"
	"github.com/adrianmross/appkit/paging"
	"github.com/danielgtaylor/huma/v2"

	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/service"
	"github.com/pursuit/pursuit/internal/spec"
	"github.com/pursuit/pursuit/internal/version"
)

// registerAPI adds the routes described as typed Huma operations. Their
// request shapes, validation and OpenAPI entries come from the Go types;
// the rest of http.go is being moved here one resource at a time.
func (s *Server) registerAPI(mux *http.ServeMux) {
	// Document serves both kinds of route, so Huma's own /openapi.json is off.
	s.api = humax.New(mux, "Pursuit", version.Version, func(c *huma.Config) { c.OpenAPIPath = "" })
	s.registerTags()
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
