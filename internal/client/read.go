package client

import (
	"context"
	"net/http"
	"net/url"
)

// ActiveSubscription is the account's current Stripe subscription.
type ActiveSubscription struct {
	SubscriptionID     string  `json:"subscriptionId"`
	StripeProductID    string  `json:"stripeProductId"`
	StripePriceID      string  `json:"stripePriceId"`
	Status             string  `json:"status"`
	Interval           string  `json:"interval"`
	UnitPriceAmount    float64 `json:"unitPriceAmount"`
	UnitPriceCurrency  string  `json:"unitPriceCurrency"`
	CurrentPeriodStart string  `json:"currentPeriodStart"`
	CurrentPeriodEnd   string  `json:"currentPeriodEnd"`
	TrialEndUTC        *string `json:"trialEndUtc"`
	CancelAtPeriodEnd  bool    `json:"cancelAtPeriodEnd"`
	CanceledAtUTC      *string `json:"canceledAtUtc"`
}

// Billing is the account's billing state: Stripe customer, active subscription
// (nil when there is none) and the entitlement keys it grants.
type Billing struct {
	HasBillingAccount  bool                `json:"hasBillingAccount"`
	StripeCustomerID   *string             `json:"stripeCustomerId"`
	ActiveSubscription *ActiveSubscription `json:"activeSubscription"`
	Entitlements       []string            `json:"entitlements"`
}

// GetBilling returns the account's billing state.
func (c *Client) GetBilling(ctx context.Context) (*Billing, error) {
	var out Billing
	if err := c.do(ctx, http.MethodGet, "/api/v1/billing/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Entitlement keys that gate form features.
const (
	EntitlementGoogleSheets       = "forms.google-sheets"
	EntitlementNotion             = "forms.notion"
	EntitlementEmailAttachments   = "forms.email-attachments"
	EntitlementConditionalActions = "forms.conditional-actions"
	EntitlementPaymentForms       = "forms.payments"
)

// Entitlements lists the feature keys granted to the account.
type Entitlements struct {
	FeatureKeys []string `json:"featureKeys"`
}

// Has reports whether the account holds the feature key.
func (e Entitlements) Has(key string) bool {
	for _, k := range e.FeatureKeys {
		if k == key {
			return true
		}
	}
	return false
}

// GetEntitlements returns the feature keys granted to the account.
func (c *Client) GetEntitlements(ctx context.Context) (*Entitlements, error) {
	var out Entitlements
	if err := c.do(ctx, http.MethodGet, "/api/v1/billing/entitlements", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UserProfile is the authenticated user's account details.
type UserProfile struct {
	ID           string   `json:"id"`
	Email        string   `json:"email"`
	IsSuperAdmin bool     `json:"isSuperAdmin"`
	Permissions  []string `json:"permissions"`
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
	path := "/api/v1/execution-logs"
	if formID != "" {
		path += "?formId=" + url.QueryEscape(formID)
	}
	return listPages(func(page int) ([]ExecutionLog, int, error) {
		var resp struct {
			Logs       []ExecutionLog `json:"logs"`
			TotalCount int            `json:"totalCount"`
		}
		if err := c.do(ctx, http.MethodGet, pagedPath(path, "page", page), nil, &resp); err != nil {
			return nil, 0, err
		}
		return resp.Logs, resp.TotalCount, nil
	})
}
