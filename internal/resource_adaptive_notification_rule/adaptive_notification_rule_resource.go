package resource_adaptive_notification_rule

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-tsuga/internal/resource_notification_rule"
)

func AdaptiveNotificationRuleResourceSchema(ctx context.Context) schema.Schema {
	// The tags and the fallback target share the notification rule's schema, so both resources
	// accept the same tag limits and target destinations.
	notificationRuleAttributes := resource_notification_rule.NotificationRuleResourceSchema(ctx).Attributes
	targetAttributes := notificationRuleAttributes["targets"].(schema.ListNestedAttribute).NestedObject.Attributes

	return schema.Schema{
		Description: "Rules that page the owners of the failing service when one of their monitors or SLOs fires",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the adaptive notification rule",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the adaptive notification rule",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"owner": schema.StringAttribute{
				Required:    true,
				Description: "Team ID that owns and manages the rule",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"tags": notificationRuleAttributes["tags"],
			"is_active": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the rule sends notifications when one of its monitors or SLOs fires",
			},
			"should_create_investigation": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether each page records an investigation holding the sampled traces and the call graph the destination was resolved from",
			},
			"monitor_ids": schema.ListAttribute{
				Optional:    true,
				Description: "Monitors the rule reacts to. Only trace monitors and metric monitors over OTel span metrics are eligible. Set at least one monitor or SLO.",
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtMost(100),
				},
			},
			"slo_ids": schema.ListAttribute{
				Optional:    true,
				Description: "SLOs the rule reacts to. Only SLOs over traces or OTel span metrics are eligible. Set at least one monitor or SLO.",
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtMost(100),
				},
			},
			"fallback_target": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Notification target paged when the rule can't resolve an owner for the failing service",
				Attributes:  targetAttributes,
			},
		},
	}
}

type AdaptiveNotificationRuleModel struct {
	Id                        types.String                            `tfsdk:"id"`
	Name                      types.String                            `tfsdk:"name"`
	Owner                     types.String                            `tfsdk:"owner"`
	Tags                      types.List                              `tfsdk:"tags"`
	IsActive                  types.Bool                              `tfsdk:"is_active"`
	ShouldCreateInvestigation types.Bool                              `tfsdk:"should_create_investigation"`
	MonitorIds                types.List                              `tfsdk:"monitor_ids"`
	SloIds                    types.List                              `tfsdk:"slo_ids"`
	FallbackTarget            *resource_notification_rule.TargetModel `tfsdk:"fallback_target"`
}
