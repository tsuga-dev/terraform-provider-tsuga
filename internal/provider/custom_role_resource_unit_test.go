package provider

import (
	"context"
	"encoding/json"
	"testing"

	"terraform-provider-tsuga/internal/resource_custom_role"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCustomRoleRequestBody(t *testing.T) {
	ctx := context.Background()
	queryElemType := resource_custom_role.TelemetryQueryElemType
	assets := types.MapValueMust(types.StringType, map[string]attr.Value{
		"team":           types.StringValue("read"),
		"monitor-folder": types.StringValue("write"),
	})

	tests := []struct {
		name string
		plan resource_custom_role.CustomRoleModel
		want string
	}{
		{
			name: "specific scope with description and query cleared",
			plan: resource_custom_role.CustomRoleModel{
				Name:        types.StringValue("platform-on-call"),
				Description: types.StringNull(),
				TeamScope: &resource_custom_role.TeamScopeModel{
					Type:    types.StringValue("specific"),
					TeamIds: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("team-1"), types.StringValue("team-2")}),
				},
				Assets:         assets,
				Telemetry:      types.SetValueMust(types.StringType, []attr.Value{}),
				TelemetryQuery: types.ListNull(queryElemType),
			},
			want: `{"assets":{"monitor-folder":"write","team":"read"},"description":null,"name":"platform-on-call",` +
				`"teamScope":{"type":"specific","teamIds":["team-1","team-2"]},"telemetry":[],"telemetryQuery":null}`,
		},
		{
			name: "all scope with description and query",
			plan: resource_custom_role.CustomRoleModel{
				Name:        types.StringValue("readers"),
				Description: types.StringValue("Reads production telemetry"),
				TeamScope: &resource_custom_role.TeamScopeModel{
					Type:    types.StringValue("all"),
					TeamIds: types.SetNull(types.StringType),
				},
				Assets:    assets,
				Telemetry: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("log"), types.StringValue("trace")}),
				TelemetryQuery: types.ListValueMust(queryElemType, []attr.Value{
					types.ObjectValueMust(queryElemType.AttrTypes, map[string]attr.Value{
						"key":    types.StringValue("context.env"),
						"values": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("production")}),
					}),
				}),
			},
			want: `{"assets":{"monitor-folder":"write","team":"read"},"description":"Reads production telemetry","name":"readers",` +
				`"teamScope":{"type":"all"},"telemetry":["log","trace"],"telemetryQuery":[{"key":"context.env","values":["production"]}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, diags := (&customRoleResource{}).buildCustomRoleRequestBody(ctx, tc.plan)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}
			if string(raw) != tc.want {
				t.Fatalf("body = %s\nwant   %s", raw, tc.want)
			}
		})
	}
}

func TestFlattenCustomRole(t *testing.T) {
	ctx := context.Background()
	queryElemType := resource_custom_role.TelemetryQueryElemType

	t.Run("omitted fields read back as null", func(t *testing.T) {
		data := customRoleAPIData{
			ID:        "role-1",
			Name:      "readers",
			TeamScope: customRoleAPITeamScope{Type: "public"},
			Assets:    map[string]string{"team": "read"},
			Telemetry: []string{},
		}

		state, diags := flattenCustomRole(ctx, data)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		if !state.Description.IsNull() {
			t.Fatalf("description = %s, want null", state.Description)
		}
		if state.TeamScope.Type.ValueString() != "public" || !state.TeamScope.TeamIds.IsNull() {
			t.Fatalf("team_scope = %+v, want type public and null team_ids", state.TeamScope)
		}
		if !state.TelemetryQuery.IsNull() {
			t.Fatalf("telemetry_query = %s, want null", state.TelemetryQuery)
		}
		if state.Telemetry.IsNull() || len(state.Telemetry.Elements()) != 0 {
			t.Fatalf("telemetry = %s, want an empty set", state.Telemetry)
		}
	})

	t.Run("telemetry_query without entries is null when omitted and empty when returned empty", func(t *testing.T) {
		cases := []struct {
			name     string
			returned []customRoleAPITelemetryQuery
			wantNull bool
		}{
			{name: "field omitted", returned: nil, wantNull: true},
			{name: "empty array returned", returned: []customRoleAPITelemetryQuery{}, wantNull: false},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				data := customRoleAPIData{ID: "role-1", TeamScope: customRoleAPITeamScope{Type: "all"}, TelemetryQuery: tc.returned}

				state, diags := flattenCustomRole(ctx, data)
				if diags.HasError() {
					t.Fatalf("unexpected diagnostics: %v", diags)
				}

				if state.TelemetryQuery.IsNull() != tc.wantNull || len(state.TelemetryQuery.Elements()) != 0 {
					t.Fatalf("telemetry_query = %s, want null=%t and no elements", state.TelemetryQuery, tc.wantNull)
				}
			})
		}
	})

	t.Run("populated fields", func(t *testing.T) {
		data := customRoleAPIData{
			ID:             "role-1",
			Name:           "platform-on-call",
			Description:    "Edits monitors",
			TeamScope:      customRoleAPITeamScope{Type: "all-except", TeamIds: []string{"team-1"}},
			Assets:         map[string]string{"team": "read", "monitor": "write"},
			Telemetry:      []string{"trace", "log"},
			TelemetryQuery: []customRoleAPITelemetryQuery{{Key: "context.env", Values: []string{"staging", "dev"}}},
		}

		state, diags := flattenCustomRole(ctx, data)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		if state.Description.ValueString() != "Edits monitors" {
			t.Fatalf("description = %s", state.Description)
		}
		wantTeamIds := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("team-1")})
		if state.TeamScope.Type.ValueString() != "all-except" || !state.TeamScope.TeamIds.Equal(wantTeamIds) {
			t.Fatalf("team_scope = %+v, want type all-except and team_ids %s", state.TeamScope, wantTeamIds)
		}
		if got := state.Assets.Elements()["monitor"]; !got.Equal(types.StringValue("write")) {
			t.Fatalf("assets.monitor = %s, want write", got)
		}
		// Sets compare regardless of order.
		wantTelemetry := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("log"), types.StringValue("trace")})
		if !state.Telemetry.Equal(wantTelemetry) {
			t.Fatalf("telemetry = %s, want %s", state.Telemetry, wantTelemetry)
		}
		wantQuery := types.ListValueMust(queryElemType, []attr.Value{
			types.ObjectValueMust(queryElemType.AttrTypes, map[string]attr.Value{
				"key":    types.StringValue("context.env"),
				"values": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("staging"), types.StringValue("dev")}),
			}),
		})
		if !state.TelemetryQuery.Equal(wantQuery) {
			t.Fatalf("telemetry_query = %s, want %s", state.TelemetryQuery, wantQuery)
		}
	})
}
