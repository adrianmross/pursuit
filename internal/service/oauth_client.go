package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/pursuit/pursuit/internal/auth"
	"github.com/pursuit/pursuit/internal/domain"
	"github.com/pursuit/pursuit/internal/sqlstore"
)

// Clients other than the CLI identify themselves two ways. A client ID
// that is an https URL names a metadata document we fetch (CIMD, the way
// Claude and ChatGPT prefer). Anything else registers first through
// POST /v1/auth/register (RFC 7591). Both are public clients: PKCE, no secret.

const (
	maxClientIDURL    = 512
	clientDocMaxBytes = 64 << 10
	clientDocTTL      = time.Hour
	maxRedirectURIs   = 5
	maxDCRClients     = 5000
	dcrUnusedTTL      = 30 * 24 * time.Hour
)

// OAuthClient is who is asking for a token, as the consent page shows it.
type OAuthClient struct {
	ID           string
	Name         string
	RedirectURIs []string
	// Source is builtin (the CLI and legacy loopback tools), cimd or dcr.
	Source string
}

func isClientIDURL(id string) bool { return strings.HasPrefix(id, "https://") }

// resolveClient finds the client for a client_id. Builtin clients keep the
// strict loopback rule; an unregistered plain ID is still accepted as one,
// as before.
func (s *Service) resolveClient(ctx context.Context, id string) (OAuthClient, error) {
	id, ok := normalizeClientID(id)
	if !ok {
		return OAuthClient{}, oauthErr("invalid_request", fmt.Sprintf("client_id is at most %d characters", maxClientID))
	}
	if !isClientIDURL(id) {
		c, err := s.DB.GetOAuthClient(ctx, id)
		if domain.IsCode(err, "not_found") {
			return OAuthClient{ID: id, Name: id, Source: "builtin"}, nil
		}
		if err != nil {
			return OAuthClient{}, err
		}
		return clientFromRow(c), nil
	}
	if err := validClientIDURL(id); err != nil {
		return OAuthClient{}, oauthErr("invalid_client", err.Error())
	}
	row, err := s.DB.GetOAuthClient(ctx, id)
	if err != nil && !domain.IsCode(err, "not_found") {
		return OAuthClient{}, err
	}
	if err == nil && row.ExpiresAt > domain.FormatTime(s.now()) {
		return clientFromRow(row), nil
	}
	return s.fetchClient(ctx, id)
}

func clientFromRow(r sqlstore.OAuthClient) OAuthClient {
	return OAuthClient{ID: r.ID, Name: r.Name, RedirectURIs: r.RedirectURIs, Source: r.Source}
}

// validClientIDURL applies the client ID metadata document rules: https,
// a path, no credentials, fragment or dot segments, and the default port.
func validClientIDURL(raw string) error {
	u, err := url.Parse(raw)
	bad := errors.New("client_id must be an https URL with a path, no credentials, fragment or dot segments, on port 443")
	if err != nil || len(raw) > maxClientIDURL || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" ||
		u.Path == "" || u.Path == "/" || (u.Port() != "" && u.Port() != "443") {
		return bad
	}
	for _, seg := range strings.Split(u.Path, "/") {
		if seg == "." || seg == ".." {
			return bad
		}
	}
	return nil
}

type clientDoc struct {
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
	AuthMethod   string   `json:"token_endpoint_auth_method"`
}

func (s *Service) fetchClient(ctx context.Context, id string) (OAuthClient, error) {
	fetch := s.ClientDoc
	if fetch == nil {
		fetch = FetchClientDoc
	}
	body, err := fetch(ctx, id)
	if err != nil {
		return OAuthClient{}, oauthErr("invalid_client", "could not fetch the client metadata document: "+err.Error())
	}
	var doc clientDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return OAuthClient{}, oauthErr("invalid_client", "the client metadata document is not JSON")
	}
	if doc.ClientID != id {
		return OAuthClient{}, oauthErr("invalid_client", "client_id in the document does not match the URL it was fetched from")
	}
	if doc.AuthMethod != "" && doc.AuthMethod != "none" {
		return OAuthClient{}, oauthErr("invalid_client", "only public clients (token_endpoint_auth_method none) are supported")
	}
	if err := validRedirects(doc.RedirectURIs); err != nil {
		return OAuthClient{}, oauthErr("invalid_client", err.Error())
	}
	now := s.now()
	row := sqlstore.OAuthClient{
		ID: id, Name: cleanClientName(doc.ClientName, id), RedirectURIs: doc.RedirectURIs, Source: "cimd",
		CreatedAt: domain.FormatTime(now), ExpiresAt: domain.FormatTime(now.Add(clientDocTTL)),
	}
	if err := s.DB.UpsertOAuthClient(ctx, row); err != nil {
		return OAuthClient{}, err
	}
	return clientFromRow(row), nil
}

// cleanClientName drops control characters and caps the length, since the
// name is shown on the consent page and the client chose it.
func cleanClientName(name, fallback string) string {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	if r := []rune(name); len(r) > 100 {
		name = string(r[:100])
	}
	if name == "" {
		return fallback
	}
	return name
}

// FetchClientDoc downloads a client metadata document. The URL is chosen
// by whoever starts an authorization, so the fetch must not reach our own
// network: the dial itself refuses private, loopback and link-local
// addresses (checked on the resolved IP, so DNS tricks don't help), there
// are no redirects and no proxy, and time and size are capped.
func FetchClientDoc(ctx context.Context, rawURL string) ([]byte, error) {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil || BlockedIP(ip) {
				return errors.New("address not allowed")
			}
			return nil
		},
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: nil, DialContext: dialer.DialContext, TLSHandshakeTimeout: 5 * time.Second, DisableKeepAlives: true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, clientDocMaxBytes+1))
	if err != nil {
		return nil, errors.New("read failed")
	}
	if len(body) > clientDocMaxBytes {
		return nil, errors.New("document is larger than 64 KB")
	}
	return body, nil
}

var extraBlocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
}

// BlockedIP reports addresses a client metadata fetch must never reach.
func BlockedIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, p := range extraBlocked {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func loopbackHost(h string) bool { return h == "127.0.0.1" || h == "::1" || h == "localhost" }

// ValidClientRedirect is the rule for a registered client's redirect URI:
// an https URL, or an http loopback URL (any port, or none) for a local app.
func ValidClientRedirect(raw string) error {
	u, err := url.Parse(raw)
	bad := errors.New("redirect_uris must be https URLs, or http://127.0.0.1, http://[::1] or http://localhost, with no credentials or fragment")
	if err != nil || len(raw) > 512 || u.User != nil || u.Fragment != "" || u.Hostname() == "" {
		return bad
	}
	switch {
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && loopbackHost(u.Hostname()):
		return nil
	}
	return bad
}

func validRedirects(uris []string) error {
	if len(uris) == 0 || len(uris) > maxRedirectURIs {
		return fmt.Errorf("redirect_uris needs between 1 and %d entries", maxRedirectURIs)
	}
	for _, u := range uris {
		if err := ValidClientRedirect(u); err != nil {
			return err
		}
	}
	return nil
}

// redirectMatches is exact, except that a loopback redirect ignores the port
// (RFC 8252 section 7.3): a local app picks one at run time.
func redirectMatches(registered []string, got string) bool {
	g, err := url.Parse(got)
	if err != nil {
		return false
	}
	for _, r := range registered {
		if r == got {
			return true
		}
		ru, err := url.Parse(r)
		if err == nil && ru.Scheme == "http" && g.Scheme == "http" && loopbackHost(ru.Hostname()) &&
			ru.Hostname() == g.Hostname() && ru.Path == g.Path && ru.RawQuery == g.RawQuery {
			return true
		}
	}
	return false
}

// ClientRegistration is an RFC 7591 request. Unknown members are ignored.
type ClientRegistration struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
}

// ClientInfo is the RFC 7591 response.
type ClientInfo struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
}

// RegisterOAuthClient is open dynamic client registration. Anyone can
// register, so a client gets nothing but the right to ask: the consent page
// still shows the human what is being requested, and OAuth never grants approve.
func (s *Service) RegisterOAuthClient(ctx context.Context, in ClientRegistration) (ClientInfo, error) {
	bad := func(code, msg string) (ClientInfo, error) { return ClientInfo{}, oauthErr(code, msg) }
	if err := validRedirects(in.RedirectURIs); err != nil {
		return bad("invalid_redirect_uri", err.Error())
	}
	if m := in.TokenEndpointAuthMethod; m != "" && m != "none" {
		return bad("invalid_client_metadata", "only public clients (token_endpoint_auth_method none) are supported")
	}
	for _, r := range in.ResponseTypes {
		if r != "code" {
			return bad("invalid_client_metadata", "response_types must be code")
		}
	}
	for _, g := range in.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			return bad("invalid_client_metadata", "grant_types may be authorization_code and refresh_token")
		}
	}
	now := s.now()
	_ = s.DB.DeleteUnusedOAuthClients(ctx, "dcr", domain.FormatTime(now.Add(-dcrUnusedTTL)))
	n, err := s.DB.CountOAuthClients(ctx, "dcr")
	if err != nil {
		return ClientInfo{}, err
	}
	if n >= maxDCRClients {
		return bad("temporarily_unavailable", "too many registered clients; try again later")
	}
	secret, err := auth.RandomURLToken(16)
	if err != nil {
		return ClientInfo{}, err
	}
	row := sqlstore.OAuthClient{
		ID: "pc_" + secret, Name: cleanClientName(in.ClientName, "Unnamed app"), RedirectURIs: in.RedirectURIs,
		Source: "dcr", CreatedAt: domain.FormatTime(now),
	}
	if err := s.DB.UpsertOAuthClient(ctx, row); err != nil {
		return ClientInfo{}, err
	}
	return ClientInfo{
		ClientID: row.ID, ClientName: row.Name, RedirectURIs: row.RedirectURIs, TokenEndpointAuthMethod: "none",
		GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"}, ClientIDIssuedAt: now.Unix(),
	}, nil
}
