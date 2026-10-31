package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/pursuit/pursuit/internal/service"
)

// OAuth2 endpoints for `pursuit auth login`.
//
// Public, form-encoded, RFC-shaped errors ({"error": "..."}):
//   POST /v1/auth/device_authorization  RFC 8628
//   POST /v1/auth/token                 authorization_code + PKCE, device_code
//   POST /v1/auth/revoke                RFC 7009
//
// Authenticated with the dashboard session (approve scope), JSON:
//   POST /v1/oauth/authorize            loopback consent; returns redirect_to
//   GET  /v1/oauth/device?user_code=    what a device code is asking for
//   POST /v1/oauth/device/approve       approve or deny a device code
//
// The authorize URL the CLI opens is the dashboard page /authorize, which
// signs the human in and calls POST /v1/oauth/authorize.

func (s *Server) registerOAuth(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/device_authorization", s.oauthDeviceAuthorization)
	mux.HandleFunc("POST /v1/auth/token", s.oauthToken)
	mux.HandleFunc("POST /v1/auth/revoke", s.oauthRevoke)
	mux.HandleFunc("POST /v1/oauth/authorize", s.oauthAuthorize)
	mux.HandleFunc("GET /v1/oauth/device", s.oauthDeviceLookup)
	mux.HandleFunc("POST /v1/oauth/device/approve", s.oauthDeviceApprove)
}

func (s *Server) oauthForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		writeOAuthErr(w, &service.OAuthError{Code: "invalid_request", Description: "send an application/x-www-form-urlencoded body"})
		return false
	}
	return true
}

// writeOAuthErr writes an RFC 6749 section 5.2 error. Non-OAuth errors
// become server_error without leaking details.
func writeOAuthErr(w http.ResponseWriter, err error) {
	var oe *service.OAuthError
	if !errors.As(err, &oe) {
		oe = &service.OAuthError{Code: "server_error", Description: "internal error"}
	}
	status := http.StatusBadRequest
	if oe.Code == "server_error" {
		status = http.StatusInternalServerError
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, oe)
}

func (s *Server) oauthDeviceAuthorization(w http.ResponseWriter, r *http.Request) {
	if !s.oauthForm(w, r) {
		return
	}
	out, err := s.Svc.OAuthDeviceStart(r.Context(), r.PostForm.Get("client_id"), r.PostForm.Get("scope"), publicBase(r))
	if err != nil {
		s.logOAuth("device_authorization", err)
		writeOAuthErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, out)
}

func (s *Server) oauthToken(w http.ResponseWriter, r *http.Request) {
	if !s.oauthForm(w, r) {
		return
	}
	f := r.PostForm
	var out service.OAuthToken
	var err error
	switch f.Get("grant_type") {
	case "authorization_code":
		out, err = s.Svc.OAuthExchangeCode(r.Context(), f.Get("code"), f.Get("code_verifier"), f.Get("redirect_uri"), f.Get("client_id"))
	case service.DeviceCodeGrantType:
		out, err = s.Svc.OAuthExchangeDevice(r.Context(), f.Get("device_code"), f.Get("client_id"))
	default:
		err = &service.OAuthError{Code: "unsupported_grant_type", Description: "use authorization_code or " + service.DeviceCodeGrantType}
	}
	if err != nil {
		s.logOAuth("token", err)
		writeOAuthErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, 200, out)
}

func (s *Server) oauthRevoke(w http.ResponseWriter, r *http.Request) {
	if !s.oauthForm(w, r) {
		return
	}
	if err := s.Svc.OAuthRevoke(r.Context(), r.PostForm.Get("token")); err != nil {
		s.logOAuth("revoke", err)
		writeOAuthErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"revoked": true})
}

// logOAuth records unexpected failures only, never codes or tokens.
// Expected protocol answers (authorization_pending, invalid_grant) are noise.
func (s *Server) logOAuth(endpoint string, err error) {
	var oe *service.OAuthError
	if errors.As(err, &oe) {
		return
	}
	s.log().Error("oauth", "endpoint", endpoint, "err", err.Error())
}

func (s *Server) oauthAuthorize(w http.ResponseWriter, r *http.Request) {
	body, ok := s.read(w, r)
	if !ok {
		return
	}
	var in service.AuthorizeInput
	if err := decode(body, &in); err != nil {
		writeErr(w, err)
		return
	}
	to, err := s.Svc.OAuthAuthorize(r.Context(), in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"redirect_to": to})
}

func (s *Server) oauthDeviceLookup(w http.ResponseWriter, r *http.Request) {
	q, err := query(r, "user_code")
	if err != nil {
		writeErr(w, err)
		return
	}
	out, err := s.Svc.OAuthDeviceLookup(r.Context(), first(q, "user_code"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (s *Server) oauthDeviceApprove(w http.ResponseWriter, r *http.Request) {
	body, ok := s.read(w, r)
	if !ok {
		return
	}
	var in struct {
		UserCode string `json:"user_code"`
		Approve  bool   `json:"approve"`
	}
	if err := decode(body, &in); err != nil {
		writeErr(w, err)
		return
	}
	status, err := s.Svc.OAuthDeviceDecide(r.Context(), in.UserCode, in.Approve)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": status})
}

// publicBase is the origin the caller reached us on, for verification_uri.
func publicBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if fh := r.Header.Get("X-Forwarded-Host"); fh != "" {
		host = strings.TrimSpace(strings.Split(fh, ",")[0])
	}
	return scheme + "://" + host
}
