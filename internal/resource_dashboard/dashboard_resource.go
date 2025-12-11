package resource_dashboard

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-tsuga/internal/resource_team"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
			"filters": schema.ListAttribute{
				Optional:    true,
				Description: "Filters applied to every widget on the dashboard",
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtMost(10),
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
								"timeseries":  visualizationSeriesSchema(),
								"top_list":    visualizationSeriesSchema(),
								"pie":         visualizationSeriesSchema(),
								"query_value": visualizationQueryValueSchema(),
								"bar":         visualizationBarSchema(),
								"list":        visualizationListSchema(),
								"note":        visualizationNoteSchema(),
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
			"queries": schema.ListNestedAttribute{
				Required: true,
				Validators: []validator.List{
					listvalidator.SizeAtMost(10),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"aggregate": aggregateSchema(),
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
											stringvalidator.OneOf("per-second", "per-minute", "per-hour", "rate", "rolling"),
										},
									},
									"window": schema.StringAttribute{
										Optional: true,
									},
								},
							},
						},
					},
				},
			},
			"formula": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"visible_series": schema.ListAttribute{
				Optional:    true,
				ElementType: types.BoolType,
			},
			"group_by": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtMost(3),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"fields": schema.ListAttribute{
							Required:    true,
							ElementType: types.StringType,
						},
						"limit": schema.Int64Attribute{
							Required: true,
						},
					},
				},
			},
			"normalizer": normalizerSchema(),
		},
	}
}

func visualizationQueryValueSchema() schema.Attribute {
	attr := visualizationSeriesSchema().(schema.SingleNestedAttribute)
	attr.Attributes["background_mode"] = schema.StringAttribute{
		Optional: true,
		Validators: []validator.String{
			stringvalidator.OneOf("background", "no-background"),
		},
	}
	attr.Attributes["conditions"] = schema.ListNestedAttribute{
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
	return attr
}

func visualizationListSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Computed: true,
			},
			"source": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("logs"),
				},
			},
			"query": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(10000),
				},
			},
			"list_columns": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"attribute": schema.StringAttribute{
							Required: true,
						},
						"normalizer": normalizerSchema(),
					},
				},
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

func aggregateSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"count": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Computed: true,
					},
				},
			},
			"sum": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Computed: true,
					},
					"field": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.LengthAtMost(250),
						},
					},
				},
			},
			"average": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Computed: true,
					},
					"field": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.LengthAtMost(250),
						},
					},
				},
			},
			"min": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Computed: true,
					},
					"field": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.LengthAtMost(250),
						},
					},
				},
			},
			"max": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Computed: true,
					},
					"field": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.LengthAtMost(250),
						},
					},
				},
			},
			"unique_count": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Computed: true,
					},
					"field": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.LengthAtMost(250),
						},
					},
				},
			},
			"percentile": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Computed: true,
					},
					"field": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.LengthAtMost(250),
						},
					},
					"percentile": schema.Float64Attribute{
						Required: true,
					},
				},
			},
		},
	}
}

func normalizerSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("duration", "data", "custom", "date", "level"),
				},
			},
			"unit": schema.StringAttribute{
				Optional: true,
			},
		},
	}
}

type DashboardModel struct {
	Id      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Owner   types.String `tfsdk:"owner"`
	Filters types.List   `tfsdk:"filters"`
	Tags    types.List   `tfsdk:"tags"`
	Graphs  types.List   `tfsdk:"graphs"`
}

type GraphModel struct {
	Id            types.String       `tfsdk:"id"`
	Name          types.String       `tfsdk:"name"`
	Layout        *GraphLayoutModel  `tfsdk:"layout"`
	Visualization VisualizationModel `tfsdk:"visualization"`
}

type GraphLayoutModel struct {
	X types.Float64 `tfsdk:"x"`
	Y types.Float64 `tfsdk:"y"`
	W types.Float64 `tfsdk:"w"`
	H types.Float64 `tfsdk:"h"`
}

type VisualizationModel struct {
	Timeseries *SeriesVisualizationModel `tfsdk:"timeseries"`
	TopList    *SeriesVisualizationModel `tfsdk:"top_list"`
	Pie        *SeriesVisualizationModel `tfsdk:"pie"`
	QueryValue *QueryValueVisualization  `tfsdk:"query_value"`
	Bar        *BarVisualization         `tfsdk:"bar"`
	List       *ListVisualization        `tfsdk:"list"`
	Note       *NoteVisualizationModel   `tfsdk:"note"`
}

type SeriesVisualizationModel struct {
	Type          types.String     `tfsdk:"type"`
	Source        types.String     `tfsdk:"source"`
	Queries       types.List       `tfsdk:"queries"`
	Formula       types.String     `tfsdk:"formula"`
	VisibleSeries types.List       `tfsdk:"visible_series"`
	GroupBy       types.List       `tfsdk:"group_by"`
	Normalizer    *NormalizerModel `tfsdk:"normalizer"`
}

type QueryValueVisualization struct {
	SeriesVisualizationModel
	BackgroundMode types.String `tfsdk:"background_mode"`
	Conditions     types.List   `tfsdk:"conditions"`
}

type BarVisualization struct {
	SeriesVisualizationModel
	TimeBucket *TimeBucketModel `tfsdk:"time_bucket"`
}

type ListVisualization struct {
	Type        types.String `tfsdk:"type"`
	Source      types.String `tfsdk:"source"`
	Query       types.String `tfsdk:"query"`
	ListColumns types.List   `tfsdk:"list_columns"`
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
	Count      *AggregateCountModel      `tfsdk:"count"`
	Sum        *AggregateFieldModel      `tfsdk:"sum"`
	Average    *AggregateFieldModel      `tfsdk:"average"`
	Min        *AggregateFieldModel      `tfsdk:"min"`
	Max        *AggregateFieldModel      `tfsdk:"max"`
	Uniq       *AggregateFieldModel      `tfsdk:"unique_count"`
	Percentile *AggregatePercentileModel `tfsdk:"percentile"`
}

type AggregateCountModel struct {
	Type types.String `tfsdk:"type"`
}

type AggregateFieldModel struct {
	Type  types.String `tfsdk:"type"`
	Field types.String `tfsdk:"field"`
}

type AggregatePercentileModel struct {
	Type       types.String  `tfsdk:"type"`
	Field      types.String  `tfsdk:"field"`
	Percentile types.Float64 `tfsdk:"percentile"`
}

type FunctionModel struct {
	Type   types.String `tfsdk:"type"`
	Window types.String `tfsdk:"window"`
}

type GroupByModel struct {
	Fields types.List  `tfsdk:"fields"`
	Limit  types.Int64 `tfsdk:"limit"`
}

type NormalizerModel struct {
	Type types.String `tfsdk:"type"`
	Unit types.String `tfsdk:"unit"`
}

type ConditionModel struct {
	Operator types.String  `tfsdk:"operator"`
	Value    types.Float64 `tfsdk:"value"`
	Color    types.String  `tfsdk:"color"`
}

type TimeBucketModel struct {
	Time   types.Float64 `tfsdk:"time"`
	Metric types.String  `tfsdk:"metric"`
}

type ListColumnModel struct {
	Attribute  types.String     `tfsdk:"attribute"`
	Normalizer *NormalizerModel `tfsdk:"normalizer"`
}

func GraphAttrTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"id":            types.StringType,
		"name":          types.StringType,
		"layout":        types.ObjectType{AttrTypes: GraphLayoutAttrTypes(ctx)},
		"visualization": types.ObjectType{AttrTypes: VisualizationAttrTypes(ctx)},
	}
}

func GraphLayoutAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"x": types.Float64Type,
		"y": types.Float64Type,
		"w": types.Float64Type,
		"h": types.Float64Type,
	}
}

func VisualizationAttrTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"timeseries":  types.ObjectType{AttrTypes: SeriesVisualizationAttrTypes(ctx)},
		"top_list":    types.ObjectType{AttrTypes: SeriesVisualizationAttrTypes(ctx)},
		"pie":         types.ObjectType{AttrTypes: SeriesVisualizationAttrTypes(ctx)},
		"query_value": types.ObjectType{AttrTypes: QueryValueVisualizationAttrTypes(ctx)},
		"bar":         types.ObjectType{AttrTypes: BarVisualizationAttrTypes(ctx)},
		"list":        types.ObjectType{AttrTypes: ListVisualizationAttrTypes(ctx)},
		"note":        types.ObjectType{AttrTypes: NoteVisualizationAttrTypes(ctx)},
	}
}

func SeriesVisualizationAttrTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"type":           types.StringType,
		"source":         types.StringType,
		"queries":        types.ListType{ElemType: types.ObjectType{AttrTypes: QueryAttrTypes(ctx)}},
		"formula":        types.StringType,
		"visible_series": types.ListType{ElemType: types.BoolType},
		"group_by":       types.ListType{ElemType: types.ObjectType{AttrTypes: GroupByAttrTypes()}},
		"normalizer":     types.ObjectType{AttrTypes: NormalizerAttrTypes(ctx)},
	}
}

func QueryValueVisualizationAttrTypes(ctx context.Context) map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes(ctx)
	attrs["background_mode"] = types.StringType
	attrs["conditions"] = types.ListType{ElemType: types.ObjectType{AttrTypes: ConditionAttrTypes(ctx)}}
	return attrs
}

func BarVisualizationAttrTypes(ctx context.Context) map[string]attr.Type {
	attrs := SeriesVisualizationAttrTypes(ctx)
	attrs["time_bucket"] = types.ObjectType{AttrTypes: TimeBucketAttrTypes(ctx)}
	return attrs
}

func ListVisualizationAttrTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"type":         types.StringType,
		"source":       types.StringType,
		"query":        types.StringType,
		"list_columns": types.ListType{ElemType: types.ObjectType{AttrTypes: ListColumnAttrTypes(ctx)}},
	}
}

func NoteVisualizationAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"type":                 types.StringType,
		"note":                 types.StringType,
		"note_align":           types.StringType,
		"note_justify_content": types.StringType,
		"note_color":           types.StringType,
	}
}

func QueryAttrTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"aggregate": types.ObjectType{AttrTypes: AggregateAttrTypes(ctx)},
		"filter":    types.StringType,
		"functions": types.ListType{ElemType: types.ObjectType{AttrTypes: FunctionAttrTypes()}},
	}
}

func AggregateAttrTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"count":        types.ObjectType{AttrTypes: AggregateCountAttrTypes(ctx)},
		"sum":          types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"average":      types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"min":          types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"max":          types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"unique_count": types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"percentile":   types.ObjectType{AttrTypes: AggregatePercentileAttrTypes()},
	}
}

func AggregateCountAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
	}
}

func AggregateFieldAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":  types.StringType,
		"field": types.StringType,
	}
}

func AggregatePercentileAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":       types.StringType,
		"field":      types.StringType,
		"percentile": types.Float64Type,
	}
}

func FunctionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":   types.StringType,
		"window": types.StringType,
	}
}

func GroupByAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"fields": types.ListType{ElemType: types.StringType},
		"limit":  types.Int64Type,
	}
}

func NormalizerAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"unit": types.StringType,
	}
}

func ConditionAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"operator": types.StringType,
		"value":    types.Float64Type,
		"color":    types.StringType,
	}
}

func TimeBucketAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"time":   types.Float64Type,
		"metric": types.StringType,
	}
}

func ListColumnAttrTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"attribute":  types.StringType,
		"normalizer": types.ObjectType{AttrTypes: NormalizerAttrTypes(ctx)},
	}
}
