package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_notification_rule"

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

func (r *notificationRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *notificationRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_rule"
}

func (r *notificationRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_notification_rule.NotificationRuleResourceSchema(ctx)
}

func (r *notificationRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_notification_rule.NotificationRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate targets: each target's config must have exactly one config type set
	if !config.Targets.IsNull() && !config.Targets.IsUnknown() {
		var targets []resource_notification_rule.TargetModel
		resp.Diagnostics.Append(config.Targets.ElementsAs(ctx, &targets, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		for i, target := range targets {
			diags := r.validateTargetConfig(ctx, target.Config, fmt.Sprintf("targets[%d].config", i))
			resp.Diagnostics.Append(diags...)
		}
	}
}

func (r *notificationRuleResource) validateTargetConfig(ctx context.Context, cfg resource_notification_rule.TargetConfigModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	setCount := 0
	if cfg.Slack != nil {
		setCount++
	}
	if cfg.IncidentIO != nil {
		setCount++
	}
	if cfg.PagerDuty != nil {
		setCount++
	}
	if cfg.Email != nil {
		setCount++
	}
	if cfg.GrafanaIRM != nil {
		setCount++
	}
	if cfg.MicrosoftTeams != nil {
		setCount++
	}
	if cfg.Webhook != nil {
		setCount++
	}

	if setCount != 1 {
		diags.AddError(
			"Invalid target config configuration",
			fmt.Sprintf("%s: exactly one of slack, incident_io, pagerduty, email, grafana_irm, microsoft_teams, or webhook must be set.", pathPrefix),
		)
	}

	return diags
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

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp notificationRuleAPIResponse
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

func (r *notificationRuleResource) buildNotificationRuleRequestBody(ctx context.Context, plan resource_notification_rule.NotificationRuleModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	teamsFilter, expandDiags := expandStringList(ctx, plan.TeamsFilter)
	diags.Append(expandDiags...)
	prioritiesFilter, expandDiags := expandIntList(ctx, plan.PrioritiesFilter)
	diags.Append(expandDiags...)
	transitionTypesFilter, expandDiags := expandStringList(ctx, plan.TransitionTypesFilter)
	diags.Append(expandDiags...)
	targets, expandDiags := expandNotificationRuleTargets(ctx, plan.Targets)
	diags.Append(expandDiags...)
	if diags.HasError() {
		return nil, diags
	}

	requestBody := map[string]interface{}{
		"name":                  plan.Name.ValueString(),
		"teamsFilter":           teamsFilter,
		"prioritiesFilter":      prioritiesFilter,
		"transitionTypesFilter": transitionTypesFilter,
		"owner":                 plan.Owner.ValueString(),
		"isActive":              plan.IsActive.ValueBool(),
		"targets":               targets,
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

	var apiResp notificationRuleAPIResponse
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

type notificationRuleAPIResponse struct {
	Data notificationRuleAPIData `json:"data"`
}

type notificationRuleAPIData struct {
	ID                    string                      `json:"id"`
	Name                  string                      `json:"name"`
	TeamsFilter           []string                    `json:"teamsFilter"`
	PrioritiesFilter      []int64                     `json:"prioritiesFilter"`
	TransitionTypesFilter []string                    `json:"transitionTypesFilter"`
	Owner                 string                      `json:"owner"`
	IsActive              bool                        `json:"isActive"`
	Tags                  []apiTag                    `json:"tags"`
	Targets               []notificationRuleAPITarget `json:"targets"`
}

type notificationRuleAPITarget struct {
	ID             string                              `json:"id"`
	Config         notificationRuleAPITargetConfig     `json:"config"`
	RateLimit      *notificationRuleAPITargetRateLimit `json:"rateLimit,omitempty"`
	RenotifyConfig *notificationRuleAPITargetRenotify  `json:"renotifyConfig,omitempty"`
}

type notificationRuleAPITargetConfig struct {
	Type            string   `json:"type"`
	Channel         string   `json:"channel,omitempty"`
	Addresses       []string `json:"addresses,omitempty"`
	IntegrationID   string   `json:"integrationId,omitempty"`
	IntegrationName string   `json:"integrationName,omitempty"`
}

type notificationRuleAPITargetRateLimit struct {
	MaxMessages int64 `json:"maxMessages"`
	Minutes     int64 `json:"minutes"`
}

type notificationRuleAPITargetRenotify struct {
	Mode                    string   `json:"mode"`
	RenotificationStates    []string `json:"renotificationStates"`
	RenotifyIntervalMinutes int64    `json:"renotifyIntervalMinutes"`
}

func expandNotificationRuleTargets(ctx context.Context, targets types.List) ([]notificationRuleAPITarget, diag.Diagnostics) {
	var diags diag.Diagnostics
	if targets.IsNull() || targets.IsUnknown() {
		return nil, diags
	}

	var targetModels []resource_notification_rule.TargetModel
	diags.Append(targets.ElementsAs(ctx, &targetModels, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]notificationRuleAPITarget, 0, len(targetModels))
	for _, t := range targetModels {
		apiTarget := notificationRuleAPITarget{
			ID: t.Id.ValueString(),
		}

		conf, confDiags := expandTargetConfig(ctx, t.Config)
		diags.Append(confDiags...)
		if diags.HasError() {
			return nil, diags
		}
		apiTarget.Config = conf

		if t.RateLimit != nil {
			apiTarget.RateLimit = &notificationRuleAPITargetRateLimit{
				MaxMessages: t.RateLimit.MaxMessages.ValueInt64(),
				Minutes:     t.RateLimit.Minutes.ValueInt64(),
			}
		}

		if t.RenotifyConfig != nil {
			if !t.RenotifyConfig.RenotificationStates.IsNull() && !t.RenotifyConfig.RenotificationStates.IsUnknown() {
				renotifyStates, renotifyDiags := expandStringList(ctx, t.RenotifyConfig.RenotificationStates)
				diags.Append(renotifyDiags...)
				if diags.HasError() {
					return nil, diags
				}
				apiTarget.RenotifyConfig = &notificationRuleAPITargetRenotify{
					Mode:                    t.RenotifyConfig.Mode.ValueString(),
					RenotificationStates:    renotifyStates,
					RenotifyIntervalMinutes: t.RenotifyConfig.RenotifyIntervalMinutes.ValueInt64(),
				}
			}
		}

		result = append(result, apiTarget)
	}

	return result, diags
}

func flattenNotificationRule(ctx context.Context, data notificationRuleAPIData) (resource_notification_rule.NotificationRuleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tags, tagDiags := flattenTags(ctx, data.Tags)
	diags.Append(tagDiags...)

	targets, targetDiags := flattenNotificationRuleTargets(ctx, data.Targets)
	diags.Append(targetDiags...)

	teamsFilter, teamDiags := types.ListValueFrom(ctx, types.StringType, data.TeamsFilter)
	diags.Append(teamDiags...)
	prioritiesFilter, priorityDiags := types.ListValueFrom(ctx, types.Int64Type, data.PrioritiesFilter)
	diags.Append(priorityDiags...)
	transitionTypesFilter, transitionDiags := types.ListValueFrom(ctx, types.StringType, data.TransitionTypesFilter)
	diags.Append(transitionDiags...)

	state := resource_notification_rule.NotificationRuleModel{
		Id:                    types.StringValue(data.ID),
		Name:                  types.StringValue(data.Name),
		TeamsFilter:           teamsFilter,
		PrioritiesFilter:      prioritiesFilter,
		TransitionTypesFilter: transitionTypesFilter,
		Owner:                 types.StringValue(data.Owner),
		IsActive:              types.BoolValue(data.IsActive),
		Tags:                  tags,
		Targets:               targets,
	}

	return state, diags
}

func flattenNotificationRuleTargets(ctx context.Context, targets []notificationRuleAPITarget) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_notification_rule.TargetAttrTypes(ctx)}
	if len(targets) == 0 {
		return types.ListNull(elemType), nil
	}

	configTypes := resource_notification_rule.TargetConfigAttrTypes(ctx)
	rateLimitType := types.ObjectType{AttrTypes: resource_notification_rule.TargetRateLimitAttrTypes(ctx)}
	renotifyType := types.ObjectType{AttrTypes: resource_notification_rule.TargetRenotifyAttrTypes(ctx)}

	values := make([]attr.Value, 0, len(targets))
	for _, t := range targets {
		configValues := map[string]attr.Value{
			"slack":           types.ObjectNull(resource_notification_rule.SlackAttrTypes(ctx)),
			"incident_io":     types.ObjectNull(resource_notification_rule.IntegrationConfigAttrTypes(ctx)),
			"pagerduty":       types.ObjectNull(resource_notification_rule.IntegrationConfigAttrTypes(ctx)),
			"email":           types.ObjectNull(resource_notification_rule.EmailAttrTypes(ctx)),
			"grafana_irm":     types.ObjectNull(resource_notification_rule.IntegrationConfigAttrTypes(ctx)),
			"microsoft_teams": types.ObjectNull(resource_notification_rule.IntegrationConfigAttrTypes(ctx)),
			"webhook":         types.ObjectNull(resource_notification_rule.IntegrationConfigAttrTypes(ctx)),
		}

		switch t.Config.Type {
		case "slack":
			configValues["slack"] = types.ObjectValueMust(resource_notification_rule.SlackAttrTypes(ctx), map[string]attr.Value{
				"type":             types.StringValue("slack"),
				"channel":          types.StringValue(t.Config.Channel),
				"integration_id":   types.StringValue(t.Config.IntegrationID),
				"integration_name": stringValueOrNull(t.Config.IntegrationName),
			})
		case "incident-io":
			configValues["incident_io"] = types.ObjectValueMust(resource_notification_rule.IntegrationConfigAttrTypes(ctx), map[string]attr.Value{
				"type":             types.StringValue("incident-io"),
				"integration_id":   types.StringValue(t.Config.IntegrationID),
				"integration_name": stringValueOrNull(t.Config.IntegrationName),
			})
		case "pagerduty":
			configValues["pagerduty"] = types.ObjectValueMust(resource_notification_rule.IntegrationConfigAttrTypes(ctx), map[string]attr.Value{
				"type":             types.StringValue("pagerduty"),
				"integration_id":   types.StringValue(t.Config.IntegrationID),
				"integration_name": stringValueOrNull(t.Config.IntegrationName),
			})
		case "grafana-irm":
			configValues["grafana_irm"] = types.ObjectValueMust(resource_notification_rule.IntegrationConfigAttrTypes(ctx), map[string]attr.Value{
				"type":             types.StringValue("grafana-irm"),
				"integration_id":   types.StringValue(t.Config.IntegrationID),
				"integration_name": stringValueOrNull(t.Config.IntegrationName),
			})
		case "microsoft-teams":
			configValues["microsoft_teams"] = types.ObjectValueMust(resource_notification_rule.IntegrationConfigAttrTypes(ctx), map[string]attr.Value{
				"type":             types.StringValue("microsoft-teams"),
				"integration_id":   types.StringValue(t.Config.IntegrationID),
				"integration_name": stringValueOrNull(t.Config.IntegrationName),
			})
		case "webhook":
			configValues["webhook"] = types.ObjectValueMust(resource_notification_rule.IntegrationConfigAttrTypes(ctx), map[string]attr.Value{
				"type":             types.StringValue("webhook"),
				"integration_id":   types.StringValue(t.Config.IntegrationID),
				"integration_name": stringValueOrNull(t.Config.IntegrationName),
			})
		case "email":
			addresses, diags := types.ListValueFrom(ctx, types.StringType, t.Config.Addresses)
			if diags.HasError() {
				return types.ListNull(elemType), diags
			}
			configValues["email"] = types.ObjectValueMust(resource_notification_rule.EmailAttrTypes(ctx), map[string]attr.Value{
				"type":      types.StringValue("email"),
				"addresses": addresses,
			})
		}

		rateLimitValue := types.ObjectNull(rateLimitType.AttrTypes)
		if t.RateLimit != nil {
			rateLimitValue = types.ObjectValueMust(rateLimitType.AttrTypes, map[string]attr.Value{
				"max_messages": types.Int64Value(t.RateLimit.MaxMessages),
				"minutes":      types.Int64Value(t.RateLimit.Minutes),
			})
		}

		renotifyValue := types.ObjectNull(renotifyType.AttrTypes)
		if t.RenotifyConfig != nil {
			renotifyStates, diags := types.ListValueFrom(ctx, types.StringType, t.RenotifyConfig.RenotificationStates)
			if diags.HasError() {
				return types.ListNull(elemType), diags
			}
			renotifyValue = types.ObjectValueMust(renotifyType.AttrTypes, map[string]attr.Value{
				"mode":                      types.StringValue(t.RenotifyConfig.Mode),
				"renotification_states":     renotifyStates,
				"renotify_interval_minutes": types.Int64Value(t.RenotifyConfig.RenotifyIntervalMinutes),
			})
		}

		values = append(values, types.ObjectValueMust(resource_notification_rule.TargetAttrTypes(ctx), map[string]attr.Value{
			"id":              types.StringValue(t.ID),
			"config":          types.ObjectValueMust(configTypes, configValues),
			"rate_limit":      rateLimitValue,
			"renotify_config": renotifyValue,
		}))
	}

	return types.ListValue(elemType, values)
}

func expandTargetConfig(ctx context.Context, cfg resource_notification_rule.TargetConfigModel) (notificationRuleAPITargetConfig, diag.Diagnostics) {
	var diags diag.Diagnostics

	setCount := 0
	if cfg.Slack != nil {
		setCount++
	}
	if cfg.IncidentIO != nil {
		setCount++
	}
	if cfg.PagerDuty != nil {
		setCount++
	}
	if cfg.Email != nil {
		setCount++
	}
	if cfg.GrafanaIRM != nil {
		setCount++
	}
	if cfg.MicrosoftTeams != nil {
		setCount++
	}
	if cfg.Webhook != nil {
		setCount++
	}

	if setCount != 1 {
		diags.AddError("Invalid target config", "Exactly one of slack, incident_io, pagerduty, email, grafana_irm, microsoft_teams, webhook must be set in config.")
		return notificationRuleAPITargetConfig{}, diags
	}

	switch {
	case cfg.Slack != nil:
		conf := notificationRuleAPITargetConfig{
			Type:          "slack",
			Channel:       cfg.Slack.Channel.ValueString(),
			IntegrationID: cfg.Slack.IntegrationID.ValueString(),
		}
		if !cfg.Slack.IntegrationName.IsNull() && !cfg.Slack.IntegrationName.IsUnknown() {
			conf.IntegrationName = cfg.Slack.IntegrationName.ValueString()
		}
		return conf, diags
	case cfg.IncidentIO != nil:
		conf := notificationRuleAPITargetConfig{
			Type:          "incident-io",
			IntegrationID: cfg.IncidentIO.IntegrationID.ValueString(),
		}
		if !cfg.IncidentIO.IntegrationName.IsNull() && !cfg.IncidentIO.IntegrationName.IsUnknown() {
			conf.IntegrationName = cfg.IncidentIO.IntegrationName.ValueString()
		}
		return conf, diags
	case cfg.PagerDuty != nil:
		conf := notificationRuleAPITargetConfig{
			Type:          "pagerduty",
			IntegrationID: cfg.PagerDuty.IntegrationID.ValueString(),
		}
		if !cfg.PagerDuty.IntegrationName.IsNull() && !cfg.PagerDuty.IntegrationName.IsUnknown() {
			conf.IntegrationName = cfg.PagerDuty.IntegrationName.ValueString()
		}
		return conf, diags
	case cfg.Email != nil:
		addresses, addrDiags := expandStringList(ctx, cfg.Email.Addresses)
		diags.Append(addrDiags...)
		return notificationRuleAPITargetConfig{
			Type:      "email",
			Addresses: addresses,
		}, diags
	case cfg.GrafanaIRM != nil:
		conf := notificationRuleAPITargetConfig{
			Type:          "grafana-irm",
			IntegrationID: cfg.GrafanaIRM.IntegrationID.ValueString(),
		}
		if !cfg.GrafanaIRM.IntegrationName.IsNull() && !cfg.GrafanaIRM.IntegrationName.IsUnknown() {
			conf.IntegrationName = cfg.GrafanaIRM.IntegrationName.ValueString()
		}
		return conf, diags
	case cfg.MicrosoftTeams != nil:
		conf := notificationRuleAPITargetConfig{
			Type:          "microsoft-teams",
			IntegrationID: cfg.MicrosoftTeams.IntegrationID.ValueString(),
		}
		if !cfg.MicrosoftTeams.IntegrationName.IsNull() && !cfg.MicrosoftTeams.IntegrationName.IsUnknown() {
			conf.IntegrationName = cfg.MicrosoftTeams.IntegrationName.ValueString()
		}
		return conf, diags
	case cfg.Webhook != nil:
		conf := notificationRuleAPITargetConfig{
			Type:          "webhook",
			IntegrationID: cfg.Webhook.IntegrationID.ValueString(),
		}
		if !cfg.Webhook.IntegrationName.IsNull() && !cfg.Webhook.IntegrationName.IsUnknown() {
			conf.IntegrationName = cfg.Webhook.IntegrationName.ValueString()
		}
		return conf, diags
	default:
		diags.AddError("Invalid target config", "Exactly one config block must be set for each target.")
		return notificationRuleAPITargetConfig{}, diags
	}
}
