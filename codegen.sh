#!/bin/bash
set -eux

# Flatten top-level API responses by unwrapping {"data": {...}} envelopes into the top-level
# so tfplugingen generates flat schemas without duplicate nested data blocks.
TMP_SPEC="$(mktemp)"
node ./tools/flatten-data.js ./public-open-api.json >"$TMP_SPEC"

# Generate the provider code spec
go tool -modfile=tools/go.mod tfplugingen-openapi generate \
  --config ./generator_config.yml \
  --output ./provider-code-spec.json \
  "$TMP_SPEC"

rm -f "$TMP_SPEC"

# Patch the provider code spec before Go generation
# - team datasource: sets $(id) and $(name) to computed_optional so users can look up a team by either field.
node ./tools/patch-provider-code-spec.js ./provider-code-spec.json

# Generate the resources code
go tool -modfile=tools/go.mod tfplugingen-framework generate resources \
  --input ./provider-code-spec.json \
  --output ./internal

# Generate the data sources code
go tool -modfile=tools/go.mod tfplugingen-framework generate data-sources \
  --input ./provider-code-spec.json \
  --output ./internal

# tsuga_dashboard is hand-written (absent from generator_config.yml), so it gets
# no spec-derived validators from tfplugingen: generate them from the spec here.
node ./tools/gen-dashboard-validators.js ./public-open-api.json
gofmt -w ./internal/resource_dashboard/validators_gen.go ./internal/normalizer/units_gen.go

# tsuga_custom_role is hand-written as well: generate the asset type lists its
# schema validates against from the spec.
node ./tools/gen-custom-role-validators.js ./public-open-api.json
gofmt -w ./internal/resource_custom_role/validators_gen.go
