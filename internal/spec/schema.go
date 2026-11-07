package spec

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/service"
)

// schemas turns Go types into components.schemas, so the document follows
// the structs the handlers actually encode and decode.
type schemas map[string]any

// renamed are Go names that differ from what the product calls them.
var renamed = map[string]string{
	"Interview":      "Round",
	"InterviewInput": "RoundInput",
	"InterviewPatch": "RoundPatch",
	"InterviewCount": "Progress",
}

var (
	timeType = reflect.TypeOf(time.Time{})
	rawType  = reflect.TypeOf(json.RawMessage(nil))
	optType  = reflect.TypeOf(service.OptString{})
)

// of returns a schema for v's type. Response types list their always-present
// fields as required; request types don't, since the service validates input.
func (g schemas) of(v any, request bool) map[string]any {
	return g.typ(reflect.TypeOf(v), request)
}

func (g schemas) typ(t reflect.Type, request bool) map[string]any {
	switch t {
	case timeType:
		return map[string]any{"type": "string", "format": "date-time"}
	case rawType:
		return map[string]any{}
	case optType:
		return map[string]any{"type": []string{"string", "null"}}
	}
	switch t.Kind() {
	case reflect.Pointer:
		return nullable(g.typ(t.Elem(), request))
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{"type": "string", "contentEncoding": "base64"}
		}
		return map[string]any{"type": "array", "items": g.typ(t.Elem(), request)}
	case reflect.Map:
		m := map[string]any{"type": "object", "additionalProperties": g.typ(t.Elem(), request)}
		if !request {
			m["type"] = []string{"object", "null"} // a nil map encodes as null
		}
		return m
	case reflect.Struct:
		name := t.Name()
		if r, ok := renamed[name]; ok {
			name = r
		}
		if name == "" {
			return g.object(t, request)
		}
		if _, ok := g[name]; !ok {
			g[name] = nil // reserve the name before recursing
			g[name] = g.object(t, request)
		}
		return map[string]any{"$ref": "#/components/schemas/" + name}
	}
	return map[string]any{}
}

func (g schemas) object(t reflect.Type, request bool) map[string]any {
	props := map[string]any{}
	var required []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		props[name] = g.typ(f.Type, request)
		if !request && !strings.Contains(opts, "omitempty") {
			required = append(required, name)
		}
	}
	out := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

func nullable(s map[string]any) map[string]any {
	if t, ok := s["type"].(string); ok {
		out := map[string]any{}
		for k, v := range s {
			out[k] = v
		}
		out["type"] = []string{t, "null"}
		return out
	}
	if len(s) == 0 {
		return s
	}
	return map[string]any{"anyOf": []any{s, map[string]any{"type": "null"}}}
}
