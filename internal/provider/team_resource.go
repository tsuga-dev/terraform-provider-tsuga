package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_team"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*teamResource)(nil)
var _ resource.ResourceWithConfigure = (*teamResource)(nil)
var _ resource.ResourceWithImportState = (*dashboardResource)(nil)

func NewTeamResource() resource.Resource {
	return &teamResource{}
}

type teamResource struct {
	client *TsugaClient
}

func (r *teamResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*TsugaClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *TsugaClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *teamResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (r *teamResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_team.TeamResourceSchema(ctx)
}

func (r *teamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *teamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_team.TeamModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := map[string]interface{}{
		"name":       plan.Name.ValueString(),
		"visibility": plan.Visibility.ValueString(),
	}

	if !plan.Description.IsNull() {
		requestBody["description"] = plan.Description.ValueString()
	}

	if tags, diags := expandTags(ctx, plan.Tags); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	} else if tags != nil {
		requestBody["tags"] = tags
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPost, "/v1/teams", requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create team: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create team: %s", err))
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

	plan.Id = types.StringValue(apiResp.Data.ID)
	plan.Name = types.StringValue(apiResp.Data.Name)
	plan.Visibility = types.StringValue(apiResp.Data.Visibility)

	if apiResp.Data.Description != "" {
		plan.Description = types.StringValue(apiResp.Data.Description)
	} else {
		plan.Description = types.StringNull()
	}

	if tags, diags := flattenTags(ctx, apiResp.Data.Tags); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	} else {
		plan.Tags = tags
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_team.TeamModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/teams/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read team: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
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

	state.Id = types.StringValue(apiResp.Data.ID)
	state.Name = types.StringValue(apiResp.Data.Name)
	state.Visibility = types.StringValue(apiResp.Data.Visibility)

	if apiResp.Data.Description != "" {
		state.Description = types.StringValue(apiResp.Data.Description)
	} else {
		state.Description = types.StringNull()
	}

	if tags, diags := flattenTags(ctx, apiResp.Data.Tags); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	} else {
		state.Tags = tags
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *teamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_team.TeamModel
	var state resource_team.TeamModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := map[string]interface{}{
		"name":       plan.Name.ValueString(),
		"visibility": plan.Visibility.ValueString(),
	}

	if !plan.Description.IsNull() {
		requestBody["description"] = plan.Description.ValueString()
	}

	if tags, diags := expandTags(ctx, plan.Tags); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	} else if tags != nil {
		requestBody["tags"] = tags
	}

	path := fmt.Sprintf("/v1/teams/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodPut, path, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update team: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update team: %s", err))
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

	plan.Id = types.StringValue(apiResp.Data.ID)
	plan.Name = types.StringValue(apiResp.Data.Name)
	plan.Visibility = types.StringValue(apiResp.Data.Visibility)

	if apiResp.Data.Description != "" {
		plan.Description = types.StringValue(apiResp.Data.Description)
	} else {
		plan.Description = types.StringNull()
	}
	if tags, diags := flattenTags(ctx, apiResp.Data.Tags); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	} else {
		plan.Tags = tags
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_team.TeamModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/teams/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete team: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete team: %s", err))
			return
		}
	}
}

type teamAPIResponse struct {
	Data struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Visibility  string   `json:"visibility"`
		Tags        []apiTag `json:"tags"`
	} `json:"data"`
}
