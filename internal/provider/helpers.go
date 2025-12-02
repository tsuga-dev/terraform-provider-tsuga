package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stringValue converts a types.String to a Go string.
// Returns empty string if the value is null or unknown.
func stringValue(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

// stringValueOrNull converts a Go string to types.String.
// Returns types.StringNull() if the string is empty, otherwise types.StringValue().
func stringValueOrNull(v string) types.String {
	if v == "" {
		return types.StringNull()
	}
	return types.StringValue(v)
}

// expandStringList converts a types.List to a []string.
// Returns nil (not an empty slice) if the list is null or unknown.
func expandStringList(ctx context.Context, value types.List) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}

	var result []string
	diags.Append(value.ElementsAs(ctx, &result, false)...)
	return result, diags
}

// expandIntList converts a types.List to a []int64.
// Returns nil (not an empty slice) if the list is null or unknown.
func expandIntList(ctx context.Context, value types.List) ([]int64, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}

	var result []int64
	diags.Append(value.ElementsAs(ctx, &result, false)...)
	return result, diags
}

// expandBoolList converts a types.List to a []bool.
// Returns nil (not an empty slice) if the list is null or unknown.
func expandBoolList(ctx context.Context, list types.List) ([]bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}
	var vals []bool
	diags.Append(list.ElementsAs(ctx, &vals, false)...)
	return vals, diags
}
