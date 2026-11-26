package provider

import (
	"context"
	"encoding/json"
	"testing"

	"terraform-provider-tsuga/internal/resource_notification_integration"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var testCtx = context.Background()

// object builds an object of the given type from the attributes provided, the others being null.
func object(t *testing.T, objectType attr.Type, provided map[string]attr.Value) basetypes.ObjectValue {
	t.Helper()

	attrTypes := objectType.(basetypes.ObjectType).AttrTypes
	values := make(map[string]attr.Value, len(attrTypes))
	for name, attrType := range attrTypes {
		if value, ok := provided[name]; ok {
			values[name] = value
			continue
		}
		null, err := attrType.ValueFromTerraform(testCtx, tftypes.NewValue(attrType.TerraformType(testCtx), nil))
		if err != nil {
			t.Fatalf("failed to build a null %s: %v", name, err)
		}
		values[name] = null
	}
	return types.ObjectValueMust(attrTypes, values)
}

func str(value string) attr.Value { return types.StringValue(value) }

// settingWith builds a setting with the one block set.
func settingWith(t *testing.T, block string, fields map[string]attr.Value) resource_notification_integration.SettingValue {
	t.Helper()

	attrTypes := resource_notification_integration.SettingValue{}.AttributeTypes(testCtx)
	return resource_notification_integration.NewSettingValueMust(attrTypes, object(t, types.ObjectType{AttrTypes: attrTypes}, map[string]attr.Value{
		block: object(t, attrTypes[block], fields),
	}).Attributes())
}

func webhookSetting(t *testing.T, fields map[string]attr.Value) resource_notification_integration.SettingValue {
	t.Helper()

	webhookType := resource_notification_integration.SettingValue{}.AttributeTypes(testCtx)["webhook"].(basetypes.ObjectType)
	return settingWith(t, "webhook", withDefaults(fields, map[string]attr.Value{
		"url":    str("https://example.com/hook"),
		"method": str("POST"),
		"authentication": object(t, webhookType.AttrTypes["authentication"], map[string]attr.Value{
			"none": object(t, webhookType.AttrTypes["authentication"].(basetypes.ObjectType).AttrTypes["none"], nil),
		}),
	}))
}

func withDefaults(fields, defaults map[string]attr.Value) map[string]attr.Value {
	merged := map[string]attr.Value{}
	for k, v := range defaults {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return merged
}

func payloadTemplate(t *testing.T, templateType, value string) attr.Value {
	t.Helper()

	webhookType := resource_notification_integration.SettingValue{}.AttributeTypes(testCtx)["webhook"].(basetypes.ObjectType)
	return object(t, webhookType.AttrTypes["payload_template"], map[string]attr.Value{"type": str(templateType), "value": str(value)})
}

func TestExpandIntegrationSetting_Pagerduty(t *testing.T) {
	got, diags := expandIntegrationSetting(testCtx, settingWith(t, "pagerduty", map[string]attr.Value{"integration_key": str("abc123")}))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	want := map[string]any{"type": "pagerduty", "integrationKey": "abc123"}
	if !mapsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExpandIntegrationSetting_GrafanaIrmSendsUrl(t *testing.T) {
	got, diags := expandIntegrationSetting(testCtx, settingWith(t, "grafana_irm", map[string]attr.Value{"url": str("https://grafana.example.com/hook")}))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	want := map[string]any{"type": "grafana-irm", "url": "https://grafana.example.com/hook"}
	if !mapsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	back, diags := flattenIntegrationSetting(testCtx, map[string]interface{}{"type": "grafana-irm", "urlLastChars": "hook"}, resource_notification_integration.SettingValue{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	grafana := back.GrafanaIrm.Attributes()
	if !grafana["url_last_chars"].Equal(str("hook")) {
		t.Fatalf("got %v, want url_last_chars read from urlLastChars", grafana)
	}
}

func TestNotificationIntegrationOwner_RoundTrips(t *testing.T) {
	config := resource_notification_integration.NotificationIntegrationModel{
		Name:    types.StringValue("Example"),
		Owner:   types.StringValue("team-123"),
		Setting: settingWith(t, "pagerduty", map[string]attr.Value{"integration_key": str("abc123")}),
		Tags:    types.ListNull(types.ObjectType{}),
	}

	body, diags := (&notificationIntegrationResource{}).buildNotificationIntegrationRequestBody(testCtx, config)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if body["owner"] != "team-123" {
		t.Fatalf("got owner %v in the request body, want team-123", body["owner"])
	}

	prior := config
	prior.Owner = types.StringValue("different-team")

	state, diags := flattenNotificationIntegration(testCtx, notificationIntegrationAPIData{
		ID:      "resource-123",
		Name:    "Example",
		Owner:   "team-123",
		Setting: map[string]interface{}{"type": "pagerduty", "integrationKeyLastChars": "c123"},
	}, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if state.Owner.ValueString() != "team-123" {
		t.Fatalf("got owner %q in state, want team-123", state.Owner.ValueString())
	}
}

func TestExpandIntegrationSetting_NoneSet(t *testing.T) {
	attrTypes := resource_notification_integration.SettingValue{}.AttributeTypes(testCtx)
	empty := resource_notification_integration.NewSettingValueMust(attrTypes, object(t, types.ObjectType{AttrTypes: attrTypes}, nil).Attributes())

	_, diags := expandIntegrationSetting(testCtx, empty)
	if !diags.HasError() {
		t.Fatal("expected an error when no setting variant is set")
	}
}

func TestFlattenIntegrationSetting_NeverReadsSecretsBack(t *testing.T) {
	prior := settingWith(t, "pagerduty", map[string]attr.Value{"integration_key": str("configured")})

	got, diags := flattenIntegrationSetting(testCtx, map[string]interface{}{"type": "pagerduty", "integrationKeyLastChars": "ured"}, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	pagerduty := got.Pagerduty.Attributes()
	if pagerduty["integration_key"] != writeOnlySecret {
		t.Fatalf("got %v, want the write-only placeholder rather than the prior secret", pagerduty["integration_key"])
	}
	if !pagerduty["integration_key_last_chars"].Equal(str("ured")) {
		t.Fatalf("got %v, want the last characters from the API", pagerduty["integration_key_last_chars"])
	}
}

func TestFlattenIntegrationSetting_UnknownType(t *testing.T) {
	_, diags := flattenIntegrationSetting(testCtx, map[string]interface{}{"type": "carrier-pigeon"}, resource_notification_integration.SettingValue{})
	if !diags.HasError() {
		t.Fatal("expected an error for an unknown integration type")
	}
}

func TestWebhookPayloadTemplate_RoundTrips(t *testing.T) {
	setting := webhookSetting(t, map[string]attr.Value{
		"payload_template": payloadTemplate(t, "json", `{"status":"{{alert}}","nested":{"a":1,"b":[true,false,null]}}`),
	})

	expanded, diags := expandIntegrationSetting(testCtx, setting)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	template, ok := expanded["payloadTemplate"].(map[string]any)
	if !ok {
		t.Fatalf("expected payloadTemplate to be a map, got %T", expanded["payloadTemplate"])
	}
	value, ok := template["value"].(map[string]any)
	if !ok {
		t.Fatalf("expected payloadTemplate.value to decode as a JSON object, got %T", template["value"])
	}
	if value["status"] != "{{alert}}" {
		t.Fatalf("got status %v, want {{alert}}", value["status"])
	}

	// Flatten as if the API echoed the same template back, and confirm the JSON string
	// round-trips (semantically, since map key order is not guaranteed).
	flattened, diags := flattenIntegrationSetting(testCtx, map[string]interface{}{
		"type":            "webhook",
		"urlLastChars":    "hook",
		"method":          "POST",
		"payloadTemplate": map[string]interface{}{"type": "json", "value": value},
	}, resource_notification_integration.SettingValue{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var roundTripped map[string]any
	text := flattened.Webhook.Attributes()["payload_template"].(basetypes.ObjectValue).Attributes()["value"].(basetypes.StringValue).ValueString()
	if err := json.Unmarshal([]byte(text), &roundTripped); err != nil {
		t.Fatalf("flattened payload template value is not valid JSON: %s", err)
	}
	if roundTripped["status"] != "{{alert}}" {
		t.Fatalf("got status %v after round trip, want {{alert}}", roundTripped["status"])
	}
}

func TestExpandIntegrationSetting_InvalidPayloadTemplate(t *testing.T) {
	_, diags := expandIntegrationSetting(testCtx, webhookSetting(t, map[string]attr.Value{
		"payload_template": payloadTemplate(t, "json", `{not json`),
	}))
	if !diags.HasError() {
		t.Fatal("expected an error for a payload template that is not JSON")
	}
}

func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestExpandIntegrationSetting_WebhookAlwaysSendsCustomHeaders(t *testing.T) {
	got, diags := expandIntegrationSetting(testCtx, webhookSetting(t, map[string]attr.Value{
		"payload_template": payloadTemplate(t, "json", `{"a":1}`),
	}))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	headers, ok := got["customHeaders"].(map[string]string)
	if !ok || len(headers) != 0 {
		t.Fatalf("got customHeaders %#v, want an empty map", got["customHeaders"])
	}
}

func authenticationWith(t *testing.T, variant string, fields map[string]attr.Value) attr.Value {
	t.Helper()

	webhookType := resource_notification_integration.SettingValue{}.AttributeTypes(testCtx)["webhook"].(basetypes.ObjectType)
	authenticationType := webhookType.AttrTypes["authentication"]
	return object(t, authenticationType, map[string]attr.Value{
		variant: object(t, authenticationType.(basetypes.ObjectType).AttrTypes[variant], fields),
	})
}

func TestExpandIntegrationSetting_CustomHeaderAuthUsesApiSpelling(t *testing.T) {
	got, diags := expandIntegrationSetting(testCtx, webhookSetting(t, map[string]attr.Value{
		"authentication": authenticationWith(t, "custom_header", map[string]attr.Value{"key": str("X-Api-Key"), "value": str("secret")}),
	}))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	auth, ok := got["authentication"].(map[string]any)
	if !ok {
		t.Fatalf("got authentication %#v, want a map", got["authentication"])
	}
	if auth["type"] != "custom-header" || auth["key"] != "X-Api-Key" || auth["value"] != "secret" {
		t.Fatalf("got %v, want the custom-header key and value to be sent", auth)
	}
}

func TestExpandIntegrationSetting_NoneAuthentication(t *testing.T) {
	got, diags := expandIntegrationSetting(testCtx, webhookSetting(t, nil))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	auth, ok := got["authentication"].(map[string]any)
	if !ok || !mapsEqual(auth, map[string]any{"type": "none"}) {
		t.Fatalf("got authentication %#v, want the none variant", got["authentication"])
	}
}

func flattenWebhook(t *testing.T, apiSetting map[string]interface{}, prior resource_notification_integration.SettingValue) map[string]attr.Value {
	t.Helper()

	apiSetting["type"] = "webhook"
	if _, ok := apiSetting["urlLastChars"]; !ok {
		apiSetting["urlLastChars"] = "hook"
	}
	if _, ok := apiSetting["method"]; !ok {
		apiSetting["method"] = "POST"
	}
	got, diags := flattenIntegrationSetting(testCtx, apiSetting, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	return got.Webhook.Attributes()
}

func TestFlattenWebhookSetting_MapsAuthenticationVariants(t *testing.T) {
	authentication := func(auth map[string]interface{}) map[string]attr.Value {
		return flattenWebhook(t, map[string]interface{}{"authentication": auth}, resource_notification_integration.SettingValue{})["authentication"].(basetypes.ObjectValue).Attributes()
	}
	set := func(attributes map[string]attr.Value, name string) basetypes.ObjectValue {
		return attributes[name].(basetypes.ObjectValue)
	}

	none := authentication(map[string]interface{}{"type": "none"})
	if set(none, "none").IsNull() || !set(none, "bearer").IsNull() {
		t.Fatalf("got %v, want only the none block", none)
	}

	bearer := authentication(map[string]interface{}{"type": "bearer", "tokenLastChars": "cret"})
	// Write-only: the secret is never read back, but the export path needs a non-null value.
	if set(bearer, "bearer").IsNull() || !set(bearer, "basic").IsNull() || !set(bearer, "custom_header").IsNull() ||
		set(bearer, "bearer").Attributes()["token"] != writeOnlySecret {
		t.Fatalf("got %v, want only the bearer block with the write-only placeholder", bearer)
	}

	basic := set(authentication(map[string]interface{}{"type": "basic", "username": "alice", "passwordLastChars": "cret"}), "basic").Attributes()
	if !basic["username"].Equal(str("alice")) || basic["password"] != writeOnlySecret {
		t.Fatalf("got %v, want the basic block with the username read back", basic)
	}
}

func headersOf(t *testing.T, headers map[string]string) attr.Value {
	t.Helper()

	elements := map[string]attr.Value{}
	for k, v := range headers {
		elements[k] = str(v)
	}
	return types.MapValueMust(types.StringType, elements)
}

func TestFlattenWebhookSetting_ReadsCustomHeadersFromApi(t *testing.T) {
	prior := webhookSetting(t, map[string]attr.Value{"custom_headers": headersOf(t, map[string]string{"X-Api-Key": "configured"})})

	got := flattenWebhook(t, map[string]interface{}{
		"customHeaders": map[string]interface{}{"X-Api-Key": "rotated-in-app", "X-Added-In-App": "plain"},
	}, prior)
	// Header values come back in full, so an edit made in the app must show up as drift.
	headers := got["custom_headers"].(basetypes.MapValue).Elements()
	if !headers["X-Api-Key"].Equal(str("rotated-in-app")) || !headers["X-Added-In-App"].Equal(str("plain")) {
		t.Fatalf("got %v, want the API values", headers)
	}
}

func TestFlattenWebhookSetting_PreservesConfiguredPayloadTemplateFormatting(t *testing.T) {
	configured := "{\n  \"status\": \"{{alert}}\"\n}"
	prior := webhookSetting(t, map[string]attr.Value{"payload_template": payloadTemplate(t, "json", configured)})

	got := flattenWebhook(t, map[string]interface{}{
		"payloadTemplate": map[string]interface{}{"type": "json", "value": map[string]interface{}{"status": "{{alert}}"}},
	}, prior)
	if value := got["payload_template"].(basetypes.ObjectValue).Attributes()["value"]; !value.Equal(str(configured)) {
		t.Fatalf("got %v, want the configured string preserved", value)
	}
}

func TestFlattenWebhookSetting_RewritesPayloadTemplateWhenItActuallyChanged(t *testing.T) {
	prior := webhookSetting(t, map[string]attr.Value{"payload_template": payloadTemplate(t, "json", `{"status":"{{alert}}"}`)})

	got := flattenWebhook(t, map[string]interface{}{
		"payloadTemplate": map[string]interface{}{"type": "json", "value": map[string]interface{}{"status": "changed-in-app"}},
	}, prior)
	if value := got["payload_template"].(basetypes.ObjectValue).Attributes()["value"]; !value.Equal(str(`{"status":"changed-in-app"}`)) {
		t.Fatalf("got %v, want the API value so the drift is reported", value)
	}
}

func TestFlattenWebhookSetting_KeepsAnExplicitlyEmptyCustomHeaderMap(t *testing.T) {
	prior := webhookSetting(t, map[string]attr.Value{"custom_headers": headersOf(t, nil)})

	got := flattenWebhook(t, map[string]interface{}{"customHeaders": map[string]interface{}{}}, prior)
	headers := got["custom_headers"].(basetypes.MapValue)
	if headers.IsNull() {
		t.Fatal("got null, want the configured empty map so the state matches the plan")
	}
	if len(headers.Elements()) != 0 {
		t.Fatalf("got %v, want an empty map", headers)
	}
}

func TestFlattenWebhookSetting_ReportsCustomHeadersEmptiedOutOfBand(t *testing.T) {
	prior := webhookSetting(t, map[string]attr.Value{"custom_headers": headersOf(t, map[string]string{"X-Api-Key": "secret"})})

	got := flattenWebhook(t, map[string]interface{}{"customHeaders": map[string]interface{}{}}, prior)
	if !got["custom_headers"].(basetypes.MapValue).IsNull() {
		t.Fatalf("got %v, want null so the removal is reported as drift", got["custom_headers"])
	}
}

func TestFlattenWebhookSetting_PreservesTemplateAcrossEquivalentNumberSpellings(t *testing.T) {
	configured := `{"threshold":1e3,"ratio":1.0}`
	prior := webhookSetting(t, map[string]attr.Value{"payload_template": payloadTemplate(t, "json", configured)})

	got := flattenWebhook(t, map[string]interface{}{
		// The API decodes and re-serialises, so it hands back the canonical spelling.
		"payloadTemplate": map[string]interface{}{"type": "json", "value": map[string]interface{}{"threshold": float64(1000), "ratio": float64(1)}},
	}, prior)
	if value := got["payload_template"].(basetypes.ObjectValue).Attributes()["value"]; !value.Equal(str(configured)) {
		t.Fatalf("got %v, want the configured spelling preserved", value)
	}
}

func TestFlattenNotificationIntegration_FitsTheGeneratedSchema(t *testing.T) {
	state, diags := flattenNotificationIntegration(testCtx, notificationIntegrationAPIData{
		ID:    "integration-1",
		Name:  "Example",
		Owner: "team-1",
		Setting: map[string]interface{}{
			"type":           "webhook",
			"urlLastChars":   "hook",
			"method":         "POST",
			"customHeaders":  map[string]interface{}{"X-Env": "dev"},
			"authentication": map[string]interface{}{"type": "bearer", "tokenLastChars": "cret"},
			"payloadTemplate": map[string]interface{}{
				"type":  "json",
				"value": map[string]interface{}{"status": "{{alert}}"},
			},
			"eventNameMapping": map[string]interface{}{"ok": "resolved", "alert": "firing"},
		},
		Tags: []apiTag{{Key: "env", Value: "dev"}},
	}, resource_notification_integration.NotificationIntegrationModel{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	got := tfsdk.State{Schema: resource_notification_integration.NotificationIntegrationResourceSchema(testCtx)}
	if d := got.Set(testCtx, &state); d.HasError() {
		t.Fatalf("flattened integration does not fit the schema: %v", d)
	}
}

func TestSchema_MarksSecretsWriteOnly(t *testing.T) {
	var resp resource.SchemaResponse
	(&notificationIntegrationResource{}).Schema(testCtx, resource.SchemaRequest{}, &resp)

	setting := resp.Schema.Attributes["setting"].(schema.SingleNestedAttribute)
	pagerduty := setting.Attributes["pagerduty"].(schema.SingleNestedAttribute).Attributes
	key := pagerduty["integration_key"].(schema.StringAttribute)
	if !key.WriteOnly || !key.Sensitive {
		t.Fatalf("integration_key: writeOnly=%t sensitive=%t, want both", key.WriteOnly, key.Sensitive)
	}
	if pagerduty["integration_key_last_chars"].(schema.StringAttribute).WriteOnly {
		t.Fatal("integration_key_last_chars must not be write-only")
	}
	if setting.Attributes["jira"].(schema.SingleNestedAttribute).Attributes["email"].(schema.StringAttribute).WriteOnly {
		t.Fatal("jira email is not a secret")
	}
}
