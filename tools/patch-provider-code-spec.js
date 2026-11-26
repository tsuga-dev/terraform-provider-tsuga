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
 *  - notification_rule and adaptive_notification_rule resources (their target configs are
 *    expanded from a discriminated oneOf by expand-oneof.js): the oneOf blocks and the free-text
 *    fields the response echoes (query_string, teams, jira statuses, monitor_ids, slo_ids) are
 *    plain optional, since computed_optional would plan unselected blocks as `(known after apply)`
 *    and keep a value dropped from the config. ExactlyOneOf across the destinations, which the
 *    spec cannot express. tags use the shared resource_team tag type, as for the other
 *    resources. notification_rule: cluster_ids_filter defaults to an empty list, so omitting it
 *    clears the filter.
 *  - notification_silence resource (its schedule and teamsFilter are expanded by expand-oneof.js):
 *    the schedule blocks, the weekdays, time_zone, teams, query_string and notification_rule_ids are
 *    plain optional (computed_optional would plan unselected blocks as `(known after apply)` and
 *    keep a value dropped from the config); ExactlyOneOf across the schedule blocks, which the
 *    spec cannot express; tags use the shared resource_team tag type.
 *  - custom_role resource (its teamScope is expanded from a discriminated oneOf by expand-oneof.js):
 *    `assets` becomes a map keyed by the spec's resource types, with a key OneOf, a value OneOf
 *    and a validator for the read-only types, all derived from the spec; team_ids and telemetry
 *    are sets, as their order is irrelevant; description and telemetry_query are plain optional
 *    so dropping them clears them (team_ids too, as the response omits it for the all and public
 *    scopes); id uses use_state_for_unknown.
 *  - cloud_account resource (its request body is expanded from a discriminated
 *    oneOf by expand-oneof.js): the spec cannot express that exactly one of
 *    aws, gcp and azure is set, so adds ExactlyOneOf across them. Also makes
 *    the three blocks plain optional (codegen marks them computed_optional,
 *    which would plan the unselected ones as `(known after apply)`) with
 *    requires_replace on their fields (connection settings are immutable); makes
 *    account_friendly_name plain optional (computed_optional would keep the
 *    stored name when it is dropped from the config, so it could never be
 *    cleared); and adds use_state_for_unknown to `id`, as for the team resource.
 *  - every resource: validators tfplugingen does not generate, derived from the spec: list and
 *    set item constraints (enum, length, excluded literal) and the `kebab-case` string format.
 */
const fs = require("fs");

function fail(message) {
  console.error(message);
  process.exit(1);
}

if (process.argv.length < 4) {
  fail("Usage: patch-provider-code-spec.js <provider-code-spec.json> <expanded-openapi.json>");
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

for (const tagsResourceName of ["operation_api_key", "rum_public_token", "notification_rule", "adaptive_notification_rule", "notification_integration", "notification_silence"]) {
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

const cloudAccount = requireResource("cloud_account");
requireAttr(cloudAccount, "id").string.plan_modifiers = planModifier(
  STRING_PLAN_MODIFIER,
  "stringplanmodifier.UseStateForUnknown()",
);
requireAttr(cloudAccount, "account_friendly_name").string.computed_optional_required = "optional";
for (const name of ["aws", "gcp", "azure"]) {
  const block = requireAttr(cloudAccount, name).single_nested;
  block.computed_optional_required = "optional";
  // Object-level plan modifiers run on null blocks and the generated
  // ValueFromObject rejects null objects, so replace on each field instead.
  for (const attr of block.attributes) {
    attr.string.plan_modifiers = planModifier(STRING_PLAN_MODIFIER, "stringplanmodifier.RequiresReplace()");
  }
}
requireAttr(cloudAccount, "aws").single_nested.validators = [
  {
    custom: {
      imports: [
        {path: "github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"},
        {path: "github.com/hashicorp/terraform-plugin-framework/path"},
      ],
      schema_definition:
      'objectvalidator.ExactlyOneOf(path.MatchRoot("aws"), path.MatchRoot("gcp"), path.MatchRoot("azure"))',
    },
  },
];

const kindOf = (attr) => Object.keys(attr).find((key) => key !== "name");

// Calls fn(attr, kind, name) on every attribute, nested ones included.
function walkAttrs(attributes, fn) {
  for (const attr of attributes) {
    const kind = kindOf(attr);
    fn(attr[kind], kind, attr.name);
    const nested = attr[kind].attributes || attr[kind].nested_object?.attributes;
    if (nested) walkAttrs(nested, fn);
  }
}

// Exactly one attribute of an object is set, which the spec cannot express (a oneOf is lost when
// expanded). internal/validators.ExactlyOneOf, placed on the object holding the blocks.
const SHARED_VALIDATORS = "terraform-provider-tsuga/internal/validators";
const exactlyOneOf = (summary, noun) => [
  {
    custom: {
      imports: [{path: SHARED_VALIDATORS}],
      schema_definition: `validators.ExactlyOneOf(${JSON.stringify(summary)}, ${JSON.stringify(noun)})`,
    },
  },
];

// Blocks expanded from a oneOf, and the given optional free text the response echoes, are plain
// optional: codegen marks them computed_optional, which would plan the unselected blocks as
// `(known after apply)` on every update and keep a value that was dropped from the config.
function makeOptional(resource, names) {
  walkAttrs(resource.schema.attributes, (attr, kind, name) => {
    if ((kind === "single_nested" || names.includes(name)) && attr.computed_optional_required === "computed_optional") {
      attr.computed_optional_required = "optional";
    }
  });
}
// The spec cannot express that exactly one destination is set: validators.ExactlyOneOf.
function patchNotificationRule(resource, targetAttributes, optionalNames) {
  makeOptional(resource, optionalNames);
  requireEntry(targetAttributes, "attribute", "config").single_nested.validators = exactlyOneOf("Invalid target config", "destination");
}

const notificationRule = requireResource("notification_rule");
patchNotificationRule(
  notificationRule,
  requireAttr(notificationRule, "targets").list_nested.nested_object.attributes,
  ["query_string", "teams", "open_status", "closed_status"],
);
// Omitting cluster_ids_filter clears it: the API treats an omitted or empty filter alike, so the
// attribute defaults to an empty list.
requireAttr(notificationRule, "cluster_ids_filter").list.default = {
  custom: {
    imports: [
      {path: "github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"},
      {path: "github.com/hashicorp/terraform-plugin-framework/attr"},
      {path: "github.com/hashicorp/terraform-plugin-framework/types"},
    ],
    schema_definition: "listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{}))",
  },
};

const integration = requireResource("notification_integration");
// The API never returns a secret, only its `<name>_last_chars`: the secret request fields are
// sensitive. They are also write-only, which the code spec cannot express; the resource's Schema
// method marks every sensitive string write-only.
const INTEGRATION_SECRETS = [
  "setting.google_chat.webhook_url",
  "setting.grafana_irm.url",
  "setting.incident_io.token",
  "setting.jira.api_token",
  "setting.microsoft_teams.webhook_url",
  "setting.pagerduty.integration_key",
  "setting.servicenow.password",
  "setting.squadcast.webhook_url",
  "setting.webhook.authentication.basic.password",
  "setting.webhook.authentication.bearer.token",
  "setting.webhook.authentication.custom_header.value",
  "setting.webhook.url",
];
for (const secretPath of INTEGRATION_SECRETS) {
  let attributes = integration.schema.attributes;
  let attr;
  for (const name of secretPath.split(".")) {
    if (!attributes) fail(`${secretPath} not found in ${integration.name}`);
    attr = requireEntry(attributes, "attribute", name);
    attributes = attr.single_nested?.attributes;
  }
  if (!attr.string) fail(`${secretPath} is not a string attribute in ${integration.name}`);
  attr.string.sensitive = true;
}
walkAttrs(integration.schema.attributes, (attr, kind, name) => {
  // custom_headers is required by the spec, but the provider sends {} when it is omitted.
  if ((kind === "single_nested" && attr.computed_optional_required === "computed_optional") || name === "custom_headers") {
    attr.computed_optional_required = "optional";
  }
});
const integrationSetting = requireAttr(integration, "setting").single_nested;
const authentication = requireAttr(integration, "authentication").single_nested;
integrationSetting.validators = exactlyOneOf("Invalid integration setting", "integration");
authentication.validators = exactlyOneOf("Invalid webhook authentication", "authentication method");
// Changing which integration is set replaces the resource (the API cannot change the type);
// settingTypeRequiresReplaceModifier lives in the resource package.
integrationSetting.plan_modifiers = planModifier("github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier", "settingTypeRequiresReplaceModifier{}");
// secrets_version is provider-only: write-only secrets are never in state, so Terraform cannot see a rotation.
integration.schema.attributes.push({
  name: "secrets_version",
  string: {
    computed_optional_required: "optional",
    description:
    "Secrets are write-only and never stored in state, so Terraform cannot detect when one changes. Set this to any new value after rotating a secret to have it sent again.",
  },
});

const notificationSilence = requireResource("notification_silence");
makeOptional(notificationSilence, [
  "teams", "time_zone", "query_string", "notification_rule_ids",
  "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday",
]);

requireAttr(notificationSilence, "schedule").single_nested.validators = exactlyOneOf("Invalid schedule", "schedule");
// The spec's descriptions name sibling fields in camelCase; Terraform users see snake_case.
walkAttrs(notificationSilence.schema.attributes, (attr) => {
  attr.description &&= attr.description.replace(/`([a-z]+(?:[A-Z][a-z]*)+)`/g, (_, name) =>
    `\`${name.replace(/[A-Z]/g, (c) => `_${c.toLowerCase()}`)}\``);
});

const adaptiveRule = requireResource("adaptive_notification_rule");
patchNotificationRule(
  adaptiveRule,
  requireAttr(adaptiveRule, "fallback_target").single_nested.attributes,
  ["open_status", "closed_status", "monitor_ids", "slo_ids"],
);
// The provider always sends both lists, so omitting one clears it instead of keeping it as the API does.
requireAttr(adaptiveRule, "monitor_ids").list.description =
  "Monitors this rule reacts to. Only trace monitors and metric monitors over OTel span metrics are eligible. " +
  "Omitting this attribute removes all monitors from the rule. Set at least one monitor or SLO.";
requireAttr(adaptiveRule, "slo_ids").list.description =
  "SLOs this rule reacts to. Only SLOs over traces or OTel span metrics are eligible. " +
  "Omitting this attribute removes all SLOs from the rule. Set at least one monitor or SLO.";

// The expanded OpenAPI spec (argv[3]) the generic steps below derive validators from, read along
// the create request body of every resource in generator_config.yml.
function createRequestPaths() {
  const paths = {};
  let section = null;
  let resource = null;
  let operation = null;
  for (const line of fs.readFileSync("generator_config.yml", "utf8").split("\n")) {
    let m;
    if ((m = line.match(/^(\w+):$/))) section = m[1];
    else if (section === "resources" && (m = line.match(/^  (\w+):$/))) resource = m[1];
    else if ((m = line.match(/^    (\w+):$/))) operation = m[1];
    else if (section === "resources" && operation === "create" && (m = line.match(/^      path: (.+)$/))) paths[resource] = m[1];
  }
  return paths;
}

const apiSpec = JSON.parse(fs.readFileSync(process.argv[3], "utf8"));
const deref = (schema) =>
      schema?.$ref ? schema.$ref.replace(/^#\//, "").split("/").reduce((node, part) => node[part], apiSpec) : schema;
const nonNull = (schema) => deref(schema?.anyOf ? schema.anyOf.find((s) => s.type !== "null") : schema);
const snakeCase = (name) => name.replace(/[A-Z]/g, (c) => "_" + c.toLowerCase());
const propertyFor = (schema, name) => {
  const properties = nonNull(schema)?.properties || {};
  const key = Object.keys(properties).find((k) => snakeCase(k) === name);
  return key && nonNull(properties[key]);
};

// custom_role. The spec cannot express that `assets` is a map keyed by resource type (it is a closed
// object, which codegen would turn into one attribute per type with the hyphens stripped), that
// the order of team_ids and telemetry is irrelevant, or that dropping description or
// telemetry_query from the config clears them rather than keeping the stored value.
function listToSet(attr) {
  attr.set = attr.list;
  delete attr.list;
  attr.set.validators = (attr.set.validators || [])
    .filter((v) => !v.custom.schema_definition.includes("UniqueValues"))
    .map((v) => ({
      custom: {
        imports: v.custom.imports.map((i) => ({path: i.path.replace("listvalidator", "setvalidator")})),
        schema_definition: v.custom.schema_definition.replaceAll("listvalidator.", "setvalidator."),
      },
    }));
}

const customRole = requireResource("custom_role");
requireAttr(customRole, "id").string.plan_modifiers = planModifier(
  STRING_PLAN_MODIFIER,
  "stringplanmodifier.UseStateForUnknown()",
);
requireAttr(customRole, "description").string.computed_optional_required = "optional";
requireAttr(customRole, "telemetry_query").list_nested.computed_optional_required = "optional";
for (const name of ["team_ids", "telemetry"]) {
  listToSet(requireAttr(customRole, name));
}
// team_ids is only set for the specific and all-except scopes; the response omits it otherwise.
requireAttr(customRole, "team_ids").set.computed_optional_required = "optional";
const assets = requireAttr(customRole, "assets");
const assetLevels = Object.fromEntries(
  Object.entries(
    JSON.parse(fs.readFileSync(process.argv[3], "utf8")).paths["/v1/custom-roles"].post.requestBody.content["application/json"].schema
      .properties.assets.properties,
  ).map(([assetType, property]) => [assetType, property.enum]),
);
const assetsSchema = JSON.parse(fs.readFileSync(process.argv[3], "utf8")).paths["/v1/custom-roles"].post.requestBody.content["application/json"].schema.properties.assets;
const stringSlice = (values) => `[]string{${values.map((v) => JSON.stringify(v)).join(", ")}}`;
const readOnlyAssetTypes = Object.keys(assetLevels).filter((t) => assetLevels[t].join() === "read");
const mapValidator = (definition) => ({
  custom: {
    imports: [
      {path: "github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"},
      {path: "github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"},
    ],
    schema_definition: definition,
  },
});
assets.map = {
  computed_optional_required: "required",
  element_type: {string: {}},
  description: assets.single_nested.description,
  validators: [
    mapValidator(`mapvalidator.KeysAre(stringvalidator.OneOf(${Object.keys(assetLevels).map((t) => JSON.stringify(t)).join(", ")}))`),
    mapValidator('mapvalidator.ValueStringsAre(stringvalidator.OneOf("read", "write"))'),
    // requiredAssets and readOnlyAssets (validators.go) check the spec's required keys, and reject `write` on the types the spec only allows `read` on.
    {
      custom: {
        imports: [{path: "github.com/hashicorp/terraform-plugin-framework/schema/validator"}],
        schema_definition: `validator.Map(requiredAssets(${stringSlice(assetsSchema.required)}))`,
      },
    },
    {
      custom: {
        imports: [{path: "github.com/hashicorp/terraform-plugin-framework/schema/validator"}],
        schema_definition: `validator.Map(readOnlyAssets(${stringSlice(readOnlyAssetTypes)}))`,
      },
    },
  ],
};
delete assets.single_nested;

// tfplugingen emits neither the item constraints of list and set attributes nor format-based
// string checks. Each is derived from the spec: item enums, item lengths, an item pattern that only
// excludes one literal (`^(?!\*$)`, which Go's regexp cannot compile), and the `kebab-case` format.
const ELEMENT_TYPES = {
  string: {pkg: "stringvalidator", each: "ValueStringsAre"},
  int64: {pkg: "int64validator", each: "ValueInt64sAre"},
};
const literal = (value) => (typeof value === "string" ? JSON.stringify(value) : String(value));
const KEBAB_CASE = {
  imports: [{path: "regexp"}, {path: "github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"}],
  schema_definition:
  'stringvalidator.RegexMatches(regexp.MustCompile("^[a-z0-9]+(-[a-z0-9]+)*$"), "must be kebab-case: lowercase letters, digits and single hyphens")',
};

function itemRules(items) {
  const rules = [];
  if (items.enum) rules.push(`OneOf(${items.enum.map(literal).join(", ")})`);
  if (items.minLength !== undefined && items.maxLength !== undefined) rules.push(`LengthBetween(${items.minLength}, ${items.maxLength})`);
  else if (items.minLength !== undefined) rules.push(`LengthAtLeast(${items.minLength})`);
  else if (items.maxLength !== undefined) rules.push(`LengthAtMost(${items.maxLength})`);
  const excluded = items.pattern?.match(/^\^\(\?!(.+)\$\)$/);
  if (excluded) rules.push(`NoneOf(${JSON.stringify(excluded[1].replace(/\\(.)/g, "$1"))})`);
  return rules;
}

function addSpecValidators(attributes = [], schema) {
  for (const attr of attributes) {
    const kind = kindOf(attr);
    const prop = propertyFor(schema, attr.name);
    if (!prop) continue;
    if (kind === "list" || kind === "set") {
      const handler = ELEMENT_TYPES[Object.keys(attr[kind].element_type)[0]];
      const rules = handler ? itemRules(nonNull(prop.items) || {}) : [];
      if (rules.length) {
        const pkg = `${kind}validator`;
        attr[kind].validators = [
          ...(attr[kind].validators || []),
          {
            custom: {
              imports: [
                {path: `github.com/hashicorp/terraform-plugin-framework-validators/${pkg}`},
                {path: `github.com/hashicorp/terraform-plugin-framework-validators/${handler.pkg}`},
              ],
              schema_definition: `${pkg}.${handler.each}(${rules.map((rule) => `${handler.pkg}.${rule}`).join(", ")})`,
            },
          },
        ];
      }
    } else if (kind === "string" && prop.format === "kebab-case") {
      attr.string.validators = [...(attr.string.validators || []), {custom: KEBAB_CASE}];
    } else if (kind === "single_nested") {
      addSpecValidators(attr.single_nested.attributes, prop);
    } else if (kind === "list_nested") {
      addSpecValidators(attr.list_nested.nested_object.attributes, nonNull(prop.items));
    }
  }
}

for (const [name, apiPath] of Object.entries(createRequestPaths())) {
  const body = apiSpec.paths[apiPath].post.requestBody.content["application/json"].schema;
  addSpecValidators(requireResource(name).schema.attributes, body);
}

fs.writeFileSync(specPath, JSON.stringify(spec, null, 2) + "\n");
