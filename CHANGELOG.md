# Changelog

All notable changes to the Tsuga Terraform provider are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
