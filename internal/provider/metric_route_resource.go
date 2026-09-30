package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_metric_route"
	"terraform-provider-tsuga/internal/resource_team"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*metricRouteResource)(nil)
var _ resource.ResourceWithConfigure = (*metricRouteResource)(nil)
var _ resource.ResourceWithImportState = (*metricRouteResource)(nil)
var _ resource.ResourceWithValidateConfig = (*metricRouteResource)(nil)

func NewMetricRouteResource() resource.Resource {
	return &metricRouteResource{}
}

type metricRouteResource struct {
	client *TsugaClient
}

func (r *metricRouteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *metricRouteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_metric_route"
}

func (r *metricRouteResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_metric_route.MetricRouteResourceSchema(ctx)
}

func (r *metricRouteResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_metric_route.MetricRouteModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !config.Processors.IsNull() && !config.Processors.IsUnknown() {
		diags := r.validateProcessors(ctx, config.Processors, "processors")
		resp.Diagnostics.Append(diags...)
	}
}

func (r *metricRouteResource) validateProcessors(ctx context.Context, processors types.List, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics
	if processors.IsNull() || processors.IsUnknown() {
		return diags
	}

	var models []resource_metric_route.ProcessorModel
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
			if (proc.Creator.FormatString != nil) == (proc.Creator.Category != nil) {
				diags.AddError(
					"Invalid processor configuration",
					fmt.Sprintf("%s[%d].creator: exactly one of format_string or category must be set.", pathPrefix, i),
				)
			}
		}

		if setCount != 1 {
			diags.AddError(
				"Invalid processor configuration",
				fmt.Sprintf("%s[%d]: exactly one of mapper, parse_attribute, or creator must be set.", pathPrefix, i),
			)
		}
	}

	return diags
}

func (r *metricRouteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *metricRouteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_metric_route.MetricRouteModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildMetricRouteRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdateMetricRoute(ctx, http.MethodPost, "/v1/metrics/routes", requestBody, "create")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *metricRouteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_metric_route.MetricRouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/metrics/routes/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read metric route: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read metric route: %s", err))
		return
	}

	r.readJSON(ctx, httpResp.Body, resp)
}

func (r *metricRouteResource) readJSON(ctx context.Context, bodyReader io.Reader, resp *resource.ReadResponse) {
	raw, err := io.ReadAll(bodyReader)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp metricRouteAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	newState, diags := flattenMetricRoute(ctx, apiResp.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *metricRouteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_metric_route.MetricRouteModel
	var state resource_metric_route.MetricRouteModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildMetricRouteRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/metrics/routes/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateMetricRoute(ctx, http.MethodPut, path, requestBody, "update")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *metricRouteResource) buildMetricRouteRequestBody(ctx context.Context, plan resource_metric_route.MetricRouteModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	processors, expandDiags := expandMetricRouteProcessors(ctx, plan.Processors)
	diags.Append(expandDiags...)
	tags, tagDiags := expandTags(ctx, plan.Tags)
	diags.Append(tagDiags...)
	if diags.HasError() {
		return nil, diags
	}

	body := map[string]interface{}{
		"name":            plan.Name.ValueString(),
		"isEnabled":       plan.IsEnabled.ValueBool(),
		"metricNameRegex": plan.MetricNameRegex.ValueString(),
		"owner":           plan.Owner.ValueString(),
		"processors":      processors,
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		body["description"] = plan.Description.ValueString()
	}
	if tags != nil {
		body["tags"] = tags
	}

	return body, diags
}

func (r *metricRouteResource) createOrUpdateMetricRoute(ctx context.Context, method, path string, requestBody map[string]interface{}, operation string) (resource_metric_route.MetricRouteModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s metric route: %s", operation, err))
		return resource_metric_route.MetricRouteModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s metric route: %s", operation, err))
		return resource_metric_route.MetricRouteModel{}, diags
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return resource_metric_route.MetricRouteModel{}, diags
	}

	var apiResp metricRouteAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return resource_metric_route.MetricRouteModel{}, diags
	}

	newState, flattenDiags := flattenMetricRoute(ctx, apiResp.Data)
	diags.Append(flattenDiags...)
	if diags.HasError() {
		return resource_metric_route.MetricRouteModel{}, diags
	}

	return newState, diags
}

func (r *metricRouteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_metric_route.MetricRouteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/metrics/routes/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete metric route: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete metric route: %s", err))
			return
		}
	}
}

type metricRouteAPIResponse struct {
	Data metricRouteAPIData `json:"data"`
}

type metricRouteAPIData struct {
	ID              string                    `json:"id"`
	Name            string                    `json:"name"`
	Description     string                    `json:"description"`
	IsEnabled       bool                      `json:"isEnabled"`
	MetricNameRegex string                    `json:"metricNameRegex"`
	Owner           string                    `json:"owner"`
	Tags            []apiTag                  `json:"tags"`
	Processors      []metricRouteAPIProcessor `json:"processors"`
}

type metricRouteAPIProcessor struct {
	ID          string                 `json:"id,omitempty"`
	Description string                 `json:"description,omitempty"`
	Tags        []apiTag               `json:"tags,omitempty"`
	Type        string                 `json:"type"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

func expandMetricRouteProcessors(ctx context.Context, processors types.List) ([]metricRouteAPIProcessor, diag.Diagnostics) {
	var diags diag.Diagnostics
	if processors.IsNull() || processors.IsUnknown() {
		return nil, diags
	}

	var models []resource_metric_route.ProcessorModel
	diags.Append(processors.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]metricRouteAPIProcessor, 0, len(models))
	for _, p := range models {
		setCount := 0
		apiProc := metricRouteAPIProcessor{
			ID:          p.Id.ValueString(),
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
			apiProc.Params = expandMetricRouteMapperParams(p.Mapper)
		}
		if p.ParseAttribute != nil {
			setCount++
			apiProc.Type = "parse-attribute"
			params, pd := expandMetricRouteParseAttributeParams(ctx, p.ParseAttribute)
			diags.Append(pd...)
			if diags.HasError() {
				return nil, diags
			}
			apiProc.Params = params
		}
		if p.Creator != nil {
			setCount++
			apiProc.Type = "creator"
			params, pd := expandMetricRouteCreatorParams(p.Creator)
			diags.Append(pd...)
			if diags.HasError() {
				return nil, diags
			}
			apiProc.Params = params
		}

		if setCount != 1 {
			diags.AddError("Invalid processor", "Exactly one of mapper, parse_attribute, creator must be set.")
			return nil, diags
		}

		result = append(result, apiProc)
	}

	return result, diags
}

func expandMetricRouteMapperParams(m *resource_metric_route.MapperModel) map[string]interface{} {
	attrs := make([]map[string]interface{}, 0, len(m.MapAttributes))
	for _, a := range m.MapAttributes {
		item := map[string]interface{}{
			"originAttribute": a.OriginAttribute.ValueString(),
			"targetAttribute": a.TargetAttribute.ValueString(),
		}
		if !a.KeepOrigin.IsNull() && !a.KeepOrigin.IsUnknown() {
			item["keepOrigin"] = a.KeepOrigin.ValueBool()
		}
		if !a.OverrideTarget.IsNull() && !a.OverrideTarget.IsUnknown() {
			item["overrideTarget"] = a.OverrideTarget.ValueBool()
		}
		attrs = append(attrs, item)
	}
	return map[string]interface{}{
		"subtype":    "map-attributes",
		"attributes": attrs,
	}
}

func expandMetricRouteParseAttributeParams(ctx context.Context, p *resource_metric_route.ParseAttributeModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if p.Grok == nil {
		diags.AddError("Invalid parse_attribute", "The grok block must be set.")
		return nil, diags
	}

	rules, rd := expandStringList(ctx, p.Grok.Rules)
	diags.Append(rd...)
	samples, sd := expandStringList(ctx, p.Grok.Samples)
	diags.Append(sd...)
	params := map[string]interface{}{
		"subtype":       "grok",
		"attributeName": p.Grok.AttributeName.ValueString(),
		"rules":         rules,
	}
	if samples != nil {
		params["samples"] = samples
	}
	return params, diags
}

func expandMetricRouteCreatorParams(c *resource_metric_route.CreatorModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if c.FormatString != nil {
		params := map[string]interface{}{
			"subtype":         "format-string",
			"targetAttribute": c.FormatString.TargetAttribute.ValueString(),
			"formatString":    c.FormatString.FormatString.ValueString(),
		}
		if !c.FormatString.OverrideTarget.IsNull() && !c.FormatString.OverrideTarget.IsUnknown() {
			params["overrideTarget"] = c.FormatString.OverrideTarget.ValueBool()
		}
		if !c.FormatString.ReplaceMissingByEmpty.IsNull() && !c.FormatString.ReplaceMissingByEmpty.IsUnknown() {
			params["replaceMissingByEmpty"] = c.FormatString.ReplaceMissingByEmpty.ValueBool()
		}
		return params, diags
	}
	if c.Category != nil {
		clauses := make([]map[string]interface{}, 0, len(c.Category.Clauses))
		for _, cl := range c.Category.Clauses {
			clauses = append(clauses, map[string]interface{}{
				"query": cl.Query.ValueString(),
				"value": cl.Value.ValueString(),
			})
		}
		params := map[string]interface{}{
			"subtype":         "category",
			"targetAttribute": c.Category.TargetAttribute.ValueString(),
			"clauses":         clauses,
		}
		if !c.Category.DefaultValue.IsNull() && !c.Category.DefaultValue.IsUnknown() {
			params["defaultValue"] = c.Category.DefaultValue.ValueString()
		}
		return params, diags
	}
	diags.AddError("Invalid creator", "One creator block must be set.")
	return nil, diags
}

func flattenMetricRoute(ctx context.Context, data metricRouteAPIData) (resource_metric_route.MetricRouteModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tags, td := flattenTags(ctx, data.Tags)
	diags.Append(td...)

	procs, pd := flattenMetricRouteProcessors(ctx, data.Processors)
	diags.Append(pd...)

	tagsList := tags
	if tagsList.IsNull() {
		tagsList = types.ListNull(types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)})
	}

	state := resource_metric_route.MetricRouteModel{
		Id:              types.StringValue(data.ID),
		Name:            types.StringValue(data.Name),
		Description:     stringValueOrNull(data.Description),
		IsEnabled:       types.BoolValue(data.IsEnabled),
		MetricNameRegex: types.StringValue(data.MetricNameRegex),
		Owner:           types.StringValue(data.Owner),
		Tags:            tagsList,
		Processors:      procs,
	}

	return state, diags
}

func flattenMetricRouteProcessors(ctx context.Context, procs []metricRouteAPIProcessor) (types.List, diag.Diagnostics) {
	attrTypes := resource_metric_route.ProcessorAttrTypes(ctx)
	elemType := types.ObjectType{AttrTypes: attrTypes}
	if len(procs) == 0 {
		return types.ListValue(elemType, []attr.Value{})
	}

	values := make([]attr.Value, 0, len(procs))
	for _, p := range procs {
		obj := map[string]attr.Value{
			"id":              stringValueOrNull(p.ID),
			"description":     stringValueOrNull(p.Description),
			"tags":            types.ListNull(types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)}),
			"mapper":          types.ObjectNull(resource_metric_route.MapperAttrTypes()),
			"parse_attribute": types.ObjectNull(resource_metric_route.ParseAttributeAttrTypes()),
			"creator":         types.ObjectNull(resource_metric_route.CreatorAttrTypes()),
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
			obj["mapper"] = flattenMetricRouteMapper(p.Params)
		case "parse-attribute":
			obj["parse_attribute"] = flattenMetricRouteParseAttribute(ctx, p.Params)
		case "creator":
			obj["creator"] = flattenMetricRouteCreator(p.Params)
		}

		values = append(values, types.ObjectValueMust(attrTypes, obj))
	}

	return types.ListValue(elemType, values)
}

func flattenMetricRouteMapper(params map[string]interface{}) attr.Value {
	mapperAttrs := resource_metric_route.MapperAttrTypes()
	mapAttrs := types.ListNull(types.ObjectType{AttrTypes: resource_metric_route.MapAttributeAttrTypes()})
	if attrsRaw, ok := params["attributes"].([]interface{}); ok && len(attrsRaw) > 0 {
		vals := make([]attr.Value, 0, len(attrsRaw))
		for _, a := range attrsRaw {
			m, _ := a.(map[string]interface{})
			keepVal, keepOK := boolFromMap(m, "keepOrigin")
			overrideVal, overrideOK := boolFromMap(m, "overrideTarget")
			vals = append(vals, types.ObjectValueMust(resource_metric_route.MapAttributeAttrTypes(), map[string]attr.Value{
				"origin_attribute": types.StringValue(fmt.Sprintf("%v", m["originAttribute"])),
				"target_attribute": types.StringValue(fmt.Sprintf("%v", m["targetAttribute"])),
				"keep_origin":      optionalBoolValue(keepOK, keepVal),
				"override_target":  optionalBoolValue(overrideOK, overrideVal),
			}))
		}
		mapAttrs, _ = types.ListValue(types.ObjectType{AttrTypes: resource_metric_route.MapAttributeAttrTypes()}, vals)
	}

	return types.ObjectValueMust(mapperAttrs, map[string]attr.Value{
		"map_attributes": mapAttrs,
	})
}

func flattenMetricRouteParseAttribute(ctx context.Context, params map[string]interface{}) attr.Value {
	attrTypes := resource_metric_route.ParseAttributeAttrTypes()

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
		"grok": types.ObjectValueMust(resource_metric_route.ParseGrokAttrTypes(), map[string]attr.Value{
			"attribute_name": types.StringValue(fmt.Sprintf("%v", params["attributeName"])),
			"rules":          rulesVal,
			"samples":        samplesVal,
		}),
	})
}

func flattenMetricRouteCreator(params map[string]interface{}) attr.Value {
	attrTypes := resource_metric_route.CreatorAttrTypes()
	subtype, _ := params["subtype"].(string)
	formatNull := types.ObjectNull(resource_metric_route.CreatorFormatStringAttrTypes())
	categoryNull := types.ObjectNull(resource_metric_route.CreatorCategoryAttrTypes())

	if subtype == "format-string" {
		overrideVal, okO := boolFromMap(params, "overrideTarget")
		replaceVal, okR := boolFromMap(params, "replaceMissingByEmpty")
		return types.ObjectValueMust(attrTypes, map[string]attr.Value{
			"format_string": types.ObjectValueMust(resource_metric_route.CreatorFormatStringAttrTypes(), map[string]attr.Value{
				"target_attribute":         types.StringValue(fmt.Sprintf("%v", params["targetAttribute"])),
				"format_string":            types.StringValue(fmt.Sprintf("%v", params["formatString"])),
				"override_target":          optionalBoolValue(okO, overrideVal),
				"replace_missing_by_empty": optionalBoolValue(okR, replaceVal),
			}),
			"category": categoryNull,
		})
	}
	if subtype == "category" {
		clauseType := types.ObjectType{AttrTypes: resource_metric_route.CreatorCategoryClauseAttrTypes()}
		clausesVal := types.ListNull(clauseType)
		if clausesRaw, ok := params["clauses"].([]interface{}); ok {
			vals := make([]attr.Value, 0, len(clausesRaw))
			for _, cl := range clausesRaw {
				m, _ := cl.(map[string]interface{})
				vals = append(vals, types.ObjectValueMust(resource_metric_route.CreatorCategoryClauseAttrTypes(), map[string]attr.Value{
					"query": types.StringValue(fmt.Sprintf("%v", m["query"])),
					"value": types.StringValue(fmt.Sprintf("%v", m["value"])),
				}))
			}
			clausesVal, _ = types.ListValue(clauseType, vals)
		}
		defaultVal := types.StringNull()
		if dv, ok := params["defaultValue"]; ok && dv != nil {
			defaultVal = types.StringValue(fmt.Sprintf("%v", dv))
		}
		return types.ObjectValueMust(attrTypes, map[string]attr.Value{
			"category": types.ObjectValueMust(resource_metric_route.CreatorCategoryAttrTypes(), map[string]attr.Value{
				"target_attribute": types.StringValue(fmt.Sprintf("%v", params["targetAttribute"])),
				"clauses":          clausesVal,
				"default_value":    defaultVal,
			}),
			"format_string": formatNull,
		})
	}

	return types.ObjectValueMust(attrTypes, map[string]attr.Value{
		"format_string": formatNull,
		"category":      categoryNull,
	})
}
