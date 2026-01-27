package resource_tag_policy

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TagPolicyResourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Policy that enforces tag requirements on Tsuga assets or telemetry data",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the tag policy",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the tag policy",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the tag policy",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(1000),
				},
			},
			"is_active": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the tag policy is active",
			},
			"tag_key": schema.StringAttribute{
				Required:    true,
				Description: "The tag key that this policy enforces",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(128),
				},
			},
			"allowed_tag_values": schema.ListAttribute{
				Required:    true,
				Description: "List of allowed values for the tag key",
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"is_required": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the tag is required on matching assets",
			},
			"team_scope": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Team scope that narrows down the teams affected by this policy",
				Attributes: map[string]schema.Attribute{
					"team_ids": schema.ListAttribute{
						Required:    true,
						Description: "Team IDs to include or exclude",
						ElementType: types.StringType,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
							listvalidator.SizeAtMost(100),
							listvalidator.ValueStringsAre(
								stringvalidator.LengthAtMost(250),
							),
						},
					},
					"mode": schema.StringAttribute{
						Required:    true,
						Description: "Filter mode: 'include' to apply only to specified teams, 'exclude' to apply to all except specified teams",
						Validators: []validator.String{
							stringvalidator.OneOf("include", "exclude"),
						},
					},
				},
			},
			"configuration": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Configuration specifying what the policy applies to",
				Attributes: map[string]schema.Attribute{
					"telemetry": schema.SingleNestedAttribute{
						Optional:    true,
						Description: "Configuration for telemetry data policies",
						Attributes: map[string]schema.Attribute{
							"type": schema.StringAttribute{
								Computed: true,
							},
							"asset_types": schema.ListAttribute{
								Required:    true,
								Description: "Telemetry asset types: 'logs', 'metrics', 'traces'",
								ElementType: types.StringType,
								Validators: []validator.List{
									listvalidator.SizeAtLeast(1),
									listvalidator.ValueStringsAre(
										stringvalidator.OneOf("logs", "metrics", "traces"),
									),
								},
							},
							"should_insert_warning": schema.BoolAttribute{
								Required:    true,
								Description: "Whether to insert a warning when the policy is violated",
							},
							"drop_sample": schema.Float64Attribute{
								Optional:    true,
								Description: "Percentage of samples to drop (0-100) when the policy is violated",
								Validators: []validator.Float64{
									float64validator.Between(0, 100),
								},
							},
						},
					},
					"tsuga_asset": schema.SingleNestedAttribute{
						Optional:    true,
						Description: "Configuration for Tsuga asset policies",
						Attributes: map[string]schema.Attribute{
							"type": schema.StringAttribute{
								Computed: true,
							},
							"asset_types": schema.ListAttribute{
								Required:    true,
								Description: "Tsuga asset types the policy applies to",
								ElementType: types.StringType,
								Validators: []validator.List{
									listvalidator.SizeAtLeast(1),
									listvalidator.ValueStringsAre(
										stringvalidator.OneOf(
											"ingestion-api-key",
											"operation-api-key",
											"dashboard",
											"log-route",
											"monitor",
											"notification-rule",
											"notification-silence",
										),
									),
								},
							},
						},
					},
				},
			},
			"owner": schema.StringAttribute{
				Required:    true,
				Description: "Team ID that owns and manages the policy",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
		},
	}
}

type TagPolicyModel struct {
	Id               types.String       `tfsdk:"id"`
	Name             types.String       `tfsdk:"name"`
	Description      types.String       `tfsdk:"description"`
	IsActive         types.Bool         `tfsdk:"is_active"`
	TagKey           types.String       `tfsdk:"tag_key"`
	AllowedTagValues types.List         `tfsdk:"allowed_tag_values"`
	IsRequired       types.Bool         `tfsdk:"is_required"`
	TeamScope        *TeamScopeModel     `tfsdk:"team_scope"`
	Configuration    *ConfigurationModel `tfsdk:"configuration"`
	Owner            types.String        `tfsdk:"owner"`
}

type TeamScopeModel struct {
	TeamIds types.List   `tfsdk:"team_ids"`
	Mode    types.String `tfsdk:"mode"`
}

type ConfigurationModel struct {
	Telemetry  *TelemetryConfigModel  `tfsdk:"telemetry"`
	TsugaAsset *TsugaAssetConfigModel `tfsdk:"tsuga_asset"`
}

type TelemetryConfigModel struct {
	Type                types.String  `tfsdk:"type"`
	AssetTypes          types.List    `tfsdk:"asset_types"`
	ShouldInsertWarning types.Bool    `tfsdk:"should_insert_warning"`
	DropSample          types.Float64 `tfsdk:"drop_sample"`
}

type TsugaAssetConfigModel struct {
	Type       types.String `tfsdk:"type"`
	AssetTypes types.List   `tfsdk:"asset_types"`
}
