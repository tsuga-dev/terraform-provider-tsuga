package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-tsuga/internal/resource_monitor"
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
