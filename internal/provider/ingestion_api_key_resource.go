package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"terraform-provider-tsuga/internal/resource_ingestion_api_key"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*ingestionApiKeyResource)(nil)
var _ resource.ResourceWithConfigure = (*ingestionApiKeyResource)(nil)
var _ resource.ResourceWithImportState = (*ingestionApiKeyResource)(nil)

func NewIngestionApiKeyResource() resource.Resource {
	return &ingestionApiKeyResource{}
}

type ingestionApiKeyResource struct {
	client *TsugaClient
}

// ingestionApiKeyModel is the Terraform state model. It mirrors
// resource_ingestion_api_key.IngestionApiKeyModel but adds the `key` field,
// which the API only returns on creation.
type ingestionApiKeyModel struct {
	Id                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Owner              types.String `tfsdk:"owner"`
	KeyLastCharacters  types.String `tfsdk:"key_last_characters"`
	Key                types.String `tfsdk:"key"`
	Tags               types.List   `tfsdk:"tags"`
	TeamOverrideFields types.List   `tfsdk:"team_override_fields"`
}

type ingestionApiKeyAPIResponse struct {
	Data struct {
		ID                 string   `json:"id"`
		Name               string   `json:"name"`
		KeyLastCharacters  string   `json:"keyLastCharacters"`
		Owner              string   `json:"owner"`
		Key                string   `json:"key"` // only present on create
		Tags               []apiTag `json:"tags"`
		TeamOverrideFields []string `json:"teamOverrideFields"`
	} `json:"data"`
}

func (r *ingestionApiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ingestionApiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ingestion_api_key"
}

func (r *ingestionApiKeyResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	base := resource_ingestion_api_key.IngestionApiKeyResourceSchema(ctx)
	base.Description = "An ingestion API key used to send telemetry data to Tsuga."
	base.Attributes["key"] = schema.StringAttribute{
		Computed:  true,
		Sensitive: true,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
		Description: "The full API key value. Only available at creation time; not retrievable afterwards.",
	}
	resp.Schema = base
}

func (r *ingestionApiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ingestionApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ingestionApiKeyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, diags := r.modelToRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPost, "/v1/ingestion-api-keys", body)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create ingestion API key: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create ingestion API key: %s", err))
		return
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp ingestionApiKeyAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	resp.Diagnostics.Append(r.apiRespToModel(ctx, &plan, apiResp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ingestionApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ingestionApiKeyModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/ingestion-api-keys/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read ingestion API key: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read ingestion API key: %s", err))
		return
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp ingestionApiKeyAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	// The API never returns the key after creation — preserve from state.
	apiResp.Data.Key = state.Key.ValueString()

	resp.Diagnostics.Append(r.apiRespToModel(ctx, &state, apiResp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ingestionApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ingestionApiKeyModel
	var state ingestionApiKeyModel

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

	apiPath := fmt.Sprintf("/v1/ingestion-api-keys/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodPut, apiPath, body)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update ingestion API key: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update ingestion API key: %s", err))
		return
	}

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp ingestionApiKeyAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	// Preserve the key from state — the API never returns it after creation.
	apiResp.Data.Key = state.Key.ValueString()

	resp.Diagnostics.Append(r.apiRespToModel(ctx, &plan, apiResp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ingestionApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ingestionApiKeyModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/ingestion-api-keys/%s", state.Id.ValueString())
	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, apiPath, map[string]interface{}{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete ingestion API key: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete ingestion API key: %s", err))
			return
		}
	}
}

func (r *ingestionApiKeyResource) modelToRequest(ctx context.Context, model ingestionApiKeyModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	body := map[string]interface{}{
		"name":  model.Name.ValueString(),
		"owner": model.Owner.ValueString(),
	}

	if !model.Tags.IsNull() && !model.Tags.IsUnknown() {
		var tagList []resource_ingestion_api_key.TagsValue
		diags.Append(model.Tags.ElementsAs(ctx, &tagList, false)...)
		if diags.HasError() {
			return nil, diags
		}
		tags := make([]apiTag, 0, len(tagList))
		for _, t := range tagList {
			tags = append(tags, apiTag{Key: t.Key.ValueString(), Value: t.Value.ValueString()})
		}
		body["tags"] = tags
	}

	if !model.TeamOverrideFields.IsNull() && !model.TeamOverrideFields.IsUnknown() {
		var fields []string
		diags.Append(model.TeamOverrideFields.ElementsAs(ctx, &fields, false)...)
		if diags.HasError() {
			return nil, diags
		}
		body["teamOverrideFields"] = fields
	}

	return body, diags
}

func (r *ingestionApiKeyResource) apiRespToModel(ctx context.Context, model *ingestionApiKeyModel, apiResp ingestionApiKeyAPIResponse) diag.Diagnostics {
	var diags diag.Diagnostics

	model.Id = types.StringValue(apiResp.Data.ID)
	model.Name = types.StringValue(apiResp.Data.Name)
	model.KeyLastCharacters = types.StringValue(apiResp.Data.KeyLastCharacters)
	model.Owner = types.StringValue(apiResp.Data.Owner)

	if apiResp.Data.Key != "" {
		model.Key = types.StringValue(apiResp.Data.Key)
	}

	// Tags
	elemType := types.ObjectType{AttrTypes: resource_ingestion_api_key.TagsValue{}.AttributeTypes(ctx)}
	if len(apiResp.Data.Tags) == 0 {
		model.Tags = types.ListNull(elemType)
	} else {
		tagVals := make([]attr.Value, 0, len(apiResp.Data.Tags))
		for _, t := range apiResp.Data.Tags {
			tagVals = append(tagVals, types.ObjectValueMust(
				resource_ingestion_api_key.TagsValue{}.AttributeTypes(ctx),
				map[string]attr.Value{
					"key":   types.StringValue(t.Key),
					"value": types.StringValue(t.Value),
				},
			))
		}
		list, d := types.ListValue(elemType, tagVals)
		diags.Append(d...)
		model.Tags = list
	}

	// TeamOverrideFields
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

