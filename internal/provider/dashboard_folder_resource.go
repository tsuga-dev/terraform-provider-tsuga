package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_dashboard_folder"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*dashboardFolderResource)(nil)
var _ resource.ResourceWithConfigure = (*dashboardFolderResource)(nil)
var _ resource.ResourceWithImportState = (*dashboardFolderResource)(nil)

func NewDashboardFolderResource() resource.Resource {
	return &dashboardFolderResource{}
}

type dashboardFolderResource struct {
	client *TsugaClient
}

func (r *dashboardFolderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dashboardFolderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard_folder"
}

func (r *dashboardFolderResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_dashboard_folder.DashboardFolderResourceSchema(ctx)
}

func (r *dashboardFolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *dashboardFolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_dashboard_folder.DashboardFolderModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := dashboardFolderRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPost, "/v1/dashboard-folders", requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create dashboard folder: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create dashboard folder: %s", err))
		return
	}

	resp.Diagnostics.Append(applyDashboardFolderResponse(ctx, httpResp.Body, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dashboardFolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_dashboard_folder.DashboardFolderModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/dashboard-folders/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read dashboard folder: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read dashboard folder: %s", err))
		return
	}

	resp.Diagnostics.Append(applyDashboardFolderResponse(ctx, httpResp.Body, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dashboardFolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_dashboard_folder.DashboardFolderModel
	var state resource_dashboard_folder.DashboardFolderModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := dashboardFolderRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/dashboard-folders/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodPut, apiPath, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update dashboard folder: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update dashboard folder: %s", err))
		return
	}

	resp.Diagnostics.Append(applyDashboardFolderResponse(ctx, httpResp.Body, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dashboardFolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_dashboard_folder.DashboardFolderModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/dashboard-folders/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, apiPath, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete dashboard folder: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete dashboard folder: %s", err))
			return
		}
	}
}

func dashboardFolderRequestBody(ctx context.Context, plan resource_dashboard_folder.DashboardFolderModel) (map[string]interface{}, diag.Diagnostics) {
	requestBody := map[string]interface{}{
		"name":  plan.Name.ValueString(),
		"owner": plan.Owner.ValueString(),
	}

	// Sent even when null so dropping the attribute moves the folder to top level.
	if plan.ParentFolderId.IsNull() {
		requestBody["parentFolderId"] = nil
	} else {
		requestBody["parentFolderId"] = plan.ParentFolderId.ValueString()
	}

	tags, diags := expandDashboardFolderTags(ctx, plan.Tags)
	if diags.HasError() {
		return nil, diags
	}
	if tags != nil {
		requestBody["tags"] = tags
	}

	return requestBody, diags
}

// The generated schema declares its own TagsType, so the shared resource_team-based
// helpers in tags.go cannot build values the framework accepts here.
func dashboardFolderTagsElementType(ctx context.Context) attr.Type {
	return resource_dashboard_folder.TagsType{
		ObjectType: types.ObjectType{
			AttrTypes: resource_dashboard_folder.TagsValue{}.AttributeTypes(ctx),
		},
	}
}

func expandDashboardFolderTags(ctx context.Context, tags types.List) ([]apiTag, diag.Diagnostics) {
	var diags diag.Diagnostics

	if tags.IsNull() || tags.IsUnknown() {
		return nil, diags
	}

	var tagList []resource_dashboard_folder.TagsValue
	diags.Append(tags.ElementsAs(ctx, &tagList, false)...)
	if diags.HasError() {
		return nil, diags
	}

	result := make([]apiTag, 0, len(tagList))
	for _, t := range tagList {
		result = append(result, apiTag{Key: t.Key.ValueString(), Value: t.Value.ValueString()})
	}

	return result, diags
}

func flattenDashboardFolderTags(ctx context.Context, tags []apiTag, planned types.List) (types.List, diag.Diagnostics) {
	elemType := dashboardFolderTagsElementType(ctx)

	if len(tags) == 0 {
		// Terraform rejects a null result for a config that planned an empty list.
		if !planned.IsNull() && !planned.IsUnknown() && len(planned.Elements()) == 0 {
			return planned, nil
		}
		return types.ListNull(elemType), nil
	}

	attributeTypes := resource_dashboard_folder.TagsValue{}.AttributeTypes(ctx)
	values := make([]attr.Value, 0, len(tags))
	for _, t := range tags {
		values = append(values, resource_dashboard_folder.NewTagsValueMust(attributeTypes, map[string]attr.Value{
			"key":   types.StringValue(t.Key),
			"value": types.StringValue(t.Value),
		}))
	}

	return types.ListValue(elemType, values)
}

func applyDashboardFolderResponse(ctx context.Context, body io.Reader, model *resource_dashboard_folder.DashboardFolderModel) diag.Diagnostics {
	var diags diag.Diagnostics

	raw, err := io.ReadAll(body)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return diags
	}

	var apiResp dashboardFolderAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return diags
	}

	model.Id = types.StringValue(apiResp.Data.ID)
	model.Name = types.StringValue(apiResp.Data.Name)
	model.Owner = types.StringValue(apiResp.Data.Owner)

	if apiResp.Data.ParentFolderId != "" {
		model.ParentFolderId = types.StringValue(apiResp.Data.ParentFolderId)
	} else {
		model.ParentFolderId = types.StringNull()
	}

	tags, tagDiags := flattenDashboardFolderTags(ctx, apiResp.Data.Tags, model.Tags)
	diags.Append(tagDiags...)
	if diags.HasError() {
		return diags
	}
	model.Tags = tags

	return diags
}

type dashboardFolderAPIResponse struct {
	Data struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		Owner          string   `json:"owner"`
		ParentFolderId string   `json:"parentFolderId"`
		Tags           []apiTag `json:"tags"`
	} `json:"data"`
}
