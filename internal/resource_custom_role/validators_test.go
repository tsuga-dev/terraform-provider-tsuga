package resource_custom_role

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestReadOnlyAssets(t *testing.T) {
	assets := func(elements map[string]attr.Value) types.Map {
		return types.MapValueMust(types.StringType, elements)
	}
	cases := []struct {
		name      string
		value     types.Map
		wantError bool
	}{
		{"read on a read-only type", assets(map[string]attr.Value{"database": types.StringValue("read")}), false},
		{"write on a writable type", assets(map[string]attr.Value{"dashboard": types.StringValue("write")}), false},
		{"write on a read-only type", assets(map[string]attr.Value{"database": types.StringValue("write")}), true},
		{"unknown level skipped", assets(map[string]attr.Value{"database": types.StringUnknown()}), false},
		{"null map skipped", types.MapNull(types.StringType), false},
		{"unknown map skipped", types.MapUnknown(types.StringType), false},
	}

	v := readOnlyAssets([]string{"database", "service"})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.MapResponse{}
			v.ValidateMap(context.Background(), validator.MapRequest{Path: path.Root("assets"), ConfigValue: tc.value}, resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("hasError = %t, want %t: %v", resp.Diagnostics.HasError(), tc.wantError, resp.Diagnostics)
			}
		})
	}
}

func TestRequiredAssets(t *testing.T) {
	v := requiredAssets([]string{"team"})
	cases := []struct {
		name      string
		value     types.Map
		wantError bool
	}{
		{"key present", types.MapValueMust(types.StringType, map[string]attr.Value{"team": types.StringValue("read")}), false},
		{"key missing", types.MapValueMust(types.StringType, map[string]attr.Value{"dashboard": types.StringValue("read")}), true},
		{"null map skipped", types.MapNull(types.StringType), false},
		{"unknown map skipped", types.MapUnknown(types.StringType), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.MapResponse{}
			v.ValidateMap(context.Background(), validator.MapRequest{Path: path.Root("assets"), ConfigValue: tc.value}, resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("hasError = %t, want %t: %v", resp.Diagnostics.HasError(), tc.wantError, resp.Diagnostics)
			}
		})
	}
}
