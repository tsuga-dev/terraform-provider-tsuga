package resource_custom_role

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// readOnlyAssetsValidator rejects 'write' on the asset types the API only grants 'read' on, a
// per-key rule mapvalidator.ValueStringsAre cannot express.
type readOnlyAssetsValidator struct {
	assetTypes []string
}

func readOnlyAssets(assetTypes []string) validator.Map {
	return readOnlyAssetsValidator{assetTypes: assetTypes}
}

func (v readOnlyAssetsValidator) Description(_ context.Context) string {
	return fmt.Sprintf("%s only accept 'read'", quoteList(v.assetTypes))
}

func (v readOnlyAssetsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v readOnlyAssetsValidator) ValidateMap(_ context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	for _, assetType := range v.assetTypes {
		level, ok := req.ConfigValue.Elements()[assetType].(types.String)
		if !ok || level.IsNull() || level.IsUnknown() {
			continue
		}
		if level.ValueString() == "write" {
			resp.Diagnostics.AddAttributeError(
				req.Path.AtMapKey(assetType),
				"Invalid value",
				fmt.Sprintf("'%s' only accepts 'read'", assetType),
			)
		}
	}
}

// requiredAssetsValidator rejects a map that lacks a key the spec marks as required.
type requiredAssetsValidator struct {
	keys []string
}

func requiredAssets(keys []string) validator.Map {
	return requiredAssetsValidator{keys: keys}
}

func (v requiredAssetsValidator) Description(_ context.Context) string {
	return fmt.Sprintf("%s must be set", quoteList(v.keys))
}

func (v requiredAssetsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v requiredAssetsValidator) ValidateMap(_ context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	for _, key := range v.keys {
		if _, ok := req.ConfigValue.Elements()[key]; !ok {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Missing required key",
				fmt.Sprintf("assets must grant access on '%s', as 'read' or 'write'", key),
			)
		}
	}
}

func quoteList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "'"+value+"'")
	}
	return strings.Join(quoted, ", ")
}
