package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"terraform-provider-tsuga/internal/resource_cloud_account"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const cloudAccountBasePath = "/v1/inventory/cloud-accounts"

var _ resource.Resource = (*cloudAccountResource)(nil)
var _ resource.ResourceWithConfigure = (*cloudAccountResource)(nil)
var _ resource.ResourceWithConfigValidators = (*cloudAccountResource)(nil)

func NewCloudAccountResource() resource.Resource {
	return &cloudAccountResource{}
}

type cloudAccountResource struct {
	client *TsugaClient
}

func (r *cloudAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *cloudAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_account"
}

func (r *cloudAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_cloud_account.CloudAccountResourceSchema()
}

func (r *cloudAccountResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("aws"),
			path.MatchRoot("gcp"),
		),
	}
}

func expandConnectionSettings(plan resource_cloud_account.CloudAccountModel) (map[string]interface{}, string, string) {
	if plan.Aws != nil {
		return map[string]interface{}{
			"type":       "aws",
			"accountId":  plan.Aws.AccountId.ValueString(),
			"externalId": plan.Aws.ExternalId.ValueString(),
			"roleArn":    plan.Aws.RoleArn.ValueString(),
		}, "aws", plan.Aws.AccountId.ValueString()
	}
	return map[string]interface{}{
		"type":                     "gcp",
		"projectId":                plan.Gcp.ProjectId.ValueString(),
		"serviceAccountId":         plan.Gcp.ServiceAccountId.ValueString(),
		"workloadIdentityProvider": plan.Gcp.WorkloadIdentityProvider.ValueString(),
	}, "gcp", plan.Gcp.ProjectId.ValueString()
}

func flattenAPIResponse(model *resource_cloud_account.CloudAccountModel, data cloudAccountData) {
	model.Id = types.StringValue(data.ID)
	model.CloudType = types.StringValue(data.CloudType)
	model.CloudAccountId = types.StringValue(data.CloudAccountId)
	model.AccountFriendlyName = stringValueOrNull(data.AccountFriendlyName)
}

func (r *cloudAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_cloud_account.CloudAccountModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	connectionSettings, cloudType, cloudAccountId := expandConnectionSettings(plan)
	requestBody := map[string]interface{}{
		"cloudType":          cloudType,
		"cloudAccountId":     cloudAccountId,
		"connectionSettings": connectionSettings,
	}
	if !plan.AccountFriendlyName.IsNull() && !plan.AccountFriendlyName.IsUnknown() && plan.AccountFriendlyName.ValueString() != "" {
		requestBody["accountFriendlyName"] = plan.AccountFriendlyName.ValueString()
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPost, cloudAccountBasePath, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create cloud account: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create cloud account: %s", err))
		return
	}

	data, err := parseCloudAccountResponse(httpResp)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", err.Error())
		return
	}

	flattenAPIResponse(&plan, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cloudAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_cloud_account.CloudAccountModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("%s/%s", cloudAccountBasePath, state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cloud account: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read cloud account: %s", err))
		return
	}

	data, err := parseCloudAccountResponse(httpResp)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", err.Error())
		return
	}

	flattenAPIResponse(&state, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cloudAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_cloud_account.CloudAccountModel
	var state resource_cloud_account.CloudAccountModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := map[string]interface{}{
		"accountFriendlyName": plan.AccountFriendlyName.ValueString(),
	}

	apiPath := fmt.Sprintf("%s/%s", cloudAccountBasePath, state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodPut, apiPath, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update cloud account: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update cloud account: %s", err))
		return
	}

	data, err := parseCloudAccountResponse(httpResp)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", err.Error())
		return
	}

	flattenAPIResponse(&plan, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cloudAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_cloud_account.CloudAccountModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("%s/%s", cloudAccountBasePath, state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, apiPath, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete cloud account: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete cloud account: %s", err))
			return
		}
	}
}

func parseCloudAccountResponse(httpResp *http.Response) (cloudAccountData, error) {
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return cloudAccountData{}, fmt.Errorf("unable to read response body: %w", err)
	}

	var apiResp cloudAccountAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return cloudAccountData{}, fmt.Errorf("unable to parse response: %w", err)
	}
	return apiResp.Data, nil
}

type cloudAccountAPIResponse struct {
	Data cloudAccountData `json:"data"`
}

type cloudAccountData struct {
	ID                  string `json:"id"`
	CloudType           string `json:"cloudType"`
	CloudAccountId      string `json:"cloudAccountId"`
	AccountFriendlyName string `json:"accountFriendlyName"`
}
