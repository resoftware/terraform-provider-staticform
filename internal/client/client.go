// Package client is a thin HTTP client for the StaticForm REST API
// (https://api.staticform.app/api/v1). It covers the subset of endpoints the
// Terraform provider manages: forms, API keys, SMTP connections, email domains,
// and form collaborators.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the production StaticForm API host.
const DefaultBaseURL = "https://api.staticform.app"

// Client talks to the StaticForm API using an X-API-Key credential.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	userAgent  string
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the default HTTP client (useful in tests).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// WithUserAgent sets the User-Agent header sent on every request.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// New builds a Client. baseURL may be empty to use DefaultBaseURL.
func New(baseURL, apiKey string, opts ...Option) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "terraform-provider-staticform",
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// APIError represents a non-2xx response. It decodes RFC 7807 ProblemDetails
// bodies when present so validation messages surface in Terraform diagnostics.
type APIError struct {
	StatusCode int
	Title      string
	Detail     string
	Errors     map[string][]string
	Raw        string
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "staticform API error (HTTP %d)", e.StatusCode)
	if e.Title != "" {
		fmt.Fprintf(&b, ": %s", e.Title)
	}
	if e.Detail != "" {
		fmt.Fprintf(&b, " - %s", e.Detail)
	}
	for field, msgs := range e.Errors {
		fmt.Fprintf(&b, "\n  %s: %s", field, strings.Join(msgs, "; "))
	}
	if e.Title == "" && e.Detail == "" && len(e.Errors) == 0 && e.Raw != "" {
		fmt.Fprintf(&b, ": %s", e.Raw)
	}
	return b.String()
}

// IsNotFound reports whether err is an APIError with a 404 status.
func IsNotFound(err error) bool {
	ae, ok := err.(*APIError)
	return ok && ae.StatusCode == http.StatusNotFound
}

// do performs an HTTP request. body is JSON-encoded when non-nil; out is
// JSON-decoded from the response when non-nil. A 404 yields an APIError that
// IsNotFound recognises.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, raw)
	}

	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

func parseAPIError(status int, raw []byte) *APIError {
	ae := &APIError{StatusCode: status, Raw: string(raw)}
	var pd struct {
		Title  string              `json:"title"`
		Detail string              `json:"detail"`
		Errors map[string][]string `json:"errors"`
	}
	if err := json.Unmarshal(raw, &pd); err == nil {
		ae.Title = pd.Title
		ae.Detail = pd.Detail
		ae.Errors = pd.Errors
	}
	return ae
}
