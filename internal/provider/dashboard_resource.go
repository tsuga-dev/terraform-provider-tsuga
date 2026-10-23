package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

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

		seenIds := make(map[string]int, len(graphs))
		for i, graph := range graphs {
			if id := graph.Id.ValueString(); !graph.Id.IsNull() && !graph.Id.IsUnknown() {
				if j, ok := seenIds[id]; ok {
					resp.Diagnostics.AddError(
						"Duplicate graph id",
						fmt.Sprintf("graphs[%d].id and graphs[%d].id are both %q; graph ids must be unique within a dashboard.", j, i, id),
					)
				} else {
					seenIds[id] = i
				}
			}

			resp.Diagnostics.Append(r.validateGraphLayout(graph, fmt.Sprintf("graphs[%d]", i))...)

			diags := r.validateVisualization(ctx, graph.Visualization, fmt.Sprintf("graphs[%d].visualization", i))
			resp.Diagnostics.Append(diags...)
		}
	}
}

// validateGraphLayout enforces the grid-layout constraints documented by the
// API but not expressible in the schema: 0 <= x, 0 <= y, 1 <= w <= 12,
// 1 <= h, x+w <= 12, and a 2x2 minimum size except for note widgets which may
// go down to 1x1.
func (r *dashboardResource) validateGraphLayout(graph resource_dashboard.GraphModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics
	if graph.Layout == nil {
		return diags
	}

	l := graph.Layout
	known := func(v types.Float64) bool { return !v.IsNull() && !v.IsUnknown() }

	x, y, w, h := l.X.ValueFloat64(), l.Y.ValueFloat64(), l.W.ValueFloat64(), l.H.ValueFloat64()
	path := pathPrefix + ".layout"

	if known(l.X) && x < 0 {
		diags.AddError("Invalid layout configuration", fmt.Sprintf("%s.x: must be >= 0, got %v.", path, x))
	}
	if known(l.Y) && y < 0 {
		diags.AddError("Invalid layout configuration", fmt.Sprintf("%s.y: must be >= 0, got %v.", path, y))
	}
	if known(l.W) && (w < 1 || w > 12) {
		diags.AddError("Invalid layout configuration", fmt.Sprintf("%s.w: must be between 1 and 12, got %v.", path, w))
	}
	if known(l.H) && h < 1 {
		diags.AddError("Invalid layout configuration", fmt.Sprintf("%s.h: must be >= 1, got %v.", path, h))
	}
	if known(l.X) && known(l.W) && x+w > 12 {
		diags.AddError("Invalid layout configuration", fmt.Sprintf("%s: x+w must be <= 12, got x=%v w=%v.", path, x, w))
	}

	minSize := 2.0
	if graph.Visualization.Note != nil {
		minSize = 1.0
	}
	if known(l.W) && w < minSize {
		diags.AddError("Invalid layout configuration", fmt.Sprintf("%s.w: widget size must be at least %v, got %v.", path, minSize, w))
	}
	if known(l.H) && h < minSize {
		diags.AddError("Invalid layout configuration", fmt.Sprintf("%s.h: widget size must be at least %v, got %v.", path, minSize, h))
	}

	return diags
}

func (r *dashboardResource) validateVisualization(ctx context.Context, vis resource_dashboard.VisualizationModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	if countSetVisualizations(vis) != 1 {
		diags.AddError(
			"Invalid visualization configuration",
			fmt.Sprintf("%s: exactly one visualization type must be set.", pathPrefix),
		)
	}

	// Validate nested aggregates in queries for series visualizations
	if vis.Timeseries != nil {
		diags.Append(r.validateSeriesVisualization(ctx, &vis.Timeseries.SeriesBase, fmt.Sprintf("%s.timeseries", pathPrefix))...)
		diags.Append(validateYAxisSettings(vis.Timeseries.YAxisSettings, fmt.Sprintf("%s.timeseries", pathPrefix))...)
	}
	if vis.TopList != nil {
		diags.Append(r.validateSeriesVisualization(ctx, &vis.TopList.SeriesBase, fmt.Sprintf("%s.top_list", pathPrefix))...)
	}
	if vis.Pie != nil {
		diags.Append(r.validateSeriesVisualization(ctx, &vis.Pie.SeriesBase, fmt.Sprintf("%s.pie", pathPrefix))...)
	}
	if vis.QueryValue != nil {
		diags.Append(r.validateSeriesVisualization(ctx, &vis.QueryValue.SeriesBase, fmt.Sprintf("%s.query_value", pathPrefix))...)
	}
	if vis.Bar != nil {
		diags.Append(r.validateSeriesVisualization(ctx, &vis.Bar.SeriesBase, fmt.Sprintf("%s.bar", pathPrefix))...)
		diags.Append(validateYAxisSettings(vis.Bar.YAxisSettings, fmt.Sprintf("%s.bar", pathPrefix))...)
	}
	if vis.Gauge != nil {
		diags.Append(r.validateSeriesVisualization(ctx, &vis.Gauge.SeriesBase, fmt.Sprintf("%s.gauge", pathPrefix))...)
	}
	if vis.Distribution != nil {
		diags.Append(r.validateSeriesVisualization(ctx, &vis.Distribution.SeriesBase, fmt.Sprintf("%s.distribution", pathPrefix))...)
	}
	if vis.Heatmap != nil {
		diags.Append(r.validateSeriesVisualization(ctx, &vis.Heatmap.SeriesBase, fmt.Sprintf("%s.heatmap", pathPrefix))...)
	}
	if vis.Table != nil {
		diags.Append(r.validateTableVisualization(ctx, vis.Table, fmt.Sprintf("%s.table", pathPrefix))...)
	}
	if vis.TimeseriesConnection != nil {
		diags.Append(validateYAxisSettings(vis.TimeseriesConnection.YAxisSettings, fmt.Sprintf("%s.timeseries_connection", pathPrefix))...)
	}
	if vis.BarConnection != nil {
		diags.Append(validateYAxisSettings(vis.BarConnection.YAxisSettings, fmt.Sprintf("%s.bar_connection", pathPrefix))...)
	}
	if vis.QueryValueConnection != nil {
		diags.Append(normalizer.Validate(vis.QueryValueConnection.Normalizer, fmt.Sprintf("%s.query_value_connection", pathPrefix))...)
	}
	if vis.List != nil {
		diags.Append(r.validateListColumns(ctx, vis.List.ListColumns, fmt.Sprintf("%s.list", pathPrefix))...)
	}
	if vis.ListSpans != nil {
		diags.Append(r.validateListColumns(ctx, vis.ListSpans.ListColumns, fmt.Sprintf("%s.list_spans", pathPrefix))...)
	}
	if vis.ListConnection != nil {
		diags.Append(r.validateListColumns(ctx, vis.ListConnection.ListColumns, fmt.Sprintf("%s.list_connection", pathPrefix))...)
	}
	if vis.ListRum != nil {
		diags.Append(r.validateListColumns(ctx, vis.ListRum.ListColumns, fmt.Sprintf("%s.list_rum", pathPrefix))...)
	}
	if vis.TimeseriesPromql != nil {
		diags.Append(normalizer.Validate(vis.TimeseriesPromql.Normalizer, fmt.Sprintf("%s.timeseries_promql", pathPrefix))...)
	}
	if vis.BarPromql != nil {
		diags.Append(normalizer.Validate(vis.BarPromql.Normalizer, fmt.Sprintf("%s.bar_promql", pathPrefix))...)
	}
	if vis.PiePromql != nil {
		diags.Append(normalizer.Validate(vis.PiePromql.Normalizer, fmt.Sprintf("%s.pie_promql", pathPrefix))...)
	}
	if vis.QueryValuePromql != nil {
		diags.Append(normalizer.Validate(vis.QueryValuePromql.Normalizer, fmt.Sprintf("%s.query_value_promql", pathPrefix))...)
	}
	if vis.TopListPromql != nil {
		diags.Append(normalizer.Validate(vis.TopListPromql.Normalizer, fmt.Sprintf("%s.top_list_promql", pathPrefix))...)
	}
	if vis.Slo != nil {
		diags.Append(validateSloOrganizeBy(vis.Slo.OrganizeBy, fmt.Sprintf("%s.slo", pathPrefix))...)
	}

	return diags
}

func validateSloOrganizeBy(o *resource_dashboard.SloOrganizeByModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics
	if o == nil || o.Type.IsUnknown() || o.Key.IsUnknown() {
		return diags
	}

	path := pathPrefix + ".organize_by"
	switch o.Type.ValueString() {
	case "key":
		if o.Key.IsNull() {
			diags.AddError("Invalid SLO organize_by configuration", fmt.Sprintf("%s: type \"key\" requires \"key\".", path))
		}
	case "team":
		if !o.Key.IsNull() {
			diags.AddError("Invalid SLO organize_by configuration", fmt.Sprintf("%s: type \"team\" does not accept \"key\".", path))
		}
	}
	return diags
}

// validateYAxisSettings enforces the required-value pairs the schema cannot
// express: a "number" bound requires "value", and a "pow" scale requires
// "exponent".
func validateYAxisSettings(y *resource_dashboard.YAxisSettingsModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics
	if y == nil {
		return diags
	}

	path := pathPrefix + ".y_axis_settings"
	if y.Min.Type.ValueString() == "number" && y.Min.Value.IsNull() {
		diags.AddError("Invalid y-axis configuration", fmt.Sprintf("%s.min: type \"number\" requires \"value\".", path))
	}
	if y.Max.Type.ValueString() == "number" && y.Max.Value.IsNull() {
		diags.AddError("Invalid y-axis configuration", fmt.Sprintf("%s.max: type \"number\" requires \"value\".", path))
	}
	if y.Scale.Type.ValueString() == "pow" && y.Scale.Exponent.IsNull() {
		diags.AddError("Invalid y-axis configuration", fmt.Sprintf("%s.scale: type \"pow\" requires \"exponent\".", path))
	}
	return diags
}

// validateListColumns validates the normalizers configured on list-style
// visualization columns.
func (r *dashboardResource) validateListColumns(ctx context.Context, listColumns types.List, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	if listColumns.IsNull() || listColumns.IsUnknown() {
		return diags
	}

	var columns []resource_dashboard.ListColumnModel
	diags.Append(listColumns.ElementsAs(ctx, &columns, false)...)
	if diags.HasError() {
		return diags
	}

	for i, col := range columns {
		diags.Append(normalizer.Validate(col.Normalizer, fmt.Sprintf("%s.list_columns[%d]", pathPrefix, i))...)
	}

	return diags
}

func (r *dashboardResource) validateTableVisualization(ctx context.Context, table *resource_dashboard.TableVisualizationModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	if table.Columns.IsNull() || table.Columns.IsUnknown() {
		return diags
	}

	var columns []resource_dashboard.TableColumnModel
	diags.Append(table.Columns.ElementsAs(ctx, &columns, false)...)
	if diags.HasError() {
		return diags
	}

	for i, col := range columns {
		if !col.Queries.IsNull() && !col.Queries.IsUnknown() {
			var queries []resource_dashboard.QueryModel
			diags.Append(col.Queries.ElementsAs(ctx, &queries, false)...)
			if diags.HasError() {
				return diags
			}
			for j, query := range queries {
				aggDiags := r.validateAggregate(query.Aggregate, fmt.Sprintf("%s.columns[%d].queries[%d].aggregate", pathPrefix, i, j))
				diags.Append(aggDiags...)
				diags.Append(r.validateQueryFunctions(ctx, query.Functions, fmt.Sprintf("%s.columns[%d].queries[%d]", pathPrefix, i, j))...)
			}
			diags.Append(validateFormulaReferences(col.Formula, len(queries), fmt.Sprintf("%s.columns[%d]", pathPrefix, i))...)
		}
		diags.Append(normalizer.Validate(col.Normalizer, fmt.Sprintf("%s.columns[%d]", pathPrefix, i))...)
	}

	return diags
}

func (r *dashboardResource) validateSeriesVisualization(ctx context.Context, sv *resource_dashboard.SeriesBase, pathPrefix string) diag.Diagnostics {
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
			diags.Append(r.validateQueryFunctions(ctx, query.Functions, fmt.Sprintf("%s.queries[%d]", pathPrefix, i))...)
		}
		diags.Append(validateFormulaReferences(sv.Formula, len(queries), pathPrefix)...)
	}

	diags.Append(normalizer.Validate(sv.Normalizer, pathPrefix)...)

	return diags
}

// validateFormulaReferences enforces that a formula may only reference
// submitted queries (q1..qN where N is the number of queries), matching the
// API's `tsuga-formula` format rule.
var formulaQueryRefPattern = regexp.MustCompile(`\bq(\d+)\b`)

func validateFormulaReferences(formula types.String, numQueries int, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics
	if formula.IsNull() || formula.IsUnknown() {
		return diags
	}

	for _, m := range formulaQueryRefPattern.FindAllStringSubmatch(formula.ValueString(), -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n < 1 || n > numQueries {
			diags.AddError(
				"Invalid formula configuration",
				fmt.Sprintf("%s.formula: references \"q%s\" but only %d queries were submitted (q1..q%d).", pathPrefix, m[1], numQueries, numQueries),
			)
		}
	}
	return diags
}

// validateQueryFunctions enforces the function-specific required parameters
// that the API mandates but the schema cannot express on its own: `log`
// requires `base` and `power` requires `exponent`.
func (r *dashboardResource) validateQueryFunctions(ctx context.Context, functions types.List, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	if functions.IsNull() || functions.IsUnknown() {
		return diags
	}

	var fns []resource_dashboard.FunctionModel
	diags.Append(functions.ElementsAs(ctx, &fns, false)...)
	if diags.HasError() {
		return diags
	}

	for i, fn := range fns {
		switch fn.Type.ValueString() {
		case "log":
			if fn.Base.IsNull() {
				diags.AddError(
					"Invalid query function configuration",
					fmt.Sprintf("%s.functions[%d]: the \"log\" function requires \"base\".", pathPrefix, i),
				)
			}
		case "power":
			if fn.Exponent.IsNull() {
				diags.AddError(
					"Invalid query function configuration",
					fmt.Sprintf("%s.functions[%d]: the \"power\" function requires \"exponent\".", pathPrefix, i),
				)
			}
		case "rolling":
			if !fn.Window.IsUnknown() && (fn.Window.IsNull() || fn.Window.ValueString() == "") {
				diags.AddError(
					"Invalid query function configuration",
					fmt.Sprintf("%s.functions[%d]: the \"rolling\" function requires \"window\".", pathPrefix, i),
				)
			}
		case "time-offset":
			if fn.Seconds.IsNull() {
				diags.AddError(
					"Invalid query function configuration",
					fmt.Sprintf("%s.functions[%d]: the \"time-offset\" function requires \"seconds\".", pathPrefix, i),
				)
			}
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

	r.readJSON(ctx, httpResp.Body, resp)
}

func (r *dashboardResource) readJSON(ctx context.Context, bodyReader io.Reader, resp *resource.ReadResponse) {
	raw, err := io.ReadAll(bodyReader)
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
	filters, expandDiags := expandDashboardFilters(ctx, plan.Filters)
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
	if !plan.TimePreset.IsNull() && !plan.TimePreset.IsUnknown() {
		body["timePreset"] = plan.TimePreset.ValueString()
	}
	// Sent even when null so dropping the attribute moves the dashboard out of its folder.
	if plan.FolderId.IsNull() {
		body["folderId"] = nil
	} else if !plan.FolderId.IsUnknown() {
		body["folderId"] = plan.FolderId.ValueString()
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
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Owner      string               `json:"owner"`
	FolderId   string               `json:"folderId,omitempty"`
	Filters    []dashboardAPIFilter `json:"filters"`
	Tags       []apiTag             `json:"tags"`
	TimePreset string               `json:"timePreset,omitempty"`
	Graphs     []dashboardAPIGraph  `json:"graphs"`
}

type dashboardAPIFilter struct {
	Key     string   `json:"key"`
	Values  []string `json:"values"`
	Exclude *bool    `json:"exclude,omitempty"`
}

type dashboardAPIGraph struct {
	ID                        string                 `json:"id"`
	Name                      string                 `json:"name"`
	Description               string                 `json:"description,omitempty"`
	DescriptionAlign          string                 `json:"descriptionAlign,omitempty"`
	DescriptionJustifyContent string                 `json:"descriptionJustifyContent,omitempty"`
	Layout                    *dashboardGraphLayout  `json:"layout,omitempty"`
	Visualization             dashboardVisualization `json:"visualization"`
}

type dashboardGraphLayout struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type dashboardVisualization struct {
	Type             string                        `json:"type"`
	Source           string                        `json:"source,omitempty"`
	Queries          []dashboardQuery              `json:"-"`
	RawQueries       []string                      `json:"-"`
	Formula          string                        `json:"formula,omitempty"`
	Aliases          *dashboardAliases             `json:"aliases,omitempty"`
	VisibleSeries    []bool                        `json:"visibleSeries,omitempty"`
	GroupBy          []dashboardGroupBy            `json:"groupBy,omitempty"`
	Normalizer       *dashboardNormalizer          `json:"normalizer,omitempty"`
	BackgroundMode   string                        `json:"backgroundMode,omitempty"`
	Conditions       []dashboardCondition          `json:"conditions,omitempty"`
	Precision        *float64                      `json:"precision,omitempty"`
	YAxisSettings    *dashboardYAxisSettings       `json:"yAxisSettings,omitempty"`
	TimeBucket       *dashboardTimeBucket          `json:"timeBucket,omitempty"`
	Query            string                        `json:"query,omitempty"`
	ListColumns      []dashboardListColumn         `json:"listColumns,omitempty"`
	ListColumnsSize  map[string]float64            `json:"listColumnsSize,omitempty"`
	DefaultSorting   []dashboardListSorting        `json:"defaultSorting,omitempty"`
	IsCellWrapped    *bool                         `json:"isCellWrapped,omitempty"`
	IsStacked        *bool                         `json:"isStacked,omitempty"`
	Note             string                        `json:"note,omitempty"`
	NoteAlign        string                        `json:"noteAlign,omitempty"`
	NoteJustify      string                        `json:"noteJustifyContent,omitempty"`
	NoteColor        string                        `json:"noteColor,omitempty"`
	Columns          []dashboardTableColumn        `json:"columns,omitempty"`
	ConnectionId     string                        `json:"connectionId,omitempty"`
	LegendMode       string                        `json:"legendMode,omitempty"`
	Thresholds       []dashboardThreshold          `json:"thresholds,omitempty"`
	Smoothing        *bool                         `json:"smoothing,omitempty"`
	Max              *float64                      `json:"max,omitempty"`
	ColorThresholds  []dashboardGaugeColor         `json:"colorThresholds,omitempty"`
	BoundsScale      string                        `json:"boundsScale,omitempty"`
	Markers          []int64                       `json:"percentileMarkers,omitempty"`
	Palette          string                        `json:"palette,omitempty"`
	Layout           string                        `json:"layout,omitempty"`
	CustomLink       *dashboardCustomLink          `json:"customLink,omitempty"`
	GroupByMode      string                        `json:"groupByMode,omitempty"`
	LineStyleOptions map[string]dashboardLineStyle `json:"lineStyleOptions,omitempty"`
	ColumnSizes      map[string]float64            `json:"columnSizes,omitempty"`
	Group            string                        `json:"group,omitempty"`
	SloId            string                        `json:"sloId,omitempty"`
	SloFilters       *dashboardSloFilters          `json:"filters,omitempty"`
	OrganizeBy       *dashboardSloOrganizeBy       `json:"organizeBy,omitempty"`
	Display          []string                      `json:"display,omitempty"`
}

type dashboardCustomLink struct {
	Label string `json:"label,omitempty"`
	Url   string `json:"url"`
}

type dashboardLineStyle struct {
	GroupId   string `json:"groupId"`
	LineStyle string `json:"lineStyle"`
}

type dashboardSloFilters struct {
	Owners   *dashboardSloFilter[string]       `json:"owners,omitempty"`
	Statuses *dashboardSloFilter[string]       `json:"statuses,omitempty"`
	Tags     *dashboardSloFilter[dashboardTag] `json:"tags,omitempty"`
}

type dashboardSloFilter[T any] struct {
	Values  []T   `json:"values"`
	Exclude *bool `json:"exclude,omitempty"`
}

// dashboardTag carries tfsdk tags so SLO tag filters convert to and from
// Terraform values directly.
type dashboardTag struct {
	Key   string `json:"key" tfsdk:"key"`
	Value string `json:"value" tfsdk:"value"`
}

type dashboardSloOrganizeBy struct {
	Type string `json:"type"`
	Key  string `json:"key,omitempty"`
}

type dashboardGaugeColor struct {
	From  float64 `json:"from"`
	Color string  `json:"color"`
}

// MarshalJSON handles the polymorphic "queries" field: []dashboardQuery for
// standard visualizations, []string (SQL or PromQL) for connection- and
// PromQL-based visualizations.
func (v dashboardVisualization) MarshalJSON() ([]byte, error) {
	type Alias dashboardVisualization
	aux := struct {
		Alias
		Queries interface{} `json:"queries,omitempty"`
	}{Alias: Alias(v)}
	if len(v.RawQueries) > 0 {
		aux.Queries = v.RawQueries
	} else if len(v.Queries) > 0 {
		aux.Queries = v.Queries
	}
	return json.Marshal(aux)
}

// UnmarshalJSON handles the polymorphic "queries" field based on visualization type.
func (v *dashboardVisualization) UnmarshalJSON(data []byte) error {
	type Alias dashboardVisualization
	aux := &struct {
		*Alias
		Queries json.RawMessage `json:"queries,omitempty"`
	}{Alias: (*Alias)(v)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if len(aux.Queries) > 0 {
		switch v.Type {
		case "timeseries-connection", "top-list-connection", "pie-connection", "bar-connection", "query-value-connection",
			"timeseries-promql", "bar-promql", "pie-promql", "query-value-promql", "top-list-promql":
			return json.Unmarshal(aux.Queries, &v.RawQueries)
		default:
			return json.Unmarshal(aux.Queries, &v.Queries)
		}
	}
	return nil
}

type dashboardThreshold struct {
	Value float64 `json:"value"`
	Level string  `json:"level"`
}

type dashboardAliases struct {
	Formula string            `json:"formula,omitempty"`
	Queries map[string]string `json:"queries,omitempty"`
}

type dashboardYAxisSettings struct {
	Min               dashboardYAxisBound `json:"min"`
	Max               dashboardYAxisBound `json:"max"`
	Scale             dashboardYAxisScale `json:"scale"`
	AlwaysIncludeZero bool                `json:"alwaysIncludeZero"`
}

type dashboardYAxisBound struct {
	Type  string   `json:"type"`
	Value *float64 `json:"value,omitempty"`
}

type dashboardYAxisScale struct {
	Type     string   `json:"type"`
	Exponent *float64 `json:"exponent,omitempty"`
}

type dashboardTableColumn struct {
	Name          string               `json:"name"`
	Source        string               `json:"source"`
	Queries       []dashboardQuery     `json:"queries"`
	Formula       string               `json:"formula,omitempty"`
	Aliases       *dashboardAliases    `json:"aliases,omitempty"`
	VisibleSeries []bool               `json:"visibleSeries,omitempty"`
	Normalizer    *dashboardNormalizer `json:"normalizer,omitempty"`
	Precision     *float64             `json:"precision,omitempty"`
}

type dashboardQuery struct {
	Aggregate     dashboardAggregate  `json:"aggregate"`
	Filter        string              `json:"filter,omitempty"`
	Functions     []dashboardFunction `json:"functions,omitempty"`
	TimeAggregate string              `json:"timeAggregate,omitempty"`
	MetricKind    string              `json:"metricKind,omitempty"`
	Fill          *dashboardQueryFill `json:"fill,omitempty"`
}

type dashboardQueryFill struct {
	Mode struct {
		Type string `json:"type"`
	} `json:"mode"`
}

type dashboardAggregate struct {
	Type       string  `json:"type"`
	Field      string  `json:"field,omitempty"`
	Percentile float64 `json:"percentile,omitempty"`
}

type dashboardFunction struct {
	Type     string `json:"type"`
	Window   string `json:"window,omitempty"`
	Seconds  *int64 `json:"seconds,omitempty"`
	Base     *int64 `json:"base,omitempty"`
	Exponent *int64 `json:"exponent,omitempty"`
}

type dashboardGroupBy struct {
	Fields          []string `json:"fields"`
	Limit           int64    `json:"limit"`
	SortOrder       string   `json:"sortOrder,omitempty"`
	ReplaceNullWith string   `json:"replaceNullWith,omitempty"`
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

type dashboardListSorting struct {
	Id   string `json:"id"`
	Desc bool   `json:"desc"`
}

func expandDashboardFilters(ctx context.Context, filters types.List) ([]dashboardAPIFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	if filters.IsNull() || filters.IsUnknown() {
		return nil, diags
	}

	var filterModels []resource_dashboard.FilterModel
	diags.Append(filters.ElementsAs(ctx, &filterModels, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]dashboardAPIFilter, 0, len(filterModels))
	for _, f := range filterModels {
		values, vDiags := expandStringList(ctx, f.Values)
		diags.Append(vDiags...)
		if diags.HasError() {
			return nil, diags
		}
		if values == nil {
			values = []string{}
		}
		filter := dashboardAPIFilter{
			Key:    f.Key.ValueString(),
			Values: values,
		}
		if !f.Exclude.IsNull() && !f.Exclude.IsUnknown() {
			exclude := f.Exclude.ValueBool()
			filter.Exclude = &exclude
		}
		result = append(result, filter)
	}
	return result, diags
}

func flattenDashboardFilters(ctx context.Context, filters []dashboardAPIFilter) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.FilterAttrTypes()}
	if filters == nil {
		return types.ListNull(elemType), nil
	}
	if len(filters) == 0 {
		return types.ListValue(elemType, []attr.Value{})
	}

	var diags diag.Diagnostics
	values := make([]attr.Value, 0, len(filters))
	for _, f := range filters {
		filterValues := f.Values
		if filterValues == nil {
			filterValues = []string{}
		}
		valsList, vDiags := types.ListValueFrom(ctx, types.StringType, filterValues)
		diags.Append(vDiags...)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}
		values = append(values, types.ObjectValueMust(resource_dashboard.FilterAttrTypes(), map[string]attr.Value{
			"key":     types.StringValue(f.Key),
			"values":  valsList,
			"exclude": types.BoolPointerValue(f.Exclude),
		}))
	}
	return types.ListValue(elemType, values)
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
			ID:                        g.Id.ValueString(),
			Name:                      g.Name.ValueString(),
			Description:               g.Description.ValueString(),
			DescriptionAlign:          g.DescriptionAlign.ValueString(),
			DescriptionJustifyContent: g.DescriptionJustifyContent.ValueString(),
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
	if countSetVisualizations(v) != 1 {
		diags.AddError("Invalid visualization", "Exactly one visualization block must be set.")
		return dashboardVisualization{}, diags
	}
	var vis dashboardVisualization

	buildSeries := func(b *resource_dashboard.SeriesBase, groupBy types.List, yAxis *resource_dashboard.YAxisSettingsModel, visType string) (dashboardVisualization, diag.Diagnostics) {
		var d diag.Diagnostics
		queries, qDiags := expandQueries(ctx, b.Queries)
		d.Append(qDiags...)
		groupByExpanded, gDiags := expandGroupBy(ctx, groupBy)
		d.Append(gDiags...)
		normalizer := expandNormalizer(b.Normalizer)

		var visible []bool
		if !b.VisibleSeries.IsNull() && !b.VisibleSeries.IsUnknown() {
			var boolDiags diag.Diagnostics
			visible, boolDiags = expandBoolList(ctx, b.VisibleSeries)
			d.Append(boolDiags...)
		}

		result := dashboardVisualization{
			Type:          visType,
			Source:        b.Source.ValueString(),
			Queries:       queries,
			Formula:       b.Formula.ValueString(),
			VisibleSeries: visible,
			GroupBy:       groupByExpanded,
			Normalizer:    normalizer,
		}
		if !b.Precision.IsNull() && !b.Precision.IsUnknown() {
			precision := b.Precision.ValueFloat64()
			result.Precision = &precision
		}
		result.Aliases = expandAliases(b.Aliases)
		result.YAxisSettings = expandYAxisSettings(yAxis)
		return result, d
	}

	buildList := func(l *resource_dashboard.ListVisualization, visType string) (dashboardVisualization, diag.Diagnostics) {
		var d diag.Diagnostics
		vz := dashboardVisualization{
			Type:  visType,
			Query: l.Query.ValueString(),
		}
		cols, cDiags := expandListColumns(ctx, l.ListColumns)
		d.Append(cDiags...)
		vz.ListColumns = cols
		vz.ListColumnsSize = expandListColumnsSize(l.ListColumnsSize)
		sorting, sDiags := expandListDefaultSorting(ctx, l.DefaultSorting)
		d.Append(sDiags...)
		vz.DefaultSorting = sorting
		if !l.IsCellWrapped.IsNull() && !l.IsCellWrapped.IsUnknown() {
			wrapped := l.IsCellWrapped.ValueBool()
			vz.IsCellWrapped = &wrapped
		}
		return vz, d
	}

	buildPromql := func(b *resource_dashboard.PromqlBase, visType string) (dashboardVisualization, diag.Diagnostics) {
		rawQueries, d := expandStringList(ctx, b.Queries)
		return dashboardVisualization{
			Type:       visType,
			RawQueries: rawQueries,
			Aliases:    expandAliases(b.Aliases),
			Normalizer: expandNormalizer(b.Normalizer),
			Precision:  b.Precision.ValueFloat64Pointer(),
		}, d
	}

	buildChartPromql := func(b *resource_dashboard.BarPromqlVisualization, visType string) (dashboardVisualization, diag.Diagnostics) {
		vz, d := buildPromql(&b.PromqlBase, visType)
		vz.LegendMode = stringValue(b.LegendMode)
		thresholds, tDiags := expandThresholds(ctx, b.Thresholds)
		d.Append(tDiags...)
		vz.Thresholds = thresholds
		vz.TimeBucket = expandTimeBucket(b.TimeBucket)
		vz.YAxisSettings = expandYAxisSettings(b.YAxisSettings)
		return vz, d
	}

	if v.Timeseries != nil {
		vz, d := buildSeries(&v.Timeseries.SeriesBase, v.Timeseries.GroupBy, v.Timeseries.YAxisSettings, "timeseries")
		diags.Append(d...)
		if !v.Timeseries.Smoothing.IsNull() && !v.Timeseries.Smoothing.IsUnknown() {
			smoothing := v.Timeseries.Smoothing.ValueBool()
			vz.Smoothing = &smoothing
		}
		vz.TimeBucket = expandTimeBucket(v.Timeseries.TimeBucket)
		vz.LegendMode = stringValue(v.Timeseries.LegendMode)
		vz.CustomLink = expandCustomLink(v.Timeseries.CustomLink)
		vz.GroupByMode = stringValue(v.Timeseries.GroupByMode)
		lineStyles, lDiags := expandLineStyleOptions(ctx, v.Timeseries.LineStyleOptions)
		diags.Append(lDiags...)
		vz.LineStyleOptions = lineStyles
		thresholds, tDiags := expandThresholds(ctx, v.Timeseries.Thresholds)
		diags.Append(tDiags...)
		vz.Thresholds = thresholds
		vis = vz
	}
	if v.TopList != nil {
		vz, d := buildSeries(&v.TopList.SeriesBase, v.TopList.GroupBy, nil, "top-list")
		diags.Append(d...)
		if !v.TopList.Conditions.IsNull() && !v.TopList.Conditions.IsUnknown() {
			conds, cDiags := expandConditions(ctx, v.TopList.Conditions)
			diags.Append(cDiags...)
			vz.Conditions = conds
		}
		if !v.TopList.IsStacked.IsNull() && !v.TopList.IsStacked.IsUnknown() {
			stacked := v.TopList.IsStacked.ValueBool()
			vz.IsStacked = &stacked
		}
		vz.CustomLink = expandCustomLink(v.TopList.CustomLink)
		vz.GroupByMode = stringValue(v.TopList.GroupByMode)
		vz.LegendMode = stringValue(v.TopList.LegendMode)
		vis = vz
	}
	if v.Pie != nil {
		vz, d := buildSeries(&v.Pie.SeriesBase, v.Pie.GroupBy, nil, "pie")
		diags.Append(d...)
		vz.LegendMode = stringValue(v.Pie.LegendMode)
		vz.CustomLink = expandCustomLink(v.Pie.CustomLink)
		vz.GroupByMode = stringValue(v.Pie.GroupByMode)
		vis = vz
	}
	if v.QueryValue != nil {
		vz, d := buildSeries(&v.QueryValue.SeriesBase, types.List{}, nil, "query-value")
		diags.Append(d...)
		vz.LegendMode = stringValue(v.QueryValue.LegendMode)
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
		vz, d := buildSeries(&v.Bar.SeriesBase, v.Bar.GroupBy, v.Bar.YAxisSettings, "bar")
		diags.Append(d...)
		vz.TimeBucket = expandTimeBucket(v.Bar.TimeBucket)
		vz.LegendMode = stringValue(v.Bar.LegendMode)
		vz.CustomLink = expandCustomLink(v.Bar.CustomLink)
		vz.GroupByMode = stringValue(v.Bar.GroupByMode)
		thresholds, tDiags := expandThresholds(ctx, v.Bar.Thresholds)
		diags.Append(tDiags...)
		vz.Thresholds = thresholds
		vis = vz
	}
	if v.Gauge != nil {
		vz, d := buildSeries(&v.Gauge.SeriesBase, types.List{}, nil, "gauge")
		diags.Append(d...)
		if !v.Gauge.Max.IsNull() && !v.Gauge.Max.IsUnknown() {
			m := v.Gauge.Max.ValueFloat64()
			vz.Max = &m
		}
		if !v.Gauge.ColorThresholds.IsNull() && !v.Gauge.ColorThresholds.IsUnknown() {
			cts, ctDiags := expandGaugeColorThresholds(ctx, v.Gauge.ColorThresholds)
			diags.Append(ctDiags...)
			vz.ColorThresholds = cts
		}
		vis = vz
	}
	if v.Distribution != nil {
		vz, d := buildSeries(&v.Distribution.SeriesBase, v.Distribution.GroupBy, nil, "distribution")
		diags.Append(d...)
		vz.BoundsScale = stringValue(v.Distribution.BoundsScale)
		if !v.Distribution.PercentileMarkers.IsNull() && !v.Distribution.PercentileMarkers.IsUnknown() {
			markers, mDiags := expandIntList(ctx, v.Distribution.PercentileMarkers)
			diags.Append(mDiags...)
			vz.Markers = markers
		}
		vz.Group = stringValue(v.Distribution.Group)
		vis = vz
	}
	if v.Heatmap != nil {
		vz, d := buildSeries(&v.Heatmap.SeriesBase, v.Heatmap.GroupBy, v.Heatmap.YAxisSettings, "heatmap")
		diags.Append(d...)
		vz.Palette = stringValue(v.Heatmap.Palette)
		vz.Group = stringValue(v.Heatmap.Group)
		vis = vz
	}
	if v.ListLogPatterns != nil {
		vz := dashboardVisualization{
			Type:   "list-log-patterns",
			Query:  v.ListLogPatterns.Query.ValueString(),
			Layout: stringValue(v.ListLogPatterns.Layout),
		}
		vis = vz
	}
	if v.TopListConnection != nil {
		rawQueries, sqDiags := expandStringList(ctx, v.TopListConnection.Queries)
		diags.Append(sqDiags...)
		vis = dashboardVisualization{
			Type:         "top-list-connection",
			ConnectionId: v.TopListConnection.ConnectionId.ValueString(),
			RawQueries:   rawQueries,
		}
	}
	if v.PieConnection != nil {
		rawQueries, sqDiags := expandStringList(ctx, v.PieConnection.Queries)
		diags.Append(sqDiags...)
		vis = dashboardVisualization{
			Type:         "pie-connection",
			ConnectionId: v.PieConnection.ConnectionId.ValueString(),
			RawQueries:   rawQueries,
			LegendMode:   stringValue(v.PieConnection.LegendMode),
		}
	}
	if v.BarConnection != nil {
		rawQueries, sqDiags := expandStringList(ctx, v.BarConnection.Queries)
		diags.Append(sqDiags...)
		vz := dashboardVisualization{
			Type:         "bar-connection",
			ConnectionId: v.BarConnection.ConnectionId.ValueString(),
			RawQueries:   rawQueries,
			LegendMode:   stringValue(v.BarConnection.LegendMode),
		}
		thresholds, tDiags := expandThresholds(ctx, v.BarConnection.Thresholds)
		diags.Append(tDiags...)
		vz.Thresholds = thresholds
		vz.YAxisSettings = expandYAxisSettings(v.BarConnection.YAxisSettings)
		vis = vz
	}
	if v.QueryValueConnection != nil {
		rawQueries, sqDiags := expandStringList(ctx, v.QueryValueConnection.Queries)
		diags.Append(sqDiags...)
		vz := dashboardVisualization{
			Type:           "query-value-connection",
			ConnectionId:   v.QueryValueConnection.ConnectionId.ValueString(),
			RawQueries:     rawQueries,
			LegendMode:     stringValue(v.QueryValueConnection.LegendMode),
			BackgroundMode: stringValue(v.QueryValueConnection.BackgroundMode),
			Normalizer:     expandNormalizer(v.QueryValueConnection.Normalizer),
		}
		if !v.QueryValueConnection.Conditions.IsNull() && !v.QueryValueConnection.Conditions.IsUnknown() {
			conds, cDiags := expandConditions(ctx, v.QueryValueConnection.Conditions)
			diags.Append(cDiags...)
			vz.Conditions = conds
		}
		if !v.QueryValueConnection.Precision.IsNull() && !v.QueryValueConnection.Precision.IsUnknown() {
			p := v.QueryValueConnection.Precision.ValueFloat64()
			vz.Precision = &p
		}
		vis = vz
	}
	if v.List != nil {
		vz, d := buildList(v.List, "list")
		diags.Append(d...)
		vis = vz
	}
	if v.ListSpans != nil {
		vz, d := buildList(v.ListSpans, "list-spans")
		diags.Append(d...)
		vis = vz
	}
	if v.ListRum != nil {
		vz, d := buildList(v.ListRum, "list-rum")
		diags.Append(d...)
		vis = vz
	}
	if v.Note != nil {
		vz := dashboardVisualization{
			Type:        "note",
			Note:        v.Note.Note.ValueString(),
			NoteAlign:   stringValue(v.Note.NoteAlign),
			NoteJustify: stringValue(v.Note.NoteJustifyContent),
			NoteColor:   stringValue(v.Note.NoteColor),
		}
		vis = vz
	}
	if v.Table != nil {
		vz := dashboardVisualization{
			Type: "table",
		}
		cols, colDiags := expandTableColumns(ctx, v.Table.Columns)
		diags.Append(colDiags...)
		vz.Columns = cols
		gb, gDiags := expandGroupBy(ctx, v.Table.GroupBy)
		diags.Append(gDiags...)
		vz.GroupBy = gb
		vz.GroupByMode = stringValue(v.Table.GroupByMode)
		vz.CustomLink = expandCustomLink(v.Table.CustomLink)
		vz.ColumnSizes = expandListColumnsSize(v.Table.ColumnSizes)
		sorting, sDiags := expandListDefaultSorting(ctx, v.Table.DefaultSorting)
		diags.Append(sDiags...)
		vz.DefaultSorting = sorting
		vis = vz
	}
	if v.TimeseriesConnection != nil {
		rawQueries, sqDiags := expandStringList(ctx, v.TimeseriesConnection.Queries)
		diags.Append(sqDiags...)
		vz := dashboardVisualization{
			Type:         "timeseries-connection",
			ConnectionId: v.TimeseriesConnection.ConnectionId.ValueString(),
			RawQueries:   rawQueries,
			LegendMode:   stringValue(v.TimeseriesConnection.LegendMode),
		}
		thresholds, tDiags := expandThresholds(ctx, v.TimeseriesConnection.Thresholds)
		diags.Append(tDiags...)
		vz.Thresholds = thresholds
		vz.YAxisSettings = expandYAxisSettings(v.TimeseriesConnection.YAxisSettings)
		lineStyles, lDiags := expandLineStyleOptions(ctx, v.TimeseriesConnection.LineStyleOptions)
		diags.Append(lDiags...)
		vz.LineStyleOptions = lineStyles
		vis = vz
	}
	if v.ListConnection != nil {
		vz := dashboardVisualization{
			Type:         "list-connection",
			ConnectionId: v.ListConnection.ConnectionId.ValueString(),
			Query:        v.ListConnection.Query.ValueString(),
		}
		cols, cDiags := expandListColumns(ctx, v.ListConnection.ListColumns)
		diags.Append(cDiags...)
		vz.ListColumns = cols
		vz.ListColumnsSize = expandListColumnsSize(v.ListConnection.ListColumnsSize)
		sorting, sDiags := expandListDefaultSorting(ctx, v.ListConnection.DefaultSorting)
		diags.Append(sDiags...)
		vz.DefaultSorting = sorting
		if !v.ListConnection.IsCellWrapped.IsNull() && !v.ListConnection.IsCellWrapped.IsUnknown() {
			wrapped := v.ListConnection.IsCellWrapped.ValueBool()
			vz.IsCellWrapped = &wrapped
		}
		vis = vz
	}

	if v.TimeseriesPromql != nil {
		vz, d := buildChartPromql(&v.TimeseriesPromql.BarPromqlVisualization, "timeseries-promql")
		diags.Append(d...)
		lineStyles, lDiags := expandLineStyleOptions(ctx, v.TimeseriesPromql.LineStyleOptions)
		diags.Append(lDiags...)
		vz.LineStyleOptions = lineStyles
		vz.Smoothing = v.TimeseriesPromql.Smoothing.ValueBoolPointer()
		vis = vz
	}
	if v.BarPromql != nil {
		vz, d := buildChartPromql(v.BarPromql, "bar-promql")
		diags.Append(d...)
		vis = vz
	}
	if v.PiePromql != nil {
		vz, d := buildPromql(&v.PiePromql.PromqlBase, "pie-promql")
		diags.Append(d...)
		vz.LegendMode = stringValue(v.PiePromql.LegendMode)
		vis = vz
	}
	if v.QueryValuePromql != nil {
		p := v.QueryValuePromql
		vz, d := buildPromql(&p.PromqlBase, "query-value-promql")
		diags.Append(d...)
		vz.LegendMode = stringValue(p.LegendMode)
		vz.BackgroundMode = stringValue(p.BackgroundMode)
		conds, cDiags := expandConditions(ctx, p.Conditions)
		diags.Append(cDiags...)
		vz.Conditions = conds
		vis = vz
	}
	if v.TopListPromql != nil {
		vz, d := buildPromql(&v.TopListPromql.PromqlBase, "top-list-promql")
		diags.Append(d...)
		conds, cDiags := expandConditions(ctx, v.TopListPromql.Conditions)
		diags.Append(cDiags...)
		vz.Conditions = conds
		vis = vz
	}
	if v.Slo != nil {
		display, dDiags := expandStringList(ctx, v.Slo.Display)
		diags.Append(dDiags...)
		filters, fDiags := expandSloFilters(ctx, v.Slo.Filters)
		diags.Append(fDiags...)
		vis = dashboardVisualization{
			Type:       "slo",
			SloId:      stringValue(v.Slo.SloId),
			Display:    display,
			OrganizeBy: expandSloOrganizeBy(v.Slo.OrganizeBy),
			SloFilters: filters,
		}
	}
	if v.SloOverTime != nil {
		vis = dashboardVisualization{Type: "slo-over-time", SloId: v.SloOverTime.SloId.ValueString()}
	}
	if v.SloUptime != nil {
		vis = dashboardVisualization{Type: "slo-uptime", SloId: v.SloUptime.SloId.ValueString()}
	}

	return vis, diags
}

func countSetVisualizations(v resource_dashboard.VisualizationModel) int {
	count := 0
	fields := reflect.ValueOf(v)
	for i := range fields.NumField() {
		if !fields.Field(i).IsNil() {
			count++
		}
	}
	return count
}

func expandTimeBucket(tb *resource_dashboard.TimeBucketModel) *dashboardTimeBucket {
	if tb == nil {
		return nil
	}
	return &dashboardTimeBucket{Time: tb.Time.ValueFloat64(), Metric: tb.Metric.ValueString()}
}

func flattenDashboard(ctx context.Context, data dashboardAPIData) (resource_dashboard.DashboardModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	filters, filterDiags := flattenDashboardFilters(ctx, data.Filters)
	diags.Append(filterDiags...)
	tags, tagDiags := flattenTags(ctx, data.Tags)
	diags.Append(tagDiags...)
	graphs, graphDiags := flattenDashboardGraphs(ctx, data.Graphs)
	diags.Append(graphDiags...)

	state := resource_dashboard.DashboardModel{
		Id:         types.StringValue(data.ID),
		Name:       types.StringValue(data.Name),
		Owner:      types.StringValue(data.Owner),
		FolderId:   stringValueOrNull(data.FolderId),
		Filters:    filters,
		Tags:       tags,
		TimePreset: stringValueOrNull(data.TimePreset),
		Graphs:     graphs,
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
			"id":                          types.StringValue(g.ID),
			"name":                        stringValueOrNull(g.Name),
			"description":                 stringValueOrNull(g.Description),
			"description_align":           stringValueOrNull(g.DescriptionAlign),
			"description_justify_content": stringValueOrNull(g.DescriptionJustifyContent),
			"layout":                      layoutVal,
			"visualization":               visVal,
		}))
	}

	return types.ListValue(elemType, values)
}

func flattenVisualization(ctx context.Context, vis dashboardVisualization) (attr.Value, diag.Diagnostics) {
	nullVizBase := func() map[string]attr.Value {
		return map[string]attr.Value{
			"timeseries":             types.ObjectNull(resource_dashboard.TimeseriesVisualizationAttrTypes()),
			"top_list":               types.ObjectNull(resource_dashboard.TopListVisualizationAttrTypes()),
			"pie":                    types.ObjectNull(resource_dashboard.PieVisualizationAttrTypes()),
			"query_value":            types.ObjectNull(resource_dashboard.QueryValueVisualizationAttrTypes()),
			"bar":                    types.ObjectNull(resource_dashboard.BarVisualizationAttrTypes()),
			"gauge":                  types.ObjectNull(resource_dashboard.GaugeVisualizationAttrTypes()),
			"distribution":           types.ObjectNull(resource_dashboard.DistributionVisualizationAttrTypes()),
			"heatmap":                types.ObjectNull(resource_dashboard.HeatmapVisualizationAttrTypes()),
			"list":                   types.ObjectNull(resource_dashboard.ListVisualizationAttrTypes()),
			"list_spans":             types.ObjectNull(resource_dashboard.ListVisualizationAttrTypes()),
			"list_log_patterns":      types.ObjectNull(resource_dashboard.ListLogPatternsVisualizationAttrTypes()),
			"note":                   types.ObjectNull(resource_dashboard.NoteVisualizationAttrTypes()),
			"table":                  types.ObjectNull(resource_dashboard.TableVisualizationAttrTypes()),
			"timeseries_connection":  types.ObjectNull(resource_dashboard.TimeseriesConnectionVisualizationAttrTypes()),
			"list_connection":        types.ObjectNull(resource_dashboard.ListConnectionVisualizationAttrTypes()),
			"top_list_connection":    types.ObjectNull(resource_dashboard.TopListConnectionVisualizationAttrTypes()),
			"pie_connection":         types.ObjectNull(resource_dashboard.PieConnectionVisualizationAttrTypes()),
			"bar_connection":         types.ObjectNull(resource_dashboard.BarConnectionVisualizationAttrTypes()),
			"query_value_connection": types.ObjectNull(resource_dashboard.QueryValueConnectionVisualizationAttrTypes()),
			"timeseries_promql":      types.ObjectNull(resource_dashboard.TimeseriesPromqlVisualizationAttrTypes()),
			"bar_promql":             types.ObjectNull(resource_dashboard.BarPromqlVisualizationAttrTypes()),
			"pie_promql":             types.ObjectNull(resource_dashboard.PiePromqlVisualizationAttrTypes()),
			"query_value_promql":     types.ObjectNull(resource_dashboard.QueryValuePromqlVisualizationAttrTypes()),
			"top_list_promql":        types.ObjectNull(resource_dashboard.TopListPromqlVisualizationAttrTypes()),
			"list_rum":               types.ObjectNull(resource_dashboard.ListVisualizationAttrTypes()),
			"slo":                    types.ObjectNull(resource_dashboard.SloVisualizationAttrTypes()),
			"slo_over_time":          types.ObjectNull(resource_dashboard.SloIdVisualizationAttrTypes()),
			"slo_uptime":             types.ObjectNull(resource_dashboard.SloIdVisualizationAttrTypes()),
		}
	}

	switch vis.Type {
	case "note":
		obj := nullVizBase()
		obj["note"] = types.ObjectValueMust(resource_dashboard.NoteVisualizationAttrTypes(), map[string]attr.Value{
			"type":                 types.StringValue("note"),
			"note":                 types.StringValue(vis.Note),
			"note_align":           stringValueOrNull(vis.NoteAlign),
			"note_justify_content": stringValueOrNull(vis.NoteJustify),
			"note_color":           stringValueOrNull(vis.NoteColor),
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), nil
	case "list", "list-spans", "list-rum":
		listCols, diags := flattenListColumns(vis.ListColumns)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		sorting, sDiags := flattenListDefaultSorting(vis.DefaultSorting)
		diags.Append(sDiags...)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		listVal := types.ObjectValueMust(resource_dashboard.ListVisualizationAttrTypes(), map[string]attr.Value{
			"type":              types.StringValue(vis.Type),
			"query":             types.StringValue(vis.Query),
			"list_columns":      listCols,
			"list_columns_size": flattenListColumnsSize(vis.ListColumnsSize),
			"default_sorting":   sorting,
			"is_cell_wrapped":   types.BoolPointerValue(vis.IsCellWrapped),
		})
		obj := nullVizBase()
		switch vis.Type {
		case "list-spans":
			obj["list_spans"] = listVal
		case "list-rum":
			obj["list_rum"] = listVal
		default:
			obj["list"] = listVal
		}
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), diags
	case "table":
		cols, colDiags := flattenTableColumns(ctx, vis.Columns)
		if colDiags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), colDiags
		}
		groupBy, gDiags := flattenGroupBy(ctx, vis.GroupBy)
		if gDiags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), gDiags
		}
		sorting, sDiags := flattenListDefaultSorting(vis.DefaultSorting)
		if sDiags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), sDiags
		}
		obj := nullVizBase()
		obj["table"] = types.ObjectValueMust(resource_dashboard.TableVisualizationAttrTypes(), map[string]attr.Value{
			"type":            types.StringValue("table"),
			"columns":         cols,
			"group_by":        groupBy,
			"group_by_mode":   stringValueOrNull(vis.GroupByMode),
			"custom_link":     flattenCustomLink(vis.CustomLink),
			"column_sizes":    flattenListColumnsSize(vis.ColumnSizes),
			"default_sorting": sorting,
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), nil
	case "bar", "query-value", "timeseries", "top-list", "pie", "gauge", "distribution", "heatmap":
		seriesVal, diags := flattenSeriesVisualization(ctx, vis)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		obj := nullVizBase()
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
		case "gauge":
			obj["gauge"] = seriesVal
		case "distribution":
			obj["distribution"] = seriesVal
		case "heatmap":
			obj["heatmap"] = seriesVal
		}
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), nil
	case "list-log-patterns":
		obj := nullVizBase()
		obj["list_log_patterns"] = types.ObjectValueMust(resource_dashboard.ListLogPatternsVisualizationAttrTypes(), map[string]attr.Value{
			"type":   types.StringValue("list-log-patterns"),
			"query":  types.StringValue(vis.Query),
			"layout": stringValueOrNull(vis.Layout),
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), nil
	case "top-list-connection":
		obj := nullVizBase()
		rawQueries, diags := flattenStringList(vis.RawQueries)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		obj["top_list_connection"] = types.ObjectValueMust(resource_dashboard.TopListConnectionVisualizationAttrTypes(), map[string]attr.Value{
			"type":          types.StringValue("top-list-connection"),
			"connection_id": types.StringValue(vis.ConnectionId),
			"queries":       rawQueries,
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), nil
	case "pie-connection":
		obj := nullVizBase()
		rawQueries, diags := flattenStringList(vis.RawQueries)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		obj["pie_connection"] = types.ObjectValueMust(resource_dashboard.PieConnectionVisualizationAttrTypes(), map[string]attr.Value{
			"type":          types.StringValue("pie-connection"),
			"connection_id": types.StringValue(vis.ConnectionId),
			"queries":       rawQueries,
			"legend_mode":   stringValueOrNull(vis.LegendMode),
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), nil
	case "bar-connection":
		obj := nullVizBase()
		rawQueries, diags := flattenStringList(vis.RawQueries)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		thresholds, tDiags := flattenThresholds(vis.Thresholds)
		diags.Append(tDiags...)
		obj["bar_connection"] = types.ObjectValueMust(resource_dashboard.BarConnectionVisualizationAttrTypes(), map[string]attr.Value{
			"type":            types.StringValue("bar-connection"),
			"connection_id":   types.StringValue(vis.ConnectionId),
			"queries":         rawQueries,
			"legend_mode":     stringValueOrNull(vis.LegendMode),
			"thresholds":      thresholds,
			"y_axis_settings": flattenYAxisSettings(vis.YAxisSettings),
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), diags
	case "query-value-connection":
		obj := nullVizBase()
		rawQueries, diags := flattenStringList(vis.RawQueries)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		condVal, cDiags := flattenConditions(vis.Conditions)
		diags.Append(cDiags...)
		precisionVal := types.Float64Null()
		if vis.Precision != nil {
			precisionVal = types.Float64Value(*vis.Precision)
		}
		obj["query_value_connection"] = types.ObjectValueMust(resource_dashboard.QueryValueConnectionVisualizationAttrTypes(), map[string]attr.Value{
			"type":            types.StringValue("query-value-connection"),
			"connection_id":   types.StringValue(vis.ConnectionId),
			"queries":         rawQueries,
			"legend_mode":     stringValueOrNull(vis.LegendMode),
			"background_mode": stringValueOrNull(vis.BackgroundMode),
			"conditions":      condVal,
			"normalizer":      flattenNormalizer(vis.Normalizer),
			"precision":       precisionVal,
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), diags
	case "timeseries-connection":
		obj := nullVizBase()
		rawQueries, diags := flattenStringList(vis.RawQueries)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		thresholds, tDiags := flattenThresholds(vis.Thresholds)
		diags.Append(tDiags...)
		obj["timeseries_connection"] = types.ObjectValueMust(resource_dashboard.TimeseriesConnectionVisualizationAttrTypes(), map[string]attr.Value{
			"type":               types.StringValue("timeseries-connection"),
			"connection_id":      types.StringValue(vis.ConnectionId),
			"queries":            rawQueries,
			"legend_mode":        stringValueOrNull(vis.LegendMode),
			"thresholds":         thresholds,
			"y_axis_settings":    flattenYAxisSettings(vis.YAxisSettings),
			"line_style_options": flattenLineStyleOptions(vis.LineStyleOptions),
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), diags
	case "list-connection":
		listCols, diags := flattenListColumns(vis.ListColumns)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		sorting, sDiags := flattenListDefaultSorting(vis.DefaultSorting)
		diags.Append(sDiags...)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		obj := nullVizBase()
		obj["list_connection"] = types.ObjectValueMust(resource_dashboard.ListConnectionVisualizationAttrTypes(), map[string]attr.Value{
			"type":              types.StringValue("list-connection"),
			"connection_id":     types.StringValue(vis.ConnectionId),
			"query":             types.StringValue(vis.Query),
			"list_columns":      listCols,
			"list_columns_size": flattenListColumnsSize(vis.ListColumnsSize),
			"default_sorting":   sorting,
			"is_cell_wrapped":   types.BoolPointerValue(vis.IsCellWrapped),
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), diags
	case "timeseries-promql", "bar-promql", "pie-promql", "query-value-promql", "top-list-promql":
		promqlVal, diags := flattenPromqlVisualization(ctx, vis)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		obj := nullVizBase()
		obj[strings.ReplaceAll(vis.Type, "-", "_")] = promqlVal
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), diags
	case "slo":
		display, diags := flattenStringList(vis.Display)
		filters, fDiags := flattenSloFilters(ctx, vis.SloFilters)
		diags.Append(fDiags...)
		if diags.HasError() {
			return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
		}
		obj := nullVizBase()
		obj["slo"] = types.ObjectValueMust(resource_dashboard.SloVisualizationAttrTypes(), map[string]attr.Value{
			"type":        types.StringValue("slo"),
			"slo_id":      stringValueOrNull(vis.SloId),
			"display":     display,
			"organize_by": flattenSloOrganizeBy(vis.OrganizeBy),
			"filters":     filters,
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), diags
	case "slo-over-time", "slo-uptime":
		obj := nullVizBase()
		obj[strings.ReplaceAll(vis.Type, "-", "_")] = types.ObjectValueMust(resource_dashboard.SloIdVisualizationAttrTypes(), map[string]attr.Value{
			"type":   types.StringValue(vis.Type),
			"slo_id": types.StringValue(vis.SloId),
		})
		return types.ObjectValueMust(resource_dashboard.VisualizationAttrTypes(), obj), nil
	default:
		var diags diag.Diagnostics
		diags.AddError("Unsupported visualization type", vis.Type)
		return types.ObjectNull(resource_dashboard.VisualizationAttrTypes()), diags
	}
}

func flattenPromqlVisualization(ctx context.Context, vis dashboardVisualization) (attr.Value, diag.Diagnostics) {
	queries, diags := flattenStringList(vis.RawQueries)
	obj := map[string]attr.Value{
		"type":       types.StringValue(vis.Type),
		"queries":    queries,
		"aliases":    flattenAliases(ctx, vis.Aliases),
		"normalizer": flattenNormalizer(vis.Normalizer),
		"precision":  types.Float64PointerValue(vis.Precision),
	}

	switch vis.Type {
	case "timeseries-promql", "bar-promql":
		thresholds, tDiags := flattenThresholds(vis.Thresholds)
		diags.Append(tDiags...)
		obj["legend_mode"] = stringValueOrNull(vis.LegendMode)
		obj["thresholds"] = thresholds
		obj["time_bucket"] = flattenTimeBucket(vis.TimeBucket)
		obj["y_axis_settings"] = flattenYAxisSettings(vis.YAxisSettings)
		if vis.Type == "bar-promql" {
			return types.ObjectValueMust(resource_dashboard.BarPromqlVisualizationAttrTypes(), obj), diags
		}
		obj["line_style_options"] = flattenLineStyleOptions(vis.LineStyleOptions)
		obj["smoothing"] = types.BoolPointerValue(vis.Smoothing)
		return types.ObjectValueMust(resource_dashboard.TimeseriesPromqlVisualizationAttrTypes(), obj), diags
	case "pie-promql":
		obj["legend_mode"] = stringValueOrNull(vis.LegendMode)
		return types.ObjectValueMust(resource_dashboard.PiePromqlVisualizationAttrTypes(), obj), diags
	case "query-value-promql":
		conds, cDiags := flattenConditions(vis.Conditions)
		diags.Append(cDiags...)
		obj["legend_mode"] = stringValueOrNull(vis.LegendMode)
		obj["background_mode"] = stringValueOrNull(vis.BackgroundMode)
		obj["conditions"] = conds
		return types.ObjectValueMust(resource_dashboard.QueryValuePromqlVisualizationAttrTypes(), obj), diags
	case "top-list-promql":
		conds, cDiags := flattenConditions(vis.Conditions)
		diags.Append(cDiags...)
		obj["conditions"] = conds
		return types.ObjectValueMust(resource_dashboard.TopListPromqlVisualizationAttrTypes(), obj), diags
	}
	diags.AddError("Unsupported PromQL visualization type", vis.Type)
	return types.ObjectNull(resource_dashboard.TimeseriesPromqlVisualizationAttrTypes()), diags
}

func flattenSeriesVisualization(ctx context.Context, vis dashboardVisualization) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	queries, qDiags := flattenQueries(vis.Queries)
	diags.Append(qDiags...)
	groupBy, gDiags := flattenGroupBy(ctx, vis.GroupBy)
	diags.Append(gDiags...)

	normalizerVal := flattenNormalizer(vis.Normalizer)

	visibleSeries := types.ListNull(types.BoolType)
	if len(vis.VisibleSeries) > 0 {
		v, dv := types.ListValueFrom(ctx, types.BoolType, vis.VisibleSeries)
		diags.Append(dv...)
		visibleSeries = v
	}

	precisionVal := types.Float64Null()
	if vis.Precision != nil {
		precisionVal = types.Float64Value(*vis.Precision)
	}

	obj := map[string]attr.Value{
		"type":            types.StringValue(vis.Type),
		"source":          types.StringValue(vis.Source),
		"queries":         queries,
		"formula":         stringValueOrNull(vis.Formula),
		"aliases":         flattenAliases(ctx, vis.Aliases),
		"visible_series":  visibleSeries,
		"group_by":        groupBy,
		"normalizer":      normalizerVal,
		"precision":       precisionVal,
		"y_axis_settings": flattenYAxisSettings(vis.YAxisSettings),
	}

	if vis.Type == "query-value" {
		delete(obj, "group_by")
		delete(obj, "y_axis_settings")
		condVal, cDiags := flattenConditions(vis.Conditions)
		diags.Append(cDiags...)
		obj["background_mode"] = stringValueOrNull(vis.BackgroundMode)
		obj["legend_mode"] = stringValueOrNull(vis.LegendMode)
		obj["conditions"] = condVal
		return types.ObjectValueMust(resource_dashboard.QueryValueVisualizationAttrTypes(), obj), diags
	}

	if vis.Type == "top-list" {
		delete(obj, "y_axis_settings")
		condVal, cDiags := flattenConditions(vis.Conditions)
		diags.Append(cDiags...)
		obj["conditions"] = condVal
		obj["is_stacked"] = types.BoolPointerValue(vis.IsStacked)
		obj["custom_link"] = flattenCustomLink(vis.CustomLink)
		obj["group_by_mode"] = stringValueOrNull(vis.GroupByMode)
		obj["legend_mode"] = stringValueOrNull(vis.LegendMode)
		return types.ObjectValueMust(resource_dashboard.TopListVisualizationAttrTypes(), obj), diags
	}

	if vis.Type == "bar" {
		thresholds, tDiags := flattenThresholds(vis.Thresholds)
		diags.Append(tDiags...)
		obj["time_bucket"] = flattenTimeBucket(vis.TimeBucket)
		obj["legend_mode"] = stringValueOrNull(vis.LegendMode)
		obj["custom_link"] = flattenCustomLink(vis.CustomLink)
		obj["group_by_mode"] = stringValueOrNull(vis.GroupByMode)
		obj["thresholds"] = thresholds
		return types.ObjectValueMust(resource_dashboard.BarVisualizationAttrTypes(), obj), diags
	}

	if vis.Type == "gauge" {
		delete(obj, "group_by")
		delete(obj, "y_axis_settings")
		maxVal := types.Float64Null()
		if vis.Max != nil {
			maxVal = types.Float64Value(*vis.Max)
		}
		obj["max"] = maxVal
		cts, ctDiags := flattenGaugeColorThresholds(vis.ColorThresholds)
		diags.Append(ctDiags...)
		obj["color_thresholds"] = cts
		return types.ObjectValueMust(resource_dashboard.GaugeVisualizationAttrTypes(), obj), diags
	}

	if vis.Type == "distribution" {
		delete(obj, "y_axis_settings")
		obj["bounds_scale"] = stringValueOrNull(vis.BoundsScale)
		markers, mDiags := flattenIntList(vis.Markers)
		diags.Append(mDiags...)
		obj["percentile_markers"] = markers
		obj["group"] = stringValueOrNull(vis.Group)
		return types.ObjectValueMust(resource_dashboard.DistributionVisualizationAttrTypes(), obj), diags
	}

	if vis.Type == "heatmap" {
		obj["palette"] = stringValueOrNull(vis.Palette)
		obj["group"] = stringValueOrNull(vis.Group)
		return types.ObjectValueMust(resource_dashboard.HeatmapVisualizationAttrTypes(), obj), diags
	}

	if vis.Type == "timeseries" {
		obj["time_bucket"] = flattenTimeBucket(vis.TimeBucket)
		smoothingVal := types.BoolNull()
		if vis.Smoothing != nil {
			smoothingVal = types.BoolValue(*vis.Smoothing)
		}
		thresholds, tDiags := flattenThresholds(vis.Thresholds)
		diags.Append(tDiags...)
		obj["smoothing"] = smoothingVal
		obj["legend_mode"] = stringValueOrNull(vis.LegendMode)
		obj["custom_link"] = flattenCustomLink(vis.CustomLink)
		obj["group_by_mode"] = stringValueOrNull(vis.GroupByMode)
		obj["line_style_options"] = flattenLineStyleOptions(vis.LineStyleOptions)
		obj["thresholds"] = thresholds
		return types.ObjectValueMust(resource_dashboard.TimeseriesVisualizationAttrTypes(), obj), diags
	}

	if vis.Type == "pie" {
		delete(obj, "y_axis_settings")
		obj["legend_mode"] = stringValueOrNull(vis.LegendMode)
		obj["custom_link"] = flattenCustomLink(vis.CustomLink)
		obj["group_by_mode"] = stringValueOrNull(vis.GroupByMode)
		return types.ObjectValueMust(resource_dashboard.PieVisualizationAttrTypes(), obj), diags
	}
	diags.AddError("Unsupported series visualization type", vis.Type)
	return types.ObjectNull(resource_dashboard.PieVisualizationAttrTypes()), diags
}

func flattenTimeBucket(tb *dashboardTimeBucket) attr.Value {
	if tb == nil {
		return types.ObjectNull(resource_dashboard.TimeBucketAttrTypes())
	}
	return types.ObjectValueMust(resource_dashboard.TimeBucketAttrTypes(), map[string]attr.Value{
		"time":   types.Float64Value(tb.Time),
		"metric": types.StringValue(tb.Metric),
	})
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

		fillVal := types.ObjectNull(resource_dashboard.QueryFillAttrTypes())
		if q.Fill != nil {
			fillVal = types.ObjectValueMust(resource_dashboard.QueryFillAttrTypes(), map[string]attr.Value{
				"mode": types.ObjectValueMust(resource_dashboard.QueryFillModeAttrTypes(), map[string]attr.Value{
					"type": types.StringValue(q.Fill.Mode.Type),
				}),
			})
		}

		values = append(values, types.ObjectValueMust(resource_dashboard.QueryAttrTypes(), map[string]attr.Value{
			"aggregate":      aggVal,
			"filter":         stringValueOrNull(q.Filter),
			"functions":      funcVal,
			"time_aggregate": stringValueOrNull(q.TimeAggregate),
			"metric_kind":    stringValueOrNull(q.MetricKind),
			"fill":           fillVal,
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
		countField := types.StringNull()
		if agg.Field != "" {
			countField = types.StringValue(agg.Field)
		}
		vals["count"] = types.ObjectValueMust(aggregate.CountAttrTypes(), map[string]attr.Value{
			"field": countField,
		})
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
		secondsVal := types.Int64Null()
		if f.Seconds != nil {
			secondsVal = types.Int64Value(*f.Seconds)
		}
		baseVal := types.Int64Null()
		if f.Base != nil {
			baseVal = types.Int64Value(*f.Base)
		}
		exponentVal := types.Int64Null()
		if f.Exponent != nil {
			exponentVal = types.Int64Value(*f.Exponent)
		}
		values = append(values, types.ObjectValueMust(resource_dashboard.FunctionAttrTypes(), map[string]attr.Value{
			"type":     types.StringValue(f.Type),
			"window":   stringValueOrNull(f.Window),
			"seconds":  secondsVal,
			"base":     baseVal,
			"exponent": exponentVal,
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
		sortOrder := types.StringNull()
		if gb.SortOrder != "" {
			sortOrder = types.StringValue(gb.SortOrder)
		}
		replaceNullWith := types.StringNull()
		if gb.ReplaceNullWith != "" {
			replaceNullWith = types.StringValue(gb.ReplaceNullWith)
		}
		values = append(values, types.ObjectValueMust(groupby.AttrTypes(), map[string]attr.Value{
			"fields":            fields,
			"limit":             types.Int64Value(gb.Limit),
			"sort_order":        sortOrder,
			"replace_null_with": replaceNullWith,
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
		normVal := flattenNormalizer(c.Normalizer)
		values = append(values, types.ObjectValueMust(resource_dashboard.ListColumnAttrTypes(), map[string]attr.Value{
			"attribute":  types.StringValue(c.Attribute),
			"normalizer": normVal,
		}))
	}
	return types.ListValue(elemType, values)
}

func expandListDefaultSorting(ctx context.Context, sorting types.List) ([]dashboardListSorting, diag.Diagnostics) {
	var diags diag.Diagnostics
	if sorting.IsNull() || sorting.IsUnknown() {
		return nil, diags
	}

	var models []resource_dashboard.ListDefaultSortingModel
	diags.Append(sorting.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]dashboardListSorting, 0, len(models))
	for _, s := range models {
		result = append(result, dashboardListSorting{
			Id:   s.Id.ValueString(),
			Desc: s.Desc.ValueBool(),
		})
	}
	return result, diags
}

func flattenListDefaultSorting(sorting []dashboardListSorting) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.ListDefaultSortingAttrTypes()}
	if len(sorting) == 0 {
		return types.ListNull(elemType), nil
	}
	values := make([]attr.Value, 0, len(sorting))
	for _, s := range sorting {
		values = append(values, types.ObjectValueMust(resource_dashboard.ListDefaultSortingAttrTypes(), map[string]attr.Value{
			"id":   types.StringValue(s.Id),
			"desc": types.BoolValue(s.Desc),
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
		query := dashboardQuery{
			Aggregate:     agg,
			Filter:        q.Filter.ValueString(),
			Functions:     fns,
			TimeAggregate: q.TimeAggregate.ValueString(),
			MetricKind:    q.MetricKind.ValueString(),
		}
		if q.Fill != nil {
			query.Fill = &dashboardQueryFill{}
			query.Fill.Mode.Type = q.Fill.Mode.Type.ValueString()
		}
		result = append(result, query)
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
		if !agg.Count.Field.IsNull() && !agg.Count.Field.IsUnknown() {
			res.Field = agg.Count.Field.ValueString()
		}
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
		if !f.Seconds.IsNull() && !f.Seconds.IsUnknown() {
			s := f.Seconds.ValueInt64()
			fn.Seconds = &s
		}
		if !f.Base.IsNull() && !f.Base.IsUnknown() {
			b := f.Base.ValueInt64()
			fn.Base = &b
		}
		if !f.Exponent.IsNull() && !f.Exponent.IsUnknown() {
			e := f.Exponent.ValueInt64()
			fn.Exponent = &e
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
			Fields:          fields,
			Limit:           g.Limit.ValueInt64(),
			SortOrder:       g.SortOrder.ValueString(),
			ReplaceNullWith: g.ReplaceNullWith.ValueString(),
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

func expandTableColumns(ctx context.Context, columns types.List) ([]dashboardTableColumn, diag.Diagnostics) {
	var diags diag.Diagnostics
	if columns.IsNull() || columns.IsUnknown() {
		return nil, diags
	}
	var colModels []resource_dashboard.TableColumnModel
	diags.Append(columns.ElementsAs(ctx, &colModels, false)...)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardTableColumn, 0, len(colModels))
	for _, c := range colModels {
		queries, qDiags := expandQueries(ctx, c.Queries)
		diags.Append(qDiags...)
		if diags.HasError() {
			return nil, diags
		}

		col := dashboardTableColumn{
			Name:    c.Name.ValueString(),
			Source:  c.Source.ValueString(),
			Queries: queries,
			Formula: c.Formula.ValueString(),
		}

		if !c.VisibleSeries.IsNull() && !c.VisibleSeries.IsUnknown() {
			visible, bDiags := expandBoolList(ctx, c.VisibleSeries)
			diags.Append(bDiags...)
			col.VisibleSeries = visible
		}

		col.Normalizer = expandNormalizer(c.Normalizer)

		if !c.Precision.IsNull() && !c.Precision.IsUnknown() {
			p := c.Precision.ValueFloat64()
			col.Precision = &p
		}

		col.Aliases = expandAliases(c.Aliases)

		result = append(result, col)
	}
	return result, diags
}

func expandAliases(aliases *resource_dashboard.AliasesModel) *dashboardAliases {
	if aliases == nil {
		return nil
	}
	result := &dashboardAliases{}
	if !aliases.Formula.IsNull() && !aliases.Formula.IsUnknown() {
		result.Formula = aliases.Formula.ValueString()
	}
	if !aliases.Queries.IsNull() && !aliases.Queries.IsUnknown() {
		qMap := make(map[string]string)
		for k, v := range aliases.Queries.Elements() {
			qMap[k] = v.(types.String).ValueString()
		}
		result.Queries = qMap
	}
	return result
}

func expandNormalizer(n *normalizer.Model) *dashboardNormalizer {
	if n == nil {
		return nil
	}
	result := &dashboardNormalizer{
		Type: n.Type.ValueString(),
	}
	if !n.Unit.IsNull() && !n.Unit.IsUnknown() {
		result.Unit = n.Unit.ValueString()
	}
	return result
}

func flattenNormalizer(n *dashboardNormalizer) attr.Value {
	if n == nil {
		return types.ObjectNull(normalizer.AttrTypes())
	}
	return types.ObjectValueMust(normalizer.AttrTypes(), map[string]attr.Value{
		"type": types.StringValue(n.Type),
		"unit": stringValueOrNull(n.Unit),
	})
}

func flattenAliases(_ context.Context, aliases *dashboardAliases) attr.Value {
	if aliases == nil {
		return types.ObjectNull(resource_dashboard.AliasesAttrTypes())
	}
	queriesVal := types.MapNull(types.StringType)
	if len(aliases.Queries) > 0 {
		qMap := make(map[string]attr.Value)
		for k, v := range aliases.Queries {
			qMap[k] = types.StringValue(v)
		}
		queriesVal = types.MapValueMust(types.StringType, qMap)
	}
	return types.ObjectValueMust(resource_dashboard.AliasesAttrTypes(), map[string]attr.Value{
		"formula": stringValueOrNull(aliases.Formula),
		"queries": queriesVal,
	})
}

func flattenYAxisSettings(yas *dashboardYAxisSettings) attr.Value {
	if yas == nil {
		return types.ObjectNull(resource_dashboard.YAxisSettingsAttrTypes())
	}
	flattenBound := func(b dashboardYAxisBound) attr.Value {
		valAttr := types.Float64Null()
		if b.Value != nil {
			valAttr = types.Float64Value(*b.Value)
		}
		return types.ObjectValueMust(resource_dashboard.YAxisBoundAttrTypes(), map[string]attr.Value{
			"type":  types.StringValue(b.Type),
			"value": valAttr,
		})
	}
	expAttr := types.Float64Null()
	if yas.Scale.Exponent != nil {
		expAttr = types.Float64Value(*yas.Scale.Exponent)
	}
	return types.ObjectValueMust(resource_dashboard.YAxisSettingsAttrTypes(), map[string]attr.Value{
		"min": flattenBound(yas.Min),
		"max": flattenBound(yas.Max),
		"scale": types.ObjectValueMust(resource_dashboard.YAxisScaleAttrTypes(), map[string]attr.Value{
			"type":     types.StringValue(yas.Scale.Type),
			"exponent": expAttr,
		}),
		"always_include_zero": types.BoolValue(yas.AlwaysIncludeZero),
	})
}

func flattenTableColumns(ctx context.Context, cols []dashboardTableColumn) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.TableColumnAttrTypes()}
	if len(cols) == 0 {
		return types.ListNull(elemType), nil
	}

	var diags diag.Diagnostics
	values := make([]attr.Value, 0, len(cols))
	for _, c := range cols {
		queries, qDiags := flattenQueries(c.Queries)
		diags.Append(qDiags...)

		normVal := flattenNormalizer(c.Normalizer)

		visibleSeries := types.ListNull(types.BoolType)
		if len(c.VisibleSeries) > 0 {
			v, dv := types.ListValueFrom(ctx, types.BoolType, c.VisibleSeries)
			diags.Append(dv...)
			visibleSeries = v
		}

		precisionVal := types.Float64Null()
		if c.Precision != nil {
			precisionVal = types.Float64Value(*c.Precision)
		}

		values = append(values, types.ObjectValueMust(resource_dashboard.TableColumnAttrTypes(), map[string]attr.Value{
			"name":           types.StringValue(c.Name),
			"source":         types.StringValue(c.Source),
			"queries":        queries,
			"formula":        stringValueOrNull(c.Formula),
			"aliases":        flattenAliases(ctx, c.Aliases),
			"visible_series": visibleSeries,
			"normalizer":     normVal,
			"precision":      precisionVal,
		}))
	}
	return types.ListValue(elemType, values)
}

func expandYAxisSettings(yas *resource_dashboard.YAxisSettingsModel) *dashboardYAxisSettings {
	if yas == nil {
		return nil
	}
	result := &dashboardYAxisSettings{
		AlwaysIncludeZero: yas.AlwaysIncludeZero.ValueBool(),
		Min: dashboardYAxisBound{
			Type: yas.Min.Type.ValueString(),
		},
		Max: dashboardYAxisBound{
			Type: yas.Max.Type.ValueString(),
		},
		Scale: dashboardYAxisScale{
			Type: yas.Scale.Type.ValueString(),
		},
	}
	if !yas.Min.Value.IsNull() && !yas.Min.Value.IsUnknown() {
		v := yas.Min.Value.ValueFloat64()
		result.Min.Value = &v
	}
	if !yas.Max.Value.IsNull() && !yas.Max.Value.IsUnknown() {
		v := yas.Max.Value.ValueFloat64()
		result.Max.Value = &v
	}
	if !yas.Scale.Exponent.IsNull() && !yas.Scale.Exponent.IsUnknown() {
		v := yas.Scale.Exponent.ValueFloat64()
		result.Scale.Exponent = &v
	}
	return result
}

func expandThresholds(ctx context.Context, thresholds types.List) ([]dashboardThreshold, diag.Diagnostics) {
	var diags diag.Diagnostics
	if thresholds.IsNull() || thresholds.IsUnknown() {
		return nil, diags
	}
	var models []resource_dashboard.ThresholdModel
	diags.Append(thresholds.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardThreshold, 0, len(models))
	for _, t := range models {
		result = append(result, dashboardThreshold{
			Value: t.Value.ValueFloat64(),
			Level: t.Level.ValueString(),
		})
	}
	return result, diags
}

func expandGaugeColorThresholds(ctx context.Context, list types.List) ([]dashboardGaugeColor, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}
	var models []resource_dashboard.GaugeColorThresholdModel
	diags.Append(list.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return nil, diags
	}
	result := make([]dashboardGaugeColor, 0, len(models))
	for _, m := range models {
		result = append(result, dashboardGaugeColor{
			From:  m.From.ValueFloat64(),
			Color: m.Color.ValueString(),
		})
	}
	return result, diags
}

func flattenGaugeColorThresholds(cts []dashboardGaugeColor) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.GaugeColorThresholdAttrTypes()}
	if len(cts) == 0 {
		return types.ListNull(elemType), nil
	}
	values := make([]attr.Value, 0, len(cts))
	for _, c := range cts {
		values = append(values, types.ObjectValueMust(resource_dashboard.GaugeColorThresholdAttrTypes(), map[string]attr.Value{
			"from":  types.Float64Value(c.From),
			"color": types.StringValue(c.Color),
		}))
	}
	return types.ListValue(elemType, values)
}

func flattenIntList(s []int64) (types.List, diag.Diagnostics) {
	if len(s) == 0 {
		return types.ListNull(types.Int64Type), nil
	}
	values := make([]attr.Value, 0, len(s))
	for _, v := range s {
		values = append(values, types.Int64Value(v))
	}
	return types.ListValue(types.Int64Type, values)
}

func flattenThresholds(thresholds []dashboardThreshold) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.ThresholdAttrTypes()}
	if len(thresholds) == 0 {
		return types.ListNull(elemType), nil
	}
	values := make([]attr.Value, 0, len(thresholds))
	for _, t := range thresholds {
		values = append(values, types.ObjectValueMust(resource_dashboard.ThresholdAttrTypes(), map[string]attr.Value{
			"value": types.Float64Value(t.Value),
			"level": types.StringValue(t.Level),
		}))
	}
	return types.ListValue(elemType, values)
}

func expandListColumnsSize(m types.Map) map[string]float64 {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	result := make(map[string]float64, len(m.Elements()))
	for k, v := range m.Elements() {
		result[k] = v.(types.Float64).ValueFloat64()
	}
	return result
}

func flattenListColumnsSize(m map[string]float64) types.Map {
	if len(m) == 0 {
		return types.MapNull(types.Float64Type)
	}
	vals := make(map[string]attr.Value, len(m))
	for k, v := range m {
		vals[k] = types.Float64Value(v)
	}
	return types.MapValueMust(types.Float64Type, vals)
}

func flattenStringList(s []string) (types.List, diag.Diagnostics) {
	if len(s) == 0 {
		return types.ListNull(types.StringType), nil
	}
	values := make([]attr.Value, 0, len(s))
	for _, v := range s {
		values = append(values, types.StringValue(v))
	}
	return types.ListValue(types.StringType, values)
}

func expandCustomLink(link *resource_dashboard.CustomLinkModel) *dashboardCustomLink {
	if link == nil {
		return nil
	}
	return &dashboardCustomLink{Label: stringValue(link.Label), Url: link.Url.ValueString()}
}

func flattenCustomLink(link *dashboardCustomLink) attr.Value {
	if link == nil {
		return types.ObjectNull(resource_dashboard.CustomLinkAttrTypes())
	}
	return types.ObjectValueMust(resource_dashboard.CustomLinkAttrTypes(), map[string]attr.Value{
		"label": stringValueOrNull(link.Label),
		"url":   types.StringValue(link.Url),
	})
}

func expandLineStyleOptions(ctx context.Context, m types.Map) (map[string]dashboardLineStyle, diag.Diagnostics) {
	var diags diag.Diagnostics
	if m.IsNull() || m.IsUnknown() {
		return nil, diags
	}
	var models map[string]resource_dashboard.LineStyleOptionModel
	diags.Append(m.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		return nil, diags
	}
	result := make(map[string]dashboardLineStyle, len(models))
	for k, v := range models {
		result[k] = dashboardLineStyle{GroupId: v.GroupId.ValueString(), LineStyle: v.LineStyle.ValueString()}
	}
	return result, diags
}

func flattenLineStyleOptions(m map[string]dashboardLineStyle) types.Map {
	elemType := types.ObjectType{AttrTypes: resource_dashboard.LineStyleOptionAttrTypes()}
	if len(m) == 0 {
		return types.MapNull(elemType)
	}
	vals := make(map[string]attr.Value, len(m))
	for k, v := range m {
		vals[k] = types.ObjectValueMust(elemType.AttrTypes, map[string]attr.Value{
			"group_id":   types.StringValue(v.GroupId),
			"line_style": types.StringValue(v.LineStyle),
		})
	}
	return types.MapValueMust(elemType, vals)
}

func expandSloFilters(ctx context.Context, f *resource_dashboard.SloFiltersModel) (*dashboardSloFilters, diag.Diagnostics) {
	if f == nil {
		return nil, nil
	}
	var diags, d diag.Diagnostics
	result := &dashboardSloFilters{}
	result.Owners, d = expandSloFilter[string](ctx, f.Owners)
	diags.Append(d...)
	result.Statuses, d = expandSloFilter[string](ctx, f.Statuses)
	diags.Append(d...)
	result.Tags, d = expandSloFilter[dashboardTag](ctx, f.Tags)
	diags.Append(d...)
	return result, diags
}

func expandSloFilter[T any](ctx context.Context, f *resource_dashboard.SloFilterModel) (*dashboardSloFilter[T], diag.Diagnostics) {
	if f == nil {
		return nil, nil
	}
	var diags diag.Diagnostics
	values := []T{}
	if !f.Values.IsNull() && !f.Values.IsUnknown() {
		diags.Append(f.Values.ElementsAs(ctx, &values, false)...)
	}
	return &dashboardSloFilter[T]{Values: values, Exclude: f.Exclude.ValueBoolPointer()}, diags
}

func flattenSloFilters(ctx context.Context, f *dashboardSloFilters) (attr.Value, diag.Diagnostics) {
	attrTypes := resource_dashboard.SloFiltersAttrTypes()
	if f == nil {
		return types.ObjectNull(attrTypes), nil
	}
	var diags diag.Diagnostics
	owners, d := flattenSloFilter(ctx, f.Owners, types.StringType)
	diags.Append(d...)
	statuses, d := flattenSloFilter(ctx, f.Statuses, types.StringType)
	diags.Append(d...)
	tags, d := flattenSloFilter(ctx, f.Tags, types.ObjectType{AttrTypes: resource_dashboard.SloTagAttrTypes()})
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(attrTypes), diags
	}
	return types.ObjectValueMust(attrTypes, map[string]attr.Value{
		"owners":   owners,
		"statuses": statuses,
		"tags":     tags,
	}), diags
}

func expandSloOrganizeBy(o *resource_dashboard.SloOrganizeByModel) *dashboardSloOrganizeBy {
	if o == nil {
		return nil
	}
	return &dashboardSloOrganizeBy{Type: o.Type.ValueString(), Key: stringValue(o.Key)}
}

func flattenSloOrganizeBy(o *dashboardSloOrganizeBy) attr.Value {
	attrTypes := resource_dashboard.SloOrganizeByAttrTypes()
	if o == nil {
		return types.ObjectNull(attrTypes)
	}
	return types.ObjectValueMust(attrTypes, map[string]attr.Value{
		"type": types.StringValue(o.Type),
		"key":  stringValueOrNull(o.Key),
	})
}

func flattenSloFilter[T any](ctx context.Context, f *dashboardSloFilter[T], elemType attr.Type) (attr.Value, diag.Diagnostics) {
	attrTypes := resource_dashboard.SloFilterAttrTypes(elemType)
	if f == nil {
		return types.ObjectNull(attrTypes), nil
	}
	values := f.Values
	if values == nil {
		values = []T{}
	}
	list, diags := types.ListValueFrom(ctx, elemType, values)
	if diags.HasError() {
		return types.ObjectNull(attrTypes), diags
	}
	return types.ObjectValueMust(attrTypes, map[string]attr.Value{
		"values":  list,
		"exclude": types.BoolPointerValue(f.Exclude),
	}), diags
}
