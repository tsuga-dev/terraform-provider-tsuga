# Changelog

All notable changes to the Tsuga Terraform provider are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
