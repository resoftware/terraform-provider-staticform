// Package provider implements the StaticForm Terraform/OpenTofu provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

// Ensure StaticFormProvider satisfies the provider.Provider interface.
var _ provider.Provider = (*StaticFormProvider)(nil)

// StaticFormProvider is the provider implementation.
type StaticFormProvider struct {
	version string
}

// StaticFormProviderModel maps provider schema config to Go types.
type StaticFormProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

// New returns a function that constructs the provider for a given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &StaticFormProvider{version: version}
	}
}

func (p *StaticFormProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "staticform"
	resp.Version = p.version
}

func (p *StaticFormProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage [StaticForm](https://staticform.app) forms, API keys, and integrations as code. " +
			"The provider authenticates with a StaticForm API key (`sf_live_` or `sf_test_`).",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "StaticForm API key. May also be set with the `STATICFORM_API_KEY` environment variable. " +
					"Create one under Settings → API Keys in the dashboard.",
				Optional:  true,
				Sensitive: true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Base URL of the StaticForm API. Defaults to `https://api.staticform.app`. " +
					"May also be set with the `STATICFORM_BASE_URL` environment variable.",
				Optional: true,
			},
		},
	}
}

func (p *StaticFormProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg StaticFormProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("STATICFORM_API_KEY")
	if !cfg.APIKey.IsNull() && cfg.APIKey.ValueString() != "" {
		apiKey = cfg.APIKey.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing StaticForm API key",
			"Set the api_key provider argument or the STATICFORM_API_KEY environment variable.",
		)
		return
	}

	baseURL := os.Getenv("STATICFORM_BASE_URL")
	if !cfg.BaseURL.IsNull() && cfg.BaseURL.ValueString() != "" {
		baseURL = cfg.BaseURL.ValueString()
	}

	c := client.New(baseURL, apiKey, client.WithUserAgent("terraform-provider-staticform/"+p.version))
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *StaticFormProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewFormResource,
		NewAPIKeyResource,
		NewSMTPConnectionResource,
		NewEmailDomainResource,
		NewCollaboratorResource,
		NewUserSettingsResource,
		NewNotionConnectionResource,
		NewStripeConnectionResource,
	}
}

func (p *StaticFormProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewFormDataSource,
		NewFormsDataSource,
		NewSubmissionsDataSource,
		NewAPIKeyPermissionsDataSource,
		NewGoogleConnectionsDataSource,
		NewNotionConnectionsDataSource,
		NewStripeConnectionsDataSource,
		NewSubscriptionDataSource,
		NewUserDataSource,
		NewExecutionLogsDataSource,
	}
}
