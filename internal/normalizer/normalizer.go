package normalizer

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Schema returns the schema for a normalizer object.
func Schema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("duration", "data", "percent", "date", "level", "cpu", "custom"),
				},
			},
			"unit": schema.StringAttribute{
				Optional:    true,
				Description: "Unit label (required for duration, data, and custom normalizers; custom unit label limited to 20 characters)",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(20),
				},
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
	switch m.Type.ValueString() {
	case "duration", "data", "custom":
		if m.Unit.IsNull() || m.Unit.IsUnknown() || m.Unit.ValueString() == "" {
			diags.AddError(
				"Invalid normalizer configuration",
				fmt.Sprintf("%s.normalizer: the %q normalizer requires \"unit\".", pathPrefix, m.Type.ValueString()),
			)
		}
	}
	return diags
}
