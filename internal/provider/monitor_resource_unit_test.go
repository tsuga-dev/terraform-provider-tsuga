package provider

import (
	"context"
	"terraform-provider-tsuga/internal/resource_monitor"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFlattenMonitorConfigurationCertificateExpiry_EmptyCloudAccountsIsNull(t *testing.T) {
	warnBeforeInDays := 30.0
	config := monitorAPIConfiguration{
		Type:                  "certificate-expiry",
		WarnBeforeInDays:      &warnBeforeInDays,
		CloudAccounts:         []string{},
		AggregationAlertLogic: "each",
		NoDataBehavior:        "resolve",
	}

	flattened, diags := flattenMonitorConfiguration(context.Background(), config)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if flattened.CertificateExpiry == nil {
		t.Fatal("expected certificate_expiry configuration to be set")
	}

	if !flattened.CertificateExpiry.CloudAccounts.IsNull() {
		t.Fatalf("expected cloud_accounts to be null when API returns an empty list, got: %v", flattened.CertificateExpiry.CloudAccounts)
	}
}

func TestFlattenThresholdMonitorConfiguration_ConditionIsNull(t *testing.T) {
	// The Pulumi terraform-provider bridge singularizes list attributes,
	// inferring a "condition" field from "conditions". The flatten function
	// must set Condition to an explicit null object so the bridge doesn't
	// fail with a Value Conversion Error during refresh.
	// See: https://github.com/pulumi/pulumi-terraform-bridge/issues/3315
	threshold := 10.0
	config := monitorAPIConfiguration{
		Type: "log",
		Conditions: []monitorAPICondition{
			{Formula: "q1", Operator: "greater_than", Threshold: &threshold},
		},
		NoDataBehavior:        "resolve",
		Timeframe:             5,
		AggregationAlertLogic: "no_aggregation",
		Queries: []monitorAPIQuery{
			{Filter: "level:error", Aggregate: monitorAPIAggregate{Type: "count"}},
		},
	}

	flattened, diags := flattenMonitorConfiguration(context.Background(), config)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if flattened.Log == nil {
		t.Fatal("expected log configuration to be set")
	}

	if !flattened.Log.Condition.IsNull() {
		t.Fatalf("expected condition (singular) to be null for threshold monitors, got: %v", flattened.Log.Condition)
	}

	if flattened.Log.Conditions.IsNull() {
		t.Fatal("expected conditions (plural) to be set")
	}
}

func TestExpandMonitorConfigurationCertificateExpiry_OmitsEmptyCloudAccounts(t *testing.T) {
	emptyList, diags := types.ListValueFrom(context.Background(), types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("failed to build empty cloud_accounts list: %v", diags)
	}

	config := resource_monitor.CertificateExpiryMonitorConfigurationModel{
		WarnBeforeInDays:      types.Int64Value(15),
		CloudAccounts:         emptyList,
		AggregationAlertLogic: types.StringValue("each"),
		NoDataBehavior:        types.StringValue("resolve"),
	}

	expanded, expandDiags := expandMonitorConfigurationCertificateExpiry(context.Background(), &config)
	if expandDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", expandDiags)
	}

	if _, exists := expanded["cloudAccounts"]; exists {
		t.Fatalf("expected cloudAccounts to be omitted for empty cloud_accounts, got: %#v", expanded["cloudAccounts"])
	}
}

func TestExpandAggregationFunctions_SupportsLastAndTimeOffset(t *testing.T) {
	functions, diags := types.ListValue(
		types.ObjectType{AttrTypes: resource_monitor.AggregationFunctionAttrTypes()},
		[]attr.Value{
			types.ObjectValueMust(resource_monitor.AggregationFunctionAttrTypes(), map[string]attr.Value{
				"per_second":  types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"per_minute":  types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"per_hour":    types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"rate":        types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"increase":    types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"last":        types.ObjectValueMust(resource_monitor.AggregationFunctionEmptyAttrTypes(), map[string]attr.Value{}),
				"rolling":     types.ObjectNull(resource_monitor.AggregationFunctionRollingAttrTypes()),
				"time_offset": types.ObjectNull(resource_monitor.AggregationFunctionTimeOffsetAttrTypes()),
			}),
			types.ObjectValueMust(resource_monitor.AggregationFunctionAttrTypes(), map[string]attr.Value{
				"per_second": types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"per_minute": types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"per_hour":   types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"rate":       types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"increase":   types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"last":       types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes()),
				"rolling":    types.ObjectNull(resource_monitor.AggregationFunctionRollingAttrTypes()),
				"time_offset": types.ObjectValueMust(resource_monitor.AggregationFunctionTimeOffsetAttrTypes(), map[string]attr.Value{
					"seconds": types.Int64Value(3600),
				}),
			}),
		},
	)
	if diags.HasError() {
		t.Fatalf("failed to build functions list: %v", diags)
	}

	expanded, expandDiags := expandAggregationFunctions(context.Background(), functions)
	if expandDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", expandDiags)
	}

	if len(expanded) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(expanded))
	}
	if expanded[0].Type != "last" {
		t.Fatalf("expected first function type to be last, got %q", expanded[0].Type)
	}
	if expanded[1].Type != "time-offset" {
		t.Fatalf("expected second function type to be time-offset, got %q", expanded[1].Type)
	}
	if expanded[1].Seconds == nil || *expanded[1].Seconds != 3600 {
		t.Fatalf("expected second function seconds to be 3600, got %#v", expanded[1].Seconds)
	}
}

func TestFlattenAggregationFunctions_SupportsLastAndTimeOffset(t *testing.T) {
	seconds := int64(1800)
	functions := []monitorAPIFunction{
		{Type: "last"},
		{Type: "time-offset", Seconds: &seconds},
	}

	flattened, diags := flattenAggregationFunctions(functions)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var models []resource_monitor.AggregationFunctionModel
	diags.Append(flattened.ElementsAs(context.Background(), &models, false)...)
	if diags.HasError() {
		t.Fatalf("failed to decode flattened functions: %v", diags)
	}

	if len(models) != 2 {
		t.Fatalf("expected 2 function models, got %d", len(models))
	}
	if models[0].Last == nil {
		t.Fatal("expected first function model to have last set")
	}
	if models[1].TimeOffset == nil {
		t.Fatal("expected second function model to have time_offset set")
	}
	if models[1].TimeOffset.Seconds.IsNull() || models[1].TimeOffset.Seconds.ValueInt64() != 1800 {
		t.Fatalf("expected second function time_offset.seconds to be 1800, got %v", models[1].TimeOffset.Seconds)
	}
}
