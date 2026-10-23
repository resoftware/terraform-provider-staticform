package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

var (
	_ datasource.DataSource              = (*notificationPreferencesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*notificationPreferencesDataSource)(nil)
)

// NewNotificationPreferencesDataSource constructs the staticform_notification_preferences data source.
func NewNotificationPreferencesDataSource() datasource.DataSource {
	return &notificationPreferencesDataSource{}
}

type notificationPreferencesDataSource struct{ client *client.Client }

type notificationPreferencesDataModel struct {
	EmailEnabled    types.Bool   `tfsdk:"email_enabled"`
	EmailMode       types.String `tfsdk:"email_mode"`
	DigestHourLocal types.Int64  `tfsdk:"digest_hour_local"`
	TimeZoneID      types.String `tfsdk:"time_zone_id"`
	QuietHours      types.Object `tfsdk:"quiet_hours"`
	Events          types.List   `tfsdk:"events"`
}

type notificationEventDataModel struct {
	Key            types.String `tfsdk:"key"`
	Category       types.String `tfsdk:"category"`
	OptOutAllowed  types.Bool   `tfsdk:"opt_out_allowed"`
	EmailByDefault types.Bool   `tfsdk:"email_by_default"`
	InApp          types.Bool   `tfsdk:"in_app"`
	Email          types.Bool   `tfsdk:"email"`
	SampleTitle    types.String `tfsdk:"sample_title"`
}

var notificationEventDataAttrTypes = map[string]attr.Type{
	"key":              types.StringType,
	"category":         types.StringType,
	"opt_out_allowed":  types.BoolType,
	"email_by_default": types.BoolType,
	"in_app":           types.BoolType,
	"email":            types.BoolType,
	"sample_title":     types.StringType,
}

func (d *notificationPreferencesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_preferences"
}

func (d *notificationPreferencesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *notificationPreferencesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "The signed-in user's notification preferences and the full notification event catalog (read-only). " +
			"Requires the `User.Notifications.GetPreferences` permission.",
		Attributes: map[string]dschema.Attribute{
			"email_enabled":     dschema.BoolAttribute{Computed: true, MarkdownDescription: "Master email switch."},
			"email_mode":        dschema.StringAttribute{Computed: true, MarkdownDescription: "`Immediate` or `DailyDigest`."},
			"digest_hour_local": dschema.Int64Attribute{Computed: true, MarkdownDescription: "Hour of the day (0 to 23, in `time_zone_id`) at which the daily digest is sent."},
			"time_zone_id":      dschema.StringAttribute{Computed: true, MarkdownDescription: "Time zone used for the digest hour and quiet hours."},
			"quiet_hours": dschema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Daily window in which immediate email is held. Null when there are no quiet hours.",
				Attributes: map[string]dschema.Attribute{
					"start": dschema.StringAttribute{Computed: true, MarkdownDescription: "Start of the window, formatted `HH:mm`."},
					"end":   dschema.StringAttribute{Computed: true, MarkdownDescription: "End of the window, formatted `HH:mm`."},
				},
			},
			"events": dschema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Every notification event with the user's choice, ordered by category and key.",
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"key":              dschema.StringAttribute{Computed: true, MarkdownDescription: "Event key, for example `product.StorageWarning`."},
						"category":         dschema.StringAttribute{Computed: true, MarkdownDescription: "`Account`, `System`, `Forms`, `Submissions`, `Billing`, `Integrations`, or `Attention`."},
						"opt_out_allowed":  dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the user may change this event's choices."},
						"email_by_default": dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the event is emailed unless the user chose otherwise."},
						"in_app":           dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the event appears in the in-app inbox."},
						"email":            dschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the event is also emailed."},
						"sample_title":     dschema.StringAttribute{Computed: true, MarkdownDescription: "Example title of the notification."},
					},
				},
			},
		},
	}
}

func (d *notificationPreferencesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	prefs, err := d.client.GetNotificationPreferences(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading notification preferences", err.Error())
		return
	}
	state := notificationPreferencesDataModel{
		EmailEnabled:    types.BoolValue(prefs.EmailEnabled),
		EmailMode:       types.StringValue(prefs.EmailMode),
		DigestHourLocal: types.Int64Value(int64(prefs.DigestHourLocal)),
		TimeZoneID:      types.StringValue(prefs.TimeZoneID),
		QuietHours:      types.ObjectNull(quietHoursAttrTypes),
	}
	if prefs.QuietHours != nil {
		obj, diags := types.ObjectValueFrom(ctx, quietHoursAttrTypes, quietHoursModel{
			Start: types.StringValue(normalizeQuietTime(prefs.QuietHours.Start)),
			End:   types.StringValue(normalizeQuietTime(prefs.QuietHours.End)),
		})
		resp.Diagnostics.Append(diags...)
		state.QuietHours = obj
	}
	events := make([]notificationEventDataModel, 0, len(prefs.Events))
	for _, e := range prefs.Events {
		events = append(events, notificationEventDataModel{
			Key:            types.StringValue(e.Key),
			Category:       types.StringValue(e.Category),
			OptOutAllowed:  types.BoolValue(e.OptOutAllowed),
			EmailByDefault: types.BoolValue(e.EmailByDefault),
			InApp:          types.BoolValue(e.InApp),
			Email:          types.BoolValue(e.Email),
			SampleTitle:    types.StringValue(e.SampleTitle),
		})
	}
	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: notificationEventDataAttrTypes}, events)
	resp.Diagnostics.Append(diags...)
	state.Events = list
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
