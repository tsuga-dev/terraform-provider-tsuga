#!/usr/bin/env node

/**
 * Generates Terraform plan-time validators for the hand-written tsuga_dashboard
 * resource schema (internal/resource_dashboard) straight from the OpenAPI spec,
 * so schema-level constraints (enum, length, min/max, size) stay in sync with
 * the API without hand-copying them.
 *
 * dashboard visualization is a `oneOf` the tfplugingen pipeline can't express,
 * so the schema itself is hand-written in internal/resource_dashboard. This
 * script only generates the *validators* referenced from that hand-written
 * schema (internal/resource_dashboard/validators_gen.go) plus the normalizer
 * unit allow-lists referenced from internal/normalizer (units_gen.go).
 *
 * Mapping table: each entry below names a Go validator-list function and the
 * JSON-pointer-like spec path(s) it is derived from. When an attribute is
 * shared by several visualization variants (e.g. `source`, `queries`), all
 * listed paths are resolved and must yield byte-identical constraints, or
 * generation fails loudly instead of silently picking one.
 */
const fs = require("fs");
const path = require("path");

const specPath = process.argv[2] || "./public-open-api.json";
const spec = JSON.parse(fs.readFileSync(specPath, "utf8"));
const schemas = spec.components.schemas;

function fail(msg) {
  console.error(`gen-dashboard-validators: ${msg}`);
  process.exit(1);
}

function dashboardRequestSchema() {
  const op = spec.paths["/v1/dashboards"] && spec.paths["/v1/dashboards"].post;
  if (!op) fail("POST /v1/dashboards not found in spec");
  return op.requestBody.content["application/json"].schema;
}

// Resolves a dot-separated path against the spec. "$dashboard" is a special
// root alias for the create-dashboard request body; any other first segment
// is looked up in components.schemas.
function resolve(pathStr) {
  const parts = pathStr.split(".");
  let node;
  const head = parts.shift();
  if (head === "$dashboard") {
    node = dashboardRequestSchema();
  } else {
    node = schemas[head];
    if (!node) fail(`schema not found: ${head} (from ${pathStr})`);
  }
  for (const part of parts) {
    if (node == null) fail(`path segment missing: ${pathStr}`);
    node = /^\d+$/.test(part) ? node[Number(part)] : node[part];
  }
  if (node == null) fail(`path not found: ${pathStr}`);
  return node;
}

// Unwraps a nullable `anyOf: [{type: null}, <schema>]` into <schema>.
function unwrapNullable(node) {
  if (!node.anyOf) return node;
  const branch = node.anyOf.find((b) => b.type !== "null");
  if (!branch) fail("nullable anyOf has no non-null branch");
  return branch;
}

function enumOf(node) {
  if (!node.enum) fail(`expected enum, found none on ${JSON.stringify(node).slice(0, 120)}`);
  return node.enum;
}

function enumFromMapping(node) {
  if (!node.discriminator || !node.discriminator.mapping) {
    fail(`expected discriminator.mapping, found none on ${JSON.stringify(node).slice(0, 120)}`);
  }
  return Object.keys(node.discriminator.mapping);
}

function enumFromOneOfTypes(node) {
  if (!node.oneOf) fail(`expected oneOf, found none on ${JSON.stringify(node).slice(0, 120)}`);
  const values = [];
  for (const branch of node.oneOf) {
    const t = branch.properties && branch.properties.type;
    if (!t || !t.enum) fail("oneOf branch missing properties.type.enum");
    values.push(...t.enum);
  }
  return values;
}

// Extracts the constraint signature used for a given mode, so multiple spec
// paths claiming to describe "the same" attribute can be compared for
// equality.
function signature(mode, node) {
  switch (mode) {
    case "enum":
      return { enum: enumOf(node) };
    case "enumMapping":
      return { enum: enumFromMapping(node) };
    case "enumOneOfType":
      return { enum: enumFromOneOfTypes(node) };
    case "string":
    case "nullableString": {
      const n = mode === "nullableString" ? unwrapNullable(node) : node;
      return { min: n.minLength || 0, max: n.maxLength, pattern: n.pattern };
    }
    case "int64":
    case "float64":
      return { min: node.minimum, max: node.maximum };
    case "list":
      return { min: node.minItems, max: node.maxItems };
    default:
      fail(`unknown mode: ${mode}`);
  }
}

function goStringSlice(values) {
  return values.map((v) => JSON.stringify(v)).join(", ");
}

// Renders the Go function body (validator expressions only, one per line)
// for a given mode + resolved node.
function renderValidators(mode, node) {
  switch (mode) {
    case "enum":
      return [`stringvalidator.OneOf(${goStringSlice(enumOf(node))})`];
    case "enumMapping":
      return [`stringvalidator.OneOf(${goStringSlice(enumFromMapping(node))})`];
    case "enumOneOfType":
      return [`stringvalidator.OneOf(${goStringSlice(enumFromOneOfTypes(node))})`];
    case "string":
    case "nullableString": {
      const n = mode === "nullableString" ? unwrapNullable(node) : node;
      const out = [];
      if (n.minLength) out.push(`stringvalidator.LengthAtLeast(${n.minLength})`);
      if (n.maxLength !== undefined) out.push(`stringvalidator.LengthAtMost(${n.maxLength})`);
      if (n.pattern) out.push(`stringvalidator.RegexMatches(regexp.MustCompile(${JSON.stringify(n.pattern)}), "")`);
      if (out.length === 0) fail("string mode produced no validators");
      return out;
    }
    case "int64": {
      const out = [];
      if (node.minimum !== undefined && node.maximum !== undefined) {
        out.push(`int64validator.Between(${node.minimum}, ${node.maximum})`);
      } else if (node.minimum !== undefined) {
        out.push(`int64validator.AtLeast(${node.minimum})`);
      } else if (node.maximum !== undefined) {
        out.push(`int64validator.AtMost(${node.maximum})`);
      }
      if (out.length === 0) fail("int64 mode produced no validators");
      return out;
    }
    case "float64": {
      const out = [];
      if (node.minimum !== undefined && node.maximum !== undefined) {
        out.push(`float64validator.Between(${node.minimum}, ${node.maximum})`);
      } else if (node.minimum !== undefined) {
        out.push(`float64validator.AtLeast(${node.minimum})`);
      } else if (node.maximum !== undefined) {
        out.push(`float64validator.AtMost(${node.maximum})`);
      }
      if (out.length === 0) fail("float64 mode produced no validators");
      return out;
    }
    case "list": {
      const out = [];
      if (node.minItems !== undefined && node.maxItems !== undefined) {
        out.push(`listvalidator.SizeBetween(${node.minItems}, ${node.maxItems})`);
      } else if (node.minItems !== undefined) {
        out.push(`listvalidator.SizeAtLeast(${node.minItems})`);
      } else if (node.maxItems !== undefined) {
        out.push(`listvalidator.SizeAtMost(${node.maxItems})`);
      }
      if (out.length === 0) fail("list mode produced no validators");
      return out;
    }
    default:
      fail(`unknown mode: ${mode}`);
  }
}

const elementType = {
  enum: "validator.String",
  enumMapping: "validator.String",
  enumOneOfType: "validator.String",
  string: "validator.String",
  nullableString: "validator.String",
  int64: "validator.Int64",
  float64: "validator.Float64",
  list: "validator.List",
};

// Mapping table: Go func name -> { mode, paths }. All `paths` must resolve to
// identical constraints under `mode`.
const ATTRS = {
  // Dashboard-level
  timePresetValidators: { mode: "enum", paths: ["$dashboard.properties.timePreset"] },
  ownerValidators: { mode: "string", paths: ["$dashboard.properties.owner"] },
  folderIdValidators: { mode: "nullableString", paths: ["$dashboard.properties.folderId"] },
  dashboardNameValidators: { mode: "string", paths: ["$dashboard.properties.name"] },
  filtersListValidators: { mode: "list", paths: ["$dashboard.properties.filters"] },
  filterKeyValidators: { mode: "string", paths: ["$dashboard.properties.filters.items.properties.key"] },
  filterValueValidators: {
    mode: "string",
    paths: ["$dashboard.properties.filters.items.properties.values.items"],
  },
  tagsListValidators: { mode: "list", paths: ["$dashboard.properties.tags"] },
  tagKeyValidators: { mode: "string", paths: ["$dashboard.properties.tags.items.properties.key"] },
  tagValueValidators: { mode: "string", paths: ["$dashboard.properties.tags.items.properties.value"] },

  // Graph-level
  graphIdValidators: { mode: "string", paths: ["$dashboard.properties.graphs.items.properties.id"] },
  graphNameValidators: { mode: "string", paths: ["$dashboard.properties.graphs.items.properties.name"] },
  graphDescriptionValidators: {
    mode: "string",
    paths: ["$dashboard.properties.graphs.items.properties.description"],
  },
  flexAlignValidators: {
    mode: "enum",
    paths: [
      "$dashboard.properties.graphs.items.properties.descriptionAlign",
      "$dashboard.properties.graphs.items.properties.descriptionJustifyContent",
      "InputGraphVisualizationNote.properties.noteAlign",
      "InputGraphVisualizationNote.properties.noteJustifyContent",
    ],
  },

  // Series visualizations (timeseries, top_list, pie, query_value, bar, gauge,
  // distribution, heatmap) and table columns share these.
  sourceValidators: {
    mode: "enum",
    paths: [
      "InputGraphVisualizationTimeseries.properties.source",
      "InputGraphVisualizationTopList.properties.source",
      "InputGraphVisualizationPie.properties.source",
      "InputGraphVisualizationQueryValue.properties.source",
      "InputGraphVisualizationBar.properties.source",
      "InputGraphVisualizationGauge.properties.source",
      "InputGraphVisualizationDistribution.properties.source",
      "InputGraphVisualizationHeatmap.properties.source",
      "InputGraphVisualizationTable.properties.columns.items.properties.source",
    ],
  },
  queriesListValidators: {
    mode: "list",
    paths: [
      "InputGraphVisualizationTimeseries.properties.queries",
      "InputGraphVisualizationTopList.properties.queries",
      "InputGraphVisualizationPie.properties.queries",
      "InputGraphVisualizationQueryValue.properties.queries",
      "InputGraphVisualizationBar.properties.queries",
      "InputGraphVisualizationGauge.properties.queries",
      "InputGraphVisualizationDistribution.properties.queries",
      "InputGraphVisualizationHeatmap.properties.queries",
      "InputGraphVisualizationTable.properties.columns.items.properties.queries",
    ],
  },
  formulaValidators: {
    mode: "string",
    paths: [
      "InputGraphVisualizationTimeseries.properties.formula",
      "InputGraphVisualizationTopList.properties.formula",
      "InputGraphVisualizationPie.properties.formula",
      "InputGraphVisualizationQueryValue.properties.formula",
      "InputGraphVisualizationBar.properties.formula",
      "InputGraphVisualizationGauge.properties.formula",
      "InputGraphVisualizationDistribution.properties.formula",
      "InputGraphVisualizationHeatmap.properties.formula",
      "InputGraphVisualizationTable.properties.columns.items.properties.formula",
    ],
  },
  groupByListValidators: {
    mode: "list",
    paths: [
      "InputGraphVisualizationTimeseries.properties.groupBy",
      "InputGraphVisualizationTopList.properties.groupBy",
      "InputGraphVisualizationPie.properties.groupBy",
      "InputGraphVisualizationBar.properties.groupBy",
      "InputGraphVisualizationTable.properties.groupBy",
    ],
  },
  groupByFieldsValidators: {
    mode: "list",
    paths: ["InputGraphVisualizationTimeseries.properties.groupBy.items.properties.fields"],
  },
  groupByReplaceNullWithValidators: {
    mode: "string",
    paths: ["InputGraphVisualizationTimeseries.properties.groupBy.items.properties.replaceNullWith"],
  },
  aliasesFormulaValidators: {
    mode: "string",
    paths: ["InputGraphVisualizationTimeseries.properties.aliases.properties.formula"],
  },
  aliasesQueriesValueValidators: {
    mode: "string",
    paths: ["InputGraphVisualizationTimeseries.properties.aliases.properties.queries.additionalProperties"],
  },
  legendModeValidators: {
    mode: "enum",
    paths: [
      "InputGraphVisualizationTimeseries.properties.legendMode",
      "InputGraphVisualizationTopList.properties.legendMode",
      "InputGraphVisualizationPie.properties.legendMode",
      "InputGraphVisualizationQueryValue.properties.legendMode",
      "InputGraphVisualizationBar.properties.legendMode",
    ],
  },
  backgroundModeValidators: {
    mode: "enum",
    paths: ["InputGraphVisualizationQueryValue.properties.backgroundMode"],
  },
  conditionsOperatorValidators: {
    mode: "enum",
    paths: ["InputGraphVisualizationQueryValue.properties.conditions.items.properties.operator"],
  },
  conditionsColorValidators: {
    mode: "enum",
    paths: ["InputGraphVisualizationQueryValue.properties.conditions.items.properties.color"],
  },
  paletteColorValidators: {
    mode: "enum",
    paths: [
      "InputGraphVisualizationGauge.properties.colorThresholds.items.properties.color",
      "InputGraphVisualizationHeatmap.properties.palette",
    ],
  },
  boundsScaleValidators: {
    mode: "enum",
    paths: ["InputGraphVisualizationDistribution.properties.boundsScale"],
  },
  percentileMarkerValidators: {
    mode: "int64",
    paths: ["InputGraphVisualizationDistribution.properties.percentileMarkers.items"],
  },

  // Post-processing functions
  functionTypeValidators: {
    mode: "enumMapping",
    paths: ["InputGraphVisualizationTimeseries.properties.queries.items.properties.functions.items"],
  },
  functionSecondsValidators: { mode: "int64", paths: ["InputFunctionTimeOffset.properties.seconds"] },
  functionBaseValidators: { mode: "int64", paths: ["InputFunctionLog.properties.base"] },

  // Y-axis settings (timeseries, bar, timeseries_connection, bar_connection)
  yAxisBoundTypeValidators: {
    mode: "enumOneOfType",
    paths: ["InputGraphVisualizationTimeseries.properties.yAxisSettings.properties.min"],
  },
  yAxisScaleTypeValidators: {
    mode: "enumOneOfType",
    paths: ["InputGraphVisualizationTimeseries.properties.yAxisSettings.properties.scale"],
  },
  yAxisScaleExponentValidators: {
    mode: "float64",
    paths: ["InputGraphVisualizationTimeseries.properties.yAxisSettings.properties.scale.oneOf.0.properties.exponent"],
  },

  // Table
  tableColumnsListValidators: { mode: "list", paths: ["InputGraphVisualizationTable.properties.columns"] },
  tableColumnNameValidators: {
    mode: "string",
    paths: ["InputGraphVisualizationTable.properties.columns.items.properties.name"],
  },

  // Note
  noteValidators: { mode: "string", paths: ["InputGraphVisualizationNote.properties.note"] },
  noteColorValidators: { mode: "enum", paths: ["InputGraphVisualizationNote.properties.noteColor"] },

  // List / list_spans / list_log_patterns
  listQueryValidators: {
    mode: "string",
    paths: [
      "InputGraphVisualizationList.properties.query",
      "InputGraphVisualizationListSpans.properties.query",
      "InputGraphVisualizationListLogPatterns.properties.query",
    ],
  },
  listLogPatternsLayoutValidators: {
    mode: "enum",
    paths: ["InputGraphVisualizationListLogPatterns.properties.layout"],
  },

  // Connections
  connectionIdValidators: {
    mode: "string",
    paths: [
      "InputGraphVisualizationTimeseriesConnection.properties.connectionId",
      "InputGraphVisualizationListConnection.properties.connectionId",
      "InputGraphVisualizationTopListConnection.properties.connectionId",
      "InputGraphVisualizationPieConnection.properties.connectionId",
      "InputGraphVisualizationBarConnection.properties.connectionId",
      "InputGraphVisualizationQueryValueConnection.properties.connectionId",
    ],
  },
  connectionSqlQueriesListValidators: {
    mode: "list",
    paths: [
      "InputGraphVisualizationTimeseriesConnection.properties.queries",
      "InputGraphVisualizationTopListConnection.properties.queries",
      "InputGraphVisualizationPieConnection.properties.queries",
      "InputGraphVisualizationBarConnection.properties.queries",
      "InputGraphVisualizationQueryValueConnection.properties.queries",
    ],
  },
  connectionSqlQueryValidators: {
    mode: "string",
    paths: [
      "InputGraphVisualizationTimeseriesConnection.properties.queries.items",
      "InputGraphVisualizationTopListConnection.properties.queries.items",
      "InputGraphVisualizationPieConnection.properties.queries.items",
      "InputGraphVisualizationBarConnection.properties.queries.items",
      "InputGraphVisualizationQueryValueConnection.properties.queries.items",
    ],
  },
  listConnectionQueryValidators: {
    mode: "string",
    paths: ["InputGraphVisualizationListConnection.properties.query"],
  },
};

function genValidatorsGo() {
  const funcs = [];
  for (const [name, { mode, paths }] of Object.entries(ATTRS)) {
    const nodes = paths.map(resolve);
    const base = signature(mode, nodes[0]);
    for (let i = 1; i < nodes.length; i++) {
      const sig = signature(mode, nodes[i]);
      if (JSON.stringify(sig) !== JSON.stringify(base)) {
        fail(
          `${name}: ${paths[0]} and ${paths[i]} disagree on constraints\n  ${JSON.stringify(
            base
          )}\n  ${JSON.stringify(sig)}`
        );
      }
    }
    const validators = renderValidators(mode, nodes[0]);
    funcs.push({ name, elemType: elementType[mode], validators });
  }

  const lines = [];
  lines.push("// Code generated by tools/gen-dashboard-validators.js; DO NOT EDIT.");
  lines.push("package resource_dashboard");
  lines.push("");
  lines.push("import (");
  lines.push('\t"regexp"');
  lines.push("");
  lines.push('\t"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"');
  lines.push('\t"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"');
  lines.push('\t"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"');
  lines.push('\t"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"');
  lines.push('\t"github.com/hashicorp/terraform-plugin-framework/schema/validator"');
  lines.push(")");
  for (const { name, elemType, validators } of funcs) {
    lines.push("");
    lines.push(`func ${name}() []${elemType} {`);
    lines.push(`\treturn []${elemType}{`);
    for (const v of validators) {
      lines.push(`\t\t${v},`);
    }
    lines.push("\t}");
    lines.push("}");
  }
  lines.push("");
  return lines.join("\n");
}

function genNormalizerUnitsGo() {
  const durationUnits = enumOf(resolve("DataNormalizerDuration.properties.unit"));
  const dataUnits = enumOf(resolve("DataNormalizerData.properties.unit"));
  const custom = resolve("InputDataNormalizerCustom.properties.unit");
  if (custom.minLength === undefined || custom.maxLength === undefined) {
    fail("InputDataNormalizerCustom.properties.unit must declare minLength and maxLength");
  }

  const typeNodes = [
    resolve("InputGraphVisualizationTimeseries.properties.normalizer"),
    resolve("InputGraphVisualizationTable.properties.columns.items.properties.normalizer"),
  ];
  const typeEnum = enumFromMapping(typeNodes[0]);
  for (const n of typeNodes.slice(1)) {
    const other = enumFromMapping(n);
    if (JSON.stringify(other) !== JSON.stringify(typeEnum)) {
      fail(`normalizer type enum disagrees across paths: ${JSON.stringify(typeEnum)} vs ${JSON.stringify(other)}`);
    }
  }

  const lines = [
    "// Code generated by tools/gen-dashboard-validators.js; DO NOT EDIT.",
    "package normalizer",
    "",
    'import (',
    '\t"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"',
    '\t"github.com/hashicorp/terraform-plugin-framework/schema/validator"',
    ")",
    "",
    `var durationUnits = []string{${goStringSlice(durationUnits)}}`,
    "",
    `var dataUnits = []string{${goStringSlice(dataUnits)}}`,
    "",
    "const (",
    `\tcustomUnitMinLength = ${custom.minLength}`,
    `\tcustomUnitMaxLength = ${custom.maxLength}`,
    ")",
    "",
    "func normalizerTypeValidators() []validator.String {",
    "\treturn []validator.String{",
    `\t\tstringvalidator.OneOf(${goStringSlice(typeEnum)}),`,
    "\t}",
    "}",
    "",
  ];
  return lines.join("\n");
}

fs.writeFileSync(
  path.join(process.cwd(), "internal/resource_dashboard/validators_gen.go"),
  genValidatorsGo()
);
fs.writeFileSync(path.join(process.cwd(), "internal/normalizer/units_gen.go"), genNormalizerUnitsGo());
console.log("gen-dashboard-validators: wrote validators_gen.go and units_gen.go");
