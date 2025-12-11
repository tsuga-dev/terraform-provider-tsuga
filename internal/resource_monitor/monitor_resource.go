package resource_monitor

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-tsuga/internal/resource_team"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func MonitorResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "Monitor allowing to send alerts based on telemetry data",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the monitor",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the monitor",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"message": schema.StringAttribute{
				Optional:    true,
				Description: "Message to be displayed if a notification is triggered",
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
			"configuration": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Monitor configuration",
				Attributes: map[string]schema.Attribute{
					"metric":         monitorConfigurationMetricSchema(),
					"log":            monitorConfigurationLogSchema(),
					"anomaly_log":    monitorConfigurationAnomalyLogSchema(),
					"anomaly_metric": monitorConfigurationAnomalyMetricSchema(),
				},
			},
			"priority": schema.Int64Attribute{
				Required:    true,
				Description: "Priority of the monitor (1-5)",
				Validators: []validator.Int64{
					int64validator.Between(1, 5),
				},
			},
			"owner": schema.StringAttribute{
				Required:    true,
				Description: "Team ID that owns and manages the monitor",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"dashboard_id": schema.StringAttribute{
				Optional:    true,
				Description: "Identifier of a dashboard related to the monitor",
			},
			"permissions": schema.StringAttribute{
				Required:    true,
				Description: "This controls which data the monitor can see",
				Validators: []validator.String{
					stringvalidator.OneOf("all", "owning-team-only", "owning-team-and-public"),
				},
			},
		},
	}
}

// Helper functions for common schema parts

func baseMonitorConfigurationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"timeframe": schema.Int64Attribute{
			Required:    true,
			Description: "Timeframe of the monitor in minutes",
		},
		"group_by_fields": schema.ListNestedAttribute{
			Required: true,
			Validators: []validator.List{
				listvalidator.SizeAtMost(1),
			},
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"fields": schema.ListAttribute{
						Required:    true,
						ElementType: types.StringType,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
					},
					"limit": schema.Int64Attribute{
						Required: true,
					},
				},
			},
		},
		"aggregation_alert_logic": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.OneOf("no_aggregation", "all", "any", "each", "proportion"),
			},
		},
		"proportion_alert_threshold": schema.Int64Attribute{
			Optional: true,
			Validators: []validator.Int64{
				int64validator.Between(1, 99),
			},
		},
	}
}

func monitorConditionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"formula": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"operator": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("greater_than", "less_than", "equal", "not_equal", "greater_than_or_equal", "less_than_or_equal"),
				},
			},
			"threshold": schema.Float64Attribute{
				Required: true,
			},
		},
	}
}

func monitorAnomalyConditionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"formula": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"condition_type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("rate", "error", "cpu", "general", "to_be_set"),
				},
			},
		},
	}
}

func monitorNoDataBehaviorSchema(includeConsiderZero bool) schema.Attribute {
	validators := []validator.String{
		stringvalidator.OneOf("alert", "resolve", "keep_last_status"),
	}
	if includeConsiderZero {
		validators = []validator.String{
			stringvalidator.OneOf("alert", "resolve", "keep_last_status", "consider_zero"),
		}
	}
	return schema.StringAttribute{
		Required:   true,
		Validators: validators,
	}
}

func metricQueriesSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Required: true,
		Validators: []validator.List{
			listvalidator.SizeBetween(1, 5),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"name": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(250),
					},
				},
				"filter": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(10000),
					},
				},
				"aggregate": metricAggregateSchema(),
				"value_if_no_data": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Validators: []validator.String{
						stringvalidator.OneOf("Zero", "NaN"),
					},
				},
			},
		},
	}
}

func logQueriesSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Required: true,
		Validators: []validator.List{
			listvalidator.SizeBetween(1, 5),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"name": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(250),
					},
				},
				"filter": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(10000),
					},
				},
				"aggregate": logAggregateSchema(),
				"value_if_no_data": schema.StringAttribute{
					Optional: true,
					Computed: true,
					Validators: []validator.String{
						stringvalidator.OneOf("Zero", "NaN"),
					},
				},
			},
		},
	}
}

func monitorConfigurationMetricSchema() schema.Attribute {
	attrs := baseMonitorConfigurationAttributes()
	attrs["condition"] = monitorConditionSchema()
	attrs["no_data_behavior"] = monitorNoDataBehaviorSchema(true)
	attrs["queries"] = metricQueriesSchema()
	return schema.SingleNestedAttribute{
		Optional:   true,
		Attributes: attrs,
	}
}

func monitorConfigurationLogSchema() schema.Attribute {
	attrs := baseMonitorConfigurationAttributes()
	attrs["condition"] = monitorConditionSchema()
	attrs["no_data_behavior"] = monitorNoDataBehaviorSchema(true)
	attrs["queries"] = logQueriesSchema()
	return schema.SingleNestedAttribute{
		Optional:   true,
		Attributes: attrs,
	}
}

func monitorConfigurationAnomalyLogSchema() schema.Attribute {
	attrs := baseMonitorConfigurationAttributes()
	attrs["condition"] = monitorAnomalyConditionSchema()
	attrs["no_data_behavior"] = monitorNoDataBehaviorSchema(false)
	attrs["queries"] = logQueriesSchema()
	return schema.SingleNestedAttribute{
		Optional:   true,
		Attributes: attrs,
	}
}

func monitorConfigurationAnomalyMetricSchema() schema.Attribute {
	attrs := baseMonitorConfigurationAttributes()
	attrs["condition"] = monitorAnomalyConditionSchema()
	attrs["no_data_behavior"] = monitorNoDataBehaviorSchema(false)
	attrs["queries"] = metricQueriesSchema()
	return schema.SingleNestedAttribute{
		Optional:   true,
		Attributes: attrs,
	}
}

func metricAggregateSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required:    true,
		Description: "Metric aggregate (unique_count, average, max, min, sum, or percentile)",
		Attributes: map[string]schema.Attribute{
			"average":      aggregateFieldSchema(),
			"max":          aggregateFieldSchema(),
			"min":          aggregateFieldSchema(),
			"sum":          aggregateFieldSchema(),
			"percentile":   aggregatePercentileSchema(),
			"unique_count": aggregateFieldSchema(),
		},
	}
}

func logAggregateSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required:    true,
		Description: "Log aggregate (count, unique_count, average, max, min, sum, or percentile)",
		Attributes: map[string]schema.Attribute{
			"average":      aggregateFieldSchema(),
			"count":        aggregateCountSchema(),
			"max":          aggregateFieldSchema(),
			"min":          aggregateFieldSchema(),
			"sum":          aggregateFieldSchema(),
			"percentile":   aggregatePercentileSchema(),
			"unique_count": aggregateFieldSchema(),
		},
	}
}

// Aggregate helper schemas

func aggregateFieldSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"field": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
		},
	}
}

func aggregatePercentileSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
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
	}
}

func aggregateCountSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:   true,
		Attributes: map[string]schema.Attribute{},
	}
}

// Model types
type MonitorModel struct {
	Id            types.String              `tfsdk:"id"`
	Name          types.String              `tfsdk:"name"`
	Message       types.String              `tfsdk:"message"`
	Tags          types.List                `tfsdk:"tags"`
	Configuration MonitorConfigurationModel `tfsdk:"configuration"`
	Priority      types.Int64               `tfsdk:"priority"`
	Owner         types.String              `tfsdk:"owner"`
	DashboardId   types.String              `tfsdk:"dashboard_id"`
	Permissions   types.String              `tfsdk:"permissions"`
}

type MonitorConfigurationModel struct {
	Metric        *MonitorConfigurationMetricModel        `tfsdk:"metric"`
	Log           *MonitorConfigurationLogModel           `tfsdk:"log"`
	AnomalyLog    *MonitorConfigurationAnomalyLogModel    `tfsdk:"anomaly_log"`
	AnomalyMetric *MonitorConfigurationAnomalyMetricModel `tfsdk:"anomaly_metric"`
}

type MonitorConfigurationMetricModel struct {
	Condition                MonitorConditionModel `tfsdk:"condition"`
	NoDataBehavior           types.String          `tfsdk:"no_data_behavior"`
	Timeframe                types.Int64           `tfsdk:"timeframe"`
	GroupByFields            types.List            `tfsdk:"group_by_fields"`
	AggregationAlertLogic    types.String          `tfsdk:"aggregation_alert_logic"`
	ProportionAlertThreshold types.Int64           `tfsdk:"proportion_alert_threshold"`
	Queries                  types.List            `tfsdk:"queries"`
}

type MonitorConfigurationLogModel struct {
	Condition                MonitorConditionModel `tfsdk:"condition"`
	NoDataBehavior           types.String          `tfsdk:"no_data_behavior"`
	Timeframe                types.Int64           `tfsdk:"timeframe"`
	GroupByFields            types.List            `tfsdk:"group_by_fields"`
	AggregationAlertLogic    types.String          `tfsdk:"aggregation_alert_logic"`
	ProportionAlertThreshold types.Int64           `tfsdk:"proportion_alert_threshold"`
	Queries                  types.List            `tfsdk:"queries"`
}

type MonitorConfigurationAnomalyLogModel struct {
	Condition                MonitorAnomalyConditionModel `tfsdk:"condition"`
	NoDataBehavior           types.String                 `tfsdk:"no_data_behavior"`
	Timeframe                types.Int64                  `tfsdk:"timeframe"`
	GroupByFields            types.List                   `tfsdk:"group_by_fields"`
	AggregationAlertLogic    types.String                 `tfsdk:"aggregation_alert_logic"`
	ProportionAlertThreshold types.Int64                  `tfsdk:"proportion_alert_threshold"`
	Queries                  types.List                   `tfsdk:"queries"`
}

type MonitorConfigurationAnomalyMetricModel struct {
	Condition                MonitorAnomalyConditionModel `tfsdk:"condition"`
	NoDataBehavior           types.String                 `tfsdk:"no_data_behavior"`
	Timeframe                types.Int64                  `tfsdk:"timeframe"`
	GroupByFields            types.List                   `tfsdk:"group_by_fields"`
	AggregationAlertLogic    types.String                 `tfsdk:"aggregation_alert_logic"`
	ProportionAlertThreshold types.Int64                  `tfsdk:"proportion_alert_threshold"`
	Queries                  types.List                   `tfsdk:"queries"`
}

type MonitorConditionModel struct {
	Formula   types.String  `tfsdk:"formula"`
	Operator  types.String  `tfsdk:"operator"`
	Threshold types.Float64 `tfsdk:"threshold"`
}

type MonitorAnomalyConditionModel struct {
	Formula       types.String `tfsdk:"formula"`
	ConditionType types.String `tfsdk:"condition_type"`
}

type AggregationGroupByModel struct {
	Fields types.List  `tfsdk:"fields"`
	Limit  types.Int64 `tfsdk:"limit"`
}

// Separate query models for metric and log
type MetricQueryModel struct {
	Name          types.String         `tfsdk:"name"`
	Filter        types.String         `tfsdk:"filter"`
	Aggregate     MetricAggregateModel `tfsdk:"aggregate"`
	ValueIfNoData types.String         `tfsdk:"value_if_no_data"`
}

type LogQueryModel struct {
	Name          types.String      `tfsdk:"name"`
	Filter        types.String      `tfsdk:"filter"`
	Aggregate     LogAggregateModel `tfsdk:"aggregate"`
	ValueIfNoData types.String      `tfsdk:"value_if_no_data"`
}

// Separate aggregate models for metric and log
type MetricAggregateModel struct {
	Average     *AggregateFieldModel      `tfsdk:"average"`
	Max         *AggregateFieldModel      `tfsdk:"max"`
	Min         *AggregateFieldModel      `tfsdk:"min"`
	Sum         *AggregateFieldModel      `tfsdk:"sum"`
	Percentile  *AggregatePercentileModel `tfsdk:"percentile"`
	UniqueCount *AggregateFieldModel      `tfsdk:"unique_count"`
}

type LogAggregateModel struct {
	Average     *AggregateFieldModel      `tfsdk:"average"`
	Count       *AggregateCountModel      `tfsdk:"count"`
	Max         *AggregateFieldModel      `tfsdk:"max"`
	Min         *AggregateFieldModel      `tfsdk:"min"`
	Sum         *AggregateFieldModel      `tfsdk:"sum"`
	Percentile  *AggregatePercentileModel `tfsdk:"percentile"`
	UniqueCount *AggregateFieldModel      `tfsdk:"unique_count"`
}

type AggregateFieldModel struct {
	Field types.String `tfsdk:"field"`
}

type AggregatePercentileModel struct {
	Field      types.String  `tfsdk:"field"`
	Percentile types.Float64 `tfsdk:"percentile"`
}

type AggregateCountModel struct{}

func MetricQueryAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":             types.StringType,
		"filter":           types.StringType,
		"aggregate":        types.ObjectType{AttrTypes: MetricAggregateAttrTypes()},
		"value_if_no_data": types.StringType,
	}
}

func LogQueryAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":             types.StringType,
		"filter":           types.StringType,
		"aggregate":        types.ObjectType{AttrTypes: LogAggregateAttrTypes()},
		"value_if_no_data": types.StringType,
	}
}

func MetricAggregateAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"average":      types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"max":          types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"min":          types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"sum":          types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"percentile":   types.ObjectType{AttrTypes: AggregatePercentileAttrTypes()},
		"unique_count": types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
	}
}

func LogAggregateAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"average":      types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"count":        types.ObjectType{AttrTypes: AggregateCountAttrTypes()},
		"max":          types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"min":          types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"sum":          types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
		"percentile":   types.ObjectType{AttrTypes: AggregatePercentileAttrTypes()},
		"unique_count": types.ObjectType{AttrTypes: AggregateFieldAttrTypes()},
	}
}

func AggregateFieldAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.StringType,
	}
}

func AggregatePercentileAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"field":      types.StringType,
		"percentile": types.Float64Type,
	}
}

func AggregateCountAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{}
}

func AggregationGroupByAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"fields": types.ListType{ElemType: types.StringType},
		"limit":  types.Int64Type,
	}
}
