package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/apiclient"
	"terraform-provider-tsuga/internal/resource_notification_rule"
	"terraform-provider-tsuga/internal/teamsfilter"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*notificationRuleResource)(nil)
var _ resource.ResourceWithConfigure = (*notificationRuleResource)(nil)
var _ resource.ResourceWithImportState = (*notificationRuleResource)(nil)
var _ resource.ResourceWithValidateConfig = (*notificationRuleResource)(nil)

func NewNotificationRuleResource() resource.Resource {
	return &notificationRuleResource{}
}

type notificationRuleResource struct {
	client *TsugaClient
}

func (r *notificationRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *notificationRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_rule"
}

func (r *notificationRuleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_notification_rule.NotificationRuleResourceSchema(ctx)
}

func (r *notificationRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_notification_rule.NotificationRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate teams_filter: teams is required when type is "specific-teams"
	filter := config.TeamsFilter
	if !filter.TeamsFilterType.IsNull() && !filter.TeamsFilterType.IsUnknown() && filter.TeamsFilterType.ValueString() == "specific-teams" {
		if filter.Teams.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("teams_filter").AtName("teams"),
				"Missing required attribute",
				"teams is required when teams_filter.type is 'specific-teams'",
			)
		} else if !filter.Teams.IsUnknown() && len(filter.Teams.Elements()) == 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("teams_filter").AtName("teams"),
				"Invalid attribute value",
				"teams must contain at least one team ID when teams_filter.type is 'specific-teams'",
			)
		}
	}
}

func (r *notificationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *notificationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_notification_rule.NotificationRuleModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildNotificationRuleRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdateNotificationRule(ctx, http.MethodPost, "/v1/notification-rules", requestBody, "create")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *notificationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_notification_rule.NotificationRuleModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/notification-rules/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read notification rule: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read notification rule: %s", err))
		return
	}

	r.readJSON(ctx, httpResp.Body, resp)
}

func (r *notificationRuleResource) readJSON(ctx context.Context, bodyReader io.Reader, resp *resource.ReadResponse) {
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp apiclient.GetNotificationRuleResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	newState, diags := flattenNotificationRule(ctx, apiResp.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *notificationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_notification_rule.NotificationRuleModel
	var state resource_notification_rule.NotificationRuleModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildNotificationRuleRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/notification-rules/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateNotificationRule(ctx, http.MethodPut, path, requestBody, "update")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *notificationRuleResource) buildNotificationRuleRequestBody(ctx context.Context, plan resource_notification_rule.NotificationRuleModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	teamsFilter, expandDiags := teamsfilter.Expand(ctx, &teamsfilter.Model{Type: plan.TeamsFilter.TeamsFilterType, Teams: plan.TeamsFilter.Teams})
	diags.Append(expandDiags...)
	clusterIdsFilter, expandDiags := expandStringList(ctx, plan.ClusterIdsFilter)
	diags.Append(expandDiags...)
	prioritiesFilter, expandDiags := expandIntList(ctx, plan.PrioritiesFilter)
	diags.Append(expandDiags...)
	transitionTypesFilter, expandDiags := expandStringList(ctx, plan.TransitionTypesFilter)
	diags.Append(expandDiags...)
	var targetsBody apiclient.CreateNotificationRuleJSONBody
	diags.Append(expandNotificationRuleTargets(ctx, plan.Targets, &targetsBody)...)
	if diags.HasError() {
		return nil, diags
	}

	requestBody := map[string]any{
		"name":                  plan.Name.ValueString(),
		"teamsFilter":           teamsFilter,
		"prioritiesFilter":      prioritiesFilter,
		"transitionTypesFilter": transitionTypesFilter,
		"owner":                 plan.Owner.ValueString(),
		"isActive":              plan.IsActive.ValueBool(),
		"targets":               targetsBody.Targets,
	}

	// An omitted attribute plans as an empty list, which the API reads as "every cluster".
	if clusterIdsFilter == nil {
		clusterIdsFilter = []string{}
	}
	requestBody["clusterIdsFilter"] = clusterIdsFilter

	// Send an explicit empty string when the value is absent: the API keeps
	// omitted optional fields on update, so omitting it would leave a previously
	// set query_string in place instead of clearing it.
	if !plan.QueryString.IsUnknown() {
		requestBody["queryString"] = plan.QueryString.ValueString()
	}

	if tags, tagDiags := expandTags(ctx, plan.Tags); tagDiags.HasError() {
		diags.Append(tagDiags...)
		return nil, diags
	} else if tags != nil {
		requestBody["tags"] = tags
	}

	return requestBody, diags
}

func (r *notificationRuleResource) createOrUpdateNotificationRule(ctx context.Context, method, path string, requestBody map[string]interface{}, operation string) (resource_notification_rule.NotificationRuleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s notification rule: %s", operation, err))
		return resource_notification_rule.NotificationRuleModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s notification rule: %s", operation, err))
		return resource_notification_rule.NotificationRuleModel{}, diags
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return resource_notification_rule.NotificationRuleModel{}, diags
	}

	var apiResp apiclient.GetNotificationRuleResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return resource_notification_rule.NotificationRuleModel{}, diags
	}

	newState, flattenDiags := flattenNotificationRule(ctx, apiResp.Data)
	diags.Append(flattenDiags...)
	if diags.HasError() {
		return resource_notification_rule.NotificationRuleModel{}, diags
	}

	return newState, diags
}

func (r *notificationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_notification_rule.NotificationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/notification-rules/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete notification rule: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete notification rule: %s", err))
			return
		}
	}
}

func flattenNotificationRule(ctx context.Context, data apiclient.Rule) (resource_notification_rule.NotificationRuleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tagsList, tagDiags := flattenTags(ctx, tagsFromAPI(data.Tags))
	diags.Append(tagDiags...)

	targets, targetDiags := flattenNotificationRuleTargets(ctx, data)
	diags.Append(targetDiags...)

	teamsFilterAPI, err := flattenAPITeamsFilter(data.TeamsFilter)
	if err != nil {
		diags.AddError("Unsupported teams filter", err.Error())
		return resource_notification_rule.NotificationRuleModel{}, diags
	}
	filter, filterDiags := teamsfilter.Flatten(ctx, teamsFilterAPI)
	diags.Append(filterDiags...)
	prioritiesFilter, priorityDiags := types.ListValueFrom(ctx, types.Int64Type, convertInts[int64](data.PrioritiesFilter))
	diags.Append(priorityDiags...)
	transitionTypesFilter, transitionDiags := types.ListValueFrom(ctx, types.StringType, stringsOf(data.TransitionTypesFilter))
	diags.Append(transitionDiags...)
	if data.ClusterIdsFilter == nil {
		data.ClusterIdsFilter = []string{}
	}
	clusterIdsFilter, clusterDiags := types.ListValueFrom(ctx, types.StringType, data.ClusterIdsFilter)
	diags.Append(clusterDiags...)
	if diags.HasError() {
		return resource_notification_rule.NotificationRuleModel{}, diags
	}

	teamsFilter, teamsFilterDiags := resource_notification_rule.NewTeamsFilterValue(
		resource_notification_rule.TeamsFilterValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"type": filter.Type, "teams": filter.Teams},
	)
	diags.Append(teamsFilterDiags...)

	return resource_notification_rule.NotificationRuleModel{
		ClusterIdsFilter:      clusterIdsFilter,
		Id:                    types.StringValue(data.Id),
		IsActive:              types.BoolValue(data.IsActive),
		Name:                  types.StringValue(data.Name),
		Owner:                 types.StringValue(data.Owner),
		PrioritiesFilter:      prioritiesFilter,
		QueryString:           stringValueOrNull(derefString(data.QueryString)),
		Tags:                  tagsList,
		Targets:               targets,
		TeamsFilter:           teamsFilter,
		TransitionTypesFilter: transitionTypesFilter,
	}, diags
}

func convertInts[To ~int | ~int64, From ~int](in []From) []To {
	out := make([]To, len(in))
	for i, n := range in {
		out[i] = To(n)
	}
	return out
}

func flattenAPITeamsFilter(filter apiclient.Rule_TeamsFilter) (teamsfilter.APITeamsFilter, error) {
	value, err := filter.ValueByDiscriminator()
	if err != nil {
		return teamsfilter.APITeamsFilter{}, err
	}
	switch v := value.(type) {
	case apiclient.TeamsFilterSpecificTeams:
		return teamsfilter.APITeamsFilter{Type: string(v.Type), Teams: v.Teams}, nil
	case apiclient.TeamsFilterAllTeams:
		return teamsfilter.APITeamsFilter{Type: string(v.Type)}, nil
	case apiclient.TeamsFilterAllPublicTeams:
		return teamsfilter.APITeamsFilter{Type: string(v.Type)}, nil
	}
	return teamsfilter.APITeamsFilter{}, fmt.Errorf("unexpected teams filter type %T", value)
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
