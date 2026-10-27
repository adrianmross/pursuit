// Package dashboard is the embedded web UI served by the daemon.
// The pages call the JSON API. They do not read the database themselves.
package dashboard

import (
	"embed"
	"io/fs"
	"mime"
	"path"
)

//go:embed dist
var files embed.FS

var dist, distErr = fs.Sub(files, "dist")

var extraTypes = map[string]string{
	".js":    "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".html":  "text/html; charset=utf-8",
	".svg":   "image/svg+xml",
	".woff2": "font/woff2",
	".json":  "application/json; charset=utf-8",
}

// Open returns an embedded asset. Any path without its own file falls back
// to index.html so client-side routes (e.g. /app/:id) render the SPA shell.
func Open(name string) ([]byte, string, error) {
	if distErr != nil {
		return nil, "", distErr
	}
	clean := path.Clean("/" + name)[1:]
	if clean == "" {
		clean = "index.html"
	}
	body, err := fs.ReadFile(dist, clean)
	if err != nil {
		if path.Ext(clean) != "" {
			return nil, "", err
		}
		clean = "index.html"
		body, err = fs.ReadFile(dist, clean)
		if err != nil {
			return nil, "", err
		}
	}
	return body, contentType(clean), nil
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
