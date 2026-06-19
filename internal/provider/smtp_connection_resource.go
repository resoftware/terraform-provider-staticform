package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

var (
	_ resource.Resource                = (*smtpConnectionResource)(nil)
	_ resource.ResourceWithConfigure   = (*smtpConnectionResource)(nil)
	_ resource.ResourceWithImportState = (*smtpConnectionResource)(nil)
)

// NewSMTPConnectionResource constructs the staticform_smtp_connection resource.
func NewSMTPConnectionResource() resource.Resource { return &smtpConnectionResource{} }

type smtpConnectionResource struct {
	client *client.Client
}

type smtpConnectionResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Label     types.String `tfsdk:"label"`
	Provider  types.String `tfsdk:"provider_preset"`
	Host      types.String `tfsdk:"host"`
	Port      types.Int64  `tfsdk:"port"`
	Security  types.String `tfsdk:"security"`
	Username  types.String `tfsdk:"username"`
	Password  types.String `tfsdk:"password"`
	FromEmail types.String `tfsdk:"from_email"`
	FromName  types.String `tfsdk:"from_name"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func (r *smtpConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smtp_connection"
}

func (r *smtpConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *smtpConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	rr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "A bring-your-own SMTP connection for sending form emails. The server live-tests the credentials on create. Attributes are immutable; changing any forces a new connection.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"label":           schema.StringAttribute{Required: true, PlanModifiers: rr, MarkdownDescription: "Human-readable label."},
			"provider_preset": schema.StringAttribute{Required: true, PlanModifiers: rr, Validators: []validator.String{stringvalidator.OneOf("CustomSmtp", "AwsSes", "Mailgun", "SendGrid", "Postmark", "Resend", "Brevo", "Mailjet", "ZohoMail", "GoogleWorkspace", "Microsoft365", "Mailtrap", "Smtp2Go")}, MarkdownDescription: "Provider preset: `CustomSmtp`, `AwsSes`, `Mailgun`, `SendGrid`, `Postmark`, `Resend`, `Brevo`, `Mailjet`, `ZohoMail`, `GoogleWorkspace`, `Microsoft365`, `Mailtrap`, `Smtp2Go`."},
			"host":            schema.StringAttribute{Required: true, PlanModifiers: rr, MarkdownDescription: "SMTP host."},
			"port":            schema.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}, Validators: []validator.Int64{int64validator.Between(1, 65535)}, MarkdownDescription: "SMTP port (1-65535)."},
			"security":        schema.StringAttribute{Required: true, PlanModifiers: rr, Validators: []validator.String{stringvalidator.OneOf("None", "StartTls", "SslOnConnect")}, MarkdownDescription: "`None`, `StartTls`, or `SslOnConnect`."},
			"username":        schema.StringAttribute{Required: true, PlanModifiers: rr, MarkdownDescription: "SMTP username."},
			"password":        schema.StringAttribute{Required: true, Sensitive: true, PlanModifiers: rr, MarkdownDescription: "SMTP password. Stored in state, never returned by the API."},
			"from_email":      schema.StringAttribute{Required: true, PlanModifiers: rr, MarkdownDescription: "From email address."},
			"from_name":       schema.StringAttribute{Required: true, PlanModifiers: rr, MarkdownDescription: "From display name."},
			"created_at":      schema.StringAttribute{Computed: true, MarkdownDescription: "Creation timestamp."},
		},
	}
}

func (r *smtpConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan smtpConnectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := r.client.CreateSMTPConnection(ctx, client.SMTPConnectionRequest{
		Label:     plan.Label.ValueString(),
		Provider:  plan.Provider.ValueString(),
		Host:      plan.Host.ValueString(),
		Port:      int(plan.Port.ValueInt64()),
		Security:  plan.Security.ValueString(),
		Username:  plan.Username.ValueString(),
		Password:  plan.Password.ValueString(),
		FromEmail: plan.FromEmail.ValueString(),
		FromName:  plan.FromName.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating SMTP connection", err.Error())
		return
	}

	plan.ID = types.StringValue(out.ID)
	plan.CreatedAt = types.StringValue(out.CreatedAtUTC)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *smtpConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state smtpConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := r.client.GetSMTPConnection(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading SMTP connection", err.Error())
		return
	}

	state.Label = types.StringValue(out.Label)
	state.Provider = types.StringValue(out.Provider)
	state.Host = types.StringValue(out.Host)
	state.Port = types.Int64Value(int64(out.Port))
	state.Security = types.StringValue(out.Security)
	state.FromEmail = types.StringValue(out.FromEmail)
	state.FromName = types.StringValue(out.FromName)
	state.CreatedAt = types.StringValue(out.CreatedAtUTC)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *smtpConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All configurable attributes force replacement, so Update is never invoked
	// with real changes; just persist the plan.
	var plan smtpConnectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *smtpConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state smtpConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSMTPConnection(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting SMTP connection", err.Error())
	}
}

func (r *smtpConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
