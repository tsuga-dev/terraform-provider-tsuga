package groupby

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// AttrTypes returns attr types for a group by object with fields, limit, sort order and
// null replacement.
func AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"fields":            types.ListType{ElemType: types.StringType},
		"limit":             types.Int64Type,
		"sort_order":        types.StringType,
		"replace_null_with": types.StringType,
	}
}

// Model represents a group by configuration.
type Model struct {
	Fields          types.List   `tfsdk:"fields"`
	Limit           types.Int64  `tfsdk:"limit"`
	SortOrder       types.String `tfsdk:"sort_order"`
	ReplaceNullWith types.String `tfsdk:"replace_null_with"`
}
