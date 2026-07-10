package resource_dashboard

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-tsuga/internal/aggregate"
	"terraform-provider-tsuga/internal/groupby"
	"terraform-provider-tsuga/internal/normalizer"
	"terraform-provider-tsuga/internal/resource_team"
)

func DashboardResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "Visualization of telemetry data with customizable graphs and filters",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the dashboard",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the dashboard",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"owner": schema.StringAttribute{
				Required:    true,
				Description: "Team ID that owns and manages the dashboard",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"filters": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Filters applied to every widget on the dashboard",
				Validators: []validator.List{
					listvalidator.SizeAtMost(10),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:    true,
							Description: "Filter key",
							Validators: []validator.String{
								stringvalidator.LengthAtMost(250),
							},
						},
						"values": schema.ListAttribute{
							Required:    true,
							Description: "Filter values",
							ElementType: types.StringType,
							Validators: []validator.List{
								listvalidator.ValueStringsAre(
									stringvalidator.LengthAtMost(250),
								),
							},
						},
					},
				},
			},
			"tags": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of key/value tags applied to the resource",
				Validators: []validator.List{
					listvalidator.SizeAtMost(50),
				},
				NestedObject: schema.NestedAttributeObject{
					CustomType: resource_team.TagsType{
						ObjectType: types.ObjectType{
							AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx),
						},
					},
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtMost(128),
							},
						},
						"value": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtMost(256),
							},
						},
					},
				},
			},
			"time_preset": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Preset time range for dashboard queries",
				Validators: []validator.String{
					stringvalidator.OneOf(
						"past-5-minutes",
						"past-15-minutes",
						"past-30-minutes",
						"past-1-hour",
						"past-2-hours",
						"past-4-hours",
						"past-6-hours",
						"past-12-hours",
						"past-24-hours",
						"past-2-days",
						"past-3-days",
						"past-7-days",
						"past-30-days",
						"past-3-months",
						"current-day",
						"current-week",
						"current-month",
						"current-year",
						"previous-day",
						"previous-week",
						"previous-month",
						"previous-3-months",
						"previous-year",
					),
				},
			},
			"graphs": schema.ListNestedAttribute{
				Required:    true,
				Description: "Ordered widgets that compose the dashboard",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:    true,
							Description: "Identifier of the graph widget",
							Validators: []validator.String{
								stringvalidator.LengthAtMost(250),
							},
						},
						"name": schema.StringAttribute{
							Optional:    true,
							Description: "Display name of the graph widget",
							Validators: []validator.String{
								stringvalidator.LengthAtMost(250),
							},
						},
						"description": schema.StringAttribute{
							Optional:    true,
							Description: "Description of the graph widget",
							Validators: []validator.String{
								stringvalidator.LengthBetween(1, 800),
							},
						},
						"description_align": schema.StringAttribute{
							Optional:    true,
							Description: "Flex alignment keyword used for widget layout",
							Validators: []validator.String{
								stringvalidator.OneOf("flex-start", "center", "flex-end"),
							},
						},
						"description_justify_content": schema.StringAttribute{
							Optional:    true,
							Description: "Flex alignment keyword used for widget layout",
							Validators: []validator.String{
								stringvalidator.OneOf("flex-start", "center", "flex-end"),
							},
						},
						"layout": schema.SingleNestedAttribute{
							Optional:    true,
							Description: "Grid layout coordinates for this widget",
							Attributes: map[string]schema.Attribute{
								"x": schema.Float64Attribute{
									Required:    true,
									Description: "Horizontal grid position of the widget",
								},
								"y": schema.Float64Attribute{
									Required:    true,
									Description: "Vertical grid position of the widget",
								},
								"w": schema.Float64Attribute{
									Required:    true,
									Description: "Width of the widget in grid units",
								},
								"h": schema.Float64Attribute{
									Required:    true,
									Description: "Height of the widget in grid units",
								},
							},
						},
						"visualization": schema.SingleNestedAttribute{
							Required: true,
							Attributes: map[string]schema.Attribute{
								"timeseries":             visualizationTimeseriesSchema(),
								"top_list":               visualizationTopListSchema(),
								"pie":                    visualizationPieSchema(),
								"query_value":            visualizationQueryValueSchema(),
								"bar":                    visualizationBarSchema(),
								"gauge":                  visualizationGaugeSchema(),
								"distribution":           visualizationDistributionSchema(),
								"heatmap":                visualizationHeatmapSchema(),
								"list":                   visualizationListSchema(),
								"list_log_patterns":      visualizationListLogPatternsSchema(),
								"note":                   visualizationNoteSchema(),
								"table":                  visualizationTableSchema(),
								"timeseries_connection":  visualizationTimeseriesConnectionSchema(),
								"list_connection":        visualizationListConnectionSchema(),
								"top_list_connection":    visualizationTopListConnectionSchema(),
								"pie_connection":         visualizationPieConnectionSchema(),
								"bar_connection":         visualizationBarConnectionSchema(),
								"query_value_connection": visualizationQueryValueConnectionSchema(),
							},
						},
					},
				},
			},
		},
	}
}

func visualizationSeriesSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"source": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("logs", "metrics", "traces"),
				},
			},
			"queries":        queriesSchema(),
			"formula":        formulaSchema(),
			"aliases":        aliasesSchema(),
			"visible_series": visibleSeriesSchema(),
			"group_by":       groupBySchema(),
			"normalizer":     normalizer.Schema(),
			"precision": schema.Float64Attribute{
				Optional:    true,
				Description: "Number of decimal places to display in the value",
			},
			"y_axis_settings": yAxisSettingsSchema(),
		},
	}
}

func visualizationTimeseriesSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	attr.Attributes["smoothing"] = schema.BoolAttribute{
		Optional:    true,
		Description: "Whether to apply automatic smoothing to the rendered timeseries",
	}
	attr.Attributes["legend_mode"] = legendModeSchema()
	return attr
}

func yAxisSettingsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"min": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf("auto", "number"),
						},
					},
					"value": schema.Float64Attribute{
						Optional: true,
					},
				},
			},
			"max": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf("auto", "number"),
						},
					},
					"value": schema.Float64Attribute{
						Optional: true,
					},
				},
			},
			"scale": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf("linear", "log", "sqrt", "pow"),
						},
					},
					"exponent": schema.Float64Attribute{
						Optional: true,
					},
				},
			},
			"always_include_zero": schema.BoolAttribute{
				Required: true,
			},
		},
	}
}

func visualizationTableSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"columns": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtMost(250),
							},
						},
						"source": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf("logs", "metrics", "traces"),
							},
						},
						"queries":        queriesSchema(),
						"formula":        formulaSchema(),
						"aliases":        aliasesSchema(),
						"visible_series": visibleSeriesSchema(),
						"normalizer":     normalizer.Schema(),
						"precision": schema.Float64Attribute{
							Optional: true,
						},
					},
				},
			},
			"group_by": groupBySchema(),
		},
	}
}

func queriesSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Required: true,
		Validators: []validator.List{
			listvalidator.SizeAtMost(15),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"aggregate": aggregate.Schema(),
				"filter": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(10000),
					},
				},
				"functions": schema.ListNestedAttribute{
					Optional: true,
					Validators: []validator.List{
						listvalidator.SizeAtMost(10),
					},
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							"type": schema.StringAttribute{
								Required: true,
								Validators: []validator.String{
									stringvalidator.OneOf("per-second", "per-minute", "per-hour", "rate", "increase", "rolling", "log", "power", "sqrt", "last", "time-offset"),
								},
							},
							"window": schema.StringAttribute{
								Optional: true,
							},
							"seconds": schema.Int64Attribute{
								Optional:    true,
								Description: "Number of seconds to offset for the time-offset function",
								Validators: []validator.Int64{
									int64validator.AtLeast(1),
								},
							},
							"base": schema.Int64Attribute{
								Optional:    true,
								Description: "Base of the logarithm for the log function",
								Validators: []validator.Int64{
									int64validator.AtLeast(2),
								},
							},
							"exponent": schema.Int64Attribute{
								Optional:    true,
								Description: "Exponent to raise values to for the power function",
							},
						},
					},
				},
			},
		},
	}
}

func formulaSchema() schema.Attribute {
	return schema.StringAttribute{
		Optional: true,
		Validators: []validator.String{
			stringvalidator.LengthAtMost(250),
		},
	}
}

func aliasesSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"formula": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"queries": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func visibleSeriesSchema() schema.Attribute {
	return schema.ListAttribute{
		Optional:    true,
		ElementType: types.BoolType,
	}
}

func groupBySchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		Validators: []validator.List{
			listvalidator.SizeAtMost(3),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"fields": schema.ListAttribute{
					Required:    true,
					ElementType: types.StringType,
					Validators: []validator.List{
						listvalidator.SizeAtMost(1),
					},
				},
				"limit": schema.Int64Attribute{
					Required: true,
				},
				"sort_order": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.OneOf("asc", "desc"),
					},
					Description: "Sort direction applied to groups: 'asc' or 'desc'.",
				},
				"replace_null_with": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtLeast(1),
					},
					Description: "Value used to group documents that have no value for a grouped field.",
				},
			},
		},
	}
}

func conditionsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"operator": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.OneOf("greater_than", "less_than", "equal", "not_equal", "greater_than_or_equal", "less_than_or_equal"),
					},
				},
				"value": schema.Float64Attribute{
					Required: true,
				},
				"color": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.OneOf("alert", "warning", "success"),
					},
				},
			},
		},
	}
}

func visualizationPieSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	delete(attr.Attributes, "y_axis_settings")
	attr.Attributes["legend_mode"] = legendModeSchema()
	return attr
}

func visualizationTopListSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	delete(attr.Attributes, "y_axis_settings")
	attr.Attributes["conditions"] = conditionsSchema()
	return attr
}

func visualizationQueryValueSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	delete(attr.Attributes, "group_by")
	delete(attr.Attributes, "y_axis_settings")
	attr.Attributes["background_mode"] = schema.StringAttribute{
		Optional: true,
		Validators: []validator.String{
			stringvalidator.OneOf("background", "no-background"),
		},
	}
	attr.Attributes["conditions"] = conditionsSchema()
	attr.Attributes["legend_mode"] = legendModeSchema()
	return attr
}

func visualizationBarSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	attr.Attributes["time_bucket"] = schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"time": schema.Float64Attribute{Required: true},
			"metric": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("sec", "min", "hour", "day"),
				},
			},
		},
	}
	attr.Attributes["legend_mode"] = legendModeSchema()
	return attr
}

func visualizationGaugeSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	attr.Description = "Displays the aggregation as a gauge"
	delete(attr.Attributes, "group_by")
	delete(attr.Attributes, "y_axis_settings")
	attr.Attributes["max"] = schema.Float64Attribute{
		Optional:    true,
		Description: "Gauge maximum value",
	}
	attr.Attributes["color_thresholds"] = schema.ListNestedAttribute{
		Optional:    true,
		Description: "Color thresholds inside the gauge range",
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"from": schema.Float64Attribute{
					Required:    true,
					Description: "Lower bound of the gauge color threshold; runs up to the next threshold or the max",
				},
				"color": schema.StringAttribute{
					Required:    true,
					Description: "Color applied to the band starting at this value",
					Validators: []validator.String{
						stringvalidator.OneOf("red", "pink", "violet", "blue", "cyan", "green", "yellow", "orange"),
					},
				},
			},
		},
	}
	return attr
}

func visualizationDistributionSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	attr.Description = "Displays the aggregation as a distribution chart"
	delete(attr.Attributes, "y_axis_settings")
	attr.Attributes["bounds_scale"] = schema.StringAttribute{
		Optional: true,
		Validators: []validator.String{
			stringvalidator.OneOf("linear", "log"),
		},
	}
	attr.Attributes["percentile_markers"] = schema.ListAttribute{
		Optional:    true,
		ElementType: types.Int64Type,
		Description: "Percentile markers (0-100) displayed on top of the distribution chart",
		Validators: []validator.List{
			listvalidator.ValueInt64sAre(
				int64validator.Between(0, 100),
			),
		},
	}
	return attr
}

func visualizationHeatmapSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	attr.Description = "Displays the aggregation as a heatmap chart"
	delete(attr.Attributes, "y_axis_settings")
	attr.Attributes["palette"] = schema.StringAttribute{
		Optional:    true,
		Description: "Color palette used to render the heatmap intensity gradient",
		Validators: []validator.String{
			stringvalidator.OneOf("red", "pink", "violet", "blue", "cyan", "green", "yellow", "orange"),
		},
	}
	return attr
}

func visualizationListLogPatternsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays log patterns clustered from logs matching the query",
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"query": schema.StringAttribute{
				Required:    true,
				Description: "Tsuga query that selects logs to cluster into patterns",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(10000),
				},
			},
			"layout": schema.StringAttribute{
				Optional:    true,
				Description: "Layout used to render log patterns",
				Validators: []validator.String{
					stringvalidator.OneOf("horizontal", "vertical"),
				},
			},
		},
	}
}

// connectionSqlQueriesSchema returns the required list of read-only SQL query
// strings shared by connection-based visualizations.
func connectionSqlQueriesSchema() schema.Attribute {
	return schema.ListAttribute{
		Required:    true,
		ElementType: types.StringType,
		Description: "Read-only SQL queries to execute against the connection",
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
			listvalidator.ValueStringsAre(
				stringvalidator.LengthBetween(1, 50000),
			),
		},
	}
}

func connectionIdSchema() schema.Attribute {
	return schema.StringAttribute{
		Required:    true,
		Description: "The ID of the connection to use to query the datastore",
		Validators: []validator.String{
			stringvalidator.LengthBetween(1, 250),
		},
	}
}

func legendModeSchema() schema.Attribute {
	return schema.StringAttribute{
		Optional:    true,
		Description: "Controls whether and how the widget displays legend or series details",
		Validators: []validator.String{
			stringvalidator.OneOf("table", "legend-only", "no-legend"),
		},
	}
}

func visualizationTopListConnectionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays the database rows-based aggregation as a ranked top list",
		Attributes: map[string]schema.Attribute{
			"type":          schema.StringAttribute{Computed: true},
			"connection_id": connectionIdSchema(),
			"queries":       connectionSqlQueriesSchema(),
		},
	}
}

func visualizationPieConnectionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays the database rows-based aggregation as a pie chart",
		Attributes: map[string]schema.Attribute{
			"type":          schema.StringAttribute{Computed: true},
			"connection_id": connectionIdSchema(),
			"queries":       connectionSqlQueriesSchema(),
			"legend_mode":   legendModeSchema(),
		},
	}
}

func visualizationBarConnectionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays the database rows-based aggregation as a bar chart",
		Attributes: map[string]schema.Attribute{
			"type":            schema.StringAttribute{Computed: true},
			"connection_id":   connectionIdSchema(),
			"queries":         connectionSqlQueriesSchema(),
			"legend_mode":     legendModeSchema(),
			"thresholds":      thresholdsSchema(),
			"y_axis_settings": yAxisSettingsSchema(),
		},
	}
}

func visualizationQueryValueConnectionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays a single value computed by a SQL query against a database connection",
		Attributes: map[string]schema.Attribute{
			"type":          schema.StringAttribute{Computed: true},
			"connection_id": connectionIdSchema(),
			"queries":       connectionSqlQueriesSchema(),
			"legend_mode":   legendModeSchema(),
			"background_mode": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("background", "no-background"),
				},
			},
			"conditions": conditionsSchema(),
			"normalizer": normalizer.Schema(),
			"precision": schema.Float64Attribute{
				Optional:    true,
				Description: "Number of decimal places to display in the value",
			},
		},
	}
}

func visualizationListSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"query": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(10000),
				},
			},
			"list_columns": listColumnsSchema(),
			"list_columns_size": schema.MapAttribute{
				Optional:    true,
				ElementType: types.Float64Type,
				Description: "Column widths keyed by column id",
			},
		},
	}
}

func listColumnsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"attribute": schema.StringAttribute{
					Required: true,
				},
				"normalizer": normalizer.Schema(),
			},
		},
	}
}

func thresholdsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional:    true,
		Description: "Threshold markers displayed on the chart",
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"value": schema.Float64Attribute{
					Required:    true,
					Description: "Y-axis value where the threshold marker is placed",
				},
				"level": schema.StringAttribute{
					Required:    true,
					Description: "Level applied to the threshold marker",
					Validators: []validator.String{
						stringvalidator.OneOf("alert", "warning", "success"),
					},
				},
			},
		},
	}
}

func visualizationTimeseriesConnectionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays database rows-based aggregation as a time series chart",
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"connection_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the connection to use to query the datastore",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"queries": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Read-only SQL queries to execute against the connection",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.ValueStringsAre(
						stringvalidator.LengthBetween(1, 50000),
					),
				},
			},
			"legend_mode": schema.StringAttribute{
				Optional:    true,
				Description: "Controls whether and how the widget displays legend or series details",
				Validators: []validator.String{
					stringvalidator.OneOf("table", "legend-only", "no-legend"),
				},
			},
			"thresholds":      thresholdsSchema(),
			"y_axis_settings": yAxisSettingsSchema(),
		},
	}
}

func visualizationListConnectionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays database rows as a tabular list",
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"connection_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the connection to use to query the datastore",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"query": schema.StringAttribute{
				Required:    true,
				Description: "Read-only SQL query to execute against the connection",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 50000),
				},
			},
			"list_columns": listColumnsSchema(),
			"list_columns_size": schema.MapAttribute{
				Optional:    true,
				ElementType: types.Float64Type,
				Description: "Column widths keyed by column id",
			},
		},
	}
}

func visualizationNoteSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"note": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(50000),
				},
			},
			"note_align": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("flex-start", "center", "flex-end"),
				},
			},
			"note_justify_content": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf("flex-start", "center", "flex-end"),
				},
			},
			"note_color": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"white",
						"gray.100",
						"amber.200",
						"lime.200",
						"emerald.200",
						"cyan.200",
						"blue.200",
						"violet.200",
						"fuchsia.200",
						"pink.200",
						"red.200",
					),
				},
			},
		},
	}
}

type DashboardModel struct {
	Id         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Owner      types.String `tfsdk:"owner"`
	Filters    types.List   `tfsdk:"filters"`
	Tags       types.List   `tfsdk:"tags"`
	TimePreset types.String `tfsdk:"time_preset"`
	Graphs     types.List   `tfsdk:"graphs"`
}

type GraphModel struct {
	Id                        types.String       `tfsdk:"id"`
	Name                      types.String       `tfsdk:"name"`
	Description               types.String       `tfsdk:"description"`
	DescriptionAlign          types.String       `tfsdk:"description_align"`
	DescriptionJustifyContent types.String       `tfsdk:"description_justify_content"`
	Layout                    *GraphLayoutModel  `tfsdk:"layout"`
	Visualization             VisualizationModel `tfsdk:"visualization"`
}

type GraphLayoutModel struct {
	X types.Float64 `tfsdk:"x"`
	Y types.Float64 `tfsdk:"y"`
	W types.Float64 `tfsdk:"w"`
	H types.Float64 `tfsdk:"h"`
}

type VisualizationModel struct {
	Timeseries           *TimeseriesVisualization           `tfsdk:"timeseries"`
	TopList              *TopListVisualization              `tfsdk:"top_list"`
	Pie                  *PieVisualization                  `tfsdk:"pie"`
	QueryValue           *QueryValueVisualization           `tfsdk:"query_value"`
	Bar                  *BarVisualization                  `tfsdk:"bar"`
	Gauge                *GaugeVisualization                `tfsdk:"gauge"`
	Distribution         *DistributionVisualization         `tfsdk:"distribution"`
	Heatmap              *HeatmapVisualization              `tfsdk:"heatmap"`
	List                 *ListVisualization                 `tfsdk:"list"`
	ListLogPatterns      *ListLogPatternsVisualization      `tfsdk:"list_log_patterns"`
	Note                 *NoteVisualizationModel            `tfsdk:"note"`
	Table                *TableVisualizationModel           `tfsdk:"table"`
	TimeseriesConnection *TimeseriesConnectionVisualization `tfsdk:"timeseries_connection"`
	ListConnection       *ListConnectionVisualization       `tfsdk:"list_connection"`
	TopListConnection    *TopListConnectionVisualization    `tfsdk:"top_list_connection"`
	PieConnection        *PieConnectionVisualization        `tfsdk:"pie_connection"`
	BarConnection        *BarConnectionVisualization        `tfsdk:"bar_connection"`
	QueryValueConnection *QueryValueConnectionVisualization `tfsdk:"query_value_connection"`
}

type SeriesBase struct {
	Type          types.String      `tfsdk:"type"`
	Source        types.String      `tfsdk:"source"`
	Queries       types.List        `tfsdk:"queries"`
	Formula       types.String      `tfsdk:"formula"`
	Aliases       *AliasesModel     `tfsdk:"aliases"`
	VisibleSeries types.List        `tfsdk:"visible_series"`
	Normalizer    *normalizer.Model `tfsdk:"normalizer"`
	Precision     types.Float64     `tfsdk:"precision"`
}

type SeriesVisualizationModel struct {
	SeriesBase
	GroupBy       types.List          `tfsdk:"group_by"`
	YAxisSettings *YAxisSettingsModel `tfsdk:"y_axis_settings"`
	LegendMode    types.String        `tfsdk:"legend_mode"`
}

type TimeseriesVisualization struct {
	SeriesVisualizationModel
	Smoothing types.Bool `tfsdk:"smoothing"`
}

type AliasesModel struct {
	Formula types.String `tfsdk:"formula"`
	Queries types.Map    `tfsdk:"queries"`
}

type YAxisSettingsModel struct {
	Min               YAxisBoundModel `tfsdk:"min"`
	Max               YAxisBoundModel `tfsdk:"max"`
	Scale             YAxisScaleModel `tfsdk:"scale"`
	AlwaysIncludeZero types.Bool      `tfsdk:"always_include_zero"`
}

type YAxisBoundModel struct {
	Type  types.String  `tfsdk:"type"`
	Value types.Float64 `tfsdk:"value"`
}

type YAxisScaleModel struct {
	Type     types.String  `tfsdk:"type"`
	Exponent types.Float64 `tfsdk:"exponent"`
}

type TableVisualizationModel struct {
	Type    types.String `tfsdk:"type"`
	Columns types.List   `tfsdk:"columns"`
	GroupBy types.List   `tfsdk:"group_by"`
}

type TableColumnModel struct {
	Name          types.String      `tfsdk:"name"`
	Source        types.String      `tfsdk:"source"`
	Queries       types.List        `tfsdk:"queries"`
	Formula       types.String      `tfsdk:"formula"`
	Aliases       *AliasesModel     `tfsdk:"aliases"`
	VisibleSeries types.List        `tfsdk:"visible_series"`
	Normalizer    *normalizer.Model `tfsdk:"normalizer"`
	Precision     types.Float64     `tfsdk:"precision"`
}

type QueryValueVisualization struct {
	SeriesBase
	BackgroundMode types.String `tfsdk:"background_mode"`
	Conditions     types.List   `tfsdk:"conditions"`
	LegendMode     types.String `tfsdk:"legend_mode"`
}

type PieVisualization struct {
	SeriesBase
	GroupBy    types.List   `tfsdk:"group_by"`
	LegendMode types.String `tfsdk:"legend_mode"`
}

type TopListVisualization struct {
	SeriesBase
	GroupBy    types.List `tfsdk:"group_by"`
	Conditions types.List `tfsdk:"conditions"`
}

type BarVisualization struct {
	SeriesVisualizationModel
	TimeBucket *TimeBucketModel `tfsdk:"time_bucket"`
}

type GaugeVisualization struct {
	SeriesBase
	Max             types.Float64 `tfsdk:"max"`
	ColorThresholds types.List    `tfsdk:"color_thresholds"`
}

type GaugeColorThresholdModel struct {
	From  types.Float64 `tfsdk:"from"`
	Color types.String  `tfsdk:"color"`
}

type DistributionVisualization struct {
	SeriesBase
	GroupBy           types.List   `tfsdk:"group_by"`
	BoundsScale       types.String `tfsdk:"bounds_scale"`
	PercentileMarkers types.List   `tfsdk:"percentile_markers"`
}

type HeatmapVisualization struct {
	SeriesBase
	GroupBy types.List   `tfsdk:"group_by"`
	Palette types.String `tfsdk:"palette"`
}

type ListLogPatternsVisualization struct {
	Type   types.String `tfsdk:"type"`
	Query  types.String `tfsdk:"query"`
	Layout types.String `tfsdk:"layout"`
}

type TopListConnectionVisualization struct {
	Type         types.String `tfsdk:"type"`
	ConnectionId types.String `tfsdk:"connection_id"`
	Queries      types.List   `tfsdk:"queries"`
}

type PieConnectionVisualization struct {
	Type         types.String `tfsdk:"type"`
	ConnectionId types.String `tfsdk:"connection_id"`
	Queries      types.List   `tfsdk:"queries"`
	LegendMode   types.String `tfsdk:"legend_mode"`
}

type BarConnectionVisualization struct {
	Type          types.String        `tfsdk:"type"`
	ConnectionId  types.String        `tfsdk:"connection_id"`
	Queries       types.List          `tfsdk:"queries"`
	LegendMode    types.String        `tfsdk:"legend_mode"`
	Thresholds    types.List          `tfsdk:"thresholds"`
	YAxisSettings *YAxisSettingsModel `tfsdk:"y_axis_settings"`
}

type QueryValueConnectionVisualization struct {
	Type           types.String      `tfsdk:"type"`
	ConnectionId   types.String      `tfsdk:"connection_id"`
	Queries        types.List        `tfsdk:"queries"`
	LegendMode     types.String      `tfsdk:"legend_mode"`
	BackgroundMode types.String      `tfsdk:"background_mode"`
	Conditions     types.List        `tfsdk:"conditions"`
	Normalizer     *normalizer.Model `tfsdk:"normalizer"`
	Precision      types.Float64     `tfsdk:"precision"`
}

type ListVisualization struct {
	Type            types.String `tfsdk:"type"`
	Query           types.String `tfsdk:"query"`
	ListColumns     types.List   `tfsdk:"list_columns"`
	ListColumnsSize types.Map    `tfsdk:"list_columns_size"`
}

type TimeseriesConnectionVisualization struct {
	Type          types.String        `tfsdk:"type"`
	ConnectionId  types.String        `tfsdk:"connection_id"`
	Queries       types.List          `tfsdk:"queries"`
	LegendMode    types.String        `tfsdk:"legend_mode"`
	Thresholds    types.List          `tfsdk:"thresholds"`
	YAxisSettings *YAxisSettingsModel `tfsdk:"y_axis_settings"`
}

type ThresholdModel struct {
	Value types.Float64 `tfsdk:"value"`
	Level types.String  `tfsdk:"level"`
}

type ListConnectionVisualization struct {
	Type            types.String `tfsdk:"type"`
	ConnectionId    types.String `tfsdk:"connection_id"`
	Query           types.String `tfsdk:"query"`
	ListColumns     types.List   `tfsdk:"list_columns"`
	ListColumnsSize types.Map    `tfsdk:"list_columns_size"`
}

type NoteVisualizationModel struct {
	Type               types.String `tfsdk:"type"`
	Note               types.String `tfsdk:"note"`
	NoteAlign          types.String `tfsdk:"note_align"`
	NoteJustifyContent types.String `tfsdk:"note_justify_content"`
	NoteColor          types.String `tfsdk:"note_color"`
}

type QueryModel struct {
	Aggregate AggregateModel `tfsdk:"aggregate"`
	Filter    types.String   `tfsdk:"filter"`
	Functions types.List     `tfsdk:"functions"`
}

type AggregateModel struct {
	Count      *aggregate.CountModel      `tfsdk:"count"`
	Sum        *aggregate.FieldModel      `tfsdk:"sum"`
	Average    *aggregate.FieldModel      `tfsdk:"average"`
	Min        *aggregate.FieldModel      `tfsdk:"min"`
	Max        *aggregate.FieldModel      `tfsdk:"max"`
	Uniq       *aggregate.FieldModel      `tfsdk:"unique_count"`
	Percentile *aggregate.PercentileModel `tfsdk:"percentile"`
}

type FunctionModel struct {
	Type     types.String `tfsdk:"type"`
	Window   types.String `tfsdk:"window"`
	Seconds  types.Int64  `tfsdk:"seconds"`
	Base     types.Int64  `tfsdk:"base"`
	Exponent types.Int64  `tfsdk:"exponent"`
}

type ConditionModel struct {
	Operator types.String  `tfsdk:"operator"`
	Value    types.Float64 `tfsdk:"value"`
	Color    types.String  `tfsdk:"color"`
}

type FilterModel struct {
	Key    types.String `tfsdk:"key"`
	Values types.List   `tfsdk:"values"`
}

type TimeBucketModel struct {
	Time   types.Float64 `tfsdk:"time"`
	Metric types.String  `tfsdk:"metric"`
}

type ListColumnModel struct {
	Attribute  types.String      `tfsdk:"attribute"`
	Normalizer *normalizer.Model `tfsdk:"normalizer"`
}

func FilterAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key":    types.StringType,
		"values": types.ListType{ElemType: types.StringType},
	}
}

func GraphAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                          types.StringType,
		"name":                        types.StringType,
		"description":                 types.StringType,
		"description_align":           types.StringType,
		"description_justify_content": types.StringType,
		"layout":                      types.ObjectType{AttrTypes: GraphLayoutAttrTypes()},
		"visualization":               types.ObjectType{AttrTypes: VisualizationAttrTypes()},
	}
}

func GraphLayoutAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"x": types.Float64Type,
		"y": types.Float64Type,
		"w": types.Float64Type,
		"h": types.Float64Type,
	}
}

func VisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"timeseries":             types.ObjectType{AttrTypes: TimeseriesVisualizationAttrTypes()},
		"top_list":               types.ObjectType{AttrTypes: TopListVisualizationAttrTypes()},
		"pie":                    types.ObjectType{AttrTypes: PieVisualizationAttrTypes()},
		"query_value":            types.ObjectType{AttrTypes: QueryValueVisualizationAttrTypes()},
		"bar":                    types.ObjectType{AttrTypes: BarVisualizationAttrTypes()},
		"gauge":                  types.ObjectType{AttrTypes: GaugeVisualizationAttrTypes()},
		"distribution":           types.ObjectType{AttrTypes: DistributionVisualizationAttrTypes()},
		"heatmap":                types.ObjectType{AttrTypes: HeatmapVisualizationAttrTypes()},
		"list":                   types.ObjectType{AttrTypes: ListVisualizationAttrTypes()},
		"list_log_patterns":      types.ObjectType{AttrTypes: ListLogPatternsVisualizationAttrTypes()},
		"note":                   types.ObjectType{AttrTypes: NoteVisualizationAttrTypes()},
		"table":                  types.ObjectType{AttrTypes: TableVisualizationAttrTypes()},
		"timeseries_connection":  types.ObjectType{AttrTypes: TimeseriesConnectionVisualizationAttrTypes()},
		"list_connection":        types.ObjectType{AttrTypes: ListConnectionVisualizationAttrTypes()},
		"top_list_connection":    types.ObjectType{AttrTypes: TopListConnectionVisualizationAttrTypes()},
		"pie_connection":         types.ObjectType{AttrTypes: PieConnectionVisualizationAttrTypes()},
		"bar_connection":         types.ObjectType{AttrTypes: BarConnectionVisualizationAttrTypes()},
		"query_value_connection": types.ObjectType{AttrTypes: QueryValueConnectionVisualizationAttrTypes()},
	}
}

func SeriesVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":            types.StringType,
		"source":          types.StringType,
		"queries":         types.ListType{ElemType: types.ObjectType{AttrTypes: QueryAttrTypes()}},
		"formula":         types.StringType,
		"aliases":         types.ObjectType{AttrTypes: AliasesAttrTypes()},
		"visible_series":  types.ListType{ElemType: types.BoolType},
		"group_by":        types.ListType{ElemType: types.ObjectType{AttrTypes: groupby.AttrTypes()}},
		"normalizer":      types.ObjectType{AttrTypes: normalizer.AttrTypes()},
		"precision":       types.Float64Type,
		"y_axis_settings": types.ObjectType{AttrTypes: YAxisSettingsAttrTypes()},
	}
}

func TimeseriesVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	attrs["smoothing"] = types.BoolType
	attrs["legend_mode"] = types.StringType
	return attrs
}

func AliasesAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"formula": types.StringType,
		"queries": types.MapType{ElemType: types.StringType},
	}
}

func YAxisSettingsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"min":                 types.ObjectType{AttrTypes: YAxisBoundAttrTypes()},
		"max":                 types.ObjectType{AttrTypes: YAxisBoundAttrTypes()},
		"scale":               types.ObjectType{AttrTypes: YAxisScaleAttrTypes()},
		"always_include_zero": types.BoolType,
	}
}

func YAxisBoundAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":  types.StringType,
		"value": types.Float64Type,
	}
}

func YAxisScaleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":     types.StringType,
		"exponent": types.Float64Type,
	}
}

func TableVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":     types.StringType,
		"columns":  types.ListType{ElemType: types.ObjectType{AttrTypes: TableColumnAttrTypes()}},
		"group_by": types.ListType{ElemType: types.ObjectType{AttrTypes: groupby.AttrTypes()}},
	}
}

func TableColumnAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":           types.StringType,
		"source":         types.StringType,
		"queries":        types.ListType{ElemType: types.ObjectType{AttrTypes: QueryAttrTypes()}},
		"formula":        types.StringType,
		"aliases":        types.ObjectType{AttrTypes: AliasesAttrTypes()},
		"visible_series": types.ListType{ElemType: types.BoolType},
		"normalizer":     types.ObjectType{AttrTypes: normalizer.AttrTypes()},
		"precision":      types.Float64Type,
	}
}

func PieVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	delete(attrs, "y_axis_settings")
	attrs["legend_mode"] = types.StringType
	return attrs
}

func QueryValueVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	delete(attrs, "group_by")
	delete(attrs, "y_axis_settings")
	attrs["background_mode"] = types.StringType
	attrs["conditions"] = types.ListType{ElemType: types.ObjectType{AttrTypes: ConditionAttrTypes()}}
	attrs["legend_mode"] = types.StringType
	return attrs
}

func TopListVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	delete(attrs, "y_axis_settings")
	attrs["conditions"] = types.ListType{ElemType: types.ObjectType{AttrTypes: ConditionAttrTypes()}}
	return attrs
}

func BarVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	attrs["time_bucket"] = types.ObjectType{AttrTypes: TimeBucketAttrTypes()}
	attrs["legend_mode"] = types.StringType
	return attrs
}

func GaugeVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	delete(attrs, "group_by")
	delete(attrs, "y_axis_settings")
	attrs["max"] = types.Float64Type
	attrs["color_thresholds"] = types.ListType{ElemType: types.ObjectType{AttrTypes: GaugeColorThresholdAttrTypes()}}
	return attrs
}

func GaugeColorThresholdAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"from":  types.Float64Type,
		"color": types.StringType,
	}
}

func DistributionVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	delete(attrs, "y_axis_settings")
	attrs["bounds_scale"] = types.StringType
	attrs["percentile_markers"] = types.ListType{ElemType: types.Int64Type}
	return attrs
}

func HeatmapVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	delete(attrs, "y_axis_settings")
	attrs["palette"] = types.StringType
	return attrs
}

func ListLogPatternsVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":   types.StringType,
		"query":  types.StringType,
		"layout": types.StringType,
	}
}

func TopListConnectionVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":          types.StringType,
		"connection_id": types.StringType,
		"queries":       types.ListType{ElemType: types.StringType},
	}
}

func PieConnectionVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":          types.StringType,
		"connection_id": types.StringType,
		"queries":       types.ListType{ElemType: types.StringType},
		"legend_mode":   types.StringType,
	}
}

func BarConnectionVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":            types.StringType,
		"connection_id":   types.StringType,
		"queries":         types.ListType{ElemType: types.StringType},
		"legend_mode":     types.StringType,
		"thresholds":      types.ListType{ElemType: types.ObjectType{AttrTypes: ThresholdAttrTypes()}},
		"y_axis_settings": types.ObjectType{AttrTypes: YAxisSettingsAttrTypes()},
	}
}

func QueryValueConnectionVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":            types.StringType,
		"connection_id":   types.StringType,
		"queries":         types.ListType{ElemType: types.StringType},
		"legend_mode":     types.StringType,
		"background_mode": types.StringType,
		"conditions":      types.ListType{ElemType: types.ObjectType{AttrTypes: ConditionAttrTypes()}},
		"normalizer":      types.ObjectType{AttrTypes: normalizer.AttrTypes()},
		"precision":       types.Float64Type,
	}
}

func ListVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":              types.StringType,
		"query":             types.StringType,
		"list_columns":      types.ListType{ElemType: types.ObjectType{AttrTypes: ListColumnAttrTypes()}},
		"list_columns_size": types.MapType{ElemType: types.Float64Type},
	}
}

func ThresholdAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"value": types.Float64Type,
		"level": types.StringType,
	}
}

func TimeseriesConnectionVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":            types.StringType,
		"connection_id":   types.StringType,
		"queries":         types.ListType{ElemType: types.StringType},
		"legend_mode":     types.StringType,
		"thresholds":      types.ListType{ElemType: types.ObjectType{AttrTypes: ThresholdAttrTypes()}},
		"y_axis_settings": types.ObjectType{AttrTypes: YAxisSettingsAttrTypes()},
	}
}

func ListConnectionVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":              types.StringType,
		"connection_id":     types.StringType,
		"query":             types.StringType,
		"list_columns":      types.ListType{ElemType: types.ObjectType{AttrTypes: ListColumnAttrTypes()}},
		"list_columns_size": types.MapType{ElemType: types.Float64Type},
	}
}

func NoteVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":                 types.StringType,
		"note":                 types.StringType,
		"note_align":           types.StringType,
		"note_justify_content": types.StringType,
		"note_color":           types.StringType,
	}
}

func QueryAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"aggregate": types.ObjectType{AttrTypes: aggregate.AttrTypes()},
		"filter":    types.StringType,
		"functions": types.ListType{ElemType: types.ObjectType{AttrTypes: FunctionAttrTypes()}},
	}
}

func FunctionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":     types.StringType,
		"window":   types.StringType,
		"seconds":  types.Int64Type,
		"base":     types.Int64Type,
		"exponent": types.Int64Type,
	}
}

// ConditionAttrTypes returns attr types for condition configuration.
func ConditionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"operator": types.StringType,
		"value":    types.Float64Type,
		"color":    types.StringType,
	}
}

func TimeBucketAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"time":   types.Float64Type,
		"metric": types.StringType,
	}
}

func ListColumnAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"attribute":  types.StringType,
		"normalizer": types.ObjectType{AttrTypes: normalizer.AttrTypes()},
	}
}
