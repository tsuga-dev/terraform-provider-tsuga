package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var folderKinds = []folderKind{dashboardFolderKind, monitorFolderKind}

func TestFolderRequestBody_ParentFolderId(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		parent types.String
		want   interface{}
	}{
		{name: "set", parent: types.StringValue("folder-123"), want: "folder-123"},
		{name: "null moves to top level", parent: types.StringNull(), want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := folderModel{
				Name:           types.StringValue("Platform"),
				Owner:          types.StringValue("team-123"),
				ParentFolderId: tc.parent,
				Tags:           types.ListNull(types.ObjectType{}),
			}

			body, diags := folderRequestBody(ctx, plan)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if body["name"] != "Platform" || body["owner"] != "team-123" {
				t.Fatalf("unexpected body: %#v", body)
			}

			got, ok := body["parentFolderId"]
			if !ok {
				t.Fatal("parentFolderId should be present")
			}
			if got != tc.want {
				t.Fatalf("parentFolderId = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// Tags built for state must round-trip back into the request body.
func TestFolderTags_RoundTrip(t *testing.T) {
	ctx := context.Background()

	for _, kind := range folderKinds {
		t.Run(kind.typeName, func(t *testing.T) {
			tags := []apiTag{{Key: "team", Value: "platform"}}
			list, diags := kind.flattenTags(ctx, tags, types.ListNull(kind.tagsElementType(ctx)))
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			got, diags := expandFolderTags(ctx, list)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if len(got) != 1 || got[0] != tags[0] {
				t.Fatalf("tags = %#v, want %#v", got, tags)
			}
		})
	}
}

func TestFolderApplyResponse(t *testing.T) {
	ctx := context.Background()
	body := `{"data":{"id":"folder-1","name":"Platform","owner":"team-1","parentFolderId":"folder-0","tags":[{"key":"team","value":"platform"}]}}`

	for _, kind := range folderKinds {
		t.Run(kind.typeName, func(t *testing.T) {
			var model folderModel
			if diags := kind.applyResponse(ctx, strings.NewReader(body), &model); diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if model.Id.ValueString() != "folder-1" || model.Name.ValueString() != "Platform" {
				t.Fatalf("unexpected model: %#v", model)
			}
			if model.Owner.ValueString() != "team-1" || model.ParentFolderId.ValueString() != "folder-0" {
				t.Fatalf("unexpected model: %#v", model)
			}
			if len(model.Tags.Elements()) != 1 {
				t.Fatalf("tags = %#v, want 1 element", model.Tags)
			}
		})
	}
}

// The framework rejects state whose list element type differs from the schema's,
// which only surfaces on apply, so assert the two agree here.
func TestFolderTagsMatchSchemaElementType(t *testing.T) {
	ctx := context.Background()

	for _, kind := range folderKinds {
		t.Run(kind.typeName, func(t *testing.T) {
			schemaTags, ok := kind.schema(ctx).Attributes["tags"].(schema.ListNestedAttribute)
			if !ok {
				t.Fatal("tags is not a ListNestedAttribute")
			}
			want := schemaTags.NestedObject.CustomType

			for _, tags := range [][]apiTag{nil, {{Key: "team", Value: "platform"}}} {
				list, diags := kind.flattenTags(ctx, tags, types.ListNull(kind.tagsElementType(ctx)))
				if diags.HasError() {
					t.Fatalf("unexpected diagnostics: %v", diags)
				}
				if got := list.ElementType(ctx); !got.Equal(want) {
					t.Fatalf("element type = %v, want %v (tags: %v)", got, want, tags)
				}
			}
		})
	}
}

// Terraform errors with "inconsistent result after apply" if a planned empty
// list comes back null, so an explicit `tags = []` has to survive the round trip.
func TestFolderFlattenTags_KeepsPlannedEmptyList(t *testing.T) {
	ctx := context.Background()

	for _, kind := range folderKinds {
		t.Run(kind.typeName, func(t *testing.T) {
			elemType := kind.tagsElementType(ctx)

			plannedEmpty, diags := types.ListValue(elemType, nil)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			got, diags := kind.flattenTags(ctx, nil, plannedEmpty)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got.IsNull() {
				t.Fatal("tags = null, want an empty list")
			}
			if len(got.Elements()) != 0 {
				t.Fatalf("tags = %#v, want an empty list", got)
			}

			// An omitted `tags` block plans as null and must stay null.
			got, diags = kind.flattenTags(ctx, nil, types.ListNull(elemType))
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if !got.IsNull() {
				t.Fatalf("tags = %#v, want null", got)
			}
		})
	}
}

func TestFolderApplyResponse_TopLevelFolderHasNullParent(t *testing.T) {
	ctx := context.Background()
	body := `{"data":{"id":"folder-1","name":"Platform","owner":"team-1","tags":[]}}`

	for _, kind := range folderKinds {
		t.Run(kind.typeName, func(t *testing.T) {
			var model folderModel
			if diags := kind.applyResponse(ctx, strings.NewReader(body), &model); diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if !model.ParentFolderId.IsNull() {
				t.Fatalf("parent_folder_id = %#v, want null", model.ParentFolderId)
			}
			if !model.Tags.IsNull() {
				t.Fatalf("tags = %#v, want null", model.Tags)
			}
		})
	}
}
