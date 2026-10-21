package normalizer

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Schema returns the schema for a normalizer object.
func Schema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Required:   true,
				Validators: normalizerTypeValidators(),
			},
			"unit": schema.StringAttribute{
				Optional: true,
				Description: fmt.Sprintf(
					"Unit label. Required for duration (one of %s), data (one of %s) and custom (%d-%d characters) normalizers; unset otherwise.",
					strings.Join(durationUnits, ", "), strings.Join(dataUnits, ", "), customUnitMinLength, customUnitMaxLength,
				),
			},
		},
	}
}

// AttrTypes returns attr types for a normalizer object.
func AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"unit": types.StringType,
	}
}

// Model represents a normalizer configuration.
type Model struct {
	Type types.String `tfsdk:"type"`
	Unit types.String `tfsdk:"unit"`
}

func Validate(m *Model, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics
	if m == nil {
		return diags
	}

	unitType := m.Type.ValueString()
	switch unitType {
	case "duration", "data", "custom":
		if m.Unit.IsUnknown() {
			return diags
		}
		if m.Unit.IsNull() || m.Unit.ValueString() == "" {
			diags.AddError(
				"Invalid normalizer configuration",
				fmt.Sprintf("%s.normalizer: the %q normalizer requires \"unit\".", pathPrefix, unitType),
			)
			return diags
		}
	default:
		return diags
	}

	unit := m.Unit.ValueString()
	switch unitType {
	case "duration":
		if !slices.Contains(durationUnits, unit) {
			diags.AddError(
				"Invalid normalizer configuration",
				fmt.Sprintf("%s.normalizer: unit %q is not valid for the \"duration\" normalizer; must be one of %v.", pathPrefix, unit, durationUnits),
			)
		}
	case "data":
		if !slices.Contains(dataUnits, unit) {
			diags.AddError(
				"Invalid normalizer configuration",
				fmt.Sprintf("%s.normalizer: unit %q is not valid for the \"data\" normalizer; must be one of %v.", pathPrefix, unit, dataUnits),
			)
		}
	case "custom":
		n := utf8.RuneCountInString(unit)
		if n < customUnitMinLength || n > customUnitMaxLength {
			diags.AddError(
				"Invalid normalizer configuration",
				fmt.Sprintf("%s.normalizer: the \"custom\" normalizer unit must be between %d and %d characters, got %d.", pathPrefix, customUnitMinLength, customUnitMaxLength, n),
			)
		}
	}
	return diags
}
