#!/usr/bin/env node

/**
 * Unwraps top-level {"data": {...}} envelopes in OpenAPI response schemas.
 * This is a pragmatic helper to match APIs that wrap responses in { data: ... }
 * while allowing codegen to produce flat Terraform schemas.
 */
const fs = require("fs");

if (process.argv.length < 3) {
  console.error("Usage: flatten-data.js <openapi.json>");
  process.exit(1);
}

const inputPath = process.argv[2];
const spec = JSON.parse(fs.readFileSync(inputPath, "utf8"));

const responses = (spec.paths && Object.values(spec.paths)) || [];

for (const response of responses) {
  for (const method of ["get", "post", "put", "patch", "delete"]) {
    const op = response[method];
    if (!op || !op.responses)
      continue;

    for (const resp of Object.values(op.responses)) {
      if (
        !resp.content ||
        !resp.content["application/json"] ||
        !resp.content["application/json"].schema
      ) {
        continue;
      }

      const schema = resp.content["application/json"].schema;
      if (
        schema.type === "object" &&
        schema.properties &&
        schema.properties.data
      ) {
        const dataSchema = schema.properties.data;
        // Replace the top-level schema with the inner data schema
        resp.content["application/json"].schema = dataSchema;
      }
    }
  }
}

process.stdout.write(JSON.stringify(spec, null, 2));
