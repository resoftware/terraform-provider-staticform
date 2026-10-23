// Package client is a thin HTTP client for the StaticForm REST API
// (https://api.staticform.app/api/v1). It covers the subset of endpoints the
// Terraform provider manages: forms, API keys, integrations, form
// collaborators, account settings and the read-only data sources.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the production StaticForm API host.
const DefaultBaseURL = "https://api.staticform.app"

// Client talks to the StaticForm API using an X-API-Key credential.
type Client struct {
	baseURL        string
	apiKey         string
	httpClient     *http.Client
	userAgent      string
	keyPassword    string
	keyReauthToken string
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

func WithKeyCreationProof(password, reauthToken string) Option {
	return func(c *Client) { c.keyPassword, c.keyReauthToken = password, reauthToken }
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

// APIError represents a non-2xx response. The API answers with RFC 7807
// ProblemDetails: Title and Detail describe the failure, ErrorCode is the
// stable machine-readable code, and Errors maps request fields to validation
// error codes.
type APIError struct {
	StatusCode int
	Title      string
	Detail     string
	ErrorCode  string
	Errors     map[string][]string
	Raw        string
}

const missingPermissionPrefix = "Missing permission: "

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "staticform API error (HTTP %d)", e.StatusCode)
	if e.Title != "" {
		fmt.Fprintf(&b, ": %s", e.Title)
	}
	if e.Detail != "" {
		fmt.Fprintf(&b, " - %s", e.Detail)
	}
	if e.ErrorCode != "" {
		fmt.Fprintf(&b, " [%s]", e.ErrorCode)
	}
	fields := make([]string, 0, len(e.Errors))
	for field := range e.Errors {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		fmt.Fprintf(&b, "\n  %s: %s", field, strings.Join(e.Errors[field], ", "))
	}
	if e.Title == "" && e.Detail == "" && e.ErrorCode == "" && len(e.Errors) == 0 && e.Raw != "" {
		fmt.Fprintf(&b, ": %s", e.Raw)
	}
	if permission := e.MissingPermission(); permission != "" {
		fmt.Fprintf(&b, "\nThe API key used by the provider does not hold the permission %s. Grant it to that key and retry.", permission)
	} else if e.ErrorCode == "ApiKeyScopeExceeded" {
		b.WriteString("\nAn API key can only create or change keys whose permissions and form scopes it holds itself. Give the provider's key those permissions and form scopes, or set explicit permissions on the managed key.")
	}
	return b.String()
}

// MissingPermission returns the permission name a 403 response reports as
// missing, or an empty string when the response does not name one.
func (e *APIError) MissingPermission() string {
	if e.StatusCode != http.StatusForbidden || (e.ErrorCode != "" && e.ErrorCode != "Forbidden") {
		return ""
	}
	name, found := strings.CutPrefix(e.Detail, missingPermissionPrefix)
	if !found || !isPermissionName(name) {
		return ""
	}
	return name
}

func isPermissionName(name string) bool {
	segments := strings.Split(name, ".")
	if len(segments) != 3 {
		return false
	}
	for _, segment := range segments {
		if segment == "" || segment[0] < 'A' || segment[0] > 'Z' {
			return false
		}
		for _, r := range segment {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
				return false
			}
		}
	}
	return true
}

// IsNotFound reports whether err is an APIError with a 404 status.
func IsNotFound(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.StatusCode == http.StatusNotFound
}

// do performs an HTTP request. body is JSON-encoded when non-nil; out is
// JSON-decoded from the response when non-nil. A 404 yields an APIError that
// IsNotFound recognises.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	_, err := c.doHeader(ctx, method, path, body, out)
	return err
}

// doHeader is do, additionally returning the response headers.
func (c *Client) doHeader(ctx context.Context, method, path string, body, out any) (http.Header, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json, application/problem+json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, raw)
	}

	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}
	}
	return resp.Header, nil
}

func parseAPIError(status int, raw []byte) *APIError {
	ae := &APIError{StatusCode: status}
	var pd struct {
		Title     string              `json:"title"`
		Detail    string              `json:"detail"`
		ErrorCode string              `json:"errorCode"`
		Errors    map[string][]string `json:"errors"`
	}
	if err := json.Unmarshal(raw, &pd); err == nil {
		ae.Title, ae.Detail, ae.ErrorCode, ae.Errors = pd.Title, pd.Detail, pd.ErrorCode, pd.Errors
	}
	if ae.Title == "" && ae.Detail == "" && ae.ErrorCode == "" && len(ae.Errors) == 0 {
		ae.Raw = strings.TrimSpace(string(raw))
		if len(ae.Raw) > maxRawErrorBody {
			ae.Raw = ae.Raw[:maxRawErrorBody] + "..."
		}
	}
	return ae
}

const maxRawErrorBody = 512

// listPageSize is the page size requested from paged list endpoints, which
// accept at most 100 items per page.
const listPageSize = 100

// unknownTotal marks a page whose response did not report the full item count.
const unknownTotal = -1

// pagedPath appends the page and page-size query parameters to path.
func pagedPath(path, pageParam string, page int) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%s%s=%d&pageSize=%d", path, separator, pageParam, page, listPageSize)
}

// getHeaderCountedPage fetches one page of a list whose total item count is
// reported in the X-Total-Count header and whose items are selected from the
// decoded response body.
func getHeaderCountedPage[R any, T any](ctx context.Context, c *Client, path string, page int, items func(R) []T) ([]T, int, error) {
	var resp R
	header, err := c.doHeader(ctx, http.MethodGet, pagedPath(path, "page", page), nil, &resp)
	if err != nil {
		return nil, 0, err
	}
	return items(resp), totalCountHeader(header), nil
}

// totalCountHeader reads the X-Total-Count header of a paged list response.
func totalCountHeader(h http.Header) int {
	total, err := strconv.Atoi(h.Get("X-Total-Count"))
	if err != nil || total < 0 {
		return unknownTotal
	}
	return total
}

// listPages reads every page of a paged list. fetch returns the items of one
// page (1-based) and the total number of items across all pages, or
// unknownTotal when the response does not report it.
func listPages[T any](fetch func(page int) (items []T, total int, err error)) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		items, total, err := fetch(page)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		more := len(items) >= listPageSize
		if total != unknownTotal {
			more = len(all) < total
		}
		if !more {
			return all, nil
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("pagination returned an empty page before the reported total of %d items", total)
		}
	}
}
