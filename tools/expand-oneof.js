#!/usr/bin/env node

/**
 * tfplugingen-openapi skips `oneOf` ("schema composition is currently not
 * supported"). This rewrites the discriminated `oneOf` schemas listed in RULES
 * into plain objects, so the generator sees them:
 *
 *  - the oneOf becomes an object with one optional property per discriminator
 *    mapping key, holding that variant minus the discriminator property;
 *  - `hoist` then lifts those properties to the top of the request body (the
 *    oneOf property itself is removed), so each variant is a top-level block;
 *  - `drop` removes request properties the API derives from the variant. They
 *    stay in the response schema, so the generator makes them computed.
 *
 * Only request bodies named in RULES are rewritten, other resources are untouched.
 */
const fs = require("fs");

if (process.argv.length < 3) {
  console.error("Usage: expand-oneof.js <openapi.json>");
  process.exit(1);
}

const spec = JSON.parse(fs.readFileSync(process.argv[2], "utf8"));

const RULES = {
  "/v1/inventory/cloud-accounts": {
    post: {hoist: "connectionSettings", drop: ["cloudType", "cloudAccountId"]},
  },
};

function deref(schema) {
  if (!schema.$ref) return schema;
  return schema.$ref
    .replace(/^#\//, "")
    .split("/")
    .reduce((node, part) => node[part], spec);
}

function expandOneOf(schema) {
  const {propertyName, mapping} = schema.discriminator;
  const properties = {};
  for (const [key, ref] of Object.entries(mapping)) {
    const variant = structuredClone(deref({$ref: ref}));
    delete variant.properties[propertyName];
    variant.required = (variant.required || []).filter((name) => name !== propertyName);
    properties[key] = variant;
  }
  return {type: "object", properties};
}

for (const [apiPath, methods] of Object.entries(RULES)) {
  for (const [method, {hoist, drop}] of Object.entries(methods)) {
    const body = spec.paths[apiPath][method].requestBody.content["application/json"].schema;
    const variants = expandOneOf(body.properties[hoist]);
    delete body.properties[hoist];
    for (const name of drop) delete body.properties[name];
    Object.assign(body.properties, variants.properties);
    body.required = (body.required || []).filter((name) => name !== hoist && !drop.includes(name));
  }
}

process.stdout.write(JSON.stringify(spec, null, 2));
