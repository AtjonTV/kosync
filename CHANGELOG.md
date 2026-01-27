# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Deprecated

### Removed

### Fixed

### Security


---

## [2026.04.1] - 2026-04-27

## Fix
- Pretty Name got lost during progress push

## [2026.04.0] - 2026-04-27

## Added
- WebUI to view documents and history
- Document Pretty Name that can be set and viewed in WebUI
- New CI pipeline with `staticheck`

## Fix
- Database migration changes are not being persisted correctly

## [2026.03.1] - 2026-01-25

## Added
- Config `backup_on_startup` to enable automatic backups on startup

## Change
- Return the same HTTP Status code for account registration disabled error as [KORSS]

## Fix
- Progress push not working (Method not allowed)
- Wrong backup file names

## [2026.03.0] - 2026-01-23

## Added
- Database restore via `--restore <path>` CLI argument
- Manual database backups via `--backup` CLI argument
- Msgpack encoded backups by default, can be changed to JSON via `backup_encoding_type` config option
- Gitlab CI pipeline for validation (SAST and compilation)

## Changed
- Refactor to the [fiber framework]

## Fix
- Issues reported by SAST

## [2026.02.0] - 2026-01-18

## Added
- Document change history
- Config `store_history` to enable history collection
- Database migration and backup mechanism

## [2026.01.1] - 2026-01-08

### Added
- Config `enable_debug_log` for verbose logging

### Changed
- Moved all configuration options into a config object

## [2026.01.0] - 2026-01-08

Initial Release

[Unreleased]: https://git.obth.eu/atjontv/kosync/compare/v2026.04.1...main
[2026.04.1]: https://git.obth.eu/atjontv/kosync/compare/v2026.04.0...v2026.04.1
[2026.04.0]: https://git.obth.eu/atjontv/kosync/compare/v2026.03.1...v2026.04.0
[2026.03.1]: https://git.obth.eu/atjontv/kosync/compare/v2026.03.0...v2026.03.1
[2026.03.0]: https://git.obth.eu/atjontv/kosync/compare/v2026.02.0...v2026.03.0
[2026.02.0]: https://git.obth.eu/atjontv/kosync/compare/v2026.01.1...v2026.02.0
[2026.01.1]: https://git.obth.eu/atjontv/kosync/compare/v2026.01.0...v2026.01.1
[2026.01.0]: https://git.obth.eu/atjontv/kosync/-/releases/v2026.01.0

[KORSS]: https://github.com/koreader/koreader-sync-server
[fiber framework]: https://gofiber.io/
