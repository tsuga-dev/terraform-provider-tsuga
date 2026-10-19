package provider

import (
	"context"
	"testing"

	"terraform-provider-tsuga/internal/resource_operation_api_key"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestOperationApiKeyRequestBody(t *testing.T) {
	plan := resource_operation_api_key.OperationApiKeyModel{
		Name:        types.StringValue("terraform"),
		Owner:       types.StringValue("team-123"),
		Scope:       types.StringValue("owning-team-only"),
		Permissions: types.MapValueMust(types.StringType, map[string]attr.Value{"dashboard-folder": types.StringValue("read")}),
		Tags:        types.ListNull(types.ObjectType{}),
		ExpiresAt:   types.StringValue("2027-01-01T00:00:00Z"),
	}

	body, diags := (&operationApiKeyResource{}).modelToRequest(context.Background(), plan)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if body["expiresAt"] != "2027-01-01T00:00:00Z" {
		t.Fatalf("expiresAt = %#v, want the configured value", body["expiresAt"])
	}
	if permissions := body["permissions"].(map[string]string); permissions["dashboard-folder"] != "read" {
		t.Fatalf("unexpected permissions: %#v", permissions)
	}
	if tags, ok := body["tags"]; ok {
		t.Fatalf("tags = %#v, want no tags field when the plan has no tags", tags)
	}
}

func TestOperationApiKeyApiRespToModel_ExpiresAt(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		configured types.String
		returned   string
		want       types.String
	}{
		{name: "keeps configured spelling of the same instant", configured: types.StringValue("2027-01-01T00:00:00Z"), returned: "2027-01-01T00:00:00.000Z", want: types.StringValue("2027-01-01T00:00:00Z")},
		{name: "takes a different instant from the API", configured: types.StringValue("2027-01-01T00:00:00Z"), returned: "2027-02-01T00:00:00.000Z", want: types.StringValue("2027-02-01T00:00:00.000Z")},
		{name: "imports the API value", configured: types.StringNull(), returned: "2027-01-01T00:00:00.000Z", want: types.StringValue("2027-01-01T00:00:00.000Z")},
		{name: "no expiration", configured: types.StringValue("2027-01-01T00:00:00Z"), returned: "", want: types.StringNull()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := resource_operation_api_key.OperationApiKeyModel{ExpiresAt: tc.configured, Key: types.StringValue("secret")}
			var apiResp operationApiKeyAPIResponse
			apiResp.Data.ID = "key-123"
			apiResp.Data.ExpiresAt = tc.returned

			diags := (&operationApiKeyResource{}).apiRespToModel(ctx, &model, apiResp)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if !model.ExpiresAt.Equal(tc.want) {
				t.Fatalf("expires_at = %s, want %s", model.ExpiresAt, tc.want)
			}
			if model.Key.ValueString() != "secret" {
				t.Fatalf("key = %q, want it preserved when the API omits it", model.Key.ValueString())
			}
		})
	}
}

func TestOperationApiKeyApiRespToModel_EmptyTags(t *testing.T) {
	ctx := context.Background()
	elemType := types.ObjectType{AttrTypes: resource_operation_api_key.TagsValue{}.AttributeTypes(ctx)}

	tests := []struct {
		name       string
		configured types.List
		wantNull   bool
	}{
		{name: "configured empty list stays empty", configured: types.ListValueMust(elemType, nil), wantNull: false},
		{name: "omitted tags stay null", configured: types.ListNull(elemType), wantNull: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model := resource_operation_api_key.OperationApiKeyModel{Tags: tc.configured}
			var apiResp operationApiKeyAPIResponse
			apiResp.Data.ID = "key-123"

			diags := (&operationApiKeyResource{}).apiRespToModel(ctx, &model, apiResp)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if model.Tags.IsNull() != tc.wantNull || len(model.Tags.Elements()) != 0 {
				t.Fatalf("tags = %s, want null=%t and no elements", model.Tags, tc.wantNull)
			}
		})
	}
}
