// Package handler is the Vercel serverless entrypoint.
// Vercel compiles this file from outside the module, so it imports only pkg/vercelapp.
package handler

import (
	"net/http"

	"github.com/pursuit/pursuit/pkg/vercelapp"
)

// Handler serves the embedded dashboard and the REST API.
// vercel.json rewrites every path to this function.
func Handler(w http.ResponseWriter, r *http.Request) {
	vercelapp.Handler(w, r)
}
