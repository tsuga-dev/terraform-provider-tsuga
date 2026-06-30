package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"terraform-provider-tsuga/internal/groupby"
	"terraform-provider-tsuga/internal/resource_monitor"
	"terraform-provider-tsuga/internal/resource_slo"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = (*sloResource)(nil)
	_ resource.ResourceWithConfigure      = (*sloResource)(nil)
	_ resource.ResourceWithImportState    = (*sloResource)(nil)
	_ resource.ResourceWithValidateConfig = (*sloResource)(nil)
)

func NewSloResource() resource.Resource {
	return &sloResource{}
}

type sloResource struct {
	client *TsugaClient
}

func (r *sloResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sloResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_slo"
}

func (r *sloResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_slo.SloResourceSchema(ctx)
}

func (r *sloResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resource_slo.SloModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate configuration: exactly one configuration type must be set.
	setCount := 0
	if config.Configuration.Event != nil {
		setCount++
	}
	if config.Configuration.Time != nil {
		setCount++
	}
	if setCount != 1 {
		resp.Diagnostics.AddError(
			"Invalid configuration",
			"Exactly one of event or time must be set in configuration",
		)
		return
	}

	var diags diag.Diagnostics

	// tags and queries reuse shared flatten helpers that map an empty slice to null. Reject
	// empty lists here as a state-consistency guard.
	if !config.Tags.IsNull() && !config.Tags.IsUnknown() && len(config.Tags.Elements()) == 0 {
		diags.AddError(
			"Invalid tags",
			"tags must contain at least one tag when set; omit the attribute to apply no tags",
		)
	}

	if config.Configuration.Event != nil {
		diags.Append(r.validateSloQueryFormula(ctx, config.Configuration.Event.GoodQuery, "configuration.event.good_query")...)
		diags.Append(r.validateSloQueryFormula(ctx, config.Configuration.Event.TotalQuery, "configuration.event.total_query")...)
	}
	if config.Configuration.Time != nil {
		diags.Append(r.validateSloQueryFormula(ctx, config.Configuration.Time.Query, "configuration.time.query")...)
	}

	diags.Append(r.validateSloAlerts(ctx, config.Alerts)...)

	resp.Diagnostics.Append(diags...)
}

func (r *sloResource) validateSloQueryFormula(ctx context.Context, qf resource_slo.SloQueryFormulaModel, pathPrefix string) diag.Diagnostics {
	var diags diag.Diagnostics

	if qf.Queries.IsNull() || qf.Queries.IsUnknown() {
		return diags
	}

	var queryModels []resource_monitor.MonitorQueryModel
	diags.Append(qf.Queries.ElementsAs(ctx, &queryModels, false)...)
	if diags.HasError() {
		return diags
	}

	// A formula needs at least one query, and an explicit empty list would round-trip to null
	if len(queryModels) == 0 {
		diags.AddError(
			"Invalid queries",
			fmt.Sprintf("%s.queries must contain at least one query", pathPrefix),
		)
		return diags
	}

	for i, q := range queryModels {
		diags.Append(validateSloAggregate(q.Aggregate, fmt.Sprintf("%s.queries[%d].aggregate", pathPrefix, i))...)
	}

	return diags
}

func validateSloAggregate(agg resource_monitor.MonitorAggregateModel, pathPrefix string) diag.Diagnostics {
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
			fmt.Sprintf("%s: exactly one of count, unique_count, sum, average, min, max, or percentile must be set.", pathPrefix),
		)
	}

	return diags
}

func (r *sloResource) validateSloAlerts(ctx context.Context, alerts types.List) diag.Diagnostics {
	var diags diag.Diagnostics

	if alerts.IsNull() || alerts.IsUnknown() {
		return diags
	}

	var alertModels []resource_slo.SloAlertModel
	diags.Append(alerts.ElementsAs(ctx, &alertModels, false)...)
	if diags.HasError() {
		return diags
	}

	for i, a := range alertModels {
		burnSet := a.Configuration.BurnRateSet()
		thresholdSet := a.Configuration.ThresholdSet()

		count := 0
		if burnSet {
			count++
		}
		if thresholdSet {
			count++
		}
		if count != 1 {
			diags.AddError(
				"Invalid alert configuration",
				fmt.Sprintf("alerts[%d].configuration: exactly one of burn_rate or threshold must be set", i),
			)
		}
	}

	return diags
}

func (r *sloResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *sloResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_slo.SloModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// No prior alerts on create: every alert is sent without an id, so the API creates them all.
	requestBody, diags := r.buildSloRequestBody(ctx, plan, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdateSlo(ctx, http.MethodPost, "/v1/slos", requestBody, "create", sloAlertRef(ctx, plan.Alerts))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sloResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Read only the id from state. On import the state holds just the id and every other
	// attribute is null; decoding the whole SloModel here would fail because nested fields
	// such as configuration are non-pointer value structs that cannot represent null.
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The prior state's alert order (matched by id) is the reference we flatten the API's
	// unordered alerts back into, so a refresh doesn't churn the order. On import this is null,
	// so the alerts come back in API order.
	var priorAlerts types.List
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("alerts"), &priorAlerts)...)
	if resp.Diagnostics.HasError() {
		return
	}

	urlPath := fmt.Sprintf("/v1/slos/%s", id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, urlPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read SLO: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read SLO: %s", err))
		return
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp sloAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	newState, diags := flattenSlo(ctx, apiResp.Data, sloAlertRef(ctx, priorAlerts))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sloResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_slo.SloModel
	var state resource_slo.SloModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Pass the prior state's alerts so ids are matched to them by content rather than by index.
	requestBody, diags := r.buildSloRequestBody(ctx, plan, sloAlertRef(ctx, state.Alerts))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/slos/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdateSlo(ctx, http.MethodPut, path, requestBody, "update", sloAlertRef(ctx, plan.Alerts))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *sloResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_slo.SloModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/slos/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete SLO: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete SLO: %s", err))
			return
		}
	}
}

func (r *sloResource) createOrUpdateSlo(ctx context.Context, method, path string, requestBody map[string]interface{}, operation string, alertRef []resource_slo.SloAlertModel) (resource_slo.SloModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s SLO: %s", operation, err))
		return resource_slo.SloModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s SLO: %s", operation, err))
		return resource_slo.SloModel{}, diags
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return resource_slo.SloModel{}, diags
	}

	var apiResp sloAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return resource_slo.SloModel{}, diags
	}

	newState, flattenDiags := flattenSlo(ctx, apiResp.Data, alertRef)
	diags.Append(flattenDiags...)
	if diags.HasError() {
		return resource_slo.SloModel{}, diags
	}

	return newState, diags
}

// API response types

type sloAPIResponse struct {
	Data sloAPIData `json:"data"`
}

type sloAPIData struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Description   string              `json:"description"`
	Tags          []apiTag            `json:"tags"`
	Configuration sloAPIConfiguration `json:"configuration"`
	Target        float64             `json:"target"`
	TimeframeDays float64             `json:"timeframeDays"`
	Owner         string              `json:"owner"`
	Permissions   string              `json:"permissions"`
	ClusterIds    []string            `json:"clusterIds"`
	Alerts        []sloAPIAlert       `json:"alerts"`
}

type sloAPIConfiguration struct {
	Type             string                         `json:"type"`
	DataSource       string                         `json:"dataSource"`
	GoodQuery        *sloAPIQueryFormula            `json:"goodQuery,omitempty"`
	TotalQuery       *sloAPIQueryFormula            `json:"totalQuery,omitempty"`
	Query            *sloAPIQueryFormula            `json:"query,omitempty"`
	SliceSizeMinutes *float64                       `json:"sliceSizeMinutes,omitempty"`
	Threshold        *sloAPITimeThreshold           `json:"threshold,omitempty"`
	GroupByFields    []monitorAPIAggregationGroupBy `json:"groupByFields,omitempty"`
	NoDataBehavior   string                         `json:"noDataBehavior"`
}

type sloAPIQueryFormula struct {
	Queries []monitorAPIQuery `json:"queries"`
	Formula string            `json:"formula"`
}

type sloAPITimeThreshold struct {
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
}

type sloAPIAlert struct {
	ID            string                   `json:"id"`
	SloId         string                   `json:"sloId"`
	Priority      float64                  `json:"priority"`
	Configuration sloAPIAlertConfiguration `json:"configuration"`
}

type sloAPIAlertConfiguration struct {
	Type      string   `json:"type"`
	BurnRate  *float64 `json:"burnRate,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
}

// Expand functions

func (r *sloResource) buildSloRequestBody(ctx context.Context, plan resource_slo.SloModel, priorAlerts []resource_slo.SloAlertModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	body := map[string]interface{}{
		"name":          plan.Name.ValueString(),
		"owner":         plan.Owner.ValueString(),
		"permissions":   plan.Permissions.ValueString(),
		"target":        plan.Target.ValueFloat64(),
		"timeframeDays": plan.TimeframeDays.ValueInt64(),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		body["description"] = plan.Description.ValueString()
	}

	if tags, tagDiags := expandTags(ctx, plan.Tags); tagDiags.HasError() {
		diags.Append(tagDiags...)
		return nil, diags
	} else if tags != nil {
		body["tags"] = tags
	}

	clusterIds, clusterDiags := expandStringList(ctx, plan.ClusterIds)
	diags.Append(clusterDiags...)
	if clusterIds == nil {
		clusterIds = []string{}
	}
	body["clusterIds"] = clusterIds

	config, configDiags := r.expandSloConfiguration(ctx, plan.Configuration)
	diags.Append(configDiags...)
	if diags.HasError() {
		return nil, diags
	}
	body["configuration"] = config

	alerts, alertDiags := r.expandSloAlerts(ctx, plan.Alerts, priorAlerts)
	diags.Append(alertDiags...)
	if diags.HasError() {
		return nil, diags
	}
	body["alerts"] = alerts

	return body, diags
}

func (r *sloResource) expandSloConfiguration(ctx context.Context, config resource_slo.SloConfigurationModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	if config.Event != nil {
		return r.expandSloEventConfiguration(ctx, config.Event)
	}
	if config.Time != nil {
		return r.expandSloTimeConfiguration(ctx, config.Time)
	}

	diags.AddError("Invalid configuration", "No configuration type set")
	return nil, diags
}

func (r *sloResource) expandSloEventConfiguration(ctx context.Context, config *resource_slo.SloEventConfigurationModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	goodQuery, gDiags := r.expandSloQueryFormula(ctx, config.GoodQuery)
	diags.Append(gDiags...)
	totalQuery, tDiags := r.expandSloQueryFormula(ctx, config.TotalQuery)
	diags.Append(tDiags...)
	groupByFields, gbDiags := expandAggregationGroupBy(ctx, config.GroupByFields)
	diags.Append(gbDiags...)

	if diags.HasError() {
		return nil, diags
	}

	result := map[string]interface{}{
		"type":           "event",
		"dataSource":     config.DataSource.ValueString(),
		"goodQuery":      goodQuery,
		"totalQuery":     totalQuery,
		"noDataBehavior": config.NoDataBehavior.ValueString(),
	}

	if len(groupByFields) > 0 {
		result["groupByFields"] = groupByFields
	}

	return result, diags
}

func (r *sloResource) expandSloTimeConfiguration(ctx context.Context, config *resource_slo.SloTimeConfigurationModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, qDiags := r.expandSloQueryFormula(ctx, config.Query)
	diags.Append(qDiags...)
	groupByFields, gbDiags := expandAggregationGroupBy(ctx, config.GroupByFields)
	diags.Append(gbDiags...)

	if diags.HasError() {
		return nil, diags
	}

	result := map[string]interface{}{
		"type":             "time",
		"dataSource":       config.DataSource.ValueString(),
		"query":            query,
		"sliceSizeMinutes": config.SliceSizeMinutes.ValueInt64(),
		"threshold": map[string]interface{}{
			"operator": config.Threshold.Operator.ValueString(),
			"value":    config.Threshold.Value.ValueFloat64(),
		},
		"noDataBehavior": config.NoDataBehavior.ValueString(),
	}

	if len(groupByFields) > 0 {
		result["groupByFields"] = groupByFields
	}

	return result, diags
}

func (r *sloResource) expandSloQueryFormula(ctx context.Context, qf resource_slo.SloQueryFormulaModel) (map[string]interface{}, diag.Diagnostics) {
	queries, diags := expandMonitorQueries(ctx, qf.Queries)
	if diags.HasError() {
		return nil, diags
	}

	if queries == nil {
		queries = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"queries": queries,
		"formula": qf.Formula.ValueString(),
	}, diags
}

func (r *sloResource) expandSloAlerts(ctx context.Context, alerts types.List, priorAlerts []resource_slo.SloAlertModel) ([]map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	result := []map[string]interface{}{}
	if alerts.IsNull() || alerts.IsUnknown() {
		return result, diags
	}

	var alertModels []resource_slo.SloAlertModel
	diags.Append(alerts.ElementsAs(ctx, &alertModels, false)...)
	if diags.HasError() {
		return nil, diags
	}

	// Each prior alert's id may be reused at most once.
	priorUsed := make([]bool, len(priorAlerts))

	for i, a := range alertModels {
		config, cDiags := expandSloAlertConfiguration(a.Configuration, fmt.Sprintf("alerts[%d].configuration", i))
		diags.Append(cDiags...)
		if cDiags.HasError() {
			continue
		}

		alert := map[string]interface{}{
			"priority":      a.Priority.ValueInt64(),
			"configuration": config,
		}

		// Derive the id by matching the planned alert to prior state by content (priority +
		// configuration), not by list index. The API reconciles alerts by id, so this keeps an
		// alert's server identity stable across reorders/removals/insertions. A planned alert that
		// matches no prior alert (new, or a content change) is sent without an id, so the API
		// creates it and deletes any prior alert that is no longer present.
		if id := matchPriorAlertID(a, priorAlerts, priorUsed); id != "" {
			alert["id"] = id
		}

		result = append(result, alert)
	}

	return result, diags
}

// matchPriorAlertID returns the id of the first unclaimed prior alert whose content equals the
// planned alert, marking it claimed. It returns "" when there is no content match.
func matchPriorAlertID(planned resource_slo.SloAlertModel, prior []resource_slo.SloAlertModel, used []bool) string {
	for i := range prior {
		if used[i] || prior[i].Id.IsNull() || prior[i].Id.IsUnknown() {
			continue
		}
		if sloAlertContentEqual(planned, prior[i]) {
			used[i] = true
			return prior[i].Id.ValueString()
		}
	}
	return ""
}

// sloAlertContentEqual reports whether two alerts have the same priority and configuration —
// everything that identifies an alert except its server-assigned id.
func sloAlertContentEqual(a, b resource_slo.SloAlertModel) bool {
	return a.Priority.Equal(b.Priority) &&
		a.Configuration.BurnRate.Equal(b.Configuration.BurnRate) &&
		a.Configuration.Threshold.Equal(b.Configuration.Threshold)
}

func expandSloAlertConfiguration(config resource_slo.SloAlertConfigurationModel, pathPrefix string) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	burnSet := config.BurnRateSet()
	thresholdSet := config.ThresholdSet()

	switch {
	case burnSet && !thresholdSet:
		return map[string]interface{}{
			"type":     "burn-rate",
			"burnRate": config.BurnRate.ValueFloat64(),
		}, diags
	case thresholdSet && !burnSet:
		return map[string]interface{}{
			"type":      "threshold",
			"threshold": config.Threshold.ValueFloat64(),
		}, diags
	default:
		diags.AddError(
			"Invalid alert configuration",
			fmt.Sprintf("%s: exactly one of burn_rate or threshold must be set", pathPrefix),
		)
		return nil, diags
	}
}

// Flatten functions

func flattenSlo(ctx context.Context, data sloAPIData, alertRef []resource_slo.SloAlertModel) (resource_slo.SloModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tags, tagDiags := flattenTags(ctx, data.Tags)
	diags.Append(tagDiags...)
	config, configDiags := flattenSloConfiguration(ctx, data.Configuration)
	diags.Append(configDiags...)
	clusterIds, clusterDiags := types.ListValueFrom(ctx, types.StringType, data.ClusterIds)
	diags.Append(clusterDiags...)
	alerts, alertDiags := flattenSloAlerts(data.Alerts, alertRef)
	diags.Append(alertDiags...)

	state := resource_slo.SloModel{
		Id:            types.StringValue(data.ID),
		Name:          types.StringValue(data.Name),
		Description:   types.StringValue(data.Description),
		Tags:          tags,
		Configuration: config,
		Target:        types.Float64Value(data.Target),
		TimeframeDays: types.Int64Value(int64(data.TimeframeDays)),
		Owner:         types.StringValue(data.Owner),
		Permissions:   types.StringValue(data.Permissions),
		ClusterIds:    clusterIds,
		Alerts:        alerts,
	}

	return state, diags
}

func flattenSloConfiguration(ctx context.Context, config sloAPIConfiguration) (resource_slo.SloConfigurationModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch config.Type {
	case "event":
		event, d := flattenSloEventConfiguration(ctx, config)
		diags.Append(d...)
		return resource_slo.SloConfigurationModel{Event: &event}, diags
	case "time":
		timeConfig, d := flattenSloTimeConfiguration(ctx, config)
		diags.Append(d...)
		return resource_slo.SloConfigurationModel{Time: &timeConfig}, diags
	default:
		diags.AddError("Unknown configuration type", config.Type)
		return resource_slo.SloConfigurationModel{}, diags
	}
}

func flattenSloEventConfiguration(ctx context.Context, config sloAPIConfiguration) (resource_slo.SloEventConfigurationModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	goodQuery, gDiags := flattenSloQueryFormula(config.GoodQuery)
	diags.Append(gDiags...)
	totalQuery, tDiags := flattenSloQueryFormula(config.TotalQuery)
	diags.Append(tDiags...)
	groupByFields, gbDiags := flattenSloGroupBy(ctx, config.GroupByFields)
	diags.Append(gbDiags...)

	return resource_slo.SloEventConfigurationModel{
		DataSource:     types.StringValue(config.DataSource),
		GoodQuery:      goodQuery,
		TotalQuery:     totalQuery,
		GroupByFields:  groupByFields,
		NoDataBehavior: types.StringValue(config.NoDataBehavior),
	}, diags
}

func flattenSloTimeConfiguration(ctx context.Context, config sloAPIConfiguration) (resource_slo.SloTimeConfigurationModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	query, qDiags := flattenSloQueryFormula(config.Query)
	diags.Append(qDiags...)
	groupByFields, gbDiags := flattenSloGroupBy(ctx, config.GroupByFields)
	diags.Append(gbDiags...)

	if config.SliceSizeMinutes == nil {
		diags.AddError("Invalid API response", "time SLO configuration is missing sliceSizeMinutes")
	}
	if config.Threshold == nil {
		diags.AddError("Invalid API response", "time SLO configuration is missing threshold")
	}
	if diags.HasError() {
		return resource_slo.SloTimeConfigurationModel{}, diags
	}

	return resource_slo.SloTimeConfigurationModel{
		DataSource:       types.StringValue(config.DataSource),
		Query:            query,
		SliceSizeMinutes: types.Int64Value(int64(*config.SliceSizeMinutes)),
		Threshold: resource_slo.SloTimeThresholdModel{
			Operator: types.StringValue(config.Threshold.Operator),
			Value:    types.Float64Value(config.Threshold.Value),
		},
		GroupByFields:  groupByFields,
		NoDataBehavior: types.StringValue(config.NoDataBehavior),
	}, diags
}

func flattenSloQueryFormula(qf *sloAPIQueryFormula) (resource_slo.SloQueryFormulaModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	if qf == nil {
		diags.AddError("Invalid API response", "SLO configuration is missing a query formula")
		return resource_slo.SloQueryFormulaModel{}, diags
	}

	queries, qDiags := flattenMonitorQueries(qf.Queries)
	diags.Append(qDiags...)

	return resource_slo.SloQueryFormulaModel{
		Queries: queries,
		Formula: types.StringValue(qf.Formula),
	}, diags
}

// flattenSloGroupBy reuses the monitor group-by flatten but maps an empty/absent list to null,
// since group_by_fields is optional on SLOs (an absent grouping must round-trip to a null attribute).
func flattenSloGroupBy(ctx context.Context, groupByFields []monitorAPIAggregationGroupBy) (types.List, diag.Diagnostics) {
	if len(groupByFields) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: groupby.AttrTypes()}), nil
	}
	return flattenAggregationGroupBy(ctx, groupByFields)
}

// sloAlertRef decodes an alerts list (a plan or prior state value) into models used purely as an
// ordering reference when flattening the API's unordered alerts. Decode errors are non-fatal: a
// missing reference just means the alerts fall back to API order, so any diagnostics are dropped.
func sloAlertRef(ctx context.Context, alerts types.List) []resource_slo.SloAlertModel {
	if alerts.IsNull() || alerts.IsUnknown() {
		return nil
	}
	var models []resource_slo.SloAlertModel
	if diags := alerts.ElementsAs(ctx, &models, false); diags.HasError() {
		return nil
	}
	return models
}

// orderSloAlerts re-orders the API's alerts to follow ref. Each ref entry claims the first
// unclaimed API alert matching by id (existing alerts) or, when the ref entry has no id yet
// (newly created), by content. Unclaimed API alerts are appended in API order.
func orderSloAlerts(alerts []sloAPIAlert, ref []resource_slo.SloAlertModel) []sloAPIAlert {
	used := make([]bool, len(alerts))
	ordered := make([]sloAPIAlert, 0, len(alerts))

	for _, r := range ref {
		match := -1
		if !r.Id.IsNull() && !r.Id.IsUnknown() && r.Id.ValueString() != "" {
			for i, a := range alerts {
				if !used[i] && a.ID == r.Id.ValueString() {
					match = i
					break
				}
			}
		}
		if match == -1 {
			for i, a := range alerts {
				if !used[i] && sloAlertContentMatches(a, r) {
					match = i
					break
				}
			}
		}
		if match >= 0 {
			used[match] = true
			ordered = append(ordered, alerts[match])
		}
	}

	for i, a := range alerts {
		if !used[i] {
			ordered = append(ordered, a)
		}
	}

	return ordered
}

// sloAlertContentMatches reports whether an API alert has the same priority and configuration as a
// reference alert, used to line up newly created alerts (which have no id yet) with their config entry.
func sloAlertContentMatches(a sloAPIAlert, r resource_slo.SloAlertModel) bool {
	if r.Priority.IsNull() || r.Priority.IsUnknown() || int64(a.Priority) != r.Priority.ValueInt64() {
		return false
	}
	switch a.Configuration.Type {
	case "burn-rate":
		return a.Configuration.BurnRate != nil &&
			r.Configuration.BurnRateSet() &&
			*a.Configuration.BurnRate == r.Configuration.BurnRate.ValueFloat64()
	case "threshold":
		return a.Configuration.Threshold != nil &&
			r.Configuration.ThresholdSet() &&
			*a.Configuration.Threshold == r.Configuration.Threshold.ValueFloat64()
	default:
		return false
	}
}

// flattenSloAlerts converts the API's alerts into state. The SLO API returns alerts in an
// arbitrary order and reconciles them by id, so we re-order the response to follow the alert
// order Terraform expects. Each ref entry is matched to a returned alert by id, falling back to
// content (priority + configuration) for newly created alerts that have no id yet; any leftover
// alerts are appended in API order. With no ref (import) the API order is used as-is.
func flattenSloAlerts(alerts []sloAPIAlert, ref []resource_slo.SloAlertModel) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elemType := types.ObjectType{AttrTypes: resource_slo.SloAlertAttrTypes()}
	if len(alerts) == 0 {
		return types.ListValueMust(elemType, []attr.Value{}), diags
	}

	ordered := orderSloAlerts(alerts, ref)

	values := make([]attr.Value, 0, len(ordered))
	for _, a := range ordered {
		configVal, cDiags := flattenSloAlertConfiguration(a.Configuration)
		diags.Append(cDiags...)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}

		obj, oDiags := types.ObjectValue(resource_slo.SloAlertAttrTypes(), map[string]attr.Value{
			"id":            types.StringValue(a.ID),
			"priority":      types.Int64Value(int64(a.Priority)),
			"configuration": configVal,
		})
		diags.Append(oDiags...)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}

		values = append(values, obj)
	}

	list, listDiags := types.ListValue(elemType, values)
	diags.Append(listDiags...)
	return list, diags
}

func flattenSloAlertConfiguration(config sloAPIAlertConfiguration) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	burnRate := types.Float64Null()
	threshold := types.Float64Null()

	switch config.Type {
	case "burn-rate":
		if config.BurnRate != nil {
			burnRate = types.Float64Value(*config.BurnRate)
		}
	case "threshold":
		if config.Threshold != nil {
			threshold = types.Float64Value(*config.Threshold)
		}
	default:
		diags.AddError(
			"Invalid alert configuration",
			fmt.Sprintf("Unrecognized alert configuration type %q returned by the API; expected burn-rate or threshold", config.Type),
		)
		return types.ObjectNull(resource_slo.SloAlertConfigurationAttrTypes()), diags
	}

	obj, oDiags := types.ObjectValue(resource_slo.SloAlertConfigurationAttrTypes(), map[string]attr.Value{
		"burn_rate": burnRate,
		"threshold": threshold,
	})
	diags.Append(oDiags...)

	return obj, diags
}
