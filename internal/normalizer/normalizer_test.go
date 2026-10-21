package normalizer

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidate_UnitRequiredTypes(t *testing.T) {
	validUnit := map[string]string{"duration": "ms", "data": "MB", "custom": "widgets"}
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

		// Unit not in the type's allow-list is rejected (custom has no
		// allow-list, just a length bound; covered separately below).
		if normType != "custom" {
			invalid := &Model{Type: types.StringValue(normType), Unit: types.StringValue("not-a-unit")}
			if diags := Validate(invalid, "v"); !diags.HasError() {
				t.Errorf("expected error when %q normalizer has invalid unit", normType)
			}
		}

		// A valid unit for the type is accepted.
		present := &Model{Type: types.StringValue(normType), Unit: types.StringValue(validUnit[normType])}
		if diags := Validate(present, "v"); diags.HasError() {
			t.Errorf("unexpected error for %q normalizer with unit %q: %v", normType, validUnit[normType], diags)
		}
	}
}

func TestValidate_CustomUnitLength(t *testing.T) {
	tooLong := &Model{Type: types.StringValue("custom"), Unit: types.StringValue("123456789012345678901")}
	if diags := Validate(tooLong, "v"); !diags.HasError() {
		t.Error("expected error for custom unit longer than 20 characters")
	}

	multiByte := &Model{Type: types.StringValue("custom"), Unit: types.StringValue(strings.Repeat("µ", 20))}
	if diags := Validate(multiByte, "v"); diags.HasError() {
		t.Errorf("unexpected error for 20-character multi-byte custom unit: %v", diags)
	}
}

func TestValidate_UnitOptionalTypes(t *testing.T) {
	for _, normType := range []string{"percent", "percent-fraction", "date", "level", "cpu", "none"} {
		m := &Model{Type: types.StringValue(normType), Unit: types.StringNull()}
		if diags := Validate(m, "v"); diags.HasError() {
			t.Errorf("unexpected error for %q normalizer without unit: %v", normType, diags)
		}
	}
}

func TestValidate_UnknownUnitSkipsCheck(t *testing.T) {
	m := &Model{Type: types.StringValue("duration"), Unit: types.StringUnknown()}
	if diags := Validate(m, "v"); diags.HasError() {
		t.Errorf("unexpected error for unknown unit: %v", diags)
	}
}

func TestValidate_NilModel(t *testing.T) {
	if diags := Validate(nil, "v"); diags.HasError() {
		t.Errorf("unexpected error for nil normalizer: %v", diags)
	}
}
