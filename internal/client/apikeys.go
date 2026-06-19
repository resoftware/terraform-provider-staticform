package client

import (
	"context"
	"net/http"
)

// APIKeyRequest is the create body for an API key.
type APIKeyRequest struct {
	Name         string   `json:"name"`
	ExpiresAtUTC *string  `json:"expiresAtUtc,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	FormIDs      []string `json:"formIds,omitempty"`
}

// APIKeyUpdateRequest is the update body for an API key. The API requires the
// full permission and form-scope sets on every update; expiration is immutable.
type APIKeyUpdateRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	FormIDs     []string `json:"formIds"`
}

// APIKey is the API key as returned on create (the only time the plaintext key
// is available).
type APIKey struct {
	APIKey       string   `json:"apiKey"`
	ID           string   `json:"id"`
	KeyHint      string   `json:"keyHint"`
	Name         string   `json:"name"`
	ExpiresAtUTC *string  `json:"expiresAtUtc"`
	CreatedAtUTC string   `json:"createdAtUtc"`
	Permissions  []string `json:"permissions"`
	FormIDs      []string `json:"formIds"`
}

// APIKeyListItem is an entry from the API key list (no plaintext key).
type APIKeyListItem struct {
	ID            string   `json:"id"`
	KeyHint       string   `json:"keyHint"`
	Name          string   `json:"name"`
	IsActive      bool     `json:"isActive"`
	ExpiresAtUTC  *string  `json:"expiresAtUtc"`
	CreatedAtUTC  string   `json:"createdAtUtc"`
	LastUsedAtUTC *string  `json:"lastUsedAtUtc"`
	Permissions   []string `json:"permissions"`
	FormIDs       []string `json:"formIds"`
}

// CreateAPIKey creates an API key and returns the plaintext key once.
func (c *Client) CreateAPIKey(ctx context.Context, req APIKeyRequest) (*APIKey, error) {
	var out APIKey
	if err := c.do(ctx, http.MethodPost, "/api/v1/users/me/api-keys", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAPIKey looks up a single API key by ID from the list endpoint (the API has
// no get-by-id for keys, and never re-exposes the plaintext key).
func (c *Client) GetAPIKey(ctx context.Context, id string) (*APIKeyListItem, error) {
	keys, err := c.ListAPIKeys(ctx)
	if err != nil {
		return nil, err
	}
	for i := range keys {
		if keys[i].ID == id {
			return &keys[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Title: "API key not found"}
}

// ListAPIKeys returns all API keys for the authenticated user.
func (c *Client) ListAPIKeys(ctx context.Context) ([]APIKeyListItem, error) {
	var env struct {
		APIKeys []APIKeyListItem `json:"apiKeys"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/me/api-keys", nil, &env); err != nil {
		return nil, err
	}
	return env.APIKeys, nil
}

// UpdateAPIKey updates an API key's name and/or expiration.
func (c *Client) UpdateAPIKey(ctx context.Context, id string, req APIKeyUpdateRequest) error {
	return c.do(ctx, http.MethodPatch, "/api/v1/users/me/api-keys/"+id, req, nil)
}

// RevokeAPIKey deletes an API key.
func (c *Client) RevokeAPIKey(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/users/me/api-keys/"+id, nil, nil)
}

// ListAPIKeyPermissions returns the permission names that may be granted to keys.
func (c *Client) ListAPIKeyPermissions(ctx context.Context) ([]string, error) {
	var out []string
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/me/api-keys/permissions", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
