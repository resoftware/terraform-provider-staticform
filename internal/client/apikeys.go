package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// APIKeyRequest is the create body for an API key.
type APIKeyRequest struct {
	Name         string   `json:"name"`
	ExpiresAtUTC *string  `json:"expiresAtUtc,omitempty"`
	Permissions  []string `json:"scopedPermissions,omitempty"`
	FormIDs      []string `json:"formIds,omitempty"`
	Password     string   `json:"password,omitempty"`
	ReauthToken  string   `json:"reauthToken,omitempty"`
}

// APIKeyUpdateRequest is the update body for an API key. The API requires the
// full permission and form-scope sets on every update; expiration is immutable.
type APIKeyUpdateRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"scopedPermissions"`
	FormIDs     []string `json:"formIds"`
}

// APIKey is the API key as returned on create (the only time the plaintext key
// is available).
type APIKey struct {
	APIKey       string   `json:"plainKey"`
	ID           string   `json:"apiKeyId"`
	KeyHint      string   `json:"keyHint"`
	Name         string   `json:"name"`
	ExpiresAtUTC *string  `json:"expiresAtUtc"`
	CreatedAtUTC string   `json:"createdAtUtc"`
	Permissions  []string `json:"scopedPermissions"`
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
	Permissions   []string `json:"scopedPermissions"`
	FormIDs       []string `json:"formIds"`
}

// CreateAPIKey creates an API key and returns the plaintext key once.
func (c *Client) CreateAPIKey(ctx context.Context, req APIKeyRequest) (*APIKey, error) {
	if req.Password == "" {
		req.Password = c.keyPassword
	}
	if req.ReauthToken == "" {
		req.ReauthToken = c.keyReauthToken
	}
	if req.Password == "" && req.ReauthToken == "" {
		return nil, fmt.Errorf("API-key creation requires key_creation_password or key_creation_reauth_token (or STATICFORM_KEY_CREATION_PASSWORD / STATICFORM_KEY_CREATION_REAUTH_TOKEN)")
	}
	req.Permissions = NativeAPIKeyPermissions(req.Permissions)
	var out APIKey
	if err := c.do(ctx, http.MethodPost, nativeAPIKeysPath, req, &out); err != nil {
		return nil, err
	}
	if out.ID == "" || out.APIKey == "" {
		return nil, fmt.Errorf("native API-key creation did not return apiKeyId and plainKey")
	}
	out.Permissions = req.Permissions
	metadata, err := c.GetAPIKey(ctx, out.ID)
	if err != nil {
		return &out, fmt.Errorf("API key was created but metadata refresh failed: %w", err)
	}
	out.Name, out.KeyHint, out.CreatedAtUTC = metadata.Name, metadata.KeyHint, metadata.CreatedAtUTC
	out.ExpiresAtUTC, out.Permissions, out.FormIDs = metadata.ExpiresAtUTC, metadata.Permissions, metadata.FormIDs
	return &out, nil
}

const nativeAPIKeysPath = "/identity/v1/api-keys"

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
	keys := []APIKeyListItem{}
	for page := 1; ; page++ {
		var result struct {
			Items       []APIKeyListItem `json:"items"`
			HasNextPage bool             `json:"hasNextPage"`
		}
		endpoint := fmt.Sprintf("%s?page=%d&pageSize=100", nativeAPIKeysPath, page)
		if err := c.do(ctx, http.MethodGet, endpoint, nil, &result); err != nil {
			return nil, err
		}
		if result.Items == nil {
			return nil, fmt.Errorf("native API-key list did not return the items envelope")
		}
		keys = append(keys, result.Items...)
		if !result.HasNextPage {
			return keys, nil
		}
		if len(result.Items) == 0 {
			return nil, fmt.Errorf("native API-key pagination returned an empty page with hasNextPage")
		}
	}
}

// UpdateAPIKey replaces scoped permissions and form scopes and optionally renames a key.
func (c *Client) UpdateAPIKey(ctx context.Context, id string, req APIKeyUpdateRequest) error {
	req.Permissions = NativeAPIKeyPermissions(req.Permissions)
	return c.do(ctx, http.MethodPut, nativeAPIKeysPath+"/"+url.PathEscape(id)+"/permissions", req, nil)
}

// RevokeAPIKey deletes an API key.
func (c *Client) RevokeAPIKey(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, nativeAPIKeysPath+"/"+url.PathEscape(id), nil, nil)
}

// ListAPIKeyPermissions returns the permission names that may be granted to keys.
func (c *Client) ListAPIKeyPermissions(ctx context.Context) ([]string, error) {
	var catalog []struct {
		Name string `json:"name"`
	}
	if err := c.do(ctx, http.MethodGet, "/identity/v1/users/me/permissions", nil, &catalog); err != nil {
		return nil, err
	}
	out := []string{}
	for _, permission := range catalog {
		if strings.HasPrefix(permission.Name, "User.") {
			out = append(out, permission.Name)
		}
	}
	return out, nil
}

func (c *Client) RotateAPIKey(ctx context.Context, id string) (*APIKey, error) {
	var out APIKey
	if err := c.do(ctx, http.MethodPost, nativeAPIKeysPath+"/"+url.PathEscape(id)+"/rotate", nil, &out); err != nil {
		return nil, err
	}
	out.ID = id
	return &out, nil
}
