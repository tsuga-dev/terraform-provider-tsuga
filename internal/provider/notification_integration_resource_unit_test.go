package provider

import (
	"context"
	"encoding/json"
	"testing"

	"terraform-provider-tsuga/internal/resource_notification_integration"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandIntegrationSetting_Pagerduty(t *testing.T) {
	setting := &resource_notification_integration.IntegrationSettingModel{
		Pagerduty: &resource_notification_integration.PagerdutySettingModel{
			IntegrationKey: types.StringValue("abc123"),
		},
	}

	got, diags := expandIntegrationSetting(context.Background(), setting)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	want := map[string]any{"type": "pagerduty", "integrationKey": "abc123"}
	if !mapsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExpandIntegrationSetting_NoneSet(t *testing.T) {
	_, diags := expandIntegrationSetting(context.Background(), &resource_notification_integration.IntegrationSettingModel{})
	if !diags.HasError() {
		t.Fatal("expected an error when no setting variant is set")
	}
}

func TestFlattenIntegrationSetting_NeverReadsSecretsBack(t *testing.T) {
	prior := &resource_notification_integration.IntegrationSettingModel{
		Pagerduty: &resource_notification_integration.PagerdutySettingModel{IntegrationKey: types.StringValue("configured")},
	}
	got, diags := flattenIntegrationSetting(map[string]interface{}{"type": "pagerduty", "integrationKeyLastChars": "ured"}, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got.Pagerduty == nil || got.Pagerduty.IntegrationKey != writeOnlySecret {
		t.Fatalf("got %+v, want the write-only placeholder rather than the prior secret", got.Pagerduty)
	}
}

func TestWebhookPayloadTemplate_RoundTrips(t *testing.T) {
	webhook := &resource_notification_integration.WebhookSettingModel{
		Url:    types.StringValue("https://example.com/hook"),
		Method: types.StringValue("POST"),
		PayloadTemplate: &resource_notification_integration.WebhookPayloadTemplateModel{
			Type:  types.StringValue("json"),
			Value: types.StringValue(`{"status":"{{alert}}","nested":{"a":1,"b":[true,false,null]}}`),
		},
	}

	expanded, diags := expandWebhookSetting(context.Background(), webhook)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	payloadTemplate, ok := expanded["payloadTemplate"].(map[string]any)
	if !ok {
		t.Fatalf("expected payloadTemplate to be a map, got %T", expanded["payloadTemplate"])
	}
	value, ok := payloadTemplate["value"].(map[string]any)
	if !ok {
		t.Fatalf("expected payloadTemplate.value to decode as a JSON object, got %T", payloadTemplate["value"])
	}
	if value["status"] != "{{alert}}" {
		t.Fatalf("got status %v, want {{alert}}", value["status"])
	}

	// Flatten as if the API echoed the same template back, and confirm the JSON string
	// round-trips (semantically, since map key order is not guaranteed).
	apiSetting := map[string]interface{}{
		"type":         "webhook",
		"urlLastChars": "hook",
		"method":       "POST",
		"payloadTemplate": map[string]interface{}{
			"type":  "json",
			"value": value,
		},
	}
	flattened, diags := flattenWebhookSetting(apiSetting, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var roundTripped map[string]any
	if err := json.Unmarshal([]byte(flattened.PayloadTemplate.Value.ValueString()), &roundTripped); err != nil {
		t.Fatalf("flattened payload template value is not valid JSON: %s", err)
	}
	if roundTripped["status"] != "{{alert}}" {
		t.Fatalf("got status %v after round trip, want {{alert}}", roundTripped["status"])
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

func TestExpandWebhookSetting_AlwaysSendsCustomHeaders(t *testing.T) {
	webhook := &resource_notification_integration.WebhookSettingModel{
		Url:           types.StringValue("https://example.com/hook"),
		Method:        types.StringValue("POST"),
		CustomHeaders: types.MapNull(types.StringType),
		PayloadTemplate: &resource_notification_integration.WebhookPayloadTemplateModel{
			Type:  types.StringValue("json"),
			Value: types.StringValue(`{"a":1}`),
		},
	}

	got, diags := expandWebhookSetting(context.Background(), webhook)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	headers, ok := got["customHeaders"].(map[string]string)
	if !ok {
		t.Fatalf("got customHeaders %#v, want an empty map", got["customHeaders"])
	}
	if len(headers) != 0 {
		t.Fatalf("got %v, want an empty map", headers)
	}
}

func TestExpandWebhookSetting_CustomHeaderAuthUsesApiSpelling(t *testing.T) {
	webhook := &resource_notification_integration.WebhookSettingModel{
		Url:    types.StringValue("https://example.com/hook"),
		Method: types.StringValue("POST"),
		Authentication: &resource_notification_integration.WebhookAuthenticationModel{
			CustomHeader: &resource_notification_integration.WebhookCustomHeaderAuthModel{
				Key:   types.StringValue("X-Api-Key"),
				Value: types.StringValue("secret"),
			},
		},
		CustomHeaders: types.MapNull(types.StringType),
	}

	got, diags := expandWebhookSetting(context.Background(), webhook)
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

func TestExpandWebhookSetting_NoneAuthentication(t *testing.T) {
	webhook := &resource_notification_integration.WebhookSettingModel{
		Url:            types.StringValue("https://example.com/hook"),
		Method:         types.StringValue("POST"),
		Authentication: &resource_notification_integration.WebhookAuthenticationModel{None: &resource_notification_integration.WebhookNoneAuthModel{}},
		CustomHeaders:  types.MapNull(types.StringType),
	}

	got, diags := expandWebhookSetting(context.Background(), webhook)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	auth, ok := got["authentication"].(map[string]any)
	if !ok || auth["type"] != "none" {
		t.Fatalf("got authentication %#v, want the none variant", got["authentication"])
	}
}

func TestFlattenWebhookSetting_MapsAuthenticationVariants(t *testing.T) {
	flatten := func(auth map[string]interface{}) *resource_notification_integration.WebhookAuthenticationModel {
		got, diags := flattenWebhookSetting(map[string]interface{}{
			"type":           "webhook",
			"urlLastChars":   "hook",
			"method":         "POST",
			"authentication": auth,
		}, nil)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		return got.Authentication
	}

	if got := flatten(map[string]interface{}{"type": "none"}); got == nil || got.None == nil || got.Bearer != nil {
		t.Fatalf("got %+v, want only the none block", got)
	}

	bearer := flatten(map[string]interface{}{"type": "bearer", "tokenLastChars": "cret"})
	// Write-only: the secret is never read back, but the export path needs a non-null value.
	if bearer == nil || bearer.Bearer == nil || bearer.Bearer.Token != writeOnlySecret || bearer.Basic != nil || bearer.CustomHeader != nil {
		t.Fatalf("got %+v, want only the bearer block with the write-only placeholder", bearer)
	}

	basic := flatten(map[string]interface{}{"type": "basic", "username": "alice", "passwordLastChars": "cret"})
	if basic == nil || basic.Basic == nil || basic.Basic.Username.ValueString() != "alice" || basic.Basic.Password != writeOnlySecret {
		t.Fatalf("got %+v, want the basic block with the username read back", basic)
	}
}

func TestFlattenWebhookSetting_ReadsCustomHeadersFromApi(t *testing.T) {
	priorHeaders, diags := types.MapValue(types.StringType, map[string]attr.Value{
		"X-Api-Key": types.StringValue("configured"),
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	apiSetting := map[string]interface{}{
		"type":          "webhook",
		"urlLastChars":  "hook",
		"method":        "POST",
		"customHeaders": map[string]interface{}{"X-Api-Key": "rotated-in-app", "X-Added-In-App": "plain"},
	}

	got, flattenDiags := flattenWebhookSetting(apiSetting, &resource_notification_integration.WebhookSettingModel{
		Url:           types.StringValue("https://example.com/hook"),
		CustomHeaders: priorHeaders,
	})
	if flattenDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", flattenDiags)
	}
	// Header values come back in full, so an edit made in the app must show up as drift.
	headers := got.CustomHeaders.Elements()
	if headers["X-Api-Key"] != types.StringValue("rotated-in-app") || headers["X-Added-In-App"] != types.StringValue("plain") {
		t.Fatalf("got %v, want the API values", headers)
	}
}

func TestFlattenWebhookSetting_PreservesConfiguredPayloadTemplateFormatting(t *testing.T) {
	configured := "{\n  \"status\": \"{{alert}}\"\n}"
	apiSetting := map[string]interface{}{
		"type":         "webhook",
		"urlLastChars": "hook",
		"method":       "POST",
		"payloadTemplate": map[string]interface{}{
			"type":  "json",
			"value": map[string]interface{}{"status": "{{alert}}"},
		},
	}
	prior := &resource_notification_integration.WebhookSettingModel{
		Url: types.StringValue("https://example.com/hook"),
		PayloadTemplate: &resource_notification_integration.WebhookPayloadTemplateModel{
			Type:  types.StringValue("json"),
			Value: types.StringValue(configured),
		},
	}

	got, diags := flattenWebhookSetting(apiSetting, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got.PayloadTemplate.Value.ValueString() != configured {
		t.Fatalf("got %q, want the configured string preserved", got.PayloadTemplate.Value.ValueString())
	}
}

func TestFlattenWebhookSetting_RewritesPayloadTemplateWhenItActuallyChanged(t *testing.T) {
	apiSetting := map[string]interface{}{
		"type":         "webhook",
		"urlLastChars": "hook",
		"method":       "POST",
		"payloadTemplate": map[string]interface{}{
			"type":  "json",
			"value": map[string]interface{}{"status": "changed-in-app"},
		},
	}
	prior := &resource_notification_integration.WebhookSettingModel{
		Url: types.StringValue("https://example.com/hook"),
		PayloadTemplate: &resource_notification_integration.WebhookPayloadTemplateModel{
			Type:  types.StringValue("json"),
			Value: types.StringValue(`{"status":"{{alert}}"}`),
		},
	}

	got, diags := flattenWebhookSetting(apiSetting, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got.PayloadTemplate.Value.ValueString() != `{"status":"changed-in-app"}` {
		t.Fatalf("got %q, want the API value so the drift is reported", got.PayloadTemplate.Value.ValueString())
	}
}

func TestFlattenWebhookSetting_KeepsAnExplicitlyEmptyCustomHeaderMap(t *testing.T) {
	empty, diags := types.MapValue(types.StringType, map[string]attr.Value{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	apiSetting := map[string]interface{}{
		"type":          "webhook",
		"urlLastChars":  "hook",
		"method":        "POST",
		"customHeaders": map[string]interface{}{},
	}

	got, flattenDiags := flattenWebhookSetting(apiSetting, &resource_notification_integration.WebhookSettingModel{
		Url:           types.StringValue("https://example.com/hook"),
		CustomHeaders: empty,
	})
	if flattenDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", flattenDiags)
	}
	if got.CustomHeaders.IsNull() {
		t.Fatal("got null, want the configured empty map so the state matches the plan")
	}
	if len(got.CustomHeaders.Elements()) != 0 {
		t.Fatalf("got %v, want an empty map", got.CustomHeaders)
	}
}

func TestFlattenWebhookSetting_ReportsCustomHeadersEmptiedOutOfBand(t *testing.T) {
	priorHeaders, diags := types.MapValue(types.StringType, map[string]attr.Value{
		"X-Api-Key": types.StringValue("secret"),
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	apiSetting := map[string]interface{}{
		"type":          "webhook",
		"urlLastChars":  "hook",
		"method":        "POST",
		"customHeaders": map[string]interface{}{},
	}

	got, flattenDiags := flattenWebhookSetting(apiSetting, &resource_notification_integration.WebhookSettingModel{
		Url:           types.StringValue("https://example.com/hook"),
		CustomHeaders: priorHeaders,
	})
	if flattenDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", flattenDiags)
	}
	if !got.CustomHeaders.IsNull() {
		t.Fatalf("got %v, want null so the removal is reported as drift", got.CustomHeaders)
	}
}

func TestFlattenWebhookSetting_PreservesTemplateAcrossEquivalentNumberSpellings(t *testing.T) {
	configured := `{"threshold":1e3,"ratio":1.0}`
	apiSetting := map[string]interface{}{
		"type":         "webhook",
		"urlLastChars": "hook",
		"method":       "POST",
		"payloadTemplate": map[string]interface{}{
			"type": "json",
			// The API decodes and re-serialises, so it hands back the canonical spelling.
			"value": map[string]interface{}{"threshold": float64(1000), "ratio": float64(1)},
		},
	}

	got, diags := flattenWebhookSetting(apiSetting, &resource_notification_integration.WebhookSettingModel{
		Url: types.StringValue("https://example.com/hook"),
		PayloadTemplate: &resource_notification_integration.WebhookPayloadTemplateModel{
			Type:  types.StringValue("json"),
			Value: types.StringValue(configured),
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got.PayloadTemplate.Value.ValueString() != configured {
		t.Fatalf("got %q, want the configured spelling preserved", got.PayloadTemplate.Value.ValueString())
	}
}
