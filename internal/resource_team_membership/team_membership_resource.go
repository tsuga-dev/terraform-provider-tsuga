package resource_team_membership

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TeamMembershipResourceSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Description: "A membership of a user in a team",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the team membership",
			},
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "Identifier of the user",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"team_id": schema.StringAttribute{
				Required:    true,
				Description: "Identifier of the team",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"role_key": schema.StringAttribute{
				Required:    true,
				Description: "Role of the user in the team",
				Validators: []validator.String{
					stringvalidator.OneOf("admin", "editor", "viewer"),
				},
			},
		},
	}
}

type TeamMembershipModel struct {
	Id      types.String `tfsdk:"id"`
	UserId  types.String `tfsdk:"user_id"`
	TeamId  types.String `tfsdk:"team_id"`
	RoleKey types.String `tfsdk:"role_key"`
}
