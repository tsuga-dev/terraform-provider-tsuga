#!/bin/bash
set -eux

# Flatten top-level API responses by unwrapping {"data": {...}} envelopes into the top-level
# so tfplugingen generates flat schemas without duplicate nested data blocks.
TMP_SPEC="$(mktemp)"
node ./tools/flatten-data.js ./public-open-api.json >"$TMP_SPEC"

# Generate the provider code spec
tfplugingen-openapi generate \
  --config ./generator_config.yml \
  --output ./provider-code-spec.json \
  "$TMP_SPEC"

rm -f "$TMP_SPEC"

# Patch the provider code spec before Go generation
# - team datasource: sets $(id) and $(name) to computed_optional so users can look up a team by either field.
node ./tools/patch-provider-code-spec.js ./provider-code-spec.json

# Generate the resources code
tfplugingen-framework generate resources \
  --input ./provider-code-spec.json \
  --output ./internal

# Generate the data sources code
tfplugingen-framework generate data-sources \
  --input ./provider-code-spec.json \
  --output ./internal
