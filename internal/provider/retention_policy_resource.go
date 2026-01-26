package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"terraform-provider-tsuga/internal/resource_retention_policy"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*retentionPolicyResource)(nil)
var _ resource.ResourceWithConfigure = (*retentionPolicyResource)(nil)
var _ resource.ResourceWithImportState = (*retentionPolicyResource)(nil)

func NewRetentionPolicyResource() resource.Resource {
	return &retentionPolicyResource{}
}

type retentionPolicyResource struct {
	client *TsugaClient
}

func (r *retentionPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *retentionPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_retention_policy"
}

func (r *retentionPolicyResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_retention_policy.RetentionPolicyResourceSchema(ctx)
}

func (r *retentionPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *retentionPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_retention_policy.RetentionPolicyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := map[string]interface{}{
		"dataSource":   plan.DataSource.ValueString(),
		"durationDays": plan.DurationDays.ValueString(),
		"isEnabled":    plan.IsEnabled.ValueBool(),
	}

	if !plan.Env.IsNull() && !plan.Env.IsUnknown() && plan.Env.ValueString() != "" {
		requestBody["env"] = plan.Env.ValueString()
	}

	if !plan.TeamId.IsNull() && !plan.TeamId.IsUnknown() && plan.TeamId.ValueString() != "" {
		requestBody["teamId"] = plan.TeamId.ValueString()
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPost, "/v1/retention-policies", requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create retention policy: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create retention policy: %s", err))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp retentionPolicyAPIResponse

	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	plan.Id = types.StringValue(apiResp.Data.ID)
	plan.DataSource = types.StringValue(apiResp.Data.DataSource)
	plan.DurationDays = types.StringValue(apiResp.Data.DurationDays)
	plan.IsEnabled = types.BoolValue(apiResp.Data.IsEnabled)

	if apiResp.Data.Env != "" {
		plan.Env = types.StringValue(apiResp.Data.Env)
	} else {
		plan.Env = types.StringNull()
	}

	if apiResp.Data.TeamId != "" {
		plan.TeamId = types.StringValue(apiResp.Data.TeamId)
	} else {
		plan.TeamId = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *retentionPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_retention_policy.RetentionPolicyModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/retention-policies/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read retention policy: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read retention policy: %s", err))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp retentionPolicyAPIResponse

	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	state.Id = types.StringValue(apiResp.Data.ID)
	state.DataSource = types.StringValue(apiResp.Data.DataSource)
	state.DurationDays = types.StringValue(apiResp.Data.DurationDays)
	state.IsEnabled = types.BoolValue(apiResp.Data.IsEnabled)

	if apiResp.Data.Env != "" {
		state.Env = types.StringValue(apiResp.Data.Env)
	} else {
		state.Env = types.StringNull()
	}

	if apiResp.Data.TeamId != "" {
		state.TeamId = types.StringValue(apiResp.Data.TeamId)
	} else {
		state.TeamId = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *retentionPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_retention_policy.RetentionPolicyModel
	var state resource_retention_policy.RetentionPolicyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := map[string]interface{}{
		"dataSource":   plan.DataSource.ValueString(),
		"durationDays": plan.DurationDays.ValueString(),
		"isEnabled":    plan.IsEnabled.ValueBool(),
	}

	if !plan.Env.IsNull() && !plan.Env.IsUnknown() && plan.Env.ValueString() != "" {
		requestBody["env"] = plan.Env.ValueString()
	}

	if !plan.TeamId.IsNull() && !plan.TeamId.IsUnknown() && plan.TeamId.ValueString() != "" {
		requestBody["teamId"] = plan.TeamId.ValueString()
	}

	apiPath := fmt.Sprintf("/v1/retention-policies/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodPut, apiPath, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update retention policy: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update retention policy: %s", err))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp retentionPolicyAPIResponse

	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	plan.Id = types.StringValue(apiResp.Data.ID)
	plan.DataSource = types.StringValue(apiResp.Data.DataSource)
	plan.DurationDays = types.StringValue(apiResp.Data.DurationDays)
	plan.IsEnabled = types.BoolValue(apiResp.Data.IsEnabled)

	if apiResp.Data.Env != "" {
		plan.Env = types.StringValue(apiResp.Data.Env)
	} else {
		plan.Env = types.StringNull()
	}

	if apiResp.Data.TeamId != "" {
		plan.TeamId = types.StringValue(apiResp.Data.TeamId)
	} else {
		plan.TeamId = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *retentionPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_retention_policy.RetentionPolicyModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/retention-policies/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, apiPath, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete retention policy: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete retention policy: %s", err))
			return
		}
	}
}

type retentionPolicyAPIResponse struct {
	Data struct {
		ID           string `json:"id"`
		Env          string `json:"env"`
		TeamId       string `json:"teamId"`
		DataSource   string `json:"dataSource"`
		DurationDays string `json:"durationDays"`
		IsEnabled    bool   `json:"isEnabled"`
	} `json:"data"`
}
