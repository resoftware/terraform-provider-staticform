package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

// ── staticform_google_connections ───────────────────────────────────────────

var (
	_ datasource.DataSource              = (*googleConnectionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*googleConnectionsDataSource)(nil)
)

// NewGoogleConnectionsDataSource constructs the staticform_google_connections data source.
func NewGoogleConnectionsDataSource() datasource.DataSource { return &googleConnectionsDataSource{} }

type googleConnectionsDataSource struct{ client *client.Client }

type googleConnectionItemModel struct {
	ID      types.String `tfsdk:"id"`
	Email   types.String `tfsdk:"google_account_email"`
	Revoked types.Bool   `tfsdk:"revoked"`
}

type googleConnectionsModel struct {
	Connections []googleConnectionItemModel `tfsdk:"connections"`
}

func (d *googleConnectionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_google_connections"
}

func (d *googleConnectionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *googleConnectionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "List Google (Sheets) OAuth connections. Use a connection `id` in a form's `SyncGoogleSheetsAction`. Google connections are created interactively in the dashboard.",
		Attributes: map[string]dschema.Attribute{
			"connections": dschema.ListNestedAttribute{
				Computed: true,
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"id":                   dschema.StringAttribute{Computed: true},
						"google_account_email": dschema.StringAttribute{Computed: true},
						"revoked":              dschema.BoolAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *googleConnectionsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	conns, err := d.client.ListGoogleConnections(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Google connections", err.Error())
		return
	}
	state := googleConnectionsModel{Connections: []googleConnectionItemModel{}}
	for _, c := range conns {
		state.Connections = append(state.Connections, googleConnectionItemModel{
			ID:      types.StringValue(c.ID),
			Email:   types.StringValue(c.GoogleAccountEmail),
			Revoked: types.BoolValue(c.Revoked),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ── staticform_notion_connections ───────────────────────────────────────────

var (
	_ datasource.DataSource              = (*notionConnectionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*notionConnectionsDataSource)(nil)
)

// NewNotionConnectionsDataSource constructs the staticform_notion_connections data source.
func NewNotionConnectionsDataSource() datasource.DataSource { return &notionConnectionsDataSource{} }

type notionConnectionsDataSource struct{ client *client.Client }

type notionConnItemModel struct {
	ID      types.String `tfsdk:"id"`
	Label   types.String `tfsdk:"label"`
	Revoked types.Bool   `tfsdk:"revoked"`
}

type notionConnectionsModel struct {
	Connections []notionConnItemModel `tfsdk:"connections"`
}

func (d *notionConnectionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notion_connections"
}

func (d *notionConnectionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *notionConnectionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "List Notion connections. Prefer the `staticform_notion_connection` resource to manage them; use this to reference connections created elsewhere.",
		Attributes: map[string]dschema.Attribute{
			"connections": dschema.ListNestedAttribute{
				Computed: true,
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"id":      dschema.StringAttribute{Computed: true},
						"label":   dschema.StringAttribute{Computed: true},
						"revoked": dschema.BoolAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *notionConnectionsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	conns, err := d.client.ListNotionConnections(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Notion connections", err.Error())
		return
	}
	state := notionConnectionsModel{Connections: []notionConnItemModel{}}
	for _, c := range conns {
		state.Connections = append(state.Connections, notionConnItemModel{
			ID:      types.StringValue(c.ID),
			Label:   types.StringValue(c.Label),
			Revoked: types.BoolValue(c.Revoked),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ── staticform_stripe_connections ───────────────────────────────────────────

var (
	_ datasource.DataSource              = (*stripeConnectionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*stripeConnectionsDataSource)(nil)
)

// NewStripeConnectionsDataSource constructs the staticform_stripe_connections data source.
func NewStripeConnectionsDataSource() datasource.DataSource { return &stripeConnectionsDataSource{} }

type stripeConnectionsDataSource struct{ client *client.Client }

type stripeConnItemModel struct {
	ID                types.String `tfsdk:"id"`
	Label             types.String `tfsdk:"label"`
	KeyMode           types.String `tfsdk:"key_mode"`
	WebhookConfigured types.Bool   `tfsdk:"webhook_configured"`
	Revoked           types.Bool   `tfsdk:"revoked"`
}

type stripeConnectionsModel struct {
	Connections []stripeConnItemModel `tfsdk:"connections"`
}

func (d *stripeConnectionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stripe_connections"
}

func (d *stripeConnectionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *stripeConnectionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "List Stripe connections. Prefer the `staticform_stripe_connection` resource to manage them; use this to reference connections created elsewhere.",
		Attributes: map[string]dschema.Attribute{
			"connections": dschema.ListNestedAttribute{
				Computed: true,
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"id":                 dschema.StringAttribute{Computed: true},
						"label":              dschema.StringAttribute{Computed: true},
						"key_mode":           dschema.StringAttribute{Computed: true},
						"webhook_configured": dschema.BoolAttribute{Computed: true},
						"revoked":            dschema.BoolAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *stripeConnectionsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	conns, err := d.client.ListStripeConnections(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Stripe connections", err.Error())
		return
	}
	state := stripeConnectionsModel{Connections: []stripeConnItemModel{}}
	for _, c := range conns {
		state.Connections = append(state.Connections, stripeConnItemModel{
			ID:                types.StringValue(c.ID),
			Label:             types.StringValue(c.Label),
			KeyMode:           types.StringValue(c.KeyMode),
			WebhookConfigured: types.BoolValue(c.WebhookConfigured),
			Revoked:           types.BoolValue(c.Revoked),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
