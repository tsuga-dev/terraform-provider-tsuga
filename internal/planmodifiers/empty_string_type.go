package planmodifiers

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// EmptyEqualsNullStringType is a custom string type that treats empty strings
// and null as semantically equivalent. This prevents Terraform from reporting
// inconsistencies when the API returns "" but the state has null (or vice versa).
type EmptyEqualsNullStringType struct {
	basetypes.StringType
}

func (t EmptyEqualsNullStringType) Equal(o attr.Type) bool {
	// Accept both our custom type and the base StringType as equal
	switch o.(type) {
	case EmptyEqualsNullStringType, basetypes.StringType:
		return true
	default:
		return false
	}
}

func (t EmptyEqualsNullStringType) String() string {
	return "EmptyEqualsNullStringType"
}

func (t EmptyEqualsNullStringType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return EmptyEqualsNullStringValue{StringValue: in}, nil
}

func (t EmptyEqualsNullStringType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}
	return EmptyEqualsNullStringValue{StringValue: stringValue}, nil
}

func (t EmptyEqualsNullStringType) ValueType(_ context.Context) attr.Value {
	return EmptyEqualsNullStringValue{}
}

// EmptyEqualsNullStringValue is a custom string value that treats empty strings
// and null as semantically equivalent.
type EmptyEqualsNullStringValue struct {
	basetypes.StringValue
}

func (v EmptyEqualsNullStringValue) Type(_ context.Context) attr.Type {
	return EmptyEqualsNullStringType{}
}

func (v EmptyEqualsNullStringValue) Equal(o attr.Value) bool {
	var other EmptyEqualsNullStringValue

	switch val := o.(type) {
	case EmptyEqualsNullStringValue:
		other = val
	case basetypes.StringValue:
		other = EmptyEqualsNullStringValue{StringValue: val}
	default:
		return false
	}

	// Treat null and empty string as equal
	vIsEmpty := v.IsNull() || v.ValueString() == ""
	oIsEmpty := other.IsNull() || other.ValueString() == ""

	if vIsEmpty && oIsEmpty {
		return true
	}
	if vIsEmpty != oIsEmpty {
		return false
	}

	// Both are non-empty, compare the actual values
	return v.ValueString() == other.ValueString()
}

func (v EmptyEqualsNullStringValue) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	var newValue EmptyEqualsNullStringValue

	switch val := newValuable.(type) {
	case EmptyEqualsNullStringValue:
		newValue = val
	case basetypes.StringValue:
		newValue = EmptyEqualsNullStringValue{StringValue: val}
	default:
		diags.AddError(
			"Semantic Equality Check Error",
			fmt.Sprintf("Expected string value type, got: %T", newValuable),
		)
		return false, diags
	}

	// Treat null and empty string as equal
	vIsEmpty := v.IsNull() || v.ValueString() == ""
	newIsEmpty := newValue.IsNull() || newValue.ValueString() == ""

	if vIsEmpty && newIsEmpty {
		return true, diags
	}
	if vIsEmpty != newIsEmpty {
		return false, diags
	}

	// Both are non-empty, compare the actual values
	return v.ValueString() == newValue.ValueString(), diags
}
