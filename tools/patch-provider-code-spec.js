#!/usr/bin/env node

/**
 * Patches provider-code-spec.json before Go code generation.
 *
 * Currently applies:
 *  - team datasource: sets `id` and `name` to computed_optional so users can
 *    look up a team by either field.
 *  - custom_usage_tag resource: adds requires_replace plan modifier to tag_key
 *    so that changing the key forces destroy + recreate (the API has no update).
 */
const fs = require("fs");

if (process.argv.length < 3) {
  console.error("Usage: patch-provider-code-spec.js <provider-code-spec.json>");
  process.exit(1);
}

const specPath = process.argv[2];
const spec = JSON.parse(fs.readFileSync(specPath, "utf8"));

const team = spec.datasources.find((d) => d.name === "team");
if (!team) {
  console.error("team datasource not found in spec");
  process.exit(1);
}

/**
 * Recursively find an attribute by name in the schema (checks top-level and
 * nested attributes inside single_nested objects).
 */
function findAttr(attributes, name) {
  for (const attr of attributes) {
    if (attr.name === name) return attr;
    const nested = attr.single_nested?.attributes;
    if (nested) {
      const found = findAttr(nested, name);
      if (found) return found;
    }
  }
  return null;
}

for (const name of ["id", "name"]) {
  const attr = findAttr(team.schema.attributes, name);
  if (!attr) {
    console.error(`attribute "${name}" not found in team datasource`);
    process.exit(1);
  }
  attr.string.computed_optional_required = "computed_optional";
}

// custom_usage_tag: tag_key requires replace (no update endpoint exists)
const customUsageTag = spec.resources.find((r) => r.name === "custom_usage_tag");
if (!customUsageTag) {
  console.error("custom_usage_tag resource not found in spec");
  process.exit(1);
}
const tagKeyAttr = findAttr(customUsageTag.schema.attributes, "tag_key");
if (!tagKeyAttr) {
  console.error("tag_key attribute not found in custom_usage_tag resource");
  process.exit(1);
}
tagKeyAttr.string.plan_modifiers = [
  {
    custom: {
      imports: [
        {path: "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"},
      ],
      schema_definition: "stringplanmodifier.RequiresReplace()",
    },
  },
];

fs.writeFileSync(specPath, JSON.stringify(spec, null, 2) + "\n");
