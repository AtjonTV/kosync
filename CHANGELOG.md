# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

Versions up to and including `26.05.0` are
[legacy KOsync](https://git.obth.eu/atjontv/kosync/-/tree/legacy-main), the Fiber and hand-written
SQLite server. `26.08.0` is the PocketBase rewrite. The KOReader protocol and the reading data carry
over, see [docs/migration.md](docs/migration.md).

## [Unreleased]

### Added
- Search on the library page, over titles, series and author names. Every word has to appear
  somewhere but they need not appear together, so "child killing" finds Lee Child's "Killing
  Floor"; accents and case are ignored on both sides, and an author is found under the spelling
  in the file as well as the one on the shelves
- Sorting on the library page, by title, by when a book was added, by when it was last read or by
  how far the reading got. The choice is remembered between visits, as the grouping already was,
  and it carries into the shelves a grouping makes rather than applying only to the ungrouped view

### Changed
- The reading time on the dashboard is broken into hours and minutes ("33 h 30 min") rather than
  written as a number of minutes: a month of reading runs to four figures of them, and "2010 min"
  has to be divided in the head before it means anything. This is the form the book page has always
  used for "Time spent", and both now share one formatter

### Deprecated

### Removed

### Fixed

### Security

---

## [26.08.1] - 2026-08-26

### Fixed
- Docker Container used `/pb_data` instead of `/data` as mountable data directory

## [26.08.0] - 2026-08-17

A rewrite on top of [PocketBase](https://pocketbase.io). Everything a device does is unchanged, so
an existing KOReader keeps syncing after the address gains one path segment; everything around the
sync is new. A legacy database is imported with `kosync import-legacy <path>`.

### Added
- **Library.** Upload an EPUB and the server reads title, authors, language, identifiers, series,
  subjects and cover out of it, computes both of KOReader's document hashes and estimates a page
  count from the reader's own progress
- **Document matching**, so an uploaded book finds the reading already done against it, with
  per-book statistics
- **Document merging**, folding two documents for the same book into one that keeps syncing
- **OPDS 2.0 catalog** at `/opds`, browsable by author, series, language and collection, with
  covers, thumbnails and acquisition links a device can download from
- **Custom collections** of books, ordered by hand, exposed both in the web interface and as OPDS
  navigation feeds
- **WebDAV target** at `/webdav` for KOReader's statistics plugin, so a device syncs its own
  page-turn database here with the credential it already has; the same database can also be
  uploaded by hand
- **Achievements**, drawn as comic-book cats, with tiers, earned from the statistics and never
  revoked
- **Mail**, all opt-in per account: achievement notifications and weekly or monthly summaries of an
  account's own reading, sent at eight in the morning in that account's timezone
- **Per-account storage quota** for uploaded books, and a nightly job that reconciles the recorded
  usage with what is on disk
- Author name normalisation, so "Sapkowski, Andrzej" and "Andrzej Sapkowski" are one author
- Device credentials can be named, listed and revoked one at a time
- Reading statistics are precomputed per day, rolled up per month, and honour the account's
  timezone; retention and rollup are configurable
- Legacy import as a CLI command: `kosync import-legacy <path>`
- Web interface: library-first dashboard, library grid with grouping by author, series or language,
  documents and their history, statistics, achievements, account settings and a setup guide for a
  new device
- Documentation under `docs/`: API, database, configuration, analytics, build, migration and the
  rewrite plan itself
- `CHANGELOG.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `.editorconfig` and `.gitmessage`

### Changed
- **BREAKING**: KOReader is pointed at `https://your-host/koreader` instead of `https://your-host`.
  The username and password stay the same
- **BREAKING**: an account and a KOReader credential are two different things. The account signs in
  to the web interface with an email address and a bcrypt password and can recover it by mail; the
  credential is what a device sends, and can never sign in to the API
- **BREAKING**: registration happens in the web interface, not from the device, because a credential
  has to belong to an account
- **BREAKING**: the Docker image keeps the database, the uploads and the backups in `/data`
- Configuration is split: KOsync's own options live in `kosync.env`, everything belonging to
  PocketBase (listen address, data directory, SMTP, backups, rate limits, token lifetimes) is set
  with its flags or in the superuser interface at `/_/`
- Backups are PocketBase's, local or S3, on a schedule or on demand
- Realtime updates are an ordinary PocketBase subscription
- Statistics are precomputed on write instead of derived by a recursive query on every request
- The web interface is Vue 3, PrimeVue 4 and Tailwind 4, built with Bun and embedded into the binary
- Go 1.26, PocketBase 0.39

### Removed
- **BREAKING**: the JMP (JSON Messaging Protocol) WebSocket API. PocketBase's realtime subscriptions
  replace it
- The hand-written database layer, the migrations runner and the JWT handling, all of which
  PocketBase now provides
- `--backup` and `--restore`; backups are taken and restored in the superuser interface
- `AI-Usage.md`. AI attribution is carried by the `AI-Agent` and `AI-Model` commit trailers, which
  cannot go stale the way a per-file table does

### Security
- MD5 appears only where KOReader's protocol demands it: as the identity of a document, and as the
  hash a device is allowed to send instead of a password. What is stored is a bcrypt hash of that
- Uploaded EPUBs, covers and statistics databases are served only to the account that owns them,
  enforced by PocketBase collection rules and covered by tests
- CI runs `gofmt`, `go vet`, `staticcheck`, Bearer, `wwhrd`, `govulncheck` and `bun audit`

## [26.05.0] - 2026-05-17

### Added
- Read statistics chart (updates, progress increase, and reading time) in WebUI
- Real-time statistics updates via PubSub
- `CODE_STYLE.md` defining Go and TypeScript/Vue coding standards
- Document and History deletion (Backend and WebUI)
- Login Modal, replacing the HTTP-Basic-Auth that caused issues on Firefox for Android
- JMP (JSON Messaging Protocol) for WebUI communication (replacing the KOsync Socket API)
- `build.go` tool for easier compilation and running
- `AI-Usage.md` for transparency regarding AI usage

### Changed
- Unified all database timestamps to 100 microsecond units for consistency and sub-second precision
- WebUI: Migrated from KOsync Socket API to JMP using `jmp-client-js`
- Updated Fiber to v3.2.0
- Updated Go to 1.26.0
- Updated dependencies
- Extracted generic bits into resuable packages in `pkg/`
- Improved test coverage

### Fixed
- WebUI: Fix broken default sort
- Type conversion bugs in JMP API

### Security
- Fixed document ownership vulnerability where any user could create, update, or delete other users' documents

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

[Unreleased]: https://git.obth.eu/atjontv/kosync/compare/v26.08.1...development
[26.08.1]: https://git.obth.eu/atjontv/kosync/compare/v26.08.0...v26.08.1
[26.08.0]: https://git.obth.eu/atjontv/kosync/compare/v26.05.0...v26.08.0
[26.05.0]: https://git.obth.eu/atjontv/kosync/compare/v2026.05.0...v26.05.0
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
