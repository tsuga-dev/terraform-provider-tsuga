package teamsfilter

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Schema returns the schema for a teams filter object.
func Schema(description string) schema.Attribute {
	return schema.SingleNestedAttribute{
		Required:    true,
		Description: description,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Filter type: 'specific-teams' to apply to only specified teams, 'all-teams' to apply to all teams, 'all-public-teams' to apply to all public teams",
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
	}
}

// Model represents a teams filter configuration.
type Model struct {
	Type  types.String `tfsdk:"type"`
	Teams types.List   `tfsdk:"teams"`
}

// APITeamsFilter represents the API structure for teams filter.
type APITeamsFilter struct {
	Type  string   `json:"type"`
	Teams []string `json:"teams,omitempty"`
}

// Expand converts the Terraform model to the API structure.
func Expand(ctx context.Context, filter *Model) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if filter == nil {
		return nil, diags
	}

	result := map[string]any{
		"type": filter.Type.ValueString(),
	}

	// Only include teams if the filter type is "specific-teams"
	if filter.Type.ValueString() == "specific-teams" {
		teams, expandDiags := expandStringList(ctx, filter.Teams)
		diags.Append(expandDiags...)
		if diags.HasError() {
			return nil, diags
		}
		result["teams"] = teams
	}

	return result, diags
}

// Flatten converts the API structure to the Terraform model.
func Flatten(ctx context.Context, filter APITeamsFilter) (*Model, diag.Diagnostics) {
	var diags diag.Diagnostics

	var teams types.List
	if filter.Teams != nil {
		var teamDiags diag.Diagnostics
		teams, teamDiags = types.ListValueFrom(ctx, types.StringType, filter.Teams)
		diags.Append(teamDiags...)
		if diags.HasError() {
			return nil, diags
		}
	} else {
		teams = types.ListNull(types.StringType)
	}

	return &Model{
		Type:  types.StringValue(filter.Type),
		Teams: teams,
	}, diags
}

// expandStringList converts a types.List to a []string.
func expandStringList(ctx context.Context, value types.List) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}

	var result []string
	diags.Append(value.ElementsAs(ctx, &result, false)...)
	return result, diags
}
