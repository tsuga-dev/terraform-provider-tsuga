package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zclconf/go-cty/cty"
)

type TerraformExportRequest struct {
	ResourceType  string          `json:"resourceType"`
	Resource      json.RawMessage `json:"resource"`
	ResourceName  string          `json:"resourceName,omitempty"`
	IncludeImport bool            `json:"includeImport,omitempty"`
}

type TerraformExportVariable struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Sensitive bool   `json:"sensitive"`
}

type TerraformExportResult struct {
	HCL       string                    `json:"hcl"`
	Variables []TerraformExportVariable `json:"variables"`
	Warnings  []string                  `json:"warnings"`
}

// jsonResource shares exactly the decoding used by Read
type jsonResource interface {
	resource.Resource
	readJSON(context.Context, io.Reader, *resource.ReadResponse)
}

func terraformExportResources(ctx context.Context) map[string]resource.Resource {
	resources := make(map[string]resource.Resource)
	for _, factory := range (&tsugaProvider{}).Resources(ctx) {
		r := factory()
		var metadata resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "tsuga"}, &metadata)
		resources[metadata.TypeName] = r
	}
	return resources
}

func TerraformResourceTypes(ctx context.Context) ([]string, error) {
	resources := terraformExportResources(ctx)
	names := make([]string, 0, len(resources))
	for name, r := range resources {
		if _, ok := r.(jsonResource); !ok {
			return nil, fmt.Errorf("%s does not implement JSON export", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func ExportTerraform(ctx context.Context, req TerraformExportRequest) (TerraformExportResult, error) {
	result := TerraformExportResult{Variables: []TerraformExportVariable{}, Warnings: []string{}}
	name := req.ResourceName
	if name == "" {
		name = "exported"
	}
	if !hclsyntax.ValidIdentifier(name) {
		return result, fmt.Errorf("invalid Terraform resource name %q", name)
	}
	r, ok := terraformExportResources(ctx)[req.ResourceType].(jsonResource)
	if !ok {
		return result, fmt.Errorf("unsupported Terraform resource %q", req.ResourceType)
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(req.Resource, &input); err != nil || len(input) == 0 {
		return result, fmt.Errorf("resource must be a non-empty JSON object")
	}
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	if err := exportDiagnostics(sr.Diagnostics); err != nil {
		return result, err
	}
	if len(sr.Schema.Blocks) != 0 {
		return result, fmt.Errorf("%s uses blocks unsupported by the exporter", req.ResourceType)
	}
	stateType := sr.Schema.Type().TerraformType(ctx)
	objectType, ok := stateType.(tftypes.Object)
	if !ok {
		return result, fmt.Errorf("resource schema must be an object")
	}
	empty := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for key, typ := range objectType.AttributeTypes {
		empty[key] = tftypes.NewValue(typ, nil)
	}
	response := resource.ReadResponse{State: tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(stateType, empty)}}
	data := req.Resource
	// Membership Read receives a query response rather than a single-resource response.
	if req.ResourceType == "tsuga_team_membership" {
		data = append(append([]byte{'['}, data...), ']')
	}
	envelope, err := json.Marshal(struct {
		Data json.RawMessage `json:"data"`
	}{data})
	if err != nil {
		return result, err
	}
	r.readJSON(ctx, bytes.NewReader(envelope), &response)
	if err := exportDiagnostics(response.Diagnostics); err != nil {
		return result, err
	}
	for _, d := range response.Diagnostics {
		result.Warnings = append(result.Warnings, d.Summary()+": "+d.Detail())
	}
	var values map[string]tftypes.Value
	if err := response.State.Raw.As(&values); err != nil {
		return result, fmt.Errorf("decode resource state: %w", err)
	}
	if req.ResourceType == "tsuga_cloud_account" {
		if err := exportCloudConnection(values, objectType); err != nil {
			return result, err
		}
	}
	if validator, ok := r.(resource.ResourceWithValidateConfig); ok {
		var validation resource.ValidateConfigResponse
		validator.ValidateConfig(ctx, resource.ValidateConfigRequest{
			Config: tfsdk.Config{Schema: sr.Schema, Raw: tftypes.NewValue(stateType, values)},
		}, &validation)
		if err := exportDiagnostics(validation.Diagnostics); err != nil {
			return result, err
		}
	}
	f := hclwrite.NewEmptyFile()
	writer := terraformExportWriter{file: f, result: &result, prefix: req.ResourceType + "_" + name}
	block := f.Body().AppendNewBlock("resource", []string{req.ResourceType, name})
	for _, key := range sortedExportKeys(sr.Schema.Attributes) {
		tokens, err := writer.attribute(values[key], sr.Schema.Attributes[key], []string{key})
		if err != nil {
			return result, err
		}
		if tokens != nil {
			block.Body().SetAttributeRaw(key, tokens)
		}
	}
	if req.IncludeImport {
		if _, ok := r.(resource.ResourceWithImportState); !ok {
			return result, fmt.Errorf("%s does not support Terraform import", req.ResourceType)
		}
		id, err := terraformExportImportID(req.ResourceType, values)
		if err != nil {
			return result, err
		}
		f.Body().AppendNewline()
		importBlock := f.Body().AppendNewBlock("import", nil)
		importBlock.Body().SetAttributeTraversal("to", hcl.Traversal{hcl.TraverseRoot{Name: req.ResourceType}, hcl.TraverseAttr{Name: name}})
		importBlock.Body().SetAttributeValue("id", cty.StringVal(id))
	}
	result.HCL = string(f.Bytes())
	return result, nil
}

func exportDiagnostics(diags diag.Diagnostics) error {
	if !diags.HasError() {
		return nil
	}
	var messages []string
	for _, d := range diags.Errors() {
		messages = append(messages, d.Summary()+": "+d.Detail())
	}
	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

func terraformExportImportID(resourceType string, values map[string]tftypes.Value) (string, error) {
	keys := []string{"id"}
	if resourceType == "tsuga_team_membership" {
		keys = []string{"user_id", "team_id"}
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		var value string
		if err := values[key].As(&value); err != nil || value == "" {
			return "", fmt.Errorf("%s is required to include an import block", key)
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, ":"), nil
}

// Cloud account responses expose the account identity, but not the connection
// settings. Preserve that identity and request the remaining inputs as variables.
func exportCloudConnection(values map[string]tftypes.Value, objectType tftypes.Object) error {
	var cloud, id string
	if err := values["cloud_type"].As(&cloud); err != nil {
		return err
	}
	if err := values["cloud_account_id"].As(&id); err != nil {
		return err
	}
	identity := map[string]string{"aws": "account_id", "gcp": "project_id", "azure": "subscription_id"}[cloud]
	if identity == "" {
		return fmt.Errorf("unsupported cloud account type %q", cloud)
	}
	typ, ok := objectType.AttributeTypes[cloud].(tftypes.Object)
	if !ok {
		return fmt.Errorf("missing %s connection schema", cloud)
	}
	fields := make(map[string]tftypes.Value, len(typ.AttributeTypes))
	for key, fieldType := range typ.AttributeTypes {
		fields[key] = tftypes.NewValue(fieldType, tftypes.UnknownValue)
	}
	fields[identity] = tftypes.NewValue(tftypes.String, id)
	values[cloud] = tftypes.NewValue(typ, fields)
	return nil
}

type terraformExportWriter struct {
	file   *hclwrite.File
	result *TerraformExportResult
	prefix string
}

func (w *terraformExportWriter) attribute(value tftypes.Value, attribute schema.Attribute, path []string) (hclwrite.Tokens, error) {
	if !attribute.IsRequired() && !attribute.IsOptional() {
		return nil, nil
	}
	if value.IsKnown() && value.IsNull() && !attribute.IsRequired() {
		return nil, nil
	}
	if attribute.IsSensitive() || !value.IsKnown() {
		return w.variable(value.Type(), path, attribute.IsSensitive())
	}
	if value.IsNull() {
		// The provider normalizes empty collections to null in several Read paths.
		switch value.Type().(type) {
		case tftypes.List, tftypes.Set:
			return hclwrite.TokensForTuple(nil), nil
		case tftypes.Map:
			return hclwrite.TokensForObject(nil), nil
		default:
			return nil, fmt.Errorf("missing required configuration at %s", strings.Join(path, "."))
		}
	}
	switch a := attribute.(type) {
	case schema.SingleNestedAttribute:
		return w.object(value, a.Attributes, path)
	case schema.ListNestedAttribute:
		return w.collection(value, a.NestedObject.Attributes, path)
	case schema.SetNestedAttribute:
		return w.collection(value, a.NestedObject.Attributes, path)
	case schema.MapNestedAttribute:
		var entries map[string]tftypes.Value
		if err := value.As(&entries); err != nil {
			return nil, err
		}
		var tokens []hclwrite.ObjectAttrTokens
		for _, key := range sortedExportKeys(entries) {
			v, err := w.object(entries[key], a.NestedObject.Attributes, appendExportPath(path, key))
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, hclwrite.ObjectAttrTokens{Name: hclwrite.TokensForValue(cty.StringVal(key)), Value: v})
		}
		return hclwrite.TokensForObject(tokens), nil
	default:
		return exportLiteral(value)
	}
}

func (w *terraformExportWriter) object(value tftypes.Value, attributes map[string]schema.Attribute, path []string) (hclwrite.Tokens, error) {
	var entries map[string]tftypes.Value
	if err := value.As(&entries); err != nil {
		return nil, err
	}
	var tokens []hclwrite.ObjectAttrTokens
	for _, key := range sortedExportKeys(attributes) {
		v, err := w.attribute(entries[key], attributes[key], appendExportPath(path, key))
		if err != nil {
			return nil, err
		}
		if v != nil {
			tokens = append(tokens, hclwrite.ObjectAttrTokens{Name: hclwrite.TokensForIdentifier(key), Value: v})
		}
	}
	return hclwrite.TokensForObject(tokens), nil
}

func (w *terraformExportWriter) collection(value tftypes.Value, attributes map[string]schema.Attribute, path []string) (hclwrite.Tokens, error) {
	var elements []tftypes.Value
	if err := value.As(&elements); err != nil {
		return nil, err
	}
	var tokens []hclwrite.Tokens
	for i, element := range elements {
		v, err := w.object(element, attributes, appendExportPath(path, fmt.Sprint(i)))
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, v)
	}
	return hclwrite.TokensForTuple(tokens), nil
}

func (w *terraformExportWriter) variable(typ tftypes.Type, path []string, sensitive bool) (hclwrite.Tokens, error) {
	// Include an ordinal so distinct paths cannot collide after sanitization.
	name := fmt.Sprintf("%s_input_%d", w.prefix, len(w.result.Variables)+1)
	ctyType, err := terraformExportCtyType(typ)
	if err != nil {
		return nil, err
	}
	typeTokens, diags := hclsyntax.LexExpression([]byte(typeexpr.TypeString(ctyType)), "", hcl.InitialPos)
	if diags.HasErrors() {
		return nil, fmt.Errorf("invalid variable type: %s", diags.Error())
	}
	var tokens hclwrite.Tokens
	for _, t := range typeTokens {
		if t.Type != hclsyntax.TokenEOF {
			tokens = append(tokens, &hclwrite.Token{Type: t.Type, Bytes: t.Bytes})
		}
	}
	w.file.Body().AppendNewline()
	block := w.file.Body().AppendNewBlock("variable", []string{name})
	block.Body().SetAttributeRaw("type", tokens)
	block.Body().SetAttributeValue("description", cty.StringVal("Supply "+strings.Join(path, ".")+" for the exported resource."))
	block.Body().SetAttributeValue("nullable", cty.BoolVal(false))
	if sensitive {
		block.Body().SetAttributeValue("sensitive", cty.BoolVal(true))
	}
	w.result.Variables = append(w.result.Variables, TerraformExportVariable{Name: name, Path: strings.Join(path, "."), Sensitive: sensitive})
	return hclwrite.TokensForTraversal(hcl.Traversal{hcl.TraverseRoot{Name: "var"}, hcl.TraverseAttr{Name: name}}), nil
}

func terraformExportCtyType(typ tftypes.Type) (cty.Type, error) {
	switch {
	case typ.Is(tftypes.String):
		return cty.String, nil
	case typ.Is(tftypes.Bool):
		return cty.Bool, nil
	case typ.Is(tftypes.Number):
		return cty.Number, nil
	case typ.Is(tftypes.DynamicPseudoType):
		return cty.DynamicPseudoType, nil
	}
	switch typ := typ.(type) {
	case tftypes.List:
		element, err := terraformExportCtyType(typ.ElementType)
		if err != nil {
			return cty.NilType, err
		}
		return cty.List(element), nil
	case tftypes.Set:
		element, err := terraformExportCtyType(typ.ElementType)
		if err != nil {
			return cty.NilType, err
		}
		return cty.Set(element), nil
	case tftypes.Map:
		element, err := terraformExportCtyType(typ.ElementType)
		if err != nil {
			return cty.NilType, err
		}
		return cty.Map(element), nil
	case tftypes.Tuple:
		elements := make([]cty.Type, 0, len(typ.ElementTypes))
		for _, elementType := range typ.ElementTypes {
			element, err := terraformExportCtyType(elementType)
			if err != nil {
				return cty.NilType, err
			}
			elements = append(elements, element)
		}
		return cty.Tuple(elements), nil
	case tftypes.Object:
		attributes := make(map[string]cty.Type, len(typ.AttributeTypes))
		for name, attributeType := range typ.AttributeTypes {
			attribute, err := terraformExportCtyType(attributeType)
			if err != nil {
				return cty.NilType, err
			}
			attributes[name] = attribute
		}
		return cty.Object(attributes), nil
	default:
		return cty.NilType, fmt.Errorf("unsupported Terraform variable type %s", typ)
	}
}

func exportLiteral(value tftypes.Value) (hclwrite.Tokens, error) {
	if value.IsNull() {
		return hclwrite.TokensForValue(cty.NullVal(cty.DynamicPseudoType)), nil
	}
	if !value.IsKnown() {
		return nil, fmt.Errorf("unexpected unknown literal")
	}
	switch typ := value.Type(); {
	case typ.Is(tftypes.String):
		var v string
		if err := value.As(&v); err != nil {
			return nil, err
		}
		return hclwrite.TokensForValue(cty.StringVal(v)), nil
	case typ.Is(tftypes.Bool):
		var v bool
		if err := value.As(&v); err != nil {
			return nil, err
		}
		return hclwrite.TokensForValue(cty.BoolVal(v)), nil
	case typ.Is(tftypes.Number):
		var v big.Float
		if err := value.As(&v); err != nil {
			return nil, err
		}
		return hclwrite.TokensForValue(cty.NumberVal(&v)), nil
	}
	switch value.Type().(type) {
	case tftypes.Object, tftypes.Map:
		var entries map[string]tftypes.Value
		if err := value.As(&entries); err != nil {
			return nil, err
		}
		var tokens []hclwrite.ObjectAttrTokens
		for _, key := range sortedExportKeys(entries) {
			v, err := exportLiteral(entries[key])
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, hclwrite.ObjectAttrTokens{Name: hclwrite.TokensForValue(cty.StringVal(key)), Value: v})
		}
		return hclwrite.TokensForObject(tokens), nil
	case tftypes.List, tftypes.Set, tftypes.Tuple:
		var elements []tftypes.Value
		if err := value.As(&elements); err != nil {
			return nil, err
		}
		var tokens []hclwrite.Tokens
		for _, element := range elements {
			v, err := exportLiteral(element)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, v)
		}
		return hclwrite.TokensForTuple(tokens), nil
	default:
		return nil, fmt.Errorf("unsupported Terraform value type %s", value.Type())
	}
}

func sortedExportKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func appendExportPath(path []string, key string) []string {
	return append(append([]string{}, path...), key)
}
