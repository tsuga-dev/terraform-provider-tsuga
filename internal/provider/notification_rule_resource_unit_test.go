package provider

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"terraform-provider-tsuga/internal/apiclient"
	"terraform-provider-tsuga/internal/resource_notification_rule"
	"terraform-provider-tsuga/internal/validators"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

const renotifyJSON = `{"mode": "each", "renotificationStates": ["alert", "alert_no_data"], "renotifyIntervalMinutes": 30}`

// objectAttribute returns the named attribute of an object, or a null object when it is absent.
func objectAttribute(obj types.Object, name string) types.Object {
	value, _ := obj.Attributes()[name].(types.Object)
	return value
}

func decodeRule(t *testing.T, targets ...string) apiclient.Rule {
	t.Helper()
	raw := `{"id": "rule-1", "teamsFilter": {"type": "all-teams"}, "targets": [` + strings.Join(targets, ",") + `]}`
	var rule apiclient.Rule
	if err := json.Unmarshal([]byte(raw), &rule); err != nil {
		t.Fatalf("failed to decode %s: %v", raw, err)
	}
	return rule
}

func target(id, config string) string {
	return `{"id": "` + id + `", "config": ` + config + `}`
}

func slackConfig(renotify string) string {
	config := `{"type": "slack", "channel": "C123", "integrationId": "T123", "integrationName": "workspace"`
	if renotify != "" {
		config += `, "renotifyConfig": ` + renotify
	}
	return config + `}`
}

func slackRule(t *testing.T, renotify string) apiclient.Rule {
	return decodeRule(t, target("slack", slackConfig(renotify)))
}

func assertJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("invalid JSON %s: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("invalid JSON %s: %v", want, err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

// roundTrip flattens the targets of an API rule to Terraform values, then expands them back to the
// request body, returned as JSON.
func roundTrip(t *testing.T, ctx context.Context, rule apiclient.Rule) ([]byte, resource_notification_rule.TargetsValue) {
	t.Helper()

	flattened, diags := flattenNotificationRuleTargets(ctx, rule)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var models []resource_notification_rule.TargetsValue
	if d := flattened.ElementsAs(ctx, &models, false); d.HasError() {
		t.Fatalf("failed to decode flattened targets: %v", d)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 flattened target, got %d", len(models))
	}

	var body apiclient.CreateNotificationRuleJSONBody
	if diags := expandNotificationRuleTargets(ctx, flattened, &body); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	raw, err := json.Marshal(body.Targets[0])
	if err != nil {
		t.Fatalf("failed to marshal expanded target: %v", err)
	}
	return raw, models[0]
}

func TestExpandNotificationRuleTargets_RenotifyIsNestedInDestination(t *testing.T) {
	// The API reads renotifyConfig from the destination inside targets[].config.
	raw, _ := roundTrip(t, context.Background(), slackRule(t, renotifyJSON))
	assertJSON(t, raw, `{"id": "slack", "config": {"type": "slack", "channel": "C123", "integrationId": "T123", "renotifyConfig": `+renotifyJSON+`}}`)
}

func TestFlattenNotificationRuleTargets_ReadsRenotifyFromDestination(t *testing.T) {
	_, back := roundTrip(t, context.Background(), slackRule(t, renotifyJSON))

	renotify := objectAttribute(objectAttribute(back.Config, "slack"), "renotify")
	if renotify.IsNull() {
		t.Fatal("expected renotifyConfig to flatten onto slack.renotify")
	}
	if got := renotify.Attributes()["renotify_interval_minutes"].(types.Int64).ValueInt64(); got != 30 {
		t.Fatalf("expected renotify_interval_minutes 30, got %d", got)
	}
	states := renotify.Attributes()["renotification_states"].(types.List)
	if len(states.Elements()) != 2 || states.Elements()[0].(types.String).ValueString() != "alert" {
		t.Fatalf("expected renotification_states [alert alert_no_data], got %s", states)
	}
}

func TestExpandNotificationRuleTargets_WithoutRenotifyOmitsTheKey(t *testing.T) {
	raw, back := roundTrip(t, context.Background(), slackRule(t, ""))
	if strings.Contains(string(raw), "renotifyConfig") {
		t.Fatalf("expected renotifyConfig to be omitted from config, got %s", raw)
	}
	if renotify := objectAttribute(objectAttribute(back.Config, "slack"), "renotify"); !renotify.IsNull() {
		t.Fatalf("expected renotify to flatten back to null, got %s", renotify)
	}
}

func TestExpandFlattenNotificationRuleTargets_RenotifyOnEveryDestination(t *testing.T) {
	for _, c := range []struct{ block, wireType, fields string }{
		{"email", "email", `"addresses": ["a@example.com"]`},
		{"webhook", "webhook", `"integrationId": "int-123", "integrationName": "name"`},
		{"pagerduty", "pagerduty", `"integrationId": "int-123", "integrationName": "name"`},
	} {
		t.Run(c.block, func(t *testing.T) {
			config := `{"type": "` + c.wireType + `", ` + c.fields + `, "renotifyConfig": ` + renotifyJSON + `}`
			raw, back := roundTrip(t, context.Background(), decodeRule(t, target(c.block, config)))
			if !strings.Contains(string(raw), `"renotifyConfig":`) {
				t.Fatalf("expected renotifyConfig in %s", raw)
			}
			if objectAttribute(objectAttribute(back.Config, c.block), "renotify").IsNull() {
				t.Fatalf("expected %s.renotify to be set", c.block)
			}
		})
	}
}

func TestSlackValue_NullRenotifyToObjectValue(t *testing.T) {
	ctx := context.Background()
	renotifyTypes := resource_notification_rule.SlackRenotifyValue{}.AttributeTypes(ctx)
	for name, renotify := range map[string]basetypes.ObjectValue{
		"null":    types.ObjectNull(renotifyTypes),
		"unknown": types.ObjectUnknown(renotifyTypes),
	} {
		v := resource_notification_rule.NewSlackValueMust(resource_notification_rule.SlackValue{}.AttributeTypes(ctx), map[string]attr.Value{
			"channel": types.StringValue("C1"), "integration_id": types.StringValue("T1"),
			"integration_name": types.StringNull(), "type": types.StringNull(),
			"renotify": renotify,
		})
		if _, d := v.ToObjectValue(ctx); d.HasError() {
			t.Fatalf("%s: %v", name, d)
		}
	}
}

func TestExpandFlattenNotificationRuleTargets_WireTypeAndComputedFields(t *testing.T) {
	ctx := context.Background()

	for _, c := range []struct{ block, wireType string }{
		{"pagerduty", "pagerduty"},
		{"servicenow", "servicenow"},
		{"google_chat", "google-chat"},
		{"microsoft_teams", "microsoft-teams"},
		{"grafana_irm", "grafana-irm"},
		{"incident_io", "incident-io"},
		{"squadcast", "squadcast"},
		{"webhook", "webhook"},
	} {
		t.Run(c.block, func(t *testing.T) {
			rule := decodeRule(t, target(c.block, `{"type": "`+c.wireType+`", "integrationId": "int-123", "integrationName": "name"}`))
			raw, back := roundTrip(t, ctx, rule)

			assertJSON(t, raw, `{"id": "`+c.block+`", "config": {"type": "`+c.wireType+`", "integrationId": "int-123"}}`)

			block := objectAttribute(back.Config, c.block).Attributes()
			if block["integration_id"].(types.String).ValueString() != "int-123" {
				t.Fatalf("expected integration_id int-123, got %v", block["integration_id"])
			}
			if block["type"].(types.String).ValueString() != c.wireType {
				t.Fatalf("expected type %s, got %v", c.wireType, block["type"])
			}
			if block["integration_name"].(types.String).ValueString() != "name" {
				t.Fatalf("expected integration_name name, got %v", block["integration_name"])
			}
		})
	}
}

func TestExpandFlattenNotificationRuleTargets_Jira(t *testing.T) {
	config := `{"type": "jira", "integrationId": "jira-123", "integrationName": "jira", "projectKey": "OPS", "issueType": "Bug", "openStatus": "To Do", "closedStatus": "Done"}`
	raw, back := roundTrip(t, context.Background(), decodeRule(t, target("jira", config)))
	assertJSON(t, raw, `{"id": "jira", "config": {"type": "jira", "integrationId": "jira-123", "projectKey": "OPS", "issueType": "Bug", "openStatus": "To Do", "closedStatus": "Done"}}`)

	jira := objectAttribute(back.Config, "jira").Attributes()
	for name, want := range map[string]string{"project_key": "OPS", "issue_type": "Bug", "open_status": "To Do", "closed_status": "Done"} {
		if got := jira[name].(types.String).ValueString(); got != want {
			t.Fatalf("expected %s %q, got %q", name, want, got)
		}
	}
}

func TestExpandNotificationRuleTargets_JiraOmitsUnsetStatuses(t *testing.T) {
	config := `{"type": "jira", "integrationId": "jira-123", "projectKey": "OPS", "issueType": "Bug"}`
	raw, back := roundTrip(t, context.Background(), decodeRule(t, target("jira", config)))
	for _, key := range []string{"openStatus", "closedStatus"} {
		if strings.Contains(string(raw), key) {
			t.Fatalf("expected %s to be omitted when unset, got %s", key, raw)
		}
	}
	if !objectAttribute(back.Config, "jira").Attributes()["open_status"].IsNull() {
		t.Fatal("expected an absent openStatus to flatten to null")
	}
}

func TestExpandFlattenNotificationRuleTargets_RateLimit(t *testing.T) {
	rule := decodeRule(t, `{"id": "email", "config": {"type": "email", "addresses": ["a@example.com"]}, "rateLimit": {"maxMessages": 5, "minutes": 10}}`)
	raw, _ := roundTrip(t, context.Background(), rule)
	assertJSON(t, raw, `{"id": "email", "config": {"type": "email", "addresses": ["a@example.com"]}, "rateLimit": {"maxMessages": 5, "minutes": 10}}`)
}

func TestFlattenNotificationRuleTargets_UnsupportedDestinationIsADiagnostic(t *testing.T) {
	_, diags := flattenNotificationRuleTargets(context.Background(), decodeRule(t, target("x", `{"type": "carrier-pigeon"}`)))
	if !diags.HasError() || !strings.Contains(diags.Errors()[0].Detail(), "carrier-pigeon") {
		t.Fatalf("expected an unsupported destination diagnostic, got %v", diags)
	}
}

func TestExactlyOneDestination(t *testing.T) {
	ctx := context.Background()
	_, back := roundTrip(t, ctx, slackRule(t, renotifyJSON))
	validate := func(cfg types.Object) string {
		resp := &validator.ObjectResponse{}
		validators.ExactlyOneOf("Invalid target config", "destination").ValidateObject(ctx, validator.ObjectRequest{ConfigValue: cfg}, resp)
		if !resp.Diagnostics.HasError() {
			return ""
		}
		return resp.Diagnostics.Errors()[0].Detail()
	}

	if got := validate(back.Config); got != "" {
		t.Fatalf("expected one destination to validate, got %q", got)
	}

	attrs := back.Config.Attributes()
	attrs["email"] = types.ObjectValueMust(objectAttribute(back.Config, "email").AttributeTypes(ctx), map[string]attr.Value{
		"addresses": types.ListNull(types.StringType),
		"type":      types.StringNull(),
		"renotify":  renotifyObject(ctx, nil),
	})
	two := types.ObjectValueMust(back.Config.AttributeTypes(ctx), attrs)
	want := "Exactly one destination must be set, one of: email, google_chat, grafana_irm, incident_io, jira, microsoft_teams, pagerduty, servicenow, slack, squadcast, webhook. Set: email, slack."
	if got := validate(two); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFlattenNotificationRule_FitsTheGeneratedSchema(t *testing.T) {
	ctx := context.Background()

	rule := slackRule(t, renotifyJSON)
	rule.Name = "rule"
	rule.PrioritiesFilter = []apiclient.RulePrioritiesFilter{1, 2}
	rule.TransitionTypesFilter = []apiclient.RuleTransitionTypesFilter{"triggered"}
	rule.Owner = "team-1"
	rule.IsActive = true
	state, diags := flattenNotificationRule(ctx, rule)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	schema := resource_notification_rule.NotificationRuleResourceSchema(ctx)
	got := tfsdk.State{Schema: schema}
	if d := got.Set(ctx, &state); d.HasError() {
		t.Fatalf("flattened rule does not fit the schema: %v", d)
	}

	var back resource_notification_rule.NotificationRuleModel
	if d := got.Get(ctx, &back); d.HasError() {
		t.Fatalf("failed to read the state back: %v", d)
	}
	if back.ClusterIdsFilter.IsNull() || len(back.ClusterIdsFilter.Elements()) != 0 {
		t.Fatalf("expected an absent clusterIdsFilter to flatten to an empty list, got %s", back.ClusterIdsFilter)
	}
}

func TestBuildNotificationRuleRequestBody_OmittedClusterIdsFilterSendsEmptyList(t *testing.T) {
	ctx := context.Background()
	r := &notificationRuleResource{}

	flattened, diags := flattenNotificationRule(ctx, slackRule(t, ""))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	// The default turns an omitted attribute into an empty list in the plan; an unset one is null.
	for name, list := range map[string]types.List{
		"planned default": flattened.ClusterIdsFilter,
		"null":            types.ListNull(types.StringType),
	} {
		t.Run(name, func(t *testing.T) {
			flattened.ClusterIdsFilter = list
			body, diags := r.buildNotificationRuleRequestBody(ctx, flattened)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got, ok := body["clusterIdsFilter"].([]string); !ok || len(got) != 0 {
				t.Fatalf("expected clusterIdsFilter to be an empty list, got %#v", body["clusterIdsFilter"])
			}
		})
	}
}

func TestExpandTarget_UnknownComputedFieldsAreNotSent(t *testing.T) {
	ctx := context.Background()
	_, back := roundTrip(t, ctx, slackRule(t, ""))

	slack := objectAttribute(back.Config, "slack")
	slackAttrs := slack.Attributes()
	slackAttrs["integration_name"] = types.StringUnknown()
	slackAttrs["type"] = types.StringUnknown()
	configAttrs := back.Config.Attributes()
	configAttrs["slack"] = types.ObjectValueMust(slack.AttributeTypes(ctx), slackAttrs)
	config := types.ObjectValueMust(back.Config.AttributeTypes(ctx), configAttrs)

	var body apiclient.CreateNotificationRuleJSONBody
	target := grow(&body.Targets)
	if diags := expandTarget(ctx, types.StringUnknown(), config, back.RateLimit, &target.Id, &target.Config, &target.RateLimit); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	raw, err := json.Marshal(body.Targets[0])
	if err != nil {
		t.Fatal(err)
	}
	assertJSON(t, raw, `{"id": "", "config": {"type": "slack", "channel": "C123", "integrationId": "T123"}}`)
}
