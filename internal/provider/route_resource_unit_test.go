package provider

import (
	"terraform-provider-tsuga/internal/resource_route"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandCreatorParams_Category(t *testing.T) {
	creator := &resource_route.CreatorModel{
		Category: &resource_route.CreatorCategoryModel{
			TargetAttribute: types.StringValue("severity"),
			Clauses: []resource_route.CreatorCategoryClauseModel{
				{Query: types.StringValue("status_code:>=500"), Value: types.StringValue("error")},
				{Query: types.StringValue("status_code:>=400"), Value: types.StringValue("warning")},
			},
			DefaultValue: types.StringValue("info"),
		},
	}

	params, diags := expandCreatorParams(creator)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if params["subtype"] != "category" {
		t.Fatalf("expected subtype category, got %#v", params["subtype"])
	}
	if params["targetAttribute"] != "severity" {
		t.Fatalf("expected targetAttribute severity, got %#v", params["targetAttribute"])
	}
	if params["defaultValue"] != "info" {
		t.Fatalf("expected defaultValue info, got %#v", params["defaultValue"])
	}

	clauses, ok := params["clauses"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected clauses to be []map[string]interface{}, got %T", params["clauses"])
	}
	if len(clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(clauses))
	}
	if clauses[0]["query"] != "status_code:>=500" || clauses[0]["value"] != "error" {
		t.Fatalf("unexpected first clause: %#v", clauses[0])
	}
}

func TestExpandCreatorParams_CategoryOmitsUnsetDefaultValue(t *testing.T) {
	creator := &resource_route.CreatorModel{
		Category: &resource_route.CreatorCategoryModel{
			TargetAttribute: types.StringValue("severity"),
			Clauses: []resource_route.CreatorCategoryClauseModel{
				{Query: types.StringValue("status_code:>=500"), Value: types.StringValue("error")},
			},
			DefaultValue: types.StringNull(),
		},
	}

	params, diags := expandCreatorParams(creator)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if _, present := params["defaultValue"]; present {
		t.Fatalf("expected defaultValue to be omitted when null, got %#v", params["defaultValue"])
	}
}
