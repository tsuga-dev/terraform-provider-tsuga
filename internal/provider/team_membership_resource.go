package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"terraform-provider-tsuga/internal/resource_team_membership"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*teamMembershipResource)(nil)
var _ resource.ResourceWithConfigure = (*teamMembershipResource)(nil)
var _ resource.ResourceWithImportState = (*teamMembershipResource)(nil)

func NewTeamMembershipResource() resource.Resource {
	return &teamMembershipResource{}
}

type teamMembershipResource struct {
	client *TsugaClient
}

func (r *teamMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *teamMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_membership"
}

func (r *teamMembershipResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resource_team_membership.TeamMembershipResourceSchema(ctx)
}

// ImportState supports importing via "userId:teamId" composite.
func (r *teamMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			`Expected import ID in the format "userId:teamId".`,
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), parts[1])...)
	// id and role_key will be populated by Read
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "")...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role_key"), "")...)
}

func (r *teamMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resource_team_membership.TeamMembershipModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := map[string]interface{}{
		"userId":  plan.UserId.ValueString(),
		"teamId":  plan.TeamId.ValueString(),
		"roleKey": plan.RoleKey.ValueString(),
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPost, "/v1/team-memberships", requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create team membership: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create team membership: %s", err))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp teamMembershipAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	plan.Id = types.StringValue(apiResp.Data.ID)
	plan.UserId = types.StringValue(apiResp.Data.UserId)
	plan.TeamId = types.StringValue(apiResp.Data.TeamId)
	plan.RoleKey = types.StringValue(apiResp.Data.RoleKey)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state resource_team_membership.TeamMembershipModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiPath := fmt.Sprintf("/v1/team-memberships?userId=%s&teamId=%s",
		state.UserId.ValueString(),
		state.TeamId.ValueString(),
	)
	httpResp, err := r.client.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read team membership: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read team membership: %s", err))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp teamMembershipListAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	if len(apiResp.Data) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	membership := apiResp.Data[0]
	state.Id = types.StringValue(membership.ID)
	state.UserId = types.StringValue(membership.UserId)
	state.TeamId = types.StringValue(membership.TeamId)
	state.RoleKey = types.StringValue(membership.RoleKey)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *teamMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan resource_team_membership.TeamMembershipModel
	var state resource_team_membership.TeamMembershipModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := map[string]interface{}{
		"userId":  state.UserId.ValueString(),
		"teamId":  state.TeamId.ValueString(),
		"roleKey": plan.RoleKey.ValueString(),
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodPut, "/v1/team-memberships", requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update team membership: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if err := r.client.checkResponse(httpResp); err != nil {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update team membership: %s", err))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	var apiResp teamMembershipAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	plan.Id = types.StringValue(apiResp.Data.ID)
	plan.UserId = types.StringValue(apiResp.Data.UserId)
	plan.TeamId = types.StringValue(apiResp.Data.TeamId)
	plan.RoleKey = types.StringValue(apiResp.Data.RoleKey)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resource_team_membership.TeamMembershipModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requestBody := map[string]interface{}{
		"userId": state.UserId.ValueString(),
		"teamId": state.TeamId.ValueString(),
	}

	httpResp, err := r.client.doRequest(ctx, http.MethodDelete, "/v1/team-memberships", requestBody)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete team membership: %s", err))
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNotFound {
		if err := r.client.checkResponse(httpResp); err != nil {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete team membership: %s", err))
			return
		}
	}
}

type teamMembershipData struct {
	ID      string `json:"id"`
	UserId  string `json:"userId"`
	TeamId  string `json:"teamId"`
	RoleKey string `json:"roleKey"`
}

type teamMembershipAPIResponse struct {
	Data teamMembershipData `json:"data"`
}

type teamMembershipListAPIResponse struct {
	Data []teamMembershipData `json:"data"`
}
