package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/aggregate"
	"terraform-provider-tsuga/internal/groupby"
	"terraform-provider-tsuga/internal/normalizer"
	"terraform-provider-tsuga/internal/resource_dashboard"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*dashboardResource)(nil)
var _ resource.ResourceWithConfigure = (*dashboardResource)(nil)
var _ resource.ResourceWithImportState = (*dashboardResource)(nil)
var _ resource.ResourceWithValidateConfig = (*dashboardResource)(nil)

func NewDashboardResource() resource.Resource {
	return &dashboardResource{}
}

type dashboardResource struct {
	client *TsugaClient
}

func (r *dashboardResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dashboardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (r *dashboardResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_dashboard.DashboardResourceSchema(ctx)
}

func (r *dashboardResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_dashboard.DashboardModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate graphs: each graph's visualization must have exactly one visualization type
	if !config.Graphs.IsNull() && !config.Graphs.IsUnknown() {
		var graphs []resource_dashboard.GraphModel
		resp.Diagnostics.Append(config.Graphs.ElementsAs(ctx, &graphs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		for i, graph := range graphs {
			diags := r.validateVisualization(ctx, graph.Visualization, fmt.Sprintf("graphs[%d].visualization", i))
			resp.Diagnostics.Append(diags...)
		}
	}
}

func (r *dashboardResource) validateVisualization(ctx context.Context, vis resource_dashboard.VisualizationModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	setCount := 0
	if vis.Timeseries != nil {
		setCount++
	}
	if vis.TopList != nil {
		setCount++
	}
	if vis.Pie != nil {
		setCount++
	}
	if vis.QueryValue != nil {
		setCount++
	}
	if vis.Bar != nil {
		setCount++
	}
	if vis.List != nil {
		setCount++
	}
	if vis.Note != nil {
		setCount++
	}

	if setCount != 1 {
		diags.AddError(
			"Invalid visualization configuration",
			fmt.Sprintf("%s: exactly one of timeseries, top_list, pie, query_value, bar, list, or note must be set.", pathPrefix),
		)
	}

	// Validate nested aggregates in queries for series visualizations
	if vis.Timeseries != nil {
		diags.Append(r.validateSeriesVisualization(ctx, vis.Timeseries, fmt.Sprintf("%s.timeseries", pathPrefix))...)
	}
	if vis.TopList != nil {
		diags.Append(r.validateSeriesVisualization(ctx, vis.TopList, fmt.Sprintf("%s.top_list", pathPrefix))...)
	}
	if vis.Pie != nil {
		diags.Append(r.validateSeriesVisualization(ctx, vis.Pie, fmt.Sprintf("%s.pie", pathPrefix))...)
	}
	if vis.QueryValue != nil {
		diags.Append(r.validateSeriesVisualization(ctx, &vis.QueryValue.SeriesVisualizationModel, fmt.Sprintf("%s.query_value", pathPrefix))...)
	}
	if vis.Bar != nil {
		diags.Append(r.validateSeriesVisualization(ctx, &vis.Bar.SeriesVisualizationModel, fmt.Sprintf("%s.bar", pathPrefix))...)
	}

	return diags
}

func (r *dashboardResource) validateSeriesVisualization(ctx context.Context, sv *resource_dashboard.SeriesVisualizationModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	// Validate queries if present
	if !sv.Queries.IsNull() && !sv.Queries.IsUnknown() {
		var queries []resource_dashboard.QueryModel
		diags.Append(sv.Queries.ElementsAs(ctx, &queries, false)...)
		if diags.HasError() {
			return diags
		}

		for i, query := range queries {
			aggDiags := r.validateAggregate(query.Aggregate, fmt.Sprintf("%s.queries[%d].aggregate", pathPrefix, i))
			diags.Append(aggDiags...)
		}
	}

	return diags
}

func (r *dashboardResource) validateAggregate(agg resource_dashboard.AggregateModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	setCount := 0
	if agg.Count != nil {
		setCount++
	}
	if agg.Sum != nil && !agg.Sum.Field.IsNull() && !agg.Sum.Field.IsUnknown() {
		setCount++
	}
	if agg.Average != nil && !agg.Average.Field.IsNull() && !agg.Average.Field.IsUnknown() {
		setCount++
	}
	if agg.Min != nil && !agg.Min.Field.IsNull() && !agg.Min.Field.IsUnknown() {
		setCount++
	}
	if agg.Max != nil && !agg.Max.Field.IsNull() && !agg.Max.Field.IsUnknown() {
		setCount++
	}
	if agg.Uniq != nil && !agg.Uniq.Field.IsNull() && !agg.Uniq.Field.IsUnknown() {
		setCount++
	}
	if agg.Percentile != nil && !agg.Percentile.Field.IsNull() && !agg.Percentile.Field.IsUnknown() {
		setCount++
	}

	if setCount != 1 {
		diags.AddError(
			"Invalid aggregate configuration",
			fmt.Sprintf("%s: exactly one of count, sum, average, min, max, unique_count, or percentile must be set.", pathPrefix),
		)
	}

	return diags
}

func (r *dashboardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *dashboardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_dashboard.DashboardModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildDashboardRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdateDashboard(ctx, http.MethodPost, "/v1/dashboards", requestBody, "create")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *dashboardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_dashboard.DashboardModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/dashboards/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read dashboard: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read dashboard: %s", err))
		return
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp dashboardAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	newState, diags := flattenDashboard(ctx, apiResp.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *dashboardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_dashboard.DashboardModel
	var state resource_dashboard.DashboardModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildDashboardRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/dashboards/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateDashboard(ctx, http.MethodPut, path, requestBody, "update")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *dashboardResource) buildDashboardRequestBody(ctx context.Context, plan resource_dashboard.DashboardModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	graphs, expandDiags := expandDashboardsGraphs(ctx, plan.Graphs)
	diags.Append(expandDiags...)
	filters, expandDiags := expandStringList(ctx, plan.Filters)
	diags.Append(expandDiags...)
	if diags.HasError() {
		return nil, diags
	}

	body := map[string]interface{}{
		"name":   plan.Name.ValueString(),
		"owner":  plan.Owner.ValueString(),
		"graphs": graphs,
	}
	if filters != nil {
		body["filters"] = filters
	}

	if tags, tagDiags := expandTags(ctx, plan.Tags); tagDiags.HasError() {
		diags.Append(tagDiags...)
		return nil, diags
	} else if tags != nil {
		body["tags"] = tags
	}

	return body, diags
}

func (r *dashboardResource) createOrUpdateDashboard(ctx context.Context, method, path string, requestBody map[string]interface{}, operation string) (resource_dashboard.DashboardModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s dashboard: %s", operation, err))
		return resource_dashboard.DashboardModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s dashboard: %s", operation, err))
		return resource_dashboard.DashboardModel{}, diags
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return resource_dashboard.DashboardModel{}, diags
	}

	var apiResp dashboardAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return resource_dashboard.DashboardModel{}, diags
	}

	newState, flattenDiags := flattenDashboard(ctx, apiResp.Data)
	diags.Append(flattenDiags...)
	if diags.HasError() {
		return resource_dashboard.DashboardModel{}, diags
	}

	return newState, diags
}

func (r *dashboardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_dashboard.DashboardModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/dashboards/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete dashboard: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete dashboard: %s", err))
			return
		}
	}
}

type dashboardAPIResponse struct {
	Data dashboardAPIData `json:"data"`
}

type dashboardAPIData struct {
	ID      string              `json:"id"`
	Name    string              `json:"name"`
	Owner   string              `json:"owner"`
	Filters []string            `json:"filters"`
	Tags    []apiTag            `json:"tags"`
	Graphs  []dashboardAPIGraph `json:"graphs"`
}

type dashboardAPIGraph struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Layout        *dashboardGraphLayout  `json:"layout,omitempty"`
	Visualization dashboardVisualization `json:"visualization"`
}

type dashboardGraphLayout struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type dashboardVisualization struct {
	Type           string                `json:"type"`
	Source         string                `json:"source,omitempty"`
	Queries        []dashboardQuery      `json:"queries,omitempty"`
	Formula        string                `json:"formula,omitempty"`
	VisibleSeries  []bool                `json:"visibleSeries,omitempty"`
	GroupBy        []dashboardGroupBy    `json:"groupBy,omitempty"`
	Normalizer     *dashboardNormalizer  `json:"normalizer,omitempty"`
	BackgroundMode string                `json:"backgroundMode,omitempty"`
	Conditions     []dashboardCondition  `json:"conditions,omitempty"`
	TimeBucket     *dashboardTimeBucket  `json:"timeBucket,omitempty"`
	Query          string                `json:"query,omitempty"`
	ListColumns    []dashboardListColumn `json:"listColumns,omitempty"`
	Note           string                `json:"note,omitempty"`
	NoteAlign      string                `json:"noteAlign,omitempty"`
	NoteJustify    string                `json:"noteJustifyContent,omitempty"`
	NoteColor      string                `json:"noteColor,omitempty"`
}

type dashboardQuery struct {
	Aggregate dashboardAggregate  `json:"aggregate"`
	Filter    string              `json:"filter,omitempty"`
	Functions []dashboardFunction `json:"functions,omitempty"`
}

type dashboardAggregate struct {
	Type       string  `json:"type"`
	Field      string  `json:"field,omitempty"`
	Percentile float64 `json:"percentile,omitempty"`
}

type dashboardFunction struct {
	Type   string `json:"type"`
	Window string `json:"window,omitempty"`
}

type dashboardGroupBy struct {
	Fields []string `json:"fields"`
	Limit  int64    `json:"limit"`
}

type dashboardNormalizer struct {
	Type string `json:"type"`
	Unit string `json:"unit,omitempty"`
}

type dashboardCondition struct {
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
	Color    string  `json:"color"`
}

type dashboardTimeBucket struct {
	Time   float64 `json:"time"`
	Metric string  `json:"metric"`
}

type dashboardListColumn struct {
	Attribute  string               `json:"attribute"`
	Normalizer *dashboardNormalizer `json:"normalizer,omitempty"`
}

func expandDashboardsGraphs(ctx context.Context, graphs types.List) ([]dashboardAPIGraph, diag.Diagnostics) {
	var diags diag.Diagnostics
	if graphs.IsNull() || graphs.IsUnknown() {
		return nil, diags
	}

	var graphModels []resource_dashboard.GraphModel
	diags.Append(graphs.ElementsAs(ctx, &graphModels, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]dashboardAPIGraph, 0, len(graphModels))
	for _, g := range graphModels {
		apiGraph := dashboardAPIGraph{
			ID:   g.Id.ValueString(),
			Name: g.Name.ValueString(),
		}
		if g.Layout != nil {
			apiGraph.Layout = &dashboardGraphLayout{
				X: g.Layout.X.ValueFloat64(),
				Y: g.Layout.Y.ValueFloat64(),
				W: g.Layout.W.ValueFloat64(),
				H: g.Layout.H.ValueFloat64(),
			}
		}

		vis, visDiags := expandVisualization(ctx, g.Visualization)
		diags.Append(visDiags...)
		if diags.HasError() {
			return nil, diags
		}
		apiGraph.Visualization = vis

		result = append(result, apiGraph)
	}

	return result, diags
}

func expandVisualization(ctx context.Context, v resource_dashboard.VisualizationModel) (dashboardVisualization, diag.Diagnostics) {
	var diags diag.Diagnostics
	setCount := 0
	var vis dashboardVisualization

	buildSeries := func(sv *resource_dashboard.SeriesVisualizationModel, visType string) (dashboardVisualization, diag.Diagnostics) {
		var d diag.Diagnostics
		queries, qDiags := expandQueries(ctx, sv.Queries)
		d.Append(qDiags...)
		groupBy, gDiags := expandGroupBy(ctx, sv.GroupBy)
		d.Append(gDiags...)
		var normalizer *dashboardNormalizer
		if sv.Normalizer != nil {
			normalizer = &dashboardNormalizer{
				Type: sv.Normalizer.Type.ValueString(),
			}
			if !sv.Normalizer.Unit.IsNull() && !sv.Normalizer.Unit.IsUnknown() {
				normalizer.Unit = sv.Normalizer.Unit.ValueString()
			}
		}

		var visible []bool
		if !sv.VisibleSeries.IsNull() && !sv.VisibleSeries.IsUnknown() {
			var boolDiags diag.Diagnostics
			visible, boolDiags = expandBoolList(ctx, sv.VisibleSeries)
			d.Append(boolDiags...)
		}

		result := dashboardVisualization{
			Type:          visType,
			Source:        sv.Source.ValueString(),
			Queries:       queries,
			Formula:       sv.Formula.ValueString(),
			VisibleSeries: visible,
			GroupBy:       groupBy,
			Normalizer:    normalizer,
		}
		return result, d
	}

	if v.Timeseries != nil {
		setCount++
		vz, d := buildSeries(v.Timeseries, "timeseries")
		diags.Append(d...)
		vis = vz
	}
	if v.TopList != nil {
		setCount++
		vz, d := buildSeries(v.TopList, "top-list")
		diags.Append(d...)
		vis = vz
	}
	if v.Pie != nil {
		setCount++
		vz, d := buildSeries(v.Pie, "pie")
		diags.Append(d...)
		vis = vz
	}
	if v.QueryValue != nil {
		setCount++
		vz, d := buildSeries(&v.QueryValue.SeriesVisualizationModel, "query-value")
		diags.Append(d...)
		if !v.QueryValue.BackgroundMode.IsNull() && !v.QueryValue.BackgroundMode.IsUnknown() {
			vz.BackgroundMode = v.QueryValue.BackgroundMode.ValueString()
		}
		if !v.QueryValue.Conditions.IsNull() && !v.QueryValue.Conditions.IsUnknown() {
			conds, cDiags := expandConditions(ctx, v.QueryValue.Conditions)
			diags.Append(cDiags...)
			vz.Conditions = conds
		}
		vis = vz
	}
	if v.Bar != nil {
		setCount++
		vz, d := buildSeries(&v.Bar.SeriesVisualizationModel, "bar")
		diags.Append(d...)
		if v.Bar.TimeBucket != nil {
			vz.TimeBucket = &dashboardTimeBucket{
				Time:   v.Bar.TimeBucket.Time.ValueFloat64(),
				Metric: v.Bar.TimeBucket.Metric.ValueString(),
			}
		}
		vis = vz
	}
	if v.List != nil {
		setCount++
		vz := dashboardVisualization{
			Type:   "list",
			Source: v.List.Source.ValueString(),
			Query:  v.List.Query.ValueString(),
		}
		if !v.List.ListColumns.IsNull() && !v.List.ListColumns.IsUnknown() {
			cols, cDiags := expandListColumns(ctx, v.List.ListColumns)
			diags.Append(cDiags...)
			vz.ListColumns = cols
		}
		vis = vz
	}
	if v.Note != nil {
		setCount++
		vz := dashboardVisualization{
			Type:        "note",
			Note:        v.Note.Note.ValueString(),
			NoteAlign:   stringValue(v.Note.NoteAlign),
			NoteJustify: stringValue(v.Note.NoteJustifyContent),
			NoteColor:   stringValue(v.Note.NoteColor),
		}
		vis = vz
	}

	if setCount != 1 {
		diags.AddError("Invalid visualization", "Exactly one visualization block must be set.")
		return dashboardVisualization{}, diags
	}

	return vis, diags
}

func flattenDashboard(ctx context.Context, data dashboardAPIData) (resource_dashboard.DashboardModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	filters, filterDiags := types.ListValueFrom(ctx, types.StringType, data.Filters)
	diags.Append(filterDiags...)
	tags, tagDiags := flattenTags(ctx, data.Tags)
	diags.Append(tagDiags...)
	graphs, graphDiags := flattenDashboardGraphs(ctx, data.Graphs)
	diags.Append(graphDiags...)

	state := resource_dashboard.DashboardModel{
		Id:      types.StringValue(data.ID),
		Name:    types.StringValue(data.Name),
		Owner:   types.StringValue(data.Owner),
		Filters: filters,
		Tags:    tags,
		Graphs:  graphs,
	}

	return state, diags
}

func flattenDashboardGraphs(ctx context.Context, graphs []dashboardAPIGraph) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.GraphAttrTypes()}
	if len(graphs) == 0 {
		return types.ListNull(elemType), nil
	}

	layoutType := types.ObjectType{AttrTypes: resource_dashboard.GraphLayoutAttrTypes()}

	values := make([]attr.Value, 0, len(graphs))
	for _, g := range graphs {
		layoutVal := types.ObjectNull(layoutType.AttrTypes)
		if g.Layout != nil {
			layoutVal = types.ObjectValueMust(layoutType.AttrTypes, map[string]attr.Value{
				"x": types.Float64Value(g.Layout.X),
				"y": types.Float64Value(g.Layout.Y),
				"w": types.Float64Value(g.Layout.W),
				"h": types.Float64Value(g.Layout.H),
			})
		}

		visVal, visDiags := flattenVisualization(ctx, g.Visualization)
		if visDiags.HasError() {
			return types.ListNull(elemType), visDiags
		}

		values = append(values, types.ObjectValueMust(resource_dashboard.GraphAttrTypes(), map[string]attr.Value{
			"id":            types.StringValue(g.ID),
			"name":          stringValueOrNull(g.Name),
			"layout":        layoutVal,
			"visualization": visVal,
		}))
	}

	return types.ListValue(elemType, values)
}

func flattenVisualization(ctx context.Context, vis dashboardVisualization) (attr.Value, diag.Diagnostics) {
	switch vis.Type {
	case "note":
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), map[string]attr.Value{
			"note": types.ObjectValueMust(resource_dashboard.NoteVisualizationAttrTypes(), map[string]attr.Value{
				"type":                 types.StringValue("note"),
				"note":                 types.StringValue(vis.Note),
				"note_align":           stringValueOrNull(vis.NoteAlign),
				"note_justify_content": stringValueOrNull(vis.NoteJustify),
				"note_color":           stringValueOrNull(vis.NoteColor),
			}),
			"timeseries":  types.ObjectNull(resource_dashboard.SeriesVisualizationAttrTypes()),
			"top_list":    types.ObjectNull(resource_dashboard.SeriesVisualizationAttrTypes()),
			"pie":         types.ObjectNull(resource_dashboard.SeriesVisualizationAttrTypes()),
			"query_value": types.ObjectNull(resource_dashboard.QueryValueVisualizationAttrTypes()),
			"bar":         types.ObjectNull(resource_dashboard.BarVisualizationAttrTypes()),
			"list":        types.ObjectNull(resource_dashboard.ListVisualizationAttrTypes()),
		}), nil
	case "list":
		listCols, diags := flattenListColumns(vis.ListColumns)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), map[string]attr.Value{
			"list": types.ObjectValueMust(resource_dashboard.ListVisualizationAttrTypes(), map[string]attr.Value{
				"type":         types.StringValue("list"),
				"source":       types.StringValue(vis.Source),
				"query":        types.StringValue(vis.Query),
				"list_columns": listCols,
			}),
			"timeseries":  types.ObjectNull(resource_dashboard.SeriesVisualizationAttrTypes()),
			"top_list":    types.ObjectNull(resource_dashboard.SeriesVisualizationAttrTypes()),
			"pie":         types.ObjectNull(resource_dashboard.SeriesVisualizationAttrTypes()),
			"query_value": types.ObjectNull(resource_dashboard.QueryValueVisualizationAttrTypes()),
			"bar":         types.ObjectNull(resource_dashboard.BarVisualizationAttrTypes()),
			"note":        types.ObjectNull(resource_dashboard.NoteVisualizationAttrTypes()),
		}), nil
	case "bar", "query-value", "timeseries", "top-list", "pie":
		seriesVal, diags := flattenSeriesVisualization(ctx, vis)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		obj := map[string]attr.Value{
			"timeseries":  types.ObjectNull(resource_dashboard.SeriesVisualizationAttrTypes()),
			"top_list":    types.ObjectNull(resource_dashboard.SeriesVisualizationAttrTypes()),
			"pie":         types.ObjectNull(resource_dashboard.SeriesVisualizationAttrTypes()),
			"query_value": types.ObjectNull(resource_dashboard.QueryValueVisualizationAttrTypes()),
			"bar":         types.ObjectNull(resource_dashboard.BarVisualizationAttrTypes()),
			"list":        types.ObjectNull(resource_dashboard.ListVisualizationAttrTypes()),
			"note":        types.ObjectNull(resource_dashboard.NoteVisualizationAttrTypes()),
		}
		switch vis.Type {
		case "timeseries":
			obj["timeseries"] = seriesVal
		case "top-list":
			obj["top_list"] = seriesVal
		case "pie":
			obj["pie"] = seriesVal
		case "query-value":
			obj["query_value"] = seriesVal
		case "bar":
			obj["bar"] = seriesVal
		}
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), nil
	default:
		var diags diag.Diagnostics
		diags.AddError("Unsupported visualization type", vis.Type)
		return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
	}
}

func flattenSeriesVisualization(ctx context.Context, vis dashboardVisualization) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	queries, qDiags := flattenQueries(vis.Queries)
	diags.Append(qDiags...)
	groupBy, gDiags := flattenGroupBy(ctx, vis.GroupBy)
	diags.Append(gDiags...)

	normalizerVal := types.ObjectNull(normalizer.AttrTypes())
	if vis.Normalizer != nil {
		normalizerVal = types.ObjectValueMust(normalizer.AttrTypes(), map[string]attr.Value{
			"type": types.StringValue(vis.Normalizer.Type),
			"unit": stringValueOrNull(vis.Normalizer.Unit),
		})
	}

	visibleSeries := types.ListNull(types.BoolType)
	if len(vis.VisibleSeries) > 0 {
		v, dv := types.ListValueFrom(ctx, types.BoolType, vis.VisibleSeries)
		diags.Append(dv...)
		visibleSeries = v
	}

	obj := map[string]attr.Value{
		"type":           types.StringValue(vis.Type),
		"source":         types.StringValue(vis.Source),
		"queries":        queries,
		"formula":        stringValueOrNull(vis.Formula),
		"visible_series": visibleSeries,
		"group_by":       groupBy,
		"normalizer":     normalizerVal,
	}

	if vis.Type == "query-value" {
		condVal, cDiags := flattenConditions(vis.Conditions)
		diags.Append(cDiags...)
		obj["background_mode"] = stringValueOrNull(vis.BackgroundMode)
		obj["conditions"] = condVal
		return types.ObjectValueMust(resource_dashboard.QueryValueVisualizationAttrTypes(), obj), diags
	}

	if vis.Type == "bar" {
		tbVal := types.ObjectNull(resource_dashboard.TimeBucketAttrTypes())
		if vis.TimeBucket != nil {
			tbVal = types.ObjectValueMust(resource_dashboard.TimeBucketAttrTypes(), map[string]attr.Value{
				"time":   types.Float64Value(vis.TimeBucket.Time),
				"metric": types.StringValue(vis.TimeBucket.Metric),
			})
		}
		obj["time_bucket"] = tbVal
		return types.ObjectValueMust(resource_dashboard.BarVisualizationAttrTypes(), obj), diags
	}

	return types.ObjectValueMust(resource_dashboard.SeriesVisualizationAttrTypes(), obj), diags
}

func flattenQueries(queries []dashboardQuery) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.QueryAttrTypes()}
	if len(queries) == 0 {
		return types.ListNull(elemType), nil
	}

	values := make([]attr.Value, 0, len(queries))
	for _, q := range queries {
		aggVal, diags := flattenAggregate(q.Aggregate)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}

		funcVal, fDiags := flattenFunctions(q.Functions)
		if fDiags.HasError() {
			return types.ListNull(elemType), fDiags
		}

		values = append(values, types.ObjectValueMust(resource_dashboard.QueryAttrTypes(), map[string]attr.Value{
			"aggregate": aggVal,
			"filter":    stringValueOrNull(q.Filter),
			"functions": funcVal,
		}))
	}

	return types.ListValue(elemType, values)
}

func flattenAggregate(agg dashboardAggregate) (attr.Value, diag.Diagnostics) {
	countNull := types.ObjectNull(aggregate.CountAttrTypes())
	fieldNull := types.ObjectNull(aggregate.FieldAttrTypes())
	percNull := types.ObjectNull(aggregate.PercentileAttrTypes())

	vals := map[string]attr.Value{
		"count":        countNull,
		"sum":          fieldNull,
		"average":      fieldNull,
		"min":          fieldNull,
		"max":          fieldNull,
		"unique_count": fieldNull,
		"percentile":   percNull,
	}

	switch agg.Type {
	case "count":
		vals["count"] = types.ObjectValueMust(aggregate.CountAttrTypes(), map[string]attr.Value{})
	case "sum", "average", "min", "max", "unique-count":
		key := map[string]string{
			"sum":          "sum",
			"average":      "average",
			"min":          "min",
			"max":          "max",
			"unique-count": "unique_count",
		}[agg.Type]
		vals[key] = types.ObjectValueMust(aggregate.FieldAttrTypes(), map[string]attr.Value{
			"field": types.StringValue(agg.Field),
		})
	case "percentile":
		vals["percentile"] = types.ObjectValueMust(aggregate.PercentileAttrTypes(), map[string]attr.Value{
			"field":      types.StringValue(agg.Field),
			"percentile": types.Float64Value(agg.Percentile),
		})
	}

	return types.ObjectValueMust(aggregate.AttrTypes(), vals), nil
}

func flattenFunctions(funcs []dashboardFunction) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.FunctionAttrTypes()}
	if len(funcs) == 0 {
		return types.ListNull(elemType), nil
	}

	values := make([]attr.Value, 0, len(funcs))
	for _, f := range funcs {
		values = append(values, types.ObjectValueMust(resource_dashboard.FunctionAttrTypes(), map[string]attr.Value{
			"type":   types.StringValue(f.Type),
			"window": stringValueOrNull(f.Window),
		}))
	}
	return types.ListValue(elemType, values)
}

func flattenGroupBy(ctx context.Context, groupBy []dashboardGroupBy) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: groupby.AttrTypes()}
	if len(groupBy) == 0 {
		return types.ListNull(elemType), nil
	}

	values := make([]attr.Value, 0, len(groupBy))
	for _, gb := range groupBy {
		fields, diags := types.ListValueFrom(ctx, types.StringType, gb.Fields)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}
		values = append(values, types.ObjectValueMust(groupby.AttrTypes(), map[string]attr.Value{
			"fields": fields,
			"limit":  types.Int64Value(gb.Limit),
		}))
	}
	return types.ListValue(elemType, values)
}

func flattenConditions(conds []dashboardCondition) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.ConditionAttrTypes()}
	if len(conds) == 0 {
		return types.ListNull(elemType), nil
	}
	values := make([]attr.Value, 0, len(conds))
	for _, c := range conds {
		values = append(values, types.ObjectValueMust(resource_dashboard.ConditionAttrTypes(), map[string]attr.Value{
			"operator": types.StringValue(c.Operator),
			"value":    types.Float64Value(c.Value),
			"color":    types.StringValue(c.Color),
		}))
	}
	return types.ListValue(elemType, values)
}

func flattenListColumns(cols []dashboardListColumn) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.ListColumnAttrTypes()}
	if len(cols) == 0 {
		return types.ListNull(elemType), nil
	}
	values := make([]attr.Value, 0, len(cols))
	for _, c := range cols {
		normVal := types.ObjectNull(normalizer.AttrTypes())
		if c.Normalizer != nil {
			normVal = types.ObjectValueMust(normalizer.AttrTypes(), map[string]attr.Value{
				"type": types.StringValue(c.Normalizer.Type),
				"unit": stringValueOrNull(c.Normalizer.Unit),
			})
		}
		values = append(values, types.ObjectValueMust(resource_dashboard.ListColumnAttrTypes(), map[string]attr.Value{
			"attribute":  types.StringValue(c.Attribute),
			"normalizer": normVal,
		}))
	}
	return types.ListValue(elemType, values)
}

func expandQueries(ctx context.Context, queries types.List) ([]dashboardQuery, diag.Diagnostics) {
	var diags diag.Diagnostics
	if queries.IsNull() || queries.IsUnknown() {
		return nil, diags
	}
	var qModels []resource_dashboard.QueryModel
	diags.Append(queries.ElementsAs(ctx, &qModels, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]dashboardQuery, 0, len(qModels))
	for _, q := range qModels {
		agg, aggDiags := expandAggregate(q.Aggregate)
		diags.Append(aggDiags...)
		if diags.HasError() {
			return nil, diags
		}
		fns, fnDiags := expandFunctions(ctx, q.Functions)
		diags.Append(fnDiags...)
		if diags.HasError() {
			return nil, diags
		}
		result = append(result, dashboardQuery{
			Aggregate: agg,
			Filter:    q.Filter.ValueString(),
			Functions: fns,
		})
	}
	return result, diags
}

func expandAggregate(agg resource_dashboard.AggregateModel) (dashboardAggregate, diag.Diagnostics) {
	var diags diag.Diagnostics
	setCount := 0
	var res dashboardAggregate

	checkField := func(m *aggregate.FieldModel, typ string) {
		if m != nil && !m.Field.IsNull() && !m.Field.IsUnknown() {
			setCount++
			res = dashboardAggregate{
				Type:  typ,
				Field: m.Field.ValueString(),
			}
		}
	}

	if agg.Count != nil {
		setCount++
		res = dashboardAggregate{Type: "count"}
	}
	checkField(agg.Sum, "sum")
	checkField(agg.Average, "average")
	checkField(agg.Min, "min")
	checkField(agg.Max, "max")
	checkField(agg.Uniq, "unique-count")

	if agg.Percentile != nil && !agg.Percentile.Field.IsNull() && !agg.Percentile.Field.IsUnknown() {
		setCount++
		res = dashboardAggregate{
			Type:       "percentile",
			Field:      agg.Percentile.Field.ValueString(),
			Percentile: agg.Percentile.Percentile.ValueFloat64(),
		}
	}

	if setCount != 1 {
		diags.AddError("Invalid aggregate", "Exactly one aggregate block must be set for each query.")
	}
	return res, diags
}

func expandFunctions(ctx context.Context, funcs types.List) ([]dashboardFunction, diag.Diagnostics) {
	var diags diag.Diagnostics
	if funcs.IsNull() || funcs.IsUnknown() {
		return nil, diags
	}
	var fModels []resource_dashboard.FunctionModel
	diags.Append(funcs.ElementsAs(ctx, &fModels, false)...)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardFunction, 0, len(fModels))
	for _, f := range fModels {
		fn := dashboardFunction{
			Type: f.Type.ValueString(),
		}
		if !f.Window.IsNull() && !f.Window.IsUnknown() {
			fn.Window = f.Window.ValueString()
		}
		result = append(result, fn)
	}
	return result, diags
}

func expandGroupBy(ctx context.Context, groupBy types.List) ([]dashboardGroupBy, diag.Diagnostics) {
	var diags diag.Diagnostics
	if groupBy.IsNull() || groupBy.IsUnknown() {
		return nil, diags
	}
	var groupModels []groupby.Model
	diags.Append(groupBy.ElementsAs(ctx, &groupModels, false)...)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardGroupBy, 0, len(groupModels))
	for _, g := range groupModels {
		fields, fDiags := expandStringList(ctx, g.Fields)
		diags.Append(fDiags...)
		if diags.HasError() {
			return nil, diags
		}
		result = append(result, dashboardGroupBy{
			Fields: fields,
			Limit:  g.Limit.ValueInt64(),
		})
	}
	return result, diags
}

func expandConditions(ctx context.Context, conds types.List) ([]dashboardCondition, diag.Diagnostics) {
	var diags diag.Diagnostics
	if conds.IsNull() || conds.IsUnknown() {
		return nil, diags
	}
	var condModels []resource_dashboard.ConditionModel
	diags.Append(conds.ElementsAs(ctx, &condModels, false)...)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardCondition, 0, len(condModels))
	for _, c := range condModels {
		result = append(result, dashboardCondition{
			Operator: c.Operator.ValueString(),
			Value:    c.Value.ValueFloat64(),
			Color:    c.Color.ValueString(),
		})
	}
	return result, diags
}

func expandListColumns(ctx context.Context, cols types.List) ([]dashboardListColumn, diag.Diagnostics) {
	var diags diag.Diagnostics
	if cols.IsNull() || cols.IsUnknown() {
		return nil, diags
	}
	var colModels []resource_dashboard.ListColumnModel
	diags.Append(cols.ElementsAs(ctx, &colModels, false)...)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardListColumn, 0, len(colModels))
	for _, c := range colModels {
		var norm *dashboardNormalizer
		if c.Normalizer != nil {
			norm = &dashboardNormalizer{
				Type: c.Normalizer.Type.ValueString(),
			}
			if !c.Normalizer.Unit.IsNull() && !c.Normalizer.Unit.IsUnknown() {
				norm.Unit = c.Normalizer.Unit.ValueString()
			}
		}
		result = append(result, dashboardListColumn{
			Attribute:  c.Attribute.ValueString(),
			Normalizer: norm,
		})
	}
	return result, diags
}
