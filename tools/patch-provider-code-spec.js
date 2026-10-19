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
 *  - dashboard_folder and monitor_folder resources: make parent_folder_id
 *    plain optional. The API returns it only for nested folders, so codegen
 *    marks it computed_optional, and terraform then carries the prior parent
 *    into the plan when the attribute is dropped, making a move back to top
 *    level unplannable.
 *    Also adds use_state_for_unknown to `id` (same reason as the team resource,
 *    and folders reference each other through parent_folder_id) and to `tags`,
 *    so a config that omits tags keeps the stored ones instead of planning
 *    `(known after apply)` on every update.
 *  - ingestion_api_key and rum_public_token resources: make
 *    team_override_fields plain optional. The OpenAPI spec lists it in
 *    `required`, but `anyOf` allows null. Omitting it in terraform sends null,
 *    which clears the fields; computed_optional would instead keep the stored
 *    value, so the fields could never be unset.
 *  - rum_public_token resource: adds requires_replace to application_type,
 *    which the API fixes at creation, and use_state_for_unknown to token,
 *    which never changes after creation.
 *  - operation_api_key resource: turns permissions into a map keyed by the
 *    API's resource types. Codegen would strip the hyphens from those keys
 *    (`dashboard-folder` becomes `dashboardfolder`), and a map also accepts
 *    resource types added to the API later.
 *    Also adds use_state_for_unknown to `id` and `key_last_characters`, which
 *    never change after creation, and to `tags`, so a config that omits tags
 *    keeps the stored ones instead of planning `(known after apply)` on every
 *    update.
 *  - operation_api_key and rum_public_token resources: declare tags with the
 *    shared resource_team tag type, as the hand-written resources do. Codegen
 *    would give each its own type, which the expandTags/flattenTags helpers
 *    cannot read.
 *  - ingestion_api_key and operation_api_key resources: mark key sensitive and
 *    add use_state_for_unknown to it. The API returns the key only on creation
 *    and never rotates it, so plans keep the stored value instead of showing
 *    `key -> (known after apply)`.
 */
const fs = require("fs");

function fail(message) {
  console.error(message);
  process.exit(1);
}

if (process.argv.length < 3) {
  fail("Usage: patch-provider-code-spec.js <provider-code-spec.json>");
}

const specPath = process.argv[2];
const spec = JSON.parse(fs.readFileSync(specPath, "utf8"));

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

function requireEntry(entries, kind, name) {
  const entry = entries.find((e) => e.name === name);
  if (!entry) fail(`${name} ${kind} not found in spec`);
  return entry;
}

const requireDatasource = (name) => requireEntry(spec.datasources, "datasource", name);
const requireResource = (name) => requireEntry(spec.resources, "resource", name);

function requireAttr(entry, name) {
  const attr = findAttr(entry.schema.attributes, name);
  if (!attr) fail(`${name} attribute not found in ${entry.name}`);
  return attr;
}

const STRING_PLAN_MODIFIER = "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier";
const LIST_PLAN_MODIFIER = "github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier";

// A single custom plan modifier, in the shape tfplugingen-framework expects.
const planModifier = (importPath, definition) => [
  {
    custom: {
      imports: [{path: importPath}],
      schema_definition: definition,
    },
  },
];

const team = requireDatasource("team");
for (const name of ["id", "name"]) {
  requireAttr(team, name).string.computed_optional_required = "computed_optional";
}

// custom_usage_tag: tag_key requires replace (no update endpoint exists)
requireAttr(requireResource("custom_usage_tag"), "tag_key").string.plan_modifiers = planModifier(
  STRING_PLAN_MODIFIER,
  "stringplanmodifier.RequiresReplace()",
);

// team: id is stable for the team lifetime; keep it known across in-place
// updates so dependent resources don't see spurious diffs.
requireAttr(requireResource("team"), "id").string.plan_modifiers = planModifier(
  STRING_PLAN_MODIFIER,
  "stringplanmodifier.UseStateForUnknown()",
);

for (const folderResourceName of ["dashboard_folder", "monitor_folder"]) {
  const folder = requireResource(folderResourceName);
  requireAttr(folder, "parent_folder_id").string.computed_optional_required = "optional";
  requireAttr(folder, "id").string.plan_modifiers = planModifier(
    STRING_PLAN_MODIFIER,
    "stringplanmodifier.UseStateForUnknown()",
  );
  requireAttr(folder, "tags").list_nested.plan_modifiers = planModifier(
    LIST_PLAN_MODIFIER,
    "listplanmodifier.UseStateForUnknown()",
  );
}

for (const keyResourceName of ["ingestion_api_key", "rum_public_token"]) {
  requireAttr(requireResource(keyResourceName), "team_override_fields").list.computed_optional_required = "optional";
}

const rumPublicToken = requireResource("rum_public_token");
requireAttr(rumPublicToken, "application_type").string.plan_modifiers = planModifier(
  STRING_PLAN_MODIFIER,
  "stringplanmodifier.RequiresReplace()",
);
requireAttr(rumPublicToken, "token").string.plan_modifiers = planModifier(
  STRING_PLAN_MODIFIER,
  "stringplanmodifier.UseStateForUnknown()",
);

const operationApiKey = requireResource("operation_api_key");
const permissions = requireAttr(operationApiKey, "permissions");
delete permissions.single_nested;
permissions.map = {
  computed_optional_required: "required",
  element_type: {string: {}},
  description:
    "Resource types the key can act on, each mapped to `read` or `write`. `write` also grants read access. " +
    "A resource type absent from the map is not accessible to the key at all. " +
    "Keys are API resource types such as `dashboard`, `dashboard-folder` or `logs`.",
  validators: [
    {
      custom: {
        imports: [
          {path: "github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"},
          {path: "github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"},
        ],
        schema_definition: 'mapvalidator.ValueStringsAre(stringvalidator.OneOf("read", "write"))',
      },
    },
  ],
};
requireAttr(operationApiKey, "tags").list_nested.plan_modifiers = planModifier(
  LIST_PLAN_MODIFIER,
  "listplanmodifier.UseStateForUnknown()",
);
for (const name of ["id", "key_last_characters"]) {
  requireAttr(operationApiKey, name).string.plan_modifiers = planModifier(
    STRING_PLAN_MODIFIER,
    "stringplanmodifier.UseStateForUnknown()",
  );
}

for (const tagsResourceName of ["operation_api_key", "rum_public_token"]) {
  requireAttr(requireResource(tagsResourceName), "tags").list_nested.nested_object.custom_type = {
    import: {path: "terraform-provider-tsuga/internal/resource_team"},
    type: "resource_team.TagsType{ObjectType: types.ObjectType{AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx)}}",
    value_type: "resource_team.TagsValue",
  };
}

for (const keyResourceName of ["ingestion_api_key", "operation_api_key"]) {
  const key = requireAttr(requireResource(keyResourceName), "key").string;
  key.sensitive = true;
  key.description = "The full API key value. Only available at creation time; not retrievable afterwards.";
  key.plan_modifiers = planModifier(STRING_PLAN_MODIFIER, "stringplanmodifier.UseStateForUnknown()");
}

fs.writeFileSync(specPath, JSON.stringify(spec, null, 2) + "\n");
