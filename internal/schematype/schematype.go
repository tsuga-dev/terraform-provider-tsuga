// Package schematype precomputes nested attribute types so the framework does
// not rebuild them at every visited path.
package schematype

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func Cache(s schema.Schema) schema.Schema {
	s.Attributes = cacheAttributes(s.Attributes)
	return s
}

func cacheAttributes(in map[string]schema.Attribute) map[string]schema.Attribute {
	out := make(map[string]schema.Attribute, len(in))
	for k, a := range in {
		out[k] = cacheAttribute(a)
	}
	return out
}

func cacheObject(o schema.NestedAttributeObject) schema.NestedAttributeObject {
	o.Attributes = cacheAttributes(o.Attributes)
	if o.CustomType == nil {
		attrTypes := make(map[string]attr.Type, len(o.Attributes))
		for k, a := range o.Attributes {
			attrTypes[k] = a.GetType()
		}
		o.CustomType = types.ObjectType{AttrTypes: attrTypes}
	}
	return o
}

func cacheAttribute(a schema.Attribute) schema.Attribute {
	switch v := a.(type) {
	case schema.SingleNestedAttribute:
		v.Attributes = cacheAttributes(v.Attributes)
		if v.CustomType == nil {
			attrTypes := make(map[string]attr.Type, len(v.Attributes))
			for k, c := range v.Attributes {
				attrTypes[k] = c.GetType()
			}
			v.CustomType = types.ObjectType{AttrTypes: attrTypes}
		}
		return v
	case schema.ListNestedAttribute:
		o := cacheObject(v.NestedObject)
		v.NestedObject = o
		if v.CustomType == nil {
			v.CustomType = types.ListType{ElemType: o.CustomType}
		}
		return v
	case schema.SetNestedAttribute:
		o := cacheObject(v.NestedObject)
		v.NestedObject = o
		if v.CustomType == nil {
			v.CustomType = types.SetType{ElemType: o.CustomType}
		}
		return v
	case schema.MapNestedAttribute:
		o := cacheObject(v.NestedObject)
		v.NestedObject = o
		if v.CustomType == nil {
			v.CustomType = types.MapType{ElemType: o.CustomType}
		}
		return v
	}
	return a
}
