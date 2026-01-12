package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/aggregate"
	"terraform-provider-tsuga/internal/groupby"
	"terraform-provider-tsuga/internal/resource_monitor"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*monitorResource)(nil)
var _ resource.ResourceWithConfigure = (*monitorResource)(nil)
var _ resource.ResourceWithImportState = (*monitorResource)(nil)
var _ resource.ResourceWithValidateConfig = (*monitorResource)(nil)

// anomalyConditionTypePlaceholder is sent to the API when creating/updating anomaly monitors.
// The API computes and returns the actual condition type (rate, error, cpu, general).
const anomalyConditionTypePlaceholder = "to_be_set"

func NewMonitorResource() resource.Resource {
	return &monitorResource{}
}

type monitorResource struct {
	client *TsugaClient
}

func (r *monitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *monitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *monitorResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_monitor.MonitorResourceSchema(ctx)
}

func (r *monitorResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_monitor.MonitorModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate configuration: exactly one configuration type must be set
	setCount := 0
	if config.Configuration.Metric != nil {
		setCount++
	}
	if config.Configuration.Log != nil {
		setCount++
	}
	if config.Configuration.AnomalyMetric != nil {
		setCount++
	}
	if config.Configuration.AnomalyLog != nil {
		setCount++
	}

	if setCount != 1 {
		resp.Diagnostics.AddError(
			"Invalid configuration",
			"Exactly one of metric, log, anomaly_metric, or anomaly_log must be set in configuration",
		)
		return
	}

	var diags diag.Diagnostics
	// Validate proportion_alert_threshold is set when aggregation_alert_logic is "proportion"
	if config.Configuration.Metric != nil {
		diags.Append(r.validateProportionAlertConfig(config.Configuration.Metric.AggregationAlertLogic, config.Configuration.Metric.ProportionAlertThreshold, "configuration.metric")...)
		diags.Append(r.validateMetricQueries(ctx, config.Configuration.Metric.Queries, "configuration.metric.queries")...)
	}
	if config.Configuration.Log != nil {
		diags.Append(r.validateProportionAlertConfig(config.Configuration.Log.AggregationAlertLogic, config.Configuration.Log.ProportionAlertThreshold, "configuration.log")...)
		diags.Append(r.validateLogQueries(ctx, config.Configuration.Log.Queries, "configuration.log.queries")...)
	}
	if config.Configuration.AnomalyMetric != nil {
		diags.Append(r.validateProportionAlertConfig(config.Configuration.AnomalyMetric.AggregationAlertLogic, config.Configuration.AnomalyMetric.ProportionAlertThreshold, "configuration.anomaly_metric")...)
		diags.Append(r.validateMetricQueries(ctx, config.Configuration.AnomalyMetric.Queries, "configuration.anomaly_metric.queries")...)
	}
	if config.Configuration.AnomalyLog != nil {
		diags.Append(r.validateProportionAlertConfig(config.Configuration.AnomalyLog.AggregationAlertLogic, config.Configuration.AnomalyLog.ProportionAlertThreshold, "configuration.anomaly_log")...)
		diags.Append(r.validateLogQueries(ctx, config.Configuration.AnomalyLog.Queries, "configuration.anomaly_log.queries")...)
	}
	resp.Diagnostics.Append(diags...)
}

func (r *monitorResource) validateProportionAlertConfig(aggregationAlertLogic types.String, proportionAlertThreshold types.Int64, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	if !aggregationAlertLogic.IsNull() && !aggregationAlertLogic.IsUnknown() {
		if aggregationAlertLogic.ValueString() == "proportion" {
			if proportionAlertThreshold.IsNull() || proportionAlertThreshold.IsUnknown() {
				diags.AddError(
					"Invalid configuration",
					fmt.Sprintf("%s.proportion_alert_threshold is required when aggregation_alert_logic is 'proportion'", pathPrefix),
				)
			}
		}
	}

	return diags
}

func (r *monitorResource) validateMetricQueries(ctx context.Context, queries types.List, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	if queries.IsNull() || queries.IsUnknown() {
		return diags
	}

	var queryModels []resource_monitor.MonitorQueryModel
	diags.Append(queries.ElementsAs(ctx, &queryModels, false)...)
	if diags.HasError() {
		return diags
	}

	for i, query := range queryModels {
		aggDiags := r.validateMetricAggregate(query.Aggregate, fmt.Sprintf("%s[%d].aggregate", pathPrefix, i))
		diags.Append(aggDiags...)
	}

	return diags
}

func (r *monitorResource) validateLogQueries(ctx context.Context, queries types.List, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	if queries.IsNull() || queries.IsUnknown() {
		return diags
	}

	var queryModels []resource_monitor.MonitorQueryModel
	diags.Append(queries.ElementsAs(ctx, &queryModels, false)...)
	if diags.HasError() {
		return diags
	}

	for i, query := range queryModels {
		aggDiags := r.validateLogAggregate(query.Aggregate, fmt.Sprintf("%s[%d].aggregate", pathPrefix, i))
		diags.Append(aggDiags...)
	}

	return diags
}

func (r *monitorResource) validateMetricAggregate(agg resource_monitor.MonitorAggregateModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	setCount := 0
	if agg.Count != nil {
		setCount++
	}
	if agg.UniqueCount != nil && !agg.UniqueCount.Field.IsNull() && !agg.UniqueCount.Field.IsUnknown() {
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
	if agg.Percentile != nil && !agg.Percentile.Field.IsNull() && !agg.Percentile.Field.IsUnknown() {
		setCount++
	}

	if setCount != 1 {
		diags.AddError(
			"Invalid aggregate configuration",
			fmt.Sprintf("%s: exactly one of count, unique_count, sum, average, min, max, or percentile must be set for metric monitors.", pathPrefix),
		)
	}

	return diags
}

func (r *monitorResource) validateLogAggregate(agg resource_monitor.MonitorAggregateModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	setCount := 0
	if agg.Count != nil {
		setCount++
	}
	if agg.UniqueCount != nil && !agg.UniqueCount.Field.IsNull() && !agg.UniqueCount.Field.IsUnknown() {
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
	if agg.Percentile != nil && !agg.Percentile.Field.IsNull() && !agg.Percentile.Field.IsUnknown() {
		setCount++
	}

	if setCount != 1 {
		diags.AddError(
			"Invalid aggregate configuration",
			fmt.Sprintf("%s: exactly one of count, unique_count, sum, average, min, max, or percentile must be set for log monitors.", pathPrefix),
		)
	}

	return diags
}

func (r *monitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *monitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_monitor.MonitorModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildMonitorRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdateMonitor(ctx, http.MethodPost, "/v1/monitors", requestBody, "create")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *monitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_monitor.MonitorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/monitors/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read monitor: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read monitor: %s", err))
		return
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp monitorAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	newState, diags := flattenMonitor(ctx, apiResp.Data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *monitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_monitor.MonitorModel
	var state resource_monitor.MonitorModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := r.buildMonitorRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/monitors/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateMonitor(ctx, http.MethodPut, path, requestBody, "update")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *monitorResource) buildMonitorRequestBody(ctx context.Context, plan resource_monitor.MonitorModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	body := map[string]interface{}{
		"name":        plan.Name.ValueString(),
		"owner":       plan.Owner.ValueString(),
		"priority":    plan.Priority.ValueInt64(),
		"permissions": plan.Permissions.ValueString(),
	}

	if !plan.Message.IsNull() && !plan.Message.IsUnknown() {
		body["message"] = plan.Message.ValueString()
	}

	if !plan.DashboardId.IsNull() && !plan.DashboardId.IsUnknown() {
		body["dashboardId"] = plan.DashboardId.ValueString()
	}

	if tags, tagDiags := expandTags(ctx, plan.Tags); tagDiags.HasError() {
		diags.Append(tagDiags...)
		return nil, diags
	} else if tags != nil {
		body["tags"] = tags
	}

	config, configDiags := expandMonitorConfiguration(ctx, plan.Configuration)
	diags.Append(configDiags...)
	if diags.HasError() {
		return nil, diags
	}
	body["configuration"] = config

	return body, diags
}

func (r *monitorResource) createOrUpdateMonitor(ctx context.Context, method, path string, requestBody map[string]interface{}, operation string) (resource_monitor.MonitorModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s monitor: %s", operation, err))
		return resource_monitor.MonitorModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s monitor: %s", operation, err))
		return resource_monitor.MonitorModel{}, diags
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return resource_monitor.MonitorModel{}, diags
	}

	var apiResp monitorAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return resource_monitor.MonitorModel{}, diags
	}

	newState, flattenDiags := flattenMonitor(ctx, apiResp.Data)
	diags.Append(flattenDiags...)
	if diags.HasError() {
		return resource_monitor.MonitorModel{}, diags
	}

	return newState, diags
}

func (r *monitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_monitor.MonitorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/monitors/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete monitor: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete monitor: %s", err))
			return
		}
	}
}

// API response types
type monitorAPIResponse struct {
	Data monitorAPIData `json:"data"`
}

type monitorAPIData struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Message       string                  `json:"message"`
	Tags          []apiTag                `json:"tags"`
	Configuration monitorAPIConfiguration `json:"configuration"`
	Priority      float64                 `json:"priority"`
	Owner         string                  `json:"owner"`
	DashboardId   string                  `json:"dashboardId"`
	Permissions   string                  `json:"permissions"`
}

type monitorAPIConfiguration struct {
	Type                     string                         `json:"type"`
	Condition                monitorAPICondition            `json:"condition"`
	NoDataBehavior           string                         `json:"noDataBehavior"`
	Timeframe                float64                        `json:"timeframe"`
	GroupByFields            []monitorAPIAggregationGroupBy `json:"groupByFields"`
	AggregationAlertLogic    string                         `json:"aggregationAlertLogic"`
	ProportionAlertThreshold *float64                       `json:"proportionAlertThreshold,omitempty"`
	Queries                  []monitorAPIQuery              `json:"queries"`
}

type monitorAPICondition struct {
	Formula       string   `json:"formula,omitempty"`
	Operator      string   `json:"operator,omitempty"`
	Threshold     *float64 `json:"threshold,omitempty"`
	ConditionType string   `json:"conditionType,omitempty"`
}

type monitorAPIAggregationGroupBy struct {
	Fields []string `json:"fields"`
	Limit  float64  `json:"limit"`
}

type monitorAPIQuery struct {
	Filter    string               `json:"filter"`
	Aggregate monitorAPIAggregate  `json:"aggregate"`
	Functions []monitorAPIFunction `json:"functions,omitempty"`
	Fill      *monitorAPIFill      `json:"fill,omitempty"`
}

type monitorAPIAggregate struct {
	Type       string   `json:"type"`
	Field      string   `json:"field,omitempty"`
	Percentile *float64 `json:"percentile,omitempty"`
}

type monitorAPIFunction struct {
	Type   string  `json:"type"`
	Window *string `json:"window,omitempty"`
}

type monitorAPIFill struct {
	Mode monitorAPIFillMode `json:"mode"`
}

type monitorAPIFillMode struct {
	Type string `json:"type"`
}

// Expand functions
func expandMonitorConfiguration(ctx context.Context, config resource_monitor.MonitorConfigurationModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	if config.Metric != nil {
		return expandMonitorConfigurationMetric(ctx, config.Metric)
	}
	if config.Log != nil {
		return expandMonitorConfigurationLog(ctx, config.Log)
	}
	if config.AnomalyMetric != nil {
		return expandMonitorConfigurationAnomalyMetric(ctx, config.AnomalyMetric)
	}
	if config.AnomalyLog != nil {
		return expandMonitorConfigurationAnomalyLog(ctx, config.AnomalyLog)
	}

	diags.AddError("Invalid configuration", "No configuration type set")
	return nil, diags
}

func expandMonitorConfigurationMetric(ctx context.Context, config *resource_monitor.MonitorConfigurationDetailsModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := expandAggregationGroupBy(ctx, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := expandMetricQueries(ctx, config.Queries)
	diags.Append(qDiags...)

	if diags.HasError() {
		return nil, diags
	}

	result := map[string]interface{}{
		"type": "metric",
		"condition": map[string]interface{}{
			"formula":   config.Condition.Formula.ValueString(),
			"operator":  config.Condition.Operator.ValueString(),
			"threshold": config.Condition.Threshold.ValueFloat64(),
		},
		"noDataBehavior":        config.NoDataBehavior.ValueString(),
		"timeframe":             float64(config.Timeframe.ValueInt64()),
		"groupByFields":         groupByFields,
		"aggregationAlertLogic": config.AggregationAlertLogic.ValueString(),
		"queries":               queries,
	}

	if !config.ProportionAlertThreshold.IsNull() && !config.ProportionAlertThreshold.IsUnknown() {
		result["proportionAlertThreshold"] = float64(config.ProportionAlertThreshold.ValueInt64())
	}

	return result, diags
}

func expandMonitorConfigurationLog(ctx context.Context, config *resource_monitor.MonitorConfigurationDetailsModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := expandAggregationGroupBy(ctx, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := expandLogQueries(ctx, config.Queries)
	diags.Append(qDiags...)

	if diags.HasError() {
		return nil, diags
	}

	result := map[string]interface{}{
		"type": "log",
		"condition": map[string]interface{}{
			"formula":   config.Condition.Formula.ValueString(),
			"operator":  config.Condition.Operator.ValueString(),
			"threshold": config.Condition.Threshold.ValueFloat64(),
		},
		"noDataBehavior":        config.NoDataBehavior.ValueString(),
		"timeframe":             float64(config.Timeframe.ValueInt64()),
		"groupByFields":         groupByFields,
		"aggregationAlertLogic": config.AggregationAlertLogic.ValueString(),
		"queries":               queries,
	}

	if !config.ProportionAlertThreshold.IsNull() && !config.ProportionAlertThreshold.IsUnknown() {
		result["proportionAlertThreshold"] = float64(config.ProportionAlertThreshold.ValueInt64())
	}

	return result, diags
}

func expandMonitorConfigurationAnomalyMetric(ctx context.Context, config *resource_monitor.AnomalyMonitorConfigurationDetailsModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := expandAggregationGroupBy(ctx, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := expandMetricQueries(ctx, config.Queries)
	diags.Append(qDiags...)

	if diags.HasError() {
		return nil, diags
	}

	// For anomaly monitors, the condition only includes formula.
	// The API computes conditionType; we send the placeholder value.
	result := map[string]interface{}{
		"type": "anomaly-metric",
		"condition": map[string]interface{}{
			"formula":       config.Condition.Formula.ValueString(),
			"conditionType": anomalyConditionTypePlaceholder,
		},
		"noDataBehavior":        config.NoDataBehavior.ValueString(),
		"timeframe":             float64(config.Timeframe.ValueInt64()),
		"groupByFields":         groupByFields,
		"aggregationAlertLogic": config.AggregationAlertLogic.ValueString(),
		"queries":               queries,
	}

	if !config.ProportionAlertThreshold.IsNull() && !config.ProportionAlertThreshold.IsUnknown() {
		result["proportionAlertThreshold"] = float64(config.ProportionAlertThreshold.ValueInt64())
	}

	return result, diags
}

func expandMonitorConfigurationAnomalyLog(ctx context.Context, config *resource_monitor.AnomalyMonitorConfigurationDetailsModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := expandAggregationGroupBy(ctx, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := expandLogQueries(ctx, config.Queries)
	diags.Append(qDiags...)

	if diags.HasError() {
		return nil, diags
	}

	// For anomaly monitors, the condition only includes formula.
	// The API computes conditionType; we send the placeholder value.
	result := map[string]interface{}{
		"type": "anomaly-log",
		"condition": map[string]interface{}{
			"formula":       config.Condition.Formula.ValueString(),
			"conditionType": anomalyConditionTypePlaceholder,
		},
		"noDataBehavior":        config.NoDataBehavior.ValueString(),
		"timeframe":             float64(config.Timeframe.ValueInt64()),
		"groupByFields":         groupByFields,
		"aggregationAlertLogic": config.AggregationAlertLogic.ValueString(),
		"queries":               queries,
	}

	if !config.ProportionAlertThreshold.IsNull() && !config.ProportionAlertThreshold.IsUnknown() {
		result["proportionAlertThreshold"] = float64(config.ProportionAlertThreshold.ValueInt64())
	}

	return result, diags
}

func expandAggregationGroupBy(ctx context.Context, groupByList types.List) ([]monitorAPIAggregationGroupBy, diag.Diagnostics) {
	var diags diag.Diagnostics

	if groupByList.IsNull() || groupByList.IsUnknown() {
		return nil, diags
	}

	var groupByModels []groupby.Model
	diags.Append(groupByList.ElementsAs(ctx, &groupByModels, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]monitorAPIAggregationGroupBy, 0, len(groupByModels))
	for i, groupBy := range groupByModels {
		if groupBy.Limit.IsNull() || groupBy.Limit.IsUnknown() {
			diags.AddError("Invalid group_by_fields", fmt.Sprintf("group_by_fields[%d].limit is required", i))
			continue
		}

		fields, fDiags := expandStringList(ctx, groupBy.Fields)
		diags.Append(fDiags...)
		if diags.HasError() {
			return nil, diags
		}

		if fields == nil {
			diags.AddError("Invalid group_by_fields", fmt.Sprintf("group_by_fields[%d].fields is required", i))
			return nil, diags
		}

		result = append(result, monitorAPIAggregationGroupBy{
			Fields: fields,
			Limit:  float64(groupBy.Limit.ValueInt64()),
		})
	}

	return result, diags
}

func expandAggregationFunctions(ctx context.Context, functions types.List) ([]monitorAPIFunction, diag.Diagnostics) {
	var diags diag.Diagnostics

	if functions.IsNull() || functions.IsUnknown() {
		return nil, diags
	}

	var functionModels []resource_monitor.AggregationFunctionModel
	diags.Append(functions.ElementsAs(ctx, &functionModels, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]monitorAPIFunction, 0, len(functionModels))
	for i, fn := range functionModels {
		setCount := 0
		var apiFn monitorAPIFunction

		if fn.PerSecond != nil {
			setCount++
			apiFn.Type = "per-second"
		}
		if fn.PerMinute != nil {
			setCount++
			apiFn.Type = "per-minute"
		}
		if fn.PerHour != nil {
			setCount++
			apiFn.Type = "per-hour"
		}
		if fn.Rate != nil {
			setCount++
			apiFn.Type = "rate"
		}
		if fn.Increase != nil {
			setCount++
			apiFn.Type = "increase"
		}
		if fn.Rolling != nil {
			setCount++
			if fn.Rolling.Window.IsNull() || fn.Rolling.Window.IsUnknown() {
				diags.AddError("Invalid functions", fmt.Sprintf("functions[%d].rolling.window is required", i))
				continue
			}
			window := fn.Rolling.Window.ValueString()
			apiFn.Type = "rolling"
			apiFn.Window = &window
		}

		if setCount != 1 {
			diags.AddError("Invalid functions", fmt.Sprintf("functions[%d]: exactly one of per_second, per_minute, per_hour, rate, increase, or rolling must be set", i))
			continue
		}

		result = append(result, apiFn)
	}

	return result, diags
}

func expandAggregationFill(fill *resource_monitor.AggregationFillModel) (*monitorAPIFill, diag.Diagnostics) {
	var diags diag.Diagnostics

	if fill == nil {
		return nil, diags
	}

	if fill.Mode.Type.IsNull() || fill.Mode.Type.IsUnknown() {
		diags.AddError("Invalid fill", "fill.mode.type is required")
		return nil, diags
	}

	return &monitorAPIFill{
		Mode: monitorAPIFillMode{
			Type: fill.Mode.Type.ValueString(),
		},
	}, diags
}

func expandMetricQueries(ctx context.Context, queries types.List) ([]map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	if queries.IsNull() || queries.IsUnknown() {
		return nil, diags
	}

	var queryModels []resource_monitor.MonitorQueryModel
	diags.Append(queries.ElementsAs(ctx, &queryModels, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]map[string]interface{}, 0, len(queryModels))
	for _, q := range queryModels {
		agg, aggDiags := expandMetricAggregate(q.Aggregate)
		diags.Append(aggDiags...)
		if diags.HasError() {
			return nil, diags
		}

		functions, fDiags := expandAggregationFunctions(ctx, q.Functions)
		diags.Append(fDiags...)
		fill, fillDiags := expandAggregationFill(q.Fill)
		diags.Append(fillDiags...)

		query := map[string]interface{}{
			"filter":    q.Filter.ValueString(),
			"aggregate": agg,
		}

		if len(functions) > 0 {
			query["functions"] = functions
		}

		if fill != nil {
			query["fill"] = fill
		}

		result = append(result, query)
	}

	return result, diags
}

func expandLogQueries(ctx context.Context, queries types.List) ([]map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	if queries.IsNull() || queries.IsUnknown() {
		return nil, diags
	}

	var queryModels []resource_monitor.MonitorQueryModel
	diags.Append(queries.ElementsAs(ctx, &queryModels, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]map[string]interface{}, 0, len(queryModels))
	for _, q := range queryModels {
		agg, aggDiags := expandLogAggregate(q.Aggregate)
		diags.Append(aggDiags...)
		if diags.HasError() {
			return nil, diags
		}

		functions, fDiags := expandAggregationFunctions(ctx, q.Functions)
		diags.Append(fDiags...)
		fill, fillDiags := expandAggregationFill(q.Fill)
		diags.Append(fillDiags...)

		query := map[string]interface{}{
			"filter":    q.Filter.ValueString(),
			"aggregate": agg,
		}

		if len(functions) > 0 {
			query["functions"] = functions
		}

		if fill != nil {
			query["fill"] = fill
		}

		result = append(result, query)
	}

	return result, diags
}

func expandMetricAggregate(agg resource_monitor.MonitorAggregateModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	if agg.Count != nil {
		return map[string]interface{}{"type": "count"}, diags
	}
	if agg.UniqueCount != nil && !agg.UniqueCount.Field.IsNull() && !agg.UniqueCount.Field.IsUnknown() {
		return map[string]interface{}{
			"type":  "unique-count",
			"field": agg.UniqueCount.Field.ValueString(),
		}, diags
	}
	if agg.Sum != nil && !agg.Sum.Field.IsNull() && !agg.Sum.Field.IsUnknown() {
		return map[string]interface{}{
			"type":  "sum",
			"field": agg.Sum.Field.ValueString(),
		}, diags
	}
	if agg.Average != nil && !agg.Average.Field.IsNull() && !agg.Average.Field.IsUnknown() {
		return map[string]interface{}{
			"type":  "average",
			"field": agg.Average.Field.ValueString(),
		}, diags
	}
	if agg.Min != nil && !agg.Min.Field.IsNull() && !agg.Min.Field.IsUnknown() {
		return map[string]interface{}{
			"type":  "min",
			"field": agg.Min.Field.ValueString(),
		}, diags
	}
	if agg.Max != nil && !agg.Max.Field.IsNull() && !agg.Max.Field.IsUnknown() {
		return map[string]interface{}{
			"type":  "max",
			"field": agg.Max.Field.ValueString(),
		}, diags
	}
	if agg.Percentile != nil && !agg.Percentile.Field.IsNull() && !agg.Percentile.Field.IsUnknown() {
		return map[string]interface{}{
			"type":       "percentile",
			"field":      agg.Percentile.Field.ValueString(),
			"percentile": agg.Percentile.Percentile.ValueFloat64(),
		}, diags
	}

	diags.AddError("Invalid aggregate", "No aggregate type set")
	return nil, diags
}

func expandLogAggregate(agg resource_monitor.MonitorAggregateModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	if agg.Count != nil {
		return map[string]interface{}{"type": "count"}, diags
	}
	if agg.UniqueCount != nil && !agg.UniqueCount.Field.IsNull() && !agg.UniqueCount.Field.IsUnknown() {
		return map[string]interface{}{
			"type":  "unique-count",
			"field": agg.UniqueCount.Field.ValueString(),
		}, diags
	}
	if agg.Sum != nil && !agg.Sum.Field.IsNull() && !agg.Sum.Field.IsUnknown() {
		return map[string]interface{}{
			"type":  "sum",
			"field": agg.Sum.Field.ValueString(),
		}, diags
	}
	if agg.Average != nil && !agg.Average.Field.IsNull() && !agg.Average.Field.IsUnknown() {
		return map[string]interface{}{
			"type":  "average",
			"field": agg.Average.Field.ValueString(),
		}, diags
	}
	if agg.Min != nil && !agg.Min.Field.IsNull() && !agg.Min.Field.IsUnknown() {
		return map[string]interface{}{
			"type":  "min",
			"field": agg.Min.Field.ValueString(),
		}, diags
	}
	if agg.Max != nil && !agg.Max.Field.IsNull() && !agg.Max.Field.IsUnknown() {
		return map[string]interface{}{
			"type":  "max",
			"field": agg.Max.Field.ValueString(),
		}, diags
	}
	if agg.Percentile != nil && !agg.Percentile.Field.IsNull() && !agg.Percentile.Field.IsUnknown() {
		return map[string]interface{}{
			"type":       "percentile",
			"field":      agg.Percentile.Field.ValueString(),
			"percentile": agg.Percentile.Percentile.ValueFloat64(),
		}, diags
	}

	diags.AddError("Invalid aggregate", "No aggregate type set")
	return nil, diags
}

// Flatten functions
func flattenMonitor(ctx context.Context, data monitorAPIData) (resource_monitor.MonitorModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tags, tagDiags := flattenTags(ctx, data.Tags)
	diags.Append(tagDiags...)
	config, configDiags := flattenMonitorConfiguration(ctx, data.Configuration)
	diags.Append(configDiags...)

	state := resource_monitor.MonitorModel{
		Id:            types.StringValue(data.ID),
		Name:          types.StringValue(data.Name),
		Message:       stringValueOrNull(data.Message),
		Tags:          tags,
		Configuration: config,
		Priority:      types.Int64Value(int64(data.Priority)),
		Owner:         types.StringValue(data.Owner),
		DashboardId:   stringValueOrNull(data.DashboardId),
		Permissions:   types.StringValue(data.Permissions),
	}

	return state, diags
}

func flattenMonitorConfiguration(ctx context.Context, config monitorAPIConfiguration) (resource_monitor.MonitorConfigurationModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch config.Type {
	case "metric":
		metric, d := flattenMonitorConfigurationMetric(ctx, config)
		diags.Append(d...)
		return resource_monitor.MonitorConfigurationModel{Metric: &metric}, diags
	case "log":
		log, d := flattenMonitorConfigurationLog(ctx, config)
		diags.Append(d...)
		return resource_monitor.MonitorConfigurationModel{Log: &log}, diags
	case "anomaly-metric":
		anomalyMetric, d := flattenMonitorConfigurationAnomalyMetric(ctx, config)
		diags.Append(d...)
		return resource_monitor.MonitorConfigurationModel{AnomalyMetric: &anomalyMetric}, diags
	case "anomaly-log":
		anomalyLog, d := flattenMonitorConfigurationAnomalyLog(ctx, config)
		diags.Append(d...)
		return resource_monitor.MonitorConfigurationModel{AnomalyLog: &anomalyLog}, diags
	default:
		diags.AddError("Unknown configuration type", config.Type)
		return resource_monitor.MonitorConfigurationModel{}, diags
	}
}

func flattenMonitorConfigurationMetric(ctx context.Context, config monitorAPIConfiguration) (resource_monitor.MonitorConfigurationDetailsModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := flattenAggregationGroupBy(ctx, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := flattenMetricQueries(config.Queries)
	diags.Append(qDiags...)

	condition := resource_monitor.MonitorConditionModel{
		Formula:   types.StringValue(config.Condition.Formula),
		Operator:  types.StringValue(config.Condition.Operator),
		Threshold: types.Float64Null(),
	}
	if config.Condition.Threshold != nil {
		condition.Threshold = types.Float64Value(*config.Condition.Threshold)
	}

	result := resource_monitor.MonitorConfigurationDetailsModel{
		Condition:             condition,
		NoDataBehavior:        types.StringValue(config.NoDataBehavior),
		Timeframe:             types.Int64Value(int64(config.Timeframe)),
		GroupByFields:         groupByFields,
		AggregationAlertLogic: types.StringValue(config.AggregationAlertLogic),
		Queries:               queries,
	}

	if config.ProportionAlertThreshold != nil {
		result.ProportionAlertThreshold = types.Int64Value(int64(*config.ProportionAlertThreshold))
	}

	return result, diags
}

func flattenMonitorConfigurationLog(ctx context.Context, config monitorAPIConfiguration) (resource_monitor.MonitorConfigurationDetailsModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := flattenAggregationGroupBy(ctx, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := flattenLogQueries(config.Queries)
	diags.Append(qDiags...)

	condition := resource_monitor.MonitorConditionModel{
		Formula:   types.StringValue(config.Condition.Formula),
		Operator:  types.StringValue(config.Condition.Operator),
		Threshold: types.Float64Null(),
	}
	if config.Condition.Threshold != nil {
		condition.Threshold = types.Float64Value(*config.Condition.Threshold)
	}

	result := resource_monitor.MonitorConfigurationDetailsModel{
		Condition:             condition,
		NoDataBehavior:        types.StringValue(config.NoDataBehavior),
		Timeframe:             types.Int64Value(int64(config.Timeframe)),
		GroupByFields:         groupByFields,
		AggregationAlertLogic: types.StringValue(config.AggregationAlertLogic),
		Queries:               queries,
	}

	if config.ProportionAlertThreshold != nil {
		result.ProportionAlertThreshold = types.Int64Value(int64(*config.ProportionAlertThreshold))
	}

	return result, diags
}

func flattenMonitorConfigurationAnomalyMetric(ctx context.Context, config monitorAPIConfiguration) (resource_monitor.AnomalyMonitorConfigurationDetailsModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := flattenAggregationGroupBy(ctx, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := flattenMetricQueries(config.Queries)
	diags.Append(qDiags...)

	condition := resource_monitor.AnomalyConditionModel{
		Formula: types.StringValue(config.Condition.Formula),
	}

	result := resource_monitor.AnomalyMonitorConfigurationDetailsModel{
		Condition:             condition,
		NoDataBehavior:        types.StringValue(config.NoDataBehavior),
		Timeframe:             types.Int64Value(int64(config.Timeframe)),
		GroupByFields:         groupByFields,
		AggregationAlertLogic: types.StringValue(config.AggregationAlertLogic),
		Queries:               queries,
	}

	if config.ProportionAlertThreshold != nil {
		result.ProportionAlertThreshold = types.Int64Value(int64(*config.ProportionAlertThreshold))
	}

	return result, diags
}

func flattenMonitorConfigurationAnomalyLog(ctx context.Context, config monitorAPIConfiguration) (resource_monitor.AnomalyMonitorConfigurationDetailsModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := flattenAggregationGroupBy(ctx, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := flattenLogQueries(config.Queries)
	diags.Append(qDiags...)

	condition := resource_monitor.AnomalyConditionModel{
		Formula: types.StringValue(config.Condition.Formula),
	}

	result := resource_monitor.AnomalyMonitorConfigurationDetailsModel{
		Condition:             condition,
		NoDataBehavior:        types.StringValue(config.NoDataBehavior),
		Timeframe:             types.Int64Value(int64(config.Timeframe)),
		GroupByFields:         groupByFields,
		AggregationAlertLogic: types.StringValue(config.AggregationAlertLogic),
		Queries:               queries,
	}

	if config.ProportionAlertThreshold != nil {
		result.ProportionAlertThreshold = types.Int64Value(int64(*config.ProportionAlertThreshold))
	}

	return result, diags
}

func flattenAggregationGroupBy(ctx context.Context, groupBy []monitorAPIAggregationGroupBy) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: groupby.AttrTypes()}
	var diags diag.Diagnostics

	if len(groupBy) == 0 {
		list, listDiags := types.ListValue(elemType, []attr.Value{})
		diags.Append(listDiags...)
		return list, diags
	}

	values := make([]attr.Value, 0, len(groupBy))
	for _, gb := range groupBy {
		fields, fDiags := types.ListValueFrom(ctx, types.StringType, gb.Fields)
		diags.Append(fDiags...)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}

		obj := map[string]attr.Value{
			"fields": fields,
			"limit":  types.Int64Value(int64(gb.Limit)),
		}

		values = append(values, types.ObjectValueMust(groupby.AttrTypes(), obj))
	}

	list, listDiags := types.ListValue(elemType, values)
	diags.Append(listDiags...)
	return list, diags
}

func flattenAggregationFunctions(functions []monitorAPIFunction) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_monitor.AggregationFunctionAttrTypes()}

	if len(functions) == 0 {
		return types.ListNull(elemType), nil
	}

	values := make([]attr.Value, 0, len(functions))
	for _, fn := range functions {
		perSecond := types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes())
		perMinute := types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes())
		perHour := types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes())
		rate := types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes())
		increase := types.ObjectNull(resource_monitor.AggregationFunctionEmptyAttrTypes())
		rolling := types.ObjectNull(resource_monitor.AggregationFunctionRollingAttrTypes())

		switch fn.Type {
		case "per-second":
			perSecond = types.ObjectValueMust(resource_monitor.AggregationFunctionEmptyAttrTypes(), map[string]attr.Value{})
		case "per-minute":
			perMinute = types.ObjectValueMust(resource_monitor.AggregationFunctionEmptyAttrTypes(), map[string]attr.Value{})
		case "per-hour":
			perHour = types.ObjectValueMust(resource_monitor.AggregationFunctionEmptyAttrTypes(), map[string]attr.Value{})
		case "rate":
			rate = types.ObjectValueMust(resource_monitor.AggregationFunctionEmptyAttrTypes(), map[string]attr.Value{})
		case "increase":
			increase = types.ObjectValueMust(resource_monitor.AggregationFunctionEmptyAttrTypes(), map[string]attr.Value{})
		case "rolling":
			window := types.StringNull()
			if fn.Window != nil {
				window = types.StringValue(*fn.Window)
			}
			rolling = types.ObjectValueMust(resource_monitor.AggregationFunctionRollingAttrTypes(), map[string]attr.Value{
				"window": window,
			})
		}

		values = append(values, types.ObjectValueMust(resource_monitor.AggregationFunctionAttrTypes(), map[string]attr.Value{
			"per_second": perSecond,
			"per_minute": perMinute,
			"per_hour":   perHour,
			"rate":       rate,
			"increase":   increase,
			"rolling":    rolling,
		}))
	}

	return types.ListValue(elemType, values)
}

func flattenAggregationFill(fill *monitorAPIFill) (attr.Value, diag.Diagnostics) {
	if fill == nil {
		return types.ObjectNull(resource_monitor.AggregationFillAttrTypes()), nil
	}

	return types.ObjectValue(resource_monitor.AggregationFillAttrTypes(), map[string]attr.Value{
		"mode": types.ObjectValueMust(resource_monitor.AggregationFillModeAttrTypes(), map[string]attr.Value{
			"type": types.StringValue(fill.Mode.Type),
		}),
	})
}

func flattenMetricQueries(queries []monitorAPIQuery) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_monitor.QueryAttrTypes()}
	if len(queries) == 0 {
		return types.ListNull(elemType), nil
	}

	values := make([]attr.Value, 0, len(queries))
	for _, q := range queries {
		aggVal, diags := flattenMetricAggregate(q.Aggregate)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}

		functionsVal, funcDiags := flattenAggregationFunctions(q.Functions)
		if funcDiags.HasError() {
			return types.ListNull(elemType), funcDiags
		}

		fillVal, fillDiags := flattenAggregationFill(q.Fill)
		if fillDiags.HasError() {
			return types.ListNull(elemType), fillDiags
		}

		obj := map[string]attr.Value{
			"filter":    types.StringValue(q.Filter),
			"aggregate": aggVal,
			"functions": functionsVal,
			"fill":      fillVal,
		}

		values = append(values, types.ObjectValueMust(resource_monitor.QueryAttrTypes(), obj))
	}

	return types.ListValue(elemType, values)
}

func flattenLogQueries(queries []monitorAPIQuery) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_monitor.QueryAttrTypes()}
	if len(queries) == 0 {
		return types.ListNull(elemType), nil
	}

	values := make([]attr.Value, 0, len(queries))
	for _, q := range queries {
		aggVal, diags := flattenLogAggregate(q.Aggregate)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}

		functionsVal, funcDiags := flattenAggregationFunctions(q.Functions)
		if funcDiags.HasError() {
			return types.ListNull(elemType), funcDiags
		}

		fillVal, fillDiags := flattenAggregationFill(q.Fill)
		if fillDiags.HasError() {
			return types.ListNull(elemType), fillDiags
		}

		obj := map[string]attr.Value{
			"filter":    types.StringValue(q.Filter),
			"aggregate": aggVal,
			"functions": functionsVal,
			"fill":      fillVal,
		}

		values = append(values, types.ObjectValueMust(resource_monitor.QueryAttrTypes(), obj))
	}

	return types.ListValue(elemType, values)
}

func flattenMetricAggregate(agg monitorAPIAggregate) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	nullField := types.ObjectNull(aggregate.FieldAttrTypes())
	nullPercentile := types.ObjectNull(aggregate.PercentileAttrTypes())
	nullCount := types.ObjectNull(aggregate.CountAttrTypes())

	vals := map[string]attr.Value{
		"count":        nullCount,
		"average":      nullField,
		"max":          nullField,
		"min":          nullField,
		"sum":          nullField,
		"percentile":   nullPercentile,
		"unique_count": nullField,
	}

	switch agg.Type {
	case "count":
		vals["count"] = types.ObjectValueMust(aggregate.CountAttrTypes(), map[string]attr.Value{})
	case "unique-count":
		vals["unique_count"] = types.ObjectValueMust(aggregate.FieldAttrTypes(), map[string]attr.Value{
			"field": types.StringValue(agg.Field),
		})
	case "sum", "average", "min", "max":
		key := agg.Type
		vals[key] = types.ObjectValueMust(aggregate.FieldAttrTypes(), map[string]attr.Value{
			"field": types.StringValue(agg.Field),
		})
	case "percentile":
		percentile := types.Float64Null()
		if agg.Percentile != nil {
			percentile = types.Float64Value(*agg.Percentile)
		}
		vals["percentile"] = types.ObjectValueMust(aggregate.PercentileAttrTypes(), map[string]attr.Value{
			"field":      types.StringValue(agg.Field),
			"percentile": percentile,
		})
	default:
		diags.AddWarning("Unknown aggregate type", fmt.Sprintf("Unrecognized aggregate type: %s", agg.Type))
	}

	return types.ObjectValueMust(aggregate.AttrTypes(), vals), diags
}

func flattenLogAggregate(agg monitorAPIAggregate) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	nullField := types.ObjectNull(aggregate.FieldAttrTypes())
	nullPercentile := types.ObjectNull(aggregate.PercentileAttrTypes())
	nullCount := types.ObjectNull(aggregate.CountAttrTypes())

	vals := map[string]attr.Value{
		"average":      nullField,
		"count":        nullCount,
		"max":          nullField,
		"min":          nullField,
		"sum":          nullField,
		"percentile":   nullPercentile,
		"unique_count": nullField,
	}

	switch agg.Type {
	case "count":
		vals["count"] = types.ObjectValueMust(aggregate.CountAttrTypes(), map[string]attr.Value{})
	case "unique-count":
		vals["unique_count"] = types.ObjectValueMust(aggregate.FieldAttrTypes(), map[string]attr.Value{
			"field": types.StringValue(agg.Field),
		})
	case "sum", "average", "min", "max":
		key := agg.Type
		vals[key] = types.ObjectValueMust(aggregate.FieldAttrTypes(), map[string]attr.Value{
			"field": types.StringValue(agg.Field),
		})
	case "percentile":
		percentile := types.Float64Null()
		if agg.Percentile != nil {
			percentile = types.Float64Value(*agg.Percentile)
		}
		vals["percentile"] = types.ObjectValueMust(aggregate.PercentileAttrTypes(), map[string]attr.Value{
			"field":      types.StringValue(agg.Field),
			"percentile": percentile,
		})
	default:
		diags.AddWarning("Unknown aggregate type", fmt.Sprintf("Unrecognized aggregate type: %s", agg.Type))
	}

	return types.ObjectValueMust(aggregate.AttrTypes(), vals), diags
}
