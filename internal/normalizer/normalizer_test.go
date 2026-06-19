package normalizer

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidate_UnitRequiredTypes(t *testing.T) {
	for _, normType := range []string{"duration", "data", "custom"} {
		// Missing unit is rejected.
		missing := &Model{Type: types.StringValue(normType), Unit: types.StringNull()}
		if diags := Validate(missing, "v"); !diags.HasError() {
			t.Errorf("expected error when %q normalizer omits unit", normType)
		}

		// Empty unit is rejected.
		empty := &Model{Type: types.StringValue(normType), Unit: types.StringValue("")}
		if diags := Validate(empty, "v"); !diags.HasError() {
			t.Errorf("expected error when %q normalizer has empty unit", normType)
		}

		// Unit present is accepted.
		present := &Model{Type: types.StringValue(normType), Unit: types.StringValue("ms")}
		if diags := Validate(present, "v"); diags.HasError() {
			t.Errorf("unexpected error for %q normalizer with unit: %v", normType, diags)
		}
	}
}

func TestValidate_UnitOptionalTypes(t *testing.T) {
	for _, normType := range []string{"percent", "date", "level", "cpu"} {
		m := &Model{Type: types.StringValue(normType), Unit: types.StringNull()}
		if diags := Validate(m, "v"); diags.HasError() {
			t.Errorf("unexpected error for %q normalizer without unit: %v", normType, diags)
		}
	}
}

func TestValidate_NilModel(t *testing.T) {
	if diags := Validate(nil, "v"); diags.HasError() {
		t.Errorf("unexpected error for nil normalizer: %v", diags)
	}
}
