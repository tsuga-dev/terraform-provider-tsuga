package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

func NewMonitorResource() resource.Resource {
	return &monitorResource{}
}

type monitorResource struct {
	client *TsugaClient
}

func (r *monitorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *monitorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *monitorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
	if config.Configuration.AnomalyLog != nil {
		setCount++
	}
	if config.Configuration.AnomalyMetric != nil {
		setCount++
	}

	if setCount != 1 {
		resp.Diagnostics.AddError(
			"Invalid configuration",
			"Exactly one of metric, log, anomaly_log, or anomaly_metric must be set in configuration",
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
	if config.Configuration.AnomalyLog != nil {
		diags.Append(r.validateProportionAlertConfig(config.Configuration.AnomalyLog.AggregationAlertLogic, config.Configuration.AnomalyLog.ProportionAlertThreshold, "configuration.anomaly_log")...)
		diags.Append(r.validateLogQueries(ctx, config.Configuration.AnomalyLog.Queries, "configuration.anomaly_log.queries")...)
	}
	if config.Configuration.AnomalyMetric != nil {
		diags.Append(r.validateProportionAlertConfig(config.Configuration.AnomalyMetric.AggregationAlertLogic, config.Configuration.AnomalyMetric.ProportionAlertThreshold, "configuration.anomaly_metric")...)
		diags.Append(r.validateMetricQueries(ctx, config.Configuration.AnomalyMetric.Queries, "configuration.anomaly_metric.queries")...)
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

	var queryModels []resource_monitor.MetricQueryModel
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

	var queryModels []resource_monitor.LogQueryModel
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

func (r *monitorResource) validateMetricAggregate(agg resource_monitor.MetricAggregateModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	setCount := 0
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
			fmt.Sprintf("%s: exactly one of unique_count, sum, average, min, max, or percentile must be set for metric monitors.", pathPrefix),
		)
	}

	return diags
}

func (r *monitorResource) validateLogAggregate(agg resource_monitor.LogAggregateModel, pathPrefix string) diag.Diagnostics {
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
	Type                     string              `json:"type"`
	Condition                monitorAPICondition `json:"condition"`
	NoDataBehavior           string              `json:"noDataBehavior"`
	Timeframe                float64             `json:"timeframe"`
	GroupByFields            []string            `json:"groupByFields"`
	AggregationAlertLogic    string              `json:"aggregationAlertLogic"`
	ProportionAlertThreshold *float64            `json:"proportionAlertThreshold,omitempty"`
	Queries                  []monitorAPIQuery   `json:"queries"`
}

type monitorAPICondition struct {
	Formula       string   `json:"formula,omitempty"`
	Operator      string   `json:"operator,omitempty"`
	Threshold     *float64 `json:"threshold,omitempty"`
	ConditionType string   `json:"conditionType,omitempty"`
}

type monitorAPIQuery struct {
	Name          string              `json:"name"`
	Filter        string              `json:"filter"`
	Aggregate     monitorAPIAggregate `json:"aggregate"`
	ValueIfNoData string              `json:"value_if_no_data,omitempty"`
}

type monitorAPIAggregate struct {
	Type       string   `json:"type"`
	Field      string   `json:"field,omitempty"`
	Percentile *float64 `json:"percentile,omitempty"`
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
	if config.AnomalyLog != nil {
		return expandMonitorConfigurationAnomalyLog(ctx, config.AnomalyLog)
	}
	if config.AnomalyMetric != nil {
		return expandMonitorConfigurationAnomalyMetric(ctx, config.AnomalyMetric)
	}

	diags.AddError("Invalid configuration", "No configuration type set")
	return nil, diags
}

func expandMonitorConfigurationMetric(ctx context.Context, config *resource_monitor.MonitorConfigurationMetricModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := expandStringList(ctx, config.GroupByFields)
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

func expandMonitorConfigurationLog(ctx context.Context, config *resource_monitor.MonitorConfigurationLogModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := expandStringList(ctx, config.GroupByFields)
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

func expandMonitorConfigurationAnomalyLog(ctx context.Context, config *resource_monitor.MonitorConfigurationAnomalyLogModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := expandStringList(ctx, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := expandLogQueries(ctx, config.Queries)
	diags.Append(qDiags...)

	if diags.HasError() {
		return nil, diags
	}

	result := map[string]interface{}{
		"type": "anomaly-log",
		"condition": map[string]interface{}{
			"formula":       config.Condition.Formula.ValueString(),
			"conditionType": config.Condition.ConditionType.ValueString(),
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

func expandMonitorConfigurationAnomalyMetric(ctx context.Context, config *resource_monitor.MonitorConfigurationAnomalyMetricModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := expandStringList(ctx, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := expandMetricQueries(ctx, config.Queries)
	diags.Append(qDiags...)

	if diags.HasError() {
		return nil, diags
	}

	result := map[string]interface{}{
		"type": "anomaly-metric",
		"condition": map[string]interface{}{
			"formula":       config.Condition.Formula.ValueString(),
			"conditionType": config.Condition.ConditionType.ValueString(),
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

func expandMetricQueries(ctx context.Context, queries types.List) ([]map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	if queries.IsNull() || queries.IsUnknown() {
		return nil, diags
	}

	var queryModels []resource_monitor.MetricQueryModel
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

		query := map[string]interface{}{
			"name":      q.Name.ValueString(),
			"filter":    q.Filter.ValueString(),
			"aggregate": agg,
		}

		if !q.ValueIfNoData.IsNull() && !q.ValueIfNoData.IsUnknown() {
			query["value_if_no_data"] = q.ValueIfNoData.ValueString()
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

	var queryModels []resource_monitor.LogQueryModel
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

		query := map[string]interface{}{
			"name":      q.Name.ValueString(),
			"filter":    q.Filter.ValueString(),
			"aggregate": agg,
		}

		if !q.ValueIfNoData.IsNull() && !q.ValueIfNoData.IsUnknown() {
			query["value_if_no_data"] = q.ValueIfNoData.ValueString()
		}

		result = append(result, query)
	}

	return result, diags
}

func expandMetricAggregate(agg resource_monitor.MetricAggregateModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

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

func expandLogAggregate(agg resource_monitor.LogAggregateModel) (map[string]interface{}, diag.Diagnostics) {
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
	case "anomaly-log":
		anomalyLog, d := flattenMonitorConfigurationAnomalyLog(ctx, config)
		diags.Append(d...)
		return resource_monitor.MonitorConfigurationModel{AnomalyLog: &anomalyLog}, diags
	case "anomaly-metric":
		anomalyMetric, d := flattenMonitorConfigurationAnomalyMetric(ctx, config)
		diags.Append(d...)
		return resource_monitor.MonitorConfigurationModel{AnomalyMetric: &anomalyMetric}, diags
	default:
		diags.AddError("Unknown configuration type", config.Type)
		return resource_monitor.MonitorConfigurationModel{}, diags
	}
}

func flattenMonitorConfigurationMetric(ctx context.Context, config monitorAPIConfiguration) (resource_monitor.MonitorConfigurationMetricModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := types.ListValueFrom(ctx, types.StringType, config.GroupByFields)
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

	result := resource_monitor.MonitorConfigurationMetricModel{
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

func flattenMonitorConfigurationLog(ctx context.Context, config monitorAPIConfiguration) (resource_monitor.MonitorConfigurationLogModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := types.ListValueFrom(ctx, types.StringType, config.GroupByFields)
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

	result := resource_monitor.MonitorConfigurationLogModel{
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

func flattenMonitorConfigurationAnomalyLog(ctx context.Context, config monitorAPIConfiguration) (resource_monitor.MonitorConfigurationAnomalyLogModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := types.ListValueFrom(ctx, types.StringType, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := flattenLogQueries(config.Queries)
	diags.Append(qDiags...)

	condition := resource_monitor.MonitorAnomalyConditionModel{
		Formula:       types.StringValue(config.Condition.Formula),
		ConditionType: types.StringValue(config.Condition.ConditionType),
	}

	result := resource_monitor.MonitorConfigurationAnomalyLogModel{
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

func flattenMonitorConfigurationAnomalyMetric(ctx context.Context, config monitorAPIConfiguration) (resource_monitor.MonitorConfigurationAnomalyMetricModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	groupByFields, gDiags := types.ListValueFrom(ctx, types.StringType, config.GroupByFields)
	diags.Append(gDiags...)
	queries, qDiags := flattenMetricQueries(config.Queries)
	diags.Append(qDiags...)

	condition := resource_monitor.MonitorAnomalyConditionModel{
		Formula:       types.StringValue(config.Condition.Formula),
		ConditionType: types.StringValue(config.Condition.ConditionType),
	}

	result := resource_monitor.MonitorConfigurationAnomalyMetricModel{
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

func flattenMetricQueries(queries []monitorAPIQuery) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_monitor.MetricQueryAttrTypes()}
	if len(queries) == 0 {
		return types.ListNull(elemType), nil
	}

	values := make([]attr.Value, 0, len(queries))
	for _, q := range queries {
		aggVal, diags := flattenMetricAggregate(q.Aggregate)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}

		obj := map[string]attr.Value{
			"name":             types.StringValue(q.Name),
			"filter":           types.StringValue(q.Filter),
			"aggregate":        aggVal,
			"value_if_no_data": stringValueOrNull(q.ValueIfNoData),
		}

		values = append(values, types.ObjectValueMust(resource_monitor.MetricQueryAttrTypes(), obj))
	}

	return types.ListValue(elemType, values)
}

func flattenLogQueries(queries []monitorAPIQuery) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: resource_monitor.LogQueryAttrTypes()}
	if len(queries) == 0 {
		return types.ListNull(elemType), nil
	}

	values := make([]attr.Value, 0, len(queries))
	for _, q := range queries {
		aggVal, diags := flattenLogAggregate(q.Aggregate)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}

		obj := map[string]attr.Value{
			"name":             types.StringValue(q.Name),
			"filter":           types.StringValue(q.Filter),
			"aggregate":        aggVal,
			"value_if_no_data": stringValueOrNull(q.ValueIfNoData),
		}

		values = append(values, types.ObjectValueMust(resource_monitor.LogQueryAttrTypes(), obj))
	}

	return types.ListValue(elemType, values)
}

func flattenMetricAggregate(agg monitorAPIAggregate) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	nullField := types.ObjectNull(resource_monitor.AggregateFieldAttrTypes())
	nullPercentile := types.ObjectNull(resource_monitor.AggregatePercentileAttrTypes())

	vals := map[string]attr.Value{
		"average":      nullField,
		"max":          nullField,
		"min":          nullField,
		"sum":          nullField,
		"percentile":   nullPercentile,
		"unique_count": nullField,
	}

	switch agg.Type {
	case "unique-count":
		vals["unique_count"] = types.ObjectValueMust(resource_monitor.AggregateFieldAttrTypes(), map[string]attr.Value{
			"field": types.StringValue(agg.Field),
		})
	case "sum", "average", "min", "max":
		key := agg.Type
		vals[key] = types.ObjectValueMust(resource_monitor.AggregateFieldAttrTypes(), map[string]attr.Value{
			"field": types.StringValue(agg.Field),
		})
	case "percentile":
		percentile := types.Float64Null()
		if agg.Percentile != nil {
			percentile = types.Float64Value(*agg.Percentile)
		}
		vals["percentile"] = types.ObjectValueMust(resource_monitor.AggregatePercentileAttrTypes(), map[string]attr.Value{
			"field":      types.StringValue(agg.Field),
			"percentile": percentile,
		})
	default:
		diags.AddWarning("Unknown aggregate type", fmt.Sprintf("Unrecognized aggregate type: %s", agg.Type))
	}

	return types.ObjectValueMust(resource_monitor.MetricAggregateAttrTypes(), vals), diags
}

func flattenLogAggregate(agg monitorAPIAggregate) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	nullField := types.ObjectNull(resource_monitor.AggregateFieldAttrTypes())
	nullPercentile := types.ObjectNull(resource_monitor.AggregatePercentileAttrTypes())
	nullCount := types.ObjectNull(resource_monitor.AggregateCountAttrTypes())

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
		vals["count"] = types.ObjectValueMust(resource_monitor.AggregateCountAttrTypes(), map[string]attr.Value{})
	case "unique-count":
		vals["unique_count"] = types.ObjectValueMust(resource_monitor.AggregateFieldAttrTypes(), map[string]attr.Value{
			"field": types.StringValue(agg.Field),
		})
	case "sum", "average", "min", "max":
		key := agg.Type
		vals[key] = types.ObjectValueMust(resource_monitor.AggregateFieldAttrTypes(), map[string]attr.Value{
			"field": types.StringValue(agg.Field),
		})
	case "percentile":
		percentile := types.Float64Null()
		if agg.Percentile != nil {
			percentile = types.Float64Value(*agg.Percentile)
		}
		vals["percentile"] = types.ObjectValueMust(resource_monitor.AggregatePercentileAttrTypes(), map[string]attr.Value{
			"field":      types.StringValue(agg.Field),
			"percentile": percentile,
		})
	default:
		diags.AddWarning("Unknown aggregate type", fmt.Sprintf("Unrecognized aggregate type: %s", agg.Type))
	}

	return types.ObjectValueMust(resource_monitor.LogAggregateAttrTypes(), vals), diags
}
