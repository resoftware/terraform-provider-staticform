package client

import (
	"context"
	"net/http"
)

// UserSettings is the authenticated user's account-level settings.
// SubmissionOverviewEmailFrequency is one of: Off, Daily, Weekly, Monthly.
type UserSettings struct {
	SubmissionOverviewEmailFrequency string `json:"submissionOverviewEmailFrequency"`
}

// GetUserSettings fetches the current account settings.
func (c *Client) GetUserSettings(ctx context.Context) (*UserSettings, error) {
	var out UserSettings
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/me/settings", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateUserSettings updates the account settings.
func (c *Client) UpdateUserSettings(ctx context.Context, s UserSettings) (*UserSettings, error) {
	var out UserSettings
	if err := c.do(ctx, http.MethodPut, "/api/v1/users/me/settings", s, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
