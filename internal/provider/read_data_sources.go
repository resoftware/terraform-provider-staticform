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
	Active            types.Bool   `tfsdk:"active"`
	Tier              types.String `tfsdk:"tier"`
	TierName          types.String `tfsdk:"tier_name"`
	PriceInCents      types.Int64  `tfsdk:"price_in_cents"`
	BillingInterval   types.String `tfsdk:"billing_interval"`
	Status            types.String `tfsdk:"status"`
	CurrentPeriodEnd  types.String `tfsdk:"current_period_end"`
	CancelAtPeriodEnd types.Bool   `tfsdk:"cancel_at_period_end"`
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
		MarkdownDescription: "The account's current subscription (read-only).",
		Attributes: map[string]dschema.Attribute{
			"active":               dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether an active subscription exists."},
			"tier":                 dschema.StringAttribute{Computed: true},
			"tier_name":            dschema.StringAttribute{Computed: true},
			"price_in_cents":       dschema.Int64Attribute{Computed: true},
			"billing_interval":     dschema.StringAttribute{Computed: true},
			"status":               dschema.StringAttribute{Computed: true},
			"current_period_end":   dschema.StringAttribute{Computed: true},
			"cancel_at_period_end": dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *subscriptionDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	sub, err := d.client.GetSubscription(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading subscription", err.Error())
		return
	}
	var state subscriptionModel
	if sub == nil || sub.ID == "" {
		state = subscriptionModel{
			Active: types.BoolValue(false), Tier: types.StringNull(), TierName: types.StringNull(),
			PriceInCents: types.Int64Null(), BillingInterval: types.StringNull(), Status: types.StringNull(),
			CurrentPeriodEnd: types.StringNull(), CancelAtPeriodEnd: types.BoolValue(false),
		}
	} else {
		state = subscriptionModel{
			Active:            types.BoolValue(true),
			Tier:              types.StringValue(sub.Tier),
			TierName:          types.StringValue(sub.TierName),
			PriceInCents:      types.Int64Value(sub.PriceInCents),
			BillingInterval:   types.StringValue(sub.BillingInterval),
			Status:            types.StringValue(sub.Status),
			CurrentPeriodEnd:  strFromPtr(sub.CurrentPeriodEndUTC),
			CancelAtPeriodEnd: types.BoolValue(sub.CancelAtPeriodEnd),
		}
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
		MarkdownDescription: "The authenticated account's profile and plan capabilities (read-only). Use the `can_use_*` flags to gate features in your configuration.",
		Attributes: map[string]dschema.Attribute{
			"id":                                 dschema.StringAttribute{Computed: true},
			"email":                              dschema.StringAttribute{Computed: true},
			"is_admin":                           dschema.BoolAttribute{Computed: true},
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
	state := userModel{
		ID:                             types.StringValue(p.ID),
		Email:                          types.StringValue(p.Email),
		IsAdmin:                        types.BoolValue(p.IsAdmin),
		CanUseGoogleSheets:             types.BoolValue(p.CanUseGoogleSheets),
		CanUseNotion:                   types.BoolValue(p.CanUseNotion),
		CanUseEmailAttachments:         types.BoolValue(p.CanUseEmailAttachments),
		CanUseConditionalSubmitActions: types.BoolValue(p.CanUseConditionalSubmitActions),
		CanUsePaymentForms:             types.BoolValue(p.CanUsePaymentForms),
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
