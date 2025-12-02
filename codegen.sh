#!/bin/bash
set -eux


# Flatten top-level API responses by unwrapping {"data": {...}} envelopes into the top-level
# so tfplugingen generates flat schemas without duplicate nested data blocks.
TMP_SPEC="$(mktemp)"
node ./tools/flatten-data.js ./public-open-api.json > "$TMP_SPEC"

# Generate the provider code spec
tfplugingen-openapi generate \
  --config ./generator_config.yml \
  --output ./provider-code-spec.json \
  "$TMP_SPEC"

# Generate the resources code
tfplugingen-framework generate resources \
  --input ./provider-code-spec.json \
  --output ./internal

rm -f "$TMP_SPEC"
