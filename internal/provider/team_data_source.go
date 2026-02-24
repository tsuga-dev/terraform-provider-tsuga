package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"terraform-provider-tsuga/internal/datasource_team"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*teamDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*teamDataSource)(nil)

func NewTeamDataSource() datasource.DataSource {
	return &teamDataSource{}
}

type teamDataSource struct {
	client *TsugaClient
}

func (d *teamDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*TsugaClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *TsugaClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *teamDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (d *teamDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasource_team.TeamDataSourceSchema(ctx)
}

func (d *teamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config datasource_team.TeamModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/teams/%s", url.PathEscape(config.Id.ValueString()))
	httpResp, err := d.client.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read team: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.Diagnostics.AddError("Team not found", fmt.Sprintf("No team was found with id %q.", config.Id.ValueString()))
		return
	}

	if err := d.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read team: %s", err))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp teamAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	config.Id = types.StringValue(apiResp.Data.ID)
	config.Name = types.StringValue(apiResp.Data.Name)
	config.Visibility = types.StringValue(apiResp.Data.Visibility)

	if apiResp.Data.Description == "" {
		config.Description = types.StringNull()
	} else {
		config.Description = types.StringValue(apiResp.Data.Description)
	}

	if tags, diags := flattenTags(ctx, apiResp.Data.Tags); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	} else {
		config.Tags = tags
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
