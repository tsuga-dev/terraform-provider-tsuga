package resource_custom_role

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var kebabCaseName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

var TelemetryQueryElemType = CustomRoleResourceSchema().Attributes["telemetry_query"].GetType().(types.ListType).ElemType.(types.ObjectType)

func CustomRoleResourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Custom global role granting its holders access to Tsuga assets within a team scope, and to telemetry signals",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the custom role",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the custom role, unique across the organization, in kebab-case (lowercase letters, digits and single hyphens, such as 'platform-on-call')",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
					stringvalidator.RegexMatches(kebabCaseName, "must be kebab-case: lowercase letters, digits and single hyphens"),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the custom role",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"team_scope": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Teams the role grants its asset permissions on",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required:    true,
						Description: "Scope type: 'all' covers every team, 'public' covers public teams only, 'specific' covers the listed teams, 'all-except' covers every team but the listed ones",
						Validators: []validator.String{
							stringvalidator.OneOf("all", "public", "specific", "all-except"),
						},
					},
					"team_ids": schema.SetAttribute{
						Optional:    true,
						Description: "Team IDs the scope applies to (required when type is 'specific' or 'all-except', not allowed otherwise)",
						ElementType: types.StringType,
						Validators: []validator.Set{
							setvalidator.SizeBetween(1, 50),
							setvalidator.ValueStringsAre(
								stringvalidator.LengthBetween(1, 250),
							),
						},
					},
				},
			},
			"assets": schema.MapAttribute{
				Required: true,
				Description: "Access level granted per Tsuga asset type within the team scope, 'read' or 'write'. 'write' also grants read access. " +
					"An asset type absent from the map grants no access, and 'team' is required. " +
					"Keys are API resource types such as 'dashboard', 'monitor-folder' or 'ingestion-api-key'; " +
					quoteList(readOnlyAssetTypes) + " only accept 'read'",
				ElementType: types.StringType,
				Validators: []validator.Map{
					mapvalidator.KeysAre(
						stringvalidator.OneOf(assetTypes...),
					),
					mapvalidator.ValueStringsAre(
						stringvalidator.OneOf("read", "write"),
					),
					readOnlyAssets(readOnlyAssetTypes),
				},
			},
			"telemetry": schema.SetAttribute{
				Required:    true,
				Description: "Telemetry signals the role can read: 'log', 'metric', 'trace', 'rum'. An empty set grants no telemetry access",
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(
						stringvalidator.OneOf("log", "metric", "trace", "rum"),
					),
				},
			},
			"telemetry_query": schema.ListNestedAttribute{
				Optional: true,
				Description: "Additional restriction applied cluster-side to every telemetry query the role holder runs. " +
					"Each entry keeps only the data points whose key matches one of its values, and entries are combined with AND",
				Validators: []validator.List{
					listvalidator.SizeAtMost(3),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:    true,
							Description: "Attribute key matched against telemetry data points, such as 'env' or 'service.name'",
							Validators: []validator.String{
								stringvalidator.LengthBetween(1, 250),
							},
						},
						"values": schema.ListAttribute{
							Required:    true,
							Description: "Values the attribute must equal for a data point to be kept. A data point matching any of them is kept",
							ElementType: types.StringType,
							Validators: []validator.List{
								listvalidator.SizeBetween(1, 20),
								listvalidator.ValueStringsAre(
									stringvalidator.LengthBetween(1, 250),
									stringvalidator.NoneOf("*"),
								),
							},
						},
					},
				},
			},
		},
	}
}

type CustomRoleModel struct {
	Id             types.String    `tfsdk:"id"`
	Name           types.String    `tfsdk:"name"`
	Description    types.String    `tfsdk:"description"`
	TeamScope      *TeamScopeModel `tfsdk:"team_scope"`
	Assets         types.Map       `tfsdk:"assets"`
	Telemetry      types.Set       `tfsdk:"telemetry"`
	TelemetryQuery types.List      `tfsdk:"telemetry_query"`
}

type TeamScopeModel struct {
	Type    types.String `tfsdk:"type"`
	TeamIds types.Set    `tfsdk:"team_ids"`
}

type TelemetryQueryModel struct {
	Key    types.String `tfsdk:"key"`
	Values types.List   `tfsdk:"values"`
}
