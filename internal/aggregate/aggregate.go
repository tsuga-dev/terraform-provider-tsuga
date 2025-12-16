package aggregate

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Schema returns a schema for aggregate types (count, unique_count, average, max, min, sum, or percentile).
func Schema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required:    true,
		Description: "Aggregate (count, unique_count, average, max, min, sum, or percentile)",
		Attributes:  Attributes(),
	}
}

// Attributes returns the map of aggregate type attributes.
func Attributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"count":        CountSchema(),
		"sum":          FieldSchema(),
		"average":      FieldSchema(),
		"min":          FieldSchema(),
		"max":          FieldSchema(),
		"unique_count": FieldSchema(),
		"percentile":   PercentileSchema(),
	}
}

// CountSchema returns the schema for the count aggregate type.
func CountSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:   true,
		Attributes: map[string]schema.Attribute{},
	}
}

// FieldSchema returns the schema for field-based aggregates (sum, average, min, max, unique_count).
func FieldSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"field": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
		},
	}
}

// PercentileSchema returns the schema for the percentile aggregate type.
func PercentileSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"field": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"percentile": schema.Float64Attribute{
				Required: true,
			},
		},
	}
}

// AttrTypes returns attr types for the aggregate object.
func AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"count":        types.ObjectType{AttrTypes: CountAttrTypes()},
		"sum":          types.ObjectType{AttrTypes: FieldAttrTypes()},
		"average":      types.ObjectType{AttrTypes: FieldAttrTypes()},
		"min":          types.ObjectType{AttrTypes: FieldAttrTypes()},
		"max":          types.ObjectType{AttrTypes: FieldAttrTypes()},
		"unique_count": types.ObjectType{AttrTypes: FieldAttrTypes()},
		"percentile":   types.ObjectType{AttrTypes: PercentileAttrTypes()},
	}
}

// CountAttrTypes returns attr types for the count aggregate.
func CountAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{}
}

// FieldAttrTypes returns attr types for field-based aggregates.
func FieldAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.StringType,
	}
}

// PercentileAttrTypes returns attr types for the percentile aggregate.
func PercentileAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"field":      types.StringType,
		"percentile": types.Float64Type,
	}
}

// Model types for aggregates.

// FieldModel represents a field-based aggregate.
type FieldModel struct {
	Field types.String `tfsdk:"field"`
}

// PercentileModel represents a percentile aggregate.
type PercentileModel struct {
	Field      types.String  `tfsdk:"field"`
	Percentile types.Float64 `tfsdk:"percentile"`
}

// CountModel represents a count aggregate.
type CountModel struct{}
