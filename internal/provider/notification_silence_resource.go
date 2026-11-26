package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"terraform-provider-tsuga/internal/resource_notification_silence"
	"terraform-provider-tsuga/internal/teamsfilter"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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

	if oneTime := config.Schedule.OneTime; !oneTime.IsNull() && !oneTime.IsUnknown() {
		attrs := oneTime.Attributes()
		startTime, _ := attrs["start_time"].(types.String)
		endTime, _ := attrs["end_time"].(types.String)
		resp.Diagnostics.Append(validateTimeWindow(startTime, endTime, simpleDateTimeLayout, path.Root("schedule").AtName("one_time"))...)
	}
	if recurring := config.Schedule.Recurring; !recurring.IsNull() && !recurring.IsUnknown() {
		resp.Diagnostics.Append(validateRecurringTimeOrder(ctx, recurring)...)
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

// Layouts for the API's `simple-date-time` and `simple-time` formats.
const (
	simpleDateTimeLayout = "2006-01-02T15:04:05"
	simpleTimeLayout     = "15:04:05"
)

func validateTimeWindow(start, end types.String, layout string, p path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	parse := func(value types.String, name string) (time.Time, bool) {
		if value.IsNull() || value.IsUnknown() {
			return time.Time{}, false
		}
		// time.Parse accepts a non-padded hour ("9:00:00"), which the API rejects;
		// the layout is fixed-width, so an exact length pins the padding.
		parsed, err := time.Parse(layout, value.ValueString())
		if err != nil || len(value.ValueString()) != len(layout) {
			diags.AddAttributeError(
				p.AtName(name),
				"Invalid schedule configuration",
				fmt.Sprintf("%s (%s) is not a valid time.", name, value.ValueString()),
			)
			return time.Time{}, false
		}
		return parsed, true
	}

	startTime, startOk := parse(start, "start_time")
	endTime, endOk := parse(end, "end_time")
	if !startOk || !endOk {
		return diags
	}

	if !endTime.After(startTime) {
		diags.AddAttributeError(
			p.AtName("end_time"),
			"Invalid schedule configuration",
			fmt.Sprintf("end_time (%s) must be after start_time (%s).", end.ValueString(), start.ValueString()),
		)
	}

	return diags
}

var weekdays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

func validateRecurringTimeOrder(ctx context.Context, recurring basetypes.ObjectValue) diag.Diagnostics {
	var diags diag.Diagnostics

	weeklySchedule, _ := recurring.Attributes()["weekly_schedule"].(basetypes.ObjectValue)
	if weeklySchedule.IsNull() || weeklySchedule.IsUnknown() {
		return diags
	}

	for _, day := range weekdays {
		ranges, _ := weeklySchedule.Attributes()[day].(basetypes.ListValue)
		if ranges.IsNull() || ranges.IsUnknown() {
			continue
		}

		for i, element := range ranges.Elements() {
			// A config can hold an unknown element (e.g. a window built from a computed value);
			// its bounds read as null and are skipped.
			timeRange, ok := element.(basetypes.ObjectValuable)
			if !ok {
				continue
			}
			object, objectDiags := timeRange.ToObjectValue(ctx)
			diags.Append(objectDiags...)
			startTime, _ := object.Attributes()["start_time"].(types.String)
			endTime, _ := object.Attributes()["end_time"].(types.String)
			diags.Append(validateTimeWindow(
				startTime,
				endTime,
				simpleTimeLayout,
				path.Root("schedule").AtName("recurring").AtName("weekly_schedule").AtName(day).AtListIndex(i),
			)...)
		}
	}

	return diags
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

	r.readJSON(ctx, httpResp.Body, resp)
}

func (r *notificationSilenceResource) readJSON(ctx context.Context, bodyReader io.Reader, resp *resource.ReadResponse) {
	body, err := io.ReadAll(bodyReader)
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

	teamsFilter, expandDiags := teamsfilter.Expand(ctx, &teamsfilter.Model{Type: plan.TeamsFilter.TeamsFilterType, Teams: plan.TeamsFilter.Teams})
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
	Type           string                        `json:"type"`
	StartTime      string                        `json:"startTime,omitempty"`
	EndTime        string                        `json:"endTime,omitempty"`
	TimeZone       string                        `json:"timeZone,omitempty"`
	WeeklySchedule map[string][]timeRangeAPIData `json:"weeklySchedule,omitempty"`
}

type timeRangeAPIData struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

func expandSchedule(ctx context.Context, schedule resource_notification_silence.ScheduleValue) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if oneTime := schedule.OneTime; !oneTime.IsNull() && !oneTime.IsUnknown() {
		attrs := oneTime.Attributes()
		result := map[string]any{
			"type":      "one-time",
			"startTime": attrs["start_time"].(types.String).ValueString(),
			"endTime":   attrs["end_time"].(types.String).ValueString(),
		}
		if timeZone := attrs["time_zone"].(types.String); !timeZone.IsNull() && !timeZone.IsUnknown() {
			result["timeZone"] = timeZone.ValueString()
		}
		return result, diags
	}

	recurring := schedule.Recurring
	if recurring.IsNull() || recurring.IsUnknown() {
		return nil, diags
	}

	configured, _ := recurring.Attributes()["weekly_schedule"].(basetypes.ObjectValue)
	weeklySchedule := make(map[string]any)
	for _, day := range weekdays {
		list, _ := configured.Attributes()[day].(basetypes.ListValue)
		if list.IsNull() || list.IsUnknown() {
			continue
		}

		timeRanges := make([]map[string]string, 0, len(list.Elements()))
		for _, element := range list.Elements() {
			object, objectDiags := element.(basetypes.ObjectValuable).ToObjectValue(ctx)
			diags.Append(objectDiags...)
			timeRanges = append(timeRanges, map[string]string{
				"startTime": object.Attributes()["start_time"].(types.String).ValueString(),
				"endTime":   object.Attributes()["end_time"].(types.String).ValueString(),
			})
		}
		weeklySchedule[day] = timeRanges
	}

	result := map[string]any{
		"type":           "recurring",
		"weeklySchedule": weeklySchedule,
	}
	if timeZone := recurring.Attributes()["time_zone"].(types.String); !timeZone.IsNull() && !timeZone.IsUnknown() {
		result["timeZone"] = timeZone.ValueString()
	}
	return result, diags
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

	teamsFilterValue, teamsFilterDiags := resource_notification_silence.NewTeamsFilterValue(
		resource_notification_silence.TeamsFilterValue{}.AttributeTypes(ctx),
		map[string]attr.Value{"type": teamsFilter.Type, "teams": teamsFilter.Teams},
	)
	diags.Append(teamsFilterDiags...)

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
		TeamsFilter:           teamsFilterValue,
		PrioritiesFilter:      prioritiesFilter,
		TransitionTypesFilter: transitionTypesFilter,
	}

	return state, diags
}

func flattenSchedule(ctx context.Context, schedule notificationSilenceSchedule) (resource_notification_silence.ScheduleValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrTypes := resource_notification_silence.ScheduleValue{}.AttributeTypes(ctx)
	oneTimeTypes := attrTypes["one_time"].(basetypes.ObjectType).AttrTypes
	recurringTypes := attrTypes["recurring"].(basetypes.ObjectType).AttrTypes
	values := map[string]attr.Value{
		"one_time":  types.ObjectNull(oneTimeTypes),
		"recurring": types.ObjectNull(recurringTypes),
	}

	switch {
	case schedule.Type == "one-time":
		values["one_time"] = types.ObjectValueMust(oneTimeTypes, map[string]attr.Value{
			"start_time": types.StringValue(schedule.StartTime),
			"end_time":   types.StringValue(schedule.EndTime),
			"time_zone":  stringValueOrNull(schedule.TimeZone),
		})
	case schedule.Type == "recurring" && schedule.WeeklySchedule != nil:
		weeklyTypes := recurringTypes["weekly_schedule"].(basetypes.ObjectType).AttrTypes
		weekly := map[string]attr.Value{}
		for _, day := range weekdays {
			elemType := weeklyTypes[day].(basetypes.ListType).ElemType
			ranges := schedule.WeeklySchedule[day]
			if len(ranges) == 0 {
				weekly[day] = types.ListNull(elemType)
				continue
			}

			rangeTypes := elemType.(attr.TypeWithAttributeTypes).AttributeTypes()
			elements := make([]attr.Value, 0, len(ranges))
			for _, timeRange := range ranges {
				object := types.ObjectValueMust(rangeTypes, map[string]attr.Value{
					"start_time": types.StringValue(timeRange.StartTime),
					"end_time":   types.StringValue(timeRange.EndTime),
				})
				element, elementDiags := elemType.(basetypes.ObjectTypable).ValueFromObject(ctx, object)
				diags.Append(elementDiags...)
				elements = append(elements, element)
			}
			list, listDiags := types.ListValue(elemType, elements)
			diags.Append(listDiags...)
			weekly[day] = list
		}
		values["recurring"] = types.ObjectValueMust(recurringTypes, map[string]attr.Value{
			"time_zone":       stringValueOrNull(schedule.TimeZone),
			"weekly_schedule": types.ObjectValueMust(weeklyTypes, weekly),
		})
	}

	result, resultDiags := resource_notification_silence.NewScheduleValue(attrTypes, values)
	diags.Append(resultDiags...)
	return result, diags
}
