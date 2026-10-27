// Package dashboard is the embedded web UI served by the daemon.
// The pages call the JSON API. They do not read the database themselves.
package dashboard

import (
	"embed"
	"os"
)

//go:embed static/index.html static/dashboard.css static/dashboard.js
var files embed.FS

// Open returns an embedded asset. name is "/", "dashboard.css", or "dashboard.js".
func Open(name string) ([]byte, string, error) {
	file, contentType := "", ""
	switch name {
	case "", "/":
		file, contentType = "static/index.html", "text/html; charset=utf-8"
	case "dashboard.css":
		file, contentType = "static/dashboard.css", "text/css; charset=utf-8"
	case "dashboard.js":
		file, contentType = "static/dashboard.js", "text/javascript; charset=utf-8"
	default:
		return nil, "", os.ErrNotExist
	}
	body, err := files.ReadFile(file)
	if err != nil {
		return nil, "", err
	}
	return body, contentType, nil
}
