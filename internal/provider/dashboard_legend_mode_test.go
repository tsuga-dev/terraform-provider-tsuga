package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-tsuga/internal/aggregate"
	"terraform-provider-tsuga/internal/groupby"
	"terraform-provider-tsuga/internal/normalizer"
	"terraform-provider-tsuga/internal/resource_dashboard"
)

// buildMinimalSeriesBase returns a SeriesBase with required fields populated
// and all optional fields null, suitable for unit-testing expand/flatten paths.
func buildMinimalSeriesBase(legendMode string) resource_dashboard.SeriesBase {
	// Construct a minimal queries list with one count aggregate.
	countNull := types.ObjectNull(aggregate.CountAttrTypes())
	fieldNull := types.ObjectNull(aggregate.FieldAttrTypes())
	percNull := types.ObjectNull(aggregate.PercentileAttrTypes())

	aggVal := types.ObjectValueMust(aggregate.AttrTypes(), map[string]attr.Value{
		"count":        types.ObjectValueMust(aggregate.CountAttrTypes(), map[string]attr.Value{
			"field": types.StringNull(),
		}),
		"sum":          fieldNull,
		"average":      fieldNull,
		"min":          fieldNull,
		"max":          fieldNull,
		"unique_count": fieldNull,
		"percentile":   percNull,
	})
	_ = countNull

	queryVal := types.ObjectValueMust(resource_dashboard.QueryAttrTypes(), map[string]attr.Value{
		"aggregate": aggVal,
		"filter":    types.StringNull(),
		"functions": types.ListNull(types.ObjectType{AttrTypes: resource_dashboard.FunctionAttrTypes()}),
	})

	queriesList := types.ListValueMust(
		types.ObjectType{AttrTypes: resource_dashboard.QueryAttrTypes()},
		[]attr.Value{queryVal},
	)

	groupByList := types.ListNull(types.ObjectType{AttrTypes: groupby.AttrTypes()})
	normalizerObj := types.ObjectNull(normalizer.AttrTypes())

	base := resource_dashboard.SeriesBase{
		Type:          types.StringNull(),
		Source:        types.StringValue("metrics"),
		Queries:       queriesList,
		Formula:       types.StringNull(),
		Aliases:       nil,
		VisibleSeries: types.ListNull(types.BoolType),
		Normalizer:    nil,
		Precision:     types.Float64Null(),
	}
	_ = groupByList
	_ = normalizerObj

	if legendMode != "" {
		base.LegendMode = types.StringValue(legendMode)
	} else {
		base.LegendMode = types.StringNull()
	}
	return base
}

// buildMinimalTimeseriesVisualization wraps SeriesBase in a full VisualizationModel
// with a timeseries block and all other visualization fields null.
func buildMinimalTimeseriesVisualization(legendMode string) resource_dashboard.VisualizationModel {
	base := buildMinimalSeriesBase(legendMode)

	groupByList := types.ListNull(types.ObjectType{AttrTypes: groupby.AttrTypes()})
	yAxisNull := (*resource_dashboard.YAxisSettingsModel)(nil)

	ts := &resource_dashboard.TimeseriesVisualization{
		SeriesVisualizationModel: resource_dashboard.SeriesVisualizationModel{
			SeriesBase:    base,
			GroupBy:       groupByList,
			YAxisSettings: yAxisNull,
		},
		Smoothing: types.BoolNull(),
	}

	return resource_dashboard.VisualizationModel{
		Timeseries:           ts,
		TopList:              nil,
		Pie:                  nil,
		QueryValue:           nil,
		Bar:                  nil,
		Gauge:                nil,
		Distribution:         nil,
		Heatmap:              nil,
		List:                 nil,
		ListLogPatterns:      nil,
		Note:                 nil,
		Table:                nil,
		TimeseriesConnection: nil,
		ListConnection:       nil,
		TopListConnection:    nil,
		PieConnection:        nil,
		BarConnection:        nil,
		QueryValueConnection: nil,
	}
}

// TestExpandTimeseries_LegendModeConfigured verifies that when legend_mode is set
// on an ordinary timeseries visualization, expandVisualization propagates the value
// to the dashboardVisualization payload (which is later JSON-marshaled to the API).
func TestExpandTimeseries_LegendModeConfigured(t *testing.T) {
	vis := buildMinimalTimeseriesVisualization("legend-only")
	got, diags := expandVisualization(context.Background(), vis)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got.Type != "timeseries" {
		t.Fatalf("expected type 'timeseries', got %q", got.Type)
	}
	if got.LegendMode != "legend-only" {
		t.Fatalf("expected LegendMode 'legend-only', got %q", got.LegendMode)
	}
}

// TestExpandTimeseries_LegendModeAbsent verifies that when legend_mode is null (not
// configured), expandVisualization leaves LegendMode as the empty string so the field
// is omitted from the JSON payload (json:"legendMode,omitempty").
func TestExpandTimeseries_LegendModeAbsent(t *testing.T) {
	vis := buildMinimalTimeseriesVisualization("")
	got, diags := expandVisualization(context.Background(), vis)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got.LegendMode != "" {
		t.Fatalf("expected empty LegendMode for null input, got %q", got.LegendMode)
	}
}

// TestFlattenTimeseries_LegendModeSet verifies that when the API response carries
// legendMode, flattenSeriesVisualization surfaces it as a non-null string in state.
func TestFlattenTimeseries_LegendModeSet(t *testing.T) {
	apiVis := dashboardVisualization{
		Type:       "timeseries",
		Source:     "metrics",
		LegendMode: "no-legend",
	}

	val, diags := flattenSeriesVisualization(context.Background(), apiVis)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	obj, ok := val.(types.Object)
	if !ok {
		t.Fatalf("expected types.Object, got %T", val)
	}

	legendModeAttr, exists := obj.Attributes()["legend_mode"]
	if !exists {
		t.Fatal("legend_mode key missing from flattened timeseries object")
	}

	legendModeStr, ok := legendModeAttr.(types.String)
	if !ok {
		t.Fatalf("expected types.String for legend_mode, got %T", legendModeAttr)
	}

	if legendModeStr.IsNull() {
		t.Fatal("expected legend_mode to be non-null, got null")
	}
	if legendModeStr.ValueString() != "no-legend" {
		t.Fatalf("expected legend_mode 'no-legend', got %q", legendModeStr.ValueString())
	}
}

// TestFlattenTimeseries_LegendModeAbsent verifies backward compatibility: when the
// API response omits legendMode (empty string), flattenSeriesVisualization stores
// types.StringNull() so existing configurations without legend_mode remain stable.
func TestFlattenTimeseries_LegendModeAbsent(t *testing.T) {
	apiVis := dashboardVisualization{
		Type:   "timeseries",
		Source: "metrics",
		// LegendMode intentionally omitted (zero value "").
	}

	val, diags := flattenSeriesVisualization(context.Background(), apiVis)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	obj, ok := val.(types.Object)
	if !ok {
		t.Fatalf("expected types.Object, got %T", val)
	}

	legendModeAttr, exists := obj.Attributes()["legend_mode"]
	if !exists {
		t.Fatal("legend_mode key missing from flattened timeseries object")
	}

	legendModeStr, ok := legendModeAttr.(types.String)
	if !ok {
		t.Fatalf("expected types.String for legend_mode, got %T", legendModeAttr)
	}

	if !legendModeStr.IsNull() {
		t.Fatalf("expected legend_mode null for absent API field, got %q", legendModeStr.ValueString())
	}
}

// TestFlattenTimeseries_LegendModeRoundTrip verifies the full round-trip: a configured
// legend_mode expands to the correct API payload, and flattening that payload
// restores the original string value.
func TestFlattenTimeseries_LegendModeRoundTrip(t *testing.T) {
	for _, mode := range []string{"table", "legend-only", "no-legend"} {
		t.Run(mode, func(t *testing.T) {
			vis := buildMinimalTimeseriesVisualization(mode)
			expanded, diags := expandVisualization(context.Background(), vis)
			if diags.HasError() {
				t.Fatalf("expand failed: %v", diags)
			}
			if expanded.LegendMode != mode {
				t.Fatalf("expand: expected %q, got %q", mode, expanded.LegendMode)
			}

			flatVal, diags := flattenSeriesVisualization(context.Background(), expanded)
			if diags.HasError() {
				t.Fatalf("flatten failed: %v", diags)
			}

			obj := flatVal.(types.Object)
			legendModeStr := obj.Attributes()["legend_mode"].(types.String)
			if legendModeStr.IsNull() || legendModeStr.ValueString() != mode {
				t.Fatalf("round-trip: expected %q, got %q (null=%v)", mode, legendModeStr.ValueString(), legendModeStr.IsNull())
			}
		})
	}
}
