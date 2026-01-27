package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_tag_policy"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*tagPolicyResource)(nil)
var _ resource.ResourceWithConfigure = (*tagPolicyResource)(nil)
var _ resource.ResourceWithImportState = (*tagPolicyResource)(nil)
var _ resource.ResourceWithValidateConfig = (*tagPolicyResource)(nil)

func NewTagPolicyResource() resource.Resource {
	return &tagPolicyResource{}
}

type tagPolicyResource struct {
	client *TsugaClient
}

func (r *tagPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tagPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_policy"
}

func (r *tagPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_tag_policy.TagPolicyResourceSchema()
}

func (r *tagPolicyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_tag_policy.TagPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate configuration: exactly one of telemetry or tsuga_asset must be set
	if config.Configuration == nil {
		return
	}

	telemetrySet := config.Configuration.Telemetry != nil
	tsugaAssetSet := config.Configuration.TsugaAsset != nil

	if !telemetrySet && !tsugaAssetSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("configuration"),
			"Missing required configuration",
			"Exactly one of 'telemetry' or 'tsuga_asset' must be configured.",
		)
	}

	if telemetrySet && tsugaAssetSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("configuration"),
			"Invalid configuration",
			"Only one of 'telemetry' or 'tsuga_asset' can be configured, not both.",
		)
	}
}

func (r *tagPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *tagPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_tag_policy.TagPolicyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildTagPolicyRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdateTagPolicy(ctx, http.MethodPost, "/v1/tag-policies", requestBody, "create")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *tagPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_tag_policy.TagPolicyModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/tag-policies/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read tag policy: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read tag policy: %s", err))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp tagPolicyAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	newState, diags := flattenTagPolicy(ctx, apiResp.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *tagPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_tag_policy.TagPolicyModel
	var state resource_tag_policy.TagPolicyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildTagPolicyRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/tag-policies/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateTagPolicy(ctx, http.MethodPut, path, requestBody, "update")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *tagPolicyResource) buildTagPolicyRequestBody(ctx context.Context, plan resource_tag_policy.TagPolicyModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	allowedTagValues, expandDiags := expandStringList(ctx, plan.AllowedTagValues)
	diags.Append(expandDiags...)
	if diags.HasError() {
		return nil, diags
	}

	requestBody := map[string]any{
		"name":             plan.Name.ValueString(),
		"isActive":         plan.IsActive.ValueBool(),
		"tagKey":           plan.TagKey.ValueString(),
		"allowedTagValues": allowedTagValues,
		"isRequired":       plan.IsRequired.ValueBool(),
		"owner":            plan.Owner.ValueString(),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		requestBody["description"] = plan.Description.ValueString()
	}

	if plan.TeamScope != nil {
		teamScope, teamDiags := expandTeamScope(ctx, plan.TeamScope)
		diags.Append(teamDiags...)
		if diags.HasError() {
			return nil, diags
		}
		requestBody["teamScope"] = teamScope
	}

	configuration, configDiags := expandConfiguration(ctx, plan.Configuration)
	diags.Append(configDiags...)
	if diags.HasError() {
		return nil, diags
	}
	requestBody["configuration"] = configuration

	return requestBody, diags
}

func (r *tagPolicyResource) createOrUpdateTagPolicy(ctx context.Context, method, path string, requestBody map[string]any, operation string) (resource_tag_policy.TagPolicyModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s tag policy: %s", operation, err))
		return resource_tag_policy.TagPolicyModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s tag policy: %s", operation, err))
		return resource_tag_policy.TagPolicyModel{}, diags
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return resource_tag_policy.TagPolicyModel{}, diags
	}

	var apiResp tagPolicyAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return resource_tag_policy.TagPolicyModel{}, diags
	}

	newState, flattenDiags := flattenTagPolicy(ctx, apiResp.Data)
	diags.Append(flattenDiags...)
	if diags.HasError() {
		return resource_tag_policy.TagPolicyModel{}, diags
	}

	return newState, diags
}

func (r *tagPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_tag_policy.TagPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/tag-policies/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]any{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete tag policy: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete tag policy: %s", err))
			return
		}
	}
}

type tagPolicyAPIResponse struct {
	Data tagPolicyAPIData `json:"data"`
}

type tagPolicyAPIData struct {
	ID               string                    `json:"id"`
	Name             string                    `json:"name"`
	Description      string                    `json:"description,omitempty"`
	IsActive         bool                      `json:"isActive"`
	TagKey           string                    `json:"tagKey"`
	AllowedTagValues []string                  `json:"allowedTagValues"`
	IsRequired       bool                      `json:"isRequired"`
	TeamScope        *tagPolicyAPITeamScope    `json:"teamScope,omitempty"`
	Configuration    tagPolicyAPIConfiguration `json:"configuration"`
	Owner            string                    `json:"owner,omitempty"`
}

type tagPolicyAPITeamScope struct {
	TeamIds []string `json:"teamIds"`
	Mode    string   `json:"mode"`
}

type tagPolicyAPIConfiguration struct {
	Type                string   `json:"type"`
	AssetTypes          []string `json:"assetTypes"`
	ShouldInsertWarning *bool    `json:"shouldInsertWarning,omitempty"`
	DropSample          *float64 `json:"dropSample,omitempty"`
}

func expandTeamScope(ctx context.Context, scope *resource_tag_policy.TeamScopeModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if scope == nil {
		return nil, diags
	}

	teamIds, expandDiags := expandStringList(ctx, scope.TeamIds)
	diags.Append(expandDiags...)
	if diags.HasError() {
		return nil, diags
	}

	return map[string]any{
		"teamIds": teamIds,
		"mode":    scope.Mode.ValueString(),
	}, diags
}

func expandConfiguration(ctx context.Context, config *resource_tag_policy.ConfigurationModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if config == nil {
		diags.AddError("Invalid configuration", "Configuration is required.")
		return nil, diags
	}

	if config.Telemetry != nil {
		assetTypes, expandDiags := expandStringList(ctx, config.Telemetry.AssetTypes)
		diags.Append(expandDiags...)
		if diags.HasError() {
			return nil, diags
		}

		result := map[string]any{
			"type":                "telemetry",
			"assetTypes":          assetTypes,
			"shouldInsertWarning": config.Telemetry.ShouldInsertWarning.ValueBool(),
		}

		if !config.Telemetry.DropSample.IsNull() && !config.Telemetry.DropSample.IsUnknown() {
			result["dropSample"] = config.Telemetry.DropSample.ValueFloat64()
		}

		return result, diags
	}

	if config.TsugaAsset != nil {
		assetTypes, expandDiags := expandStringList(ctx, config.TsugaAsset.AssetTypes)
		diags.Append(expandDiags...)
		if diags.HasError() {
			return nil, diags
		}

		return map[string]any{
			"type":       "tsuga_asset",
			"assetTypes": assetTypes,
		}, diags
	}

	diags.AddError("Invalid configuration", "Either telemetry or tsuga_asset must be configured.")
	return nil, diags
}

func flattenTagPolicy(ctx context.Context, data tagPolicyAPIData) (resource_tag_policy.TagPolicyModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	allowedTagValues, listDiags := types.ListValueFrom(ctx, types.StringType, data.AllowedTagValues)
	diags.Append(listDiags...)

	var teamScope *resource_tag_policy.TeamScopeModel
	if data.TeamScope != nil {
		teamIds, teamDiags := types.ListValueFrom(ctx, types.StringType, data.TeamScope.TeamIds)
		diags.Append(teamDiags...)
		teamScope = &resource_tag_policy.TeamScopeModel{
			TeamIds: teamIds,
			Mode:    types.StringValue(data.TeamScope.Mode),
		}
	}

	configuration, configDiags := flattenConfiguration(ctx, data.Configuration)
	diags.Append(configDiags...)

	description := types.StringNull()
	if data.Description != "" {
		description = types.StringValue(data.Description)
	}

	owner := types.StringNull()
	if data.Owner != "" {
		owner = types.StringValue(data.Owner)
	}

	state := resource_tag_policy.TagPolicyModel{
		Id:               types.StringValue(data.ID),
		Name:             types.StringValue(data.Name),
		Description:      description,
		IsActive:         types.BoolValue(data.IsActive),
		TagKey:           types.StringValue(data.TagKey),
		AllowedTagValues: allowedTagValues,
		IsRequired:       types.BoolValue(data.IsRequired),
		TeamScope:        teamScope,
		Configuration:    configuration,
		Owner:            owner,
	}

	return state, diags
}

func flattenConfiguration(ctx context.Context, config tagPolicyAPIConfiguration) (*resource_tag_policy.ConfigurationModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := &resource_tag_policy.ConfigurationModel{}

	assetTypes, listDiags := types.ListValueFrom(ctx, types.StringType, config.AssetTypes)
	diags.Append(listDiags...)

	switch config.Type {
	case "telemetry":
		dropSample := types.Float64Null()
		if config.DropSample != nil {
			dropSample = types.Float64Value(*config.DropSample)
		}

		shouldInsertWarning := types.BoolValue(false)
		if config.ShouldInsertWarning != nil {
			shouldInsertWarning = types.BoolValue(*config.ShouldInsertWarning)
		}

		result.Telemetry = &resource_tag_policy.TelemetryConfigModel{
			Type:                types.StringValue("telemetry"),
			AssetTypes:          assetTypes,
			ShouldInsertWarning: shouldInsertWarning,
			DropSample:          dropSample,
		}
	case "tsuga_asset":
		result.TsugaAsset = &resource_tag_policy.TsugaAssetConfigModel{
			Type:       types.StringValue("tsuga_asset"),
			AssetTypes: assetTypes,
		}
	}

	return result, diags
}
