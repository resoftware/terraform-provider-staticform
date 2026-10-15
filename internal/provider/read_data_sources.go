package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

// ── staticform_subscription ─────────────────────────────────────────────────

var (
	_ datasource.DataSource              = (*subscriptionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*subscriptionDataSource)(nil)
)

// NewSubscriptionDataSource constructs the staticform_subscription data source.
func NewSubscriptionDataSource() datasource.DataSource { return &subscriptionDataSource{} }

type subscriptionDataSource struct{ client *client.Client }

type subscriptionModel struct {
	Active             types.Bool    `tfsdk:"active"`
	HasBillingAccount  types.Bool    `tfsdk:"has_billing_account"`
	StripeCustomerID   types.String  `tfsdk:"stripe_customer_id"`
	SubscriptionID     types.String  `tfsdk:"subscription_id"`
	StripeProductID    types.String  `tfsdk:"stripe_product_id"`
	StripePriceID      types.String  `tfsdk:"stripe_price_id"`
	Status             types.String  `tfsdk:"status"`
	Interval           types.String  `tfsdk:"interval"`
	UnitPriceAmount    types.Float64 `tfsdk:"unit_price_amount"`
	UnitPriceCurrency  types.String  `tfsdk:"unit_price_currency"`
	CurrentPeriodStart types.String  `tfsdk:"current_period_start"`
	CurrentPeriodEnd   types.String  `tfsdk:"current_period_end"`
	TrialEnd           types.String  `tfsdk:"trial_end"`
	CancelAtPeriodEnd  types.Bool    `tfsdk:"cancel_at_period_end"`
	CanceledAt         types.String  `tfsdk:"canceled_at"`
	Entitlements       types.List    `tfsdk:"entitlements"`
}

func (d *subscriptionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subscription"
}

func (d *subscriptionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *subscriptionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "The account's billing state and active subscription (read-only). Requires the `User.Billing.Get` permission. " +
			"The subscription attributes are null when `active` is false.",
		Attributes: map[string]dschema.Attribute{
			"active":               dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether an active subscription exists."},
			"has_billing_account":  dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the account has a Stripe customer."},
			"stripe_customer_id":   dschema.StringAttribute{Computed: true, MarkdownDescription: "Stripe customer ID."},
			"subscription_id":      dschema.StringAttribute{Computed: true, MarkdownDescription: "Subscription ID."},
			"stripe_product_id":    dschema.StringAttribute{Computed: true, MarkdownDescription: "Stripe product ID of the subscribed plan."},
			"stripe_price_id":      dschema.StringAttribute{Computed: true, MarkdownDescription: "Stripe price ID of the subscribed plan."},
			"status":               dschema.StringAttribute{Computed: true, MarkdownDescription: "`Incomplete`, `IncompleteExpired`, `Trialing`, `Active`, `PastDue`, `Canceled`, `Unpaid`, or `Paused`."},
			"interval":             dschema.StringAttribute{Computed: true, MarkdownDescription: "Billing interval: `Day`, `Week`, `Month`, or `Year`."},
			"unit_price_amount":    dschema.Float64Attribute{Computed: true, MarkdownDescription: "Price per interval in the major unit of `unit_price_currency`."},
			"unit_price_currency":  dschema.StringAttribute{Computed: true, MarkdownDescription: "Currency of `unit_price_amount`."},
			"current_period_start": dschema.StringAttribute{Computed: true, MarkdownDescription: "Start of the current billing period (RFC 3339)."},
			"current_period_end":   dschema.StringAttribute{Computed: true, MarkdownDescription: "End of the current billing period (RFC 3339)."},
			"trial_end":            dschema.StringAttribute{Computed: true, MarkdownDescription: "End of the trial (RFC 3339). Null when there is no trial."},
			"cancel_at_period_end": dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the subscription ends at the end of the current period."},
			"canceled_at":          dschema.StringAttribute{Computed: true, MarkdownDescription: "When the subscription was canceled (RFC 3339). Null when it was not."},
			"entitlements":         dschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Entitlement keys granted to the account."},
		},
	}
}

func (d *subscriptionDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	billing, err := d.client.GetBilling(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading billing state", err.Error())
		return
	}
	state := subscriptionModel{
		Active:             types.BoolValue(billing.ActiveSubscription != nil),
		HasBillingAccount:  types.BoolValue(billing.HasBillingAccount),
		StripeCustomerID:   strFromPtr(billing.StripeCustomerID),
		SubscriptionID:     types.StringNull(),
		StripeProductID:    types.StringNull(),
		StripePriceID:      types.StringNull(),
		Status:             types.StringNull(),
		Interval:           types.StringNull(),
		UnitPriceAmount:    types.Float64Null(),
		UnitPriceCurrency:  types.StringNull(),
		CurrentPeriodStart: types.StringNull(),
		CurrentPeriodEnd:   types.StringNull(),
		TrialEnd:           types.StringNull(),
		CancelAtPeriodEnd:  types.BoolValue(false),
		CanceledAt:         types.StringNull(),
		Entitlements:       toStringList(append([]string{}, billing.Entitlements...)),
	}
	if sub := billing.ActiveSubscription; sub != nil {
		state.SubscriptionID = types.StringValue(sub.SubscriptionID)
		state.StripeProductID = types.StringValue(sub.StripeProductID)
		state.StripePriceID = types.StringValue(sub.StripePriceID)
		state.Status = types.StringValue(sub.Status)
		state.Interval = types.StringValue(sub.Interval)
		state.UnitPriceAmount = types.Float64Value(sub.UnitPriceAmount)
		state.UnitPriceCurrency = types.StringValue(sub.UnitPriceCurrency)
		state.CurrentPeriodStart = types.StringValue(sub.CurrentPeriodStart)
		state.CurrentPeriodEnd = types.StringValue(sub.CurrentPeriodEnd)
		state.TrialEnd = strFromPtr(sub.TrialEndUTC)
		state.CancelAtPeriodEnd = types.BoolValue(sub.CancelAtPeriodEnd)
		state.CanceledAt = strFromPtr(sub.CanceledAtUTC)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ── staticform_user ─────────────────────────────────────────────────────────

var (
	_ datasource.DataSource              = (*userDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*userDataSource)(nil)
)

// NewUserDataSource constructs the staticform_user data source.
func NewUserDataSource() datasource.DataSource { return &userDataSource{} }

type userDataSource struct{ client *client.Client }

type userModel struct {
	ID                             types.String `tfsdk:"id"`
	Email                          types.String `tfsdk:"email"`
	IsAdmin                        types.Bool   `tfsdk:"is_admin"`
	CanUseGoogleSheets             types.Bool   `tfsdk:"can_use_google_sheets"`
	CanUseNotion                   types.Bool   `tfsdk:"can_use_notion"`
	CanUseEmailAttachments         types.Bool   `tfsdk:"can_use_email_attachments"`
	CanUseConditionalSubmitActions types.Bool   `tfsdk:"can_use_conditional_submit_actions"`
	CanUsePaymentForms             types.Bool   `tfsdk:"can_use_payment_forms"`
	Permissions                    types.List   `tfsdk:"permissions"`
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "The authenticated account's profile and plan capabilities (read-only). Use the `can_use_*` flags to gate features in your configuration. " +
			"Requires the `User.Account.Get` and `User.Billing.GetEntitlements` permissions.",
		Attributes: map[string]dschema.Attribute{
			"id":                                 dschema.StringAttribute{Computed: true},
			"email":                              dschema.StringAttribute{Computed: true},
			"is_admin":                           dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the account is a superadmin."},
			"can_use_google_sheets":              dschema.BoolAttribute{Computed: true},
			"can_use_notion":                     dschema.BoolAttribute{Computed: true},
			"can_use_email_attachments":          dschema.BoolAttribute{Computed: true},
			"can_use_conditional_submit_actions": dschema.BoolAttribute{Computed: true},
			"can_use_payment_forms":              dschema.BoolAttribute{Computed: true},
			"permissions":                        dschema.ListAttribute{Computed: true, ElementType: types.StringType},
		},
	}
}

func (d *userDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	p, err := d.client.GetProfile(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading profile", err.Error())
		return
	}
	entitlements, err := d.client.GetEntitlements(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading entitlements", err.Error())
		return
	}
	state := userModel{
		ID:                             types.StringValue(p.ID),
		Email:                          types.StringValue(p.Email),
		IsAdmin:                        types.BoolValue(p.IsSuperAdmin),
		CanUseGoogleSheets:             types.BoolValue(entitlements.Has(client.EntitlementGoogleSheets)),
		CanUseNotion:                   types.BoolValue(entitlements.Has(client.EntitlementNotion)),
		CanUseEmailAttachments:         types.BoolValue(entitlements.Has(client.EntitlementEmailAttachments)),
		CanUseConditionalSubmitActions: types.BoolValue(entitlements.Has(client.EntitlementConditionalActions)),
		CanUsePaymentForms:             types.BoolValue(entitlements.Has(client.EntitlementPaymentForms)),
		Permissions:                    toStringList(p.Permissions),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ── staticform_execution_logs ───────────────────────────────────────────────

var (
	_ datasource.DataSource              = (*executionLogsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*executionLogsDataSource)(nil)
)

// NewExecutionLogsDataSource constructs the staticform_execution_logs data source.
func NewExecutionLogsDataSource() datasource.DataSource { return &executionLogsDataSource{} }

type executionLogsDataSource struct{ client *client.Client }

type executionLogModel struct {
	ID             types.String `tfsdk:"id"`
	SubmissionID   types.String `tfsdk:"submission_id"`
	SubmitActionID types.String `tfsdk:"submit_action_id"`
	FormName       types.String `tfsdk:"form_name"`
	ActionType     types.String `tfsdk:"action_type"`
	Status         types.String `tfsdk:"status"`
	StartedAt      types.String `tfsdk:"started_at"`
	DurationMs     types.Int64  `tfsdk:"duration_ms"`
	HTTPStatusCode types.Int64  `tfsdk:"http_status_code"`
	ErrorMessage   types.String `tfsdk:"error_message"`
}

type executionLogsModel struct {
	FormID types.String        `tfsdk:"form_id"`
	Logs   []executionLogModel `tfsdk:"logs"`
}

func (d *executionLogsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_execution_logs"
}

func (d *executionLogsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *executionLogsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Submit-action execution logs (read-only), optionally filtered by form.",
		Attributes: map[string]dschema.Attribute{
			"form_id": dschema.StringAttribute{Optional: true, MarkdownDescription: "Filter logs to a single form."},
			"logs": dschema.ListNestedAttribute{
				Computed: true,
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"id":               dschema.StringAttribute{Computed: true},
						"submission_id":    dschema.StringAttribute{Computed: true},
						"submit_action_id": dschema.StringAttribute{Computed: true},
						"form_name":        dschema.StringAttribute{Computed: true},
						"action_type":      dschema.StringAttribute{Computed: true},
						"status":           dschema.StringAttribute{Computed: true},
						"started_at":       dschema.StringAttribute{Computed: true},
						"duration_ms":      dschema.Int64Attribute{Computed: true},
						"http_status_code": dschema.Int64Attribute{Computed: true},
						"error_message":    dschema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *executionLogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg executionLogsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	logs, err := d.client.ListExecutionLogs(ctx, optStr(cfg.FormID))
	if err != nil {
		resp.Diagnostics.AddError("Error listing execution logs", err.Error())
		return
	}
	cfg.Logs = []executionLogModel{}
	for _, l := range logs {
		cfg.Logs = append(cfg.Logs, executionLogModel{
			ID:             types.StringValue(l.ID),
			SubmissionID:   types.StringValue(l.SubmissionID),
			SubmitActionID: types.StringValue(l.SubmitActionID),
			FormName:       types.StringValue(l.FormName),
			ActionType:     types.StringValue(l.ActionType),
			Status:         types.StringValue(l.Status),
			StartedAt:      types.StringValue(l.StartedAtUTC),
			DurationMs:     types.Int64Value(l.DurationMs),
			HTTPStatusCode: int64FromPtr(l.HTTPStatusCode),
			ErrorMessage:   strFromPtr(l.ErrorMessage),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
