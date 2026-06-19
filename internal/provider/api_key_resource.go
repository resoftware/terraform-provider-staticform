package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

var (
	_ resource.Resource                = (*apiKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*apiKeyResource)(nil)
	_ resource.ResourceWithImportState = (*apiKeyResource)(nil)
)

// NewAPIKeyResource constructs the staticform_api_key resource.
func NewAPIKeyResource() resource.Resource { return &apiKeyResource{} }

type apiKeyResource struct {
	client *client.Client
}

type apiKeyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	Permissions types.Set    `tfsdk:"permissions"`
	FormIDs     types.Set    `tfsdk:"form_ids"`
	Key         types.String `tfsdk:"key"`
	KeyHint     types.String `tfsdk:"key_hint"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A StaticForm API key. The plaintext key is returned only at creation time and stored in state; mark your state as sensitive accordingly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 100)}, MarkdownDescription: "Human-readable name for the key."},
			"expires_at": schema.StringAttribute{
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "Optional RFC 3339 expiration timestamp (e.g. `2027-01-01T00:00:00Z`). Omit for a non-expiring key. Immutable; changing it forces a new key.",
			},
			"permissions": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Permission names to scope the key to. Omit for all of the creating user's permissions. Updatable in place.",
				PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"form_ids": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Form IDs to scope the key to. Omit for all of the user's forms. Updatable in place.",
				PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
			"key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The plaintext API key. Available only immediately after creation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"key_hint":   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Last characters of the key, for identification."},
			"created_at": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Creation timestamp."},
		},
	}
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	perms, d := stringSet(ctx, plan.Permissions)
	resp.Diagnostics.Append(d...)
	formIDs, d2 := stringSet(ctx, plan.FormIDs)
	resp.Diagnostics.Append(d2...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := client.APIKeyRequest{
		Name:        plan.Name.ValueString(),
		Permissions: perms,
		FormIDs:     formIDs,
	}
	if !plan.ExpiresAt.IsNull() && plan.ExpiresAt.ValueString() != "" {
		body.ExpiresAtUTC = strPtr(plan.ExpiresAt.ValueString())
	}

	key, err := r.client.CreateAPIKey(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating API key", err.Error())
		return
	}

	plan.ID = types.StringValue(key.ID)
	plan.Key = types.StringValue(key.APIKey)
	plan.KeyHint = types.StringValue(key.KeyHint)
	plan.CreatedAt = types.StringValue(key.CreatedAtUTC)
	plan.ExpiresAt = normalizeExpiry(plan.ExpiresAt, key.ExpiresAtUTC)
	plan.Permissions = toStringSet(key.Permissions)
	plan.FormIDs = toStringSet(key.FormIDs)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.client.GetAPIKey(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading API key", err.Error())
		return
	}

	state.Name = types.StringValue(key.Name)
	state.KeyHint = types.StringValue(key.KeyHint)
	state.CreatedAt = types.StringValue(key.CreatedAtUTC)
	state.ExpiresAt = normalizeExpiry(state.ExpiresAt, key.ExpiresAtUTC)
	state.Permissions = toStringSet(key.Permissions)
	state.FormIDs = toStringSet(key.FormIDs)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	perms, d := stringSet(ctx, plan.Permissions)
	resp.Diagnostics.Append(d...)
	formIDs, d2 := stringSet(ctx, plan.FormIDs)
	resp.Diagnostics.Append(d2...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := client.APIKeyUpdateRequest{
		Name:        plan.Name.ValueString(),
		Permissions: perms,
		FormIDs:     formIDs,
	}
	if err := r.client.UpdateAPIKey(ctx, plan.ID.ValueString(), body); err != nil {
		resp.Diagnostics.AddError("Error updating API key", err.Error())
		return
	}

	plan.Permissions = toStringSet(perms)
	plan.FormIDs = toStringSet(formIDs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.RevokeAPIKey(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error revoking API key", err.Error())
	}
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// normalizeExpiry keeps the configured value when the server echoes the same
// instant, avoiding spurious diffs from timestamp formatting differences. When
// the configured value is null it falls back to the server value.
func normalizeExpiry(configured types.String, serverVal *string) types.String {
	if serverVal == nil {
		if configured.IsNull() {
			return types.StringNull()
		}
		return configured
	}
	if !configured.IsNull() && configured.ValueString() != "" {
		return configured
	}
	return types.StringValue(*serverVal)
}
