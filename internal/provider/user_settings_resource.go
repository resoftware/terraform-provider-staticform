package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

var (
	_ resource.Resource                = (*userSettingsResource)(nil)
	_ resource.ResourceWithConfigure   = (*userSettingsResource)(nil)
	_ resource.ResourceWithImportState = (*userSettingsResource)(nil)
)

// NewUserSettingsResource constructs the staticform_user_settings resource — a
// singleton for the authenticated account's notification settings.
func NewUserSettingsResource() resource.Resource { return &userSettingsResource{} }

type userSettingsResource struct {
	client *client.Client
}

type userSettingsModel struct {
	ID                               types.String `tfsdk:"id"`
	SubmitActionFailureNotification  types.Bool   `tfsdk:"submit_action_failure_notification_enabled"`
	EmailOverageEnabled              types.Bool   `tfsdk:"email_overage_enabled"`
	EmailQuotaWarningEnabled         types.Bool   `tfsdk:"email_quota_warning_enabled"`
	StorageWarningEnabled            types.Bool   `tfsdk:"storage_warning_enabled"`
	SubmissionOverviewEmailFrequency types.String `tfsdk:"submission_overview_email_frequency"`
}

func (r *userSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_settings"
}

func (r *userSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The authenticated account's notification settings. This is a singleton — one per account. " +
			"Destroying it only removes it from state; it does not reset the account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"submit_action_failure_notification_enabled": schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Email the account when a submit action fails."},
			"email_overage_enabled":                      schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Allow email overage charges beyond the plan quota."},
			"email_quota_warning_enabled":                schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Warn at 80%/95% of the email quota."},
			"storage_warning_enabled":                    schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Warn at 80%/90% of storage."},
			"submission_overview_email_frequency":        schema.StringAttribute{Optional: true, Computed: true, Validators: []validator.String{stringvalidator.OneOf("Off", "Daily", "Weekly", "Monthly")}, MarkdownDescription: "Submission summary cadence: `Off`, `Daily`, `Weekly`, or `Monthly`."},
		},
	}
}

// desired overlays the user-set plan values on top of the current server settings
// so omitted attributes keep their current value (the PUT endpoint replaces all).
func (r *userSettingsResource) desired(ctx context.Context, plan userSettingsModel) (client.UserSettings, error) {
	cur, err := r.client.GetUserSettings(ctx)
	if err != nil {
		return client.UserSettings{}, err
	}
	s := *cur
	if !plan.SubmitActionFailureNotification.IsNull() && !plan.SubmitActionFailureNotification.IsUnknown() {
		s.SubmitActionFailureNotificationEnabled = plan.SubmitActionFailureNotification.ValueBool()
	}
	if !plan.EmailOverageEnabled.IsNull() && !plan.EmailOverageEnabled.IsUnknown() {
		s.EmailOverageEnabled = plan.EmailOverageEnabled.ValueBool()
	}
	if !plan.EmailQuotaWarningEnabled.IsNull() && !plan.EmailQuotaWarningEnabled.IsUnknown() {
		s.EmailQuotaWarningEnabled = plan.EmailQuotaWarningEnabled.ValueBool()
	}
	if !plan.StorageWarningEnabled.IsNull() && !plan.StorageWarningEnabled.IsUnknown() {
		s.StorageWarningEnabled = plan.StorageWarningEnabled.ValueBool()
	}
	if !plan.SubmissionOverviewEmailFrequency.IsNull() && !plan.SubmissionOverviewEmailFrequency.IsUnknown() {
		s.SubmissionOverviewEmailFrequency = plan.SubmissionOverviewEmailFrequency.ValueString()
	}
	return s, nil
}

func applyUserSettings(s *client.UserSettings, m *userSettingsModel) {
	m.ID = types.StringValue("settings")
	m.SubmitActionFailureNotification = types.BoolValue(s.SubmitActionFailureNotificationEnabled)
	m.EmailOverageEnabled = types.BoolValue(s.EmailOverageEnabled)
	m.EmailQuotaWarningEnabled = types.BoolValue(s.EmailQuotaWarningEnabled)
	m.StorageWarningEnabled = types.BoolValue(s.StorageWarningEnabled)
	m.SubmissionOverviewEmailFrequency = types.StringValue(s.SubmissionOverviewEmailFrequency)
}

func (r *userSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	desired, err := r.desired(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error reading current settings", err.Error())
		return
	}
	out, err := r.client.UpdateUserSettings(ctx, desired)
	if err != nil {
		resp.Diagnostics.AddError("Error updating settings", err.Error())
		return
	}
	applyUserSettings(out, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.GetUserSettings(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading settings", err.Error())
		return
	}
	applyUserSettings(out, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	desired, err := r.desired(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error reading current settings", err.Error())
		return
	}
	out, err := r.client.UpdateUserSettings(ctx, desired)
	if err != nil {
		resp.Diagnostics.AddError("Error updating settings", err.Error())
		return
	}
	applyUserSettings(out, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userSettingsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// Settings can't be deleted; removing the resource just drops it from state.
}

func (r *userSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
