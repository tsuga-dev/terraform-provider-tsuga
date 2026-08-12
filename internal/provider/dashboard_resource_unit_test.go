package provider

import (
	"context"
	"terraform-provider-tsuga/internal/aggregate"
	"terraform-provider-tsuga/internal/resource_dashboard"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// functionValue builds a single query-function object value, leaving every
// optional parameter null unless overridden by extra.
func functionValue(funcType string, extra map[string]attr.Value) attr.Value {
	attrs := map[string]attr.Value{
		"type":     types.StringValue(funcType),
		"window":   types.StringNull(),
		"seconds":  types.Int64Null(),
		"base":     types.Int64Null(),
		"exponent": types.Int64Null(),
	}
	for k, v := range extra {
		attrs[k] = v
	}
	return types.ObjectValueMust(resource_dashboard.FunctionAttrTypes(), attrs)
}

func TestExpandFunctions_SupportsLogPowerSqrtIncrease(t *testing.T) {
	functions := types.ListValueMust(
		types.ObjectType{AttrTypes: resource_dashboard.FunctionAttrTypes()},
		[]attr.Value{
			functionValue("increase", nil),
			functionValue("log", map[string]attr.Value{"base": types.Int64Value(10)}),
			functionValue("power", map[string]attr.Value{"exponent": types.Int64Value(2)}),
			functionValue("sqrt", nil),
		},
	)

	expanded, diags := expandFunctions(context.Background(), functions)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(expanded) != 4 {
		t.Fatalf("expected 4 functions, got %d", len(expanded))
	}

	if expanded[0].Type != "increase" {
		t.Fatalf("expected first function type to be increase, got %q", expanded[0].Type)
	}

	if expanded[1].Type != "log" {
		t.Fatalf("expected second function type to be log, got %q", expanded[1].Type)
	}
	if expanded[1].Base == nil || *expanded[1].Base != 10 {
		t.Fatalf("expected log base to be 10, got %#v", expanded[1].Base)
	}

	if expanded[2].Type != "power" {
		t.Fatalf("expected third function type to be power, got %q", expanded[2].Type)
	}
	if expanded[2].Exponent == nil || *expanded[2].Exponent != 2 {
		t.Fatalf("expected power exponent to be 2, got %#v", expanded[2].Exponent)
	}

	if expanded[3].Type != "sqrt" {
		t.Fatalf("expected fourth function type to be sqrt, got %q", expanded[3].Type)
	}
	// sqrt and increase carry no parameters.
	if expanded[3].Base != nil || expanded[3].Exponent != nil {
		t.Fatalf("expected sqrt to carry no base/exponent, got base=%#v exponent=%#v", expanded[3].Base, expanded[3].Exponent)
	}
}

func functionsList(values ...attr.Value) types.List {
	return types.ListValueMust(
		types.ObjectType{AttrTypes: resource_dashboard.FunctionAttrTypes()},
		values,
	)
}

func TestValidateQueryFunctions_LogRequiresBase(t *testing.T) {
	r := &dashboardResource{}

	// log without base is rejected.
	missing := functionsList(functionValue("log", nil))
	if diags := r.validateQueryFunctions(context.Background(), missing, "v"); !diags.HasError() {
		t.Fatalf("expected error when log function omits base")
	}

	// log with base passes.
	present := functionsList(functionValue("log", map[string]attr.Value{"base": types.Int64Value(10)}))
	if diags := r.validateQueryFunctions(context.Background(), present, "v"); diags.HasError() {
		t.Fatalf("unexpected error for log function with base: %v", diags)
	}
}

func TestValidateQueryFunctions_PowerRequiresExponent(t *testing.T) {
	r := &dashboardResource{}

	missing := functionsList(functionValue("power", nil))
	if diags := r.validateQueryFunctions(context.Background(), missing, "v"); !diags.HasError() {
		t.Fatalf("expected error when power function omits exponent")
	}

	present := functionsList(functionValue("power", map[string]attr.Value{"exponent": types.Int64Value(2)}))
	if diags := r.validateQueryFunctions(context.Background(), present, "v"); diags.HasError() {
		t.Fatalf("unexpected error for power function with exponent: %v", diags)
	}
}

func TestValidateQueryFunctions_ParameterlessFunctionsPass(t *testing.T) {
	r := &dashboardResource{}

	list := functionsList(
		functionValue("rate", nil),
		functionValue("sqrt", nil),
		functionValue("increase", nil),
	)
	if diags := r.validateQueryFunctions(context.Background(), list, "v"); diags.HasError() {
		t.Fatalf("unexpected error for parameterless functions: %v", diags)
	}
}

func TestFlattenFunctions_RoundTripsBaseAndExponent(t *testing.T) {
	base := int64(2)
	exponent := int64(3)
	funcs := []dashboardFunction{
		{Type: "log", Base: &base},
		{Type: "power", Exponent: &exponent},
	}

	list, diags := flattenFunctions(funcs)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var models []resource_dashboard.FunctionModel
	if diags := list.ElementsAs(context.Background(), &models, false); diags.HasError() {
		t.Fatalf("failed to decode flattened functions: %v", diags)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(models))
	}

	if models[0].Type.ValueString() != "log" {
		t.Fatalf("expected first function type to be log, got %q", models[0].Type.ValueString())
	}
	if models[0].Base.IsNull() || models[0].Base.ValueInt64() != 2 {
		t.Fatalf("expected log base to be 2, got %#v", models[0].Base)
	}
	// Parameters that do not apply to log must remain null.
	if !models[0].Exponent.IsNull() {
		t.Fatalf("expected log exponent to be null, got %#v", models[0].Exponent)
	}

	if models[1].Type.ValueString() != "power" {
		t.Fatalf("expected second function type to be power, got %q", models[1].Type.ValueString())
	}
	if models[1].Exponent.IsNull() || models[1].Exponent.ValueInt64() != 3 {
		t.Fatalf("expected power exponent to be 3, got %#v", models[1].Exponent)
	}
}

func TestExpandFlattenVisualization_ListSpansRoundTrips(t *testing.T) {
	ctx := context.Background()

	sortingType := types.ObjectType{AttrTypes: resource_dashboard.ListDefaultSortingAttrTypes()}
	sorting := types.ListValueMust(sortingType, []attr.Value{
		types.ObjectValueMust(resource_dashboard.ListDefaultSortingAttrTypes(), map[string]attr.Value{
			"id":   types.StringValue("duration"),
			"desc": types.BoolValue(true),
		}),
	})

	vis := resource_dashboard.VisualizationModel{
		ListSpans: &resource_dashboard.ListVisualization{
			Query:           types.StringValue("service:api"),
			ListColumns:     types.ListNull(types.ObjectType{AttrTypes: resource_dashboard.ListColumnAttrTypes()}),
			ListColumnsSize: types.MapNull(types.Float64Type),
			DefaultSorting:  sorting,
			IsCellWrapped:   types.BoolValue(true),
		},
	}

	expanded, diags := expandVisualization(ctx, vis)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if expanded.Type != "list-spans" {
		t.Fatalf("expected type list-spans, got %q", expanded.Type)
	}
	if expanded.IsCellWrapped == nil || !*expanded.IsCellWrapped {
		t.Fatalf("expected isCellWrapped to be sent as true, got %#v", expanded.IsCellWrapped)
	}
	if len(expanded.DefaultSorting) != 1 || expanded.DefaultSorting[0].Id != "duration" || !expanded.DefaultSorting[0].Desc {
		t.Fatalf("expected defaultSorting to be sent, got %#v", expanded.DefaultSorting)
	}

	flattened, flattenDiags := flattenVisualization(ctx, expanded)
	if flattenDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", flattenDiags)
	}

	var back resource_dashboard.VisualizationModel
	if d := flattened.(types.Object).As(ctx, &back, basetypes.ObjectAsOptions{}); d.HasError() {
		t.Fatalf("failed to decode flattened visualization: %v", d)
	}
	if back.ListSpans == nil {
		t.Fatal("expected list_spans visualization to be set")
	}
	if back.ListSpans.Query.ValueString() != "service:api" {
		t.Fatalf("expected query to round-trip, got %v", back.ListSpans.Query)
	}
	var sortBack []resource_dashboard.ListDefaultSortingModel
	if d := back.ListSpans.DefaultSorting.ElementsAs(ctx, &sortBack, false); d.HasError() {
		t.Fatalf("failed to decode default_sorting: %v", d)
	}
	if len(sortBack) != 1 || sortBack[0].Id.ValueString() != "duration" || !sortBack[0].Desc.ValueBool() {
		t.Fatalf("expected default_sorting to round-trip, got %#v", sortBack)
	}
}

func TestExpandFlattenQueries_TimeAggregateRoundTrips(t *testing.T) {
	ctx := context.Background()

	queries, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: resource_dashboard.QueryAttrTypes()},
		[]resource_dashboard.QueryModel{{
			Aggregate:     resource_dashboard.AggregateModel{Count: &aggregate.CountModel{Field: types.StringNull()}},
			Filter:        types.StringValue("service:api"),
			Functions:     types.ListNull(types.ObjectType{AttrTypes: resource_dashboard.FunctionAttrTypes()}),
			TimeAggregate: types.StringValue("last"),
		}})
	if diags.HasError() {
		t.Fatalf("failed to build queries: %v", diags)
	}

	expanded, expandDiags := expandQueries(ctx, queries)
	if expandDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", expandDiags)
	}
	if expanded[0].TimeAggregate != "last" {
		t.Fatalf("expected timeAggregate to be sent, got %q", expanded[0].TimeAggregate)
	}

	flattened, flattenDiags := flattenQueries(expanded)
	if flattenDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", flattenDiags)
	}
	var back []resource_dashboard.QueryModel
	if d := flattened.ElementsAs(ctx, &back, false); d.HasError() {
		t.Fatalf("failed to decode flattened queries: %v", d)
	}
	if back[0].TimeAggregate.ValueString() != "last" {
		t.Fatalf("expected time_aggregate to round-trip, got %v", back[0].TimeAggregate)
	}
}
