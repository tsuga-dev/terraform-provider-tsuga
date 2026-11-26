package provider

import (
	"context"
	"reflect"
	"testing"

	"terraform-provider-tsuga/internal/resource_tag_policy"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

func ptr[T any](value T) *T { return &value }

func TestTagPolicy_ExpandFlattenRoundTrips(t *testing.T) {
	ctx := context.Background()

	cases := map[string]tagPolicyAPIConfiguration{
		"telemetry":   {Type: "telemetry", AssetTypes: []string{"logs", "traces"}, ShouldInsertWarning: ptr(true), DropSample: ptr(10.5)},
		"tsuga_asset": {Type: "tsuga_asset", AssetTypes: []string{"dashboard", "span-route"}},
	}
	for name, configuration := range cases {
		t.Run(name, func(t *testing.T) {
			flattened, diags := flattenConfiguration(ctx, configuration)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			expanded, diags := expandConfiguration(ctx, flattened)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			want := map[string]any{"type": configuration.Type, "assetTypes": configuration.AssetTypes}
			if configuration.Type == "telemetry" {
				want["shouldInsertWarning"] = true
				want["dropSample"] = 10.5
			}
			if !reflect.DeepEqual(expanded, want) {
				t.Fatalf("got %#v, want %#v", expanded, want)
			}
		})
	}
}

func TestExpandConfiguration_NoneSet(t *testing.T) {
	ctx := context.Background()
	empty, diags := flattenConfiguration(ctx, tagPolicyAPIConfiguration{Type: "unknown"})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if _, diags := expandConfiguration(ctx, empty); !diags.HasError() {
		t.Fatal("expected an error when no configuration block is set")
	}
}

func TestFlattenTagPolicy_FitsTheGeneratedSchema(t *testing.T) {
	ctx := context.Background()

	state, diags := flattenTagPolicy(ctx, tagPolicyAPIData{
		ID:               "policy-1",
		Name:             "policy",
		IsActive:         true,
		TagKey:           "env",
		AllowedTagValues: []string{"prod"},
		IsRequired:       true,
		TeamScope:        &tagPolicyAPITeamScope{TeamIds: []string{"team-1"}, Mode: "include"},
		Configuration:    tagPolicyAPIConfiguration{Type: "telemetry", AssetTypes: []string{}, ShouldInsertWarning: ptr(false)},
		Owner:            "team-1",
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	got := tfsdk.State{Schema: resource_tag_policy.TagPolicyResourceSchema(ctx)}
	if d := got.Set(ctx, &state); d.HasError() {
		t.Fatalf("flattened policy does not fit the schema: %v", d)
	}
}
