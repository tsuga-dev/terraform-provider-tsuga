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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.Resource = (*notificationIntegrationResource)(nil)
var _ resource.ResourceWithConfigure = (*notificationIntegrationResource)(nil)
var _ resource.ResourceWithImportState = (*notificationIntegrationResource)(nil)

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
	markWriteOnlySecrets(resp.Schema.Attributes)
}

// The API never returns a secret, only its `<name>_last_chars`. Secrets are marked sensitive in the
// generated schema (see patch-provider-code-spec.js); every sensitive string is also write-only
// here, which the code spec cannot express.
func markWriteOnlySecrets(attributes map[string]schema.Attribute) {
	for name, attribute := range attributes {
		switch a := attribute.(type) {
		case schema.StringAttribute:
			if a.Sensitive {
				a.WriteOnly = true
				attributes[name] = a
			}
		case schema.SingleNestedAttribute:
			markWriteOnlySecrets(a.Attributes)
		}
	}
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

	newState, diags := r.createOrUpdateNotificationIntegration(ctx, http.MethodPost, "/v1/notification-integrations", requestBody, plan, "create")
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

	newState, diags := flattenNotificationIntegration(ctx, data, state)
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
	if setting := requestBody["setting"].(map[string]any); setting["type"] == "webhook" {
		if _, ok := setting["eventNameMapping"]; !ok {
			setting["eventNameMapping"] = nil
		}
	}

	apiPath := fmt.Sprintf("/v1/notification-integrations/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateNotificationIntegration(ctx, http.MethodPut, apiPath, requestBody, plan, "update")
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
		"owner":   config.Owner.ValueString(),
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

func (r *notificationIntegrationResource) createOrUpdateNotificationIntegration(ctx context.Context, method, path string, requestBody map[string]any, plan resource_notification_integration.NotificationIntegrationModel, operation string) (resource_notification_integration.NotificationIntegrationModel, diag.Diagnostics) {
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
	Owner   string                 `json:"owner"`
	Setting map[string]interface{} `json:"setting"`
	Tags    []apiTag               `json:"tags"`
}

// writeOnlySecret stands in for a secret in the flattened state. Write-only secrets never reach the
// state (the framework nulls them on the way out); the empty string is for the Terraform export,
// which reads the flattened model directly and needs a non-null value to emit a variable.
var writeOnlySecret = types.StringValue("")

type integrationBlock interface {
	basetypes.ObjectValuable
	AttributeTypes(context.Context) map[string]attr.Type
}

func isSet(object basetypes.ObjectValue) bool {
	return !object.IsNull() && !object.IsUnknown()
}

// typedBlock reads a nested object as its generated value type.
func typedBlock[T integrationBlock](ctx context.Context, object basetypes.ObjectValue, newValue func(map[string]attr.Type, map[string]attr.Value) (T, diag.Diagnostics)) (T, diag.Diagnostics) {
	return newValue(object.AttributeTypes(ctx), object.Attributes())
}

// newBlock builds a generated value type from its attributes, as the object its parent holds.
func newBlock[T integrationBlock](ctx context.Context, newValue func(map[string]attr.Type, map[string]attr.Value) (T, diag.Diagnostics), values map[string]attr.Value) (basetypes.ObjectValue, diag.Diagnostics) {
	var zero T
	value, diags := newValue(zero.AttributeTypes(ctx), values)
	if diags.HasError() {
		return basetypes.ObjectValue{}, diags
	}
	object, objectDiags := value.ToObjectValue(ctx)
	return object, append(diags, objectDiags...)
}

func nullBlock[T integrationBlock](ctx context.Context) basetypes.ObjectValue {
	var zero T
	object, _ := zero.ToObjectValue(ctx)
	return object
}

func putString(m map[string]any, key string, value basetypes.StringValue) {
	if !value.IsNull() && !value.IsUnknown() {
		m[key] = value.ValueString()
	}
}

func apiString(m map[string]interface{}, key string) types.String {
	text, _ := m[key].(string)
	return stringValueOrNull(text)
}

// expandIntegrationSetting builds the API request body for the single setting variant that is
// set. The ExactlyOneOf validator already guarantees exactly one is set.
func expandIntegrationSetting(ctx context.Context, setting resource_notification_integration.SettingValue) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := map[string]any{}

	switch {
	case isSet(setting.GoogleChat):
		block, d := typedBlock(ctx, setting.GoogleChat, resource_notification_integration.NewGoogleChatValue)
		diags.Append(d...)
		result["type"] = "google-chat"
		putString(result, "webhookUrl", block.WebhookUrl)
	case isSet(setting.GrafanaIrm):
		block, d := typedBlock(ctx, setting.GrafanaIrm, resource_notification_integration.NewGrafanaIrmValue)
		diags.Append(d...)
		result["type"] = "grafana-irm"
		putString(result, "url", block.Url)
	case isSet(setting.IncidentIo):
		block, d := typedBlock(ctx, setting.IncidentIo, resource_notification_integration.NewIncidentIoValue)
		diags.Append(d...)
		result["type"] = "incident-io"
		putString(result, "alertSourceConfigId", block.AlertSourceConfigId)
		putString(result, "token", block.Token)
	case isSet(setting.Jira):
		block, d := typedBlock(ctx, setting.Jira, resource_notification_integration.NewJiraValue)
		diags.Append(d...)
		result["type"] = "jira"
		putString(result, "apiToken", block.ApiToken)
		putString(result, "email", block.Email)
		putString(result, "siteUrl", block.SiteUrl)
	case isSet(setting.MicrosoftTeams):
		block, d := typedBlock(ctx, setting.MicrosoftTeams, resource_notification_integration.NewMicrosoftTeamsValue)
		diags.Append(d...)
		result["type"] = "microsoft-teams"
		putString(result, "webhookUrl", block.WebhookUrl)
	case isSet(setting.Pagerduty):
		block, d := typedBlock(ctx, setting.Pagerduty, resource_notification_integration.NewPagerdutyValue)
		diags.Append(d...)
		result["type"] = "pagerduty"
		putString(result, "integrationKey", block.IntegrationKey)
	case isSet(setting.Servicenow):
		block, d := typedBlock(ctx, setting.Servicenow, resource_notification_integration.NewServicenowValue)
		diags.Append(d...)
		result["type"] = "servicenow"
		putString(result, "instanceName", block.InstanceName)
		putString(result, "password", block.Password)
		putString(result, "username", block.Username)
	case isSet(setting.Squadcast):
		block, d := typedBlock(ctx, setting.Squadcast, resource_notification_integration.NewSquadcastValue)
		diags.Append(d...)
		result["type"] = "squadcast"
		putString(result, "webhookUrl", block.WebhookUrl)
	case isSet(setting.Webhook):
		block, d := typedBlock(ctx, setting.Webhook, resource_notification_integration.NewWebhookValue)
		diags.Append(d...)
		diags.Append(expandWebhook(ctx, block, result)...)
	default:
		diags.AddError("Invalid setting configuration", "Exactly one variant must be set.")
	}

	if diags.HasError() {
		return nil, diags
	}
	return result, diags
}

func expandWebhook(ctx context.Context, webhook resource_notification_integration.WebhookValue, result map[string]any) diag.Diagnostics {
	var diags diag.Diagnostics

	result["type"] = "webhook"
	putString(result, "url", webhook.Url)
	putString(result, "method", webhook.Method)

	// The API requires customHeaders, so an omitted Terraform value has to send an empty map.
	headers := map[string]string{}
	if !webhook.CustomHeaders.IsNull() && !webhook.CustomHeaders.IsUnknown() {
		diags.Append(webhook.CustomHeaders.ElementsAs(ctx, &headers, false)...)
	}
	result["customHeaders"] = headers

	if isSet(webhook.Authentication) {
		authentication, d := expandAuthentication(ctx, webhook.Authentication)
		diags.Append(d...)
		result["authentication"] = authentication
	}

	if isSet(webhook.EventNameMapping) {
		mapping, d := typedBlock(ctx, webhook.EventNameMapping, resource_notification_integration.NewEventNameMappingValue)
		diags.Append(d...)
		names := map[string]any{}
		putString(names, "alert", mapping.Alert)
		putString(names, "ok", mapping.Ok)
		result["eventNameMapping"] = names
	}

	if isSet(webhook.PayloadTemplate) {
		template, d := typedBlock(ctx, webhook.PayloadTemplate, resource_notification_integration.NewPayloadTemplateValue)
		diags.Append(d...)
		payload, d := expandPayloadTemplate(template)
		diags.Append(d...)
		result["payloadTemplate"] = payload
	}

	return diags
}

func expandAuthentication(ctx context.Context, object basetypes.ObjectValue) (map[string]any, diag.Diagnostics) {
	authentication, diags := typedBlock(ctx, object, resource_notification_integration.NewAuthenticationValue)
	result := map[string]any{}

	switch {
	case isSet(authentication.Basic):
		block, d := typedBlock(ctx, authentication.Basic, resource_notification_integration.NewBasicValue)
		diags.Append(d...)
		result["type"] = "basic"
		putString(result, "username", block.Username)
		putString(result, "password", block.Password)
	case isSet(authentication.Bearer):
		block, d := typedBlock(ctx, authentication.Bearer, resource_notification_integration.NewBearerValue)
		diags.Append(d...)
		result["type"] = "bearer"
		putString(result, "token", block.Token)
	case isSet(authentication.CustomHeader):
		block, d := typedBlock(ctx, authentication.CustomHeader, resource_notification_integration.NewCustomHeaderValue)
		diags.Append(d...)
		result["type"] = "custom-header"
		putString(result, "key", block.Key)
		putString(result, "value", block.Value)
	case isSet(authentication.None):
		result["type"] = "none"
	default:
		diags.AddError("Invalid authentication configuration", "Exactly one variant must be set.")
	}
	return result, diags
}

// expandPayloadTemplate sends the template value as JSON, not as the string it is in Terraform.
func expandPayloadTemplate(template resource_notification_integration.PayloadTemplateValue) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	var value any
	if err := json.Unmarshal([]byte(template.Value.ValueString()), &value); err != nil {
		diags.AddAttributeError(
			path.Root("setting").AtName("webhook").AtName("payload_template").AtName("value"),
			"Invalid payload template",
			fmt.Sprintf("value must be valid JSON: %s", err),
		)
		return nil, diags
	}
	return map[string]any{"type": template.PayloadTemplateType.ValueString(), "value": value}, diags
}

// flattenNotificationIntegration builds the resource state from an API response. The API never
// returns secret fields (tokens, passwords, webhook URLs) in full, only redacted "last chars"
// forms, so secrets are set to the write-only placeholder rather than read from the response.
// prior is the plan on create/update and the previous state on read; it supplies the values the
// API does not return as written, such as the configured payload template text.
func flattenNotificationIntegration(ctx context.Context, data notificationIntegrationAPIData, prior resource_notification_integration.NotificationIntegrationModel) (resource_notification_integration.NotificationIntegrationModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tags, tagDiags := flattenTags(ctx, data.Tags)
	diags.Append(tagDiags...)

	setting, settingDiags := flattenIntegrationSetting(ctx, data.Setting, prior.Setting)
	diags.Append(settingDiags...)

	return resource_notification_integration.NotificationIntegrationModel{
		Id:             types.StringValue(data.ID),
		Name:           types.StringValue(data.Name),
		Owner:          types.StringValue(data.Owner),
		Tags:           tags,
		SecretsVersion: prior.SecretsVersion,
		Setting:        setting,
	}, diags
}

func flattenIntegrationSetting(ctx context.Context, api map[string]interface{}, prior resource_notification_integration.SettingValue) (resource_notification_integration.SettingValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	values := map[string]attr.Value{
		"google_chat":     nullBlock[resource_notification_integration.GoogleChatValue](ctx),
		"grafana_irm":     nullBlock[resource_notification_integration.GrafanaIrmValue](ctx),
		"incident_io":     nullBlock[resource_notification_integration.IncidentIoValue](ctx),
		"jira":            nullBlock[resource_notification_integration.JiraValue](ctx),
		"microsoft_teams": nullBlock[resource_notification_integration.MicrosoftTeamsValue](ctx),
		"pagerduty":       nullBlock[resource_notification_integration.PagerdutyValue](ctx),
		"servicenow":      nullBlock[resource_notification_integration.ServicenowValue](ctx),
		"squadcast":       nullBlock[resource_notification_integration.SquadcastValue](ctx),
		"webhook":         nullBlock[resource_notification_integration.WebhookValue](ctx),
	}

	var name string
	var block basetypes.ObjectValue
	var blockDiags diag.Diagnostics

	apiType, _ := api["type"].(string)
	switch apiType {
	case "google-chat":
		name = "google_chat"
		block, blockDiags = newBlock(ctx, resource_notification_integration.NewGoogleChatValue, map[string]attr.Value{
			"webhook_url":            writeOnlySecret,
			"webhook_url_last_chars": apiString(api, "webhookUrlLastChars"),
		})
	case "grafana-irm":
		name = "grafana_irm"
		block, blockDiags = newBlock(ctx, resource_notification_integration.NewGrafanaIrmValue, map[string]attr.Value{
			"url":            writeOnlySecret,
			"url_last_chars": apiString(api, "urlLastChars"),
		})
	case "incident-io":
		name = "incident_io"
		block, blockDiags = newBlock(ctx, resource_notification_integration.NewIncidentIoValue, map[string]attr.Value{
			"alert_source_config_id": apiString(api, "alertSourceConfigId"),
			"token":                  writeOnlySecret,
			"token_last_chars":       apiString(api, "tokenLastChars"),
		})
	case "jira":
		name = "jira"
		block, blockDiags = newBlock(ctx, resource_notification_integration.NewJiraValue, map[string]attr.Value{
			"api_token":            writeOnlySecret,
			"api_token_last_chars": apiString(api, "apiTokenLastChars"),
			"email":                apiString(api, "email"),
			"site_url":             apiString(api, "siteUrl"),
		})
	case "microsoft-teams":
		name = "microsoft_teams"
		block, blockDiags = newBlock(ctx, resource_notification_integration.NewMicrosoftTeamsValue, map[string]attr.Value{
			"webhook_url":            writeOnlySecret,
			"webhook_url_last_chars": apiString(api, "webhookUrlLastChars"),
		})
	case "pagerduty":
		name = "pagerduty"
		block, blockDiags = newBlock(ctx, resource_notification_integration.NewPagerdutyValue, map[string]attr.Value{
			"integration_key":            writeOnlySecret,
			"integration_key_last_chars": apiString(api, "integrationKeyLastChars"),
		})
	case "servicenow":
		name = "servicenow"
		block, blockDiags = newBlock(ctx, resource_notification_integration.NewServicenowValue, map[string]attr.Value{
			"instance_name":       apiString(api, "instanceName"),
			"password":            writeOnlySecret,
			"password_last_chars": apiString(api, "passwordLastChars"),
			"username":            apiString(api, "username"),
		})
	case "squadcast":
		name = "squadcast"
		block, blockDiags = newBlock(ctx, resource_notification_integration.NewSquadcastValue, map[string]attr.Value{
			"webhook_url":            writeOnlySecret,
			"webhook_url_last_chars": apiString(api, "webhookUrlLastChars"),
		})
	case "webhook":
		name = "webhook"
		block, blockDiags = flattenWebhookSetting(ctx, api, prior.Webhook)
	default:
		diags.AddError("API Error", fmt.Sprintf("Unknown notification integration type %q returned by the API.", apiType))
	}
	diags.Append(blockDiags...)
	if diags.HasError() {
		return resource_notification_integration.NewSettingValueNull(), diags
	}
	values[name] = block

	setting, settingDiags := resource_notification_integration.NewSettingValue(resource_notification_integration.SettingValue{}.AttributeTypes(ctx), values)
	diags.Append(settingDiags...)
	return setting, diags
}

func flattenWebhookSetting(ctx context.Context, api map[string]interface{}, priorObject basetypes.ObjectValue) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	var prior resource_notification_integration.WebhookValue
	if isSet(priorObject) {
		var d diag.Diagnostics
		prior, d = typedBlock(ctx, priorObject, resource_notification_integration.NewWebhookValue)
		diags.Append(d...)
	}

	authentication := nullBlock[resource_notification_integration.AuthenticationValue](ctx)
	if raw, ok := api["authentication"].(map[string]interface{}); ok {
		var d diag.Diagnostics
		authentication, d = flattenAuthentication(ctx, raw)
		diags.Append(d...)
	}

	eventNameMapping := nullBlock[resource_notification_integration.EventNameMappingValue](ctx)
	if raw, ok := api["eventNameMapping"].(map[string]interface{}); ok {
		var d diag.Diagnostics
		eventNameMapping, d = newBlock(ctx, resource_notification_integration.NewEventNameMappingValue, map[string]attr.Value{
			"alert": apiString(raw, "alert"),
			"ok":    apiString(raw, "ok"),
		})
		diags.Append(d...)
	}

	payloadTemplate := nullBlock[resource_notification_integration.PayloadTemplateValue](ctx)
	if raw, ok := api["payloadTemplate"].(map[string]interface{}); ok {
		var d diag.Diagnostics
		payloadTemplate, d = flattenPayloadTemplate(ctx, raw, prior.PayloadTemplate)
		diags.Append(d...)
	}

	if diags.HasError() {
		return basetypes.ObjectValue{}, diags
	}

	webhook, d := newBlock(ctx, resource_notification_integration.NewWebhookValue, map[string]attr.Value{
		"authentication":     authentication,
		"custom_headers":     flattenCustomHeaders(api["customHeaders"], prior.CustomHeaders),
		"event_name_mapping": eventNameMapping,
		"method":             apiString(api, "method"),
		"payload_template":   payloadTemplate,
		"url":                writeOnlySecret,
		"url_last_chars":     apiString(api, "urlLastChars"),
	})
	diags.Append(d...)
	return webhook, diags
}

func flattenAuthentication(ctx context.Context, api map[string]interface{}) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	values := map[string]attr.Value{
		"basic":         nullBlock[resource_notification_integration.BasicValue](ctx),
		"bearer":        nullBlock[resource_notification_integration.BearerValue](ctx),
		"custom_header": nullBlock[resource_notification_integration.CustomHeaderValue](ctx),
		"none":          nullBlock[resource_notification_integration.NoneValue](ctx),
	}

	var name string
	var block basetypes.ObjectValue
	apiType, _ := api["type"].(string)
	switch apiType {
	case "basic":
		name = "basic"
		block, diags = newBlock(ctx, resource_notification_integration.NewBasicValue, map[string]attr.Value{
			"password":            writeOnlySecret,
			"password_last_chars": apiString(api, "passwordLastChars"),
			"username":            apiString(api, "username"),
		})
	case "bearer":
		name = "bearer"
		block, diags = newBlock(ctx, resource_notification_integration.NewBearerValue, map[string]attr.Value{
			"token":            writeOnlySecret,
			"token_last_chars": apiString(api, "tokenLastChars"),
		})
	case "custom-header":
		name = "custom_header"
		block, diags = newBlock(ctx, resource_notification_integration.NewCustomHeaderValue, map[string]attr.Value{
			"key":              apiString(api, "key"),
			"value":            writeOnlySecret,
			"value_last_chars": apiString(api, "valueLastChars"),
		})
	case "none":
		name = "none"
		block, diags = newBlock(ctx, resource_notification_integration.NewNoneValue, map[string]attr.Value{})
	default:
		diags.AddError("API Error", fmt.Sprintf("Unknown webhook authentication type %q returned by the API.", apiType))
	}
	if diags.HasError() {
		return basetypes.ObjectValue{}, diags
	}
	values[name] = block

	authentication, d := newBlock(ctx, resource_notification_integration.NewAuthenticationValue, values)
	return authentication, append(diags, d...)
}

// flattenCustomHeaders reads the headers back in full, so an edit made in the app shows up as
// drift. An explicitly configured empty map is not the same as an absent one.
func flattenCustomHeaders(raw interface{}, prior basetypes.MapValue) basetypes.MapValue {
	if headersRaw, ok := raw.(map[string]interface{}); ok && len(headersRaw) > 0 {
		headers := make(map[string]attr.Value, len(headersRaw))
		for k, v := range headersRaw {
			s, _ := v.(string)
			headers[k] = types.StringValue(s)
		}
		return types.MapValueMust(types.StringType, headers)
	}
	if !prior.IsNull() && !prior.IsUnknown() && len(prior.Elements()) == 0 {
		return prior
	}
	return types.MapNull(types.StringType)
}

// flattenPayloadTemplate encodes the template value as a JSON string. The API stores the template
// decoded, so the configured text is kept when it holds the same value.
func flattenPayloadTemplate(ctx context.Context, api map[string]interface{}, priorObject basetypes.ObjectValue) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	valueBytes, err := json.Marshal(api["value"])
	if err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to encode payload template value: %s", err))
		return basetypes.ObjectValue{}, diags
	}

	value := string(valueBytes)
	if isSet(priorObject) {
		prior, d := typedBlock(ctx, priorObject, resource_notification_integration.NewPayloadTemplateValue)
		diags.Append(d...)
		var priorDecoded any
		if json.Unmarshal([]byte(prior.Value.ValueString()), &priorDecoded) == nil && reflect.DeepEqual(priorDecoded, api["value"]) {
			value = prior.Value.ValueString()
		}
	}

	template, d := newBlock(ctx, resource_notification_integration.NewPayloadTemplateValue, map[string]attr.Value{
		"type":  apiString(api, "type"),
		"value": types.StringValue(value),
	})
	return template, append(diags, d...)
}
