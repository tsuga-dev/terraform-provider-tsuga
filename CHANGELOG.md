# Changelog

All notable changes to the Tsuga Terraform provider are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.1.4] - 2026-06-23

# Added
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
