package httpapi

import (
	"net/http"
	"strings"

	"github.com/pursuit/pursuit/internal/service"
)

// Discovery documents for OAuth clients that find us from a URL alone.
//
//	GET /.well-known/oauth-authorization-server              RFC 8414
//	GET /.well-known/oauth-protected-resource[/path]         RFC 9728
//
// Both list only what is served today. Refresh tokens are advertised in the phase that ships them.

func (s *Server) registerOAuthMetadata(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.oauthServerMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.oauthResourceMetadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/{path...}", s.oauthResourceMetadata)
}

// oauthIssuer is the configured public URL, else the origin of the request.
func (s *Server) oauthIssuer(r *http.Request) string {
	return s.Svc.BaseURL(publicBase(r))
}

func writeMetadata(w http.ResponseWriter, doc map[string]any) {
	w.Header().Set("Cache-Control", "public, max-age=3600")
	// Browser-based clients fetch these cross-origin.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	writeJSON(w, 200, doc)
}

func (s *Server) oauthServerMetadata(w http.ResponseWriter, r *http.Request) {
	iss := s.oauthIssuer(r)
	writeMetadata(w, map[string]any{
		"issuer":                                         iss,
		"authorization_endpoint":                         iss + "/authorize",
		"token_endpoint":                                 iss + "/v1/auth/token",
		"revocation_endpoint":                            iss + "/v1/auth/revoke",
		"registration_endpoint":                          iss + "/v1/auth/register",
		"client_id_metadata_document_supported":          true,
		"authorization_response_iss_parameter_supported": true,
		"device_authorization_endpoint":                  iss + "/v1/auth/device_authorization",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", service.DeviceCodeGrantType},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"scopes_supported":                               []string{"read", "write"},
	})
}

func (s *Server) oauthResourceMetadata(w http.ResponseWriter, r *http.Request) {
	iss := s.oauthIssuer(r)
	resource := iss
	if p := strings.Trim(r.PathValue("path"), "/"); p != "" {
		resource += "/" + p
	}
	writeMetadata(w, map[string]any{
		"resource":                 resource,
		"authorization_servers":    []string{iss},
		"scopes_supported":         []string{"read", "write"},
		"bearer_methods_supported": []string{"header"},
	})
}
