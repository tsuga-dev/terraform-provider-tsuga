package provider

import (
	"context"
	"terraform-provider-tsuga/internal/resource_metric_route"
	"terraform-provider-tsuga/internal/resource_team"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandMetricRouteCreatorParams_Category(t *testing.T) {
	creator := &resource_metric_route.CreatorModel{
		Category: &resource_metric_route.CreatorCategoryModel{
			TargetAttribute: types.StringValue("severity"),
			Clauses: []resource_metric_route.CreatorCategoryClauseModel{
				{Query: types.StringValue("status_code:>=500"), Value: types.StringValue("error")},
				{Query: types.StringValue("status_code:>=400"), Value: types.StringValue("warning")},
			},
			DefaultValue: types.StringValue("info"),
		},
	}

	params, diags := expandMetricRouteCreatorParams(creator)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if params["subtype"] != "category" {
		t.Fatalf("expected subtype category, got %#v", params["subtype"])
	}
	if params["targetAttribute"] != "severity" {
		t.Fatalf("expected targetAttribute severity, got %#v", params["targetAttribute"])
	}
	if params["defaultValue"] != "info" {
		t.Fatalf("expected defaultValue info, got %#v", params["defaultValue"])
	}

	clauses, ok := params["clauses"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected clauses to be []map[string]interface{}, got %T", params["clauses"])
	}
	if len(clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(clauses))
	}
	if clauses[0]["query"] != "status_code:>=500" || clauses[0]["value"] != "error" {
		t.Fatalf("unexpected first clause: %#v", clauses[0])
	}
}

func TestExpandMetricRouteCreatorParams_CategoryOmitsUnsetDefaultValue(t *testing.T) {
	creator := &resource_metric_route.CreatorModel{
		Category: &resource_metric_route.CreatorCategoryModel{
			TargetAttribute: types.StringValue("severity"),
			Clauses: []resource_metric_route.CreatorCategoryClauseModel{
				{Query: types.StringValue("status_code:>=500"), Value: types.StringValue("error")},
			},
			DefaultValue: types.StringNull(),
		},
	}

	params, diags := expandMetricRouteCreatorParams(creator)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if _, present := params["defaultValue"]; present {
		t.Fatalf("expected defaultValue to be omitted when null, got %#v", params["defaultValue"])
	}
}

func TestExpandMetricRouteCreatorParams_FormatString(t *testing.T) {
	creator := &resource_metric_route.CreatorModel{
		FormatString: &resource_metric_route.CreatorFormatStringModel{
			TargetAttribute:       types.StringValue("service.name"),
			FormatString:          types.StringValue("{{svc}}"),
			OverrideTarget:        types.BoolValue(true),
			ReplaceMissingByEmpty: types.BoolValue(false),
		},
	}

	params, diags := expandMetricRouteCreatorParams(creator)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if params["subtype"] != "format-string" {
		t.Fatalf("expected subtype format-string, got %#v", params["subtype"])
	}
	if params["formatString"] != "{{svc}}" {
		t.Fatalf("expected formatString {{svc}}, got %#v", params["formatString"])
	}
}

func TestExpandMetricRouteParseAttributeParams_Grok(t *testing.T) {
	ctx := context.Background()
	rules, _ := types.ListValueFrom(ctx, types.StringType, []string{"%{GREEDYDATA:message}"})
	samples, _ := types.ListValueFrom(ctx, types.StringType, []string{"example line"})

	parseAttr := &resource_metric_route.ParseAttributeModel{
		Grok: &resource_metric_route.ParseGrokModel{
			AttributeName: types.StringValue("message"),
			Rules:         rules,
			Samples:       samples,
		},
	}

	params, diags := expandMetricRouteParseAttributeParams(ctx, parseAttr)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if params["subtype"] != "grok" {
		t.Fatalf("expected subtype grok, got %#v", params["subtype"])
	}
	if params["attributeName"] != "message" {
		t.Fatalf("expected attributeName message, got %#v", params["attributeName"])
	}
	gotRules, ok := params["rules"].([]string)
	if !ok || len(gotRules) != 1 || gotRules[0] != "%{GREEDYDATA:message}" {
		t.Fatalf("unexpected rules: %#v", params["rules"])
	}
}

func TestExpandMetricRouteProcessors_TypeAndParamsSubtype(t *testing.T) {
	ctx := context.Background()
	nullTags := types.ListNull(types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)})

	mapperProc := resource_metric_route.ProcessorModel{
		Id:   types.StringValue("mapper-1"),
		Tags: nullTags,
		Mapper: &resource_metric_route.MapperModel{
			MapAttributes: []resource_metric_route.MapAttributeModel{
				{
					OriginAttribute: types.StringValue("svc"),
					TargetAttribute: types.StringValue("service.name"),
					KeepOrigin:      types.BoolValue(false),
					OverrideTarget:  types.BoolValue(true),
				},
			},
		},
	}
	grokProc := resource_metric_route.ProcessorModel{
		Id:   types.StringValue("grok-1"),
		Tags: nullTags,
		ParseAttribute: &resource_metric_route.ParseAttributeModel{
			Grok: &resource_metric_route.ParseGrokModel{
				AttributeName: types.StringValue("message"),
				Rules:         mustStringList(ctx, t, []string{"%{GREEDYDATA:message}"}),
				Samples:       types.ListNull(types.StringType),
			},
		},
	}
	categoryProc := resource_metric_route.ProcessorModel{
		Id:   types.StringValue("category-1"),
		Tags: nullTags,
		Creator: &resource_metric_route.CreatorModel{
			Category: &resource_metric_route.CreatorCategoryModel{
				TargetAttribute: types.StringValue("severity"),
				Clauses: []resource_metric_route.CreatorCategoryClauseModel{
					{Query: types.StringValue("status_code:>=500"), Value: types.StringValue("error")},
				},
				DefaultValue: types.StringNull(),
			},
		},
	}

	elemType := types.ObjectType{AttrTypes: resource_metric_route.ProcessorAttrTypes(ctx)}
	list, diags := types.ListValueFrom(ctx, elemType, []resource_metric_route.ProcessorModel{mapperProc, grokProc, categoryProc})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics building list: %v", diags)
	}

	procs, diags := expandMetricRouteProcessors(ctx, list)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics expanding processors: %v", diags)
	}
	if len(procs) != 3 {
		t.Fatalf("expected 3 processors, got %d", len(procs))
	}

	if procs[0].Type != "mapper" || procs[0].Params["subtype"] != "map-attributes" {
		t.Fatalf("unexpected mapper processor: type=%q params=%#v", procs[0].Type, procs[0].Params)
	}
	if procs[1].Type != "parse-attribute" || procs[1].Params["subtype"] != "grok" {
		t.Fatalf("unexpected parse-attribute processor: type=%q params=%#v", procs[1].Type, procs[1].Params)
	}
	if procs[2].Type != "creator" || procs[2].Params["subtype"] != "category" {
		t.Fatalf("unexpected creator processor: type=%q params=%#v", procs[2].Type, procs[2].Params)
	}
}

func TestValidateMetricRouteProcessors_CreatorFormatStringAndCategoryBothSet(t *testing.T) {
	ctx := context.Background()
	nullTags := types.ListNull(types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)})

	proc := resource_metric_route.ProcessorModel{
		Id:   types.StringValue("bad-creator"),
		Tags: nullTags,
		Creator: &resource_metric_route.CreatorModel{
			FormatString: &resource_metric_route.CreatorFormatStringModel{
				TargetAttribute:       types.StringValue("unit"),
				FormatString:          types.StringValue("{{svc}}"),
				OverrideTarget:        types.BoolValue(true),
				ReplaceMissingByEmpty: types.BoolValue(false),
			},
			Category: &resource_metric_route.CreatorCategoryModel{
				TargetAttribute: types.StringValue("severity"),
				Clauses: []resource_metric_route.CreatorCategoryClauseModel{
					{Query: types.StringValue("status_code:>=500"), Value: types.StringValue("error")},
				},
				DefaultValue: types.StringNull(),
			},
		},
	}

	elemType := types.ObjectType{AttrTypes: resource_metric_route.ProcessorAttrTypes(ctx)}
	list, diags := types.ListValueFrom(ctx, elemType, []resource_metric_route.ProcessorModel{proc})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics building list: %v", diags)
	}

	r := &metricRouteResource{}
	diags = r.validateProcessors(ctx, list, "processors")
	if !diags.HasError() {
		t.Fatalf("expected error when both format_string and category are set")
	}
}

func TestValidateMetricRouteProcessors_CreatorNeitherFormatStringNorCategorySet(t *testing.T) {
	ctx := context.Background()
	nullTags := types.ListNull(types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)})

	proc := resource_metric_route.ProcessorModel{
		Id:      types.StringValue("empty-creator"),
		Tags:    nullTags,
		Creator: &resource_metric_route.CreatorModel{},
	}

	elemType := types.ObjectType{AttrTypes: resource_metric_route.ProcessorAttrTypes(ctx)}
	list, diags := types.ListValueFrom(ctx, elemType, []resource_metric_route.ProcessorModel{proc})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics building list: %v", diags)
	}

	r := &metricRouteResource{}
	diags = r.validateProcessors(ctx, list, "processors")
	if !diags.HasError() {
		t.Fatalf("expected error when neither format_string nor category are set")
	}
}

func mustStringList(ctx context.Context, t *testing.T, values []string) types.List {
	t.Helper()
	list, diags := types.ListValueFrom(ctx, types.StringType, values)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics building string list: %v", diags)
	}
	return list
}

func TestFlattenMetricRouteCreator_CategoryRoundTrip(t *testing.T) {
	params := map[string]interface{}{
		"subtype":         "category",
		"targetAttribute": "severity",
		"clauses": []interface{}{
			map[string]interface{}{"query": "status_code:>=500", "value": "error"},
		},
		"defaultValue": "info",
	}

	val := flattenMetricRouteCreator(params)
	obj, ok := val.(types.Object)
	if !ok {
		t.Fatalf("expected types.Object, got %T", val)
	}
	attrs := obj.Attributes()
	category, ok := attrs["category"].(types.Object)
	if !ok || category.IsNull() {
		t.Fatalf("expected non-null category object, got %#v", attrs["category"])
	}
	categoryAttrs := category.Attributes()
	if categoryAttrs["target_attribute"].(types.String).ValueString() != "severity" {
		t.Fatalf("unexpected target_attribute: %#v", categoryAttrs["target_attribute"])
	}
	if categoryAttrs["default_value"].(types.String).ValueString() != "info" {
		t.Fatalf("unexpected default_value: %#v", categoryAttrs["default_value"])
	}

	formatString, ok := attrs["format_string"].(types.Object)
	if !ok || !formatString.IsNull() {
		t.Fatalf("expected null format_string object, got %#v", attrs["format_string"])
	}
}

func TestFlattenMetricRouteParseAttribute_GrokRoundTrip(t *testing.T) {
	ctx := context.Background()
	params := map[string]interface{}{
		"subtype":       "grok",
		"attributeName": "message",
		"rules":         []interface{}{"%{GREEDYDATA:message}"},
		"samples":       []interface{}{"example"},
	}

	val := flattenMetricRouteParseAttribute(ctx, params)
	obj, ok := val.(types.Object)
	if !ok {
		t.Fatalf("expected types.Object, got %T", val)
	}
	grok, ok := obj.Attributes()["grok"].(types.Object)
	if !ok || grok.IsNull() {
		t.Fatalf("expected non-null grok object, got %#v", obj.Attributes()["grok"])
	}
	if grok.Attributes()["attribute_name"].(types.String).ValueString() != "message" {
		t.Fatalf("unexpected attribute_name: %#v", grok.Attributes()["attribute_name"])
	}
}
