package provider

import (
	"context"

	"terraform-provider-tsuga/internal/resource_team"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type apiTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func expandTags(ctx context.Context, tags types.List) ([]apiTag, diag.Diagnostics) {
	var diags diag.Diagnostics

	if tags.IsNull() || tags.IsUnknown() {
		return nil, diags
	}

	var tagList []resource_team.TagsValue
	diags.Append(tags.ElementsAs(ctx, &tagList, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]apiTag, 0, len(tagList))
	for _, t := range tagList {
		result = append(result, apiTag{
			Key:   t.Key.ValueString(),
			Value: t.Value.ValueString(),
		})
	}

	return result, diags
}

func flattenTags(ctx context.Context, tags []apiTag) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)}

	if len(tags) == 0 {
		return types.ListNull(elemType), nil
	}

	values := make([]attr.Value, 0, len(tags))
	for _, t := range tags {
		values = append(values, types.ObjectValueMust(
			resource_team.TagsValue{}.AttributeTypes(ctx),
			map[string]attr.Value{
				"key":   types.StringValue(t.Key),
				"value": types.StringValue(t.Value),
			},
		))
	}

	return types.ListValue(elemType, values)
}
