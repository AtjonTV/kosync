# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Document and History deletion (Backend and WebUI)
- Login Modal, replacing the HTTP-Basic-Auth that caused issues on Firefox for Android
- JMP (JSON Messaging Protocol) for WebUI communication (replacing the KOsync Socket API)
- `build.go` tool for easier compilation and running
- `AI-Usage.md` for transparency regarding AI usage

### Changed
- WebUI: Migrated from KOsync Socket API to JMP using `jmp-client-js`
- Updated Fiber to v3.2.0
- Updated Go to 1.26.0
- Updated dependencies
- Extracted generic bits into resuable packages in `pkg/`
- Improved test coverage

### Deprecated

### Removed

### Fixed
- WebUI: Fix broken default sort

### Security

---

## [2026.06.0] - 2026-02-17

If you are running any version of KOsync before `2026.05.0`, you **MUST** update to `2026.05.0` first!  
Do **NOT** update from before `2026.05.0` to this version (`2026.06.0`), otherwise you will have an empty database.

### Added
- Extensive Debug Logging
- Write to Logfile and Logfile path settings
- Experimental WebSocket API with RPC and PubSub
- WebUI updates automatically upon progress sync (PubSub)
- JSON Web Tokens for `Authorization: Bearer`
- New Environment Variables:
    - `LOG_TO_FILE`: Enable writing logs to a file.
    - `LOG_FILE`: Path to the log file (defaults to `./kosync.log`)
    - `ENABLE_WEBSOCKET_API`: Enable WebSocket API for WebUI (RPC/PubSub)
    - `PRINT_CRYPTO_KEYS`: Print JWT Keys in PEM format at startup
    - `CRYPTO_KEYS_SEED`: 32-Character Seed for JWT Keys (will be random if unset or empty)
    - `JWT_DURATION`: JWT validity duration in seconds (defaults to 6 hours)
- Automatic backups before applying migrations
- `--restore` Argument to restore the database (see doc/backup.md for details)

### Changed
- Migrated to Fiber v3
- Updated Go to 1.25.7
- Updated Bun to 1.3.9
- Updated dependencies
- `last_read_at` is now a float for sub-seconds
- WebUI uses JWT instead of username+password_hash
- WebUI login is time-bound. JWT's expire after `JWT_DURATION` seconds AND after KOsync restart (unless a CRYPTO_KEYS_SEED is set)
- **BREAKING**: Environment Variables have been renamed:
    - `ENABLE_IP_VALIDATION` -> `PROXY_IP_VALIDATION`
- **BREAKING**: The docker image no uses `1000:1000` as user instead of `root:root`

### Removed
- **BREAKING**: Migration from `database.json` to `kosync.db`

### Fixed
- Unique constraint dead-lock between documents and document_history with multiple causes

## [2026.05.0] - 2026-02-05

### Added
- SQLite Database (named `kosync.db`) with automatic migration from JSON to SQLite
- `.env` Configuration File (named `kosync.env`)

### Changed

### Deprecated
- `database.json` Database file
- Backup creation and restore (can no longer be done via CLI)

## [2026.04.1] - 2026-01-27

## Fix
- Pretty Name got lost during progress push

## [2026.04.0] - 2026-01-27

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

[Unreleased]: https://git.obth.eu/atjontv/kosync/compare/v2026.06.0...main
[2026.06.0]: https://git.obth.eu/atjontv/kosync/compare/v2026.05.0...v2026.06.0
[2026.05.0]: https://git.obth.eu/atjontv/kosync/compare/v2026.04.1...v2026.05.0
[2026.04.1]: https://git.obth.eu/atjontv/kosync/compare/v2026.04.0...v2026.04.1
[2026.04.0]: https://git.obth.eu/atjontv/kosync/compare/v2026.03.1...v2026.04.0
[2026.03.1]: https://git.obth.eu/atjontv/kosync/compare/v2026.03.0...v2026.03.1
[2026.03.0]: https://git.obth.eu/atjontv/kosync/compare/v2026.02.0...v2026.03.0
[2026.02.0]: https://git.obth.eu/atjontv/kosync/compare/v2026.01.1...v2026.02.0
[2026.01.1]: https://git.obth.eu/atjontv/kosync/compare/v2026.01.0...v2026.01.1
[2026.01.0]: https://git.obth.eu/atjontv/kosync/-/releases/v2026.01.0

[KORSS]: https://github.com/koreader/koreader-sync-server
[fiber framework]: https://gofiber.io/
