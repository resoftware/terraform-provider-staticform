package client

import (
	"context"
	"fmt"
	"net/http"
)

// Submit-action type discriminators (the API serialises enums and the
// polymorphic "type" property as strings).
const (
	ActionSendEmail        = "SendEmailAction"
	ActionTriggerWebhook   = "TriggerWebhookAction"
	ActionSyncGoogleSheets = "SyncGoogleSheetsAction"
	ActionSyncNotion       = "SyncNotionDatabaseAction"
)

// FieldConfig holds optional per-field validation and UI options. Both maps use
// string values to mirror the API's JSONB config storage.
type FieldConfig struct {
	ValidationConfig map[string]string `json:"validationConfig,omitempty"`
	Options          map[string]string `json:"options,omitempty"`
}

// Field is one form input definition.
type Field struct {
	Name     string       `json:"name"`
	Type     string       `json:"type"`
	Rule     string       `json:"rule"`
	Required bool         `json:"required"`
	Config   *FieldConfig `json:"config,omitempty"`
}

// GoogleSheetsColumnMapping maps a sheet column to a form field.
type GoogleSheetsColumnMapping struct {
	ColumnHeader  string `json:"columnHeader"`
	FormFieldName string `json:"formFieldName"`
}

// NotionPropertyMapping maps a Notion database property to a form field.
type NotionPropertyMapping struct {
	PropertyName  string `json:"propertyName"`
	PropertyType  string `json:"propertyType"`
	FormFieldName string `json:"formFieldName"`
}

// Condition is a single run-condition clause.
type Condition struct {
	FieldName     string `json:"fieldName"`
	Operator      string `json:"operator"`
	Value         string `json:"value,omitempty"`
	CaseSensitive bool   `json:"caseSensitive"`
}

// ConditionSubGroup groups conditions joined by a connector.
type ConditionSubGroup struct {
	Connector  string      `json:"connector"`
	Conditions []Condition `json:"conditions"`
}

// ConditionGroup is the top-level run-condition expression.
type ConditionGroup struct {
	Connector string              `json:"connector"`
	Groups    []ConditionSubGroup `json:"groups"`
}

// SubmitAction is a flat representation of the API's polymorphic submit action.
// The Type field selects which other fields are meaningful; unused fields are
// omitted on the wire and ignored by the server.
type SubmitAction struct {
	Type         string          `json:"type"`
	ID           *string         `json:"id,omitempty"`
	Name         string          `json:"name"`
	Enabled      bool            `json:"enabled"`
	RunCondition *ConditionGroup `json:"runCondition,omitempty"`

	// SendEmailAction
	Recipient           string                      `json:"recipient,omitempty"`
	ReplyTo             *string                     `json:"replyTo,omitempty"`
	Subject             *string                     `json:"subject,omitempty"`
	BodyTemplate        string                      `json:"bodyTemplate,omitempty"`
	BodyBuilderState    *string                     `json:"bodyBuilderState,omitempty"`
	DisableBranding     bool                        `json:"disableBranding,omitempty"`
	EmailDomainID       *string                     `json:"emailDomainId,omitempty"`
	SMTPConnectionID    *string                     `json:"smtpConnectionId,omitempty"`
	FromEmail           *string                     `json:"fromEmail,omitempty"`
	FromName            *string                     `json:"fromName,omitempty"`
	CustomEmailTemplate *EmailTemplateCustomization `json:"customEmailTemplate,omitempty"`
	FieldAttachments    []FieldAttachmentConfig     `json:"fieldAttachments,omitempty"`
	StaticAttachments   []StaticEmailAttachment     `json:"staticAttachments,omitempty"`

	// TriggerWebhookAction
	WebhookURL  string            `json:"webhookUrl,omitempty"`
	HTTPMethod  string            `json:"httpMethod,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`

	// SyncGoogleSheetsAction
	GoogleOAuthConnectionID *string                     `json:"googleOAuthConnectionId,omitempty"`
	SpreadsheetID           string                      `json:"spreadsheetId,omitempty"`
	SheetName               string                      `json:"sheetName,omitempty"`
	ColumnMappings          []GoogleSheetsColumnMapping `json:"columnMappings,omitempty"`
	IncludeSubmissionID     bool                        `json:"includeSubmissionId"`
	IncludeCreatedAt        bool                        `json:"includeCreatedAt"`
	WriteHeaderIfEmpty      bool                        `json:"writeHeaderIfEmpty"`

	// SyncNotionDatabaseAction
	NotionConnectionID *string                 `json:"notionConnectionId,omitempty"`
	DatabaseID         string                  `json:"databaseId,omitempty"`
	DatabaseName       string                  `json:"databaseName,omitempty"`
	PropertyMappings   []NotionPropertyMapping `json:"propertyMappings,omitempty"`
}

// EmailTemplateCustomization styles the StaticForm email wrapper for an email
// action (colors, logo, header/footer text, dark-mode overrides). All fields are
// optional; omitted means "use the default".
type EmailTemplateCustomization struct {
	PrimaryColor              *string `json:"primaryColor,omitempty"`
	HeaderTextColor           *string `json:"headerTextColor,omitempty"`
	FooterBackgroundColor     *string `json:"footerBackgroundColor,omitempty"`
	FooterTextColor           *string `json:"footerTextColor,omitempty"`
	BodyBackgroundColor       *string `json:"bodyBackgroundColor,omitempty"`
	BodyTextColor             *string `json:"bodyTextColor,omitempty"`
	PrimaryColorDark          *string `json:"primaryColorDark,omitempty"`
	HeaderTextColorDark       *string `json:"headerTextColorDark,omitempty"`
	FooterBackgroundColorDark *string `json:"footerBackgroundColorDark,omitempty"`
	FooterTextColorDark       *string `json:"footerTextColorDark,omitempty"`
	BodyBackgroundColorDark   *string `json:"bodyBackgroundColorDark,omitempty"`
	BodyTextColorDark         *string `json:"bodyTextColorDark,omitempty"`
	LogoURL                   *string `json:"logoUrl,omitempty"`
	LogoWidth                 *int64  `json:"logoWidth,omitempty"`
	LogoHeight                *int64  `json:"logoHeight,omitempty"`
	HideLogo                  *bool   `json:"hideLogo,omitempty"`
	HeaderText                *string `json:"headerText,omitempty"`
	HideHeaderText            *bool   `json:"hideHeaderText,omitempty"`
	SubtitleText              *string `json:"subtitleText,omitempty"`
	HideSubtitle              *bool   `json:"hideSubtitle,omitempty"`
	FooterText                *string `json:"footerText,omitempty"`
	SentByText                *string `json:"sentByText,omitempty"`
	HideSentBy                *bool   `json:"hideSentBy,omitempty"`
	ShowFormID                *bool   `json:"showFormId,omitempty"`
}

// FieldAttachmentConfig attaches an uploaded file-field's file to the email,
// with an optional send condition.
type FieldAttachmentConfig struct {
	FieldName string          `json:"fieldName"`
	Condition *ConditionGroup `json:"condition,omitempty"`
}

// StaticEmailAttachment attaches a pre-uploaded file (referenced by its S3 key
// in the form-uploads bucket) to the email, with an optional send condition.
type StaticEmailAttachment struct {
	S3Key       string          `json:"s3Key"`
	FileName    string          `json:"fileName"`
	ContentType string          `json:"contentType"`
	FileSize    int64           `json:"fileSize"`
	Condition   *ConditionGroup `json:"condition,omitempty"`
}

// RedirectConfig is one branch (success or error) of a form's redirect settings.
type RedirectConfig struct {
	Type      string  `json:"type"`
	CustomURL *string `json:"customUrl,omitempty"`
	Message   *string `json:"message,omitempty"`
}

// RedirectSettings controls where the form redirects after submission.
type RedirectSettings struct {
	Success *RedirectConfig `json:"success,omitempty"`
	Error   *RedirectConfig `json:"error,omitempty"`
}

// FixedPaymentRule charges a fixed amount when its condition matches.
type FixedPaymentRule struct {
	Condition   *ConditionGroup `json:"condition,omitempty"`
	AmountCents int64           `json:"amountCents"`
	Description *string         `json:"description,omitempty"`
}

// FieldAmountPaymentRule charges the value of a named field.
type FieldAmountPaymentRule struct {
	Condition       *ConditionGroup `json:"condition,omitempty"`
	AmountFieldName string          `json:"amountFieldName"`
	Description     *string         `json:"description,omitempty"`
}

// PaymentLineItem prices a line from a field's selected option.
type PaymentLineItem struct {
	Condition         *ConditionGroup  `json:"condition,omitempty"`
	FieldName         string           `json:"fieldName"`
	PriceMap          map[string]int64 `json:"priceMap"`
	QuantityFieldName *string          `json:"quantityFieldName,omitempty"`
	Description       *string          `json:"description,omitempty"`
}

// PaymentSettings is a form's Stripe payment configuration.
type PaymentSettings struct {
	Enabled          bool                     `json:"enabled"`
	ConnectionID     *string                  `json:"connectionId,omitempty"`
	Currency         string                   `json:"currency"`
	Mode             string                   `json:"mode"`
	FixedRules       []FixedPaymentRule       `json:"fixedRules,omitempty"`
	FieldAmountRules []FieldAmountPaymentRule `json:"fieldAmountRules,omitempty"`
	LineItems        []PaymentLineItem        `json:"lineItems,omitempty"`
	// Name of the email field whose value prefills Stripe Checkout. Nil = use the first email field.
	CustomerEmailFieldName *string `json:"customerEmailFieldName,omitempty"`
}

// FormRequest is the create/update body for a form.
type FormRequest struct {
	Name                    string            `json:"name"`
	Fields                  []Field           `json:"fields"`
	SubmitActions           []SubmitAction    `json:"submitActions"`
	CaptchaType             *string           `json:"captchaType,omitempty"`
	CaptchaSecretKey        *string           `json:"captchaSecretKey,omitempty"`
	EnableHoneypot          *bool             `json:"enableHoneypot,omitempty"`
	HoneypotFieldName       *string           `json:"honeypotFieldName,omitempty"`
	EnableLanguageDetection *bool             `json:"enableLanguageDetection,omitempty"`
	ExpectedLanguages       []string          `json:"expectedLanguages,omitempty"`
	LanguageDetectionFields []string          `json:"languageDetectionFields,omitempty"`
	RedirectSettings        *RedirectSettings `json:"redirectSettings,omitempty"`
	PaymentSettings         *PaymentSettings  `json:"paymentSettings,omitempty"`
	SubmissionMode          *string           `json:"submissionMode,omitempty"`
}

// Form is the API's representation of a form, as returned by create/get/update.
type Form struct {
	ID                      string            `json:"id"`
	Name                    string            `json:"name"`
	Fields                  []Field           `json:"fields"`
	SubmitActions           []SubmitAction    `json:"submitActions"`
	CaptchaType             string            `json:"captchaType"`
	CaptchaSecretKey        *string           `json:"captchaSecretKey"`
	EnableHoneypot          bool              `json:"enableHoneypot"`
	HoneypotFieldName       string            `json:"honeypotFieldName"`
	EnableLanguageDetection bool              `json:"enableLanguageDetection"`
	ExpectedLanguages       []string          `json:"expectedLanguages"`
	LanguageDetectionFields []string          `json:"languageDetectionFields"`
	RedirectSettings        *RedirectSettings `json:"redirectSettings"`
	PaymentSettings         *PaymentSettings  `json:"paymentSettings"`
	SubmissionMode          string            `json:"submissionMode"`
	ServerSubmissionSecret  *string           `json:"serverSubmissionSecret"`
	Tags                    []string          `json:"tags"`
}

// CreateForm creates a form. The response includes submit actions with their
// server-assigned IDs.
func (c *Client) CreateForm(ctx context.Context, req FormRequest) (*Form, error) {
	var out Form
	if err := c.do(ctx, http.MethodPost, "/api/v1/forms", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetForm fetches a single form by ID.
func (c *Client) GetForm(ctx context.Context, id string) (*Form, error) {
	var out Form
	if err := c.do(ctx, http.MethodGet, "/api/v1/forms/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateForm replaces a form's configuration. Submit actions are merged by ID
// server-side, so callers should send back the IDs they received on create/get.
func (c *Client) UpdateForm(ctx context.Context, id string, req FormRequest) (*Form, error) {
	var out Form
	if err := c.do(ctx, http.MethodPut, "/api/v1/forms/"+id, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteForm deletes a form and all its submissions.
func (c *Client) DeleteForm(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/forms/"+id, nil, nil)
}

// UpdateFormTags replaces a form's tags (max 20). Returns the normalized tags.
func (c *Client) UpdateFormTags(ctx context.Context, id string, tags []string) ([]string, error) {
	if tags == nil {
		tags = []string{}
	}
	body := struct {
		Tags []string `json:"tags"`
	}{Tags: tags}
	var out struct {
		FormID string   `json:"formId"`
		Tags   []string `json:"tags"`
	}
	if err := c.do(ctx, http.MethodPatch, "/api/v1/forms/"+id+"/tags", body, &out); err != nil {
		return nil, err
	}
	return out.Tags, nil
}

// Submission is one entry from a form's submission list.
type Submission struct {
	ID           string             `json:"id"`
	Data         map[string]*string `json:"data"`
	CreatedAtUTC string             `json:"createdAtUtc"`
	IsRead       bool               `json:"isRead"`
	ReadAtUTC    *string            `json:"readAtUtc"`
}

// ListSubmissions returns all (non-spam) submissions for a form, paginated.
func (c *Client) ListSubmissions(ctx context.Context, formID string) ([]Submission, error) {
	var all []Submission
	page := 1
	for {
		var resp struct {
			Items      []Submission `json:"items"`
			TotalCount int          `json:"totalCount"`
		}
		path := fmt.Sprintf("/api/v1/forms/%s/submissions?pageNumber=%d&pageSize=100", formID, page)
		if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Items...)
		if len(resp.Items) == 0 || len(all) >= resp.TotalCount {
			break
		}
		page++
	}
	return all, nil
}

// FormListItem is a lightweight entry from the paginated forms list.
type FormListItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListForms returns all forms accessible to the API key, following pagination.
func (c *Client) ListForms(ctx context.Context) ([]FormListItem, error) {
	var all []FormListItem
	page := 1
	for {
		var resp struct {
			Data       []FormListItem `json:"data"`
			TotalCount int            `json:"totalCount"`
			PageNumber int            `json:"pageNumber"`
			PageSize   int            `json:"pageSize"`
		}
		path := fmt.Sprintf("/api/v1/forms?pageNumber=%d&pageSize=100", page)
		if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Data...)
		if len(resp.Data) == 0 || len(all) >= resp.TotalCount {
			break
		}
		page++
	}
	return all, nil
}
