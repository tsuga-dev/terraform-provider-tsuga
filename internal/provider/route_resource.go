package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_route"
	"terraform-provider-tsuga/internal/resource_team"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.Resource = (*routeResource)(nil)
var _ resource.ResourceWithConfigure = (*routeResource)(nil)
var _ resource.ResourceWithImportState = (*routeResource)(nil)
var _ resource.ResourceWithValidateConfig = (*routeResource)(nil)

func NewRouteResource() resource.Resource {
	return &routeResource{}
}

type routeResource struct {
	client *TsugaClient
}

func (r *routeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*TsugaClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *TsugaClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *routeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_route"
}

func (r *routeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_route.RouteResourceSchema(ctx)
}

func (r *routeResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_route.RouteModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate processors: each processor must have exactly one of mapper, parse_attribute, creator, or split
	if !config.Processors.IsNull() && !config.Processors.IsUnknown() {
		diags := r.validateProcessors(ctx, config.Processors, resource_route.MaxSplitDepth, "processors")
		resp.Diagnostics.Append(diags...)
	}
}

func (r *routeResource) validateProcessors(ctx context.Context, processors types.List, depth int, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics
	if processors.IsNull() || processors.IsUnknown() {
		return diags
	}

	var models []resource_route.ProcessorModel
	diags.Append(processors.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return diags
	}

	for i, proc := range models {
		setCount := 0
		if proc.Mapper != nil {
			setCount++
		}
		if proc.ParseAttribute != nil {
			setCount++
		}
		if proc.Creator != nil {
			setCount++
		}
		if !proc.Split.IsNull() && !proc.Split.IsUnknown() {
			setCount++
		}

		if setCount != 1 {
			diags.AddError(
				"Invalid processor configuration",
				fmt.Sprintf("%s[%d]: exactly one of mapper, parse_attribute, creator, or split must be set.", pathPrefix, i),
			)
		}

		// Validate nested processors in split items
		if !proc.Split.IsNull() && !proc.Split.IsUnknown() && depth > 0 {
			var splitModel resource_route.SplitModel
			diags.Append(proc.Split.As(ctx, &splitModel, basetypes.ObjectAsOptions{})...)
			if diags.HasError() {
				continue
			}

			for j, item := range splitModel.Items {
				if !item.Processors.IsNull() && !item.Processors.IsUnknown() {
					nestedDiags := r.validateProcessors(ctx, item.Processors, depth-1, fmt.Sprintf("%s[%d].split.items[%d].processors", pathPrefix, i, j))
					diags.Append(nestedDiags...)
				}
			}
		}
	}

	return diags
}

func (r *routeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *routeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_route.RouteModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildRouteRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdateRoute(ctx, http.MethodPost, "/v1/routes", requestBody, "create")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *routeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_route.RouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/routes/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read route: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read route: %s", err))
		return
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp routeAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	newState, diags := flattenRoute(ctx, apiResp.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *routeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_route.RouteModel
	var state resource_route.RouteModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildRouteRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/routes/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateRoute(ctx, http.MethodPut, path, requestBody, "update")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *routeResource) buildRouteRequestBody(ctx context.Context, plan resource_route.RouteModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	processors, expandDiags := expandRouteProcessors(ctx, plan.Processors, resource_route.MaxSplitDepth)
	diags.Append(expandDiags...)
	tags, tagDiags := expandTags(ctx, plan.Tags)
	diags.Append(tagDiags...)
	if diags.HasError() {
		return nil, diags
	}

	body := map[string]interface{}{
		"name":       plan.Name.ValueString(),
		"isEnabled":  plan.IsEnabled.ValueBool(),
		"query":      plan.Query.ValueString(),
		"owner":      plan.Owner.ValueString(),
		"processors": processors,
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		body["description"] = plan.Description.ValueString()
	}
	if tags != nil {
		body["tags"] = tags
	}

	return body, diags
}

func (r *routeResource) createOrUpdateRoute(ctx context.Context, method, path string, requestBody map[string]interface{}, operation string) (resource_route.RouteModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s route: %s", operation, err))
		return resource_route.RouteModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s route: %s", operation, err))
		return resource_route.RouteModel{}, diags
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return resource_route.RouteModel{}, diags
	}

	var apiResp routeAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return resource_route.RouteModel{}, diags
	}

	newState, flattenDiags := flattenRoute(ctx, apiResp.Data)
	diags.Append(flattenDiags...)
	if diags.HasError() {
		return resource_route.RouteModel{}, diags
	}

	return newState, diags
}

func (r *routeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_route.RouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/routes/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete route: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete route: %s", err))
			return
		}
	}
}

type routeAPIResponse struct {
	Data routeAPIData `json:"data"`
}

type routeAPIData struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	IsEnabled   bool                `json:"isEnabled"`
	Query       string              `json:"query"`
	Owner       string              `json:"owner"`
	Tags        []apiTag            `json:"tags"`
	Processors  []routeAPIProcessor `json:"processors"`
}

type routeAPIProcessor struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Tags        []apiTag               `json:"tags,omitempty"`
	Type        string                 `json:"type"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

func expandRouteProcessors(ctx context.Context, processors types.List, depth int) ([]routeAPIProcessor, diag.Diagnostics) {
	var diags diag.Diagnostics
	if processors.IsNull() || processors.IsUnknown() {
		return nil, diags
	}

	var models []resource_route.ProcessorModel
	diags.Append(processors.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]routeAPIProcessor, 0, len(models))
	for _, p := range models {
		setCount := 0
		apiProc := routeAPIProcessor{
			ID:          p.Id.ValueString(),
			Name:        p.Name.ValueString(),
			Description: p.Description.ValueString(),
		}
		if tags, td := expandTags(ctx, p.Tags); td.HasError() {
			diags.Append(td...)
			return nil, diags
		} else if tags != nil {
			apiProc.Tags = tags
		}

		if p.Mapper != nil {
			setCount++
			apiProc.Type = "mapper"
			apiProc.Params = expandMapperParams(p.Mapper)
		}
		if p.ParseAttribute != nil {
			setCount++
			apiProc.Type = "parse-attribute"
			params, pd := expandParseAttributeParams(ctx, p.ParseAttribute)
			diags.Append(pd...)
			if diags.HasError() {
				return nil, diags
			}
			apiProc.Params = params
		}
		if p.Creator != nil {
			setCount++
			apiProc.Type = "creator"
			params, pd := expandCreatorParams(p.Creator)
			diags.Append(pd...)
			if diags.HasError() {
				return nil, diags
			}
			apiProc.Params = params
		}

		if !p.Split.IsNull() && !p.Split.IsUnknown() {
			if depth <= 0 {
				diags.AddError("split depth exceeded", "nested split processors exceed the supported depth limit")
				return nil, diags
			}
			setCount++
			apiProc.Type = "split"
			var splitModel resource_route.SplitModel
			diags.Append(p.Split.As(ctx, &splitModel, basetypes.ObjectAsOptions{})...)
			if diags.HasError() {
				return nil, diags
			}
			items, id := expandSplitItems(ctx, splitModel.Items, depth-1)
			diags.Append(id...)
			if diags.HasError() {
				return nil, diags
			}
			apiProc.Params = map[string]interface{}{
				"items": items,
			}
		}

		if setCount != 1 {
			diags.AddError("Invalid processor", "Exactly one of mapper, parse_attribute, creator, split must be set.")
			return nil, diags
		}

		result = append(result, apiProc)
	}

	return result, diags
}

func expandMapperParams(m *resource_route.MapperModel) map[string]interface{} {
	if len(m.MapAttributes) > 0 {
		attrs := make([]map[string]interface{}, 0, len(m.MapAttributes))
		for _, a := range m.MapAttributes {
			attrs = append(attrs, map[string]interface{}{
				"subtype":         "map-attributes",
				"originAttribute": a.OriginAttribute.ValueString(),
				"targetAttribute": a.TargetAttribute.ValueString(),
				"keepOrigin":      a.KeepOrigin.ValueBool(),
				"overrideTarget":  a.OverrideTarget.ValueBool(),
			})
		}
		return map[string]interface{}{
			"subtype":    "map-attributes",
			"attributes": attrs,
		}
	}
	if m.MapLevel != nil {
		return map[string]interface{}{
			"subtype":       "map-level",
			"attributeName": m.MapLevel.AttributeName.ValueString(),
		}
	}
	if m.MapTimestamp != nil {
		return map[string]interface{}{
			"subtype":       "map-timestamp",
			"attributeName": m.MapTimestamp.AttributeName.ValueString(),
		}
	}
	return map[string]interface{}{}
}

func expandParseAttributeParams(ctx context.Context, p *resource_route.ParseAttributeModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if p.Grok != nil {
		rules, rd := expandStringList(ctx, p.Grok.Rules)
		diags.Append(rd...)
		samples, sd := expandStringList(ctx, p.Grok.Samples)
		diags.Append(sd...)
		return map[string]interface{}{
			"subtype":       "grok",
			"attributeName": p.Grok.AttributeName.ValueString(),
			"rules":         rules,
			"samples":       samples,
		}, diags
	}
	if p.URL != nil {
		return map[string]interface{}{
			"subtype":         "url",
			"sourceAttribute": p.URL.SourceAttribute.ValueString(),
		}, diags
	}
	if p.UserAgent != nil {
		return map[string]interface{}{
			"subtype":         "user-agent",
			"sourceAttribute": p.UserAgent.SourceAttribute.ValueString(),
		}, diags
	}
	if p.KeyValue != nil {
		params := map[string]interface{}{
			"subtype":          "key-value",
			"sourceAttribute":  p.KeyValue.SourceAttribute.ValueString(),
			"targetAttribute":  p.KeyValue.TargetAttribute.ValueString(),
			"keyValueSplitter": p.KeyValue.KeyValueSplitter.ValueString(),
			"pairsSplitter":    p.KeyValue.PairsSplitter.ValueString(),
		}
		if !p.KeyValue.AcceptStandaloneKey.IsNull() && !p.KeyValue.AcceptStandaloneKey.IsUnknown() {
			params["acceptStandaloneKey"] = p.KeyValue.AcceptStandaloneKey.ValueBool()
		}
		return params, diags
	}
	diags.AddError("Invalid parse_attribute", "One parse_attribute block must be set.")
	return nil, diags
}

func expandCreatorParams(c *resource_route.CreatorModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if c.FormatString != nil {
		return map[string]interface{}{
			"subtype":               "format-string",
			"targetAttribute":       c.FormatString.TargetAttribute.ValueString(),
			"formatString":          c.FormatString.FormatString.ValueString(),
			"overrideTarget":        c.FormatString.OverrideTarget.ValueBool(),
			"replaceMissingByEmpty": c.FormatString.ReplaceMissingByEmpty.ValueBool(),
		}, diags
	}
	if c.MathFormula != nil {
		return map[string]interface{}{
			"subtype":           "math-formula",
			"targetAttribute":   c.MathFormula.TargetAttribute.ValueString(),
			"formula":           c.MathFormula.Formula.ValueString(),
			"overrideTarget":    c.MathFormula.OverrideTarget.ValueBool(),
			"replaceMissingBy0": c.MathFormula.ReplaceMissingBy0.ValueBool(),
		}, diags
	}
	diags.AddError("Invalid creator", "One creator block must be set.")
	return nil, diags
}

func expandSplitItems(ctx context.Context, items []resource_route.SplitItemModel, depth int) ([]map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		procs, pd := expandRouteProcessors(ctx, item.Processors, depth)
		diags.Append(pd...)
		if diags.HasError() {
			return nil, diags
		}
		result = append(result, map[string]interface{}{
			"query":      item.Query.ValueString(),
			"processors": procs,
		})
	}
	return result, diags
}

func flattenRoute(ctx context.Context, data routeAPIData) (resource_route.RouteModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tags, td := flattenTags(ctx, data.Tags)
	diags.Append(td...)

	procs, pd := flattenRouteProcessors(ctx, data.Processors, resource_route.MaxSplitDepth)
	diags.Append(pd...)

	tagsList := tags
	if tagsList.IsNull() {
		tagsList = types.ListNull(types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)})
	}

	query := types.StringValue(data.Query)

	state := resource_route.RouteModel{
		Id:          types.StringValue(data.ID),
		Name:        types.StringValue(data.Name),
		Description: stringValueOrNull(data.Description),
		IsEnabled:   types.BoolValue(data.IsEnabled),
		Query:       query,
		Owner:       types.StringValue(data.Owner),
		Tags:        tagsList,
		Processors:  procs,
	}

	return state, diags
}

func flattenRouteProcessors(ctx context.Context, procs []routeAPIProcessor, depth int) (types.List, diag.Diagnostics) {
	attrTypes := resource_route.ProcessorAttrTypesAtDepth(ctx, depth)
	elemType := types.ObjectType{AttrTypes: attrTypes}
	if len(procs) == 0 {
		return types.ListValue(elemType, []attr.Value{})
	}

	values := make([]attr.Value, 0, len(procs))
	for _, p := range procs {
		obj := map[string]attr.Value{
			"id":              stringValueOrNull(p.ID),
			"name":            stringValueOrNull(p.Name),
			"description":     stringValueOrNull(p.Description),
			"tags":            types.ListNull(types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)}),
			"mapper":          types.ObjectNull(resource_route.MapperAttrTypes(ctx)),
			"parse_attribute": types.ObjectNull(resource_route.ParseAttributeAttrTypes(ctx)),
			"creator":         types.ObjectNull(resource_route.CreatorAttrTypes(ctx)),
			"split": func() attr.Value {
				if st, ok := attrTypes["split"].(types.ObjectType); ok {
					return types.ObjectNull(st.AttrTypes)
				}
				return types.DynamicNull()
			}(),
		}
		if len(p.Tags) > 0 {
			tagsVal, td := flattenTags(ctx, p.Tags)
			if td.HasError() {
				return types.ListNull(elemType), td
			}
			obj["tags"] = tagsVal
		}

		switch p.Type {
		case "mapper":
			obj["mapper"] = flattenMapper(ctx, p.Params)
		case "parse-attribute":
			obj["parse_attribute"] = flattenParseAttribute(ctx, p.Params)
		case "creator":
			obj["creator"] = flattenCreator(ctx, p.Params)
		case "split":
			if depth <= 0 {
				return types.ListNull(elemType), diag.Diagnostics{diag.NewErrorDiagnostic("split depth exceeded", "API returned nested split processors beyond the supported depth limit")}
			}
			splitVal, diags := flattenSplit(ctx, p.Params, depth-1)
			if diags.HasError() {
				return types.ListNull(elemType), diags
			}
			obj["split"] = splitVal
		}

		values = append(values, types.ObjectValueMust(attrTypes, obj))
	}

	return types.ListValue(elemType, values)
}

func flattenMapper(ctx context.Context, params map[string]interface{}) attr.Value {
	mapperAttrs := resource_route.MapperAttrTypes(ctx)
	mapAttrs := types.ListNull(types.ObjectType{AttrTypes: resource_route.MapAttributeAttrTypes(ctx)})
	if attrsRaw, ok := params["attributes"].([]interface{}); ok && len(attrsRaw) > 0 {
		vals := make([]attr.Value, 0, len(attrsRaw))
		for _, a := range attrsRaw {
			m, _ := a.(map[string]interface{})
			keepVal, keepOK := boolFromMap(m, "keepOrigin")
			overrideVal, overrideOK := boolFromMap(m, "overrideTarget")
			vals = append(vals, types.ObjectValueMust(resource_route.MapAttributeAttrTypes(ctx), map[string]attr.Value{
				"origin_attribute": types.StringValue(fmt.Sprintf("%v", m["originAttribute"])),
				"target_attribute": types.StringValue(fmt.Sprintf("%v", m["targetAttribute"])),
				"keep_origin": func() types.Bool {
					if keepOK {
						return types.BoolValue(keepVal)
					}
					return types.BoolNull()
				}(),
				"override_target": func() types.Bool {
					if overrideOK {
						return types.BoolValue(overrideVal)
					}
					return types.BoolNull()
				}(),
			}))
		}
		mapAttrs, _ = types.ListValue(types.ObjectType{AttrTypes: resource_route.MapAttributeAttrTypes(ctx)}, vals)
	}

	mapLevel := types.ObjectNull(resource_route.MapperLevelAttrTypes(ctx))
	mapTimestamp := types.ObjectNull(resource_route.MapperTimestampAttrTypes(ctx))
	if subtype, _ := params["subtype"].(string); subtype == "map-level" {
		mapLevel = types.ObjectValueMust(resource_route.MapperLevelAttrTypes(ctx), map[string]attr.Value{
			"attribute_name": types.StringValue(fmt.Sprintf("%v", params["attributeName"])),
		})
	} else if subtype == "map-timestamp" {
		mapTimestamp = types.ObjectValueMust(resource_route.MapperTimestampAttrTypes(ctx), map[string]attr.Value{
			"attribute_name": types.StringValue(fmt.Sprintf("%v", params["attributeName"])),
		})
	}

	return types.ObjectValueMust(mapperAttrs, map[string]attr.Value{
		"map_attributes": mapAttrs,
		"map_level":      mapLevel,
		"map_timestamp":  mapTimestamp,
	})
}

func flattenParseAttribute(ctx context.Context, params map[string]interface{}) attr.Value {
	attrTypes := resource_route.ParseAttributeAttrTypes(ctx)
	nullVal := types.ObjectNull(attrTypes)

	subtype, _ := params["subtype"].(string)
	switch subtype {
	case "grok":
		rules, rulesOk := sliceToStrings(params["rules"])
		var rulesVal types.List
		if rulesOk {
			rulesVal, _ = types.ListValueFrom(ctx, types.StringType, rules)
		} else {
			rulesVal = types.ListNull(types.StringType)
		}
		var samplesVal types.List
		if samples, ok := sliceToStrings(params["samples"]); ok {
			samplesVal, _ = types.ListValueFrom(ctx, types.StringType, samples)
		} else {
			samplesVal = types.ListNull(types.StringType)
		}
		return types.ObjectValueMust(attrTypes, map[string]attr.Value{
			"grok": types.ObjectValueMust(resource_route.ParseGrokAttrTypes(ctx), map[string]attr.Value{
				"attribute_name": types.StringValue(fmt.Sprintf("%v", params["attributeName"])),
				"rules":          rulesVal,
				"samples":        samplesVal,
			}),
			"url":        types.ObjectNull(resource_route.ParseURLAttrTypes(ctx)),
			"user_agent": types.ObjectNull(resource_route.ParseUserAgentAttrTypes(ctx)),
			"key_value":  types.ObjectNull(resource_route.ParseKeyValueAttrTypes(ctx)),
		})
	case "url":
		return types.ObjectValueMust(attrTypes, map[string]attr.Value{
			"url": types.ObjectValueMust(resource_route.ParseURLAttrTypes(ctx), map[string]attr.Value{
				"source_attribute": types.StringValue(fmt.Sprintf("%v", params["sourceAttribute"])),
			}),
			"grok":       types.ObjectNull(resource_route.ParseGrokAttrTypes(ctx)),
			"user_agent": types.ObjectNull(resource_route.ParseUserAgentAttrTypes(ctx)),
			"key_value":  types.ObjectNull(resource_route.ParseKeyValueAttrTypes(ctx)),
		})
	case "user-agent":
		return types.ObjectValueMust(attrTypes, map[string]attr.Value{
			"user_agent": types.ObjectValueMust(resource_route.ParseUserAgentAttrTypes(ctx), map[string]attr.Value{
				"source_attribute": types.StringValue(fmt.Sprintf("%v", params["sourceAttribute"])),
			}),
			"grok":      types.ObjectNull(resource_route.ParseGrokAttrTypes(ctx)),
			"url":       types.ObjectNull(resource_route.ParseURLAttrTypes(ctx)),
			"key_value": types.ObjectNull(resource_route.ParseKeyValueAttrTypes(ctx)),
		})
	case "key-value":
		val, ok := boolFromMap(params, "acceptStandaloneKey")
		acceptVal := optionalBoolValue(ok, val)
		return types.ObjectValueMust(attrTypes, map[string]attr.Value{
			"key_value": types.ObjectValueMust(resource_route.ParseKeyValueAttrTypes(ctx), map[string]attr.Value{
				"source_attribute":      types.StringValue(fmt.Sprintf("%v", params["sourceAttribute"])),
				"target_attribute":      types.StringValue(fmt.Sprintf("%v", params["targetAttribute"])),
				"key_value_splitter":    types.StringValue(fmt.Sprintf("%v", params["keyValueSplitter"])),
				"pairs_splitter":        types.StringValue(fmt.Sprintf("%v", params["pairsSplitter"])),
				"accept_standalone_key": acceptVal,
			}),
			"grok":       types.ObjectNull(resource_route.ParseGrokAttrTypes(ctx)),
			"url":        types.ObjectNull(resource_route.ParseURLAttrTypes(ctx)),
			"user_agent": types.ObjectNull(resource_route.ParseUserAgentAttrTypes(ctx)),
		})
	default:
		return nullVal
	}
}

func flattenCreator(ctx context.Context, params map[string]interface{}) attr.Value {
	attrTypes := resource_route.CreatorAttrTypes(ctx)
	subtype, _ := params["subtype"].(string)
	formatNull := types.ObjectNull(resource_route.CreatorFormatStringAttrTypes(ctx))
	mathNull := types.ObjectNull(resource_route.CreatorMathFormulaAttrTypes(ctx))

	if subtype == "format-string" {
		overrideVal, okO := boolFromMap(params, "overrideTarget")
		replaceVal, okR := boolFromMap(params, "replaceMissingByEmpty")
		return types.ObjectValueMust(attrTypes, map[string]attr.Value{
			"format_string": types.ObjectValueMust(resource_route.CreatorFormatStringAttrTypes(ctx), map[string]attr.Value{
				"target_attribute": types.StringValue(fmt.Sprintf("%v", params["targetAttribute"])),
				"format_string":    types.StringValue(fmt.Sprintf("%v", params["formatString"])),
				"override_target": func() types.Bool {
					if okO {
						return types.BoolValue(overrideVal)
					}
					return types.BoolNull()
				}(),
				"replace_missing_by_empty": func() types.Bool {
					if okR {
						return types.BoolValue(replaceVal)
					}
					return types.BoolNull()
				}(),
			}),
			"math_formula": mathNull,
		})
	}
	if subtype == "math-formula" {
		overrideVal, okO := boolFromMap(params, "overrideTarget")
		replaceVal, okR := boolFromMap(params, "replaceMissingBy0")
		return types.ObjectValueMust(attrTypes, map[string]attr.Value{
			"math_formula": types.ObjectValueMust(resource_route.CreatorMathFormulaAttrTypes(ctx), map[string]attr.Value{
				"target_attribute": types.StringValue(fmt.Sprintf("%v", params["targetAttribute"])),
				"formula":          types.StringValue(fmt.Sprintf("%v", params["formula"])),
				"override_target": func() types.Bool {
					if okO {
						return types.BoolValue(overrideVal)
					}
					return types.BoolNull()
				}(),
				"replace_missing_by_0": func() types.Bool {
					if okR {
						return types.BoolValue(replaceVal)
					}
					return types.BoolNull()
				}(),
			}),
			"format_string": formatNull,
		})
	}

	return types.ObjectValueMust(attrTypes, map[string]attr.Value{
		"format_string": formatNull,
		"math_formula":  mathNull,
	})
}

func flattenSplit(ctx context.Context, params map[string]interface{}, depth int) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	itemsRaw, ok := params["items"].([]interface{})
	if !ok {
		return types.ObjectNull(resource_route.SplitAttrTypesAtDepth(ctx, depth)), diags
	}
	elemType := types.ObjectType{AttrTypes: resource_route.SplitItemAttrTypesAtDepth(ctx, depth)}
	items := make([]attr.Value, 0, len(itemsRaw))
	for _, it := range itemsRaw {
		m, _ := it.(map[string]interface{})
		procsRaw := m["processors"]
		procsSlice, _ := procsRaw.([]interface{})
		var procs []routeAPIProcessor
		for _, pr := range procsSlice {
			pm, _ := pr.(map[string]interface{})
			procs = append(procs, mapToProcessor(pm))
		}
		procVal, pd := flattenRouteProcessors(ctx, procs, depth)
		diags.Append(pd...)
		if diags.HasError() {
			return types.ObjectNull(resource_route.SplitAttrTypesAtDepth(ctx, depth)), diags
		}
		items = append(items, types.ObjectValueMust(resource_route.SplitItemAttrTypesAtDepth(ctx, depth), map[string]attr.Value{
			"query":      types.StringValue(fmt.Sprintf("%v", m["query"])),
			"processors": procVal,
		}))
	}
	listVal, _ := types.ListValue(elemType, items)
	return types.ObjectValueMust(resource_route.SplitAttrTypesAtDepth(ctx, depth), map[string]attr.Value{
		"items": listVal,
	}), diags
}

func mapToProcessor(m map[string]interface{}) routeAPIProcessor {
	params := map[string]interface{}{}
	if p, ok := m["params"].(map[string]interface{}); ok {
		params = p
	}
	var tags []apiTag
	if traw, ok := m["tags"].([]interface{}); ok {
		for _, t := range traw {
			if tm, ok := t.(map[string]interface{}); ok {
				tags = append(tags, apiTag{Key: fmt.Sprintf("%v", tm["key"]), Value: fmt.Sprintf("%v", tm["value"])})
			}
		}
	}
	return routeAPIProcessor{
		ID:          stringFromMap(m, "id"),
		Name:        stringFromMap(m, "name"),
		Description: stringFromMap(m, "description"),
		Type:        stringFromMap(m, "type"),
		Params:      params,
		Tags:        tags,
	}
}

func stringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func boolFromMap(m map[string]interface{}, key string) (bool, bool) {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b, true
		}
	}
	return false, false
}

func optionalBoolValue(ok bool, val bool) types.Bool {
	if ok {
		return types.BoolValue(val)
	}
	return types.BoolNull()
}

func sliceToStrings(v interface{}) ([]string, bool) {
	a, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	res := make([]string, 0, len(a))
	for _, el := range a {
		res = append(res, fmt.Sprintf("%v", el))
	}
	return res, true
}
