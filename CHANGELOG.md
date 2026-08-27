# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Versions up to and including `1.5.0` are
[legacy KOsync](https://git.obth.eu/atjontv/kosync/-/tree/legacy-main). `2.0.0` is the PocketBase rewrite. The KOReader protocol and the reading data carry
over, see [docs/technical/migration.md](docs/technical/migration.md).

This Project had two version scheme changes:
1. From `YYYY.MINOR.PATCH` to `YY.MM.PATCH` (May 2026)
2. From `YY.MM.PATCH` to `MAJOR.MINOR.PATCH` (August 2026)

Following the current versioning, the old will be shown in `()`.

## [Unreleased]

### Added
- Search on the library page, over titles, series and author names. The words may match in any
  order, and accents and case are ignored on both sides
- Sorting on the library page, by title, by when a book was added, by when it was last read or by
  how far the reading got. The choice is remembered between visits and carries into grouped shelves
- A nightly pass that reads the stored file of every book without a cover and keeps whatever it
  finds, so books uploaded before the reader learned a cover shape gain one without being uploaded
  again. It also runs shortly after startup
- The publisher's blurb, read out of the book's `dc:description` and shown under "About this book"
  on the book page. It also leads the description the OPDS catalog hands a device
- The blurb of every book already in the library, read once on upgrade. A description that somebody
  typed is never overwritten
- A preview of a book, on the book page next to Download: a look inside the file from the browser,
  without opening it on a reader where it would count as reading. It records nothing and keeps no
  position. Chapters are named from the book's own table of contents, the arrow keys and the
  buttons page, and the chapter list opens from the header. See
  [docs/technical/api.md](docs/technical/api.md#book-preview)
- The documentation is published to the
  [project wiki](https://git.obth.eu/atjontv/kosync/-/wikis/home) on every push to `main`. The
  repository stays the original: a page edited in the wiki is overwritten by the next sync, and
  every page says so at the top
- A guide for readers, in `docs/user/`: what KOsync does, setting a device up step by step, and a
  page each for the library, the reading, the statistics and the account. See
  [docs/user/index.md](docs/user/index.md)

### Changed
- The documentation is split in two. What developers and operators need moved to
  `docs/technical/`, the plans with it, and `docs/user/` is the new guide for readers. Links from
  the README and the changelog were updated; a bookmark to a `docs/*.md` file was not
- The description a device sees under "book information" in the catalog leads with the book's own
  blurb when it has one, with where the reading stands after it rather than in its place
- The reading time on the dashboard is written as hours and minutes ("33 h 30 min") rather than as
  a number of minutes, which is the form the book page already used

### Deprecated

### Removed

### Fixed
- Covers are found in books that declare them the way books actually do — a `<meta name="cover">`
  naming the file, a guide reference, a cover page holding the picture, an image the manifest calls
  a cover, or failing all of that the page the book opens on — and not only the two ways the
  standards describe. On a shelf of 273 real books this filled in 15 that showed a placeholder.
  Books already in the library gain their covers from the nightly pass above

### Security
- **BREAKING.** Books and their covers are stored as protected files: a request for the file is now
  checked against the same ownership rule as a request for the book. Until this release the address
  was merely unguessable, which is not the same thing — it never expired and it outlived the
  account. Nothing has to be done on upgrade and no address in the web interface has to be touched
  by hand. A book address saved outside KOsync — a bookmark, a script, an `<img>` elsewhere — stops
  working; the catalog at `/opds` is the supported way for a program to fetch a book

---

## [2.0.1] (26.08.1) - 2026-08-26

### Fixed
- Docker Container used `/pb_data` instead of `/data` as mountable data directory

## [2.0.0] (26.08.0) - 2026-08-17

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

## [1.5.0] (26.05.0) - 2026-05-17

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

## [1.4.0] (2026.06.0) - 2026-02-17

If you are running any version of KOsync before `1.3.0`, you **MUST** update to `1.3.0` first!  
Do **NOT** update from before `1.3.0` to this version (`1.4.0`), otherwise you will have an empty database.

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

## [1.3.0] (2026.05.0) - 2026-02-05

### Added
- SQLite Database (named `kosync.db`) with automatic migration from JSON to SQLite
- `.env` Configuration File (named `kosync.env`)

### Changed

### Deprecated
- `database.json` Database file
- Backup creation and restore (can no longer be done via CLI)

## [1.2.1] (2026.04.1) - 2026-01-27

## Fix
- Pretty Name got lost during progress push

## [1.2.0] (2026.04.0) - 2026-01-27

## Added
- WebUI to view documents and history
- Document Pretty Name that can be set and viewed in WebUI
- New CI pipeline with `staticheck`

## Fix
- Database migration changes are not being persisted correctly

## [1.1.1] (2026.03.1) - 2026-01-25

## Added
- Config `backup_on_startup` to enable automatic backups on startup

## Change
- Return the same HTTP Status code for account registration disabled error as [KORSS]

## Fix
- Progress push not working (Method not allowed)
- Wrong backup file names

## [1.1.0] (2026.03.0) - 2026-01-23

## Added
- Database restore via `--restore <path>` CLI argument
- Manual database backups via `--backup` CLI argument
- Msgpack encoded backups by default, can be changed to JSON via `backup_encoding_type` config option
- Gitlab CI pipeline for validation (SAST and compilation)

## Changed
- Refactor to the [fiber framework]

## Fix
- Issues reported by SAST

## [1.0.2] (2026.02.0) - 2026-01-18

## Added
- Document change history
- Config `store_history` to enable history collection
- Database migration and backup mechanism

## [1.0.1] (2026.01.1) - 2026-01-08

### Added
- Config `enable_debug_log` for verbose logging

### Changed
- Moved all configuration options into a config object

## [1.0.0] (2026.01.0) - 2026-01-08

Initial Release

[Unreleased]: https://git.obth.eu/atjontv/kosync/compare/v2.0.1...development
[2.0.1]: https://git.obth.eu/atjontv/kosync/compare/v2.0.0...v2.0.1
[2.0.0]: https://git.obth.eu/atjontv/kosync/compare/v1.5.0...v2.0.0
[1.5.0]: https://git.obth.eu/atjontv/kosync/compare/v1.4.0...v1.5.0
[1.4.0]: https://git.obth.eu/atjontv/kosync/compare/v1.3.0...v1.4.0
[1.3.0]: https://git.obth.eu/atjontv/kosync/compare/v1.2.1...v1.3.0
[1.2.1]: https://git.obth.eu/atjontv/kosync/compare/v1.2.0...v1.2.1
[1.2.0]: https://git.obth.eu/atjontv/kosync/compare/v1.1.1...v1.2.0
[1.1.1]: https://git.obth.eu/atjontv/kosync/compare/v1.1.0...v1.1.1
[1.1.0]: https://git.obth.eu/atjontv/kosync/compare/v1.0.2...v1.1.0
[1.0.2]: https://git.obth.eu/atjontv/kosync/compare/v1.0.1...v1.0.2
[1.0.1]: https://git.obth.eu/atjontv/kosync/compare/v1.0.0...v1.0.1
[1.0.0]: https://git.obth.eu/atjontv/kosync/-/releases/v1.0.0

[KORSS]: https://github.com/koreader/koreader-sync-server
[fiber framework]: https://gofiber.io/
