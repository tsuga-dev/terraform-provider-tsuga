package resource_route

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-tsuga/internal/resource_team"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// MaxSplitDepth caps how many nested split processors we model without causing
// Terraform's type system to recurse infinitely.
const MaxSplitDepth = 8

func RouteResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "Log route allowing to standardize logs and enrich them with additional data",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the log route",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human readable name shown for the route",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"description": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(50000),
				},
			},
			"is_enabled": schema.BoolAttribute{
				Required: true,
			},
			"query": schema.StringAttribute{
				Required:    true,
				Description: "Query that selects which logs should enter the route",
				Validators: []validator.String{
					stringvalidator.LengthAtMost(50000),
				},
			},
			"owner": schema.StringAttribute{
				Required:    true,
				Description: "Team ID owning and managing the route",
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
			"processors": func() schema.Attribute {
				attr := processorsListAttribute(ctx, MaxSplitDepth)
				attr.Description = "Ordered processors applied to logs that match the route"
				return attr
			}(),
		},
	}
}

func processorsListAttribute(ctx context.Context, depth int) schema.ListNestedAttribute {
	var splitAttr schema.Attribute = schema.SingleNestedAttribute{Computed: true}
	if depth > 0 {
		splitAttr = splitSchema(ctx, depth-1)
	}

	return schema.ListNestedAttribute{
		Required: true,
		Validators: []validator.List{
			listvalidator.SizeAtMost(50),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{
					Required:    true,
					Description: "Identifier of the processor",
				},
				"name": schema.StringAttribute{
					Optional:    true,
					Description: "Display name of the processor",
					Validators: []validator.String{
						stringvalidator.LengthAtMost(250),
					},
				},
				"description": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(50000),
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
				"mapper": schema.SingleNestedAttribute{
					Optional: true,
					Attributes: map[string]schema.Attribute{
						"map_attributes": schema.ListNestedAttribute{
							Optional:    true,
							Description: "Mappings that map individual attributes to new targets",
							Validators: []validator.List{
								listvalidator.SizeAtMost(50),
							},
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"origin_attribute": schema.StringAttribute{
										Required:    true,
										Description: "Attribute name to map to the target attribute",
									},
									"target_attribute": schema.StringAttribute{
										Required:    true,
										Description: "Attribute name that will receive the mapped value",
									},
									"keep_origin": schema.BoolAttribute{
										Optional:    true,
										Computed:    true,
										Description: "Preserve the source attribute after mapping (defaults to false)",
									},
									"override_target": schema.BoolAttribute{
										Optional:    true,
										Computed:    true,
										Description: "Overwrite the target attribute when it already exists (defaults to true)",
									},
								},
							},
						},
						"map_level": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"attribute_name": schema.StringAttribute{
									Required:    true,
									Description: "Attribute whose value will determine the log level",
								},
							},
						},
						"map_timestamp": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"attribute_name": schema.StringAttribute{
									Required:    true,
									Description: "Attribute whose value will determine the log timestamp",
								},
							},
						},
					},
				},
				"parse_attribute": schema.SingleNestedAttribute{
					Optional: true,
					Attributes: map[string]schema.Attribute{
						"grok": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"attribute_name": schema.StringAttribute{
									Required:    true,
									Description: "Attribute whose value will be parsed with Grok rules",
								},
								"rules": schema.ListAttribute{
									Required:    true,
									Description: "Ordered Grok rules evaluated until one matches",
									ElementType: types.StringType,
									Validators: []validator.List{
										listvalidator.SizeAtMost(5),
									},
								},
								"samples": schema.ListAttribute{
									Optional:    true,
									Computed:    true,
									Description: "Example log lines for validation",
									ElementType: types.StringType,
									Validators: []validator.List{
										listvalidator.SizeAtMost(5),
									},
								},
							},
						},
						"url": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"source_attribute": schema.StringAttribute{
									Required:    true,
									Description: "Attribute containing the URL to parse",
								},
							},
						},
						"user_agent": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"source_attribute": schema.StringAttribute{
									Required:    true,
									Description: "Attribute containing the user agent string to parse",
								},
							},
						},
						"key_value": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"source_attribute": schema.StringAttribute{
									Required:    true,
									Description: "Attribute containing the key/value string segment to parse",
								},
								"target_attribute": schema.StringAttribute{
									Required:    true,
									Description: "Attribute prefix where extracted key/value pairs will be written",
								},
								"key_value_splitter": schema.StringAttribute{
									Required:    true,
									Description: "Delimiter separating keys from values in the source string",
								},
								"pairs_splitter": schema.StringAttribute{
									Required:    true,
									Description: "Delimiter separating each key/value pair",
								},
								"accept_standalone_key": schema.BoolAttribute{Optional: true, Computed: true},
							},
						},
					},
				},
				"creator": schema.SingleNestedAttribute{
					Optional: true,
					Attributes: map[string]schema.Attribute{
						"format_string": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"target_attribute": schema.StringAttribute{
									Required:    true,
									Description: "Attribute that will receive the formatted value",
								},
								"format_string": schema.StringAttribute{
									Required:    true,
									Description: "Template string used to build the target attribute value",
								},
								"override_target": schema.BoolAttribute{
									Optional:    true,
									Computed:    true,
									Description: "Set to true to overwrite an existing target attribute value (defaults to true)",
								},
								"replace_missing_by_empty": schema.BoolAttribute{Optional: true, Computed: true},
							},
						},
						"math_formula": schema.SingleNestedAttribute{
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"target_attribute": schema.StringAttribute{
									Required:    true,
									Description: "Attribute that will receive the computed value",
								},
								"formula": schema.StringAttribute{
									Required:    true,
									Description: "Mathematical formula evaluated to populate the target attribute",
								},
								"override_target": schema.BoolAttribute{
									Optional:    true,
									Computed:    true,
									Description: "Set to true to overwrite an existing target attribute value (defaults to true)",
								},
								"replace_missing_by_0": schema.BoolAttribute{Optional: true, Computed: true},
							},
						},
					},
				},
				"split": splitAttr,
			},
		},
	}
}

func splitSchema(ctx context.Context, depth int) schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"items": schema.ListNestedAttribute{
				Required:    true,
				Description: "Conditional branches evaluated in order before falling back to the default",
				Validators: []validator.List{
					listvalidator.SizeAtMost(11),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"query": schema.StringAttribute{
							Required:    true,
							Description: "Query that determines whether logs enter this branch",
							Validators: []validator.String{
								stringvalidator.LengthAtMost(50000),
							},
						},
						"processors": func() schema.Attribute {
							attr := processorsListAttribute(ctx, depth)
							attr.Description = "Processors executed when the branch query matches"
							return attr
						}(),
					},
				},
			},
		},
	}
}

type RouteModel struct {
	Id          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	IsEnabled   types.Bool   `tfsdk:"is_enabled"`
	Query       types.String `tfsdk:"query"`
	Owner       types.String `tfsdk:"owner"`
	Tags        types.List   `tfsdk:"tags"`
	Processors  types.List   `tfsdk:"processors"`
}

type ProcessorModel struct {
	Id             types.String         `tfsdk:"id"`
	Name           types.String         `tfsdk:"name"`
	Description    types.String         `tfsdk:"description"`
	Tags           types.List           `tfsdk:"tags"`
	Mapper         *MapperModel         `tfsdk:"mapper"`
	ParseAttribute *ParseAttributeModel `tfsdk:"parse_attribute"`
	Creator        *CreatorModel        `tfsdk:"creator"`
	Split          types.Object         `tfsdk:"split"`
}

type MapperModel struct {
	MapAttributes []MapAttributeModel   `tfsdk:"map_attributes"`
	MapLevel      *MapperLevelModel     `tfsdk:"map_level"`
	MapTimestamp  *MapperTimestampModel `tfsdk:"map_timestamp"`
}

type MapAttributeModel struct {
	OriginAttribute types.String `tfsdk:"origin_attribute"`
	TargetAttribute types.String `tfsdk:"target_attribute"`
	KeepOrigin      types.Bool   `tfsdk:"keep_origin"`
	OverrideTarget  types.Bool   `tfsdk:"override_target"`
}

type MapperLevelModel struct {
	AttributeName types.String `tfsdk:"attribute_name"`
}

type MapperTimestampModel struct {
	AttributeName types.String `tfsdk:"attribute_name"`
}

type ParseAttributeModel struct {
	Grok      *ParseGrokModel      `tfsdk:"grok"`
	URL       *ParseURLModel       `tfsdk:"url"`
	UserAgent *ParseUserAgentModel `tfsdk:"user_agent"`
	KeyValue  *ParseKeyValueModel  `tfsdk:"key_value"`
}

type ParseGrokModel struct {
	AttributeName types.String `tfsdk:"attribute_name"`
	Rules         types.List   `tfsdk:"rules"`
	Samples       types.List   `tfsdk:"samples"`
}

type ParseURLModel struct {
	SourceAttribute types.String `tfsdk:"source_attribute"`
}

type ParseUserAgentModel struct {
	SourceAttribute types.String `tfsdk:"source_attribute"`
}

type ParseKeyValueModel struct {
	SourceAttribute     types.String `tfsdk:"source_attribute"`
	TargetAttribute     types.String `tfsdk:"target_attribute"`
	KeyValueSplitter    types.String `tfsdk:"key_value_splitter"`
	PairsSplitter       types.String `tfsdk:"pairs_splitter"`
	AcceptStandaloneKey types.Bool   `tfsdk:"accept_standalone_key"`
}

type CreatorModel struct {
	FormatString *CreatorFormatStringModel `tfsdk:"format_string"`
	MathFormula  *CreatorMathFormulaModel  `tfsdk:"math_formula"`
}

type CreatorFormatStringModel struct {
	TargetAttribute       types.String `tfsdk:"target_attribute"`
	FormatString          types.String `tfsdk:"format_string"`
	OverrideTarget        types.Bool   `tfsdk:"override_target"`
	ReplaceMissingByEmpty types.Bool   `tfsdk:"replace_missing_by_empty"`
}

type CreatorMathFormulaModel struct {
	TargetAttribute   types.String `tfsdk:"target_attribute"`
	Formula           types.String `tfsdk:"formula"`
	OverrideTarget    types.Bool   `tfsdk:"override_target"`
	ReplaceMissingBy0 types.Bool   `tfsdk:"replace_missing_by_0"`
}

type SplitModel struct {
	Items []SplitItemModel `tfsdk:"items"`
}

type SplitItemModel struct {
	Query      types.String `tfsdk:"query"`
	Processors types.List   `tfsdk:"processors"`
}

// ProcessorAttrTypesAtDepth returns attribute types for processors while
// honoring the split depth budget so recursive schemas terminate after
// MaxSplitDepth levels.
func ProcessorAttrTypesAtDepth(ctx context.Context, depth int) map[string]attr.Type {
	if depth < 0 {
		depth = 0
	}
	if depth > MaxSplitDepth {
		depth = MaxSplitDepth
	}
	return processorAttrTypesWithDepth(ctx, depth)
}

func processorAttrTypesWithDepth(ctx context.Context, depth int) map[string]attr.Type {
	attrTypes := map[string]attr.Type{
		"id":              types.StringType,
		"name":            types.StringType,
		"description":     types.StringType,
		"tags":            types.ListType{ElemType: types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)}},
		"mapper":          types.ObjectType{AttrTypes: MapperAttrTypes()},
		"parse_attribute": types.ObjectType{AttrTypes: ParseAttributeAttrTypes()},
		"creator":         types.ObjectType{AttrTypes: CreatorAttrTypes()},
	}

	if depth > 0 {
		attrTypes["split"] = types.ObjectType{AttrTypes: splitAttrTypesWithDepth(ctx, depth-1)}
	} else {
		attrTypes["split"] = types.ObjectType{AttrTypes: map[string]attr.Type{}}
	}

	return attrTypes
}

func MapperAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"map_attributes": types.ListType{ElemType: types.ObjectType{AttrTypes: MapAttributeAttrTypes()}},
		"map_level":      types.ObjectType{AttrTypes: MapperLevelAttrTypes()},
		"map_timestamp":  types.ObjectType{AttrTypes: MapperTimestampAttrTypes()},
	}
}

func MapAttributeAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"origin_attribute": types.StringType,
		"target_attribute": types.StringType,
		"keep_origin":      types.BoolType,
		"override_target":  types.BoolType,
	}
}

func MapperLevelAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"attribute_name": types.StringType,
	}
}

func MapperTimestampAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"attribute_name": types.StringType,
	}
}

func ParseAttributeAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"grok":       types.ObjectType{AttrTypes: ParseGrokAttrTypes()},
		"url":        types.ObjectType{AttrTypes: ParseURLAttrTypes()},
		"user_agent": types.ObjectType{AttrTypes: ParseUserAgentAttrTypes()},
		"key_value":  types.ObjectType{AttrTypes: ParseKeyValueAttrTypes()},
	}
}

func ParseGrokAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"attribute_name": types.StringType,
		"rules":          types.ListType{ElemType: types.StringType},
		"samples":        types.ListType{ElemType: types.StringType},
	}
}

func ParseURLAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"source_attribute": types.StringType,
	}
}

func ParseUserAgentAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"source_attribute": types.StringType,
	}
}

func ParseKeyValueAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"source_attribute":      types.StringType,
		"target_attribute":      types.StringType,
		"key_value_splitter":    types.StringType,
		"pairs_splitter":        types.StringType,
		"accept_standalone_key": types.BoolType,
	}
}

func CreatorAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"format_string": types.ObjectType{AttrTypes: CreatorFormatStringAttrTypes()},
		"math_formula":  types.ObjectType{AttrTypes: CreatorMathFormulaAttrTypes()},
	}
}

func CreatorFormatStringAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"target_attribute":         types.StringType,
		"format_string":            types.StringType,
		"override_target":          types.BoolType,
		"replace_missing_by_empty": types.BoolType,
	}
}

func CreatorMathFormulaAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"target_attribute":     types.StringType,
		"formula":              types.StringType,
		"override_target":      types.BoolType,
		"replace_missing_by_0": types.BoolType,
	}
}

// SplitAttrTypesAtDepth mirrors ProcessorAttrTypesAtDepth but for the split
// object hierarchy, ensuring the schema never recurses beyond the allowed
// depth.
func SplitAttrTypesAtDepth(ctx context.Context, depth int) map[string]attr.Type {
	if depth < 0 {
		return map[string]attr.Type{}
	}
	if depth > MaxSplitDepth-1 {
		depth = MaxSplitDepth - 1
	}
	return splitAttrTypesWithDepth(ctx, depth)
}

// SplitItemAttrTypesAtDepth ensures nested split processors only recurse while
// the depth budget remains, giving us predictable object types for Terraform.
func SplitItemAttrTypesAtDepth(ctx context.Context, depth int) map[string]attr.Type {
	if depth < 0 {
		return map[string]attr.Type{}
	}
	if depth > MaxSplitDepth-1 {
		depth = MaxSplitDepth - 1
	}
	return splitItemAttrTypesWithDepth(ctx, depth)
}

func splitAttrTypesWithDepth(ctx context.Context, depth int) map[string]attr.Type {
	if depth < 0 {
		return map[string]attr.Type{}
	}
	return map[string]attr.Type{
		"items": types.ListType{ElemType: types.ObjectType{AttrTypes: splitItemAttrTypesWithDepth(ctx, depth)}},
	}
}

func splitItemAttrTypesWithDepth(ctx context.Context, depth int) map[string]attr.Type {
	if depth < 0 {
		return map[string]attr.Type{}
	}
	return map[string]attr.Type{
		"query":      types.StringType,
		"processors": types.ListType{ElemType: types.ObjectType{AttrTypes: processorAttrTypesWithDepth(ctx, depth)}},
	}
}
