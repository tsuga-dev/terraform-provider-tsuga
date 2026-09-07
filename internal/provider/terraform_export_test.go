package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type terraformExportFixture struct {
	Name         string          `json:"name"`
	ResourceType string          `json:"resourceType"`
	Resource     json.RawMessage `json:"resource"`
}

func TestTerraformExportImportRefresh(t *testing.T) {
	data, err := os.ReadFile("testdata/terraform-export/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []terraformExportFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			payload := fixture.Resource
			if fixture.ResourceType == "tsuga_team_membership" {
				payload = append(append([]byte{'['}, payload...), ']')
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet {
					t.Errorf("unexpected mutation: %s", request.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(append(append([]byte(`{"data":`), payload...), '}'))
			}))
			defer server.Close()
			r := terraformExportResources(ctx)[fixture.ResourceType]
			configurable, ok := r.(resource.ResourceWithConfigure)
			if !ok {
				t.Fatal("resource does not support Configure")
			}
			var configure resource.ConfigureResponse
			configurable.Configure(ctx, resource.ConfigureRequest{ProviderData: &TsugaClient{BaseURL: server.URL}}, &configure)
			if configure.Diagnostics.HasError() {
				t.Fatal(configure.Diagnostics)
			}
			var sr resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &sr)
			var input map[string]json.RawMessage
			if err := json.Unmarshal(fixture.Resource, &input); err != nil {
				t.Fatal(err)
			}
			var id string
			if err := json.Unmarshal(input["id"], &id); err != nil {
				t.Fatal(err)
			}
			if fixture.ResourceType == "tsuga_team_membership" {
				id = "user-123:team-123"
			}
			importable, ok := r.(resource.ResourceWithImportState)
			if !ok {
				if fixture.ResourceType != "tsuga_cloud_account" {
					t.Fatal("resource does not support import")
				}
				return
			}
			imported := resource.ImportStateResponse{State: tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}}
			importable.ImportState(ctx, resource.ImportStateRequest{ID: id}, &imported)
			if imported.Diagnostics.HasError() {
				t.Fatal(imported.Diagnostics)
			}
			assertStateString := func(state tfsdk.State, attribute, want string) {
				var value types.String
				diags := state.GetAttribute(ctx, path.Root(attribute), &value)
				if diags.HasError() {
					t.Fatal(diags)
				}
				if value.IsNull() || value.IsUnknown() || value.ValueString() != want {
					t.Fatalf("expected %s=%q, got %q", attribute, want, value.ValueString())
				}
			}
			if fixture.ResourceType == "tsuga_team_membership" {
				assertStateString(imported.State, "user_id", "user-123")
				assertStateString(imported.State, "team_id", "team-123")
			} else {
				assertStateString(imported.State, "id", id)
			}
			response := resource.ReadResponse{State: imported.State}
			r.Read(ctx, resource.ReadRequest{State: imported.State}, &response)
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
			if response.State.Raw.IsNull() {
				t.Fatal("refresh removed an existing resource")
			}
			var expectedID string
			if err := json.Unmarshal(input["id"], &expectedID); err != nil {
				t.Fatal(err)
			}
			assertStateString(response.State, "id", expectedID)
		})
	}
}

func TestTerraformExportFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/terraform-export/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []terraformExportFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, fixture := range fixtures {
		covered[fixture.ResourceType] = true
		t.Run(fixture.Name, func(t *testing.T) {
			req := TerraformExportRequest{ResourceType: fixture.ResourceType, Resource: fixture.Resource, ResourceName: strings.ReplaceAll(fixture.Name, "-", "_"), IncludeImport: fixture.ResourceType != "tsuga_cloud_account"}
			result, err := ExportTerraform(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			parsed, diags := hclsyntax.ParseConfig([]byte(result.HCL), "export.tf", hcl.InitialPos)
			if diags.HasErrors() {
				t.Fatal(diags.Error())
			}
			body, ok := parsed.Body.(*hclsyntax.Body)
			if !ok {
				t.Fatal("expected HCL body")
			}
			if body.Blocks[0].Type != "resource" {
				t.Fatal("expected resource block first")
			}
			if _, exists := body.Blocks[0].Body.Attributes["id"]; exists {
				t.Fatal("computed resource id exported")
			}
			if result.HCL != string(hclwrite.Format([]byte(result.HCL))) {
				t.Fatal("HCL is not formatted")
			}
			if strings.Contains(result.HCL, "never-export-this-secret") {
				t.Fatal("export leaked a secret")
			}
			for _, variable := range result.Variables {
				if !strings.Contains(result.HCL, "var."+variable.Name) {
					t.Fatalf("unused input variable %s", variable.Name)
				}
			}
			path := filepath.Join("testdata", "terraform-export", fixture.Name+".tf")
			if os.Getenv("UPDATE_TERRAFORM_EXPORT") == "1" {
				if err := os.WriteFile(path, []byte(result.HCL), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(want) != result.HCL {
				t.Fatalf("export differs from %s:\n%s", path, result.HCL)
			}
			again, err := ExportTerraform(context.Background(), req)
			if err != nil || !reflect.DeepEqual(result, again) {
				t.Fatalf("export is not deterministic: %v", err)
			}
		})
	}
	names, err := TerraformResourceTypes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	for name := range covered {
		actual = append(actual, name)
	}
	sort.Strings(actual)
	if !reflect.DeepEqual(names, actual) {
		t.Fatalf("every provider resource needs a fixture; registry %v, fixtures %v", names, actual)
	}
}

func TestTerraformExportErrors(t *testing.T) {
	for _, tc := range []struct {
		name, resourceType, resource, resourceName string
		includeImport                              bool
	}{
		{"unsupported", "tsuga_missing", `{}`, "", false},
		{"null", "tsuga_team", `null`, "", false},
		{"array", "tsuga_team", `[]`, "", false},
		{"malformed", "tsuga_team", `{`, "", false},
		{"invalid-name", "tsuga_team", `{"name":"test"}`, "bad.name", false},
		{"missing-import-id", "tsuga_team", `{"name":"test","visibility":"public"}`, "", true},
		{"unknown-union", "tsuga_monitor", `{"configuration":{"type":"unsupported"}}`, "", false},
		{"invalid-field-type", "tsuga_team", `{"name":123}`, "", false},
		{"unknown-cloud", "tsuga_cloud_account", `{"cloudType":"unknown"}`, "", false},
		{"unsupported-import", "tsuga_cloud_account", `{"id":"cloud","cloudType":"aws","cloudAccountId":"123"}`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ExportTerraform(context.Background(), TerraformExportRequest{ResourceType: tc.resourceType, Resource: json.RawMessage(tc.resource), ResourceName: tc.resourceName, IncludeImport: tc.includeImport})
			if err == nil {
				t.Fatal("expected an error")
			}
			if result.HCL != "" {
				t.Fatal("returned partial HCL on error")
			}
		})
	}
}
