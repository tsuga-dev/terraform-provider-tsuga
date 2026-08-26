package provider

import (
	"context"
	"encoding/json"
	"testing"

	"terraform-provider-tsuga/internal/resource_notification_rule"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func targetsList(t *testing.T, ctx context.Context, targets ...resource_notification_rule.TargetModel) types.List {
	t.Helper()

	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: resource_notification_rule.TargetAttrTypes(ctx)}, targets)
	if diags.HasError() {
		t.Fatalf("failed to build targets list: %v", diags)
	}
	return list
}

func renotifyModel(t *testing.T, ctx context.Context, states ...string) *resource_notification_rule.TargetRenotifyModel {
	t.Helper()

	statesList, diags := types.ListValueFrom(ctx, types.StringType, states)
	if diags.HasError() {
		t.Fatalf("failed to build renotification_states list: %v", diags)
	}
	return &resource_notification_rule.TargetRenotifyModel{
		Mode:                    types.StringValue("each"),
		RenotificationStates:    statesList,
		RenotifyIntervalMinutes: types.Int64Value(30),
	}
}

func slackTarget(id string) resource_notification_rule.TargetModel {
	return resource_notification_rule.TargetModel{
		Id: types.StringValue(id),
		Config: resource_notification_rule.TargetConfigModel{
			Slack: &resource_notification_rule.SlackConfigModel{
				Channel:       types.StringValue("C123"),
				IntegrationID: types.StringValue("T123"),
			},
		},
	}
}

func expandOneTarget(t *testing.T, ctx context.Context, target resource_notification_rule.TargetModel) notificationRuleAPITarget {
	t.Helper()

	expanded, diags := expandNotificationRuleTargets(ctx, targetsList(t, ctx, target))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(expanded) != 1 {
		t.Fatalf("expected 1 expanded target, got %d", len(expanded))
	}
	return expanded[0]
}

func flattenOneTarget(t *testing.T, ctx context.Context, target notificationRuleAPITarget) resource_notification_rule.TargetModel {
	t.Helper()

	flattened, diags := flattenNotificationRuleTargets(ctx, []notificationRuleAPITarget{target})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var models []resource_notification_rule.TargetModel
	if d := flattened.ElementsAs(ctx, &models, false); d.HasError() {
		t.Fatalf("failed to decode flattened targets: %v", d)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 flattened target, got %d", len(models))
	}
	return models[0]
}

func TestExpandNotificationRuleTargets_RenotifyIsNestedInConfig(t *testing.T) {
	// The API reads renotifyConfig from targets[].config, not from the target itself.
	ctx := context.Background()

	target := slackTarget("slack")
	target.Config.Renotify = renotifyModel(t, ctx, "alert", "alert_no_data")

	raw, err := json.Marshal(expandOneTarget(t, ctx, target))
	if err != nil {
		t.Fatalf("failed to marshal expanded target: %v", err)
	}

	var wire struct {
		RenotifyConfig json.RawMessage `json:"renotifyConfig"`
		Config         struct {
			RenotifyConfig *notificationRuleAPITargetRenotify `json:"renotifyConfig"`
		} `json:"config"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("failed to unmarshal wire payload: %v", err)
	}

	if wire.RenotifyConfig != nil {
		t.Fatalf("expected no target-level renotifyConfig, got %s", wire.RenotifyConfig)
	}
	if wire.Config.RenotifyConfig == nil {
		t.Fatalf("expected config.renotifyConfig to be set, got payload %s", raw)
	}
	if wire.Config.RenotifyConfig.Mode != "each" {
		t.Fatalf("expected config.renotifyConfig.mode to be each, got %q", wire.Config.RenotifyConfig.Mode)
	}
	if wire.Config.RenotifyConfig.RenotifyIntervalMinutes != 30 {
		t.Fatalf("expected config.renotifyConfig.renotifyIntervalMinutes to be 30, got %d", wire.Config.RenotifyConfig.RenotifyIntervalMinutes)
	}
	if len(wire.Config.RenotifyConfig.RenotificationStates) != 2 {
		t.Fatalf("expected 2 renotificationStates, got %#v", wire.Config.RenotifyConfig.RenotificationStates)
	}
}

func TestExpandFlattenNotificationRuleTargets_RenotifyRoundTrips(t *testing.T) {
	ctx := context.Background()

	target := slackTarget("slack")
	target.Config.Renotify = renotifyModel(t, ctx, "alert")

	back := flattenOneTarget(t, ctx, expandOneTarget(t, ctx, target))
	if back.Config.Renotify == nil {
		t.Fatal("expected renotify to survive the round-trip")
	}
	if back.Config.Renotify.Mode.ValueString() != "each" {
		t.Fatalf("expected mode each, got %v", back.Config.Renotify.Mode)
	}
	if back.Config.Renotify.RenotifyIntervalMinutes.ValueInt64() != 30 {
		t.Fatalf("expected renotify_interval_minutes 30, got %v", back.Config.Renotify.RenotifyIntervalMinutes)
	}

	var states []string
	if d := back.Config.Renotify.RenotificationStates.ElementsAs(ctx, &states, false); d.HasError() {
		t.Fatalf("failed to decode renotification_states: %v", d)
	}
	if len(states) != 1 || states[0] != "alert" {
		t.Fatalf("expected renotification_states [alert], got %#v", states)
	}
}

func TestExpandNotificationRuleTargets_WithoutRenotifyOmitsTheKey(t *testing.T) {
	ctx := context.Background()

	expanded := expandOneTarget(t, ctx, slackTarget("slack"))
	if expanded.Config.RenotifyConfig != nil {
		t.Fatalf("expected no renotify config, got %#v", expanded.Config.RenotifyConfig)
	}

	raw, err := json.Marshal(expanded)
	if err != nil {
		t.Fatalf("failed to marshal expanded target: %v", err)
	}

	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("failed to unmarshal wire payload: %v", err)
	}
	if _, exists := wire["renotifyConfig"]; exists {
		t.Fatalf("expected renotifyConfig to be omitted at target level, got %s", raw)
	}
	if _, exists := wire["config"].(map[string]any)["renotifyConfig"]; exists {
		t.Fatalf("expected renotifyConfig to be omitted from config, got %s", raw)
	}

	back := flattenOneTarget(t, ctx, expanded)
	if back.Config.Renotify != nil {
		t.Fatalf("expected renotify to flatten back to null, got %#v", back.Config.Renotify)
	}
}

func TestFlattenNotificationRuleTargets_ReadsRenotifyFromConfig(t *testing.T) {
	ctx := context.Background()

	back := flattenOneTarget(t, ctx, notificationRuleAPITarget{
		ID: "slack",
		Config: notificationRuleAPITargetConfig{
			Type:          "slack",
			Channel:       "C123",
			IntegrationID: "T123",
			RenotifyConfig: &notificationRuleAPITargetRenotify{
				Mode:                    "each",
				RenotificationStates:    []string{"alert_no_data"},
				RenotifyIntervalMinutes: 45,
			},
		},
	})

	if back.Config.Renotify == nil {
		t.Fatal("expected config.renotifyConfig to flatten onto config.renotify")
	}
	if back.Config.Renotify.RenotifyIntervalMinutes.ValueInt64() != 45 {
		t.Fatalf("expected renotify_interval_minutes 45, got %v", back.Config.Renotify.RenotifyIntervalMinutes)
	}
}

func TestExpandFlattenNotificationRuleTargets_ServiceNow(t *testing.T) {
	ctx := context.Background()

	expanded := expandOneTarget(t, ctx, resource_notification_rule.TargetModel{
		Id: types.StringValue("servicenow"),
		Config: resource_notification_rule.TargetConfigModel{
			ServiceNow: &resource_notification_rule.IntegrationOnlyConfigModel{
				IntegrationID: types.StringValue("snow-123"),
			},
		},
	})
	if expanded.Config.Type != "servicenow" {
		t.Fatalf("expected wire type servicenow, got %q", expanded.Config.Type)
	}

	back := flattenOneTarget(t, ctx, expanded)
	if back.Config.ServiceNow == nil {
		t.Fatal("expected servicenow config to survive the round-trip")
	}
	if back.Config.ServiceNow.IntegrationID.ValueString() != "snow-123" {
		t.Fatalf("expected integration_id snow-123, got %v", back.Config.ServiceNow.IntegrationID)
	}
	if back.Config.ServiceNow.Type.ValueString() != "servicenow" {
		t.Fatalf("expected type servicenow, got %v", back.Config.ServiceNow.Type)
	}
}

func TestExpandFlattenNotificationRuleTargets_GoogleChatUsesKebabCaseWireType(t *testing.T) {
	ctx := context.Background()

	expanded := expandOneTarget(t, ctx, resource_notification_rule.TargetModel{
		Id: types.StringValue("google-chat"),
		Config: resource_notification_rule.TargetConfigModel{
			GoogleChat: &resource_notification_rule.IntegrationOnlyConfigModel{
				IntegrationID: types.StringValue("gchat-123"),
			},
		},
	})
	if expanded.Config.Type != "google-chat" {
		t.Fatalf("expected wire type google-chat, got %q", expanded.Config.Type)
	}

	back := flattenOneTarget(t, ctx, expanded)
	if back.Config.GoogleChat == nil {
		t.Fatal("expected google_chat config to survive the round-trip")
	}
	if back.Config.GoogleChat.IntegrationID.ValueString() != "gchat-123" {
		t.Fatalf("expected integration_id gchat-123, got %v", back.Config.GoogleChat.IntegrationID)
	}
	if back.Config.GoogleChat.Type.ValueString() != "google-chat" {
		t.Fatalf("expected type google-chat, got %v", back.Config.GoogleChat.Type)
	}
}

func TestExpandFlattenNotificationRuleTargets_Jira(t *testing.T) {
	ctx := context.Background()

	expanded := expandOneTarget(t, ctx, resource_notification_rule.TargetModel{
		Id: types.StringValue("jira"),
		Config: resource_notification_rule.TargetConfigModel{
			Jira: &resource_notification_rule.JiraConfigModel{
				IntegrationID: types.StringValue("jira-123"),
				ProjectKey:    types.StringValue("OPS"),
				IssueType:     types.StringValue("Bug"),
				OpenStatus:    types.StringValue("To Do"),
				ClosedStatus:  types.StringValue("Done"),
			},
		},
	})
	if expanded.Config.Type != "jira" {
		t.Fatalf("expected wire type jira, got %q", expanded.Config.Type)
	}
	if expanded.Config.ProjectKey != "OPS" || expanded.Config.IssueType != "Bug" {
		t.Fatalf("expected projectKey OPS and issueType Bug, got %q and %q", expanded.Config.ProjectKey, expanded.Config.IssueType)
	}
	if expanded.Config.OpenStatus != "To Do" || expanded.Config.ClosedStatus != "Done" {
		t.Fatalf("expected openStatus 'To Do' and closedStatus Done, got %q and %q", expanded.Config.OpenStatus, expanded.Config.ClosedStatus)
	}

	back := flattenOneTarget(t, ctx, expanded)
	if back.Config.Jira == nil {
		t.Fatal("expected jira config to survive the round-trip")
	}
	if back.Config.Jira.ProjectKey.ValueString() != "OPS" {
		t.Fatalf("expected project_key OPS, got %v", back.Config.Jira.ProjectKey)
	}
	if back.Config.Jira.IssueType.ValueString() != "Bug" {
		t.Fatalf("expected issue_type Bug, got %v", back.Config.Jira.IssueType)
	}
	if back.Config.Jira.OpenStatus.ValueString() != "To Do" {
		t.Fatalf("expected open_status 'To Do', got %v", back.Config.Jira.OpenStatus)
	}
	if back.Config.Jira.ClosedStatus.ValueString() != "Done" {
		t.Fatalf("expected closed_status Done, got %v", back.Config.Jira.ClosedStatus)
	}
}

func TestExpandNotificationRuleTargets_JiraOmitsUnsetStatuses(t *testing.T) {
	ctx := context.Background()

	expanded := expandOneTarget(t, ctx, resource_notification_rule.TargetModel{
		Id: types.StringValue("jira"),
		Config: resource_notification_rule.TargetConfigModel{
			Jira: &resource_notification_rule.JiraConfigModel{
				IntegrationID: types.StringValue("jira-123"),
				ProjectKey:    types.StringValue("OPS"),
				IssueType:     types.StringValue("Bug"),
			},
		},
	})

	raw, err := json.Marshal(expanded.Config)
	if err != nil {
		t.Fatalf("failed to marshal expanded config: %v", err)
	}

	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("failed to unmarshal wire payload: %v", err)
	}
	if _, exists := wire["openStatus"]; exists {
		t.Fatalf("expected openStatus to be omitted when unset, got %s", raw)
	}
	if _, exists := wire["closedStatus"]; exists {
		t.Fatalf("expected closedStatus to be omitted when unset, got %s", raw)
	}
}

func TestValidateTargetConfig_JiraRejectsRenotify(t *testing.T) {
	ctx := context.Background()
	r := &notificationRuleResource{}

	cfg := resource_notification_rule.TargetConfigModel{
		Jira: &resource_notification_rule.JiraConfigModel{
			IntegrationID: types.StringValue("jira-123"),
			ProjectKey:    types.StringValue("OPS"),
			IssueType:     types.StringValue("Bug"),
		},
	}

	if diags := r.validateTargetConfig(cfg, 0); diags.HasError() {
		t.Fatalf("expected a jira target without renotify to validate, got %v", diags)
	}

	cfg.Renotify = renotifyModel(t, ctx, "alert")
	diags := r.validateTargetConfig(cfg, 0)
	if !diags.HasError() {
		t.Fatal("expected an error for a jira target combined with renotify")
	}
	if got := diags.Errors()[0].Detail(); got != "renotify is not supported on a jira destination." {
		t.Fatalf("unexpected error detail: %q", got)
	}
}
