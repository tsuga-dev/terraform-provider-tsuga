package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestNestedSchemaTypesAreCached(t *testing.T) {
	for name, r := range map[string]resource.Resource{
		"dashboard": NewDashboardResource(),
		"monitor":   NewMonitorResource(),
	} {
		var resp resource.SchemaResponse
		r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
		checkCached(t, name, resp.Schema.Attributes)
	}
}

func checkCached(t *testing.T, path string, attrs map[string]schema.Attribute) {
	t.Helper()
	for k, a := range attrs {
		p := path + "." + k
		var (
			custom, objCustom bool
			children          map[string]schema.Attribute
		)
		switch v := a.(type) {
		case schema.SingleNestedAttribute:
			custom, objCustom, children = v.CustomType != nil, true, v.Attributes
		case schema.ListNestedAttribute:
			custom, objCustom, children = v.CustomType != nil, v.NestedObject.CustomType != nil, v.NestedObject.Attributes
		case schema.SetNestedAttribute:
			custom, objCustom, children = v.CustomType != nil, v.NestedObject.CustomType != nil, v.NestedObject.Attributes
		case schema.MapNestedAttribute:
			custom, objCustom, children = v.CustomType != nil, v.NestedObject.CustomType != nil, v.NestedObject.Attributes
		default:
			continue
		}
		if !custom || !objCustom {
			t.Errorf("%s: nested attribute type not cached", p)
		}
		checkCached(t, p, children)
	}
}
