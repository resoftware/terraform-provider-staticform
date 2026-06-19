package client

import (
	"context"
	"fmt"
	"net/http"
)

// Subscription is the account's active subscription (may be empty if none).
type Subscription struct {
	ID                    string  `json:"id"`
	Tier                  string  `json:"tier"`
	TierName              string  `json:"tierName"`
	PriceInCents          int64   `json:"priceInCents"`
	BillingInterval       string  `json:"billingInterval"`
	Status                string  `json:"status"`
	CurrentPeriodStartUTC *string `json:"currentPeriodStartUtc"`
	CurrentPeriodEndUTC   *string `json:"currentPeriodEndUtc"`
	CancelAtPeriodEnd     bool    `json:"cancelAtPeriodEnd"`
	CancelledAtUTC        *string `json:"cancelledAtUtc"`
}

// GetSubscription returns the current subscription, or nil if there is none.
func (c *Client) GetSubscription(ctx context.Context) (*Subscription, error) {
	var out *Subscription
	if err := c.do(ctx, http.MethodGet, "/api/v1/subscriptions/my", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UserProfile is the authenticated user's profile and plan capabilities.
type UserProfile struct {
	ID                             string   `json:"id"`
	Email                          string   `json:"email"`
	IsAdmin                        bool     `json:"isAdmin"`
	CanUseGoogleSheets             bool     `json:"canUseGoogleSheets"`
	CanUseNotion                   bool     `json:"canUseNotion"`
	CanUseEmailAttachments         bool     `json:"canUseEmailAttachments"`
	CanUseConditionalSubmitActions bool     `json:"canUseConditionalSubmitActions"`
	CanUsePaymentForms             bool     `json:"canUsePaymentForms"`
	Permissions                    []string `json:"permissions"`
}

// GetProfile returns the authenticated user's profile.
func (c *Client) GetProfile(ctx context.Context) (*UserProfile, error) {
	var out UserProfile
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ExecutionLog is one submit-action execution record.
type ExecutionLog struct {
	ID             string  `json:"id"`
	SubmissionID   string  `json:"submissionId"`
	SubmitActionID string  `json:"submitActionId"`
	FormName       string  `json:"formName"`
	ActionType     string  `json:"actionType"`
	Status         string  `json:"status"`
	StartedAtUTC   string  `json:"startedAtUtc"`
	CompletedAtUTC *string `json:"completedAtUtc"`
	DurationMs     int64   `json:"durationMs"`
	AttemptNumber  int64   `json:"attemptNumber"`
	IsManualRetry  bool    `json:"isManualRetry"`
	HTTPStatusCode *int64  `json:"httpStatusCode"`
	ErrorMessage   *string `json:"errorMessage"`
}

// ListExecutionLogs returns submit-action execution logs, optionally filtered by
// form ID, following pagination.
func (c *Client) ListExecutionLogs(ctx context.Context, formID string) ([]ExecutionLog, error) {
	var all []ExecutionLog
	page := 1
	for {
		var resp struct {
			Logs       []ExecutionLog `json:"logs"`
			TotalCount int            `json:"totalCount"`
		}
		path := fmt.Sprintf("/api/v1/execution-logs?page=%d&pageSize=100", page)
		if formID != "" {
			path += "&formId=" + formID
		}
		if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Logs...)
		if len(resp.Logs) == 0 || len(all) >= resp.TotalCount {
			break
		}
		page++
	}
	return all, nil
}
