package provider

import (
	"context"
	"encoding/json"
	"testing"

	"terraform-provider-tsuga/internal/resource_adaptive_notification_rule"
	"terraform-provider-tsuga/internal/resource_notification_rule"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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

func emailTarget(id string) *resource_notification_rule.TargetModel {
	return &resource_notification_rule.TargetModel{
		Id: types.StringValue(id),
		Config: resource_notification_rule.TargetConfigModel{
			Email: &resource_notification_rule.EmailConfigModel{
				Addresses: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("oncall@example.com")}),
			},
		},
	}
}

func TestBuildAdaptiveNotificationRuleRequestBody_SendsEmptyListsForOmittedIDs(t *testing.T) {
	ctx := context.Background()
	plan := resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{
		Name:                      types.StringValue("rule"),
		Owner:                     types.StringValue("team-1"),
		IsActive:                  types.BoolValue(true),
		ShouldCreateInvestigation: types.BoolValue(false),
		Tags:                      types.ListNull(types.ObjectType{}),
		MonitorIds:                types.ListValueMust(types.StringType, []attr.Value{types.StringValue("monitor-1")}),
		SloIds:                    types.ListNull(types.StringType),
		FallbackTarget:            emailTarget("fallback"),
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
	data := adaptiveNotificationRuleAPIData{
		ID:         "rule-1",
		Name:       "rule",
		Owner:      "team-1",
		IsActive:   true,
		MonitorIDs: []string{"monitor-1"},
		SloIDs:     []string{},
		FallbackTarget: notificationRuleAPITarget{
			ID:     "fallback",
			Config: notificationRuleAPITargetConfig{Type: "email", Addresses: []string{"oncall@example.com"}},
		},
	}

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
	if state.FallbackTarget.Id.ValueString() != "fallback" || state.FallbackTarget.Config.Email == nil {
		t.Fatalf("unexpected fallback target: %#v", state.FallbackTarget)
	}
}
