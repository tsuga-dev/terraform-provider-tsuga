#!/bin/bash
set -eux

# Flatten top-level API responses by unwrapping {"data": {...}} envelopes into the top-level
# so tfplugingen generates flat schemas without duplicate nested data blocks.
TMP_SPEC="$(mktemp)"
node ./tools/flatten-data.js ./public-open-api.json >"$TMP_SPEC"

# Expand the discriminated oneOf request bodies tfplugingen cannot read into plain objects.
TMP_EXPANDED="$(mktemp)"
node ./tools/expand-oneof.js "$TMP_SPEC" >"$TMP_EXPANDED"
mv "$TMP_EXPANDED" "$TMP_SPEC"

# Generate the provider code spec
go tool -modfile=tools/go.mod tfplugingen-openapi generate \
	--config ./generator_config.yml \
	--output ./provider-code-spec.json \
	"$TMP_SPEC"

# Patch the provider code spec before Go generation
# - team datasource: sets $(id) and $(name) to computed_optional so users can look up a team by either field.
node ./tools/patch-provider-code-spec.js ./provider-code-spec.json "$TMP_SPEC"
rm -f "$TMP_SPEC"

# Generate the resources code
go tool -modfile=tools/go.mod tfplugingen-framework generate resources \
	--input ./provider-code-spec.json \
	--output ./internal

# Upstream tfplugingen-framework template bug: Object attributes with a custom_type get
# types.ObjectValue(v.X.AttributeTypes(ctx), v.X.Attributes()), which errors "Missing Object Attribute
# Value" when X is null/unknown (the SingleNested template handles it). Make it null/unknown-safe.
perl -0pi -e 's/(\w+)Val, d := types\.ObjectValue\(v\.Renotify\.AttributeTypes\(ctx\), v\.Renotify\.Attributes\(\)\)/var $1Val basetypes.ObjectValue\n\tvar d diag.Diagnostics\n\tswitch {\n\tcase v.Renotify.IsNull():\n\t\t$1Val = types.ObjectNull(RenotifyValue{}.AttributeTypes(ctx))\n\tcase v.Renotify.IsUnknown():\n\t\t$1Val = types.ObjectUnknown(RenotifyValue{}.AttributeTypes(ctx))\n\tdefault:\n\t\t$1Val, d = types.ObjectValue(RenotifyValue{}.AttributeTypes(ctx), v.Renotify.Attributes())\n\t}/g' ./internal/resource_notification_rule/notification_rule_resource_gen.go
gofmt -w ./internal/resource_notification_rule/notification_rule_resource_gen.go

# Generate the data sources code
go tool -modfile=tools/go.mod tfplugingen-framework generate data-sources \
	--input ./provider-code-spec.json \
	--output ./internal

# Generate the API types of the notification rules (request and response bodies, discriminated
# target configs) from the unmodified spec, see oapi-codegen.yml.
go tool -modfile=tools/go.mod oapi-codegen --config ./oapi-codegen.yml ./public-open-api.json

# tsuga_dashboard is hand-written (absent from generator_config.yml), so it gets
# no spec-derived validators from tfplugingen: generate them from the spec here.
node ./tools/gen-dashboard-validators.js ./public-open-api.json
gofmt -w ./internal/resource_dashboard/validators_gen.go ./internal/normalizer/units_gen.go

# tsuga_custom_role is hand-written as well: generate the asset type lists its
# schema validates against from the spec.
node ./tools/gen-custom-role-validators.js ./public-open-api.json
gofmt -w ./internal/resource_custom_role/validators_gen.go
