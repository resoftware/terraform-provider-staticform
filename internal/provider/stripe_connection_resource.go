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
	_ resource.Resource                = (*stripeConnectionResource)(nil)
	_ resource.ResourceWithConfigure   = (*stripeConnectionResource)(nil)
	_ resource.ResourceWithImportState = (*stripeConnectionResource)(nil)
)

// NewStripeConnectionResource constructs the staticform_stripe_connection resource.
func NewStripeConnectionResource() resource.Resource { return &stripeConnectionResource{} }

type stripeConnectionResource struct {
	client *client.Client
}

type stripeConnectionModel struct {
	ID                types.String `tfsdk:"id"`
	Label             types.String `tfsdk:"label"`
	SecretKey         types.String `tfsdk:"secret_key"`
	WebhookSecret     types.String `tfsdk:"webhook_secret"`
	KeyMode           types.String `tfsdk:"key_mode"`
	WebhookConfigured types.Bool   `tfsdk:"webhook_configured"`
	CreatedAt         types.String `tfsdk:"created_at"`
}

func (r *stripeConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_stripe_connection"
}

func (r *stripeConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *stripeConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	rr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Stripe payment connection, created from a restricted secret key (`rk_...`). Referenced by a form's `payment` block via its `id`. The label and key are immutable; the webhook secret can be updated in place.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"label":      schema.StringAttribute{Required: true, PlanModifiers: rr, MarkdownDescription: "Human-readable label."},
			"secret_key": schema.StringAttribute{Required: true, Sensitive: true, PlanModifiers: rr, MarkdownDescription: "Stripe restricted secret key (`rk_...`). Verified against Stripe on create; stored encrypted, never returned."},
			"webhook_secret": schema.StringAttribute{
				Optional: true, Sensitive: true,
				MarkdownDescription: "Stripe webhook signing secret (`whsec_...`). Updatable in place; stored encrypted, never returned.",
			},
			"key_mode":           schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "`live` or `test`, derived from the secret key."},
			"webhook_configured": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a webhook secret is set."},
			"created_at":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *stripeConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan stripeConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.CreateStripeConnection(ctx, plan.Label.ValueString(), plan.SecretKey.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating Stripe connection", err.Error())
		return
	}
	plan.ID = types.StringValue(out.ID)
	plan.KeyMode = types.StringValue(out.KeyMode)
	plan.WebhookConfigured = types.BoolValue(false)
	plan.CreatedAt = types.StringValue(out.CreatedAtUTC)

	if !plan.WebhookSecret.IsNull() && plan.WebhookSecret.ValueString() != "" {
		if err := r.client.SetStripeWebhookSecret(ctx, out.ID, plan.WebhookSecret.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error setting Stripe webhook secret", err.Error())
			return
		}
		plan.WebhookConfigured = types.BoolValue(true)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *stripeConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state stripeConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.GetStripeConnection(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Stripe connection", err.Error())
		return
	}
	state.Label = types.StringValue(out.Label)
	state.KeyMode = types.StringValue(out.KeyMode)
	state.WebhookConfigured = types.BoolValue(out.WebhookConfigured)
	state.CreatedAt = types.StringValue(out.CreatedAtUTC)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *stripeConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan stripeConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Only webhook_secret is updatable in place.
	if !plan.WebhookSecret.IsNull() && plan.WebhookSecret.ValueString() != "" {
		if err := r.client.SetStripeWebhookSecret(ctx, plan.ID.ValueString(), plan.WebhookSecret.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error setting Stripe webhook secret", err.Error())
			return
		}
		plan.WebhookConfigured = types.BoolValue(true)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *stripeConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state stripeConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteStripeConnection(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Stripe connection", err.Error())
	}
}

func (r *stripeConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
