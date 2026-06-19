package client

import (
	"context"
	"net/http"
)

// ── Notion connections (non-interactive: integration token) ─────────────────

// NotionConnection is a Notion integration connection.
type NotionConnection struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Revoked      bool   `json:"revoked"`
	CreatedAtUTC string `json:"createdAtUtc"`
}

// CreateNotionConnection creates a Notion connection from an integration token
// (the server validates the token against Notion).
func (c *Client) CreateNotionConnection(ctx context.Context, label, apiKey string) (*NotionConnection, error) {
	body := struct {
		Label  string `json:"label"`
		APIKey string `json:"apiKey"`
	}{label, apiKey}
	var out NotionConnection
	if err := c.do(ctx, http.MethodPost, "/api/v1/integrations/notion/connections", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListNotionConnections returns all Notion connections.
func (c *Client) ListNotionConnections(ctx context.Context) ([]NotionConnection, error) {
	var env struct {
		Connections []NotionConnection `json:"connections"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/integrations/notion/connections", nil, &env); err != nil {
		return nil, err
	}
	return env.Connections, nil
}

// GetNotionConnection finds a Notion connection by ID via the list endpoint.
func (c *Client) GetNotionConnection(ctx context.Context, id string) (*NotionConnection, error) {
	conns, err := c.ListNotionConnections(ctx)
	if err != nil {
		return nil, err
	}
	for i := range conns {
		if conns[i].ID == id {
			return &conns[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Title: "Notion connection not found"}
}

// DeleteNotionConnection deletes a Notion connection.
func (c *Client) DeleteNotionConnection(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/integrations/notion/connections/"+id, nil, nil)
}

// ── Stripe connections (non-interactive: restricted secret key) ─────────────

// StripeConnection is a Stripe payment connection.
type StripeConnection struct {
	ID                string `json:"id"`
	Label             string `json:"label"`
	KeyMode           string `json:"keyMode"`
	WebhookConfigured bool   `json:"webhookConfigured"`
	Revoked           bool   `json:"revoked"`
	CreatedAtUTC      string `json:"createdAtUtc"`
}

// CreateStripeConnection creates a Stripe connection from a restricted secret
// key (`rk_...`); the server verifies it against Stripe. Returns id/label/keyMode.
func (c *Client) CreateStripeConnection(ctx context.Context, label, secretKey string) (*StripeConnection, error) {
	body := struct {
		Label     string `json:"label"`
		SecretKey string `json:"secretKey"`
	}{label, secretKey}
	var out StripeConnection
	if err := c.do(ctx, http.MethodPost, "/api/v1/integrations/stripe/connections", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetStripeWebhookSecret sets the webhook signing secret (`whsec_...`).
func (c *Client) SetStripeWebhookSecret(ctx context.Context, id, secret string) error {
	body := struct {
		WebhookSecret string `json:"webhookSecret"`
	}{secret}
	return c.do(ctx, http.MethodPatch, "/api/v1/integrations/stripe/connections/"+id+"/webhook", body, nil)
}

// ListStripeConnections returns all Stripe connections.
func (c *Client) ListStripeConnections(ctx context.Context) ([]StripeConnection, error) {
	var env struct {
		Connections []StripeConnection `json:"connections"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/integrations/stripe/connections", nil, &env); err != nil {
		return nil, err
	}
	return env.Connections, nil
}

// GetStripeConnection finds a Stripe connection by ID via the list endpoint.
func (c *Client) GetStripeConnection(ctx context.Context, id string) (*StripeConnection, error) {
	conns, err := c.ListStripeConnections(ctx)
	if err != nil {
		return nil, err
	}
	for i := range conns {
		if conns[i].ID == id {
			return &conns[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Title: "Stripe connection not found"}
}

// DeleteStripeConnection deletes a Stripe connection.
func (c *Client) DeleteStripeConnection(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/integrations/stripe/connections/"+id, nil, nil)
}

// ── Google connections (OAuth: read-only here) ──────────────────────────────

// GoogleConnection is a Google OAuth connection (created interactively).
type GoogleConnection struct {
	ID                 string `json:"id"`
	GoogleAccountEmail string `json:"googleAccountEmail"`
	Revoked            bool   `json:"revoked"`
	CreatedAtUTC       string `json:"createdAtUtc"`
}

// ListGoogleConnections returns all Google connections.
func (c *Client) ListGoogleConnections(ctx context.Context) ([]GoogleConnection, error) {
	var env struct {
		Connections []GoogleConnection `json:"connections"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/integrations/google/connections", nil, &env); err != nil {
		return nil, err
	}
	return env.Connections, nil
}
