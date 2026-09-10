package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"terraform-provider-tsuga/internal/resource_notification_integration"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*notificationIntegrationResource)(nil)
var _ resource.ResourceWithConfigure = (*notificationIntegrationResource)(nil)
var _ resource.ResourceWithImportState = (*notificationIntegrationResource)(nil)
var _ resource.ResourceWithValidateConfig = (*notificationIntegrationResource)(nil)

func NewNotificationIntegrationResource() resource.Resource {
	return &notificationIntegrationResource{}
}

type notificationIntegrationResource struct {
	client *TsugaClient
}

func (r *notificationIntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *notificationIntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_integration"
}

func (r *notificationIntegrationResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_notification_integration.NotificationIntegrationResourceSchema(ctx)
}

func (r *notificationIntegrationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_notification_integration.NotificationIntegrationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Setting == nil {
		return
	}

	if !exactlyOneSet(
		config.Setting.GoogleChat != nil,
		config.Setting.GrafanaIrm != nil,
		config.Setting.IncidentIo != nil,
		config.Setting.Jira != nil,
		config.Setting.MicrosoftTeams != nil,
		config.Setting.Pagerduty != nil,
		config.Setting.Servicenow != nil,
		config.Setting.Squadcast != nil,
		config.Setting.Webhook != nil,
	) {
		resp.Diagnostics.AddAttributeError(
			path.Root("setting"),
			"Invalid setting configuration",
			"Exactly one of google_chat, grafana_irm, incident_io, jira, microsoft_teams, pagerduty, servicenow, squadcast, or webhook must be set.",
		)
	}

	if config.Setting.Webhook != nil && config.Setting.Webhook.Authentication != nil {
		auth := config.Setting.Webhook.Authentication
		if !exactlyOneSet(auth.Bearer != nil, auth.Basic != nil, auth.CustomHeader != nil, auth.None != nil) {
			resp.Diagnostics.AddAttributeError(
				path.Root("setting").AtName("webhook").AtName("authentication"),
				"Invalid authentication configuration",
				"Exactly one of bearer, basic, custom_header, or none must be set.",
			)
		}
	}
}

func exactlyOneSet(flags ...bool) bool {
	count := 0
	for _, flag := range flags {
		if flag {
			count++
		}
	}
	return count == 1
}

func (r *notificationIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *notificationIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config resource_notification_integration.NotificationIntegrationModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildNotificationIntegrationRequestBody(ctx, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdateNotificationIntegration(ctx, http.MethodPost, "/v1/notification-integrations", requestBody, &plan, "create")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *notificationIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_notification_integration.NotificationIntegrationModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/notification-integrations/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read notification integration: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read notification integration: %s", err))
		return
	}

	r.readJSON(ctx, httpResp.Body, resp)
}

func (r *notificationIntegrationResource) readJSON(ctx context.Context, bodyReader io.Reader, resp *resource.ReadResponse) {
	var state resource_notification_integration.NotificationIntegrationModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data, err := decodeNotificationIntegration(bodyReader)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", err.Error())
		return
	}

	newState, diags := flattenNotificationIntegration(ctx, data, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func decodeNotificationIntegration(bodyReader io.Reader) (notificationIntegrationAPIData, error) {
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return notificationIntegrationAPIData{}, fmt.Errorf("unable to read response body: %s", err)
	}
	var apiResp notificationIntegrationAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return notificationIntegrationAPIData{}, fmt.Errorf("unable to parse response: %s", err)
	}
	return apiResp.Data, nil
}

func (r *notificationIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, config, state resource_notification_integration.NotificationIntegrationModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildNotificationIntegrationRequestBody(ctx, config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The API keeps the stored mapping when the key is omitted, so an absent block must send null to unset it.
	if config.Setting != nil && config.Setting.Webhook != nil && config.Setting.Webhook.EventNameMapping == nil {
		requestBody["setting"].(map[string]any)["eventNameMapping"] = nil
	}

	apiPath := fmt.Sprintf("/v1/notification-integrations/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateNotificationIntegration(ctx, http.MethodPut, apiPath, requestBody, &plan, "update")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *notificationIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_notification_integration.NotificationIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/notification-integrations/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, apiPath, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete notification integration: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete notification integration: %s", err))
			return
		}
	}
}

// Built from the config rather than the plan: write-only secrets are only present in the config.
func (r *notificationIntegrationResource) buildNotificationIntegrationRequestBody(ctx context.Context, config resource_notification_integration.NotificationIntegrationModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	setting, settingDiags := expandIntegrationSetting(ctx, config.Setting)
	diags.Append(settingDiags...)
	if diags.HasError() {
		return nil, diags
	}

	requestBody := map[string]any{
		"name":    config.Name.ValueString(),
		"setting": setting,
	}

	if tags, tagDiags := expandTags(ctx, config.Tags); tagDiags.HasError() {
		diags.Append(tagDiags...)
		return nil, diags
	} else if tags != nil {
		requestBody["tags"] = tags
	}

	return requestBody, diags
}

func (r *notificationIntegrationResource) createOrUpdateNotificationIntegration(ctx context.Context, method, path string, requestBody map[string]any, plan *resource_notification_integration.NotificationIntegrationModel, operation string) (resource_notification_integration.NotificationIntegrationModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s notification integration: %s", operation, err))
		return resource_notification_integration.NotificationIntegrationModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s notification integration: %s", operation, err))
		return resource_notification_integration.NotificationIntegrationModel{}, diags
	}

	data, err := decodeNotificationIntegration(httpResp.Body)
	if err != nil {
		diags.AddError("Parse Error", err.Error())
		return resource_notification_integration.NotificationIntegrationModel{}, diags
	}

	newState, flattenDiags := flattenNotificationIntegration(ctx, data, plan)
	diags.Append(flattenDiags...)
	if diags.HasError() {
		return resource_notification_integration.NotificationIntegrationModel{}, diags
	}

	return newState, diags
}

type notificationIntegrationAPIResponse struct {
	Data notificationIntegrationAPIData `json:"data"`
}

type notificationIntegrationAPIData struct {
	ID      string                 `json:"id"`
	Name    string                 `json:"name"`
	Setting map[string]interface{} `json:"setting"`
	Tags    []apiTag               `json:"tags"`
}

// expandIntegrationSetting builds the API request body for the single setting variant that is
// set on the plan. ValidateConfig already guarantees exactly one is set.
func expandIntegrationSetting(ctx context.Context, setting *resource_notification_integration.IntegrationSettingModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if setting == nil {
		diags.AddError("Invalid setting configuration", "setting is required.")
		return nil, diags
	}

	switch {
	case setting.GoogleChat != nil:
		return map[string]any{
			"type":       "google-chat",
			"webhookUrl": setting.GoogleChat.WebhookUrl.ValueString(),
		}, diags
	case setting.GrafanaIrm != nil:
		return map[string]any{
			"type": "grafana-irm",
			"url":  setting.GrafanaIrm.WebhookUrl.ValueString(),
		}, diags
	case setting.IncidentIo != nil:
		return map[string]any{
			"type":                "incident-io",
			"alertSourceConfigId": setting.IncidentIo.AlertSourceConfigId.ValueString(),
			"token":               setting.IncidentIo.Token.ValueString(),
		}, diags
	case setting.Jira != nil:
		return map[string]any{
			"type":     "jira",
			"siteUrl":  setting.Jira.SiteUrl.ValueString(),
			"email":    setting.Jira.Email.ValueString(),
			"apiToken": setting.Jira.ApiToken.ValueString(),
		}, diags
	case setting.MicrosoftTeams != nil:
		return map[string]any{
			"type":       "microsoft-teams",
			"webhookUrl": setting.MicrosoftTeams.WebhookUrl.ValueString(),
		}, diags
	case setting.Pagerduty != nil:
		return map[string]any{
			"type":           "pagerduty",
			"integrationKey": setting.Pagerduty.IntegrationKey.ValueString(),
		}, diags
	case setting.Servicenow != nil:
		return map[string]any{
			"type":         "servicenow",
			"instanceName": setting.Servicenow.InstanceName.ValueString(),
			"username":     setting.Servicenow.Username.ValueString(),
			"password":     setting.Servicenow.Password.ValueString(),
		}, diags
	case setting.Squadcast != nil:
		return map[string]any{
			"type":       "squadcast",
			"webhookUrl": setting.Squadcast.WebhookUrl.ValueString(),
		}, diags
	case setting.Webhook != nil:
		return expandWebhookSetting(ctx, setting.Webhook)
	}

	diags.AddError("Invalid setting configuration", "Exactly one of google_chat, grafana_irm, incident_io, jira, microsoft_teams, pagerduty, servicenow, squadcast, or webhook must be set.")
	return nil, diags
}

func expandWebhookSetting(ctx context.Context, webhook *resource_notification_integration.WebhookSettingModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	result := map[string]any{
		"type":   "webhook",
		"url":    webhook.Url.ValueString(),
		"method": webhook.Method.ValueString(),
	}

	if a := webhook.Authentication; a != nil {
		switch {
		case a.Bearer != nil:
			result["authentication"] = map[string]any{"type": "bearer", "token": a.Bearer.Token.ValueString()}
		case a.Basic != nil:
			result["authentication"] = map[string]any{"type": "basic", "username": a.Basic.Username.ValueString(), "password": a.Basic.Password.ValueString()}
		case a.CustomHeader != nil:
			result["authentication"] = map[string]any{"type": "custom-header", "key": a.CustomHeader.Key.ValueString(), "value": a.CustomHeader.Value.ValueString()}
		case a.None != nil:
			result["authentication"] = map[string]any{"type": "none"}
		}
	}

	// The API requires customHeaders, so an omitted Terraform value has to send an empty map.
	headers := make(map[string]string, len(webhook.CustomHeaders.Elements()))
	if !webhook.CustomHeaders.IsNull() && !webhook.CustomHeaders.IsUnknown() {
		diags.Append(webhook.CustomHeaders.ElementsAs(ctx, &headers, false)...)
	}
	result["customHeaders"] = headers

	if webhook.PayloadTemplate != nil {
		var value any
		if err := json.Unmarshal([]byte(webhook.PayloadTemplate.Value.ValueString()), &value); err != nil {
			diags.AddAttributeError(
				path.Root("setting").AtName("webhook").AtName("payload_template").AtName("value"),
				"Invalid payload template",
				fmt.Sprintf("value must be valid JSON: %s", err),
			)
			return nil, diags
		}
		result["payloadTemplate"] = map[string]any{
			"type":  webhook.PayloadTemplate.Type.ValueString(),
			"value": value,
		}
	}

	if webhook.EventNameMapping != nil {
		result["eventNameMapping"] = map[string]any{
			"ok":    webhook.EventNameMapping.Ok.ValueString(),
			"alert": webhook.EventNameMapping.Alert.ValueString(),
		}
	}

	return result, diags
}

// flattenNotificationIntegration builds the resource state from an API response. The API never
// returns secret fields (tokens, passwords, webhook URLs) in full, only redacted "last chars"
// forms, so secret fields are carried over from priorSetting (the plan just submitted, or the
// previous state on a plain Read) instead of the API response.
// prior is the plan on create/update and the previous state on read; it supplies the values the
// API does not return, such as the configured payload template text.
func flattenNotificationIntegration(ctx context.Context, data notificationIntegrationAPIData, prior *resource_notification_integration.NotificationIntegrationModel) (resource_notification_integration.NotificationIntegrationModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tags, tagDiags := flattenTags(ctx, data.Tags)
	diags.Append(tagDiags...)

	var priorSetting *resource_notification_integration.IntegrationSettingModel
	secretsVersion := types.StringNull()
	if prior != nil {
		priorSetting = prior.Setting
		secretsVersion = prior.SecretsVersion
	}
	setting, settingDiags := flattenIntegrationSetting(data.Setting, priorSetting)
	diags.Append(settingDiags...)

	state := resource_notification_integration.NotificationIntegrationModel{
		Id:             types.StringValue(data.ID),
		Name:           types.StringValue(data.Name),
		Tags:           tags,
		SecretsVersion: secretsVersion,
		Setting:        setting,
	}

	return state, diags
}

func flattenIntegrationSetting(apiSetting map[string]interface{}, prior *resource_notification_integration.IntegrationSettingModel) (*resource_notification_integration.IntegrationSettingModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	settingType, _ := apiSetting["type"].(string)
	result := &resource_notification_integration.IntegrationSettingModel{}

	switch settingType {
	case "google-chat":
		result.GoogleChat = &resource_notification_integration.WebhookUrlSettingModel{
			WebhookUrl: writeOnlySecret,
		}
	case "grafana-irm":
		result.GrafanaIrm = &resource_notification_integration.WebhookUrlSettingModel{
			WebhookUrl: writeOnlySecret,
		}
	case "incident-io":
		alertSourceConfigId, _ := apiSetting["alertSourceConfigId"].(string)
		result.IncidentIo = &resource_notification_integration.IncidentIoSettingModel{
			AlertSourceConfigId: types.StringValue(alertSourceConfigId),
			Token:               writeOnlySecret,
		}
	case "jira":
		siteUrl, _ := apiSetting["siteUrl"].(string)
		email, _ := apiSetting["email"].(string)
		result.Jira = &resource_notification_integration.JiraSettingModel{
			SiteUrl:  types.StringValue(siteUrl),
			Email:    types.StringValue(email),
			ApiToken: writeOnlySecret,
		}
	case "microsoft-teams":
		result.MicrosoftTeams = &resource_notification_integration.WebhookUrlSettingModel{
			WebhookUrl: writeOnlySecret,
		}
	case "pagerduty":
		result.Pagerduty = &resource_notification_integration.PagerdutySettingModel{
			IntegrationKey: writeOnlySecret,
		}
	case "servicenow":
		instanceName, _ := apiSetting["instanceName"].(string)
		username, _ := apiSetting["username"].(string)
		result.Servicenow = &resource_notification_integration.ServicenowSettingModel{
			InstanceName: types.StringValue(instanceName),
			Username:     types.StringValue(username),
			Password:     writeOnlySecret,
		}
	case "squadcast":
		result.Squadcast = &resource_notification_integration.WebhookUrlSettingModel{
			WebhookUrl: writeOnlySecret,
		}
	case "webhook":
		var webhookDiags diag.Diagnostics
		var priorWebhook *resource_notification_integration.WebhookSettingModel
		if prior != nil {
			priorWebhook = prior.Webhook
		}
		result.Webhook, webhookDiags = flattenWebhookSetting(apiSetting, priorWebhook)
		diags.Append(webhookDiags...)
	default:
		diags.AddError("API Error", fmt.Sprintf("Unknown notification integration setting type %q returned by the API.", settingType))
	}

	return result, diags
}

// priorStringOrEmpty reads a secret-carrying field off the prior setting (the plan just applied,
// or the previous state on a plain Read). It returns an empty string, never null, so a fresh
// `terraform import` with no prior secret produces a diff instead of a permanently null attribute.
// Write-only secrets never reach the state: the framework nulls them on the way out. The empty
// string is for the Terraform export, which reads this model directly and needs a non-null value
// to emit a variable for the secret.
var writeOnlySecret = types.StringValue("")

func flattenWebhookSetting(apiSetting map[string]interface{}, prior *resource_notification_integration.WebhookSettingModel) (*resource_notification_integration.WebhookSettingModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	method, _ := apiSetting["method"].(string)

	result := &resource_notification_integration.WebhookSettingModel{
		Url:    writeOnlySecret,
		Method: types.StringValue(method),
	}

	if authRaw, ok := apiSetting["authentication"].(map[string]interface{}); ok {
		result.Authentication = flattenWebhookAuthentication(authRaw)
	}

	if headersRaw, ok := apiSetting["customHeaders"].(map[string]interface{}); ok && len(headersRaw) > 0 {
		headers := make(map[string]attr.Value, len(headersRaw))
		for k, v := range headersRaw {
			s, _ := v.(string)
			headers[k] = types.StringValue(s)
		}
		mapValue, mapDiags := types.MapValue(types.StringType, headers)
		diags.Append(mapDiags...)
		result.CustomHeaders = mapValue
	} else if prior != nil && !prior.CustomHeaders.IsNull() && !prior.CustomHeaders.IsUnknown() &&
		len(prior.CustomHeaders.Elements()) == 0 {
		// An explicitly configured empty map is not the same as an absent one.
		result.CustomHeaders = prior.CustomHeaders
	} else {
		result.CustomHeaders = types.MapNull(types.StringType)
	}

	if payloadTemplateRaw, ok := apiSetting["payloadTemplate"].(map[string]interface{}); ok {
		templateType, _ := payloadTemplateRaw["type"].(string)
		valueBytes, err := json.Marshal(payloadTemplateRaw["value"])
		if err != nil {
			diags.AddError("API Error", fmt.Sprintf("Unable to encode payload template value: %s", err))
		} else {
			// The API stores the template decoded, so keep the configured text when it holds the same value.
			value := string(valueBytes)
			if prior != nil && prior.PayloadTemplate != nil {
				priorValue := prior.PayloadTemplate.Value.ValueString()
				var priorDecoded any
				if json.Unmarshal([]byte(priorValue), &priorDecoded) == nil &&
					reflect.DeepEqual(priorDecoded, payloadTemplateRaw["value"]) {
					value = priorValue
				}
			}
			result.PayloadTemplate = &resource_notification_integration.WebhookPayloadTemplateModel{
				Type:  types.StringValue(templateType),
				Value: types.StringValue(value),
			}
		}
	}

	if mappingRaw, ok := apiSetting["eventNameMapping"].(map[string]interface{}); ok {
		ok1, _ := mappingRaw["ok"].(string)
		alert, _ := mappingRaw["alert"].(string)
		result.EventNameMapping = &resource_notification_integration.WebhookEventNameMappingModel{
			Ok:    types.StringValue(ok1),
			Alert: types.StringValue(alert),
		}
	}

	return result, diags
}

func flattenWebhookAuthentication(authRaw map[string]interface{}) *resource_notification_integration.WebhookAuthenticationModel {
	apiString := func(field string) types.String {
		value, _ := authRaw[field].(string)
		return types.StringValue(value)
	}

	switch authRaw["type"] {
	case "none":
		return &resource_notification_integration.WebhookAuthenticationModel{
			None: &resource_notification_integration.WebhookNoneAuthModel{},
		}
	case "bearer":
		return &resource_notification_integration.WebhookAuthenticationModel{
			Bearer: &resource_notification_integration.WebhookBearerAuthModel{Token: writeOnlySecret},
		}
	case "basic":
		return &resource_notification_integration.WebhookAuthenticationModel{
			Basic: &resource_notification_integration.WebhookBasicAuthModel{Username: apiString("username"), Password: writeOnlySecret},
		}
	case "custom-header":
		return &resource_notification_integration.WebhookAuthenticationModel{
			CustomHeader: &resource_notification_integration.WebhookCustomHeaderAuthModel{Key: apiString("key"), Value: writeOnlySecret},
		}
	}
	return nil
}
