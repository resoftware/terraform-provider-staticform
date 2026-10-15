package client

import (
	"context"
	"net/http"
)

// QuietHours is the daily window in which immediate email is held. Start and
// End are wall-clock times in the preferences' time zone, formatted HH:mm:ss.
type QuietHours struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// NotificationEventPreference is one catalog event merged with the caller's
// choice. InApp and Email are the values the user chose, not the effective
// delivery outcome, which also depends on the master email switch.
type NotificationEventPreference struct {
	Key            string `json:"key"`
	Category       string `json:"category"`
	OptOutAllowed  bool   `json:"optOutAllowed"`
	EmailByDefault bool   `json:"emailByDefault"`
	InApp          bool   `json:"inApp"`
	Email          bool   `json:"email"`
	SampleTitle    string `json:"sampleTitle"`
}

// NotificationPreferences are the signed-in user's notification preferences:
// the global email switches and the full event catalog with their choices.
// EmailMode is Immediate or DailyDigest.
type NotificationPreferences struct {
	EmailEnabled    bool                          `json:"emailEnabled"`
	EmailMode       string                        `json:"emailMode"`
	DigestHourLocal int                           `json:"digestHourLocal"`
	QuietHours      *QuietHours                   `json:"quietHours"`
	TimeZoneID      string                        `json:"timeZoneId"`
	Events          []NotificationEventPreference `json:"events"`
}

// NotificationEventChoice is one entry of an update request.
type NotificationEventChoice struct {
	Key   string `json:"key"`
	InApp bool   `json:"inApp"`
	Email bool   `json:"email"`
}

// NotificationPreferencesRequest replaces the global switches and every
// per-event override. Events not listed fall back to their catalog defaults.
type NotificationPreferencesRequest struct {
	EmailEnabled    bool                      `json:"emailEnabled"`
	EmailMode       string                    `json:"emailMode"`
	DigestHourLocal int                       `json:"digestHourLocal"`
	QuietHours      *QuietHours               `json:"quietHours"`
	TimeZoneID      string                    `json:"timeZoneId"`
	Events          []NotificationEventChoice `json:"events"`
}

// GetNotificationPreferences fetches the signed-in user's notification preferences.
func (c *Client) GetNotificationPreferences(ctx context.Context) (*NotificationPreferences, error) {
	var out NotificationPreferences
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/me/notification-preferences", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateNotificationPreferences replaces the signed-in user's notification preferences.
func (c *Client) UpdateNotificationPreferences(ctx context.Context, req NotificationPreferencesRequest) (*NotificationPreferences, error) {
	if req.Events == nil {
		req.Events = []NotificationEventChoice{}
	}
	var out NotificationPreferences
	if err := c.do(ctx, http.MethodPut, "/api/v1/users/me/notification-preferences", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Request converts the preferences into an update request that reproduces every
// event choice as currently reported.
func (p *NotificationPreferences) Request() NotificationPreferencesRequest {
	req := NotificationPreferencesRequest{
		EmailEnabled:    p.EmailEnabled,
		EmailMode:       p.EmailMode,
		DigestHourLocal: p.DigestHourLocal,
		QuietHours:      p.QuietHours,
		TimeZoneID:      p.TimeZoneID,
		Events:          make([]NotificationEventChoice, 0, len(p.Events)),
	}
	for _, e := range p.Events {
		req.Events = append(req.Events, NotificationEventChoice{Key: e.Key, InApp: e.InApp, Email: e.Email})
	}
	return req
}

// Event returns the catalog entry for key.
func (p *NotificationPreferences) Event(key string) (NotificationEventPreference, bool) {
	for _, e := range p.Events {
		if e.Key == key {
			return e, true
		}
	}
	return NotificationEventPreference{}, false
}
