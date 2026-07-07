package resource_slo

import (
	"context"
	"terraform-provider-tsuga/internal/resource_monitor"
	"terraform-provider-tsuga/internal/resource_team"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// SloResourceSchema describes the tsuga_slo resource. The SLI configuration is a
// discriminated union (event vs time); both variants reuse the monitor aggregation-query
// shape (see resource_monitor.QueriesSchema) for their good/total/query formulas.
func SloResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "Service Level Objective: its SLI configuration, target percentage, rolling timeframe, cluster scope, and attached alerts",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the SLO",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the SLO",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Free-form description of the SLO",
				// The SLO API keeps the prior description when a PUT omits it, so model it as
				// computed and reuse the prior value when the config leaves it unset, otherwise
				// every plan that omits description would show it as "known after apply".
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
			"configuration": schema.SingleNestedAttribute{
				Required:    true,
				Description: "SLO SLI configuration. Exactly one of event or time must be set.",
				Attributes: map[string]schema.Attribute{
					"event": sloEventConfigurationSchema(),
					"time":  sloTimeConfigurationSchema(),
				},
			},
			"target": schema.Float64Attribute{
				Required:    true,
				Description: "Target percentage (0 < target < 100, e.g. 99.9)",
				Validators: []validator.Float64{
					// The API requires an exclusive 0 < target < 100 bound.
					exclusiveBetween(0, 100),
				},
			},
			"timeframe_days": schema.Int64Attribute{
				Required:    true,
				Description: "Rolling SLO window in days (7, 30, or 90)",
				Validators: []validator.Int64{
					int64validator.OneOf(7, 30, 90),
				},
			},
			"owner": schema.StringAttribute{
				Required:    true,
				Description: "Team ID that owns and manages the SLO",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"permissions": schema.StringAttribute{
				Required:    true,
				Description: "This controls which data the SLO can see",
				Validators: []validator.String{
					stringvalidator.OneOf("all", "owning-team-only", "owning-team-and-public"),
				},
			},
			"cluster_ids": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Clusters this SLO runs against. Empty = all clusters; non-empty = only the listed cluster IDs",
				ElementType: types.StringType,
			},
			"alerts": schema.ListNestedAttribute{
				Required:    true,
				Description: "Alerts attached to this SLO. The set is reconciled to exactly this list: alerts with a known id are updated, alerts without an id are created, and existing alerts absent from the list are deleted. Send an empty list to clear all alerts.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Server-assigned identifier of the alert",
						},
						"priority": schema.Int64Attribute{
							Required:    true,
							Description: "Alert priority (1 is highest, 5 is lowest)",
							Validators: []validator.Int64{
								int64validator.Between(1, 5),
							},
						},
						"configuration": schema.SingleNestedAttribute{
							Required:    true,
							Description: "Alert trigger configuration. Exactly one of burn_rate or threshold must be set.",
							Attributes: map[string]schema.Attribute{
								"burn_rate": schema.Float64Attribute{
									Optional:    true,
									Description: "Burn rate multiplier that triggers the alert (1-100). The short/long evaluation windows are derived from the predefined template with the closest burn rate.",
									Validators: []validator.Float64{
										float64validator.Between(1, 100),
									},
								},
								"threshold": schema.Float64Attribute{
									Optional:    true,
									Description: "SLO target percentage below which the alert triggers (0 < threshold < 100)",
									Validators: []validator.Float64{
										// The API requires an exclusive 0 < threshold < 100 bound.
										exclusiveBetween(0, 100),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func sloDataSourceSchema() schema.Attribute {
	return schema.StringAttribute{
		Required:    true,
		Description: "Telemetry source queried by this SLO: logs, metrics, or traces",
		Validators: []validator.String{
			stringvalidator.OneOf("logs", "metrics", "traces"),
		},
	}
}

// sloQueryFormulaSchema returns the schema for an SLO query formula: a list of monitor
// aggregation queries combined by a formula. Reuses resource_monitor.QueriesSchema so the
// wire format matches a monitor query exactly.
func sloQueryFormulaSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required:    true,
		Description: "Aggregation queries combined by a formula to produce the SLO signal",
		Attributes: map[string]schema.Attribute{
			"queries": resource_monitor.QueriesSchema(),
			"formula": schema.StringAttribute{
				Required:    true,
				Description: "Formula referencing query outputs (e.g. q1+q2)",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
		},
	}
}

// sloGroupByFieldsSchema mirrors the monitor group_by_fields schema but is optional for SLOs.
func sloGroupByFieldsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional:    true,
		Description: "Hierarchical grouping applied to the SLO results: 1 to 7 levels, each naming one attribute. Omit for no grouping.",
		Validators: []validator.List{
			// API supports [0, 7] and is an optional field. Disallow empty list to keep state consistent.
			listvalidator.SizeBetween(1, 7),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"fields": schema.ListAttribute{
					Required:    true,
					ElementType: types.StringType,
					Validators: []validator.List{
						// API supports [0, 1] and is an optional field. Disallow empty list to keep state consistent.
						listvalidator.SizeBetween(1, 1),
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

func sloEventConfigurationSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Event-based SLO: a ratio of good events over total events",
		Attributes: map[string]schema.Attribute{
			"data_source":     sloDataSourceSchema(),
			"good_query":      sloQueryFormulaSchema(),
			"total_query":     sloQueryFormulaSchema(),
			"group_by_fields": sloGroupByFieldsSchema(),
			"no_data_behavior": schema.StringAttribute{
				Required:    true,
				Description: "How to treat an SLO with no data: good (meets the SLO) or bad (breaches it)",
				Validators: []validator.String{
					stringvalidator.OneOf("good", "bad"),
				},
			},
		},
	}
}

func sloTimeConfigurationSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Time-based SLO: a thresholded query evaluated over fixed-size time slices",
		Attributes: map[string]schema.Attribute{
			"data_source": sloDataSourceSchema(),
			"query":       sloQueryFormulaSchema(),
			"slice_size_minutes": schema.Int64Attribute{
				Required:    true,
				Description: "Size of each evaluation slice in minutes (30-1440)",
				Validators: []validator.Int64{
					int64validator.Between(3, 1440),
				},
			},
			"threshold": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Comparison between the query signal and a threshold value that determines whether a slice is good",
				Attributes: map[string]schema.Attribute{
					"operator": schema.StringAttribute{
						Required:    true,
						Description: "Comparison operator between the query value and the threshold",
						Validators: []validator.String{
							stringvalidator.OneOf("greater_than", "less_than", "greater_than_or_equal", "less_than_or_equal"),
						},
					},
					"value": schema.Float64Attribute{
						Required:    true,
						Description: "Threshold value compared against the query signal",
					},
				},
			},
			"group_by_fields": sloGroupByFieldsSchema(),
			"no_data_behavior": schema.StringAttribute{
				Required:    true,
				Description: "How to treat time slices with no data: good (meets the SLO), bad (breaches it), or ignore (exclude from the error budget)",
				Validators: []validator.String{
					stringvalidator.OneOf("good", "bad", "ignore"),
				},
			},
		},
	}
}

// Model types

type SloModel struct {
	Id            types.String          `tfsdk:"id"`
	Name          types.String          `tfsdk:"name"`
	Description   types.String          `tfsdk:"description"`
	Tags          types.List            `tfsdk:"tags"`
	Configuration SloConfigurationModel `tfsdk:"configuration"`
	Target        types.Float64         `tfsdk:"target"`
	TimeframeDays types.Int64           `tfsdk:"timeframe_days"`
	Owner         types.String          `tfsdk:"owner"`
	Permissions   types.String          `tfsdk:"permissions"`
	ClusterIds    types.List            `tfsdk:"cluster_ids"`
	Alerts        types.List            `tfsdk:"alerts"`
}

type SloConfigurationModel struct {
	Event *SloEventConfigurationModel `tfsdk:"event"`
	Time  *SloTimeConfigurationModel  `tfsdk:"time"`
}

type SloEventConfigurationModel struct {
	DataSource     types.String         `tfsdk:"data_source"`
	GoodQuery      SloQueryFormulaModel `tfsdk:"good_query"`
	TotalQuery     SloQueryFormulaModel `tfsdk:"total_query"`
	GroupByFields  types.List           `tfsdk:"group_by_fields"`
	NoDataBehavior types.String         `tfsdk:"no_data_behavior"`
}

type SloTimeConfigurationModel struct {
	DataSource       types.String          `tfsdk:"data_source"`
	Query            SloQueryFormulaModel  `tfsdk:"query"`
	SliceSizeMinutes types.Int64           `tfsdk:"slice_size_minutes"`
	Threshold        SloTimeThresholdModel `tfsdk:"threshold"`
	GroupByFields    types.List            `tfsdk:"group_by_fields"`
	NoDataBehavior   types.String          `tfsdk:"no_data_behavior"`
}

type SloQueryFormulaModel struct {
	Queries types.List   `tfsdk:"queries"`
	Formula types.String `tfsdk:"formula"`
}

type SloTimeThresholdModel struct {
	Operator types.String  `tfsdk:"operator"`
	Value    types.Float64 `tfsdk:"value"`
}

type SloAlertModel struct {
	Id            types.String               `tfsdk:"id"`
	Priority      types.Int64                `tfsdk:"priority"`
	Configuration SloAlertConfigurationModel `tfsdk:"configuration"`
}

type SloAlertConfigurationModel struct {
	BurnRate  types.Float64 `tfsdk:"burn_rate"`
	Threshold types.Float64 `tfsdk:"threshold"`
}

// BurnRateSet reports whether burn_rate holds a concrete (non-null, known) value.
func (c SloAlertConfigurationModel) BurnRateSet() bool {
	return !c.BurnRate.IsNull() && !c.BurnRate.IsUnknown()
}

// ThresholdSet reports whether threshold holds a concrete (non-null, known) value.
func (c SloAlertConfigurationModel) ThresholdSet() bool {
	return !c.Threshold.IsNull() && !c.Threshold.IsUnknown()
}

// SloAlertAttrTypes returns the attr types for a single SLO alert object.
func SloAlertAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":            types.StringType,
		"priority":      types.Int64Type,
		"configuration": types.ObjectType{AttrTypes: SloAlertConfigurationAttrTypes()},
	}
}

// SloAlertConfigurationAttrTypes returns the attr types for an SLO alert configuration object.
func SloAlertConfigurationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"burn_rate": types.Float64Type,
		"threshold": types.Float64Type,
	}
}
