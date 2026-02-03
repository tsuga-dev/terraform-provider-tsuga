package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_notification_silence"
	"terraform-provider-tsuga/internal/teamsfilter"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*notificationSilenceResource)(nil)
var _ resource.ResourceWithConfigure = (*notificationSilenceResource)(nil)
var _ resource.ResourceWithImportState = (*notificationSilenceResource)(nil)
var _ resource.ResourceWithValidateConfig = (*notificationSilenceResource)(nil)

func NewNotificationSilenceResource() resource.Resource {
	return &notificationSilenceResource{}
}

type notificationSilenceResource struct {
	client *TsugaClient
}

func (r *notificationSilenceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *notificationSilenceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_silence"
}

func (r *notificationSilenceResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_notification_silence.NotificationSilenceResourceSchema(ctx)
}

func (r *notificationSilenceResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_notification_silence.NotificationSilenceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate teams_filter: teams is required when type is "specific-teams"
	if config.TeamsFilter != nil && !config.TeamsFilter.Type.IsNull() && !config.TeamsFilter.Type.IsUnknown() {
		filterType := config.TeamsFilter.Type.ValueString()
		if filterType == "specific-teams" {
			if config.TeamsFilter.Teams.IsNull() {
				resp.Diagnostics.AddAttributeError(
					path.Root("teams_filter").AtName("teams"),
					"Missing required attribute",
					"teams is required when teams_filter.type is 'specific-teams'",
				)
			} else if !config.TeamsFilter.Teams.IsUnknown() && len(config.TeamsFilter.Teams.Elements()) == 0 {
				resp.Diagnostics.AddAttributeError(
					path.Root("teams_filter").AtName("teams"),
					"Invalid attribute value",
					"teams must contain at least one team ID when teams_filter.type is 'specific-teams'",
				)
			}
		}
	}

}

func (r *notificationSilenceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *notificationSilenceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_notification_silence.NotificationSilenceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildNotificationSilenceRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdateNotificationSilence(ctx, http.MethodPost, "/v1/notification-silences", requestBody, "create")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *notificationSilenceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_notification_silence.NotificationSilenceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/notification-silences/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read notification silence: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read notification silence: %s", err))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp notificationSilenceAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	newState, diags := flattenNotificationSilence(ctx, apiResp.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *notificationSilenceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_notification_silence.NotificationSilenceModel
	var state resource_notification_silence.NotificationSilenceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildNotificationSilenceRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/notification-silences/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateNotificationSilence(ctx, http.MethodPut, path, requestBody, "update")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *notificationSilenceResource) buildNotificationSilenceRequestBody(ctx context.Context, plan resource_notification_silence.NotificationSilenceModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	teamsFilter, expandDiags := teamsfilter.Expand(ctx, plan.TeamsFilter)
	diags.Append(expandDiags...)
	prioritiesFilter, expandDiags := expandIntList(ctx, plan.PrioritiesFilter)
	diags.Append(expandDiags...)
	transitionTypesFilter, expandDiags := expandStringList(ctx, plan.TransitionTypesFilter)
	diags.Append(expandDiags...)
	schedule, expandDiags := expandSchedule(ctx, plan.Schedule)
	diags.Append(expandDiags...)
	notificationRuleIds, expandDiags := expandStringList(ctx, plan.NotificationRuleIds)
	diags.Append(expandDiags...)
	if diags.HasError() {
		return nil, diags
	}

	requestBody := map[string]any{
		"name":                  plan.Name.ValueString(),
		"owner":                 plan.Owner.ValueString(),
		"isActive":              plan.IsActive.ValueBool(),
		"schedule":              schedule,
		"teamsFilter":           teamsFilter,
		"prioritiesFilter":      prioritiesFilter,
		"transitionTypesFilter": transitionTypesFilter,
	}

	if !plan.Reason.IsNull() && !plan.Reason.IsUnknown() {
		requestBody["reason"] = plan.Reason.ValueString()
	}

	if notificationRuleIds != nil {
		requestBody["notificationRuleIds"] = notificationRuleIds
	}

	if !plan.QueryString.IsNull() && !plan.QueryString.IsUnknown() {
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

func (r *notificationSilenceResource) createOrUpdateNotificationSilence(ctx context.Context, method, path string, requestBody map[string]any, operation string) (resource_notification_silence.NotificationSilenceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s notification silence: %s", operation, err))
		return resource_notification_silence.NotificationSilenceModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s notification silence: %s", operation, err))
		return resource_notification_silence.NotificationSilenceModel{}, diags
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return resource_notification_silence.NotificationSilenceModel{}, diags
	}

	var apiResp notificationSilenceAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return resource_notification_silence.NotificationSilenceModel{}, diags
	}

	newState, flattenDiags := flattenNotificationSilence(ctx, apiResp.Data)
	diags.Append(flattenDiags...)
	if diags.HasError() {
		return resource_notification_silence.NotificationSilenceModel{}, diags
	}

	return newState, diags
}

func (r *notificationSilenceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_notification_silence.NotificationSilenceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/notification-silences/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete notification silence: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete notification silence: %s", err))
			return
		}
	}
}

type notificationSilenceAPIResponse struct {
	Data notificationSilenceAPIData `json:"data"`
}

type notificationSilenceAPIData struct {
	ID                    string                      `json:"id"`
	Name                  string                      `json:"name"`
	Reason                string                      `json:"reason,omitempty"`
	Owner                 string                      `json:"owner"`
	Tags                  []apiTag                    `json:"tags"`
	IsActive              bool                        `json:"isActive"`
	Schedule              notificationSilenceSchedule `json:"schedule"`
	NotificationRuleIds   []string                    `json:"notificationRuleIds,omitempty"`
	QueryString           string                      `json:"queryString,omitempty"`
	TeamsFilter           teamsfilter.APITeamsFilter  `json:"teamsFilter"`
	PrioritiesFilter      []int64                     `json:"prioritiesFilter"`
	TransitionTypesFilter []string                    `json:"transitionTypesFilter"`
}

type notificationSilenceSchedule struct {
	Type           string                 `json:"type"`
	StartTime      string                 `json:"startTime,omitempty"`
	EndTime        string                 `json:"endTime,omitempty"`
	WeeklySchedule *weeklyScheduleAPIData `json:"weeklySchedule,omitempty"`
}

type weeklyScheduleAPIData struct {
	Monday    []timeRangeAPIData `json:"monday,omitempty"`
	Tuesday   []timeRangeAPIData `json:"tuesday,omitempty"`
	Wednesday []timeRangeAPIData `json:"wednesday,omitempty"`
	Thursday  []timeRangeAPIData `json:"thursday,omitempty"`
	Friday    []timeRangeAPIData `json:"friday,omitempty"`
	Saturday  []timeRangeAPIData `json:"saturday,omitempty"`
	Sunday    []timeRangeAPIData `json:"sunday,omitempty"`
}

type timeRangeAPIData struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

func expandSchedule(ctx context.Context, schedule *resource_notification_silence.ScheduleModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if schedule == nil || schedule.Recurring == nil {
		return nil, diags
	}

	weeklySchedule := make(map[string]any)

	days := []struct {
		name string
		list types.List
	}{
		{"monday", schedule.Recurring.Monday},
		{"tuesday", schedule.Recurring.Tuesday},
		{"wednesday", schedule.Recurring.Wednesday},
		{"thursday", schedule.Recurring.Thursday},
		{"friday", schedule.Recurring.Friday},
		{"saturday", schedule.Recurring.Saturday},
		{"sunday", schedule.Recurring.Sunday},
	}

	for _, day := range days {
		if !day.list.IsNull() && !day.list.IsUnknown() {
			var timeRanges []resource_notification_silence.TimeRangeModel
			diags.Append(day.list.ElementsAs(ctx, &timeRanges, false)...)
			if diags.HasError() {
				return nil, diags
			}

			apiTimeRanges := make([]map[string]string, 0, len(timeRanges))
			for _, tr := range timeRanges {
				apiTimeRanges = append(apiTimeRanges, map[string]string{
					"startTime": tr.StartTime.ValueString(),
					"endTime":   tr.EndTime.ValueString(),
				})
			}
			weeklySchedule[day.name] = apiTimeRanges
		}
	}

	return map[string]any{
		"type":           "recurring",
		"weeklySchedule": weeklySchedule,
	}, diags
}

func flattenNotificationSilence(ctx context.Context, data notificationSilenceAPIData) (resource_notification_silence.NotificationSilenceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tags, tagDiags := flattenTags(ctx, data.Tags)
	diags.Append(tagDiags...)

	teamsFilter, teamDiags := teamsfilter.Flatten(ctx, data.TeamsFilter)
	diags.Append(teamDiags...)

	prioritiesFilter, priorityDiags := types.ListValueFrom(ctx, types.Int64Type, data.PrioritiesFilter)
	diags.Append(priorityDiags...)

	transitionTypesFilter, transitionDiags := types.ListValueFrom(ctx, types.StringType, data.TransitionTypesFilter)
	diags.Append(transitionDiags...)

	schedule, scheduleDiags := flattenSchedule(ctx, data.Schedule)
	diags.Append(scheduleDiags...)

	var notificationRuleIds types.List
	if data.NotificationRuleIds != nil {
		var ruleIdsDiags diag.Diagnostics
		notificationRuleIds, ruleIdsDiags = types.ListValueFrom(ctx, types.StringType, data.NotificationRuleIds)
		diags.Append(ruleIdsDiags...)
	} else {
		notificationRuleIds = types.ListNull(types.StringType)
	}

	state := resource_notification_silence.NotificationSilenceModel{
		Id:                    types.StringValue(data.ID),
		Name:                  types.StringValue(data.Name),
		Reason:                stringValueOrNull(data.Reason),
		Owner:                 types.StringValue(data.Owner),
		Tags:                  tags,
		IsActive:              types.BoolValue(data.IsActive),
		Schedule:              schedule,
		NotificationRuleIds:   notificationRuleIds,
		QueryString:           stringValueOrNull(data.QueryString),
		TeamsFilter:           teamsFilter,
		PrioritiesFilter:      prioritiesFilter,
		TransitionTypesFilter: transitionTypesFilter,
	}

	return state, diags
}

func flattenSchedule(ctx context.Context, schedule notificationSilenceSchedule) (*resource_notification_silence.ScheduleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	result := &resource_notification_silence.ScheduleModel{}

	if schedule.Type == "recurring" && schedule.WeeklySchedule != nil {
		recurring := &resource_notification_silence.RecurringScheduleModel{}

		timeRangeElemType := types.ObjectType{AttrTypes: resource_notification_silence.TimeRangeAttrTypes(ctx)}

		flattenDayTimeRanges := func(ranges []timeRangeAPIData) (types.List, diag.Diagnostics) {
			if len(ranges) == 0 {
				return types.ListNull(timeRangeElemType), nil
			}
			values := make([]attr.Value, 0, len(ranges))
			for _, tr := range ranges {
				values = append(values, types.ObjectValueMust(resource_notification_silence.TimeRangeAttrTypes(ctx), map[string]attr.Value{
					"start_time": types.StringValue(tr.StartTime),
					"end_time":   types.StringValue(tr.EndTime),
				}))
			}
			return types.ListValue(timeRangeElemType, values)
		}

		var dayDiags diag.Diagnostics

		recurring.Monday, dayDiags = flattenDayTimeRanges(schedule.WeeklySchedule.Monday)
		diags.Append(dayDiags...)
		recurring.Tuesday, dayDiags = flattenDayTimeRanges(schedule.WeeklySchedule.Tuesday)
		diags.Append(dayDiags...)
		recurring.Wednesday, dayDiags = flattenDayTimeRanges(schedule.WeeklySchedule.Wednesday)
		diags.Append(dayDiags...)
		recurring.Thursday, dayDiags = flattenDayTimeRanges(schedule.WeeklySchedule.Thursday)
		diags.Append(dayDiags...)
		recurring.Friday, dayDiags = flattenDayTimeRanges(schedule.WeeklySchedule.Friday)
		diags.Append(dayDiags...)
		recurring.Saturday, dayDiags = flattenDayTimeRanges(schedule.WeeklySchedule.Saturday)
		diags.Append(dayDiags...)
		recurring.Sunday, dayDiags = flattenDayTimeRanges(schedule.WeeklySchedule.Sunday)
		diags.Append(dayDiags...)

		result.Recurring = recurring
	}

	return result, diags
}
