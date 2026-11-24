package provider

import (
	"context"
	"encoding/json"
	"testing"

	"terraform-provider-tsuga/internal/apiclient"
	"terraform-provider-tsuga/internal/resource_adaptive_notification_rule"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidateAdaptiveNotificationRuleTargets(t *testing.T) {
	ids := func(values ...string) types.List {
		elements := make([]attr.Value, 0, len(values))
		for _, v := range values {
			elements = append(elements, types.StringValue(v))
		}
		return types.ListValueMust(types.StringType, elements)
	}
	null := types.ListNull(types.StringType)
	unknown := types.ListUnknown(types.StringType)

	cases := []struct {
		name               string
		monitorIDs, sloIDs types.List
		wantError          bool
	}{
		{"neither list set", null, null, true},
		{"both lists empty", ids(), ids(), true},
		{"empty monitors, omitted SLOs", ids(), null, true},
		{"monitors only", ids("monitor-1"), null, false},
		{"SLOs only", null, ids("slo-1"), false},
		{"empty monitors with SLOs", ids(), ids("slo-1"), false},
		{"both lists set", ids("monitor-1"), ids("slo-1"), false},
		// A list set from a variable is unknown during terraform validate, and one built from
		// values computed at apply can still be unknown during plan.
		{"unknown monitors", unknown, null, false},
		{"unknown SLOs with empty monitors", ids(), unknown, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := validateAdaptiveNotificationRuleTargets(c.monitorIDs, c.sloIDs)
			if diags.HasError() != c.wantError {
				t.Fatalf("wantError=%v, got diagnostics: %v", c.wantError, diags)
			}
		})
	}
}

// emailTarget round-trips through the schema type, so the value has the types Terraform passes in.
func emailTarget(t *testing.T, id string) resource_adaptive_notification_rule.FallbackTargetValue {
	ctx := context.Background()
	fallback := emailAPITarget(t, id).FallbackTarget
	plain, diags := flattenNotificationRuleTarget(ctx, fallback.Id, fallback.Config, fallback.RateLimit)
	if diags.HasError() {
		t.Fatalf("failed to flatten fallback target: %v", diags)
	}
	raw, err := plain.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("failed to convert fallback target: %v", err)
	}
	typ := resource_adaptive_notification_rule.AdaptiveNotificationRuleResourceSchema(ctx).Attributes["fallback_target"].GetType()
	value, err := typ.ValueFromTerraform(ctx, raw)
	if err != nil {
		t.Fatalf("failed to build fallback target: %v", err)
	}
	return value.(resource_adaptive_notification_rule.FallbackTargetValue)
}

func emailAPITarget(t *testing.T, id string) apiclient.AdaptiveRule {
	t.Helper()
	var rule apiclient.AdaptiveRule
	raw := `{"fallbackTarget": {"id": "` + id + `", "config": {"type": "email", "addresses": ["oncall@example.com"]}}}`
	if err := json.Unmarshal([]byte(raw), &rule); err != nil {
		t.Fatal(err)
	}
	return rule
}

func TestBuildAdaptiveNotificationRuleRequestBody_SendsEmptyListsForOmittedIDs(t *testing.T) {
	ctx := context.Background()
	plan := resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{
		Name:           types.StringValue("rule"),
		Owner:          types.StringValue("team-1"),
		IsActive:       types.BoolValue(true),
		Tags:           types.ListNull(types.ObjectType{}),
		MonitorIds:     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("monitor-1")}),
		SloIds:         types.ListNull(types.StringType),
		FallbackTarget: emailTarget(t, "fallback"),
	}

	body, diags := buildAdaptiveNotificationRuleRequestBody(ctx, plan)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	// Dropping slo_ids must clear the list on update, which the API only does for an empty array.
	if sloIDs, ok := decoded["sloIds"].([]any); !ok || len(sloIDs) != 0 {
		t.Fatalf("expected sloIds to be an empty array, got %#v", decoded["sloIds"])
	}
	if monitorIDs, ok := decoded["monitorIds"].([]any); !ok || len(monitorIDs) != 1 || monitorIDs[0] != "monitor-1" {
		t.Fatalf("expected monitorIds [monitor-1], got %#v", decoded["monitorIds"])
	}
	fallback := decoded["fallbackTarget"].(map[string]any)
	if fallback["id"] != "fallback" || fallback["config"].(map[string]any)["type"] != "email" {
		t.Fatalf("unexpected fallbackTarget: %#v", fallback)
	}
	if _, ok := decoded["tags"]; ok {
		t.Fatalf("expected tags to be omitted when unset, got %#v", decoded["tags"])
	}
}

func TestFlattenAdaptiveNotificationRule_EmptyListKeepsConfiguredForm(t *testing.T) {
	ctx := context.Background()
	data := emailAPITarget(t, "fallback")
	data.Id = "rule-1"
	data.Name = "rule"
	data.Owner = "team-1"
	data.IsActive = true
	data.MonitorIds = []string{"monitor-1"}
	data.SloIds = []string{}

	omitted := resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{SloIds: types.ListNull(types.StringType)}
	state, diags := flattenAdaptiveNotificationRule(ctx, data, omitted)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !state.SloIds.IsNull() {
		t.Fatalf("expected slo_ids to stay null when omitted, got %s", state.SloIds)
	}

	setEmpty := resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{SloIds: types.ListValueMust(types.StringType, nil)}
	state, diags = flattenAdaptiveNotificationRule(ctx, data, setEmpty)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if state.SloIds.IsNull() || len(state.SloIds.Elements()) != 0 {
		t.Fatalf("expected slo_ids to stay an empty list when set to [], got %s", state.SloIds)
	}

	if len(state.MonitorIds.Elements()) != 1 {
		t.Fatalf("expected one monitor id, got %s", state.MonitorIds)
	}
	if state.FallbackTarget.Id.ValueString() != "fallback" || objectAttribute(state.FallbackTarget.Config, "email").IsNull() {
		t.Fatalf("unexpected fallback target: %#v", state.FallbackTarget)
	}
}

func TestFlattenAdaptiveNotificationRule_FitsTheGeneratedSchema(t *testing.T) {
	ctx := context.Background()

	var data apiclient.AdaptiveRule
	raw := `{"id": "rule-1", "name": "rule", "owner": "team-1", "monitorIds": ["monitor-1"], "tags": [{"key": "k", "value": "v"}],
		"fallbackTarget": {"id": "fallback", "config": {"type": "email", "addresses": ["oncall@example.com"]}}}`
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}
	state, diags := flattenAdaptiveNotificationRule(ctx, data, resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	got := tfsdk.State{Schema: resource_adaptive_notification_rule.AdaptiveNotificationRuleResourceSchema(ctx)}
	if d := got.Set(ctx, &state); d.HasError() {
		t.Fatalf("flattened rule does not fit the schema: %v", d)
	}
}
