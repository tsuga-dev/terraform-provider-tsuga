package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_adaptive_notification_rule"
	"terraform-provider-tsuga/internal/resource_notification_rule"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.Resource = (*adaptiveNotificationRuleResource)(nil)
var _ resource.ResourceWithConfigure = (*adaptiveNotificationRuleResource)(nil)
var _ resource.ResourceWithImportState = (*adaptiveNotificationRuleResource)(nil)
var _ resource.ResourceWithValidateConfig = (*adaptiveNotificationRuleResource)(nil)

func NewAdaptiveNotificationRuleResource() resource.Resource {
	return &adaptiveNotificationRuleResource{}
}

type adaptiveNotificationRuleResource struct {
	client *TsugaClient
}

func (r *adaptiveNotificationRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *adaptiveNotificationRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_adaptive_notification_rule"
}

func (r *adaptiveNotificationRuleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_adaptive_notification_rule.AdaptiveNotificationRuleResourceSchema(ctx)
}

func (r *adaptiveNotificationRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var monitorIDs, sloIDs types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("monitor_ids"), &monitorIDs)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("slo_ids"), &sloIDs)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateAdaptiveNotificationRuleTargets(monitorIDs, sloIDs)...)
}

// The API rejects a rule with no monitor and no SLO; this reports it at plan time instead of apply.
func validateAdaptiveNotificationRuleTargets(monitorIDs, sloIDs types.List) diag.Diagnostics {
	var diags diag.Diagnostics

	// An unknown list may still hold IDs once its value is known.
	if monitorIDs.IsUnknown() || sloIDs.IsUnknown() {
		return diags
	}

	if len(monitorIDs.Elements())+len(sloIDs.Elements()) == 0 {
		diags.AddError(
			"Missing monitors and SLOs",
			"An adaptive notification rule needs at least one monitor or SLO: set monitor_ids, slo_ids, or both.",
		)
	}

	return diags
}

func (r *adaptiveNotificationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *adaptiveNotificationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_adaptive_notification_rule.AdaptiveNotificationRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := buildAdaptiveNotificationRuleRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := r.createOrUpdate(ctx, http.MethodPost, "/v1/adaptive-notification-rules", requestBody, plan, "create")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *adaptiveNotificationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_adaptive_notification_rule.AdaptiveNotificationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/adaptive-notification-rules/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read adaptive notification rule: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read adaptive notification rule: %s", err))
		return
	}

	r.readJSON(ctx, httpResp.Body, resp)
}

func (r *adaptiveNotificationRuleResource) readJSON(ctx context.Context, bodyReader io.Reader, resp *resource.ReadResponse) {
	var prior resource_adaptive_notification_rule.AdaptiveNotificationRuleModel
	if !resp.State.Raw.IsNull() {
		resp.Diagnostics.Append(resp.State.Get(ctx, &prior)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	data, diags := decodeAdaptiveNotificationRule(bodyReader)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	newState, diags := flattenAdaptiveNotificationRule(ctx, data, prior)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *adaptiveNotificationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_adaptive_notification_rule.AdaptiveNotificationRuleModel
	var state resource_adaptive_notification_rule.AdaptiveNotificationRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := buildAdaptiveNotificationRuleRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/adaptive-notification-rules/%s", state.Id.ValueString())
	newState, diags := r.createOrUpdate(ctx, http.MethodPut, path, requestBody, plan, "update")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *adaptiveNotificationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_adaptive_notification_rule.AdaptiveNotificationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/v1/adaptive-notification-rules/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, path, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete adaptive notification rule: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete adaptive notification rule: %s", err))
			return
		}
	}
}

func (r *adaptiveNotificationRuleResource) createOrUpdate(ctx context.Context, method, path string, requestBody map[string]any, plan resource_adaptive_notification_rule.AdaptiveNotificationRuleModel, operation string) (resource_adaptive_notification_rule.AdaptiveNotificationRuleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	httpResp, err := r.client.doRequest(ctx, method, path, requestBody)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to %s adaptive notification rule: %s", operation, err))
		return resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{}, diags
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		diags.AddError("API Error", fmt.Sprintf("Unable to %s adaptive notification rule: %s", operation, err))
		return resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{}, diags
	}

	data, decodeDiags := decodeAdaptiveNotificationRule(httpResp.Body)
	diags.Append(decodeDiags...)
	if diags.HasError() {
		return resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{}, diags
	}

	return flattenAdaptiveNotificationRule(ctx, data, plan)
}

type adaptiveNotificationRuleAPIData struct {
	ID                        string                    `json:"id"`
	Name                      string                    `json:"name"`
	Owner                     string                    `json:"owner"`
	IsActive                  bool                      `json:"isActive"`
	ShouldCreateInvestigation bool                      `json:"shouldCreateInvestigation"`
	Tags                      []apiTag                  `json:"tags"`
	MonitorIDs                []string                  `json:"monitorIds"`
	SloIDs                    []string                  `json:"sloIds"`
	FallbackTarget            notificationRuleAPITarget `json:"fallbackTarget"`
}

func decodeAdaptiveNotificationRule(bodyReader io.Reader) (adaptiveNotificationRuleAPIData, diag.Diagnostics) {
	var diags diag.Diagnostics

	body, err := io.ReadAll(bodyReader)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return adaptiveNotificationRuleAPIData{}, diags
	}

	var apiResp struct {
		Data adaptiveNotificationRuleAPIData `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return adaptiveNotificationRuleAPIData{}, diags
	}

	return apiResp.Data, diags
}

func buildAdaptiveNotificationRuleRequestBody(ctx context.Context, plan resource_adaptive_notification_rule.AdaptiveNotificationRuleModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Always send both lists: the API keeps an omitted list on update, so dropping monitor_ids
	// or slo_ids from the configuration has to send an empty list to clear it.
	monitorIDs, expandDiags := expandStringList(ctx, plan.MonitorIds)
	diags.Append(expandDiags...)
	sloIDs, expandDiags := expandStringList(ctx, plan.SloIds)
	diags.Append(expandDiags...)
	fallbackTarget, expandDiags := expandAdaptiveNotificationRuleFallbackTarget(ctx, plan.FallbackTarget)
	diags.Append(expandDiags...)
	if diags.HasError() {
		return nil, diags
	}
	if monitorIDs == nil {
		monitorIDs = []string{}
	}
	if sloIDs == nil {
		sloIDs = []string{}
	}

	requestBody := map[string]any{
		"name":                      plan.Name.ValueString(),
		"owner":                     plan.Owner.ValueString(),
		"isActive":                  plan.IsActive.ValueBool(),
		"shouldCreateInvestigation": plan.ShouldCreateInvestigation.ValueBool(),
		"monitorIds":                monitorIDs,
		"sloIds":                    sloIDs,
		"fallbackTarget":            fallbackTarget,
	}

	if tags, tagDiags := expandTags(ctx, plan.Tags); tagDiags.HasError() {
		diags.Append(tagDiags...)
		return nil, diags
	} else if tags != nil {
		requestBody["tags"] = tags
	}

	return requestBody, diags
}

func expandAdaptiveNotificationRuleFallbackTarget(ctx context.Context, target *resource_notification_rule.TargetModel) (notificationRuleAPITarget, diag.Diagnostics) {
	conf, diags := expandTargetConfig(ctx, target.Config)
	if diags.HasError() {
		return notificationRuleAPITarget{}, diags
	}

	apiTarget := notificationRuleAPITarget{ID: target.Id.ValueString(), Config: conf}
	if target.RateLimit != nil {
		apiTarget.RateLimit = &notificationRuleAPITargetRateLimit{
			MaxMessages: target.RateLimit.MaxMessages.ValueInt64(),
			Minutes:     target.RateLimit.Minutes.ValueInt64(),
		}
	}

	return apiTarget, diags
}

// flattenAdaptiveNotificationRule takes the prior plan or state so an empty list keeps the form
// the configuration used: null when the attribute is omitted, an empty list when it is set to [].
func flattenAdaptiveNotificationRule(ctx context.Context, data adaptiveNotificationRuleAPIData, prior resource_adaptive_notification_rule.AdaptiveNotificationRuleModel) (resource_adaptive_notification_rule.AdaptiveNotificationRuleModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	tags, tagDiags := flattenTags(ctx, data.Tags)
	diags.Append(tagDiags...)
	monitorIDs, listDiags := flattenAdaptiveNotificationRuleIDs(ctx, data.MonitorIDs, prior.MonitorIds)
	diags.Append(listDiags...)
	sloIDs, listDiags := flattenAdaptiveNotificationRuleIDs(ctx, data.SloIDs, prior.SloIds)
	diags.Append(listDiags...)

	// The fallback target has the shape of one notification rule target, so it reuses that
	// flattener on a one-element list.
	targets, targetDiags := flattenNotificationRuleTargets(ctx, []notificationRuleAPITarget{data.FallbackTarget})
	diags.Append(targetDiags...)
	if diags.HasError() {
		return resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{}, diags
	}
	var fallbackTarget resource_notification_rule.TargetModel
	diags.Append(targets.Elements()[0].(types.Object).As(ctx, &fallbackTarget, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{}, diags
	}

	return resource_adaptive_notification_rule.AdaptiveNotificationRuleModel{
		Id:                        types.StringValue(data.ID),
		Name:                      types.StringValue(data.Name),
		Owner:                     types.StringValue(data.Owner),
		Tags:                      tags,
		IsActive:                  types.BoolValue(data.IsActive),
		ShouldCreateInvestigation: types.BoolValue(data.ShouldCreateInvestigation),
		MonitorIds:                monitorIDs,
		SloIds:                    sloIDs,
		FallbackTarget:            &fallbackTarget,
	}, diags
}

func flattenAdaptiveNotificationRuleIDs(ctx context.Context, ids []string, prior types.List) (types.List, diag.Diagnostics) {
	if len(ids) == 0 {
		if prior.IsNull() || prior.IsUnknown() {
			return types.ListNull(types.StringType), nil
		}
		return types.ListValueMust(types.StringType, nil), nil
	}
	return types.ListValueFrom(ctx, types.StringType, ids)
}
