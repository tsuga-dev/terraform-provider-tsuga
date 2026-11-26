#!/usr/bin/env node

/**
 * tfplugingen-openapi skips `oneOf` ("schema composition is currently not
 * supported"). This rewrites the discriminated `oneOf` schemas of the resources
 * listed in RULES into plain objects, so the generator sees them. Each rule
 * reshapes the request bodies and the response schema of one resource; every
 * other resource is left untouched.
 *
 * A discriminated oneOf becomes an object with one optional property per
 * variant, keyed by the discriminator value (hyphens become underscores). The
 * discriminator property itself is dropped from request variants, and kept in
 * response variants, where the generator makes it computed.
 */
const fs = require("fs");

if (process.argv.length < 3) {
  console.error("Usage: expand-oneof.js <openapi.json>");
  process.exit(1);
}

const spec = JSON.parse(fs.readFileSync(process.argv[2], "utf8"));

function deref(schema) {
  if (!schema.$ref) return schema;
  return schema.$ref
    .replace(/^#\//, "")
    .split("/")
    .reduce((node, part) => node[part], spec);
}

const snake = (key) => key.replaceAll("-", "_");

// [spec name, variant clone, discriminator] for each variant of a discriminated oneOf.
function variants(schema) {
  const {propertyName, mapping} = schema.discriminator;
  return schema.oneOf.map((raw) => {
    const variant = structuredClone(deref(raw));
    const mapped = Object.keys(mapping || {}).find((key) => mapping[key] === raw.$ref);
    return [mapped || variant.properties[propertyName].enum[0], variant, propertyName];
  });
}

function deleteKeys(schema, ...names) {
  schema.required = (schema.required || []).filter((name) => !names.includes(name));
  for (const name of names) delete schema.properties[name];
  return schema;
}

function rename(schema, from, to) {
  schema.properties[to] = schema.properties[from];
  delete schema.properties[from];
  schema.required = (schema.required || []).map((name) => (name === from ? to : name));
  return schema;
}

// An object with one optional property per variant.
function expandOneOf(schema, {keepDiscriminator = false} = {}) {
  const properties = {};
  for (const [name, variant, discriminator] of variants(schema)) {
    properties[snake(name)] = keepDiscriminator ? variant : deleteKeys(variant, discriminator);
  }
  return {type: "object", properties};
}

// One object with the discriminator as a required enum and the union of the variants' properties.
function unionOneOf(schema) {
  const {propertyName} = schema.discriminator;
  const list = variants(schema);
  const values = list.map(([name]) => name);
  const requires = (variant) => {
    const names = (variant.required || []).filter((name) => name !== propertyName);
    return names.length ? ` Requires ${names.map((name) => `\`${name.replace(/[A-Z]/g, (c) => "_" + c.toLowerCase())}\``).join(", ")}.` : "";
  };
  const description = list
        .map(([, variant], i) => variant.properties[propertyName].description && `\`${values[i]}\`: ${variant.properties[propertyName].description}${requires(variant)}`)
        .filter(Boolean)
        .join(" ");
  const properties = {[propertyName]: {type: "string", enum: values, ...(description && {description})}};
  for (const [, variant] of list) Object.assign(properties, deleteKeys(variant, propertyName).properties);
  return {type: "object", description: schema.description, required: [propertyName], properties};
}

// A notification target: one block per integration in its config, each keeping its `renotify`.
function reshapeNotificationTarget(target, role) {
  const config = expandOneOf(target.properties.config, {keepDiscriminator: role === "response"});
  for (const block of Object.values(config.properties)) {
    if (block.properties.renotifyConfig) rename(block, "renotifyConfig", "renotify");
  }
  config.required = [];
  target.properties.config = config;
}

const RULES = [
  // tsuga_cloud_account: connectionSettings variants become top-level blocks;
  // cloudType and cloudAccountId are derived from them, so they are response-only.
  {
    requests: [["/v1/inventory/cloud-accounts", "post"]],
    apply(body) {
      const blocks = expandOneOf(body.properties.connectionSettings);
      deleteKeys(body, "connectionSettings", "cloudType", "cloudAccountId");
      Object.assign(body.properties, blocks.properties);
    },
  },

  // tsuga_notification_rule: teamsFilter becomes one object with a `type` enum and the union of
  // the variants' properties. Each target config gets one block per integration, each keeping its
  // own `renotifyConfig` (a shared `$ref` schema).
  {
    requests: [
      ["/v1/notification-rules", "post"],
      ["/v1/notification-rules/{id}", "put"],
    ],
    responses: [["/v1/notification-rules/{id}", "get"]],
    apply(rule, role) {
      rule.properties.teamsFilter = unionOneOf(rule.properties.teamsFilter);
      reshapeNotificationTarget(rule.properties.targets.items, role);
    },
  },

  // tsuga_custom_role: teamScope becomes one object with a `type` enum and the union of the variants' properties.
  {
    requests: [
      ["/v1/custom-roles", "post"],
      ["/v1/custom-roles/{id}", "put"],
    ],
    responses: [["/v1/custom-roles/{id}", "get"]],
    apply(role) {
      role.properties.teamScope = unionOneOf(role.properties.teamScope);
    },
  },

  // tsuga_notification_integration: setting becomes one block per integration. The webhook's
  // authentication does the same, and its payloadTemplate becomes one object with a `type` enum
  // and a `value` string: the spec types `value` as a free-form object, which tfplugingen cannot
  // map, so the provider carries it as a JSON-encoded string. Responses carry no discriminator on
  // these blocks either.
  {
    requests: [
      ["/v1/notification-integrations", "post"],
      ["/v1/notification-integrations/{id}", "put"],
    ],
    responses: [["/v1/notification-integrations/{id}", "get"]],
    apply(integration) {
      const setting = expandOneOf(integration.properties.setting);
      const webhook = setting.properties.webhook.properties;
      webhook.authentication = expandOneOf(webhook.authentication);
      webhook.payloadTemplate = unionOneOf(webhook.payloadTemplate);
      webhook.payloadTemplate.properties.value = {type: "string"};
      webhook.payloadTemplate.required.push("value");
      integration.properties.setting = setting;
    },
  },

  // tsuga_adaptive_notification_rule: the fallback target is one notification target.
  {
    requests: [
      ["/v1/adaptive-notification-rules", "post"],
      ["/v1/adaptive-notification-rules/{id}", "put"],
    ],
    responses: [["/v1/adaptive-notification-rules/{id}", "get"]],
    apply(rule, role) {
      reshapeNotificationTarget(rule.properties.fallbackTarget, role);
    },
  },
];

const seen = new Set();
for (const rule of RULES) {
  const targets = [
    ...(rule.requests || []).map(([path, method]) => ["request", spec.paths[path][method].requestBody.content["application/json"].schema]),
    ...(rule.responses || []).map(([path, method]) => ["response", deref(spec.paths[path][method].responses["200"].content["application/json"].schema)]),
  ];
  for (const [role, target] of targets) {
    if (seen.has(target)) continue;
    seen.add(target);
    rule.apply(target, role);
  }
}

process.stdout.write(JSON.stringify(spec, null, 2));
