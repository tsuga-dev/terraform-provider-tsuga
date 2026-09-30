package resource_metric_route

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

func MetricRouteResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "Metric route allowing to standardize metrics and enrich them with additional data",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the metric route",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human readable name shown for the metric route",
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
			"metric_name_regex": schema.StringAttribute{
				Required:    true,
				Description: "Regex matched against metric names to decide which metrics enter the route processor chain",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"owner": schema.StringAttribute{
				Required:    true,
				Description: "Team ID owning and managing the metric route",
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
			"processors": schema.ListNestedAttribute{
				Required:    true,
				Description: "Ordered processors applied to metrics that match the route",
				Validators: []validator.List{
					listvalidator.SizeAtMost(50),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:    true,
							Description: "Identifier of the processor",
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
									Required:    true,
									Description: "Mappings that map individual attributes to new targets",
									Validators: []validator.List{
										listvalidator.SizeBetween(1, 50),
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
								"category": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"target_attribute": schema.StringAttribute{
											Required:    true,
											Description: "Attribute that will receive the category value",
											Validators: []validator.String{
												stringvalidator.LengthBetween(1, 250),
											},
										},
										"clauses": schema.ListNestedAttribute{
											Required:    true,
											Description: "Conditions evaluated in order to determine the category value",
											Validators: []validator.List{
												listvalidator.SizeBetween(1, 15),
											},
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"query": schema.StringAttribute{
														Required:    true,
														Description: "Query that selects the metrics assigned to this category",
														Validators: []validator.String{
															stringvalidator.LengthBetween(1, 10000),
														},
													},
													"value": schema.StringAttribute{
														Required:    true,
														Description: "Category value assigned when the query matches",
														Validators: []validator.String{
															stringvalidator.LengthBetween(1, 250),
														},
													},
												},
											},
										},
										"default_value": schema.StringAttribute{
											Optional:    true,
											Description: "Category value used when no condition matches",
											Validators: []validator.String{
												stringvalidator.LengthAtMost(250),
											},
										},
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

type MetricRouteModel struct {
	Id              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	IsEnabled       types.Bool   `tfsdk:"is_enabled"`
	MetricNameRegex types.String `tfsdk:"metric_name_regex"`
	Owner           types.String `tfsdk:"owner"`
	Tags            types.List   `tfsdk:"tags"`
	Processors      types.List   `tfsdk:"processors"`
}

type ProcessorModel struct {
	Id             types.String         `tfsdk:"id"`
	Description    types.String         `tfsdk:"description"`
	Tags           types.List           `tfsdk:"tags"`
	Mapper         *MapperModel         `tfsdk:"mapper"`
	ParseAttribute *ParseAttributeModel `tfsdk:"parse_attribute"`
	Creator        *CreatorModel        `tfsdk:"creator"`
}

type MapperModel struct {
	MapAttributes []MapAttributeModel `tfsdk:"map_attributes"`
}

type MapAttributeModel struct {
	OriginAttribute types.String `tfsdk:"origin_attribute"`
	TargetAttribute types.String `tfsdk:"target_attribute"`
	KeepOrigin      types.Bool   `tfsdk:"keep_origin"`
	OverrideTarget  types.Bool   `tfsdk:"override_target"`
}

type ParseAttributeModel struct {
	Grok *ParseGrokModel `tfsdk:"grok"`
}

type ParseGrokModel struct {
	AttributeName types.String `tfsdk:"attribute_name"`
	Rules         types.List   `tfsdk:"rules"`
	Samples       types.List   `tfsdk:"samples"`
}

type CreatorModel struct {
	FormatString *CreatorFormatStringModel `tfsdk:"format_string"`
	Category     *CreatorCategoryModel     `tfsdk:"category"`
}

type CreatorFormatStringModel struct {
	TargetAttribute       types.String `tfsdk:"target_attribute"`
	FormatString          types.String `tfsdk:"format_string"`
	OverrideTarget        types.Bool   `tfsdk:"override_target"`
	ReplaceMissingByEmpty types.Bool   `tfsdk:"replace_missing_by_empty"`
}

type CreatorCategoryModel struct {
	TargetAttribute types.String                 `tfsdk:"target_attribute"`
	Clauses         []CreatorCategoryClauseModel `tfsdk:"clauses"`
	DefaultValue    types.String                 `tfsdk:"default_value"`
}

type CreatorCategoryClauseModel struct {
	Query types.String `tfsdk:"query"`
	Value types.String `tfsdk:"value"`
}

func ProcessorAttrTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"id":              types.StringType,
		"description":     types.StringType,
		"tags":            types.ListType{ElemType: types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)}},
		"mapper":          types.ObjectType{AttrTypes: MapperAttrTypes()},
		"parse_attribute": types.ObjectType{AttrTypes: ParseAttributeAttrTypes()},
		"creator":         types.ObjectType{AttrTypes: CreatorAttrTypes()},
	}
}

func MapperAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"map_attributes": types.ListType{ElemType: types.ObjectType{AttrTypes: MapAttributeAttrTypes()}},
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

func ParseAttributeAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"grok": types.ObjectType{AttrTypes: ParseGrokAttrTypes()},
	}
}

func ParseGrokAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"attribute_name": types.StringType,
		"rules":          types.ListType{ElemType: types.StringType},
		"samples":        types.ListType{ElemType: types.StringType},
	}
}

func CreatorAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"format_string": types.ObjectType{AttrTypes: CreatorFormatStringAttrTypes()},
		"category":      types.ObjectType{AttrTypes: CreatorCategoryAttrTypes()},
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

func CreatorCategoryAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"target_attribute": types.StringType,
		"clauses":          types.ListType{ElemType: types.ObjectType{AttrTypes: CreatorCategoryClauseAttrTypes()}},
		"default_value":    types.StringType,
	}
}

func CreatorCategoryClauseAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"query": types.StringType,
		"value": types.StringType,
	}
}
