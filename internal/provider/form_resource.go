package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

// Shared regexes for form-level validation, matching the API's server-side rules.
var (
	fieldNameRegexp = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	langCodeRegexp  = regexp.MustCompile(`^[a-z]{3}$`)
	currencyRegexp  = regexp.MustCompile(`^[a-zA-Z]{3}$`)
)

var (
	_ resource.Resource                   = (*formResource)(nil)
	_ resource.ResourceWithConfigure      = (*formResource)(nil)
	_ resource.ResourceWithImportState    = (*formResource)(nil)
	_ resource.ResourceWithValidateConfig = (*formResource)(nil)
)

// NewFormResource constructs the staticform_form resource.
func NewFormResource() resource.Resource { return &formResource{} }

type formResource struct {
	client *client.Client
}

type fieldModel struct {
	Name             types.String `tfsdk:"name"`
	Type             types.String `tfsdk:"type"`
	Rule             types.String `tfsdk:"rule"`
	Required         types.Bool   `tfsdk:"required"`
	ValidationConfig types.Map    `tfsdk:"validation_config"`
	Options          types.Map    `tfsdk:"options"`
}

type columnMappingModel struct {
	ColumnHeader  types.String `tfsdk:"column_header"`
	FormFieldName types.String `tfsdk:"form_field_name"`
}

type propertyMappingModel struct {
	PropertyName  types.String `tfsdk:"property_name"`
	PropertyType  types.String `tfsdk:"property_type"`
	FormFieldName types.String `tfsdk:"form_field_name"`
}

type conditionModel struct {
	FieldName     types.String `tfsdk:"field_name"`
	Operator      types.String `tfsdk:"operator"`
	Value         types.String `tfsdk:"value"`
	CaseSensitive types.Bool   `tfsdk:"case_sensitive"`
}

type runConditionModel struct {
	Connector  types.String     `tfsdk:"connector"`
	Conditions []conditionModel `tfsdk:"condition"`
}

type submitActionModel struct {
	ID           types.String        `tfsdk:"id"`
	Type         types.String        `tfsdk:"type"`
	Name         types.String        `tfsdk:"name"`
	Enabled      types.Bool          `tfsdk:"enabled"`
	RunCondition []runConditionModel `tfsdk:"run_condition"`

	// SendEmailAction
	Recipient         types.String            `tfsdk:"recipient"`
	ReplyTo           types.String            `tfsdk:"reply_to"`
	Subject           types.String            `tfsdk:"subject"`
	BodyTemplate      types.String            `tfsdk:"body_template"`
	BodyBuilderState  types.String            `tfsdk:"body_builder_state"`
	DisableBranding   types.Bool              `tfsdk:"disable_branding"`
	EmailDomainID     types.String            `tfsdk:"email_domain_id"`
	SMTPConnectionID  types.String            `tfsdk:"smtp_connection_id"`
	FromEmail         types.String            `tfsdk:"from_email"`
	FromName          types.String            `tfsdk:"from_name"`
	EmailTemplate     []emailTemplateModel    `tfsdk:"email_template"`
	FieldAttachments  []fieldAttachmentModel  `tfsdk:"field_attachment"`
	StaticAttachments []staticAttachmentModel `tfsdk:"static_attachment"`

	// TriggerWebhookAction
	WebhookURL  types.String `tfsdk:"webhook_url"`
	HTTPMethod  types.String `tfsdk:"http_method"`
	ContentType types.String `tfsdk:"content_type"`
	Headers     types.Map    `tfsdk:"headers"`

	// SyncGoogleSheetsAction
	GoogleConnectionID  types.String         `tfsdk:"google_connection_id"`
	SpreadsheetID       types.String         `tfsdk:"spreadsheet_id"`
	SheetName           types.String         `tfsdk:"sheet_name"`
	ColumnMappings      []columnMappingModel `tfsdk:"column_mapping"`
	IncludeSubmissionID types.Bool           `tfsdk:"include_submission_id"`
	IncludeCreatedAt    types.Bool           `tfsdk:"include_created_at"`
	WriteHeaderIfEmpty  types.Bool           `tfsdk:"write_header_if_empty"`

	// SyncNotionDatabaseAction
	NotionConnectionID types.String           `tfsdk:"notion_connection_id"`
	DatabaseID         types.String           `tfsdk:"database_id"`
	DatabaseName       types.String           `tfsdk:"database_name"`
	PropertyMappings   []propertyMappingModel `tfsdk:"property_mapping"`
}

type fieldAttachmentModel struct {
	FieldName types.String        `tfsdk:"field_name"`
	Condition []runConditionModel `tfsdk:"condition"`
}

type staticAttachmentModel struct {
	Source      types.String        `tfsdk:"source"`
	SourceHash  types.String        `tfsdk:"source_hash"`
	S3Key       types.String        `tfsdk:"s3_key"`
	FileName    types.String        `tfsdk:"file_name"`
	ContentType types.String        `tfsdk:"content_type"`
	FileSize    types.Int64         `tfsdk:"file_size"`
	Condition   []runConditionModel `tfsdk:"condition"`
}

type emailTemplateModel struct {
	PrimaryColor              types.String `tfsdk:"primary_color"`
	HeaderTextColor           types.String `tfsdk:"header_text_color"`
	FooterBackgroundColor     types.String `tfsdk:"footer_background_color"`
	FooterTextColor           types.String `tfsdk:"footer_text_color"`
	BodyBackgroundColor       types.String `tfsdk:"body_background_color"`
	BodyTextColor             types.String `tfsdk:"body_text_color"`
	PrimaryColorDark          types.String `tfsdk:"primary_color_dark"`
	HeaderTextColorDark       types.String `tfsdk:"header_text_color_dark"`
	FooterBackgroundColorDark types.String `tfsdk:"footer_background_color_dark"`
	FooterTextColorDark       types.String `tfsdk:"footer_text_color_dark"`
	BodyBackgroundColorDark   types.String `tfsdk:"body_background_color_dark"`
	BodyTextColorDark         types.String `tfsdk:"body_text_color_dark"`
	LogoURL                   types.String `tfsdk:"logo_url"`
	LogoWidth                 types.Int64  `tfsdk:"logo_width"`
	LogoHeight                types.Int64  `tfsdk:"logo_height"`
	HeaderText                types.String `tfsdk:"header_text"`
	HideHeaderText            types.Bool   `tfsdk:"hide_header_text"`
	SubtitleText              types.String `tfsdk:"subtitle_text"`
	HideSubtitle              types.Bool   `tfsdk:"hide_subtitle"`
	FooterText                types.String `tfsdk:"footer_text"`
	SentByText                types.String `tfsdk:"sent_by_text"`
	HideSentBy                types.Bool   `tfsdk:"hide_sent_by"`
	ShowFormID                types.Bool   `tfsdk:"show_form_id"`
}

type redirectBranchModel struct {
	Type      types.String `tfsdk:"type"`
	CustomURL types.String `tfsdk:"custom_url"`
	Message   types.String `tfsdk:"message"`
}

type redirectModel struct {
	Success []redirectBranchModel `tfsdk:"success"`
	Error   []redirectBranchModel `tfsdk:"error"`
}

type formResourceModel struct {
	ID                      types.String        `tfsdk:"id"`
	Name                    types.String        `tfsdk:"name"`
	SubmissionMode          types.String        `tfsdk:"submission_mode"`
	ServerSubmissionSecret  types.String        `tfsdk:"server_submission_secret"`
	CaptchaType             types.String        `tfsdk:"captcha_type"`
	CaptchaSecretKey        types.String        `tfsdk:"captcha_secret_key"`
	EnableHoneypot          types.Bool          `tfsdk:"enable_honeypot"`
	HoneypotFieldName       types.String        `tfsdk:"honeypot_field_name"`
	EnableLanguageDetection types.Bool          `tfsdk:"enable_language_detection"`
	ExpectedLanguages       types.List          `tfsdk:"expected_languages"`
	LanguageDetectionFields types.List          `tfsdk:"language_detection_fields"`
	Tags                    types.Set           `tfsdk:"tags"`
	Fields                  []fieldModel        `tfsdk:"field"`
	SubmitActions           []submitActionModel `tfsdk:"submit_action"`
	Redirect                []redirectModel     `tfsdk:"redirect"`
	Payment                 []paymentModel      `tfsdk:"payment"`
}

type fixedRuleModel struct {
	AmountCents types.Int64  `tfsdk:"amount_cents"`
	Description types.String `tfsdk:"description"`
}

type fieldAmountRuleModel struct {
	AmountFieldName types.String `tfsdk:"amount_field_name"`
	Description     types.String `tfsdk:"description"`
}

type lineItemModel struct {
	FieldName         types.String `tfsdk:"field_name"`
	PriceMap          types.Map    `tfsdk:"price_map"`
	QuantityFieldName types.String `tfsdk:"quantity_field_name"`
	Description       types.String `tfsdk:"description"`
}

type paymentModel struct {
	ConnectionID           types.String           `tfsdk:"connection_id"`
	Currency               types.String           `tfsdk:"currency"`
	Mode                   types.String           `tfsdk:"mode"`
	CustomerEmailFieldName types.String           `tfsdk:"customer_email_field_name"`
	FixedRule              []fixedRuleModel       `tfsdk:"fixed_rule"`
	FieldAmountRule        []fieldAmountRuleModel `tfsdk:"field_amount_rule"`
	LineItem               []lineItemModel        `tfsdk:"line_item"`
}

func (r *formResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_form"
}

// ValidateConfig enforces cross-block rules the per-attribute validators cannot express.
func (r *formResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m formResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A payment form sends the submitter to Stripe Checkout, which only returns them via an HTTP
	// redirect. Both the success and error outcomes must therefore be configured as redirects.
	if len(m.Payment) > 0 {
		hasBothRedirects := len(m.Redirect) > 0 && len(m.Redirect[0].Success) > 0 && len(m.Redirect[0].Error) > 0
		if !hasBothRedirects {
			resp.Diagnostics.AddAttributeError(
				path.Root("payment"),
				"Payment forms require redirect responses",
				"A form with a `payment` block must also define a `redirect` block with both `success` and `error` set. "+
					"Payment forms can only respond with an HTTP redirect, because the buyer returns from Stripe Checkout via a redirect.",
			)
		}
	}
}

func (r *formResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *formResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	mappingBlock := func(headerName, headerDesc string) schema.ListNestedBlock {
		return schema.ListNestedBlock{
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					headerName:        schema.StringAttribute{Required: true, MarkdownDescription: headerDesc},
					"form_field_name": schema.StringAttribute{Required: true, MarkdownDescription: "Name of the form field to map from."},
				},
			},
		}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "A StaticForm form: its fields, spam protection, redirect behaviour, and submit actions (email, webhook, Google Sheets, Notion).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Form ID. This is the value to POST submissions to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{Required: true, MarkdownDescription: "Human-readable form name.", Validators: []validator.String{stringvalidator.LengthBetween(1, 255)}},
			"submission_mode": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString("ClientSide"),
				Validators:          []validator.String{stringvalidator.OneOf("ClientSide", "ServerSide")},
				MarkdownDescription: "`ClientSide` (browser submissions, full spam protection) or `ServerSide` (backend submissions, requires the server secret).",
			},
			"server_submission_secret": schema.StringAttribute{
				Computed: true, Sensitive: true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Secret required to submit to a ServerSide form. Only set when `submission_mode = \"ServerSide\"`.",
			},
			"captcha_type": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString("None"),
				Validators:          []validator.String{stringvalidator.OneOf("None", "RecaptchaV2", "RecaptchaV3", "HCaptcha", "Turnstile")},
				MarkdownDescription: "Captcha provider: `None`, `RecaptchaV2`, `RecaptchaV3`, `HCaptcha`, or `Turnstile` (Cloudflare Turnstile).",
			},
			"captcha_secret_key": schema.StringAttribute{
				Optional: true, Sensitive: true,
				MarkdownDescription: "Captcha secret key. Required when `captcha_type` is not `None`.",
			},
			"enable_honeypot": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Enable the honeypot spam trap field.",
			},
			"honeypot_field_name": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("website_url"),
				MarkdownDescription: "Name of the honeypot field.",
			},
			"enable_language_detection": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Enable expected-language spam filtering.",
			},
			"expected_languages": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				Validators:          []validator.List{listvalidator.ValueStringsAre(stringvalidator.RegexMatches(langCodeRegexp, "must be a 3-letter lowercase ISO 639-2/T code"))},
				MarkdownDescription: "ISO 639-2/T codes (3 lowercase letters) allowed when language detection is on.",
			},
			"language_detection_fields": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Field names to run language detection against. Empty means all text fields.",
			},
			"tags": schema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				Validators:          []validator.Set{setvalidator.SizeAtMost(20), setvalidator.ValueStringsAre(stringvalidator.LengthBetween(1, 50))},
				MarkdownDescription: "Up to 20 tags (1-50 chars each) for organizing forms.",
			},
		},
		Blocks: map[string]schema.Block{
			"field": schema.ListNestedBlock{
				MarkdownDescription: "An input field. Order is preserved.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":              schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtMost(255), stringvalidator.RegexMatches(fieldNameRegexp, "must start with a lowercase letter and contain only lowercase letters, digits, and underscores")}, MarkdownDescription: "Field name. Lowercase, starts with a letter, `[a-z0-9_]`."},
						"type":              schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("Text", "File", "Number", "Boolean", "Enum", "Datetime", "Date")}, MarkdownDescription: "`Text`, `File`, `Number`, `Boolean`, `Enum`, `Datetime`, or `Date`."},
						"rule":              schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("None"), Validators: []validator.String{stringvalidator.OneOf("None", "Email", "Number", "PositiveNumber", "NumberRange", "Website", "Before", "After", "Between", "FileExtension", "FileSize")}, MarkdownDescription: "Validation rule: `None`, `Email`, `Website`, `Number`, `PositiveNumber`, `NumberRange`, `Before`, `After`, `Between`, `FileExtension`, or `FileSize`. Some rules require matching `validation_config` keys."},
						"required":          schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Whether the field is required."},
						"validation_config": schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Validation parameters (string values) keyed by rule: `minLength`/`maxLength` (Text, character bounds); `min`/`max` (Number, rule `NumberRange`); `before`/`after` (Date/Datetime, ISO-8601, rules `Before`/`After`/`Between`); `extensions` (File, rule `FileExtension`, comma-separated without dots, e.g. `pdf,doc,docx`); `maxFileSizeBytes` (File, rule `FileSize`, size in bytes)."},
						"options":           schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "UI/behaviour options (string values): `options` (Enum, required — JSON array of choices, e.g. `[\"Small\",\"Large\"]`); `allowMultiple` (Enum, `true`/`false` for multi-select); `isTextarea` (Text, `true`/`false` to render a multiline textarea); `maxFiles` (File, max number of files per submission)."},
					},
				},
			},
			"submit_action": schema.ListNestedBlock{
				MarkdownDescription: "An action run on submission. `type` selects which fields apply.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id":      schema.StringAttribute{Computed: true, MarkdownDescription: "Server-assigned action ID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
						"type":    schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("SendEmailAction", "TriggerWebhookAction", "SyncGoogleSheetsAction", "SyncNotionDatabaseAction")}, MarkdownDescription: "`SendEmailAction`, `TriggerWebhookAction`, `SyncGoogleSheetsAction`, or `SyncNotionDatabaseAction`."},
						"name":    schema.StringAttribute{Required: true, MarkdownDescription: "Action label."},
						"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the action runs."},

						"recipient":          schema.StringAttribute{Optional: true, MarkdownDescription: "Email: recipient(s), comma-separated. Supports the same `{{token}}` placeholders as `body_template` (e.g. `{{form.email}}` to send to an address submitted in the form)."},
						"reply_to":           schema.StringAttribute{Optional: true, MarkdownDescription: "Email: reply-to address. Supports `{{token}}` placeholders (e.g. `{{form.email}}`)."},
						"subject":            schema.StringAttribute{Optional: true, MarkdownDescription: "Email: subject. Supports the same `{{token}}` placeholders as `body_template` (e.g. `{{formName}}`, `{{form.email}}`, `{{timestamp}}`)."},
						"body_template":      schema.StringAttribute{Optional: true, MarkdownDescription: "Email body (HTML) or webhook request body. Supports `{{token}}` placeholders (spaces inside the braces are allowed): field values `{{fieldName}}` or `{{form.fieldName}}`; form metadata `{{formName}}`, `{{formId}}`, `{{submissionId}}`; all answers `{{submissionData}}` (formatted) or `{{submissionDataJson}}` (JSON object); date/time `{{timestamp}}`, `{{date}}`, `{{time}}` with optional zone and format, e.g. `{{timestamp:Europe/Amsterdam|yyyy-MM-dd HH:mm|notz}}` (`:zone` IANA/Windows timezone, `|format` .NET format string, `|notz` drops the ` UTC` suffix); and, when a `payment` block is present, `{{payment.amount}}`, `{{payment.amountCents}}`, `{{payment.currency}}`, `{{payment.status}}`, `{{payment.stripePaymentIntentId}}`, `{{payment.stripeCheckoutSessionId}}`. For email this is the rendered content sent; for webhooks it is the request body."},
						"body_builder_state": schema.StringAttribute{Optional: true, MarkdownDescription: "Email: optional JSON state for the dashboard's visual email builder. `body_template` is the source of truth for what is sent; set this only to round-trip the visual builder."},
						"disable_branding":   schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Email: remove StaticForm branding (Agency plan)."},
						"email_domain_id":    schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("smtp_connection_id"))}, MarkdownDescription: "Email: send via a verified managed domain (`staticform_email_domain` ID). Mutually exclusive with `smtp_connection_id`."},
						"smtp_connection_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Email: send via a BYO SMTP connection (`staticform_smtp_connection` ID). Mutually exclusive with `email_domain_id`."},
						"from_email":         schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Email: From address (managed-domain path only; ignored for SMTP)."},
						"from_name":          schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Email: From display name (managed-domain path only; ignored for SMTP)."},

						"webhook_url":  schema.StringAttribute{Optional: true, MarkdownDescription: "Webhook: target URL."},
						"http_method":  schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Validators: []validator.String{stringvalidator.OneOf("Get", "Post", "Put", "Patch", "Delete")}, MarkdownDescription: "Webhook: `Get`, `Post`, `Put`, `Patch`, `Delete` (default `Post`)."},
						"content_type": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Validators: []validator.String{stringvalidator.OneOf("Json", "FormUrlEncoded")}, MarkdownDescription: "Webhook: `Json` or `FormUrlEncoded` (default `Json`)."},
						"headers":      schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Webhook: custom headers."},

						"google_connection_id":  schema.StringAttribute{Optional: true, MarkdownDescription: "Google Sheets: OAuth connection ID."},
						"spreadsheet_id":        schema.StringAttribute{Optional: true, MarkdownDescription: "Google Sheets: spreadsheet ID."},
						"sheet_name":            schema.StringAttribute{Optional: true, MarkdownDescription: "Google Sheets: worksheet tab name."},
						"include_submission_id": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Google Sheets: include a submission ID column."},
						"include_created_at":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Google Sheets: include a created-at column."},
						"write_header_if_empty": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Google Sheets: write a header row if the sheet is empty."},

						"notion_connection_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Notion: connection ID."},
						"database_id":          schema.StringAttribute{Optional: true, MarkdownDescription: "Notion: database ID."},
						"database_name":        schema.StringAttribute{Optional: true, MarkdownDescription: "Notion: database name."},
					},
					Blocks: map[string]schema.Block{
						"column_mapping":   mappingBlock("column_header", "Google Sheets column header."),
						"property_mapping": notionPropertyBlock(),
						"email_template":   emailTemplateBlock(),
						"run_condition":    conditionGroupBlock("Optional: only run when these conditions match (Pro plan)."),
						"field_attachment": schema.ListNestedBlock{
							MarkdownDescription: "Email: attach an uploaded file-field's file (Pro plan).",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"field_name": schema.StringAttribute{Required: true, MarkdownDescription: "Name of the file field to attach."},
								},
								Blocks: map[string]schema.Block{
									"condition": conditionGroupBlock("Optional: only attach when these conditions match."),
								},
							},
						},
						"static_attachment": schema.ListNestedBlock{
							MarkdownDescription: "Email: attach a static file (Pro plan). Provide `source` to upload a local file automatically, or `s3_key` (+ metadata) to reference one already in the form-uploads bucket.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"source":       schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("s3_key"))}, MarkdownDescription: "Path to a local file to upload. Mutually exclusive with `s3_key`."},
									"source_hash":  schema.StringAttribute{Optional: true, MarkdownDescription: "Optional content hash (e.g. `filesha256(...)`); changing it re-uploads `source`."},
									"s3_key":       schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{uploadComputedString{}}, MarkdownDescription: "Object key in the form-uploads bucket. Computed when `source` is set."},
									"file_name":    schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{uploadComputedString{}}, MarkdownDescription: "Display file name. Computed when `source` is set."},
									"content_type": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{uploadComputedString{}}, MarkdownDescription: "MIME type. Computed when `source` is set."},
									"file_size":    schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{uploadComputedInt64{}}, MarkdownDescription: "File size in bytes. Computed when `source` is set."},
								},
								Blocks: map[string]schema.Block{
									"condition": conditionGroupBlock("Optional: only attach when these conditions match."),
								},
							},
						},
					},
				},
			},
			"redirect": schema.ListNestedBlock{
				MarkdownDescription: "Redirect behaviour after submission (client-side forms only). Required with both `success` and `error` when a `payment` block is present.",
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"success": redirectBranchBlock(),
						"error":   redirectBranchBlock(),
					},
				},
			},
			"payment": schema.ListNestedBlock{
				MarkdownDescription: "Stripe payment collection (Pro plan). At most one block. Requires a Stripe connection ID. When set, a `redirect` block with both `success` and `error` is required, because paid forms can only respond with an HTTP redirect (the buyer returns from Stripe Checkout via a redirect).",
				Validators:          []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"connection_id":             schema.StringAttribute{Required: true, MarkdownDescription: "Stripe connection ID."},
						"currency":                  schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.RegexMatches(currencyRegexp, "must be a 3-letter ISO 4217 currency code")}, MarkdownDescription: "ISO 4217 currency code (lowercase, e.g. `eur`)."},
						"mode":                      schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("Fixed", "FieldAmount", "LineItems")}, MarkdownDescription: "`Fixed`, `FieldAmount`, or `LineItems`."},
						"customer_email_field_name": schema.StringAttribute{Optional: true, MarkdownDescription: "Name of the email field whose value prefills the Stripe Checkout email. When omitted, the first field with the email validation rule is used."},
					},
					Blocks: map[string]schema.Block{
						"fixed_rule": schema.ListNestedBlock{
							MarkdownDescription: "Fixed-amount pricing rules (mode `Fixed`).",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"amount_cents": schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(0)}, MarkdownDescription: "Charge amount in the smallest currency unit."},
									"description":  schema.StringAttribute{Optional: true, MarkdownDescription: "Stripe line-item label."},
								},
							},
						},
						"field_amount_rule": schema.ListNestedBlock{
							MarkdownDescription: "Amount-from-field rules (mode `FieldAmount`).",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"amount_field_name": schema.StringAttribute{Required: true, MarkdownDescription: "Number field whose value is the charge."},
									"description":       schema.StringAttribute{Optional: true},
								},
							},
						},
						"line_item": schema.ListNestedBlock{
							MarkdownDescription: "Line items priced from fields (mode `LineItems`).",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"field_name":          schema.StringAttribute{Required: true, MarkdownDescription: "Enum field whose selected option sets the price."},
									"price_map":           schema.MapAttribute{Required: true, ElementType: types.Int64Type, Validators: []validator.Map{mapvalidator.ValueInt64sAre(int64validator.AtLeast(0))}, MarkdownDescription: "Map of option value to price (smallest currency unit)."},
									"quantity_field_name": schema.StringAttribute{Optional: true, MarkdownDescription: "Optional number field used as a quantity multiplier."},
									"description":         schema.StringAttribute{Optional: true},
								},
							},
						},
					},
				},
			},
		},
	}
}

func notionPropertyBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"property_name":   schema.StringAttribute{Required: true, MarkdownDescription: "Notion property name."},
				"property_type":   schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("title", "rich_text", "email", "phone_number", "url", "number", "checkbox", "date", "select", "multi_select")}, MarkdownDescription: "Notion property type: `title`, `rich_text`, `email`, `phone_number`, `url`, `number`, `checkbox`, `date`, `select`, or `multi_select`. Unrecognized types fall back to `rich_text`."},
				"form_field_name": schema.StringAttribute{Required: true, MarkdownDescription: "Form field to map from."},
			},
		},
	}
}

func conditionGroupBlock(desc string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		MarkdownDescription: desc,
		Validators:          []validator.List{listvalidator.SizeAtMost(1)},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"connector": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("And"), Validators: []validator.String{stringvalidator.OneOf("And", "Or")}, MarkdownDescription: "`And` or `Or` joining the conditions."},
			},
			Blocks: map[string]schema.Block{
				"condition": schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"field_name":     schema.StringAttribute{Required: true},
							"operator":       schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("Equals", "NotEmpty", "IsEmpty", "Contains", "MatchesRegex", "NotMatchesRegex")}, MarkdownDescription: "`Equals`, `NotEmpty`, `IsEmpty`, `Contains`, `MatchesRegex`, `NotMatchesRegex`."},
							"value":          schema.StringAttribute{Optional: true},
							"case_sensitive": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
						},
					},
				},
			},
		},
	}
}

func emailTemplateBlock() schema.ListNestedBlock {
	colorAttr := func(desc string) schema.Attribute {
		return schema.StringAttribute{Optional: true, MarkdownDescription: desc}
	}
	return schema.ListNestedBlock{
		MarkdownDescription: "Email: customize the StaticForm email wrapper (colors, logo, header/footer, dark mode). At most one block.",
		Validators:          []validator.List{listvalidator.SizeAtMost(1)},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"primary_color":                colorAttr("Primary/header background color."),
				"header_text_color":            colorAttr("Header text color."),
				"footer_background_color":      colorAttr("Footer background color (Agency)."),
				"footer_text_color":            colorAttr("Footer text color (Agency)."),
				"body_background_color":        colorAttr("Body background color."),
				"body_text_color":              colorAttr("Body text color."),
				"primary_color_dark":           colorAttr("Dark-mode primary color."),
				"header_text_color_dark":       colorAttr("Dark-mode header text color."),
				"footer_background_color_dark": colorAttr("Dark-mode footer background color (Agency)."),
				"footer_text_color_dark":       colorAttr("Dark-mode footer text color (Agency)."),
				"body_background_color_dark":   colorAttr("Dark-mode body background color."),
				"body_text_color_dark":         colorAttr("Dark-mode body text color."),
				"logo_url":                     schema.StringAttribute{Optional: true, MarkdownDescription: "Logo image URL."},
				"logo_width":                   schema.Int64Attribute{Optional: true, MarkdownDescription: "Logo width in px."},
				"logo_height":                  schema.Int64Attribute{Optional: true, MarkdownDescription: "Logo height in px."},
				"header_text":                  schema.StringAttribute{Optional: true, MarkdownDescription: "Header text."},
				"hide_header_text":             schema.BoolAttribute{Optional: true, MarkdownDescription: "Hide the header text."},
				"subtitle_text":                schema.StringAttribute{Optional: true, MarkdownDescription: "Subtitle text."},
				"hide_subtitle":                schema.BoolAttribute{Optional: true, MarkdownDescription: "Hide the subtitle."},
				"footer_text":                  schema.StringAttribute{Optional: true, MarkdownDescription: "Footer text (Agency)."},
				"sent_by_text":                 schema.StringAttribute{Optional: true, MarkdownDescription: "\"Sent by\" text (Agency)."},
				"hide_sent_by":                 schema.BoolAttribute{Optional: true, MarkdownDescription: "Hide the \"sent by\" line (Agency)."},
				"show_form_id":                 schema.BoolAttribute{Optional: true, MarkdownDescription: "Show the form ID in the footer (Agency)."},
			},
		},
	}
}

func redirectBranchBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"type":       schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("InternalPage", "CustomUrl")}, MarkdownDescription: "`InternalPage` or `CustomUrl`."},
				"custom_url": schema.StringAttribute{Optional: true, MarkdownDescription: "Target URL when type is `CustomUrl`."},
				"message":    schema.StringAttribute{Optional: true, MarkdownDescription: "Message shown for `InternalPage`."},
			},
		},
	}
}

func (r *formResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan formResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, d := r.toRequest(ctx, plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfgTags := plan.Tags
	uploads := hasSourceAttachments(plan)

	// Static attachments with a `source` must be uploaded, which needs the form
	// ID — so create the form first (without those attachments), upload, then
	// update the form to attach them.
	createBody := body
	if uploads {
		createBody = stripUnresolvedAttachments(body)
	}
	form, err := r.client.CreateForm(ctx, createBody)
	if err != nil {
		resp.Diagnostics.AddError("Error creating form", err.Error())
		return
	}

	if uploads {
		// Carry the server-assigned action IDs onto the plan so the follow-up
		// update merges actions in place rather than recreating them.
		for i := range plan.SubmitActions {
			if i < len(form.SubmitActions) {
				plan.SubmitActions[i].ID = strFromPtr(form.SubmitActions[i].ID)
			}
		}
		resp.Diagnostics.Append(r.resolveUploads(ctx, form.ID, &plan, nil)...)
		if resp.Diagnostics.HasError() {
			return
		}
		srcMap := attachmentSourceMap(plan)
		body2, d2 := r.toRequest(ctx, plan)
		resp.Diagnostics.Append(d2...)
		if resp.Diagnostics.HasError() {
			return
		}
		form, err = r.client.UpdateForm(ctx, form.ID, body2)
		if err != nil {
			resp.Diagnostics.AddError("Error attaching files to form", err.Error())
			return
		}
		resp.Diagnostics.Append(r.apply(ctx, form, &plan)...)
		mergeAttachmentSources(&plan, srcMap)
	} else {
		resp.Diagnostics.Append(r.apply(ctx, form, &plan)...)
	}

	resp.Diagnostics.Append(r.syncTags(ctx, form.ID, cfgTags, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// syncTags writes the configured tags via the dedicated tags endpoint (tags are
// not part of the form create/update body) and records the normalized result.
func (r *formResource) syncTags(ctx context.Context, formID string, configured types.Set, m *formResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if configured.IsNull() || configured.IsUnknown() {
		m.Tags = types.SetNull(types.StringType)
		// Still clear server-side tags in case this is an update removing them.
		if _, err := r.client.UpdateFormTags(ctx, formID, nil); err != nil {
			diags.AddError("Error clearing form tags", err.Error())
		}
		return diags
	}
	tags, d := stringSet(ctx, configured)
	diags.Append(d...)
	norm, err := r.client.UpdateFormTags(ctx, formID, tags)
	if err != nil {
		diags.AddError("Error setting form tags", err.Error())
		return diags
	}
	if len(norm) == 0 {
		m.Tags = types.SetNull(types.StringType)
	} else {
		m.Tags = toStringSet(norm)
	}
	return diags
}

func (r *formResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state formResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	form, err := r.client.GetForm(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading form", err.Error())
		return
	}

	// The API does not echo the provider-only attachment source/source_hash, so
	// preserve them across the refresh by re-merging from the prior state.
	srcMap := attachmentSourceMap(state)
	resp.Diagnostics.Append(r.apply(ctx, form, &state)...)
	mergeAttachmentSources(&state, srcMap)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *formResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan formResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfgTags := plan.Tags

	// Resolve any source-based static attachments first (the form ID is known on
	// update), reusing already-uploaded files whose source/source_hash are unchanged.
	var srcMap map[string]staticAttachmentModel
	if hasSourceAttachments(plan) {
		var state formResourceModel
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(r.resolveUploads(ctx, plan.ID.ValueString(), &plan, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
		srcMap = attachmentSourceMap(plan)
	}

	body, d := r.toRequest(ctx, plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	form, err := r.client.UpdateForm(ctx, plan.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Error updating form", err.Error())
		return
	}

	resp.Diagnostics.Append(r.apply(ctx, form, &plan)...)
	if srcMap != nil {
		mergeAttachmentSources(&plan, srcMap)
	}
	resp.Diagnostics.Append(r.syncTags(ctx, form.ID, cfgTags, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *formResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state formResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteForm(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting form", err.Error())
	}
}

func (r *formResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
