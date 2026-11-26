# Changelog

All notable changes to the Tsuga Terraform provider are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `tsuga_notification_integration`: read-only `*_last_chars` attributes (such as `token_last_chars`, `webhook_url_last_chars`) with the last characters of each secret, as the API returns them.
- `tsuga_notification_rule`: `cluster_ids_filter`, the cluster IDs that must match for the rule to fire. Omitting it, or leaving it empty, matches every cluster; removing it from a rule clears the filter.

### Removed

- **Breaking:** `hide_time` and `hide_transition` were removed from `slack` targets of `tsuga_notification_rule` and `tsuga_adaptive_notification_rule`, because the Tsuga API no longer supports them. Remove them from your configuration.
- **Breaking:** the read-only `type` of `tsuga_tag_policy` `configuration.telemetry` and `configuration.tsuga_asset` was removed, since the block itself names the type. Stop referencing it.

### Changed

- **Breaking:** `grafana_irm` in `tsuga_notification_integration` exposes `url` and `url_last_chars`, as in the Tsuga API, instead of `webhook_url` and `webhook_url_last_chars`. Rename them in your configuration.
- **Breaking:** `renotify` in `tsuga_notification_rule` targets and the `tsuga_adaptive_notification_rule` fallback target moved from `config` into each destination block (`config.slack.renotify` instead of `config.renotify`), as in the Tsuga API. Move it in your configuration.
- `tsuga_notification_rule`: schema is now generated from the OpenAPI spec. Apart from `renotify`, attribute names and nesting are unchanged, and the spec's limits (such as the allowed `priorities_filter`, `transition_types_filter` and `renotification_states` values, at most 50 tags, and tag value format) are checked at plan time instead of failing at apply. `tsuga_adaptive_notification_rule` shares the target schema, so its `fallback_target` and `tags` get the same checks and descriptions.
- `tsuga_adaptive_notification_rule`: schema is now generated from the OpenAPI spec, with its descriptions. Apart from `renotify`, attribute names and nesting are unchanged, and the spec's limits (such as at most 100 `monitor_ids`, at most 50 tags, tag value format and the allowed `renotification_states` values) are checked at plan time instead of failing at apply.
- `tsuga_notification_integration`: schema is now generated from the OpenAPI spec, with its descriptions. Attribute names and nesting are unchanged, and the spec's limits (HTTPS URLs without credentials for `webhook_url`, `url` and `site_url`, and the lengths of the text fields) are checked at plan time instead of failing at apply.
- `tsuga_custom_role`: schema is now generated from the OpenAPI spec, with its descriptions. Attribute names and nesting are unchanged. `name` is limited to 100 characters (it was 250), as the API requires, and the lengths of `team_ids`, `telemetry_query` and `values` items are checked at plan time.
- `name` of `tsuga_team`, `tsuga_ingestion_api_key`, `tsuga_operation_api_key` and `tsuga_rum_public_token` is checked as kebab-case at plan time, and so are the lengths of the list items of `team_override_fields` (`tsuga_ingestion_api_key`, `tsuga_rum_public_token`), `cluster_ids_filter` and email `addresses` (`tsuga_notification_rule`, `tsuga_adaptive_notification_rule`), as the API spec defines them, instead of failing at apply.
- **Breaking:** `tsuga_notification_silence`: the day windows of `schedule.recurring` moved under `schedule.recurring.weekly_schedule` (`schedule.recurring.weekly_schedule.monday` instead of `schedule.recurring.monday`), as in the Tsuga API. Move them in your configuration.
- `tsuga_notification_silence`: schema is now generated from the OpenAPI spec, with its descriptions. Apart from `weekly_schedule`, attribute names and nesting are unchanged, and the spec's limits (the allowed `priorities_filter` and `transition_types_filter` values, at least one window per weekday, and the lengths of text fields) are checked at plan time instead of failing at apply. `reason` is now required, as the API has always required it.
- `tsuga_tag_policy`: schema is now generated from the OpenAPI spec, with its descriptions. Apart from the removed `type`, attribute names and nesting are unchanged. `configuration.tsuga_asset.asset_types` now accepts `span-route`, `asset_types` may be empty (the API reads it as every type), and the spec's limits (such as at most 1000 `allowed_tag_values`, the lengths of text fields and `drop_sample` between 0 and 100) are checked at plan time instead of failing at apply. `tag_key` may be up to 250 characters (it was 128).

## [3.2.0] - 2026-11-18

### Added

- `tsuga_monitor`: `rum` and `anomaly_rum` configurations, for threshold and anomaly monitors over RUM data. They take the same attributes as `trace` and `anomaly_trace`.

## [3.1.2] - 2026-11-12

### Changed

- `tsuga_cloud_account`: schema is now generated from the OpenAPI spec. Connection fields are checked against the API's formats and lengths at plan time instead of failing at apply.

### Fixed

- `tsuga_cloud_account`: the GCP example used a `workload_identity_provider` the API rejects; it now uses the full resource name (`//iam.googleapis.com/projects/...`).
- `tsuga_user` data source: the API now returns `role` as an object; the data source reads its `type`, and still accepts the old string form.

## [3.1.1] - 2026-11-05

### Changed

- `tsuga_dashboard`, `tsuga_monitor`: faster plan and diff on large configurations; nested attribute types are now computed once per schema instead of at every visited path.

## [3.1.0] - 2026-10-28

### Added

- `tsuga_monitor`: `consecutive_windows` on `log`, `metric` and `trace` configurations: the number of consecutive `timeframe` windows that must all meet the conditions before the monitor alerts, at most 10. Leaving it out, or setting it to 1, keeps single-window evaluation, and removing it from a monitor returns it to single-window evaluation.

## [3.0.0] - 2026-10-26

### Removed

- **Breaking:** the `rolling` query function was removed from the Tsuga API. `tsuga_dashboard` no longer accepts `type = "rolling"` or `window` in query `functions`; `tsuga_monitor` and `tsuga_slo` no longer accept the `rolling` function block.

## [2.16.0] - 2026-10-23

### Added

- `tsuga_custom_role`: new resource managing custom global roles.

## [2.15.0] - 2026-10-20

### Added

- `tsuga_dashboard`: new widget types `timeseries_promql`, `bar_promql`, `pie_promql`, `query_value_promql`, `top_list_promql`, `list_rum`, `slo`, `slo_over_time` and `slo_uptime`.
- `tsuga_dashboard`: queries accept `source = "rum"`, the `cumsum` function, `fill` and `metric_kind`.
- `tsuga_dashboard`: new widget settings `custom_link` and `group_by_mode` (bar, pie, top list, timeseries, table), `thresholds` (bar, timeseries), `line_style_options` (timeseries, timeseries connection), `legend_mode` (top list), `column_sizes` and `default_sorting` (table), `group` (heatmap, distribution) and `y_axis_settings` (heatmap).
- `tsuga_dashboard`: `time_preset` accepts `past-15-days`.

## [2.14.0] - 2026-10-20

### Fixed

- `tsuga_team` data source: looking a team up by `name` now uses the API `name` filter. It used to read only the first 100 teams, so teams past that returned "Team not found".

### Added

- Plan-time validation of TQL queries on:
  - `tsuga_route`
  - `tsuga_metric_route`
  - `tsuga_monitor`
  - `tsuga_slo`
  - `tsuga_dashboard`
  - `tsuga_notification_rule`
  - `tsuga_notification_silence`.
- `tsuga_dashboard`: widget attributes are now validated:
  - `layout`
  - graph `id` uniqueness
  - `formula` query references (`q1`..`qN`)
  - `rolling`/`time-offset` query functions
  - `y_axis_settings` (`number` bound requires `value`, `pow` scale requires `exponent`).
- `tsuga_dashboard`: validate `normalizer.unit` per-type unit list (`duration`, `data`) or length bound (`custom`).

### Changed

- **Breaking:** `tsuga_dashboard` widgets that violate the validations above now fail at plan time instead of being accepted.

### Added

- Requests now include the `x-tsuga-source-terraform-version` header, set to the Terraform version reported by the CLI, when it is known.

## [2.13.0] - 2026-10-19

### Added

- `tsuga_rum_public_token`: new resource for RUM public tokens.
- `tsuga_operation_api_key`: new resource for operation API keys.

### Changed

- `tsuga_ingestion_api_key`: omitting `team_override_fields` now clears them. Previously the stored value was kept, so they could never be unset from Terraform.

## [2.12.0] - 2026-10-15

### Added

- `tsuga_adaptive_notification_rule`: new resource for adaptive notification rules. Dropping `monitor_ids` or `slo_ids` clears that list.

## [2.11.0] - 2026-10-15

### Added

- `tsuga_monitor_folder`: new resource organizing monitors into folders.
- `tsuga_monitor`: `folder_id`, the monitor folder holding the monitor. Dropping it moves the monitor out of its folder.

## [2.10.0] - 2026-09-30

### Added

- `tsuga_metric_route`: new resource for metric routes.

## [2.9.0] - 2026-09-24

### Added

- `tsuga_notification_integration`: `owner`, the team that owns the integration.

## [2.8.0] - 2026-09-17

### Added

- `tsuga_dashboard`: `time_bucket` on the `timeseries` widget.
- `tsuga_dashboard`: `legend_mode` on the `bar`, `pie`, `query_value` and `timeseries` widgets.
- `tsuga_dashboard`: `is_stacked` on the `top_list` widget, requesting stacked rendering.

## [2.7.0] - 2026-09-16

### Added

- `tsuga_dashboard`: `filters[].exclude` inverts a dashboard-wide filter so widgets exclude matching telemetry.

## [2.6.0] - 2026-09-10

### Added

- `tsuga_notification_integration`: new resource managing notification destinations.

## [2.5.1] - 2026-09-09

### Fixed

- Pulled high-severity transport security issue from https://github.com/grpc/grpc-go/pull/9365

## [2.5.0] - 2026-09-08

### Added

- Added WASM exports to the Terraform provider.

## [2.4.0] - 2026-09-01

### Added

- `tsuga_cloud_account`: new `azure` connection block, taking `client_id`, `subscription_id` and `tenant_id`. Mutually exclusive with `aws` and `gcp`, immutable, and `cloud_account_id` is derived from `subscription_id`.

### Fixed

- Updated `go.mod` dependencies to latest available.

## [2.3.0] - 2026-08-26

### Added

- `tsuga_notification_rule`: new `targets.config.servicenow`, `targets.config.google_chat` and `targets.config.jira` destination blocks. `jira` takes `project_key` and `issue_type`, plus the optional `open_status` and `closed_status`.

### Changed

- `tsuga_notification_rule`: the target-level `renotify_config` block is now `targets.config.renotify`, alongside the destination block, matching where the API reads it. This breaks existing configuration, and repeat notifications never took effect from the old location. It cannot be combined with a `jira` destination, which files one issue per notification.

### Removed

- `tsuga_slo`: removed `fill` from aggregation queries. The API derives it from the SLO type (`zero` for event SLOs, `null` for time SLOs).

## [2.2.4] - 2026-08-13

### Added

- `tsuga_dashboard_folder`: new resource managing dashboard folders. Supports `name`, `owner`, `parent_folder_id` (folders nest one level deep) and `tags`.
- `tsuga_dashboard`: new `folder_id` attribute placing the dashboard in a dashboard folder. Omit it to keep the dashboard outside any folder.

## [2.2.3] - 2026-08-12

### Added

- `tsuga_monitor`: new `configuration.anomaly_trace` block for anomaly monitors over trace aggregations.
- `tsuga_monitor` and `tsuga_slo`: `time_aggregate` (`avg`, `sum`, `min`, `max`, `last`) on aggregation queries, the per-series rollup applied within each time bucket before the cross-series aggregate. When omitted, the API derives a default from the metric type.
- `tsuga_dashboard`: `time_aggregate` on widget queries.
- `tsuga_dashboard`: new `visualization.list_spans` widget listing individual spans, with `query`, `list_columns`, `list_columns_size` and `default_sorting`.
- `tsuga_dashboard`: `default_sorting` on the `list` and `list_connection` widgets. It requires at least one entry — omit the attribute rather than setting it to an empty list.
- `tsuga_dashboard`: `is_cell_wrapped` on the `list`, `list_spans` and `list_connection` widgets, controlling whether cell contents wrap instead of being truncated.
- `tsuga_tag_policy`: `metric-route` asset type.

### Changed

- `tsuga_monitor`: `timeframe` is now validated at plan time (at least 1 minute, and between 5 and 1440 minutes for anomaly monitors), and `dashboard_id` must be between 1 and 250 characters, matching the API.
- `tsuga_notification_silence`: schedule times are now validated at plan time instead of failing on apply.

### Fixed

- `tsuga_dashboard`: the `normalizer.type` validator now accepts `none` and `percent-fraction`.

## [2.2.2] - 2026-07-21

### Fixed

- Fixed an issue related to log route samples.

## [2.2.1] - 2026-07-13

### Added

- Support the `query_string` attribute on notification rules to narrow which alert transitions trigger the rule, matching on the monitor transition group key and the monitor's tags.

## [2.2.0] - 2026-07-07

### Added

- Added the `tsuga_cloud_account` resource to connect AWS and GCP cloud accounts for inventory scanning.

### Fixed

- `tsuga_team` no longer shows `id -> (known after apply)` on an in-place update (e.g. editing the description). The team `id` is now held stable in the plan, so resources referencing it (team memberships, tag policies) no longer show spurious diffs.

## [2.1.6] - 2026-07-02

### Removed

- Removed support for SLOs with a 28-day timeframe.

## [2.1.5] - 2026-07-01

### Added

- Support SLOs.
- Support the `sort_order` (`asc` / `desc`) and `replace_null_with` attributes on `group_by_fields` for monitor and dashboard group-by.

## [2.1.4] - 2026-06-23

### Added

- Support the `field` attribute on the `count` aggregate for monitor and dashboard queries. The `count` block previously had no `field`, so it was silently dropped before being sent to the API. Counting a field on the metrics data source (where the query engine requires a field) is now reachable. `count` without a field stays valid on the logs and traces data sources.

## [2.1.3] - 2026-06-18

### Added

- Support the `log`, `power`, `sqrt`, and `increase` query functions on dashboards.
- Add dashboard visualization types: `gauge`, `distribution`, `heatmap`, `list-log-patterns`, and the connection-based `bar`, `pie`, `top-list`, and `query-value` variants.
- Add the `cpu` data normalizer, `timeseries` smoothing, and graph description alignment fields to dashboards.
- Add the `category` creator subtype for routes.
- Add the `rum-public-token` asset type for tag policies.

## [2.1.2] - 2026-06-17

### Fixed

- Fix the `tsuga_monitor` example.
- Always pass teamOverrideFields for ingestion API keys.

## [2.1.1] - 2026-05-18

### Changed

- Regenerate the provider against the latest API spec.

## [2.0.3] - 2026-05-11

### Changed

- Migrate retention policies' `durationDays` to an integer type.

## [2.0.2] - 2026-04-29

### Changed

- Regenerate the provider from the updated Go SDK / API spec.

## [2.0.1] - 2026-04-22

### Added

- Allow the `percent` normalizer on dashboard queries.

## [2.0.0] - 2026-04-21

### Added

- Add alias examples and time offsets to dashboards.

### Fixed

- Fix provider examples.

## [1.2.3] - 2026-04-07

### Added

- Expose custom usage tags.

### Fixed

- Fix provider examples and configuration.

## [1.2.2] - 2026-03-24

### Changed

- Use a pointer so the monitor `condition` field is non-computed.

## [1.2.1] - 2026-03-24

### Deprecated

- Deprecate the monitor `condition` field.

## [1.2.0] - 2026-03-23

### Added

- Support trace monitors and multiple monitor conditions.

---

Releases up to and including v1.1.1 predate this changelog; see the
[GitHub Releases](https://github.com/tsuga-dev/terraform-provider-tsuga/releases)
page for their artifacts.
