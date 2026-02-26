package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = (*tsugaProvider)(nil)

func New(version, commit, date string) func() provider.Provider {
	return func() provider.Provider {
		return &tsugaProvider{
			version: version,
			commit:  commit,
			date:    date,
		}
	}
}

type tsugaProvider struct {
	version string
	commit  string
	date    string
}

type tsugaProviderModel struct {
	BaseURL types.String `tfsdk:"base_url"`
	Token   types.String `tfsdk:"token"`
}

func (p *tsugaProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "The base URL for the Tsuga API. Defaults to TSUGA_BASE_URL environment variable, or https://api.tsuga.com if not set.",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Bearer token for API authentication. Defaults to TSUGA_TOKEN environment variable.",
			},
		},
	}
}

func (p *tsugaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config tsugaProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Default to environment variables if not set in config
	baseURL := os.Getenv("TSUGA_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.tsuga.com"
	}
	token := os.Getenv("TSUGA_TOKEN")

	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}

	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}

	if baseURL == "" {
		resp.Diagnostics.AddError(
			"Missing API Base URL",
			"The provider cannot create the Tsuga API client as the API base URL is explicitly set to an empty value. "+
				"Remove the base_url configuration to use the default (https://api.tsuga.com), or set it to a valid URL.",
		)
	}

	if token == "" {
		resp.Diagnostics.AddError(
			"Missing API Token",
			"The provider cannot create the Tsuga API client as there is a missing or empty value for the API token. "+
				"Set the token value in the configuration or use the TSUGA_TOKEN environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create and configure the client
	client := &TsugaClient{
		BaseURL: baseURL,
		Token:   token,
		Version: p.version,
		Commit:  p.commit,
		Date:    p.date,
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *tsugaProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "tsuga"
}

func (p *tsugaProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewTeamDataSource,
		NewUserDataSource,
	}
}

func (p *tsugaProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewIngestionApiKeyResource,
		NewTeamResource,
		NewTeamMembershipResource,
		NewNotificationRuleResource,
		NewNotificationSilenceResource,
		NewDashboardResource,
		NewRouteResource,
		NewMonitorResource,
		NewRetentionPolicyResource,
		NewTagPolicyResource,
	}
}
