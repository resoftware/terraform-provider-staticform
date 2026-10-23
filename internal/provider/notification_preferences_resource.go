package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

var (
	_ resource.Resource                   = (*notificationPreferencesResource)(nil)
	_ resource.ResourceWithConfigure      = (*notificationPreferencesResource)(nil)
	_ resource.ResourceWithImportState    = (*notificationPreferencesResource)(nil)
	_ resource.ResourceWithValidateConfig = (*notificationPreferencesResource)(nil)
)

const notificationPreferencesID = "notification-preferences"

var objectAsOptions = basetypes.ObjectAsOptions{}

var quietHoursTime = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

var (
	quietHoursAttrTypes = map[string]attr.Type{"start": types.StringType, "end": types.StringType}
	eventAttrTypes      = map[string]attr.Type{"in_app": types.BoolType, "email": types.BoolType}
)

// NewNotificationPreferencesResource constructs the staticform_notification_preferences
// resource, a singleton for the authenticated user's notification preferences.
func NewNotificationPreferencesResource() resource.Resource {
	return &notificationPreferencesResource{}
}

type notificationPreferencesResource struct {
	client *client.Client
}

type notificationPreferencesModel struct {
	ID              types.String `tfsdk:"id"`
	EmailEnabled    types.Bool   `tfsdk:"email_enabled"`
	EmailMode       types.String `tfsdk:"email_mode"`
	DigestHourLocal types.Int64  `tfsdk:"digest_hour_local"`
	TimeZoneID      types.String `tfsdk:"time_zone_id"`
	QuietHours      types.Object `tfsdk:"quiet_hours"`
	Events          types.Map    `tfsdk:"events"`
}

type quietHoursModel struct {
	Start types.String `tfsdk:"start"`
	End   types.String `tfsdk:"end"`
}

type eventPreferenceModel struct {
	InApp types.Bool `tfsdk:"in_app"`
	Email types.Bool `tfsdk:"email"`
}

func (r *notificationPreferencesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_preferences"
}

func (r *notificationPreferencesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *notificationPreferencesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The signed-in user's notification preferences: the global email switches and the per-event in-app and email choices. " +
			"This is a singleton: one per user. Only the events listed in `events` are managed; every other event keeps the choice it has on the server, " +
			"and global attributes you omit keep their current server value. `quiet_hours` is the exception: omitting it means no quiet hours. " +
			"Destroying the resource only removes it from state; the preferences stay as they are on the server. " +
			"Requires the `User.Notifications.GetPreferences` and `User.Notifications.UpdatePreferences` permissions.\n\n" +
			"Event keys come from the notification catalog. The three events that used to be separate account settings are " +
			"`product.SubmitActionFailed` (formerly `submit_action_failure_notification_enabled`), " +
			"`product.EmailQuotaWarning` (formerly `email_quota_warning_enabled`) and " +
			"`product.StorageWarning` (formerly `storage_warning_enabled`). The other keys are `product.SubmissionReceived`, " +
			"`product.DataExported`, `product.OverrideSubscriptionExpiringSoon`, `product.EmailDomainVerificationFailed` and `system.test`. " +
			"The `staticform_notification_preferences` data source lists the catalog the API currently serves.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"email_enabled": schema.BoolAttribute{
				Optional: true, Computed: true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Master email switch. When `false`, no notification is emailed regardless of the per-event `email` choices.",
			},
			"email_mode": schema.StringAttribute{
				Optional: true, Computed: true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.OneOf("Immediate", "DailyDigest")},
				MarkdownDescription: "How email is delivered: `Immediate` or `DailyDigest`.",
			},
			"digest_hour_local": schema.Int64Attribute{
				Optional: true, Computed: true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Validators:          []validator.Int64{int64validator.Between(0, 23)},
				MarkdownDescription: "Hour of the day (0 to 23, in `time_zone_id`) at which the daily digest is sent.",
			},
			"time_zone_id": schema.StringAttribute{
				Optional: true, Computed: true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
				MarkdownDescription: "Time zone used for the digest hour and quiet hours, for example `Europe/Amsterdam`.",
			},
			"quiet_hours": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Daily window in which immediate email is held until it ends. Omit it for no quiet hours. The window may wrap midnight, for example `22:00` to `07:00`.",
				Attributes: map[string]schema.Attribute{
					"start": schema.StringAttribute{
						Required:            true,
						Validators:          []validator.String{stringvalidator.RegexMatches(quietHoursTime, "must be a time formatted HH:mm")},
						MarkdownDescription: "Start of the window, formatted `HH:mm`.",
					},
					"end": schema.StringAttribute{
						Required:            true,
						Validators:          []validator.String{stringvalidator.RegexMatches(quietHoursTime, "must be a time formatted HH:mm")},
						MarkdownDescription: "End of the window, formatted `HH:mm`. Must differ from `start`.",
					},
				},
			},
			"events": schema.MapNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Per-event choices keyed by event key, for example `product.StorageWarning`. Only the listed events are managed.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"in_app": schema.BoolAttribute{
							Optional: true, Computed: true,
							PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
							MarkdownDescription: "Whether the notification appears in the in-app inbox. Omitted keeps the current server value.",
						},
						"email": schema.BoolAttribute{
							Optional: true, Computed: true,
							PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
							MarkdownDescription: "Whether the notification is also emailed. Requires `in_app` to be `true`. Omitted keeps the current server value, or `false` when `in_app` is `false`.",
						},
					},
				},
			},
		},
	}
}

func (r *notificationPreferencesResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config notificationPreferencesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !config.QuietHours.IsNull() && !config.QuietHours.IsUnknown() {
		var q quietHoursModel
		resp.Diagnostics.Append(config.QuietHours.As(ctx, &q, objectAsOptions)...)
		if !q.Start.IsUnknown() && !q.End.IsUnknown() && q.Start.ValueString() == q.End.ValueString() {
			resp.Diagnostics.AddAttributeError(path.Root("quiet_hours"), "Empty quiet hours", "quiet_hours start and end must differ.")
		}
	}
	if config.Events.IsNull() || config.Events.IsUnknown() {
		return
	}
	events := map[string]eventPreferenceModel{}
	resp.Diagnostics.Append(config.Events.ElementsAs(ctx, &events, false)...)
	for key, e := range events {
		if !e.InApp.IsNull() && !e.InApp.IsUnknown() && !e.InApp.ValueBool() &&
			!e.Email.IsNull() && !e.Email.IsUnknown() && e.Email.ValueBool() {
			resp.Diagnostics.AddAttributeError(path.Root("events").AtMapKey(key), "Email requires in_app",
				fmt.Sprintf("Event %q has email enabled but in_app disabled. Email requires the in-app notification to be on.", key))
		}
	}
}

func normalizeQuietTime(s string) string {
	if len(s) == len("15:04:05") && strings.HasSuffix(s, ":00") {
		return s[:len("15:04")]
	}
	return s
}

func wireQuietTime(s string) string {
	if len(s) == len("15:04") {
		return s + ":00"
	}
	return s
}

func knownKeys(p *client.NotificationPreferences) string {
	keys := make([]string, 0, len(p.Events))
	for _, e := range p.Events {
		keys = append(keys, e.Key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// desired overlays the plan on the current server preferences, so omitted
// attributes and events that are not managed keep their current value.
func (r *notificationPreferencesResource) desired(ctx context.Context, plan notificationPreferencesModel) (client.NotificationPreferencesRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	cur, err := r.client.GetNotificationPreferences(ctx)
	if err != nil {
		diags.AddError("Error reading current notification preferences", err.Error())
		return client.NotificationPreferencesRequest{}, diags
	}
	req := cur.Request()
	if !plan.EmailEnabled.IsNull() && !plan.EmailEnabled.IsUnknown() {
		req.EmailEnabled = plan.EmailEnabled.ValueBool()
	}
	if !plan.EmailMode.IsNull() && !plan.EmailMode.IsUnknown() {
		req.EmailMode = plan.EmailMode.ValueString()
	}
	if !plan.DigestHourLocal.IsNull() && !plan.DigestHourLocal.IsUnknown() {
		req.DigestHourLocal = int(plan.DigestHourLocal.ValueInt64())
	}
	if !plan.TimeZoneID.IsNull() && !plan.TimeZoneID.IsUnknown() {
		req.TimeZoneID = plan.TimeZoneID.ValueString()
	}
	req.QuietHours = nil
	if !plan.QuietHours.IsNull() && !plan.QuietHours.IsUnknown() {
		var q quietHoursModel
		diags.Append(plan.QuietHours.As(ctx, &q, objectAsOptions)...)
		req.QuietHours = &client.QuietHours{Start: wireQuietTime(q.Start.ValueString()), End: wireQuietTime(q.End.ValueString())}
	}
	if plan.Events.IsNull() || plan.Events.IsUnknown() {
		return req, diags
	}
	events := map[string]eventPreferenceModel{}
	diags.Append(plan.Events.ElementsAs(ctx, &events, false)...)
	for key, choice := range events {
		current, ok := cur.Event(key)
		if !ok {
			diags.AddAttributeError(path.Root("events").AtMapKey(key), "Unknown notification event",
				fmt.Sprintf("The API has no notification event %q. Known events: %s.", key, knownKeys(cur)))
			continue
		}
		if !current.OptOutAllowed {
			diags.AddAttributeError(path.Root("events").AtMapKey(key), "Notification cannot be changed",
				fmt.Sprintf("The notification event %q is mandatory and cannot be switched off or on.", key))
			continue
		}
		inApp, email := current.InApp, current.Email
		if !choice.InApp.IsNull() && !choice.InApp.IsUnknown() {
			inApp = choice.InApp.ValueBool()
			if !inApp {
				email = false
			}
		}
		if !choice.Email.IsNull() && !choice.Email.IsUnknown() {
			email = choice.Email.ValueBool()
		}
		if email && !inApp {
			diags.AddAttributeError(path.Root("events").AtMapKey(key), "Email requires in_app",
				fmt.Sprintf("Event %q has email enabled but in_app disabled. Email requires the in-app notification to be on.", key))
			continue
		}
		for i := range req.Events {
			if req.Events[i].Key == key {
				req.Events[i].InApp, req.Events[i].Email = inApp, email
			}
		}
	}
	return req, diags
}

// applyNotificationPreferences copies the server preferences into the model.
// Events are limited to the keys already in the model, so unmanaged events never
// enter state.
func applyNotificationPreferences(ctx context.Context, p *client.NotificationPreferences, m *notificationPreferencesModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(notificationPreferencesID)
	m.EmailEnabled = types.BoolValue(p.EmailEnabled)
	m.EmailMode = types.StringValue(p.EmailMode)
	m.DigestHourLocal = types.Int64Value(int64(p.DigestHourLocal))
	m.TimeZoneID = types.StringValue(p.TimeZoneID)
	m.QuietHours = types.ObjectNull(quietHoursAttrTypes)
	if p.QuietHours != nil {
		obj, d := types.ObjectValueFrom(ctx, quietHoursAttrTypes, quietHoursModel{
			Start: types.StringValue(normalizeQuietTime(p.QuietHours.Start)),
			End:   types.StringValue(normalizeQuietTime(p.QuietHours.End)),
		})
		diags.Append(d...)
		m.QuietHours = obj
	}
	if m.Events.IsNull() || m.Events.IsUnknown() {
		m.Events = types.MapNull(types.ObjectType{AttrTypes: eventAttrTypes})
		return diags
	}
	managed := map[string]eventPreferenceModel{}
	diags.Append(m.Events.ElementsAs(ctx, &managed, false)...)
	out := map[string]eventPreferenceModel{}
	for key := range managed {
		if e, ok := p.Event(key); ok {
			out[key] = eventPreferenceModel{InApp: types.BoolValue(e.InApp), Email: types.BoolValue(e.Email)}
		}
	}
	events, d := types.MapValueFrom(ctx, types.ObjectType{AttrTypes: eventAttrTypes}, out)
	diags.Append(d...)
	m.Events = events
	return diags
}

func (r *notificationPreferencesResource) apply(ctx context.Context, plan *notificationPreferencesModel) diag.Diagnostics {
	req, diags := r.desired(ctx, *plan)
	if diags.HasError() {
		return diags
	}
	out, err := r.client.UpdateNotificationPreferences(ctx, req)
	if err != nil {
		diags.AddError("Error updating notification preferences", err.Error())
		return diags
	}
	diags.Append(applyNotificationPreferences(ctx, out, plan)...)
	return diags
}

func (r *notificationPreferencesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan notificationPreferencesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *notificationPreferencesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state notificationPreferencesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.GetNotificationPreferences(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading notification preferences", err.Error())
		return
	}
	resp.Diagnostics.Append(applyNotificationPreferences(ctx, out, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *notificationPreferencesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan notificationPreferencesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *notificationPreferencesResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *notificationPreferencesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
