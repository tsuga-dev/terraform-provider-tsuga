package resource_notification_rule

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

func NotificationRuleResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "Rules to trigger notifications to targets based on alert events",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the notification rule",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the notification rule",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"teams_filter": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Team filter that narrows down the teams that can receive notifications from this rule",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required:    true,
						Description: "Filter type: 'specific-teams' to notify only specified teams, 'all-teams' to notify all teams, 'all-public-teams' to notify all public teams",
						Validators: []validator.String{
							stringvalidator.OneOf("specific-teams", "all-teams", "all-public-teams"),
						},
					},
					"teams": schema.ListAttribute{
						Optional:    true,
						Description: "Team IDs to select (required when type is 'specific-teams')",
						ElementType: types.StringType,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
							listvalidator.SizeAtMost(100),
						},
					},
				},
			},
			"priorities_filter": schema.ListAttribute{
				Required:    true,
				Description: "Priorities that narrow down the alerts that can trigger a notification",
				ElementType: types.Int64Type,
				Validators: []validator.List{
					listvalidator.SizeAtMost(5),
					listvalidator.ValueInt64sAre(int64validator.Between(1, 5)),
				},
			},
			"transition_types_filter": schema.ListAttribute{
				Required:    true,
				Description: "Alert state transitions that can trigger a notification",
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtMost(3),
					listvalidator.ValueStringsAre(
						stringvalidator.OneOf("triggered", "resolved", "no-data"),
					),
				},
			},
			"owner": schema.StringAttribute{
				Required:    true,
				Description: "Team ID that owns and manages the rule",
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
				Required: true,
			},
			"targets": schema.ListNestedAttribute{
				Required:    true,
				Description: "Notification targets that can receive notifications when the rule matches",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:    true,
							Description: "Identifier of the notification target",
							Validators: []validator.String{
								stringvalidator.LengthAtMost(250),
							},
						},
						"config": schema.SingleNestedAttribute{
							Required: true,
							Attributes: map[string]schema.Attribute{
								"slack": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"type": schema.StringAttribute{
											Computed: true,
										},
										"channel": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.LengthAtMost(250),
											},
										},
										"integration_id": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.LengthAtMost(250),
											},
										},
										"integration_name": schema.StringAttribute{
											Computed: true,
										},
										"hide_time": schema.BoolAttribute{
											Optional:    true,
											Description: "When true, the timestamp is hidden from the Slack message",
										},
										"hide_transition": schema.BoolAttribute{
											Optional:    true,
											Description: "When true, the transition info is hidden from the Slack message",
										},
									},
								},
								"incident_io": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"type": schema.StringAttribute{
											Computed: true,
										},
										"integration_id": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.LengthAtMost(250),
											},
										},
										"integration_name": schema.StringAttribute{
											Computed: true,
										},
									},
								},
								"pagerduty": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"type": schema.StringAttribute{
											Computed: true,
										},
										"integration_id": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.LengthAtMost(250),
											},
										},
										"integration_name": schema.StringAttribute{
											Computed: true,
										},
									},
								},
								"email": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"type": schema.StringAttribute{
											Computed: true,
										},
										"addresses": schema.ListAttribute{
											Required:    true,
											ElementType: types.StringType,
										},
									},
								},
								"grafana_irm": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"type": schema.StringAttribute{
											Computed: true,
										},
										"integration_id": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.LengthAtMost(250),
											},
										},
										"integration_name": schema.StringAttribute{
											Computed: true,
										},
									},
								},
								"microsoft_teams": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"type": schema.StringAttribute{
											Computed: true,
										},
										"integration_id": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.LengthAtMost(250),
											},
										},
										"integration_name": schema.StringAttribute{
											Computed: true,
										},
									},
								},
								"webhook": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"type": schema.StringAttribute{
											Computed: true,
										},
										"integration_id": schema.StringAttribute{
											Required: true,
											Validators: []validator.String{
												stringvalidator.LengthAtMost(250),
											},
										},
										"integration_name": schema.StringAttribute{
											Computed: true,
										},
									},
								},
							},
						},
						"rate_limit": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"max_messages": schema.Int64Attribute{
									Required: true,
								},
								"minutes": schema.Int64Attribute{
									Required: true,
								},
							},
						},
						"renotify_config": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"mode": schema.StringAttribute{
									Required: true,
									Validators: []validator.String{
										stringvalidator.OneOf("each"),
									},
								},
								"renotification_states": schema.ListAttribute{
									Required:    true,
									ElementType: types.StringType,
									Validators: []validator.List{
										listvalidator.ValueStringsAre(
											stringvalidator.OneOf("alert", "alert_no_data"),
										),
									},
								},
								"renotify_interval_minutes": schema.Int64Attribute{
									Required: true,
								},
							},
						},
					},
				},
			},
		},
	}
}

type NotificationRuleModel struct {
	Id                    types.String     `tfsdk:"id"`
	Name                  types.String     `tfsdk:"name"`
	TeamsFilter           *TeamsFilterModel `tfsdk:"teams_filter"`
	PrioritiesFilter      types.List       `tfsdk:"priorities_filter"`
	TransitionTypesFilter types.List       `tfsdk:"transition_types_filter"`
	Owner                 types.String     `tfsdk:"owner"`
	Tags                  types.List       `tfsdk:"tags"`
	IsActive              types.Bool       `tfsdk:"is_active"`
	Targets               types.List       `tfsdk:"targets"`
}

type TeamsFilterModel struct {
	Type  types.String `tfsdk:"type"`
	Teams types.List   `tfsdk:"teams"`
}

type TargetModel struct {
	Id             types.String          `tfsdk:"id"`
	Config         TargetConfigModel     `tfsdk:"config"`
	RateLimit      *TargetRateLimitModel `tfsdk:"rate_limit"`
	RenotifyConfig *TargetRenotifyModel  `tfsdk:"renotify_config"`
}

type TargetConfigModel struct {
	Slack          *SlackConfigModel           `tfsdk:"slack"`
	IncidentIO     *IntegrationOnlyConfigModel `tfsdk:"incident_io"`
	PagerDuty      *IntegrationOnlyConfigModel `tfsdk:"pagerduty"`
	Email          *EmailConfigModel           `tfsdk:"email"`
	GrafanaIRM     *IntegrationOnlyConfigModel `tfsdk:"grafana_irm"`
	MicrosoftTeams *IntegrationOnlyConfigModel `tfsdk:"microsoft_teams"`
	Webhook        *IntegrationOnlyConfigModel `tfsdk:"webhook"`
}

type TargetRateLimitModel struct {
	MaxMessages types.Int64 `tfsdk:"max_messages"`
	Minutes     types.Int64 `tfsdk:"minutes"`
}

type TargetRenotifyModel struct {
	Mode                    types.String `tfsdk:"mode"`
	RenotificationStates    types.List   `tfsdk:"renotification_states"`
	RenotifyIntervalMinutes types.Int64  `tfsdk:"renotify_interval_minutes"`
}

type SlackConfigModel struct {
	Type            types.String `tfsdk:"type"`
	Channel         types.String `tfsdk:"channel"`
	IntegrationID   types.String `tfsdk:"integration_id"`
	IntegrationName types.String `tfsdk:"integration_name"`
	HideTime        types.Bool   `tfsdk:"hide_time"`
	HideTransition  types.Bool   `tfsdk:"hide_transition"`
}

type IntegrationOnlyConfigModel struct {
	Type            types.String `tfsdk:"type"`
	IntegrationID   types.String `tfsdk:"integration_id"`
	IntegrationName types.String `tfsdk:"integration_name"`
}

type EmailConfigModel struct {
	Type      types.String `tfsdk:"type"`
	Addresses types.List   `tfsdk:"addresses"`
}

func TargetConfigAttrTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"slack":           types.ObjectType{AttrTypes: SlackAttrTypes(ctx)},
		"incident_io":     types.ObjectType{AttrTypes: IntegrationConfigAttrTypes(ctx)},
		"pagerduty":       types.ObjectType{AttrTypes: IntegrationConfigAttrTypes(ctx)},
		"email":           types.ObjectType{AttrTypes: EmailAttrTypes(ctx)},
		"grafana_irm":     types.ObjectType{AttrTypes: IntegrationConfigAttrTypes(ctx)},
		"microsoft_teams": types.ObjectType{AttrTypes: IntegrationConfigAttrTypes(ctx)},
		"webhook":         types.ObjectType{AttrTypes: IntegrationConfigAttrTypes(ctx)},
	}
}

func SlackAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"type":             types.StringType,
		"channel":          types.StringType,
		"integration_id":   types.StringType,
		"integration_name": types.StringType,
		"hide_time":        types.BoolType,
		"hide_transition":  types.BoolType,
	}
}

func IntegrationConfigAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"type":             types.StringType,
		"integration_id":   types.StringType,
		"integration_name": types.StringType,
	}
}

func EmailAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"type":      types.StringType,
		"addresses": types.ListType{ElemType: types.StringType},
	}
}

func TargetRateLimitAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"max_messages": types.Int64Type,
		"minutes":      types.Int64Type,
	}
}

func TargetRenotifyAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"mode":                      types.StringType,
		"renotification_states":     types.ListType{ElemType: types.StringType},
		"renotify_interval_minutes": types.Int64Type,
	}
}

func TargetAttrTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"id":              types.StringType,
		"config":          types.ObjectType{AttrTypes: TargetConfigAttrTypes(ctx)},
		"rate_limit":      types.ObjectType{AttrTypes: TargetRateLimitAttrTypes(ctx)},
		"renotify_config": types.ObjectType{AttrTypes: TargetRenotifyAttrTypes(ctx)},
	}
}

func TeamsFilterAttrTypes(_ context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"type":  types.StringType,
		"teams": types.ListType{ElemType: types.StringType},
	}
}
