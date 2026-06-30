package provider

import (
	"context"
	"terraform-provider-tsuga/internal/aggregate"
	"terraform-provider-tsuga/internal/groupby"
	"terraform-provider-tsuga/internal/resource_monitor"
	"terraform-provider-tsuga/internal/resource_slo"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// sloCountQueries builds a minimal queries list with a single count aggregate, matching the
// monitor aggregation-query shape that SLOs reuse.
func sloCountQueries(t *testing.T, filter string) types.List {
	t.Helper()

	elemType := types.ObjectType{AttrTypes: resource_monitor.QueryAttrTypes()}
	list, diags := types.ListValueFrom(context.Background(), elemType, []resource_monitor.MonitorQueryModel{
		{
			Filter: types.StringValue(filter),
			Aggregate: resource_monitor.MonitorAggregateModel{
				Count: &aggregate.CountModel{Field: types.StringNull()},
			},
			Functions: types.ListNull(types.ObjectType{AttrTypes: resource_monitor.AggregationFunctionAttrTypes()}),
		},
	})
	if diags.HasError() {
		t.Fatalf("failed to build queries list: %v", diags)
	}
	return list
}

func TestExpandSloAlertConfiguration_BurnRate(t *testing.T) {
	expanded, diags := expandSloAlertConfiguration(resource_slo.SloAlertConfigurationModel{
		BurnRate:  types.Float64Value(14.4),
		Threshold: types.Float64Null(),
	}, "alerts[0].configuration")
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if expanded["type"] != "burn-rate" {
		t.Fatalf("expected type burn-rate, got %v", expanded["type"])
	}
	if expanded["burnRate"] != 14.4 {
		t.Fatalf("expected burnRate 14.4, got %v", expanded["burnRate"])
	}
	if _, exists := expanded["threshold"]; exists {
		t.Fatalf("expected threshold to be omitted for burn-rate alert, got %v", expanded["threshold"])
	}
}

func TestExpandSloAlertConfiguration_Threshold(t *testing.T) {
	expanded, diags := expandSloAlertConfiguration(resource_slo.SloAlertConfigurationModel{
		BurnRate:  types.Float64Null(),
		Threshold: types.Float64Value(99.0),
	}, "alerts[0].configuration")
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if expanded["type"] != "threshold" {
		t.Fatalf("expected type threshold, got %v", expanded["type"])
	}
	if expanded["threshold"] != 99.0 {
		t.Fatalf("expected threshold 99.0, got %v", expanded["threshold"])
	}
}

func TestExpandSloAlertConfiguration_BothSetErrors(t *testing.T) {
	_, diags := expandSloAlertConfiguration(resource_slo.SloAlertConfigurationModel{
		BurnRate:  types.Float64Value(14.4),
		Threshold: types.Float64Value(99.0),
	}, "alerts[0].configuration")
	if !diags.HasError() {
		t.Fatal("expected an error when both burn_rate and threshold are set")
	}
}

func TestExpandSloAlertConfiguration_NoneSetErrors(t *testing.T) {
	_, diags := expandSloAlertConfiguration(resource_slo.SloAlertConfigurationModel{
		BurnRate:  types.Float64Null(),
		Threshold: types.Float64Null(),
	}, "alerts[0].configuration")
	if !diags.HasError() {
		t.Fatal("expected an error when neither burn_rate nor threshold is set")
	}
}

func TestValidateSloAggregate_ExactlyOne(t *testing.T) {
	// No aggregate set -> error.
	diags := validateSloAggregate(resource_monitor.MonitorAggregateModel{}, "q[0].aggregate")
	if !diags.HasError() {
		t.Fatal("expected an error when no aggregate type is set")
	}

	// Exactly one (count) set -> ok.
	diags = validateSloAggregate(resource_monitor.MonitorAggregateModel{
		Count: &aggregate.CountModel{Field: types.StringNull()},
	}, "q[0].aggregate")
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics for a single count aggregate: %v", diags)
	}

	// Two aggregates set -> error.
	diags = validateSloAggregate(resource_monitor.MonitorAggregateModel{
		Count:   &aggregate.CountModel{Field: types.StringNull()},
		Average: &aggregate.FieldModel{Field: types.StringValue("latency")},
	}, "q[0].aggregate")
	if !diags.HasError() {
		t.Fatal("expected an error when two aggregate types are set")
	}
}

func sloAlertList(t *testing.T, alerts ...resource_slo.SloAlertModel) types.List {
	t.Helper()
	elemType := types.ObjectType{AttrTypes: resource_slo.SloAlertAttrTypes()}
	list, diags := types.ListValueFrom(context.Background(), elemType, alerts)
	if diags.HasError() {
		t.Fatalf("failed to build alerts list: %v", diags)
	}
	return list
}

func TestExpandSloAlerts_MatchesPriorIdsByContent(t *testing.T) {
	ctx := context.Background()
	// Prior state has two alerts with server ids.
	prior := []resource_slo.SloAlertModel{
		sloAlertRefEntry("alert-1", 1, types.Float64Value(14.4), types.Float64Null()),
		sloAlertRefEntry("alert-2", 3, types.Float64Null(), types.Float64Value(99.0)),
	}
	// Plan keeps the burn-rate alert unchanged (reuse alert-1) and edits the threshold alert's value
	alerts := sloAlertList(t,
		sloAlertRefEntry("", 1, types.Float64Value(14.4), types.Float64Null()),
		sloAlertRefEntry("", 4, types.Float64Null(), types.Float64Value(95.0)),
	)

	r := &sloResource{}
	expanded, diags := r.expandSloAlerts(ctx, alerts, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(expanded) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(expanded))
	}
	if expanded[0]["id"] != "alert-1" {
		t.Fatalf("expected unchanged burn-rate alert to reuse its content-matched id alert-1, got %#v", expanded[0]["id"])
	}
	if _, exists := expanded[1]["id"]; exists {
		t.Fatalf("expected content-changed threshold alert to be sent without an id, got %#v", expanded[1]["id"])
	}
}

func TestExpandSloAlerts_RemovePreservesRemainingAlertId(t *testing.T) {
	ctx := context.Background()
	// Prior: A(id1, burn-rate) then B(id2, threshold).
	prior := []resource_slo.SloAlertModel{
		sloAlertRefEntry("id1", 1, types.Float64Value(14.4), types.Float64Null()),
		sloAlertRefEntry("id2", 3, types.Float64Null(), types.Float64Value(99.0)),
	}
	// Remove A, keep B unchanged. B must keep id2 — not inherit A's id1 by position.
	alerts := sloAlertList(t,
		sloAlertRefEntry("", 3, types.Float64Null(), types.Float64Value(99.0)),
	)

	r := &sloResource{}
	expanded, diags := r.expandSloAlerts(ctx, alerts, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(expanded) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(expanded))
	}
	if expanded[0]["id"] != "id2" {
		t.Fatalf("removing the first alert must keep the remaining alert's own id id2, got %#v", expanded[0]["id"])
	}
}

func TestExpandSloAlerts_DuplicatesPreserveBothIds(t *testing.T) {
	ctx := context.Background()
	// Two prior alerts with identical content but distinct ids. Two identical planned alerts must
	// reuse both prior ids, one each (claim-once), not double up on one.
	prior := []resource_slo.SloAlertModel{
		sloAlertRefEntry("id1", 3, types.Float64Null(), types.Float64Value(99.0)),
		sloAlertRefEntry("id2", 3, types.Float64Null(), types.Float64Value(99.0)),
	}
	alerts := sloAlertList(t,
		sloAlertRefEntry("", 3, types.Float64Null(), types.Float64Value(99.0)),
		sloAlertRefEntry("", 3, types.Float64Null(), types.Float64Value(99.0)),
	)

	r := &sloResource{}
	expanded, diags := r.expandSloAlerts(ctx, alerts, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(expanded) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(expanded))
	}
	seen := map[interface{}]bool{expanded[0]["id"]: true, expanded[1]["id"]: true}
	if !seen["id1"] || !seen["id2"] {
		t.Fatalf("expected two identical alerts to reuse both distinct ids id1 and id2, got [%v, %v]", expanded[0]["id"], expanded[1]["id"])
	}
}

func TestExpandSloAlerts_ReorderMatchesByContent(t *testing.T) {
	ctx := context.Background()
	// Prior [A(id-a, burn), B(id-b, threshold)] reversed in the plan to [B, A]. Each planned alert
	// reuses the id of the prior alert with matching content, not the id at its position.
	prior := []resource_slo.SloAlertModel{
		sloAlertRefEntry("id-a", 1, types.Float64Value(14.4), types.Float64Null()),
		sloAlertRefEntry("id-b", 3, types.Float64Null(), types.Float64Value(99.0)),
	}
	alerts := sloAlertList(t,
		sloAlertRefEntry("", 3, types.Float64Null(), types.Float64Value(99.0)),
		sloAlertRefEntry("", 1, types.Float64Value(14.4), types.Float64Null()),
	)

	r := &sloResource{}
	expanded, diags := r.expandSloAlerts(ctx, alerts, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if expanded[0]["id"] != "id-b" || expanded[1]["id"] != "id-a" {
		t.Fatalf("expected reordered alerts to reuse their content-matched ids [id-b, id-a], got [%v, %v]", expanded[0]["id"], expanded[1]["id"])
	}
}

func TestOrderSloAlerts_DuplicatesStableById(t *testing.T) {
	// The API returns two identical-content alerts in the opposite order from state. Matching by id
	// before fields must keep them in the reference (state) order so a refresh shows no churn.
	threshold := 99.0
	alerts := []sloAPIAlert{
		{ID: "id2", Priority: 3, Configuration: sloAPIAlertConfiguration{Type: "threshold", Threshold: &threshold}},
		{ID: "id1", Priority: 3, Configuration: sloAPIAlertConfiguration{Type: "threshold", Threshold: &threshold}},
	}
	ref := []resource_slo.SloAlertModel{
		sloAlertRefEntry("id1", 3, types.Float64Null(), types.Float64Value(99.0)),
		sloAlertRefEntry("id2", 3, types.Float64Null(), types.Float64Value(99.0)),
	}

	ordered := orderSloAlerts(alerts, ref)
	if ordered[0].ID != "id1" || ordered[1].ID != "id2" {
		t.Fatalf("expected identical alerts ordered by id to match the reference [id1, id2], got [%q, %q]", ordered[0].ID, ordered[1].ID)
	}
}

func sloAlertRefEntry(id string, priority int64, burnRate, threshold types.Float64) resource_slo.SloAlertModel {
	idVal := types.StringValue(id)
	if id == "" {
		idVal = types.StringNull()
	}
	return resource_slo.SloAlertModel{
		Id:       idVal,
		Priority: types.Int64Value(priority),
		Configuration: resource_slo.SloAlertConfigurationModel{
			BurnRate:  burnRate,
			Threshold: threshold,
		},
	}
}

func TestFlattenSloAlerts_ReordersToReferenceById(t *testing.T) {
	ctx := context.Background()
	burn := 14.4
	threshold := 99.0
	// API returns alerts in [alert-1, alert-2] order...
	alerts := []sloAPIAlert{
		{ID: "alert-1", Priority: 1, Configuration: sloAPIAlertConfiguration{Type: "burn-rate", BurnRate: &burn}},
		{ID: "alert-2", Priority: 3, Configuration: sloAPIAlertConfiguration{Type: "threshold", Threshold: &threshold}},
	}
	// ...but the reference (plan/state) wants them reversed.
	ref := []resource_slo.SloAlertModel{
		sloAlertRefEntry("alert-2", 3, types.Float64Null(), types.Float64Value(99.0)),
		sloAlertRefEntry("alert-1", 1, types.Float64Value(14.4), types.Float64Null()),
	}

	list, diags := flattenSloAlerts(alerts, ref)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var models []resource_slo.SloAlertModel
	diags.Append(list.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		t.Fatalf("failed to decode alerts: %v", diags)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(models))
	}
	if models[0].Id.ValueString() != "alert-2" || models[1].Id.ValueString() != "alert-1" {
		t.Fatalf("expected alerts re-ordered to match reference [alert-2, alert-1], got %q, %q", models[0].Id.ValueString(), models[1].Id.ValueString())
	}
	if models[0].Configuration.Threshold.ValueFloat64() != 99.0 {
		t.Fatalf("expected first alert threshold 99.0, got %v", models[0].Configuration.Threshold)
	}
	if models[1].Configuration.BurnRate.ValueFloat64() != 14.4 {
		t.Fatalf("expected second alert burn_rate 14.4, got %v", models[1].Configuration.BurnRate)
	}
}

func TestFlattenSloAlerts_MatchesNewAlertByContentThenAppendsLeftovers(t *testing.T) {
	ctx := context.Background()
	burn := 6.0
	threshold := 95.0
	// API returns a freshly-created alert (server id) the plan referenced only by content,
	// plus an extra server-side alert with no reference entry.
	alerts := []sloAPIAlert{
		{ID: "server-extra", Priority: 5, Configuration: sloAPIAlertConfiguration{Type: "threshold", Threshold: &threshold}},
		{ID: "server-new", Priority: 2, Configuration: sloAPIAlertConfiguration{Type: "burn-rate", BurnRate: &burn}},
	}
	// Reference has one entry with no id yet, matched by content (priority 2, burn-rate 6).
	ref := []resource_slo.SloAlertModel{
		sloAlertRefEntry("", 2, types.Float64Value(6.0), types.Float64Null()),
	}

	list, diags := flattenSloAlerts(alerts, ref)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	var models []resource_slo.SloAlertModel
	diags.Append(list.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		t.Fatalf("failed to decode alerts: %v", diags)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(models))
	}
	// Content-matched new alert comes first (its server id resolved), leftover appended after.
	if models[0].Id.ValueString() != "server-new" {
		t.Fatalf("expected content-matched alert 'server-new' first, got %q", models[0].Id.ValueString())
	}
	if models[1].Id.ValueString() != "server-extra" {
		t.Fatalf("expected leftover alert 'server-extra' appended, got %q", models[1].Id.ValueString())
	}
}

func TestFlattenSloAlerts_NoReferenceKeepsApiOrder(t *testing.T) {
	ctx := context.Background()
	burn := 14.4
	threshold := 99.0
	alerts := []sloAPIAlert{
		{ID: "a1", Priority: 1, Configuration: sloAPIAlertConfiguration{Type: "burn-rate", BurnRate: &burn}},
		{ID: "a2", Priority: 3, Configuration: sloAPIAlertConfiguration{Type: "threshold", Threshold: &threshold}},
	}

	list, diags := flattenSloAlerts(alerts, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	var models []resource_slo.SloAlertModel
	diags.Append(list.ElementsAs(ctx, &models, false)...)
	if diags.HasError() {
		t.Fatalf("failed to decode alerts: %v", diags)
	}
	if models[0].Id.ValueString() != "a1" || models[1].Id.ValueString() != "a2" {
		t.Fatalf("expected API order [a1, a2] with no reference, got %q, %q", models[0].Id.ValueString(), models[1].Id.ValueString())
	}
}

func TestFlattenSloConfiguration_Event(t *testing.T) {
	ctx := context.Background()
	config := sloAPIConfiguration{
		Type:       "event",
		DataSource: "logs",
		GoodQuery: &sloAPIQueryFormula{
			Queries: []monitorAPIQuery{{Filter: "status:ok", Aggregate: monitorAPIAggregate{Type: "count"}}},
			Formula: "q1",
		},
		TotalQuery: &sloAPIQueryFormula{
			Queries: []monitorAPIQuery{{Filter: "*", Aggregate: monitorAPIAggregate{Type: "count"}}},
			Formula: "q1",
		},
		NoDataBehavior: "good",
	}

	model, diags := flattenSloConfiguration(ctx, config)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if model.Event == nil {
		t.Fatal("expected event configuration to be set")
	}
	if model.Time != nil {
		t.Fatal("expected time configuration to be nil")
	}
	if model.Event.DataSource.ValueString() != "logs" {
		t.Fatalf("expected data_source logs, got %v", model.Event.DataSource)
	}
	if model.Event.GoodQuery.Formula.ValueString() != "q1" {
		t.Fatalf("expected good_query formula q1, got %v", model.Event.GoodQuery.Formula)
	}
	if model.Event.NoDataBehavior.ValueString() != "good" {
		t.Fatalf("expected no_data_behavior good, got %v", model.Event.NoDataBehavior)
	}
	if !model.Event.GroupByFields.IsNull() {
		t.Fatalf("expected group_by_fields to be null when absent, got %v", model.Event.GroupByFields)
	}
}

func TestFlattenSloConfiguration_Time(t *testing.T) {
	ctx := context.Background()
	sliceSize := 5.0
	config := sloAPIConfiguration{
		Type:       "time",
		DataSource: "traces",
		Query: &sloAPIQueryFormula{
			Queries: []monitorAPIQuery{{Filter: "service:api", Aggregate: monitorAPIAggregate{Type: "average", Field: "duration"}}},
			Formula: "q1",
		},
		SliceSizeMinutes: &sliceSize,
		Threshold:        &sloAPITimeThreshold{Operator: "less_than", Value: 300},
		GroupByFields:    []monitorAPIAggregationGroupBy{{Fields: []string{"endpoint"}, Limit: 10}},
		NoDataBehavior:   "ignore",
	}

	model, diags := flattenSloConfiguration(ctx, config)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if model.Time == nil {
		t.Fatal("expected time configuration to be set")
	}
	if model.Time.SliceSizeMinutes.ValueInt64() != 5 {
		t.Fatalf("expected slice_size_minutes 5, got %v", model.Time.SliceSizeMinutes)
	}
	if model.Time.Threshold.Operator.ValueString() != "less_than" {
		t.Fatalf("expected threshold operator less_than, got %v", model.Time.Threshold.Operator)
	}
	if model.Time.Threshold.Value.ValueFloat64() != 300 {
		t.Fatalf("expected threshold value 300, got %v", model.Time.Threshold.Value)
	}
	if model.Time.NoDataBehavior.ValueString() != "ignore" {
		t.Fatalf("expected no_data_behavior ignore, got %v", model.Time.NoDataBehavior)
	}
	if model.Time.GroupByFields.IsNull() {
		t.Fatal("expected group_by_fields to be set when present")
	}
}

func TestFlattenSloGroupBy_EmptyIsNull(t *testing.T) {
	ctx := context.Background()
	list, diags := flattenSloGroupBy(ctx, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !list.IsNull() {
		t.Fatalf("expected null list for empty group_by_fields, got %v", list)
	}

	list, diags = flattenSloGroupBy(ctx, []monitorAPIAggregationGroupBy{{Fields: []string{"endpoint"}, Limit: 10}})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if list.IsNull() {
		t.Fatal("expected non-null list when group_by_fields present")
	}
	if len(list.Elements()) != 1 {
		t.Fatalf("expected 1 group_by element, got %d", len(list.Elements()))
	}
}

func TestFlattenSloGroupBy_MultipleLevelsPreserved(t *testing.T) {
	ctx := context.Background()
	// The API allows up to 7 grouping levels; a multi-level grouping must round-trip every level
	// rather than being truncated to one.
	list, diags := flattenSloGroupBy(ctx, []monitorAPIAggregationGroupBy{
		{Fields: []string{"service"}, Limit: 10},
		{Fields: []string{"endpoint"}, Limit: 5},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := len(list.Elements()); got != 2 {
		t.Fatalf("expected 2 group_by levels preserved, got %d", got)
	}
}

func TestExpandSloEventConfiguration_OmitsEmptyGroupBy(t *testing.T) {
	ctx := context.Background()
	groupByElemType := types.ObjectType{AttrTypes: groupby.AttrTypes()}

	config := &resource_slo.SloEventConfigurationModel{
		DataSource: types.StringValue("logs"),
		GoodQuery: resource_slo.SloQueryFormulaModel{
			Queries: sloCountQueries(t, "status:ok"),
			Formula: types.StringValue("q1"),
		},
		TotalQuery: resource_slo.SloQueryFormulaModel{
			Queries: sloCountQueries(t, "*"),
			Formula: types.StringValue("q1"),
		},
		GroupByFields:  types.ListNull(groupByElemType),
		NoDataBehavior: types.StringValue("good"),
	}

	r := &sloResource{}
	expanded, diags := r.expandSloEventConfiguration(ctx, config)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if expanded["type"] != "event" {
		t.Fatalf("expected type event, got %v", expanded["type"])
	}
	if _, exists := expanded["groupByFields"]; exists {
		t.Fatalf("expected groupByFields to be omitted when empty, got %#v", expanded["groupByFields"])
	}
}

func TestExpandSloTimeConfiguration_BuildsThreshold(t *testing.T) {
	ctx := context.Background()
	groupByElemType := types.ObjectType{AttrTypes: groupby.AttrTypes()}

	config := &resource_slo.SloTimeConfigurationModel{
		DataSource: types.StringValue("metrics"),
		Query: resource_slo.SloQueryFormulaModel{
			Queries: sloCountQueries(t, "service:api"),
			Formula: types.StringValue("q1"),
		},
		SliceSizeMinutes: types.Int64Value(5),
		Threshold: resource_slo.SloTimeThresholdModel{
			Operator: types.StringValue("less_than"),
			Value:    types.Float64Value(300),
		},
		GroupByFields:  types.ListNull(groupByElemType),
		NoDataBehavior: types.StringValue("ignore"),
	}

	r := &sloResource{}
	expanded, diags := r.expandSloTimeConfiguration(ctx, config)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if expanded["type"] != "time" {
		t.Fatalf("expected type time, got %v", expanded["type"])
	}
	if expanded["sliceSizeMinutes"] != int64(5) {
		t.Fatalf("expected sliceSizeMinutes 5, got %#v", expanded["sliceSizeMinutes"])
	}
	threshold, ok := expanded["threshold"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected threshold to be a map, got %#v", expanded["threshold"])
	}
	if threshold["operator"] != "less_than" {
		t.Fatalf("expected threshold operator less_than, got %v", threshold["operator"])
	}
	if threshold["value"] != float64(300) {
		t.Fatalf("expected threshold value 300, got %#v", threshold["value"])
	}
}

func TestFlattenSlo_EmptyDescriptionPreservedAsEmptyString(t *testing.T) {
	ctx := context.Background()
	countQuery := []monitorAPIQuery{{Filter: "*", Aggregate: monitorAPIAggregate{Type: "count"}}}
	data := sloAPIData{
		ID:          "slo-1",
		Name:        "n",
		Description: "", // server returns an empty/absent description
		Configuration: sloAPIConfiguration{
			Type:           "event",
			DataSource:     "logs",
			GoodQuery:      &sloAPIQueryFormula{Queries: countQuery, Formula: "q1"},
			TotalQuery:     &sloAPIQueryFormula{Queries: countQuery, Formula: "q1"},
			NoDataBehavior: "good",
		},
		Target:        99.0,
		TimeframeDays: 28,
		Owner:         "team-1",
		Permissions:   "all",
		ClusterIds:    []string{},
		Alerts:        []sloAPIAlert{},
	}

	state, diags := flattenSlo(ctx, data, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	// Must stay an empty string, not null: a config of description = "" plans as "" and would
	// otherwise hit "inconsistent result after apply".
	if state.Description.IsNull() {
		t.Fatal("expected empty description to flatten to \"\", got null")
	}
	if state.Description.ValueString() != "" {
		t.Fatalf("expected empty description, got %q", state.Description.ValueString())
	}
}

func TestValidateSloQueryFormula_RejectsEmptyQueries(t *testing.T) {
	ctx := context.Background()
	r := &sloResource{}
	elemType := types.ObjectType{AttrTypes: resource_monitor.QueryAttrTypes()}

	// Explicit empty queries list must be rejected at validation time (it would otherwise
	// round-trip to null and fail the apply with an inconsistent-result error).
	emptyQueries, d := types.ListValueFrom(ctx, elemType, []resource_monitor.MonitorQueryModel{})
	if d.HasError() {
		t.Fatalf("failed to build empty queries list: %v", d)
	}
	emptyFormula := resource_slo.SloQueryFormulaModel{Queries: emptyQueries, Formula: types.StringValue("q1")}
	if diags := r.validateSloQueryFormula(ctx, emptyFormula, "configuration.event.good_query"); !diags.HasError() {
		t.Fatal("expected an error for an empty queries list")
	}

	// A non-empty queries list passes.
	nonEmpty := resource_slo.SloQueryFormulaModel{Queries: sloCountQueries(t, "*"), Formula: types.StringValue("q1")}
	if diags := r.validateSloQueryFormula(ctx, nonEmpty, "configuration.event.good_query"); diags.HasError() {
		t.Fatalf("unexpected diagnostics for a non-empty queries list: %v", diags)
	}
}

func TestFlattenSloTimeConfiguration_ErrorsOnMissingRequiredFields(t *testing.T) {
	ctx := context.Background()
	// A "time" config from the API must carry sliceSizeMinutes and threshold; a response missing
	// them must fail rather than normalize to slice size 0 / null threshold (invalid state).
	config := sloAPIConfiguration{
		Type:           "time",
		DataSource:     "metrics",
		Query:          &sloAPIQueryFormula{Queries: []monitorAPIQuery{{Filter: "x", Aggregate: monitorAPIAggregate{Type: "count"}}}, Formula: "q1"},
		NoDataBehavior: "good",
		// SliceSizeMinutes and Threshold intentionally nil.
	}
	if _, diags := flattenSloTimeConfiguration(ctx, config); !diags.HasError() {
		t.Fatal("expected an error when a time config is missing sliceSizeMinutes/threshold")
	}
}

func TestFlattenSloAlertConfiguration_ErrorsOnUnknownType(t *testing.T) {
	// An unrecognized alert type would otherwise leave both burn_rate and threshold null,
	// violating the exactly-one contract; flatten must fail instead.
	if _, diags := flattenSloAlertConfiguration(sloAPIAlertConfiguration{Type: "bogus"}); !diags.HasError() {
		t.Fatal("expected an error for an unrecognized alert configuration type")
	}
}
