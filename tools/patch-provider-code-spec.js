#!/usr/bin/env node

/**
 * Patches provider-code-spec.json before Go code generation.
 *
 * Currently applies:
 *  - team datasource: sets `id` and `name` to computed_optional so users can
 *    look up a team by either field.
 *  - custom_usage_tag resource: adds requires_replace plan modifier to tag_key
 *    so that changing the key forces destroy + recreate (the API has no update).
 *  - team resource: adds use_state_for_unknown to `id` so an in-place update
 *    (e.g. editing the description) keeps the id known in the plan instead of
 *    showing `id -> (known after apply)`, which otherwise cascades spurious
 *    diffs onto resources that reference it (team memberships, tag policies).
 *  - ingestion_api_key resource: keeps team_override_fields computed_optional.
 *    The OpenAPI spec lists it in `required`, but `anyOf` allows null. We want
 *    to allow users to omit it in terraform, which will send null to the API.
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

// team: id is stable for the team lifetime; keep it known across in-place
// updates so dependent resources don't see spurious diffs.
const teamResource = spec.resources.find((r) => r.name === "team");
if (!teamResource) {
  console.error("team resource not found in spec");
  process.exit(1);
}
const teamIdAttr = findAttr(teamResource.schema.attributes, "id");
if (!teamIdAttr) {
  console.error("id attribute not found in team resource");
  process.exit(1);
}
teamIdAttr.string.plan_modifiers = [
  {
    custom: {
      imports: [
        {path: "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"},
      ],
      schema_definition: "stringplanmodifier.UseStateForUnknown()",
    },
  },
];

const ingestionApiKey = spec.resources.find((r) => r.name === "ingestion_api_key");
if (!ingestionApiKey) {
  console.error("ingestion_api_key resource not found in spec");
  process.exit(1);
}
const teamOverrideFieldsAttr = findAttr(
  ingestionApiKey.schema.attributes,
  "team_override_fields",
);
if (!teamOverrideFieldsAttr) {
  console.error("team_override_fields attribute not found in ingestion_api_key resource");
  process.exit(1);
}
teamOverrideFieldsAttr.list.computed_optional_required = "computed_optional";

fs.writeFileSync(specPath, JSON.stringify(spec, null, 2) + "\n");
