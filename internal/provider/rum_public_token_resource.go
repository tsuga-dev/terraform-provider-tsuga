package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_rum_public_token"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*rumPublicTokenResource)(nil)
var _ resource.ResourceWithConfigure = (*rumPublicTokenResource)(nil)
var _ resource.ResourceWithImportState = (*rumPublicTokenResource)(nil)

func NewRumPublicTokenResource() resource.Resource {
	return &rumPublicTokenResource{}
}

type rumPublicTokenResource struct {
	client *TsugaClient
}

type rumPublicTokenAPIResponse struct {
	Data struct {
		ID                 string   `json:"id"`
		Name               string   `json:"name"`
		Token              string   `json:"token"`
		Owner              string   `json:"owner"`
		ApplicationType    string   `json:"applicationType"`
		AllowedUrlPrefixes []string `json:"allowedUrlPrefixes"`
		Tags               []apiTag `json:"tags"`
		TeamOverrideFields []string `json:"teamOverrideFields"`
	} `json:"data"`
}

func (r *rumPublicTokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *rumPublicTokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rum_public_token"
}

func (r *rumPublicTokenResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	base := resource_rum_public_token.RumPublicTokenResourceSchema(ctx)
	base.Description = "A RUM public token used by the browser and mobile SDKs to send real user monitoring data to Tsuga."
	resp.Schema = base
}

func (r *rumPublicTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *rumPublicTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_rum_public_token.RumPublicTokenModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := r.modelToRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPost, "/v1/public-tokens", body)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create RUM public token: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create RUM public token: %s", err))
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

func (r *rumPublicTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_rum_public_token.RumPublicTokenModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/public-tokens/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read RUM public token: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read RUM public token: %s", err))
		return
	}

	r.readJSON(ctx, httpResp.Body, resp)
}

func (r *rumPublicTokenResource) readJSON(ctx context.Context, bodyReader io.Reader, resp *resource.ReadResponse) {
	var state resource_rum_public_token.RumPublicTokenModel
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

func (r *rumPublicTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_rum_public_token.RumPublicTokenModel
	var state resource_rum_public_token.RumPublicTokenModel

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

	apiPath := fmt.Sprintf("/v1/public-tokens/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodPut, apiPath, body)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update RUM public token: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update RUM public token: %s", err))
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

func (r *rumPublicTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_rum_public_token.RumPublicTokenModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/public-tokens/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, apiPath, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete RUM public token: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete RUM public token: %s", err))
			return
		}
	}
}

func (r *rumPublicTokenResource) decode(bodyReader io.Reader) (rumPublicTokenAPIResponse, error) {
	var apiResp rumPublicTokenAPIResponse
	raw, err := io.ReadAll(bodyReader)
	if err != nil {
		return apiResp, fmt.Errorf("unable to read response body: %w", err)
	}
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return apiResp, fmt.Errorf("unable to parse response: %w", err)
	}
	return apiResp, nil
}

func (r *rumPublicTokenResource) modelToRequest(ctx context.Context, model resource_rum_public_token.RumPublicTokenModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	// The API requires every field on create and update, so empty lists are sent rather than omitted.
	allowedUrlPrefixes, d := expandStringList(ctx, model.AllowedUrlPrefixes)
	diags.Append(d...)
	if allowedUrlPrefixes == nil {
		allowedUrlPrefixes = []string{}
	}

	tags, d := expandTags(ctx, model.Tags)
	diags.Append(d...)

	fields, d := expandStringList(ctx, model.TeamOverrideFields)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	body := map[string]interface{}{
		"name":               model.Name.ValueString(),
		"owner":              model.Owner.ValueString(),
		"applicationType":    model.ApplicationType.ValueString(),
		"allowedUrlPrefixes": allowedUrlPrefixes,
		"tags":               tags,
		"teamOverrideFields": nil,
	}
	if len(fields) > 0 {
		body["teamOverrideFields"] = fields
	}

	return body, diags
}

func (r *rumPublicTokenResource) apiRespToModel(ctx context.Context, model *resource_rum_public_token.RumPublicTokenModel, apiResp rumPublicTokenAPIResponse) diag.Diagnostics {
	var diags diag.Diagnostics

	model.Id = types.StringValue(apiResp.Data.ID)
	model.Name = types.StringValue(apiResp.Data.Name)
	model.Token = types.StringValue(apiResp.Data.Token)
	model.Owner = types.StringValue(apiResp.Data.Owner)
	model.ApplicationType = types.StringValue(apiResp.Data.ApplicationType)

	// allowed_url_prefixes is required, so an empty API value stays an empty list.
	prefixes := make([]attr.Value, 0, len(apiResp.Data.AllowedUrlPrefixes))
	for _, p := range apiResp.Data.AllowedUrlPrefixes {
		prefixes = append(prefixes, types.StringValue(p))
	}
	list, d := types.ListValue(types.StringType, prefixes)
	diags.Append(d...)
	model.AllowedUrlPrefixes = list

	tags, d := flattenTags(ctx, apiResp.Data.Tags)
	diags.Append(d...)
	model.Tags = tags

	if len(apiResp.Data.TeamOverrideFields) == 0 {
		model.TeamOverrideFields = types.ListNull(types.StringType)
	} else {
		fields := make([]attr.Value, 0, len(apiResp.Data.TeamOverrideFields))
		for _, f := range apiResp.Data.TeamOverrideFields {
			fields = append(fields, types.StringValue(f))
		}
		list, d := types.ListValue(types.StringType, fields)
		diags.Append(d...)
		model.TeamOverrideFields = list
	}

	return diags
}
