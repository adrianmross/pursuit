// Package apiclient is the HTTP client used by the pursuit CLI.
package apiclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pursuit/pursuit/internal/version"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

type Response struct {
	Status      int
	Body        []byte
	ContentType string
	Header      http.Header
}

type Options struct {
	IdempotencyKey string
	DryRun         bool
}

func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body []byte, contentType string, opt Options) (Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = "http://127.0.0.1:8787"
	}
	u, err := url.Parse(base + path)
	if err != nil {
		return Response{}, unavailable(err)
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return Response{}, unavailable(err)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("User-Agent", "pursuit/"+version.Version)
	req.Header.Set("Accept", "application/json, text/csv;q=0.9")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if opt.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", opt.IdempotencyKey)
	}
	if opt.DryRun {
		req.Header.Set("X-Dry-Run", "true")
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return Response{}, unavailable(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return Response{}, unavailable(err)
	}
	return Response{Status: resp.StatusCode, Body: b, ContentType: resp.Header.Get("Content-Type"), Header: resp.Header}, nil
}

func unavailable(err error) error {
	return fmt.Errorf("unavailable: %s; check that pursuit serve is running and PURSUIT_SERVER is correct", err.Error())
}
