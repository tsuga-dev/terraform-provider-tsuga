package resource_notification_silence

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-tsuga/internal/resource_team"
	"terraform-provider-tsuga/internal/teamsfilter"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func NotificationSilenceResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "Silences to suppress notifications based on schedules and filters",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique identifier of the silence",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the silence",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the silence",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"owner": schema.StringAttribute{
				Required:    true,
				Description: "Team ID that owns and manages the silence",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
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
			"is_active": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the silence is currently enabled",
			},
			"schedule": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Schedule defining when the silence is active",
				// We might extend this to include one-time schedules in the future
				Attributes: map[string]schema.Attribute{
					"recurring": schema.SingleNestedAttribute{
						Required:    true,
						Description: "Recurring weekly silence schedule",
						Attributes: map[string]schema.Attribute{
							"monday": schema.ListNestedAttribute{
								Optional:    true,
								Description: "Time ranges for Monday",
								NestedObject: schema.NestedAttributeObject{
									Attributes: timeRangeAttributes(),
								},
							},
							"tuesday": schema.ListNestedAttribute{
								Optional:    true,
								Description: "Time ranges for Tuesday",
								NestedObject: schema.NestedAttributeObject{
									Attributes: timeRangeAttributes(),
								},
							},
							"wednesday": schema.ListNestedAttribute{
								Optional:    true,
								Description: "Time ranges for Wednesday",
								NestedObject: schema.NestedAttributeObject{
									Attributes: timeRangeAttributes(),
								},
							},
							"thursday": schema.ListNestedAttribute{
								Optional:    true,
								Description: "Time ranges for Thursday",
								NestedObject: schema.NestedAttributeObject{
									Attributes: timeRangeAttributes(),
								},
							},
							"friday": schema.ListNestedAttribute{
								Optional:    true,
								Description: "Time ranges for Friday",
								NestedObject: schema.NestedAttributeObject{
									Attributes: timeRangeAttributes(),
								},
							},
							"saturday": schema.ListNestedAttribute{
								Optional:    true,
								Description: "Time ranges for Saturday",
								NestedObject: schema.NestedAttributeObject{
									Attributes: timeRangeAttributes(),
								},
							},
							"sunday": schema.ListNestedAttribute{
								Optional:    true,
								Description: "Time ranges for Sunday",
								NestedObject: schema.NestedAttributeObject{
									Attributes: timeRangeAttributes(),
								},
							},
						},
					},
				},
			},
			"notification_rule_ids": schema.ListAttribute{
				Optional:    true,
				Description: "Notification rule IDs this silence applies to",
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.ValueStringsAre(
						stringvalidator.LengthAtMost(250),
					),
				},
			},
			"query_string": schema.StringAttribute{
				Optional:    true,
				Description: "Query string filtering which alerts this silence applies to",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(10000),
				},
			},
			"teams_filter": teamsfilter.Schema("Team filter that narrows down which teams' alerts this silence applies to"),
			"priorities_filter": schema.ListAttribute{
				Required:    true,
				Description: "Monitor priorities filtering which alerts this silence applies to",
				ElementType: types.Int64Type,
				Validators: []validator.List{
					listvalidator.SizeAtMost(5),
					listvalidator.ValueInt64sAre(int64validator.Between(1, 5)),
				},
			},
			"transition_types_filter": schema.ListAttribute{
				Required:    true,
				Description: "Transition types filtering which alerts this silence applies to",
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtMost(3),
					listvalidator.ValueStringsAre(
						stringvalidator.OneOf("triggered", "resolved", "no-data"),
					),
				},
			},
		},
	}
}

func timeRangeAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"start_time": schema.StringAttribute{
			Required:    true,
			Description: "Start time in HH:MM:SSZ format (e.g., 09:00:00Z)",
		},
		"end_time": schema.StringAttribute{
			Required:    true,
			Description: "End time in HH:MM:SSZ format (e.g., 17:00:00Z)",
		},
	}
}

type NotificationSilenceModel struct {
	Id                    types.String       `tfsdk:"id"`
	Name                  types.String       `tfsdk:"name"`
	Description           types.String       `tfsdk:"description"`
	Owner                 types.String       `tfsdk:"owner"`
	Tags                  types.List         `tfsdk:"tags"`
	IsActive              types.Bool         `tfsdk:"is_active"`
	Schedule              *ScheduleModel     `tfsdk:"schedule"`
	NotificationRuleIds   types.List         `tfsdk:"notification_rule_ids"`
	QueryString           types.String       `tfsdk:"query_string"`
	TeamsFilter           *teamsfilter.Model `tfsdk:"teams_filter"`
	PrioritiesFilter      types.List         `tfsdk:"priorities_filter"`
	TransitionTypesFilter types.List         `tfsdk:"transition_types_filter"`
}

type ScheduleModel struct {
	Recurring *RecurringScheduleModel `tfsdk:"recurring"`
}

type RecurringScheduleModel struct {
	Monday    types.List `tfsdk:"monday"`
	Tuesday   types.List `tfsdk:"tuesday"`
	Wednesday types.List `tfsdk:"wednesday"`
	Thursday  types.List `tfsdk:"thursday"`
	Friday    types.List `tfsdk:"friday"`
	Saturday  types.List `tfsdk:"saturday"`
	Sunday    types.List `tfsdk:"sunday"`
}

type TimeRangeModel struct {
	StartTime types.String `tfsdk:"start_time"`
	EndTime   types.String `tfsdk:"end_time"`
}

func TimeRangeAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"start_time": types.StringType,
		"end_time":   types.StringType,
	}
}
