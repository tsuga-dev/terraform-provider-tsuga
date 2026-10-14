package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_dashboard_folder"
	"terraform-provider-tsuga/internal/resource_monitor_folder"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// folderKind describes one kind of folder. Every kind exposes the same API
// shape and generates the same schema, so a single resource serves them all.
type folderKind struct {
	typeName string // suffix of the Terraform type name
	apiPath  string
	noun     string // used in error messages
	schema   func(context.Context) schema.Schema
}

var dashboardFolderKind = folderKind{
	typeName: "dashboard_folder",
	apiPath:  "/v1/dashboard-folders",
	noun:     "dashboard folder",
	schema:   resource_dashboard_folder.DashboardFolderResourceSchema,
}

var monitorFolderKind = folderKind{
	typeName: "monitor_folder",
	apiPath:  "/v1/monitor-folders",
	noun:     "monitor folder",
	schema:   resource_monitor_folder.MonitorFolderResourceSchema,
}

var _ resource.Resource = (*folderResource)(nil)
var _ resource.ResourceWithConfigure = (*folderResource)(nil)
var _ resource.ResourceWithImportState = (*folderResource)(nil)

func NewDashboardFolderResource() resource.Resource {
	return &folderResource{kind: dashboardFolderKind}
}

func NewMonitorFolderResource() resource.Resource {
	return &folderResource{kind: monitorFolderKind}
}

type folderResource struct {
	kind   folderKind
	client *TsugaClient
}

// folderModel has the shape of the model generated for every folder kind.
type folderModel struct {
	Id             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Owner          types.String `tfsdk:"owner"`
	ParentFolderId types.String `tfsdk:"parent_folder_id"`
	Tags           types.List   `tfsdk:"tags"`
}

func (r *folderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *folderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.kind.typeName
}

func (r *folderResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.kind.schema(ctx)
}

func (r *folderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *folderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan folderModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := folderRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPost, r.kind.apiPath, requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create %s: %s", r.kind.noun, err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create %s: %s", r.kind.noun, err))
		return
	}

	resp.Diagnostics.Append(r.kind.applyResponse(ctx, httpResp.Body, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *folderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state folderModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodGet, r.itemPath(state), nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read %s: %s", r.kind.noun, err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read %s: %s", r.kind.noun, err))
		return
	}

	r.readJSON(ctx, httpResp.Body, resp)
}

func (r *folderResource) readJSON(ctx context.Context, bodyReader io.Reader, resp *resource.ReadResponse) {
	var state folderModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.kind.applyResponse(ctx, bodyReader, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *folderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan folderModel
	var state folderModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody, diags := folderRequestBody(ctx, plan)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPut, r.itemPath(state), requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update %s: %s", r.kind.noun, err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update %s: %s", r.kind.noun, err))
		return
	}

	resp.Diagnostics.Append(r.kind.applyResponse(ctx, httpResp.Body, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *folderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state folderModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, r.itemPath(state), map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete %s: %s", r.kind.noun, err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete %s: %s", r.kind.noun, err))
			return
		}
	}
}

func (r *folderResource) itemPath(state folderModel) string {
	return fmt.Sprintf("%s/%s", r.kind.apiPath, state.Id.ValueString())
}

func folderRequestBody(ctx context.Context, plan folderModel) (map[string]interface{}, diag.Diagnostics) {
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

	tags, diags := expandFolderTags(ctx, plan.Tags)
	if diags.HasError() {
		return nil, diags
	}
	if tags != nil {
		requestBody["tags"] = tags
	}

	return requestBody, diags
}

// The shared helpers in tags.go read and build resource_team's TagsValue, which
// the framework rejects here: each folder kind's generated schema declares its
// own TagsType. Folder tags are therefore read through a plain struct and built
// with the element type the schema declares.
type folderTag struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

func expandFolderTags(ctx context.Context, tags types.List) ([]apiTag, diag.Diagnostics) {
	var diags diag.Diagnostics

	if tags.IsNull() || tags.IsUnknown() {
		return nil, diags
	}

	var tagList []folderTag
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

func (k folderKind) tagsElementType(ctx context.Context) basetypes.ObjectTypable {
	return k.schema(ctx).Attributes["tags"].(schema.ListNestedAttribute).NestedObject.Type()
}

func (k folderKind) flattenTags(ctx context.Context, tags []apiTag, planned types.List) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := k.tagsElementType(ctx)

	if len(tags) == 0 {
		// Terraform rejects a null result for a config that planned an empty list.
		if !planned.IsNull() && !planned.IsUnknown() && len(planned.Elements()) == 0 {
			return planned, nil
		}
		return types.ListNull(elemType), nil
	}

	attributeTypes := map[string]attr.Type{"key": types.StringType, "value": types.StringType}
	values := make([]attr.Value, 0, len(tags))
	for _, t := range tags {
		object, objectDiags := types.ObjectValue(attributeTypes, map[string]attr.Value{
			"key":   types.StringValue(t.Key),
			"value": types.StringValue(t.Value),
		})
		diags.Append(objectDiags...)
		value, valueDiags := elemType.ValueFromObject(ctx, object)
		diags.Append(valueDiags...)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}
		values = append(values, value)
	}

	list, listDiags := types.ListValue(elemType, values)
	diags.Append(listDiags...)
	return list, diags
}

func (k folderKind) applyResponse(ctx context.Context, body io.Reader, model *folderModel) diag.Diagnostics {
	var diags diag.Diagnostics

	raw, err := io.ReadAll(body)
	if err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return diags
	}

	var apiResp folderAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		diags.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return diags
	}

	model.Id = types.StringValue(apiResp.Data.ID)
	model.Name = types.StringValue(apiResp.Data.Name)
	model.Owner = types.StringValue(apiResp.Data.Owner)
	model.ParentFolderId = stringValueOrNull(apiResp.Data.ParentFolderId)

	tags, tagDiags := k.flattenTags(ctx, apiResp.Data.Tags, model.Tags)
	diags.Append(tagDiags...)
	if diags.HasError() {
		return diags
	}
	model.Tags = tags

	return diags
}

type folderAPIResponse struct {
	Data struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		Owner          string   `json:"owner"`
		ParentFolderId string   `json:"parentFolderId"`
		Tags           []apiTag `json:"tags"`
	} `json:"data"`
}
