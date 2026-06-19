package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

var (
	_ resource.Resource                = (*notionConnectionResource)(nil)
	_ resource.ResourceWithConfigure   = (*notionConnectionResource)(nil)
	_ resource.ResourceWithImportState = (*notionConnectionResource)(nil)
)

// NewNotionConnectionResource constructs the staticform_notion_connection resource.
func NewNotionConnectionResource() resource.Resource { return &notionConnectionResource{} }

type notionConnectionResource struct {
	client *client.Client
}

type notionConnectionModel struct {
	ID        types.String `tfsdk:"id"`
	Label     types.String `tfsdk:"label"`
	APIKey    types.String `tfsdk:"api_key"`
	Revoked   types.Bool   `tfsdk:"revoked"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func (r *notionConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notion_connection"
}

func (r *notionConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *notionConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	rr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Notion integration connection, created from an internal integration token. Referenced by `SyncNotionDatabaseAction` submit actions via its `id`. Attributes are immutable; changing any forces a new connection.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"label":      schema.StringAttribute{Required: true, PlanModifiers: rr, MarkdownDescription: "Human-readable label."},
			"api_key":    schema.StringAttribute{Required: true, Sensitive: true, PlanModifiers: rr, MarkdownDescription: "Notion internal integration token. Validated against Notion on create; stored encrypted, never returned."},
			"revoked":    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the connection has been revoked."},
			"created_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *notionConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan notionConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.CreateNotionConnection(ctx, plan.Label.ValueString(), plan.APIKey.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating Notion connection", err.Error())
		return
	}
	plan.ID = types.StringValue(out.ID)
	plan.CreatedAt = types.StringValue(out.CreatedAtUTC)
	plan.Revoked = types.BoolValue(out.Revoked)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *notionConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state notionConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.GetNotionConnection(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Notion connection", err.Error())
		return
	}
	state.Label = types.StringValue(out.Label)
	state.Revoked = types.BoolValue(out.Revoked)
	state.CreatedAt = types.StringValue(out.CreatedAtUTC)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *notionConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan notionConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *notionConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state notionConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteNotionConnection(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Notion connection", err.Error())
	}
}

func (r *notionConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
