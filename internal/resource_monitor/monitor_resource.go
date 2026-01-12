package resource_monitor

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
	"terraform-provider-tsuga/internal/resource_team"
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
					"metric":         monitorConfigurationSchema(),
					"log":            monitorConfigurationSchema(),
					"anomaly_metric": anomalyMonitorConfigurationSchema(),
					"anomaly_log":    anomalyMonitorConfigurationSchema(),
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

func anomalyConditionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"formula": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
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

func queriesSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Required: true,
		Validators: []validator.List{
			listvalidator.SizeAtMost(15),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"filter": schema.StringAttribute{
					Required: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(10000),
					},
				},
				"aggregate": aggregate.Schema(),
				"functions": aggregationFunctionsSchema(),
				"fill":      aggregationFillSchema(),
			},
		},
	}
}

func aggregationFunctionsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		Validators: []validator.List{
			listvalidator.SizeAtMost(10),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"per_second": aggregationFunctionEmptySchema(),
				"per_minute": aggregationFunctionEmptySchema(),
				"per_hour":   aggregationFunctionEmptySchema(),
				"rate":       aggregationFunctionEmptySchema(),
				"increase":   aggregationFunctionEmptySchema(),
				"rolling":    aggregationFunctionRollingSchema(),
			},
		},
	}
}

func aggregationFunctionEmptySchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:   true,
		Attributes: map[string]schema.Attribute{},
	}
}

func aggregationFunctionRollingSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"window": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
		},
	}
}

func aggregationFillSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"mode": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.OneOf("zero", "null"),
						},
					},
				},
			},
		},
	}
}

func monitorConfigurationSchema() schema.Attribute {
	attrs := baseMonitorConfigurationAttributes()
	attrs["condition"] = monitorConditionSchema()
	attrs["no_data_behavior"] = monitorNoDataBehaviorSchema(true)
	attrs["queries"] = queriesSchema()
	return schema.SingleNestedAttribute{
		Optional:   true,
		Attributes: attrs,
	}
}

func anomalyMonitorConfigurationSchema() schema.Attribute {
	attrs := baseMonitorConfigurationAttributes()
	attrs["condition"] = anomalyConditionSchema()
	attrs["no_data_behavior"] = monitorNoDataBehaviorSchema(false) // anomaly monitors don't support consider_zero
	attrs["queries"] = queriesSchema()
	return schema.SingleNestedAttribute{
		Optional:   true,
		Attributes: attrs,
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
	Metric        *MonitorConfigurationDetailsModel        `tfsdk:"metric"`
	Log           *MonitorConfigurationDetailsModel        `tfsdk:"log"`
	AnomalyMetric *AnomalyMonitorConfigurationDetailsModel `tfsdk:"anomaly_metric"`
	AnomalyLog    *AnomalyMonitorConfigurationDetailsModel `tfsdk:"anomaly_log"`
}

type MonitorConfigurationDetailsModel struct {
	Condition                MonitorConditionModel `tfsdk:"condition"`
	NoDataBehavior           types.String          `tfsdk:"no_data_behavior"`
	Timeframe                types.Int64           `tfsdk:"timeframe"`
	GroupByFields            types.List            `tfsdk:"group_by_fields"`
	AggregationAlertLogic    types.String          `tfsdk:"aggregation_alert_logic"`
	ProportionAlertThreshold types.Int64           `tfsdk:"proportion_alert_threshold"`
	Queries                  types.List            `tfsdk:"queries"`
}

type MonitorConditionModel struct {
	Formula   types.String  `tfsdk:"formula"`
	Operator  types.String  `tfsdk:"operator"`
	Threshold types.Float64 `tfsdk:"threshold"`
}

type AnomalyMonitorConfigurationDetailsModel struct {
	Condition                AnomalyConditionModel `tfsdk:"condition"`
	NoDataBehavior           types.String          `tfsdk:"no_data_behavior"`
	Timeframe                types.Int64           `tfsdk:"timeframe"`
	GroupByFields            types.List            `tfsdk:"group_by_fields"`
	AggregationAlertLogic    types.String          `tfsdk:"aggregation_alert_logic"`
	ProportionAlertThreshold types.Int64           `tfsdk:"proportion_alert_threshold"`
	Queries                  types.List            `tfsdk:"queries"`
}

type AnomalyConditionModel struct {
	Formula types.String `tfsdk:"formula"`
}

type MonitorQueryModel struct {
	Filter    types.String          `tfsdk:"filter"`
	Aggregate MonitorAggregateModel `tfsdk:"aggregate"`
	Functions types.List            `tfsdk:"functions"`
	Fill      *AggregationFillModel `tfsdk:"fill"`
}

type MonitorAggregateModel struct {
	Count       *aggregate.CountModel      `tfsdk:"count"`
	Average     *aggregate.FieldModel      `tfsdk:"average"`
	Max         *aggregate.FieldModel      `tfsdk:"max"`
	Min         *aggregate.FieldModel      `tfsdk:"min"`
	Sum         *aggregate.FieldModel      `tfsdk:"sum"`
	Percentile  *aggregate.PercentileModel `tfsdk:"percentile"`
	UniqueCount *aggregate.FieldModel      `tfsdk:"unique_count"`
}

type AggregationFunctionModel struct {
	PerSecond *AggregationFunctionEmptyModel   `tfsdk:"per_second"`
	PerMinute *AggregationFunctionEmptyModel   `tfsdk:"per_minute"`
	PerHour   *AggregationFunctionEmptyModel   `tfsdk:"per_hour"`
	Rate      *AggregationFunctionEmptyModel   `tfsdk:"rate"`
	Increase  *AggregationFunctionEmptyModel   `tfsdk:"increase"`
	Rolling   *AggregationFunctionRollingModel `tfsdk:"rolling"`
}

type AggregationFunctionEmptyModel struct{}

type AggregationFunctionRollingModel struct {
	Window types.String `tfsdk:"window"`
}

type AggregationFillModel struct {
	Mode AggregationFillModeModel `tfsdk:"mode"`
}

type AggregationFillModeModel struct {
	Type types.String `tfsdk:"type"`
}

func QueryAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"filter":    types.StringType,
		"aggregate": types.ObjectType{AttrTypes: aggregate.AttrTypes()},
		"functions": types.ListType{ElemType: types.ObjectType{AttrTypes: AggregationFunctionAttrTypes()}},
		"fill":      types.ObjectType{AttrTypes: AggregationFillAttrTypes()},
	}
}

func AggregationFunctionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"per_second": types.ObjectType{AttrTypes: AggregationFunctionEmptyAttrTypes()},
		"per_minute": types.ObjectType{AttrTypes: AggregationFunctionEmptyAttrTypes()},
		"per_hour":   types.ObjectType{AttrTypes: AggregationFunctionEmptyAttrTypes()},
		"rate":       types.ObjectType{AttrTypes: AggregationFunctionEmptyAttrTypes()},
		"increase":   types.ObjectType{AttrTypes: AggregationFunctionEmptyAttrTypes()},
		"rolling":    types.ObjectType{AttrTypes: AggregationFunctionRollingAttrTypes()},
	}
}

func AggregationFunctionEmptyAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{}
}

func AggregationFunctionRollingAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"window": types.StringType,
	}
}

func AggregationFillAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"mode": types.ObjectType{AttrTypes: AggregationFillModeAttrTypes()},
	}
}

func AggregationFillModeAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
	}
}
