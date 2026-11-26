package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_custom_role"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*customRoleResource)(nil)
var _ resource.ResourceWithConfigure = (*customRoleResource)(nil)
var _ resource.ResourceWithImportState = (*customRoleResource)(nil)
var _ resource.ResourceWithValidateConfig = (*customRoleResource)(nil)

func NewCustomRoleResource() resource.Resource {
	return &customRoleResource{}
}

type customRoleResource struct {
	client *TsugaClient
}

func (r *customRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *customRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_role"
}

func (r *customRoleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_custom_role.CustomRoleResourceSchema(ctx)
}

func (r *customRoleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_custom_role.CustomRoleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.TeamScope.IsNull() || config.TeamScope.IsUnknown() {
		return
	}

	scopeType, teamIds := config.TeamScope.TeamScopeType, config.TeamScope.TeamIds
	if scopeType.IsNull() || scopeType.IsUnknown() || teamIds.IsUnknown() {
		return
	}

	teamIdsPath := path.Root("team_scope").AtName("team_ids")
	switch scopeType.ValueString() {
	case "specific", "all-except":
		if teamIds.IsNull() {
			resp.Diagnostics.AddAttributeError(
				teamIdsPath,
				"Missing required attribute",
				fmt.Sprintf("team_ids is required when team_scope.type is '%s'", scopeType.ValueString()),
			)
		}
	default:
		if !teamIds.IsNull() {
			resp.Diagnostics.AddAttributeError(
				teamIdsPath,
				"Invalid attribute value",
				fmt.Sprintf("team_ids must not be set when team_scope.type is '%s'", scopeType.ValueString()),
			)
		}
	}
}

func (r *customRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *customRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_custom_role.CustomRoleModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildCustomRoleRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdateCustomRole(ctx, http.MethodPost, "/v1/custom-roles", requestBody, "create")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *customRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_custom_role.CustomRoleModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/custom-roles/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read custom role: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read custom role: %s", err))
		return
	}

	r.readJSON(ctx, httpResp.Body, resp)
}

func (r *customRoleResource) readJSON(ctx context.Context, bodyReader io.Reader, resp *resource.ReadResponse) {
	newState, diags := decodeCustomRole(ctx, bodyReader)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *customRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_custom_role.CustomRoleModel
	var state resource_custom_role.CustomRoleModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildCustomRoleRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/custom-roles/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateCustomRole(ctx, http.MethodPut, path, requestBody, "update")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *customRoleResource) buildCustomRoleRequestBody(ctx context.Context, plan resource_custom_role.CustomRoleModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	assets := map[string]string{}
	diags.Append(plan.Assets.ElementsAs(ctx, &assets, false)...)
	telemetry, expandDiags := expandStringSet(ctx, plan.Telemetry)
	diags.Append(expandDiags...)
	teamScope, expandDiags := expandCustomRoleTeamScope(ctx, plan.TeamScope)
	diags.Append(expandDiags...)
	telemetryQuery, expandDiags := expandCustomRoleTelemetryQuery(ctx, plan.TelemetryQuery)
	diags.Append(expandDiags...)
	if diags.HasError() {
		return nil, diags
	}
	if telemetry == nil {
		telemetry = []string{}
	}

	// description and telemetryQuery are sent as null rather than omitted: the API keeps the stored value when they are absent.
	requestBody := map[string]any{
		"name":           plan.Name.ValueString(),
		"description":    nil,
		"teamScope":      teamScope,
		"assets":         assets,
		"telemetry":      telemetry,
		"telemetryQuery": telemetryQuery,
	}
	if !plan.Description.IsNull() {
		requestBody["description"] = plan.Description.ValueString()
	}

	return requestBody, diags
}

func (r *customRoleResource) createOrUpdateCustomRole(ctx context.Context, method, path string, requestBody map[string]any, operation string) (resource_custom_role.CustomRoleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s custom role: %s", operation, err))
		return resource_custom_role.CustomRoleModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s custom role: %s", operation, err))
		return resource_custom_role.CustomRoleModel{}, diags
	}

	newState, decodeDiags := decodeCustomRole(ctx, httpResp.Body)
	diags.Append(decodeDiags...)
	if diags.HasError() {
		return resource_custom_role.CustomRoleModel{}, diags
	}

	return newState, diags
}

func (r *customRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_custom_role.CustomRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/custom-roles/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]any{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete custom role: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete custom role: %s", err))
			return
		}
	}
}

type customRoleAPIResponse struct {
	Data customRoleAPIData `json:"data"`
}

type customRoleAPIData struct {
	ID             string                        `json:"id"`
	Name           string                        `json:"name"`
	Description    string                        `json:"description,omitempty"`
	TeamScope      customRoleAPITeamScope        `json:"teamScope"`
	Assets         map[string]string             `json:"assets"`
	Telemetry      []string                      `json:"telemetry"`
	TelemetryQuery []customRoleAPITelemetryQuery `json:"telemetryQuery,omitempty"`
}

type customRoleAPITeamScope struct {
	Type    string   `json:"type"`
	TeamIds []string `json:"teamIds,omitempty"`
}

type customRoleAPITelemetryQuery struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

func expandCustomRoleTeamScope(ctx context.Context, scope resource_custom_role.TeamScopeValue) (customRoleAPITeamScope, diag.Diagnostics) {
	teamIds, diags := expandStringSet(ctx, scope.TeamIds)
	return customRoleAPITeamScope{Type: scope.TeamScopeType.ValueString(), TeamIds: teamIds}, diags
}

func expandCustomRoleTelemetryQuery(ctx context.Context, query types.List) ([]customRoleAPITelemetryQuery, diag.Diagnostics) {
	var diags diag.Diagnostics
	if query.IsNull() || query.IsUnknown() {
		return nil, diags
	}

	var entries []resource_custom_role.TelemetryQueryValue
	diags.Append(query.ElementsAs(ctx, &entries, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]customRoleAPITelemetryQuery, 0, len(entries))
	for _, entry := range entries {
		values, valuesDiags := expandStringList(ctx, entry.Values)
		diags.Append(valuesDiags...)
		result = append(result, customRoleAPITelemetryQuery{Key: entry.Key.ValueString(), Values: values})
	}
	return result, diags
}

func decodeCustomRole(ctx context.Context, bodyReader io.Reader) (resource_custom_role.CustomRoleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	body, err := io.ReadAll(bodyReader)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return resource_custom_role.CustomRoleModel{}, diags
	}

	var apiResp customRoleAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return resource_custom_role.CustomRoleModel{}, diags
	}

	return flattenCustomRole(ctx, apiResp.Data)
}

func flattenCustomRole(ctx context.Context, data customRoleAPIData) (resource_custom_role.CustomRoleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	teamScope, scopeDiags := flattenCustomRoleTeamScope(ctx, data.TeamScope)
	diags.Append(scopeDiags...)

	assetsData := data.Assets
	if assetsData == nil {
		assetsData = map[string]string{}
	}
	assets, assetsDiags := types.MapValueFrom(ctx, types.StringType, assetsData)
	diags.Append(assetsDiags...)

	telemetryData := data.Telemetry
	if telemetryData == nil {
		telemetryData = []string{}
	}
	telemetry, telemetryDiags := types.SetValueFrom(ctx, types.StringType, telemetryData)
	diags.Append(telemetryDiags...)

	telemetryQuery, queryDiags := flattenCustomRoleTelemetryQuery(ctx, data.TelemetryQuery)
	diags.Append(queryDiags...)

	state := resource_custom_role.CustomRoleModel{
		Id:             types.StringValue(data.ID),
		Name:           types.StringValue(data.Name),
		Description:    stringValueOrNull(data.Description),
		TeamScope:      teamScope,
		Assets:         assets,
		Telemetry:      telemetry,
		TelemetryQuery: telemetryQuery,
	}

	return state, diags
}

func flattenCustomRoleTeamScope(ctx context.Context, scope customRoleAPITeamScope) (resource_custom_role.TeamScopeValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	teamIds := types.SetNull(types.StringType)
	if scope.TeamIds != nil {
		var teamDiags diag.Diagnostics
		teamIds, teamDiags = types.SetValueFrom(ctx, types.StringType, scope.TeamIds)
		diags.Append(teamDiags...)
	}

	return resource_custom_role.NewTeamScopeValueMust(
		resource_custom_role.TeamScopeValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"type": types.StringValue(scope.Type), "team_ids": teamIds},
	), diags
}

func flattenCustomRoleTelemetryQuery(ctx context.Context, entries []customRoleAPITelemetryQuery) (types.List, diag.Diagnostics) {
	attrTypes := resource_custom_role.TelemetryQueryValue{}.AttributeTypes(ctx)
	elemType := resource_custom_role.TelemetryQueryType{ObjectType: types.ObjectType{AttrTypes: attrTypes}}
	if entries == nil {
		return types.ListNull(elemType), nil
	}

	var diags diag.Diagnostics
	values := make([]attr.Value, 0, len(entries))
	for _, entry := range entries {
		entryValues, valuesDiags := types.ListValueFrom(ctx, types.StringType, entry.Values)
		diags.Append(valuesDiags...)
		values = append(values, resource_custom_role.NewTelemetryQueryValueMust(attrTypes, map[string]attr.Value{
			"key":    types.StringValue(entry.Key),
			"values": entryValues,
		}))
	}

	list, listDiags := types.ListValue(elemType, values)
	diags.Append(listDiags...)
	return list, diags
}
