package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

// domainRegexp is a lenient sanity check; the server is the authority on validity.
var domainRegexp = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

var (
	_ resource.Resource                = (*emailDomainResource)(nil)
	_ resource.ResourceWithConfigure   = (*emailDomainResource)(nil)
	_ resource.ResourceWithImportState = (*emailDomainResource)(nil)
)

// NewEmailDomainResource constructs the staticform_email_domain resource.
func NewEmailDomainResource() resource.Resource { return &emailDomainResource{} }

type emailDomainResource struct {
	client *client.Client
}

type dnsRecordModel struct {
	Name  types.String `tfsdk:"name"`
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

// dnsRecordObjectType is the element type for the computed DNS-record lists. The
// lists must be types.List (not a Go slice) so they can hold the "unknown" value
// the framework assigns to computed attributes during planning.
var dnsRecordObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"name":  types.StringType,
	"type":  types.StringType,
	"value": types.StringType,
}}

type emailDomainResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	DomainName           types.String `tfsdk:"domain_name"`
	MailFromSubdomain    types.String `tfsdk:"mail_from_subdomain"`
	Status               types.String `tfsdk:"status"`
	DkimRecords          types.List   `tfsdk:"dkim_records"`
	MailFromDomain       types.String `tfsdk:"mail_from_domain"`
	MailFromRecords      types.List   `tfsdk:"mail_from_records"`
	SuggestedDmarcRecord types.String `tfsdk:"suggested_dmarc_record"`
	DmarcStatus          types.String `tfsdk:"dmarc_status"`
	CreatedAt            types.String `tfsdk:"created_at"`
}

func (r *emailDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_domain"
}

func (r *emailDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *emailDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	dnsAttr := schema.ListNestedAttribute{
		Computed: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"name":  schema.StringAttribute{Computed: true},
				"type":  schema.StringAttribute{Computed: true},
				"value": schema.StringAttribute{Computed: true},
			},
		},
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "A managed sending domain (Amazon SES). Publish the returned DKIM records in DNS to verify it. Verification is asynchronous: apply does not block on it; the `status` attribute reflects the latest state on refresh.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"domain_name": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.RegexMatches(domainRegexp, "must be a valid domain name, e.g. example.com")},
				MarkdownDescription: "Domain to send from, e.g. `example.com`.",
			},
			"mail_from_subdomain": schema.StringAttribute{
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "MAIL FROM subdomain label (defaults to `bounces`). Empty string skips MAIL FROM configuration.",
			},
			"status":                 schema.StringAttribute{Computed: true, MarkdownDescription: "`NotStarted`, `Pending`, `Verified`, `Failed`, or `TemporaryFailure`."},
			"dkim_records":           dnsAttr,
			"mail_from_domain":       schema.StringAttribute{Computed: true},
			"mail_from_records":      dnsAttr,
			"suggested_dmarc_record": schema.StringAttribute{Computed: true},
			"dmarc_status":           schema.StringAttribute{Computed: true, MarkdownDescription: "`Unchecked`, `Found`, `Missing`, or `LookupFailed`."},
			"created_at":             schema.StringAttribute{Computed: true},
		},
	}
}

func (r *emailDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan emailDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := client.EmailDomainRequest{DomainName: plan.DomainName.ValueString()}
	if !plan.MailFromSubdomain.IsNull() {
		v := plan.MailFromSubdomain.ValueString()
		body.MailFromSubdomain = &v
	}

	out, err := r.client.CreateEmailDomain(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating email domain", err.Error())
		return
	}

	applyEmailDomain(out, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *emailDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state emailDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := r.client.GetEmailDomain(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading email domain", err.Error())
		return
	}

	applyEmailDomain(out, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *emailDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan emailDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *emailDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state emailDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteEmailDomain(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting email domain", err.Error())
	}
}

func (r *emailDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applyEmailDomain(d *client.EmailDomain, m *emailDomainResourceModel) {
	m.ID = types.StringValue(d.ID)
	m.DomainName = types.StringValue(d.DomainName)
	m.Status = types.StringValue(d.Status)
	m.MailFromDomain = strFromPtr(d.MailFromDomain)
	m.SuggestedDmarcRecord = strFromPtr(d.SuggestedDmarcRecord)
	m.DmarcStatus = types.StringValue(d.DmarcStatus)
	m.CreatedAt = types.StringValue(d.CreatedAtUTC)
	m.DkimRecords = dnsRecordsToList(d.DkimRecords)
	m.MailFromRecords = dnsRecordsToList(d.MailFromRecords)
}

func dnsRecordsToList(in []client.DNSRecord) types.List {
	out := make([]dnsRecordModel, len(in))
	for i, r := range in {
		out[i] = dnsRecordModel{
			Name:  types.StringValue(r.Name),
			Type:  nullIfEmpty(r.Type),
			Value: types.StringValue(r.Value),
		}
	}
	l, _ := types.ListValueFrom(context.Background(), dnsRecordObjectType, out)
	return l
}
