// Package dashboard is the embedded web UI served by the daemon.
// The pages call the JSON API. They do not read the database themselves.
package dashboard

import (
	"embed"
	"io/fs"
	"mime"
	"path"
	"strings"
)

// all: includes underscore-prefixed paths such as the docs export's _next/.
//
//go:embed all:dist
var files embed.FS

var dist, distErr = fs.Sub(files, "dist")

var extraTypes = map[string]string{
	".js":    "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".html":  "text/html; charset=utf-8",
	".svg":   "image/svg+xml",
	".woff2": "font/woff2",
	".json":  "application/json; charset=utf-8",
	".wasm":  "application/wasm",
}

// Open returns an embedded asset and the status to serve it with. Every real
// dashboard route is its own prerendered file, so a page path without one
// gets the exported 404.html with a 404 status.
func Open(name string) ([]byte, string, int, error) {
	if distErr != nil {
		return nil, "", 0, distErr
	}
	clean := path.Clean("/" + name)[1:]
	if clean == "" {
		clean = "index.html"
	}
	body, err := fs.ReadFile(dist, clean)
	if err != nil {
		if path.Ext(clean) != "" {
			return nil, "", 0, err
		}
		// The static docs export (/docs/*) is plain files: /docs/cli is cli.html.
		for _, alt := range []string{clean + ".html", clean + "/index.html"} {
			if b, e := fs.ReadFile(dist, alt); e == nil {
				return b, contentType(alt), 200, nil
			}
		}
		if clean == "docs" || strings.HasPrefix(clean, "docs/") {
			return nil, "", 0, err
		}
		body, err = fs.ReadFile(dist, "404.html")
		if err != nil {
			return nil, "", 0, err
		}
		return body, contentType("404.html"), 404, nil
	}
	return body, contentType(clean), 200, nil
}

func contentType(name string) string {
	ext := path.Ext(name)
	if t, ok := extraTypes[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}
