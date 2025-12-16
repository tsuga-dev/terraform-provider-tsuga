package normalizer

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
					stringvalidator.OneOf("duration", "data", "custom", "date", "level"),
				},
			},
			"unit": schema.StringAttribute{
				Optional: true,
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
