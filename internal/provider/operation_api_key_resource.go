package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"terraform-provider-tsuga/internal/resource_operation_api_key"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*operationApiKeyResource)(nil)
var _ resource.ResourceWithConfigure = (*operationApiKeyResource)(nil)
var _ resource.ResourceWithImportState = (*operationApiKeyResource)(nil)

func NewOperationApiKeyResource() resource.Resource {
	return &operationApiKeyResource{}
}

type operationApiKeyResource struct {
	client *TsugaClient
}

type operationApiKeyAPIResponse struct {
	Data struct {
		ID                string            `json:"id"`
		Name              string            `json:"name"`
		KeyLastCharacters string            `json:"keyLastCharacters"`
		Owner             string            `json:"owner"`
		Key               string            `json:"key"` // only present on create
		Tags              []apiTag          `json:"tags"`
		Scope             string            `json:"scope"`
		Permissions       map[string]string `json:"permissions"`
		ExpiresAt         string            `json:"expiresAt"`
	} `json:"data"`
}

func (r *operationApiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *operationApiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_operation_api_key"
}

func (r *operationApiKeyResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	base := resource_operation_api_key.OperationApiKeyResourceSchema(ctx)
	base.Description = "An operation API key used to authenticate API, Terraform, MCP, and Tsuga CLI clients."
	resp.Schema = base
}

func (r *operationApiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *operationApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_operation_api_key.OperationApiKeyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := r.modelToRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPost, "/v1/operation-api-keys", body)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create operation API key: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create operation API key: %s", err))
		return
	}

	apiResp, err := r.decode(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", err.Error())
		return
	}

	resp.Diagnostics.Append(r.apiRespToModel(ctx, &plan, apiResp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *operationApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_operation_api_key.OperationApiKeyModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/operation-api-keys/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read operation API key: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read operation API key: %s", err))
		return
	}

	r.readJSON(ctx, httpResp.Body, resp)
}

func (r *operationApiKeyResource) readJSON(ctx context.Context, bodyReader io.Reader, resp *resource.ReadResponse) {
	var state resource_operation_api_key.OperationApiKeyModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.decode(bodyReader)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", err.Error())
		return
	}

	resp.Diagnostics.Append(r.apiRespToModel(ctx, &state, apiResp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *operationApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_operation_api_key.OperationApiKeyModel
	var state resource_operation_api_key.OperationApiKeyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := r.modelToRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/operation-api-keys/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodPut, apiPath, body)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update operation API key: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update operation API key: %s", err))
		return
	}

	apiResp, err := r.decode(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", err.Error())
		return
	}

	resp.Diagnostics.Append(r.apiRespToModel(ctx, &plan, apiResp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *operationApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_operation_api_key.OperationApiKeyModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/operation-api-keys/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, apiPath, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete operation API key: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete operation API key: %s", err))
			return
		}
	}
}

func (r *operationApiKeyResource) decode(bodyReader io.Reader) (operationApiKeyAPIResponse, error) {
	var apiResp operationApiKeyAPIResponse
	raw, err := io.ReadAll(bodyReader)
	if err != nil {
		return apiResp, fmt.Errorf("unable to read response body: %w", err)
	}
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return apiResp, fmt.Errorf("unable to parse response: %w", err)
	}
	return apiResp, nil
}

func (r *operationApiKeyResource) modelToRequest(ctx context.Context, model resource_operation_api_key.OperationApiKeyModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	permissions := map[string]string{}
	diags.Append(model.Permissions.ElementsAs(ctx, &permissions, false)...)

	if diags.HasError() {
		return nil, diags
	}

	body := map[string]interface{}{
		"name":        model.Name.ValueString(),
		"owner":       model.Owner.ValueString(),
		"scope":       model.Scope.ValueString(),
		"permissions": permissions,
		"expiresAt":   model.ExpiresAt.ValueString(),
	}

	if tags, tagDiags := expandTags(ctx, model.Tags); tagDiags.HasError() {
		diags.Append(tagDiags...)
		return nil, diags
	} else if tags != nil {
		body["tags"] = tags
	}

	return body, diags
}

func (r *operationApiKeyResource) apiRespToModel(ctx context.Context, model *resource_operation_api_key.OperationApiKeyModel, apiResp operationApiKeyAPIResponse) diag.Diagnostics {
	var diags diag.Diagnostics

	model.Id = types.StringValue(apiResp.Data.ID)
	model.Name = types.StringValue(apiResp.Data.Name)
	model.KeyLastCharacters = types.StringValue(apiResp.Data.KeyLastCharacters)
	model.Owner = types.StringValue(apiResp.Data.Owner)
	model.Scope = types.StringValue(apiResp.Data.Scope)

	if apiResp.Data.Key != "" {
		model.Key = types.StringValue(apiResp.Data.Key)
	}

	// The API normalizes timestamps (e.g. adds milliseconds): keep the configured
	// spelling while it denotes the same instant.
	if apiResp.Data.ExpiresAt == "" {
		model.ExpiresAt = types.StringNull()
	} else if !sameInstant(model.ExpiresAt.ValueString(), apiResp.Data.ExpiresAt) {
		model.ExpiresAt = types.StringValue(apiResp.Data.ExpiresAt)
	}

	permissions := make(map[string]attr.Value, len(apiResp.Data.Permissions))
	for resourceType, permission := range apiResp.Data.Permissions {
		permissions[resourceType] = types.StringValue(permission)
	}
	permissionsMap, d := types.MapValue(types.StringType, permissions)
	diags.Append(d...)
	model.Permissions = permissionsMap

	// flattenTags reads no tags back as null, which Terraform rejects when the config set `tags = []`,
	// so keep the configured empty list, like the folders do.
	if len(apiResp.Data.Tags) > 0 || model.Tags.IsNull() || model.Tags.IsUnknown() || len(model.Tags.Elements()) > 0 {
		tags, d := flattenTags(ctx, apiResp.Data.Tags)
		diags.Append(d...)
		model.Tags = tags
	}

	return diags
}

func sameInstant(a, b string) bool {
	ta, errA := time.Parse(time.RFC3339, a)
	tb, errB := time.Parse(time.RFC3339, b)
	return errA == nil && errB == nil && ta.Equal(tb)
}
