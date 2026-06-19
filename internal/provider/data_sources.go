package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

// ── staticform_form (single) ────────────────────────────────────────────────

var (
	_ datasource.DataSource              = (*formDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*formDataSource)(nil)
)

// NewFormDataSource constructs the staticform_form data source.
func NewFormDataSource() datasource.DataSource { return &formDataSource{} }

type formDataSource struct{ client *client.Client }

type formDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	SubmissionMode types.String `tfsdk:"submission_mode"`
}

func (d *formDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_form"
}

func (d *formDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *formDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Look up an existing StaticForm form by ID.",
		Attributes: map[string]dschema.Attribute{
			"id":              dschema.StringAttribute{Required: true, MarkdownDescription: "Form ID."},
			"name":            dschema.StringAttribute{Computed: true, MarkdownDescription: "Form name."},
			"submission_mode": dschema.StringAttribute{Computed: true, MarkdownDescription: "`ClientSide` or `ServerSide`."},
		},
	}
}

func (d *formDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg formDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	form, err := d.client.GetForm(ctx, cfg.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading form", err.Error())
		return
	}
	cfg.Name = types.StringValue(form.Name)
	cfg.SubmissionMode = types.StringValue(form.SubmissionMode)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// ── staticform_forms (list) ─────────────────────────────────────────────────

var (
	_ datasource.DataSource              = (*formsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*formsDataSource)(nil)
)

// NewFormsDataSource constructs the staticform_forms data source.
func NewFormsDataSource() datasource.DataSource { return &formsDataSource{} }

type formsDataSource struct{ client *client.Client }

type formListItemModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type formsDataSourceModel struct {
	Forms []formListItemModel `tfsdk:"forms"`
}

func (d *formsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_forms"
}

func (d *formsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *formsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "List all forms accessible to the API key.",
		Attributes: map[string]dschema.Attribute{
			"forms": dschema.ListNestedAttribute{
				Computed: true,
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"id":   dschema.StringAttribute{Computed: true},
						"name": dschema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *formsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	forms, err := d.client.ListForms(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing forms", err.Error())
		return
	}
	var state formsDataSourceModel
	for _, f := range forms {
		state.Forms = append(state.Forms, formListItemModel{
			ID:   types.StringValue(f.ID),
			Name: types.StringValue(f.Name),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ── staticform_submissions ──────────────────────────────────────────────────

var (
	_ datasource.DataSource              = (*submissionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*submissionsDataSource)(nil)
)

// NewSubmissionsDataSource constructs the staticform_submissions data source.
func NewSubmissionsDataSource() datasource.DataSource { return &submissionsDataSource{} }

type submissionsDataSource struct{ client *client.Client }

type submissionModel struct {
	ID        types.String `tfsdk:"id"`
	Data      types.Map    `tfsdk:"data"`
	CreatedAt types.String `tfsdk:"created_at"`
	IsRead    types.Bool   `tfsdk:"is_read"`
}

type submissionsDataSourceModel struct {
	FormID      types.String      `tfsdk:"form_id"`
	Submissions []submissionModel `tfsdk:"submissions"`
}

func (d *submissionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_submissions"
}

func (d *submissionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *submissionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "Read the (non-spam) submissions of a form. Read-only.",
		Attributes: map[string]dschema.Attribute{
			"form_id": dschema.StringAttribute{Required: true, MarkdownDescription: "Form ID to read submissions from."},
			"submissions": dschema.ListNestedAttribute{
				Computed: true,
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"id":         dschema.StringAttribute{Computed: true},
						"data":       dschema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Submitted field values."},
						"created_at": dschema.StringAttribute{Computed: true},
						"is_read":    dschema.BoolAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *submissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg submissionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	subs, err := d.client.ListSubmissions(ctx, cfg.FormID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing submissions", err.Error())
		return
	}
	cfg.Submissions = []submissionModel{}
	for _, s := range subs {
		data := map[string]string{}
		for k, v := range s.Data {
			if v != nil {
				data[k] = *v
			} else {
				data[k] = ""
			}
		}
		cfg.Submissions = append(cfg.Submissions, submissionModel{
			ID:        types.StringValue(s.ID),
			Data:      toStringMap(data),
			CreatedAt: types.StringValue(s.CreatedAtUTC),
			IsRead:    types.BoolValue(s.IsRead),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// ── staticform_api_key_permissions ──────────────────────────────────────────

var (
	_ datasource.DataSource              = (*apiKeyPermissionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*apiKeyPermissionsDataSource)(nil)
)

// NewAPIKeyPermissionsDataSource constructs the staticform_api_key_permissions data source.
func NewAPIKeyPermissionsDataSource() datasource.DataSource { return &apiKeyPermissionsDataSource{} }

type apiKeyPermissionsDataSource struct{ client *client.Client }

type apiKeyPermissionsModel struct {
	Names types.List `tfsdk:"names"`
}

func (d *apiKeyPermissionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key_permissions"
}

func (d *apiKeyPermissionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *apiKeyPermissionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		MarkdownDescription: "The permission names that can be granted to a `staticform_api_key`.",
		Attributes: map[string]dschema.Attribute{
			"names": dschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Available permission names."},
		},
	}
}

func (d *apiKeyPermissionsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	perms, err := d.client.ListAPIKeyPermissions(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing permissions", err.Error())
		return
	}
	state := apiKeyPermissionsModel{Names: toStringList(perms)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
