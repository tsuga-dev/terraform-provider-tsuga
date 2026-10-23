package resource_dashboard

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
				Validators:  dashboardNameValidators(),
			},
			"owner": schema.StringAttribute{
				Required:    true,
				Description: "Team ID that owns and manages the dashboard",
				Validators:  ownerValidators(),
			},
			"folder_id": schema.StringAttribute{
				Optional:    true,
				Description: "ID of the dashboard folder holding the dashboard. Omit to leave the dashboard outside any folder.",
				Validators:  folderIdValidators(),
			},
			"filters": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Filters applied to every widget on the dashboard",
				Validators:  filtersListValidators(),
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:    true,
							Description: "Filter key",
							Validators:  filterKeyValidators(),
						},
						"values": schema.ListAttribute{
							Required:    true,
							Description: "Filter values",
							ElementType: types.StringType,
							Validators: []validator.List{
								listvalidator.ValueStringsAre(filterValueValidators()...),
							},
						},
						"exclude": schema.BoolAttribute{
							Optional:    true,
							Description: "If true, widgets exclude telemetry matching this filter instead of including it",
						},
					},
				},
			},
			"tags": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of key/value tags applied to the resource",
				Validators:  tagsListValidators(),
				NestedObject: schema.NestedAttributeObject{
					CustomType: resource_team.TagsType{
						ObjectType: types.ObjectType{
							AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx),
						},
					},
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:   true,
							Validators: tagKeyValidators(),
						},
						"value": schema.StringAttribute{
							Required:   true,
							Validators: tagValueValidators(),
						},
					},
				},
			},
			"time_preset": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Preset time range for dashboard queries",
				Validators:  timePresetValidators(),
			},
			"graphs": schema.ListNestedAttribute{
				Required:    true,
				Description: "Ordered widgets that compose the dashboard",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:    true,
							Description: "Identifier of the graph widget",
							Validators:  graphIdValidators(),
						},
						"name": schema.StringAttribute{
							Optional:    true,
							Description: "Display name of the graph widget",
							Validators:  graphNameValidators(),
						},
						"description": schema.StringAttribute{
							Optional:    true,
							Description: "Description of the graph widget",
							Validators:  graphDescriptionValidators(),
						},
						"description_align": schema.StringAttribute{
							Optional:    true,
							Description: "Flex alignment keyword used for widget layout",
							Validators:  flexAlignValidators(),
						},
						"description_justify_content": schema.StringAttribute{
							Optional:    true,
							Description: "Flex alignment keyword used for widget layout",
							Validators:  flexAlignValidators(),
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
								"list_spans":             visualizationListSpansSchema(),
								"list_log_patterns":      visualizationListLogPatternsSchema(),
								"note":                   visualizationNoteSchema(),
								"table":                  visualizationTableSchema(),
								"timeseries_connection":  visualizationTimeseriesConnectionSchema(),
								"list_connection":        visualizationListConnectionSchema(),
								"top_list_connection":    visualizationTopListConnectionSchema(),
								"pie_connection":         visualizationPieConnectionSchema(),
								"bar_connection":         visualizationBarConnectionSchema(),
								"query_value_connection": visualizationQueryValueConnectionSchema(),
								"timeseries_promql":      visualizationTimeseriesPromqlSchema(),
								"bar_promql":             visualizationBarPromqlSchema(),
								"pie_promql":             visualizationPiePromqlSchema(),
								"query_value_promql":     visualizationQueryValuePromqlSchema(),
								"top_list_promql":        visualizationTopListPromqlSchema(),
								"list_rum":               visualizationListRumSchema(),
								"slo":                    visualizationSloSchema(),
								"slo_over_time":          visualizationSloOverTimeSchema(),
								"slo_uptime":             visualizationSloUptimeSchema(),
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
				Required:   true,
				Validators: sourceValidators(),
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
	attr.Attributes["legend_mode"] = legendModeSchema()
	attr.Attributes["time_bucket"] = timeBucketSchema()
	attr.Attributes["smoothing"] = schema.BoolAttribute{
		Optional:    true,
		Description: "Whether to apply automatic smoothing to the rendered timeseries",
	}
	attr.Attributes["custom_link"] = customLinkSchema()
	attr.Attributes["group_by_mode"] = groupByModeSchema()
	attr.Attributes["line_style_options"] = lineStyleOptionsSchema()
	attr.Attributes["thresholds"] = thresholdsSchema()
	return attr
}

func customLinkSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Custom link added to the widget context menu",
		Attributes: map[string]schema.Attribute{
			"label": schema.StringAttribute{
				Optional:    true,
				Description: "Text of the entry added to the widget context menu. Defaults to \"Open custom link\" when omitted.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"url": schema.StringAttribute{
				Required:    true,
				Description: "URL opened in a new tab by the context menu entry. Every `{{group.<field>}}` placeholder is replaced by the URL-encoded value the clicked series has for that group-by field; without placeholders, every series opens the same URL. `{` and `}` are only allowed as part of a placeholder. Tsuga omits the menu entry when the clicked series has no value for one of the referenced fields.",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
					stringvalidator.RegexMatches(CustomLinkURLPattern, "must be an http(s) URL; `{` and `}` are only allowed in `{{group.<field>}}` placeholders"),
				},
			},
		},
	}
}

var CustomLinkURLPattern = regexp.MustCompile(`^https?://(?:[^\s{}]|\{\{group\.[^{}\s]+\}\})+$`)

func groupByModeSchema() schema.Attribute {
	return schema.StringAttribute{
		Optional:    true,
		Description: "`absolute` keeps each group at its own value; `relative` shows it as a percentage of the ungrouped total (defaults to absolute)",
		Validators: []validator.String{
			stringvalidator.OneOf("relative", "absolute"),
		},
	}
}

func lineStyleOptionsSchema() schema.Attribute {
	return schema.MapNestedAttribute{
		Optional:    true,
		Description: "Line style of each series, keyed by 1-based query index. The last index is the formula when there is one. For widgets with a single query, only the `1` entry is read and it applies to every series. Defaults to regular.",
		Validators: []validator.Map{
			mapvalidator.SizeAtLeast(1),
			mapvalidator.KeysAre(stringvalidator.RegexMatches(regexp.MustCompile(`^[1-9][0-9]*$`), "must be a 1-based query index")),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"group_id": schema.StringAttribute{
					Required: true,
				},
				"line_style": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.OneOf("regular", "dashed"),
					},
				},
			},
		},
	}
}

func timeBucketSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Groups time-series data into fixed-size time buckets",
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
}

func yAxisSettingsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"min": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required:   true,
						Validators: yAxisBoundTypeValidators(),
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
						Required:   true,
						Validators: yAxisBoundTypeValidators(),
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
						Required:   true,
						Validators: yAxisScaleTypeValidators(),
					},
					"exponent": schema.Float64Attribute{
						Optional:   true,
						Validators: yAxisScaleExponentValidators(),
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
	sorting := listDefaultSortingSchema().(schema.ListNestedAttribute)
	sorting.Description = "Default sorting applied to a table widget. Column IDs are `label` for the grouping column and `col-<index>` for each entry in `columns`. Users can still change sorting by selecting columns in the rendered table."
	id := sorting.NestedObject.Attributes["id"].(schema.StringAttribute)
	id.Description = "ID of the column to sort by"
	sorting.NestedObject.Attributes["id"] = id
	desc := sorting.NestedObject.Attributes["desc"].(schema.BoolAttribute)
	desc.Description = "Whether the sort is descending"
	sorting.NestedObject.Attributes["desc"] = desc

	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"columns": schema.ListNestedAttribute{
				Required:   true,
				Validators: tableColumnsListValidators(),
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:   true,
							Validators: tableColumnNameValidators(),
						},
						"source": schema.StringAttribute{
							Required:   true,
							Validators: sourceValidators(),
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
			"group_by":      groupBySchema(),
			"group_by_mode": groupByModeSchema(),
			"custom_link":   customLinkSchema(),
			"column_sizes": schema.MapAttribute{
				Optional:    true,
				ElementType: types.Float64Type,
				Description: "Table column widths in pixels, keyed by column id: `label` for the grouping column and `col-<index>` for each entry in `columns`. Columns without an entry keep their default width.",
				Validators: []validator.Map{
					mapvalidator.SizeAtLeast(1),
				},
			},
			"default_sorting": sorting,
		},
	}
}

func queriesSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Required:   true,
		Validators: queriesListValidators(),
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"aggregate": aggregate.Schema(),
				"filter": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(10000),
					},
				},
				"time_aggregate": schema.StringAttribute{
					Optional:    true,
					Description: "Per-series rollup applied within each time bucket before the cross-series aggregate. When omitted, a default is derived from the metric type.",
					Validators: []validator.String{
						stringvalidator.OneOf("avg", "sum", "min", "max", "last"),
					},
				},
				"metric_kind": schema.StringAttribute{
					Optional:    true,
					Description: "Variant of the metric in `aggregate.field` to query, for metric names reported under several kinds (for instance both cumulative and delta). When omitted, Tsuga picks one variant by type and temporality precedence.",
					Validators: []validator.String{
						stringvalidator.OneOf("gauge", "cumulative-counter", "delta-counter", "cumulative-histogram", "delta-histogram", "summary"),
					},
				},
				"fill": schema.SingleNestedAttribute{
					Optional:    true,
					Description: "Missing-point fill behavior for aggregation results. Omit to keep missing points null.",
					Attributes: map[string]schema.Attribute{
						"mode": schema.SingleNestedAttribute{
							Required:    true,
							Description: "How missing aggregation points should be filled.",
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{
									Required:    true,
									Description: "`zero` fills missing points with zero. `null` keeps missing points null.",
									Validators: []validator.String{
										stringvalidator.OneOf("zero", "null"),
									},
								},
							},
						},
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
								Required:   true,
								Validators: functionTypeValidators(),
							},
							"window": schema.StringAttribute{
								Optional:    true,
								Description: "Duration of the rolling window for the rolling function",
								Validators:  functionWindowValidators(),
							},
							"seconds": schema.Int64Attribute{
								Optional:    true,
								Description: "Number of seconds to offset for the time-offset function",
								Validators:  functionSecondsValidators(),
							},
							"base": schema.Int64Attribute{
								Optional:    true,
								Description: "Base of the logarithm for the log function",
								Validators:  functionBaseValidators(),
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
		Optional:   true,
		Validators: formulaValidators(),
	}
}

func aliasesSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"formula": schema.StringAttribute{
				Optional:   true,
				Validators: aliasesFormulaValidators(),
			},
			"queries": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Map{
					mapvalidator.ValueStringsAre(aliasesQueriesValueValidators()...),
				},
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
		Optional:   true,
		Validators: groupByListValidators(),
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"fields": schema.ListAttribute{
					Required:    true,
					ElementType: types.StringType,
					Validators:  groupByFieldsValidators(),
				},
				"limit": schema.Int64Attribute{
					Required: true,
				},
				"sort_order": schema.StringAttribute{
					Optional:    true,
					Validators:  []validator.String{stringvalidator.OneOf("asc", "desc")},
					Description: "Sort direction applied to groups: 'asc' or 'desc'.",
				},
				"replace_null_with": schema.StringAttribute{
					Optional:    true,
					Validators:  groupByReplaceNullWithValidators(),
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
					Required:   true,
					Validators: conditionsOperatorValidators(),
				},
				"value": schema.Float64Attribute{
					Required: true,
				},
				"color": schema.StringAttribute{
					Required:   true,
					Validators: conditionsColorValidators(),
				},
			},
		},
	}
}

func visualizationPieSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	delete(attr.Attributes, "y_axis_settings")
	attr.Attributes["legend_mode"] = legendModeSchema()
	attr.Attributes["custom_link"] = customLinkSchema()
	attr.Attributes["group_by_mode"] = groupByModeSchema()
	return attr
}

func visualizationTopListSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	delete(attr.Attributes, "y_axis_settings")
	attr.Attributes["is_stacked"] = schema.BoolAttribute{
		Optional:    true,
		Description: "Requests stacked rendering for a top-list widget. Tsuga renders stacked rows only for one count or sum query with exactly two grouped fields, no formula, non-negative values, and a single-cluster context; otherwise the widget renders as a normal top list.",
	}
	attr.Attributes["conditions"] = conditionsSchema()
	attr.Attributes["custom_link"] = customLinkSchema()
	attr.Attributes["group_by_mode"] = groupByModeSchema()
	attr.Attributes["legend_mode"] = legendModeSchema()
	return attr
}

func visualizationQueryValueSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	delete(attr.Attributes, "group_by")
	delete(attr.Attributes, "y_axis_settings")
	attr.Attributes["legend_mode"] = legendModeSchema()
	attr.Attributes["background_mode"] = schema.StringAttribute{
		Optional:   true,
		Validators: backgroundModeValidators(),
	}
	attr.Attributes["conditions"] = conditionsSchema()
	return attr
}

func visualizationBarSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	attr.Attributes["legend_mode"] = legendModeSchema()
	attr.Attributes["time_bucket"] = timeBucketSchema()
	attr.Attributes["custom_link"] = customLinkSchema()
	attr.Attributes["group_by_mode"] = groupByModeSchema()
	attr.Attributes["thresholds"] = thresholdsSchema()
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
					Validators:  paletteColorValidators(),
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
		Optional:   true,
		Validators: boundsScaleValidators(),
	}
	attr.Attributes["percentile_markers"] = schema.ListAttribute{
		Optional:    true,
		ElementType: types.Int64Type,
		Description: "Percentile markers (0-100) displayed on top of the distribution chart",
		Validators: []validator.List{
			listvalidator.ValueInt64sAre(percentileMarkerValidators()...),
		},
	}
	attr.Attributes["group"] = groupSchema()
	return attr
}

func groupSchema() schema.Attribute {
	return schema.StringAttribute{
		Optional:    true,
		Description: "Attribute that switches the count to \"Groups\" mode: records are grouped by this attribute, the aggregation produces one value per group, and the chart buckets those per-group values. When omitted, individual records are bucketed.",
		Validators: []validator.String{
			stringvalidator.LengthAtMost(250),
		},
	}
}

func visualizationHeatmapSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	attr.Description = "Displays the aggregation as a heatmap chart"
	attr.Attributes["group"] = groupSchema()
	attr.Attributes["palette"] = schema.StringAttribute{
		Optional:    true,
		Description: "Color palette used to render the heatmap intensity gradient",
		Validators:  paletteColorValidators(),
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
				Validators:  listQueryValidators(),
			},
			"layout": schema.StringAttribute{
				Optional:    true,
				Description: "Layout used to render log patterns",
				Validators:  listLogPatternsLayoutValidators(),
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
		Validators: append(connectionSqlQueriesListValidators(),
			listvalidator.ValueStringsAre(connectionSqlQueryValidators()...),
		),
	}
}

func connectionIdSchema() schema.Attribute {
	return schema.StringAttribute{
		Required:    true,
		Description: "The ID of the connection to use to query the datastore",
		Validators:  connectionIdValidators(),
	}
}

func legendModeSchema() schema.Attribute {
	return schema.StringAttribute{
		Optional:    true,
		Description: "Controls whether and how the widget displays legend or series details; must be table, legend-only, or no-legend",
		Validators:  legendModeValidators(),
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
				Optional:   true,
				Validators: backgroundModeValidators(),
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
				Required:   true,
				Validators: listQueryValidators(),
			},
			"list_columns": listColumnsSchema(),
			"list_columns_size": schema.MapAttribute{
				Optional:    true,
				ElementType: types.Float64Type,
				Description: "Column widths keyed by column id",
			},
			"is_cell_wrapped": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether cell contents wrap instead of being truncated",
			},
			"default_sorting": listDefaultSortingSchema(),
		},
	}
}

func visualizationListSpansSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays individual spans as a tabular list",
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"query": schema.StringAttribute{
				Required:   true,
				Validators: listQueryValidators(),
			},
			"list_columns": listColumnsSchema(),
			"list_columns_size": schema.MapAttribute{
				Optional:    true,
				ElementType: types.Float64Type,
				Description: "Column widths keyed by column id",
			},
			"is_cell_wrapped": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether cell contents wrap instead of being truncated",
			},
			"default_sorting": listDefaultSortingSchema(),
		},
	}
}

func listDefaultSortingSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional:    true,
		Description: "Default sorting applied to the list widget",
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{
					Required:    true,
					Description: "Column attribute used for the default list sort",
					Validators: []validator.String{
						stringvalidator.LengthAtMost(250),
					},
				},
				"desc": schema.BoolAttribute{
					Required:    true,
					Description: "Sort direction: true for descending order, false for ascending",
				},
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
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"value": schema.Float64Attribute{
					Required:    true,
					Description: "Y-axis value where the threshold marker is placed",
				},
				"level": schema.StringAttribute{
					Required:    true,
					Description: "Level applied to the threshold marker",
					Validators:  conditionsColorValidators(),
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
				Validators:  connectionIdValidators(),
			},
			"queries": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Read-only SQL queries to execute against the connection",
				Validators: append(connectionSqlQueriesListValidators(),
					listvalidator.ValueStringsAre(connectionSqlQueryValidators()...),
				),
			},
			"legend_mode":        legendModeSchema(),
			"thresholds":         thresholdsSchema(),
			"y_axis_settings":    yAxisSettingsSchema(),
			"line_style_options": lineStyleOptionsSchema(),
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
				Validators:  connectionIdValidators(),
			},
			"query": schema.StringAttribute{
				Required:    true,
				Description: "Read-only SQL query to execute against the connection",
				Validators:  listConnectionQueryValidators(),
			},
			"list_columns": listColumnsSchema(),
			"list_columns_size": schema.MapAttribute{
				Optional:    true,
				ElementType: types.Float64Type,
				Description: "Column widths keyed by column id",
			},
			"is_cell_wrapped": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether cell contents wrap instead of being truncated",
			},
			"default_sorting": listDefaultSortingSchema(),
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
				Required:   true,
				Validators: noteValidators(),
			},
			"note_align": schema.StringAttribute{
				Optional:   true,
				Validators: flexAlignValidators(),
			},
			"note_justify_content": schema.StringAttribute{
				Optional:   true,
				Validators: flexAlignValidators(),
			},
			"note_color": schema.StringAttribute{
				Optional:   true,
				Validators: noteColorValidators(),
			},
		},
	}
}

func visualizationPromqlSchema(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: description,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"queries": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "PromQL expressions configured for this widget",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.ValueStringsAre(
						stringvalidator.LengthBetween(1, 50000),
					),
				},
			},
			"aliases":    aliasesSchema(),
			"normalizer": normalizer.Schema(),
			"precision": schema.Float64Attribute{
				Optional:    true,
				Description: "Number of decimal places to display in the value",
			},
		},
	}
}

func visualizationTimeseriesPromqlSchema() schema.Attribute {
	attr := visualizationChartPromqlSchema("Displays PromQL metrics query-based aggregation as a time series chart")
	attr.Attributes["line_style_options"] = lineStyleOptionsSchema()
	attr.Attributes["smoothing"] = schema.BoolAttribute{
		Optional:    true,
		Description: "Whether to apply automatic smoothing to the rendered timeseries",
	}
	return attr
}

func visualizationBarPromqlSchema() schema.Attribute {
	return visualizationChartPromqlSchema("Displays PromQL metrics query-based aggregation as a bar chart")
}

func visualizationChartPromqlSchema(description string) schema.SingleNestedAttribute {
	attr := visualizationPromqlSchema(description)
	attr.Attributes["legend_mode"] = legendModeSchema()
	attr.Attributes["thresholds"] = thresholdsSchema()
	attr.Attributes["time_bucket"] = timeBucketSchema()
	attr.Attributes["y_axis_settings"] = yAxisSettingsSchema()
	return attr
}

func visualizationPiePromqlSchema() schema.Attribute {
	attr := visualizationPromqlSchema("Displays PromQL metrics query-based aggregation as a pie chart")
	attr.Attributes["legend_mode"] = legendModeSchema()
	return attr
}

func visualizationQueryValuePromqlSchema() schema.Attribute {
	attr := visualizationPromqlSchema("Displays a single value computed by PromQL metrics queries")
	attr.Attributes["legend_mode"] = legendModeSchema()
	attr.Attributes["background_mode"] = schema.StringAttribute{
		Optional: true,
		Validators: []validator.String{
			stringvalidator.OneOf("background", "no-background"),
		},
	}
	attr.Attributes["conditions"] = conditionsSchema()
	return attr
}

func visualizationTopListPromqlSchema() schema.Attribute {
	attr := visualizationPromqlSchema("Displays PromQL metrics query-based aggregation as a ranked top list")
	attr.Attributes["conditions"] = conditionsSchema()
	return attr
}

func visualizationListRumSchema() schema.Attribute {
	attr := visualizationListSchema().(schema.SingleNestedAttribute)
	attr.Description = "Displays matching RUM events in a tabular list"
	query := attr.Attributes["query"].(schema.StringAttribute)
	query.Description = "Query that selects RUM events for the list. Prefix every field with the stream it belongs to (`measurements.`, `events.`, or `exceptions.`) and filter on a single stream per query. A query without stream-prefixed fields searches the `events` stream."
	attr.Attributes["query"] = query
	return attr
}

func sloIdSchema(required bool, description string) schema.Attribute {
	return schema.StringAttribute{
		Required:    required,
		Optional:    !required,
		Description: description,
		Validators: []validator.String{
			stringvalidator.LengthBetween(1, 250),
		},
	}
}

func sloFilterExcludeSchema() schema.Attribute {
	return schema.BoolAttribute{
		Optional:    true,
		Description: "If true, exclude rows matching these values instead of including them",
	}
}

const sloFilterValuesDescription = "Values compared by this filter. When `exclude` is omitted or false, rows matching any value are kept; when `exclude` is true, matching rows are removed."

func visualizationSloSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays the latest status of SLOs as summary cards with the result count per status, a table of results, or both. Status is not recomputed over the dashboard time range.",
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"slo_id": sloIdSchema(false, "Id of the SLO to display. Omit it to cover every SLO that matches `filters`."),
			"display": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Sections to show: `summary` for the result count per status, `table` for the list of results.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.UniqueValues(),
					listvalidator.ValueStringsAre(
						stringvalidator.OneOf("summary", "table"),
					),
				},
			},
			"organize_by": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Groups the results by owning team, or by the value of a tag key or group-by field. Omit it to list results ungrouped.",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required:    true,
						Description: "`team` groups the results by the team owning their SLO. `key` groups them by the value of `key`.",
						Validators: []validator.String{
							stringvalidator.OneOf("team", "key"),
						},
					},
					"key": schema.StringAttribute{
						Optional:    true,
						Description: "Tag key or group-by field to group by, such as `env` or `context.team`. Required when `type` is `key`, and not allowed otherwise. A result falls under each value it has for this key: its group-by value when its SLO groups by this field, and its SLO's tag values for this key. Results with no value for the key are not shown.",
						Validators: []validator.String{
							stringvalidator.LengthBetween(1, 250),
						},
					},
				},
			},
			"filters": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Narrows the SLOs the widget covers. Omit it, or any of its fields, to apply no filter.",
				Validators: []validator.Object{
					objectvalidator.AtLeastOneOf(
						path.MatchRelative().AtName("owners"),
						path.MatchRelative().AtName("statuses"),
						path.MatchRelative().AtName("tags"),
					),
				},
				Attributes: map[string]schema.Attribute{
					"owners": schema.SingleNestedAttribute{
						Optional:    true,
						Description: "Restrict results to SLOs owned by any of these team IDs",
						Attributes: map[string]schema.Attribute{
							"values": schema.ListAttribute{
								Required:    true,
								ElementType: types.StringType,
								Description: sloFilterValuesDescription,
								Validators: []validator.List{
									listvalidator.SizeAtMost(50),
									listvalidator.ValueStringsAre(
										stringvalidator.LengthBetween(1, 250),
									),
								},
							},
							"exclude": sloFilterExcludeSchema(),
						},
					},
					"statuses": schema.SingleNestedAttribute{
						Optional:    true,
						Description: "Keep only the results whose status is one of these values. A result is one cluster of an ungrouped SLO, or one group of a grouped SLO.",
						Attributes: map[string]schema.Attribute{
							"values": schema.ListAttribute{
								Required:    true,
								ElementType: types.StringType,
								Description: sloFilterValuesDescription + " Statuses: healthy (meeting the target), breached (breaching it), or no_data (not evaluated yet).",
								Validators: []validator.List{
									listvalidator.SizeAtMost(3),
									listvalidator.ValueStringsAre(
										stringvalidator.OneOf("healthy", "breached", "no_data"),
									),
								},
							},
							"exclude": sloFilterExcludeSchema(),
						},
					},
					"tags": schema.SingleNestedAttribute{
						Optional:    true,
						Description: "Restrict results to SLOs that have at least one of the listed key/value tags",
						Attributes: map[string]schema.Attribute{
							"values": schema.ListNestedAttribute{
								Required:    true,
								Description: sloFilterValuesDescription,
								Validators: []validator.List{
									listvalidator.SizeAtMost(50),
								},
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"key": schema.StringAttribute{
											Required:    true,
											Description: "Tag key",
											Validators: []validator.String{
												stringvalidator.LengthAtMost(128),
											},
										},
										"value": schema.StringAttribute{
											Required:    true,
											Description: "Tag value. Leading or trailing whitespace is rejected.",
											Validators: []validator.String{
												stringvalidator.LengthAtMost(256),
												stringvalidator.RegexMatches(regexp.MustCompile(`^\S(.*\S)?$`), "must not have leading or trailing whitespace"),
											},
										},
									},
								},
							},
							"exclude": sloFilterExcludeSchema(),
						},
					},
				},
			},
		},
	}
}

func visualizationSloOverTimeSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays the good and bad events, or the per-slice status, of an SLO over the dashboard time range. Experimental.",
		Attributes: map[string]schema.Attribute{
			"type":   schema.StringAttribute{Computed: true},
			"slo_id": sloIdSchema(true, "Id of the SLO to display"),
		},
	}
}

func visualizationSloUptimeSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Displays the SLI percentage of an SLO computed over the dashboard time range, colored against the SLO target. Experimental.",
		Attributes: map[string]schema.Attribute{
			"type":   schema.StringAttribute{Computed: true},
			"slo_id": sloIdSchema(true, "Id of the SLO to display"),
		},
	}
}

type DashboardModel struct {
	Id         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Owner      types.String `tfsdk:"owner"`
	FolderId   types.String `tfsdk:"folder_id"`
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
	ListSpans            *ListVisualization                 `tfsdk:"list_spans"`
	ListLogPatterns      *ListLogPatternsVisualization      `tfsdk:"list_log_patterns"`
	Note                 *NoteVisualizationModel            `tfsdk:"note"`
	Table                *TableVisualizationModel           `tfsdk:"table"`
	TimeseriesConnection *TimeseriesConnectionVisualization `tfsdk:"timeseries_connection"`
	ListConnection       *ListConnectionVisualization       `tfsdk:"list_connection"`
	TopListConnection    *TopListConnectionVisualization    `tfsdk:"top_list_connection"`
	PieConnection        *PieConnectionVisualization        `tfsdk:"pie_connection"`
	BarConnection        *BarConnectionVisualization        `tfsdk:"bar_connection"`
	QueryValueConnection *QueryValueConnectionVisualization `tfsdk:"query_value_connection"`
	TimeseriesPromql     *TimeseriesPromqlVisualization     `tfsdk:"timeseries_promql"`
	BarPromql            *BarPromqlVisualization            `tfsdk:"bar_promql"`
	PiePromql            *PiePromqlVisualization            `tfsdk:"pie_promql"`
	QueryValuePromql     *QueryValuePromqlVisualization     `tfsdk:"query_value_promql"`
	TopListPromql        *TopListPromqlVisualization        `tfsdk:"top_list_promql"`
	ListRum              *ListVisualization                 `tfsdk:"list_rum"`
	Slo                  *SloVisualization                  `tfsdk:"slo"`
	SloOverTime          *SloIdVisualization                `tfsdk:"slo_over_time"`
	SloUptime            *SloIdVisualization                `tfsdk:"slo_uptime"`
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
}

type TimeseriesVisualization struct {
	SeriesVisualizationModel
	LegendMode       types.String     `tfsdk:"legend_mode"`
	TimeBucket       *TimeBucketModel `tfsdk:"time_bucket"`
	Smoothing        types.Bool       `tfsdk:"smoothing"`
	CustomLink       *CustomLinkModel `tfsdk:"custom_link"`
	GroupByMode      types.String     `tfsdk:"group_by_mode"`
	LineStyleOptions types.Map        `tfsdk:"line_style_options"`
	Thresholds       types.List       `tfsdk:"thresholds"`
}

type CustomLinkModel struct {
	Label types.String `tfsdk:"label"`
	Url   types.String `tfsdk:"url"`
}

type LineStyleOptionModel struct {
	GroupId   types.String `tfsdk:"group_id"`
	LineStyle types.String `tfsdk:"line_style"`
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
	Type           types.String     `tfsdk:"type"`
	Columns        types.List       `tfsdk:"columns"`
	GroupBy        types.List       `tfsdk:"group_by"`
	GroupByMode    types.String     `tfsdk:"group_by_mode"`
	CustomLink     *CustomLinkModel `tfsdk:"custom_link"`
	ColumnSizes    types.Map        `tfsdk:"column_sizes"`
	DefaultSorting types.List       `tfsdk:"default_sorting"`
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
	LegendMode     types.String `tfsdk:"legend_mode"`
	BackgroundMode types.String `tfsdk:"background_mode"`
	Conditions     types.List   `tfsdk:"conditions"`
}

type PieVisualization struct {
	SeriesBase
	LegendMode  types.String     `tfsdk:"legend_mode"`
	GroupBy     types.List       `tfsdk:"group_by"`
	CustomLink  *CustomLinkModel `tfsdk:"custom_link"`
	GroupByMode types.String     `tfsdk:"group_by_mode"`
}

type TopListVisualization struct {
	SeriesBase
	GroupBy     types.List       `tfsdk:"group_by"`
	Conditions  types.List       `tfsdk:"conditions"`
	IsStacked   types.Bool       `tfsdk:"is_stacked"`
	CustomLink  *CustomLinkModel `tfsdk:"custom_link"`
	GroupByMode types.String     `tfsdk:"group_by_mode"`
	LegendMode  types.String     `tfsdk:"legend_mode"`
}

type BarVisualization struct {
	SeriesVisualizationModel
	LegendMode  types.String     `tfsdk:"legend_mode"`
	TimeBucket  *TimeBucketModel `tfsdk:"time_bucket"`
	CustomLink  *CustomLinkModel `tfsdk:"custom_link"`
	GroupByMode types.String     `tfsdk:"group_by_mode"`
	Thresholds  types.List       `tfsdk:"thresholds"`
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
	Group             types.String `tfsdk:"group"`
}

type HeatmapVisualization struct {
	SeriesBase
	GroupBy       types.List          `tfsdk:"group_by"`
	Palette       types.String        `tfsdk:"palette"`
	Group         types.String        `tfsdk:"group"`
	YAxisSettings *YAxisSettingsModel `tfsdk:"y_axis_settings"`
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
	DefaultSorting  types.List   `tfsdk:"default_sorting"`
	IsCellWrapped   types.Bool   `tfsdk:"is_cell_wrapped"`
}

type ListDefaultSortingModel struct {
	Id   types.String `tfsdk:"id"`
	Desc types.Bool   `tfsdk:"desc"`
}

type TimeseriesConnectionVisualization struct {
	Type             types.String        `tfsdk:"type"`
	ConnectionId     types.String        `tfsdk:"connection_id"`
	Queries          types.List          `tfsdk:"queries"`
	LegendMode       types.String        `tfsdk:"legend_mode"`
	Thresholds       types.List          `tfsdk:"thresholds"`
	YAxisSettings    *YAxisSettingsModel `tfsdk:"y_axis_settings"`
	LineStyleOptions types.Map           `tfsdk:"line_style_options"`
}

type PromqlBase struct {
	Type       types.String      `tfsdk:"type"`
	Queries    types.List        `tfsdk:"queries"`
	Aliases    *AliasesModel     `tfsdk:"aliases"`
	Normalizer *normalizer.Model `tfsdk:"normalizer"`
	Precision  types.Float64     `tfsdk:"precision"`
}

type TimeseriesPromqlVisualization struct {
	BarPromqlVisualization
	LineStyleOptions types.Map  `tfsdk:"line_style_options"`
	Smoothing        types.Bool `tfsdk:"smoothing"`
}

type BarPromqlVisualization struct {
	PromqlBase
	LegendMode    types.String        `tfsdk:"legend_mode"`
	Thresholds    types.List          `tfsdk:"thresholds"`
	TimeBucket    *TimeBucketModel    `tfsdk:"time_bucket"`
	YAxisSettings *YAxisSettingsModel `tfsdk:"y_axis_settings"`
}

type PiePromqlVisualization struct {
	PromqlBase
	LegendMode types.String `tfsdk:"legend_mode"`
}

type QueryValuePromqlVisualization struct {
	PromqlBase
	LegendMode     types.String `tfsdk:"legend_mode"`
	BackgroundMode types.String `tfsdk:"background_mode"`
	Conditions     types.List   `tfsdk:"conditions"`
}

type TopListPromqlVisualization struct {
	PromqlBase
	Conditions types.List `tfsdk:"conditions"`
}

type SloVisualization struct {
	Type       types.String        `tfsdk:"type"`
	SloId      types.String        `tfsdk:"slo_id"`
	Display    types.List          `tfsdk:"display"`
	OrganizeBy *SloOrganizeByModel `tfsdk:"organize_by"`
	Filters    *SloFiltersModel    `tfsdk:"filters"`
}

type SloOrganizeByModel struct {
	Type types.String `tfsdk:"type"`
	Key  types.String `tfsdk:"key"`
}

type SloFiltersModel struct {
	Owners   *SloFilterModel `tfsdk:"owners"`
	Statuses *SloFilterModel `tfsdk:"statuses"`
	Tags     *SloFilterModel `tfsdk:"tags"`
}

// SloFilterModel is one SLO widget filter; values holds strings for owners and
// statuses and key/value objects for tags.
type SloFilterModel struct {
	Values  types.List `tfsdk:"values"`
	Exclude types.Bool `tfsdk:"exclude"`
}

type SloIdVisualization struct {
	Type  types.String `tfsdk:"type"`
	SloId types.String `tfsdk:"slo_id"`
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
	DefaultSorting  types.List   `tfsdk:"default_sorting"`
	IsCellWrapped   types.Bool   `tfsdk:"is_cell_wrapped"`
}

type NoteVisualizationModel struct {
	Type               types.String `tfsdk:"type"`
	Note               types.String `tfsdk:"note"`
	NoteAlign          types.String `tfsdk:"note_align"`
	NoteJustifyContent types.String `tfsdk:"note_justify_content"`
	NoteColor          types.String `tfsdk:"note_color"`
}

type QueryModel struct {
	Aggregate     AggregateModel  `tfsdk:"aggregate"`
	Filter        types.String    `tfsdk:"filter"`
	Functions     types.List      `tfsdk:"functions"`
	TimeAggregate types.String    `tfsdk:"time_aggregate"`
	MetricKind    types.String    `tfsdk:"metric_kind"`
	Fill          *QueryFillModel `tfsdk:"fill"`
}

type QueryFillModel struct {
	Mode QueryFillModeModel `tfsdk:"mode"`
}

type QueryFillModeModel struct {
	Type types.String `tfsdk:"type"`
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
	Key     types.String `tfsdk:"key"`
	Values  types.List   `tfsdk:"values"`
	Exclude types.Bool   `tfsdk:"exclude"`
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
		"key":     types.StringType,
		"values":  types.ListType{ElemType: types.StringType},
		"exclude": types.BoolType,
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
		"list_spans":             types.ObjectType{AttrTypes: ListVisualizationAttrTypes()},
		"list_log_patterns":      types.ObjectType{AttrTypes: ListLogPatternsVisualizationAttrTypes()},
		"note":                   types.ObjectType{AttrTypes: NoteVisualizationAttrTypes()},
		"table":                  types.ObjectType{AttrTypes: TableVisualizationAttrTypes()},
		"timeseries_connection":  types.ObjectType{AttrTypes: TimeseriesConnectionVisualizationAttrTypes()},
		"list_connection":        types.ObjectType{AttrTypes: ListConnectionVisualizationAttrTypes()},
		"top_list_connection":    types.ObjectType{AttrTypes: TopListConnectionVisualizationAttrTypes()},
		"pie_connection":         types.ObjectType{AttrTypes: PieConnectionVisualizationAttrTypes()},
		"bar_connection":         types.ObjectType{AttrTypes: BarConnectionVisualizationAttrTypes()},
		"query_value_connection": types.ObjectType{AttrTypes: QueryValueConnectionVisualizationAttrTypes()},
		"timeseries_promql":      types.ObjectType{AttrTypes: TimeseriesPromqlVisualizationAttrTypes()},
		"bar_promql":             types.ObjectType{AttrTypes: BarPromqlVisualizationAttrTypes()},
		"pie_promql":             types.ObjectType{AttrTypes: PiePromqlVisualizationAttrTypes()},
		"query_value_promql":     types.ObjectType{AttrTypes: QueryValuePromqlVisualizationAttrTypes()},
		"top_list_promql":        types.ObjectType{AttrTypes: TopListPromqlVisualizationAttrTypes()},
		"list_rum":               types.ObjectType{AttrTypes: ListVisualizationAttrTypes()},
		"slo":                    types.ObjectType{AttrTypes: SloVisualizationAttrTypes()},
		"slo_over_time":          types.ObjectType{AttrTypes: SloIdVisualizationAttrTypes()},
		"slo_uptime":             types.ObjectType{AttrTypes: SloIdVisualizationAttrTypes()},
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
	attrs["legend_mode"] = types.StringType
	attrs["time_bucket"] = types.ObjectType{AttrTypes: TimeBucketAttrTypes()}
	attrs["smoothing"] = types.BoolType
	attrs["custom_link"] = types.ObjectType{AttrTypes: CustomLinkAttrTypes()}
	attrs["group_by_mode"] = types.StringType
	attrs["line_style_options"] = LineStyleOptionsType()
	attrs["thresholds"] = ThresholdsType()
	return attrs
}

func CustomLinkAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"label": types.StringType,
		"url":   types.StringType,
	}
}

func LineStyleOptionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"group_id":   types.StringType,
		"line_style": types.StringType,
	}
}

func LineStyleOptionsType() types.MapType {
	return types.MapType{ElemType: types.ObjectType{AttrTypes: LineStyleOptionAttrTypes()}}
}

func ThresholdsType() types.ListType {
	return types.ListType{ElemType: types.ObjectType{AttrTypes: ThresholdAttrTypes()}}
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
		"type":            types.StringType,
		"columns":         types.ListType{ElemType: types.ObjectType{AttrTypes: TableColumnAttrTypes()}},
		"group_by":        types.ListType{ElemType: types.ObjectType{AttrTypes: groupby.AttrTypes()}},
		"group_by_mode":   types.StringType,
		"custom_link":     types.ObjectType{AttrTypes: CustomLinkAttrTypes()},
		"column_sizes":    types.MapType{ElemType: types.Float64Type},
		"default_sorting": types.ListType{ElemType: types.ObjectType{AttrTypes: ListDefaultSortingAttrTypes()}},
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
	attrs["custom_link"] = types.ObjectType{AttrTypes: CustomLinkAttrTypes()}
	attrs["group_by_mode"] = types.StringType
	return attrs
}

func QueryValueVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	delete(attrs, "group_by")
	delete(attrs, "y_axis_settings")
	attrs["legend_mode"] = types.StringType
	attrs["background_mode"] = types.StringType
	attrs["conditions"] = types.ListType{ElemType: types.ObjectType{AttrTypes: ConditionAttrTypes()}}
	return attrs
}

func TopListVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	delete(attrs, "y_axis_settings")
	attrs["is_stacked"] = types.BoolType
	attrs["conditions"] = types.ListType{ElemType: types.ObjectType{AttrTypes: ConditionAttrTypes()}}
	attrs["custom_link"] = types.ObjectType{AttrTypes: CustomLinkAttrTypes()}
	attrs["group_by_mode"] = types.StringType
	attrs["legend_mode"] = types.StringType
	return attrs
}

func BarVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	attrs["legend_mode"] = types.StringType
	attrs["time_bucket"] = types.ObjectType{AttrTypes: TimeBucketAttrTypes()}
	attrs["custom_link"] = types.ObjectType{AttrTypes: CustomLinkAttrTypes()}
	attrs["group_by_mode"] = types.StringType
	attrs["thresholds"] = ThresholdsType()
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
	attrs["group"] = types.StringType
	return attrs
}

func HeatmapVisualizationAttrTypes() map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes()
	attrs["palette"] = types.StringType
	attrs["group"] = types.StringType
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
		"thresholds":      ThresholdsType(),
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
		"default_sorting":   types.ListType{ElemType: types.ObjectType{AttrTypes: ListDefaultSortingAttrTypes()}},
		"is_cell_wrapped":   types.BoolType,
	}
}

func ListDefaultSortingAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":   types.StringType,
		"desc": types.BoolType,
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
		"type":               types.StringType,
		"connection_id":      types.StringType,
		"queries":            types.ListType{ElemType: types.StringType},
		"legend_mode":        types.StringType,
		"thresholds":         ThresholdsType(),
		"y_axis_settings":    types.ObjectType{AttrTypes: YAxisSettingsAttrTypes()},
		"line_style_options": LineStyleOptionsType(),
	}
}

func TimeseriesPromqlVisualizationAttrTypes() map[string]attr.Type {
	return visualizationTimeseriesPromqlSchema().GetType().(types.ObjectType).AttrTypes
}

func BarPromqlVisualizationAttrTypes() map[string]attr.Type {
	return visualizationBarPromqlSchema().GetType().(types.ObjectType).AttrTypes
}

func PiePromqlVisualizationAttrTypes() map[string]attr.Type {
	return visualizationPiePromqlSchema().GetType().(types.ObjectType).AttrTypes
}

func QueryValuePromqlVisualizationAttrTypes() map[string]attr.Type {
	return visualizationQueryValuePromqlSchema().GetType().(types.ObjectType).AttrTypes
}

func TopListPromqlVisualizationAttrTypes() map[string]attr.Type {
	return visualizationTopListPromqlSchema().GetType().(types.ObjectType).AttrTypes
}

func SloVisualizationAttrTypes() map[string]attr.Type {
	return visualizationSloSchema().GetType().(types.ObjectType).AttrTypes
}

func SloFiltersAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"owners":   types.ObjectType{AttrTypes: SloFilterAttrTypes(types.StringType)},
		"statuses": types.ObjectType{AttrTypes: SloFilterAttrTypes(types.StringType)},
		"tags":     types.ObjectType{AttrTypes: SloFilterAttrTypes(types.ObjectType{AttrTypes: SloTagAttrTypes()})},
	}
}

func SloFilterAttrTypes(valueType attr.Type) map[string]attr.Type {
	return map[string]attr.Type{
		"values":  types.ListType{ElemType: valueType},
		"exclude": types.BoolType,
	}
}

func SloTagAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key":   types.StringType,
		"value": types.StringType,
	}
}

func SloOrganizeByAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"key":  types.StringType,
	}
}

func SloIdVisualizationAttrTypes() map[string]attr.Type {
	return visualizationSloOverTimeSchema().GetType().(types.ObjectType).AttrTypes
}

func ListConnectionVisualizationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":              types.StringType,
		"connection_id":     types.StringType,
		"query":             types.StringType,
		"list_columns":      types.ListType{ElemType: types.ObjectType{AttrTypes: ListColumnAttrTypes()}},
		"list_columns_size": types.MapType{ElemType: types.Float64Type},
		"default_sorting":   types.ListType{ElemType: types.ObjectType{AttrTypes: ListDefaultSortingAttrTypes()}},
		"is_cell_wrapped":   types.BoolType,
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
		"aggregate":      types.ObjectType{AttrTypes: aggregate.AttrTypes()},
		"filter":         types.StringType,
		"functions":      types.ListType{ElemType: types.ObjectType{AttrTypes: FunctionAttrTypes()}},
		"time_aggregate": types.StringType,
		"metric_kind":    types.StringType,
		"fill":           types.ObjectType{AttrTypes: QueryFillAttrTypes()},
	}
}

func QueryFillAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"mode": types.ObjectType{AttrTypes: QueryFillModeAttrTypes()},
	}
}

func QueryFillModeAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
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
