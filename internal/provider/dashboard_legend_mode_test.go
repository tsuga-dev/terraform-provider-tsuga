package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-tsuga/internal/aggregate"
	"terraform-provider-tsuga/internal/groupby"
	"terraform-provider-tsuga/internal/resource_dashboard"
)

// minimalQueriesList builds a one-element queries list using a count aggregate,
// the minimum required to produce a valid expand input.
func minimalQueriesList() types.List {
	aggVal := types.ObjectValueMust(aggregate.AttrTypes(), map[string]attr.Value{
		"count": types.ObjectValueMust(aggregate.CountAttrTypes(), map[string]attr.Value{
			"field": types.StringNull(),
		}),
		"sum":          types.ObjectNull(aggregate.FieldAttrTypes()),
		"average":      types.ObjectNull(aggregate.FieldAttrTypes()),
		"min":          types.ObjectNull(aggregate.FieldAttrTypes()),
		"max":          types.ObjectNull(aggregate.FieldAttrTypes()),
		"unique_count": types.ObjectNull(aggregate.FieldAttrTypes()),
		"percentile":   types.ObjectNull(aggregate.PercentileAttrTypes()),
	})
	queryVal := types.ObjectValueMust(resource_dashboard.QueryAttrTypes(), map[string]attr.Value{
		"aggregate": aggVal,
		"filter":    types.StringNull(),
		"functions": types.ListNull(types.ObjectType{AttrTypes: resource_dashboard.FunctionAttrTypes()}),
	})
	return types.ListValueMust(
		types.ObjectType{AttrTypes: resource_dashboard.QueryAttrTypes()},
		[]attr.Value{queryVal},
	)
}

func minimalSeriesBase() resource_dashboard.SeriesBase {
	return resource_dashboard.SeriesBase{
		Type:          types.StringNull(),
		Source:        types.StringValue("metrics"),
		Queries:       minimalQueriesList(),
		Formula:       types.StringNull(),
		Aliases:       nil,
		VisibleSeries: types.ListNull(types.BoolType),
		Normalizer:    nil,
		Precision:     types.Float64Null(),
	}
}

// visByType builds a VisualizationModel with exactly one series type populated.
// legendMode is the value to set ("" means null/unset).
type buildVisFunc func(legendMode string) resource_dashboard.VisualizationModel

var legendModeVisBuilders = map[string]buildVisFunc{
	"timeseries": func(legendMode string) resource_dashboard.VisualizationModel {
		lm := types.StringNull()
		if legendMode != "" {
			lm = types.StringValue(legendMode)
		}
		return resource_dashboard.VisualizationModel{
			Timeseries: &resource_dashboard.TimeseriesVisualization{
				SeriesVisualizationModel: resource_dashboard.SeriesVisualizationModel{
					SeriesBase:    minimalSeriesBase(),
					GroupBy:       types.ListNull(types.ObjectType{AttrTypes: groupby.AttrTypes()}),
					YAxisSettings: nil,
					LegendMode:    lm,
				},
				Smoothing: types.BoolNull(),
			},
		}
	},
	"pie": func(legendMode string) resource_dashboard.VisualizationModel {
		lm := types.StringNull()
		if legendMode != "" {
			lm = types.StringValue(legendMode)
		}
		return resource_dashboard.VisualizationModel{
			Pie: &resource_dashboard.PieVisualization{
				SeriesBase: minimalSeriesBase(),
				GroupBy:    types.ListNull(types.ObjectType{AttrTypes: groupby.AttrTypes()}),
				LegendMode: lm,
			},
		}
	},
	"query_value": func(legendMode string) resource_dashboard.VisualizationModel {
		lm := types.StringNull()
		if legendMode != "" {
			lm = types.StringValue(legendMode)
		}
		return resource_dashboard.VisualizationModel{
			QueryValue: &resource_dashboard.QueryValueVisualization{
				SeriesBase:     minimalSeriesBase(),
				BackgroundMode: types.StringNull(),
				Conditions:     types.ListNull(types.ObjectType{AttrTypes: resource_dashboard.ConditionAttrTypes()}),
				LegendMode:     lm,
			},
		}
	},
	"bar": func(legendMode string) resource_dashboard.VisualizationModel {
		lm := types.StringNull()
		if legendMode != "" {
			lm = types.StringValue(legendMode)
		}
		return resource_dashboard.VisualizationModel{
			Bar: &resource_dashboard.BarVisualization{
				SeriesVisualizationModel: resource_dashboard.SeriesVisualizationModel{
					SeriesBase:    minimalSeriesBase(),
					GroupBy:       types.ListNull(types.ObjectType{AttrTypes: groupby.AttrTypes()}),
					YAxisSettings: nil,
					LegendMode:    lm,
				},
				TimeBucket: nil,
			},
		}
	},
}

// apiTypeByTFType maps Terraform attribute names to the API type string used
// in dashboardVisualization.
var apiTypeByTFType = map[string]string{
	"timeseries":  "timeseries",
	"pie":         "pie",
	"query_value": "query-value",
	"bar":         "bar",
}

// TestExpandSeriesLegendMode_Configured verifies that a non-null legend_mode on
// any of the four supported series types reaches the API payload as LegendMode.
func TestExpandSeriesLegendMode_Configured(t *testing.T) {
	for _, mode := range []string{"table", "legend-only", "no-legend"} {
		for tfType, buildVis := range legendModeVisBuilders {
			t.Run(fmt.Sprintf("%s/%s", tfType, mode), func(t *testing.T) {
				vis := buildVis(mode)
				got, diags := expandVisualization(context.Background(), vis)
				if diags.HasError() {
					t.Fatalf("expandVisualization failed: %v", diags)
				}
				if got.LegendMode != mode {
					t.Fatalf("expected LegendMode %q, got %q", mode, got.LegendMode)
				}
			})
		}
	}
}

// TestExpandSeriesLegendMode_Absent verifies that a null legend_mode produces
// an empty LegendMode string, so the json:"legendMode,omitempty" tag omits the
// field from the serialized API payload.
func TestExpandSeriesLegendMode_Absent(t *testing.T) {
	for tfType, buildVis := range legendModeVisBuilders {
		t.Run(tfType, func(t *testing.T) {
			vis := buildVis("")
			got, diags := expandVisualization(context.Background(), vis)
			if diags.HasError() {
				t.Fatalf("expandVisualization failed: %v", diags)
			}
			if got.LegendMode != "" {
				t.Fatalf("expected empty LegendMode for null input, got %q", got.LegendMode)
			}
		})
	}
}

// TestFlattenSeriesLegendMode_Set verifies that a LegendMode present in the API
// response is surfaced as a non-null types.String in each supported type's state.
func TestFlattenSeriesLegendMode_Set(t *testing.T) {
	for tfType, apiType := range apiTypeByTFType {
		t.Run(tfType, func(t *testing.T) {
			apiVis := dashboardVisualization{
				Type:       apiType,
				Source:     "metrics",
				LegendMode: "legend-only",
			}
			val, diags := flattenSeriesVisualization(context.Background(), apiVis)
			if diags.HasError() {
				t.Fatalf("flattenSeriesVisualization failed: %v", diags)
			}
			obj := val.(types.Object)
			lm := obj.Attributes()["legend_mode"].(types.String)
			if lm.IsNull() || lm.ValueString() != "legend-only" {
				t.Fatalf("expected legend_mode='legend-only', got null=%v value=%q", lm.IsNull(), lm.ValueString())
			}
		})
	}
}

// TestFlattenSeriesLegendMode_Absent verifies backward compatibility: an API
// response that omits legendMode produces types.StringNull() in state, so
// existing configurations without legend_mode produce no plan diff.
func TestFlattenSeriesLegendMode_Absent(t *testing.T) {
	for tfType, apiType := range apiTypeByTFType {
		t.Run(tfType, func(t *testing.T) {
			apiVis := dashboardVisualization{
				Type:   apiType,
				Source: "metrics",
				// LegendMode intentionally zero-valued.
			}
			val, diags := flattenSeriesVisualization(context.Background(), apiVis)
			if diags.HasError() {
				t.Fatalf("flattenSeriesVisualization failed: %v", diags)
			}
			obj := val.(types.Object)
			lm := obj.Attributes()["legend_mode"].(types.String)
			if !lm.IsNull() {
				t.Fatalf("expected legend_mode null for absent API field, got %q", lm.ValueString())
			}
		})
	}
}

// TestSeriesLegendMode_JSONWireRoundTrip crosses the JSON boundary to verify
// that the legendMode key is correctly included in (or excluded from) the
// marshaled payload, and that the full expand->marshal->unmarshal->flatten
// chain preserves the value end-to-end.
func TestSeriesLegendMode_JSONWireRoundTrip(t *testing.T) {
	for _, mode := range []string{"table", "legend-only", "no-legend"} {
		for tfType, buildVis := range legendModeVisBuilders {
			t.Run(fmt.Sprintf("%s/%s", tfType, mode), func(t *testing.T) {
				// Expand Terraform model → API struct.
				expanded, diags := expandVisualization(context.Background(), buildVis(mode))
				if diags.HasError() {
					t.Fatalf("expand: %v", diags)
				}

				// Marshal to JSON (exercises the MarshalJSON / omitempty logic).
				raw, err := json.Marshal(expanded)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}

				// Verify the key is present in the JSON payload.
				var m map[string]interface{}
				if err := json.Unmarshal(raw, &m); err != nil {
					t.Fatalf("unmarshal to map: %v", err)
				}
				v, ok := m["legendMode"]
				if !ok {
					t.Fatalf("legendMode missing from JSON payload: %s", raw)
				}
				if v != mode {
					t.Fatalf("legendMode in JSON: want %q, got %q", mode, v)
				}

				// Unmarshal back and flatten to confirm the value round-trips.
				var recovered dashboardVisualization
				if err := json.Unmarshal(raw, &recovered); err != nil {
					t.Fatalf("unmarshal to struct: %v", err)
				}
				flatVal, diags := flattenSeriesVisualization(context.Background(), recovered)
				if diags.HasError() {
					t.Fatalf("flatten: %v", diags)
				}
				obj := flatVal.(types.Object)
				lm := obj.Attributes()["legend_mode"].(types.String)
				if lm.IsNull() || lm.ValueString() != mode {
					t.Fatalf("round-trip: want %q, got null=%v value=%q", mode, lm.IsNull(), lm.ValueString())
				}
			})
		}
	}
}

// TestSeriesLegendMode_JSONWire_Absent verifies that a null legend_mode (empty
// string after expand) is omitted from the JSON payload entirely.
func TestSeriesLegendMode_JSONWire_Absent(t *testing.T) {
	for tfType, buildVis := range legendModeVisBuilders {
		t.Run(tfType, func(t *testing.T) {
			expanded, diags := expandVisualization(context.Background(), buildVis(""))
			if diags.HasError() {
				t.Fatalf("expand: %v", diags)
			}
			raw, err := json.Marshal(expanded)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var m map[string]interface{}
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if _, present := m["legendMode"]; present {
				t.Fatalf("legendMode must be absent from JSON when null, but found it in: %s", raw)
			}
		})
	}
}

// TestAccDashboardResource_SeriesLegendMode exercises the legend_mode attribute
// on standard timeseries and bar series visualizations through a live create +
// read round trip, mirroring the existing timeseries_connection.legend_mode
// acceptance step (dashboard_resource_test.go line 174/215).
//
// The test is gated on TF_ACC so it compiles and validates config even when
// live API credentials are unavailable.
func TestAccDashboardResource_SeriesLegendMode(t *testing.T) {
	teamName := fmt.Sprintf("test-legend-mode-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = %q
  visibility = "public"
}

resource "tsuga_dashboard" "test" {
  name  = "legend-mode-test"
  owner = tsuga_team.test.id

  graphs = [
    {
      id   = "ts-1"
      name = "Timeseries with legend_mode"
      visualization = {
        timeseries = {
          source      = "metrics"
          legend_mode = "legend-only"
          queries = [{
            aggregate = { count = {} }
          }]
        }
      }
    },
    {
      id   = "bar-1"
      name = "Bar with legend_mode"
      visualization = {
        bar = {
          source      = "metrics"
          legend_mode = "no-legend"
          queries = [{
            aggregate = { count = {} }
          }]
        }
      }
    },
    {
      id   = "pie-1"
      name = "Pie with legend_mode"
      visualization = {
        pie = {
          source      = "metrics"
          legend_mode = "table"
          queries = [{
            aggregate = { count = {} }
          }]
        }
      }
    },
    {
      id   = "qv-1"
      name = "QueryValue with legend_mode"
      visualization = {
        query_value = {
          source      = "metrics"
          legend_mode = "legend-only"
          queries = [{
            aggregate = { count = {} }
          }]
        }
      }
    },
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "graphs.#", "4"),
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "graphs.0.visualization.timeseries.legend_mode", "legend-only"),
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "graphs.1.visualization.bar.legend_mode", "no-legend"),
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "graphs.2.visualization.pie.legend_mode", "table"),
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "graphs.3.visualization.query_value.legend_mode", "legend-only"),
				),
			},
		},
	})
}
