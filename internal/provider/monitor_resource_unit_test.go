package provider

import (
	"context"
	"terraform-provider-tsuga/internal/resource_monitor"
	"testing"

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
