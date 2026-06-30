package resource_slo

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExclusiveBetween(t *testing.T) {
	cases := []struct {
		name      string
		value     types.Float64
		wantError bool
	}{
		{"valid mid", types.Float64Value(50), false},
		{"valid high", types.Float64Value(99.9), false},
		{"lower bound excluded", types.Float64Value(0), true},
		{"upper bound excluded", types.Float64Value(100), true},
		{"below range", types.Float64Value(-1), true},
		{"above range", types.Float64Value(150), true},
		{"null skipped", types.Float64Null(), false},
		{"unknown skipped", types.Float64Unknown(), false},
	}

	v := exclusiveBetween(0, 100)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.Float64Response{}
			v.ValidateFloat64(context.Background(), validator.Float64Request{
				Path:        path.Root("target"),
				ConfigValue: tc.value,
			}, resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("exclusiveBetween(0,100) on %v: HasError=%v, want %v", tc.value, resp.Diagnostics.HasError(), tc.wantError)
			}
		})
	}
}
