package schematype_test

import (
	"context"
	"testing"

	"terraform-provider-tsuga/internal/resource_dashboard"
	"terraform-provider-tsuga/internal/resource_monitor"
	"terraform-provider-tsuga/internal/schematype"
)

func TestCachePreservesType(t *testing.T) {
	ctx := context.Background()
	d := resource_dashboard.DashboardResourceSchema(ctx)
	if !schematype.Cache(d).Type().Equal(d.Type()) {
		t.Fatal("dashboard: cached schema type differs from original")
	}
	m := resource_monitor.MonitorResourceSchema(ctx)
	if !schematype.Cache(m).Type().Equal(m.Type()) {
		t.Fatal("monitor: cached schema type differs from original")
	}
}
