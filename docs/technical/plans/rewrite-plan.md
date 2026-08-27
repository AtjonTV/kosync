<!--
File:        docs/plans/rewrite-plan.md
Project:     https://git.obth.eu/atjontv/kosync
Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->

# KOsync v2 — Rewrite Plan (PocketBase)

Status: proposal, no code written yet.
Target: replace the Fiber v3 + hand-rolled SQLite + JMP-WebSocket server with PocketBase v0.39.9 as
framework, database, auth provider, realtime transport and mail sender, keeping the KOReader protocol
compatible and the WebUI look identical.

---

## 1. What the legacy app actually is

Inventory of `kosync_legacy` (~6.4k lines Go, ~1.3k lines Vue/TS), so nothing gets silently dropped.

| Area | Legacy implementation | Fate in v2 |
| --- | --- | --- |
| HTTP framework | Fiber v3, routes wired in `internal/kosync/kosync.go` | PocketBase router (`se.Router`) |
| DB | `modernc.org/sqlite` + hand-written SQL, custom migration runner (`schema_versions`, numbered `.sql` files) | PocketBase collections + Go migrations |
| Backups | `database_backup.go` (SQLite online backup API), auto-backup before migration, `--restore` flag | PocketBase built-in backups (`/api/backups`, S3 optional, superuser UI) |
| Auth (KOReader) | `x-auth-user` + `x-auth-key` (MD5 hex) compared in plaintext against `users.password` | `koreader_accounts` auth collection, MD5 hex bcrypt-hashed at rest |
| Auth (WebUI) | Same MD5 credentials → self-issued Ed25519 JWT (`crypt.go`), basic-auth bridge at `/api/auth.basic` | PocketBase `users` auth collection (bcrypt, real password rules, verification, reset) |
| KOReader API | `/users/auth`, `/users/create`, `/syncs/progress` (PUT/GET) | Same shapes under `/koreader/...` |
| WebUI API | `/api/documents.all`, `.update`, `.delete`, `.history.delete`, `.history.restore`, `/api/statistics.read` | PocketBase collection CRUD + 3 custom routes |
| Realtime | Custom WebSocket + custom JMP protocol (`pkg/jmp`, `pkg/jmp-client-js`), topics `user.documents`, `user.statistics` | PocketBase realtime (SSE) subscriptions — **JMP and its JS client are deleted** |
| Statistics | Two large recursive-CTE queries computed **on every request** (`database_statistics.go`) | Precomputed rows in `reading_days`, async worker + cron retention |
| Config | `godotenv` + struct-tag env decoding (`pkg/decode`, `pkg/environ`) | Same mechanism, much smaller struct (PocketBase owns listen addr, data dir, SMTP) |
| Logging | Custom `Klog` wrapper | PocketBase `app.Logger()` (structured, persisted in `_logs`, visible in superuser UI) |
| WebUI | Vue 3 + Pinia + PrimeVue 4 (Aura, blue preset) + Tailwind 4 + primeicons + chart.js | **Identical stack and styling**, PocketBase JS SDK instead of `fetchApi`/JMP |
| Bundling | `go:generate bun build` → `internal/webui/public` → `//go:embed` | Same approach |
| Build | `go run build.go` | Same, adapted to `server/` module root |
| CI | GitLab: bearer, wwhrd, staticcheck, go test, vitest, govulncheck, bun audit, compile, buildx, sonarqube | Same skeleton, extended |

Legacy quirks worth naming now, because v2 changes them deliberately:

1. **Timestamp unit.** Legacy stores `last_read_at` in units of 1/10000 s (`UnixMicro()/100`) and returns
   that raw number to KOReader as `timestamp`. KOReader's protocol expects Unix **seconds**. v2 stores
   PocketBase `date` values (ms precision) and returns proper Unix seconds. Migration divides by 10 to get ms.
2. **Passwords at rest.** Legacy stores the KOReader MD5 in plaintext. v2 bcrypt-hashes it (§5.1).
3. **`if(...)` in SQL.** The upsert uses SQLite's `if()` extension quirk; v2 does that logic in Go.
4. **History soft-delete.** Legacy sets `deleted_at` on history rows; nothing ever un-deletes them, and
   "restore" means *re-apply that state to the document*. v2 hard-deletes history rows and keeps
   restore-as-re-apply. Simpler, same user-visible behaviour.

---

## 2. Target architecture

Single Go binary, single module at `kosync/server`, PocketBase embedded as a library (not a fork).

```
kosync/
├── server/
│   ├── go.mod                      module git.obth.eu/atjontv/kosync
│   ├── main.go                     PocketBase app wiring only
│   ├── build.go                    //go:build ignore — `go run build.go`
│   ├── internal/
│   │   ├── config/                 env config (godotenv + struct tags, ported from pkg/decode)
│   │   ├── migrations/             Go migrations: collections, rules, indexes, seed data
│   │   ├── koreader/               /koreader/* routes, MD5 header auth, credential cache
│   │   ├── kosyncapi/              custom /api/kosync/* routes (credentials, restore, merge)
│   │   ├── analytics/              queue drain worker, recompute SQL, cron rollup/retention
│   │   ├── importer/               `import-legacy` cobra command
│   │   ├── hooks/                  record hooks (enqueue analytics, guard registration)
│   │   └── webui/                  //go:embed public/* + go:generate bun build
│   ├── pkg/decode/                 ported verbatim from legacy (still useful, has tests)
│   └── testdata/                   pb_data fixture + legacy.db fixture for the importer
├── webui/                          Vue 3 + PrimeVue + Tailwind + PocketBase JS SDK
└── docs/                           api.md, config.md, database.md, analytics.md, migration.md
```

`main.go` stays thin: register migrations, config, hooks, routes, cron, worker, static, then `app.Start()`.
PocketBase's own CLI (`serve`, `migrate`, `superuser`) comes for free, and `import-legacy` is added to
`app.RootCmd`.

External tooling stays at **Go + Bun** (plus Docker for release images), as required.

---

## 3. Data model

All collections are created by **Go migrations** (`server/internal/migrations`) so the schema is versioned
in source, reviewable in a diff, and applied automatically on boot. No JS hooks, no `pb_migrations`
JavaScript. `migratecmd` is registered with `TemplateLang: migratecmd.TemplateLangGo` so future schema
edits made in the superuser UI can be exported as Go files.

### 3.1 `users` — WebUI accounts (PocketBase default auth collection)

| Field | Type | Notes |
| --- | --- | --- |
| `email` | email | required, unique; used for recovery + achievement mails (later phase) |
| `password` | password | bcrypt, PocketBase rules |
| `verified` | bool | PocketBase verification flow |
| `name` | text | display name, optional |
| `settings` | json | UI prefs (theme, default chart range) |

Rules: `create` public (WebUI registration), everything else self-scoped
(`id = @request.auth.id`). If `DISABLE_REGISTRATION=true`, an `OnRecordCreateRequest("users")` hook
rejects with 403 — an env flag is more operationally convenient than editing the rule.

### 3.2 `koreader_accounts` — device credentials (auth collection)

This is the core of requirement 1: **KOReader credentials are separate records, owned by a `users` record.**

| Field | Type | Notes |
| --- | --- | --- |
| `username` | text | required, **unique**, the KOReader `x-auth-user` |
| `password` | password | the value is the **MD5 hex** KOReader sends; PocketBase bcrypt-hashes it |
| `owner` | relation → `users` | required, cascade delete, `maxSelect: 1` |
| `label` | text | e.g. "Kobo Clara", shown in the UI |
| `enabled` | bool | lets a user revoke a device without deleting its history |
| `last_used` | date | updated (throttled) by the auth middleware |

Auth options: `PasswordAuth.IdentityFields = ["username"]`, email not required and hidden, OAuth2 / MFA /
OTP / verification all disabled, `AuthRule` set so disabled accounts cannot authenticate.

Rules: `list`/`view`/`update`/`delete` = `owner.id = @request.auth.id`; `create` = superuser only —
creation goes through a custom route so the server, not the browser, decides what MD5 means (§4.2).
The `password` field is never readable via the API.

### 3.3 `documents` — current progress state

| Field | Type | Notes |
| --- | --- | --- |
| `owner` | relation → `users` | required, cascade delete |
| `document` | text | the KOReader document hash (binary or filename method) |
| `title` | text | editable in the WebUI |
| `current_location` | text | xpointer / page fragment |
| `progress` | number | 0..1 |
| `last_device` | text | |
| `last_device_id` | text | |
| `last_read_at` | date | ms precision |
| `source_account` | relation → `koreader_accounts` | which credential last pushed, optional |
| `book` | relation → `books` | **phase 8**, empty until EPUB support lands |

Unique index on `(owner, document)`. Index on `(owner, last_read_at)` for the dashboard.
Rules: all five = `owner.id = @request.auth.id`. That single rule also governs **realtime**
subscriptions, which is what replaces the JMP `user.documents` topic.

### 3.4 `document_history` — every superseded state

Same payload fields plus `document_ref` (relation → `documents`, cascade delete) and `owner`.
Written by the server whenever a document row is superseded. Rules: `list`/`view`/`delete` owner-scoped,
`create`/`update` superuser only. Index on `(document_ref, last_read_at desc)`.

### 3.5 `reading_days` — precomputed daily analytics (requirement 6)

| Field | Type |
| --- | --- |
| `owner` | relation → `users` |
| `date` | text `YYYY-MM-DD` |
| `update_count` | number |
| `progress_increase` | number (percentage points) |
| `reading_time` | number (seconds) |
| `documents_touched` | number |
| `pages_read` | number (the sum of the day's book rows — §16.5, built in phase 12) |
| `computed_at` | date |

Unique index `(owner, date)`. `list`/`view` owner-scoped, writes superuser only. The WebUI reads and
**subscribes** to this collection directly — no statistics endpoint, no request-time CTE, and the
`user.statistics` JMP topic disappears for free.

### 3.6 `reading_months` — retention rollup

`owner`, `month` (`YYYY-MM`), `update_count`, `progress_increase`, `reading_time`, `days_active`,
`pages_read`. Unique `(owner, month)`. Produced by the retention cron from `reading_days` rows older than
`ANALYTICS_RETENTION_DAYS`.

### 3.7 `analytics_queue` — recompute work items

`owner`, `date`, `enqueued_at`, unique `(owner, date)`. Superuser-only rules; invisible to clients.
DB-backed rather than an in-memory channel so that (a) a burst of pushes for the same day collapses into
one recompute via the unique index, and (b) pending work survives a restart.

### 3.8 Phase-8+ collections (designed now, built later)

- `books` — `owner`, `file` (file field, `application/epub+zip`), `cover` (file field with
  `thumbs: ["100x150", "200x300"]`), `title`, `authors`, `identifiers` (json: ISBN/UUID), `language`,
  `page_count`, `word_count`, `content_hash`, `koreader_hashes` (json: precomputed binary+filename hashes
  used for auto-matching). See §16 for the whole library design.
- `reading_book_days` — the §3.5 measures keyed by `(owner, date, book)` instead of `(owner, date)`,
  computed by the same worker. §16.5 explains why it is a separate table rather than a regrouping.
- `achievements` — `key`, `name`, `description`, `icon` (SVG file), `kind` (`pages|books|streak`),
  `threshold`, `repeatable` (bool).
- `user_achievements` — `owner`, `achievement`, `tier` (nth time earned), `awarded_at`, `context` (json).
  Unique `(owner, achievement, tier)` makes awarding idempotent, which matters for requirement 6's
  "3500 pages ⇒ 3× the 1k achievement".

---

## 4. HTTP surface

### 4.1 KOReader compatibility (requirement 2)

Registered as a group with the MD5 auth middleware; **PocketBase JWTs are not accepted here**.

| Method | Route | Behaviour |
| --- | --- | --- |
| GET | `/koreader/users/auth` | 200 on valid credentials, 401 otherwise |
| POST | `/koreader/users/create` | **always 402** + message pointing at the WebUI (requirement 7) |
| PUT | `/koreader/syncs/progress` | upsert `documents`, append `document_history`, enqueue analytics |
| GET | `/koreader/syncs/progress/{document}` | current state, or 404 |

Request/response bodies are byte-compatible with the legacy server and KORSS, except the `timestamp`
correction in §1. Users point KOReader's "custom sync server" at `https://host/koreader`.

`POST /koreader/users/create` returning 402 is exactly what the legacy server did with
`DISABLE_REGISTRATION=true`, so KOReader already renders a sensible error. `docs/` and the WebUI's setup
instructions get updated: **register in the WebUI first, create a KOReader credential, then log in on the
device.**

### 4.2 Custom app APIs

Only for things PocketBase's generated CRUD genuinely cannot express:

| Method | Route | Why it can't be plain CRUD |
| --- | --- | --- |
| POST | `/api/kosync/koreader-accounts` | server computes MD5 from the plaintext, so a browser bug can never store a non-MD5 "password" |
| POST | `/api/kosync/koreader-accounts/{id}/password` | same, for rotation |
| POST | `/api/kosync/documents/{id}/restore-history/{historyId}` | multi-record transaction: current state → history, history state → current |
| POST | `/api/kosync/documents/merge` | **phase 11** — fold N documents into one, re-parent history, recompute analytics; see §23 |

All guarded by `apis.RequireAuth("users")` plus explicit ownership checks.

### 4.3 Everything else is PocketBase

List documents, edit a title, delete a document, delete a history entry, read statistics, register, log
in, reset a password, subscribe to live updates — all through `pocketbase` JS SDK calls against the
collections above. This is what makes requirement 3 (drop the WebSocket/JMP stack) a deletion rather than
a reimplementation.

---

## 5. Authentication design

### 5.1 KOReader MD5 (decision: bcrypt over the MD5)

KOReader hashes the password with MD5 client-side and sends the hex in `x-auth-key`. v2 treats **that hex
string as the password value** of a `koreader_accounts` record, so PocketBase bcrypt-hashes it at rest.
Verification is `record.ValidatePassword(md5hex)`.

- Attack surface: a database leak yields bcrypt hashes, not usable KOReader credentials.
- Consequence: the stored value can never be shown back to the user. The WebUI therefore offers
  *set / rotate*, never *reveal* — the plaintext is displayed exactly once, at creation time.
- MD5 hex is 32 chars, comfortably above PocketBase's 8-char minimum.

### 5.2 The bcrypt-per-push problem — and the fix

This is the one performance trap in the design and it must be handled in phase 1, not later.

KOReader is typically configured to push **every 2 pages**. Every push would otherwise cost a full bcrypt
verification (~50–100 ms of CPU at PocketBase's default cost). With a handful of active readers that
saturates a small VPS.

Mitigation: an in-process credential cache in `internal/koreader` — key `username + md5hex` (the md5 is
itself hashed before use as a key), value `{accountId, ownerId, enabled}`, TTL `KOREADER_AUTH_CACHE_TTL`
(default 300 s), bounded size with LRU eviction, invalidated by `OnRecordAfterUpdateSuccess` /
`AfterDeleteSuccess` hooks on `koreader_accounts`. A **failed** verification is never cached, so brute
force still pays full bcrypt cost. This is explicitly covered by tests (cache hit, TTL expiry,
invalidation on password change, no caching of failures).

### 5.3 Brute force / transport

`/koreader/*` is added to PocketBase's rate-limit rules (settings-driven, e.g. 30 req/min per IP for
auth, higher for progress). MD5 over the wire is only as safe as the transport, so the docs keep the
existing "put Caddy in front" guidance and the README states plainly that HTTPS is required.

---

## 6. Analytics pipeline (requirement 6)

### 6.1 Why

The legacy `GetReadStatistics` runs a 6-CTE recursive query over `documents ∪ document_history` on every
dashboard load and on every websocket announce. It is O(all history) per request and gets slower forever.

### 6.2 Write path

1. Progress arrives (KOReader push, WebUI edit, history restore, merge).
2. A hook enqueues `(owner, date)` into `analytics_queue`. The unique index collapses duplicates, so 200
   pushes on one day produce one work item.
3. A single worker goroutine — started in `OnServe`, stopped in `OnTerminate` — drains the queue every
   `ANALYTICS_WORKER_INTERVAL` (default 5 s), oldest first, bounded batch size.
4. For each item it recomputes that one `(owner, date)` from source data and upserts `reading_days`.
5. PocketBase realtime pushes the updated row to any subscribed browser. No custom announce code.

### 6.3 The computation

The legacy CTE is ported, with the day scoped to a single date (cheap) instead of a recursive date series:

- `update_count` — distinct `last_read_at` values that day across current + history.
- `progress_increase` — per document, `max(progress that day) − max(progress before that day)`, summed,
  ×100 for percentage points. (Legacy semantics preserved, including the clamp at 0 for re-reads.)
- `reading_time` — sum of gaps between consecutive updates where `0 < Δ < ANALYTICS_SESSION_GAP_SECONDS`
  (default 300 s, was a hard-coded 300 s in legacy). This is a heuristic and `docs/analytics.md` will say so.
- `pages_read` — `progress_increase × book.page_count` once a book is linked; 0 otherwise (phase 8).

Timezone: computed in **UTC**, as legacy did. A per-user timezone is noted as a possible later refinement;
changing it silently would rewrite everyone's history.

### 6.4 Retention (requirement 6, second half)

Two cron jobs registered on `app.Cron()`:

- `analytics.retention` — daily 03:15. `reading_days` rows older than `ANALYTICS_RETENTION_DAYS`
  (default 90) are either folded into `reading_months` and deleted (`ANALYTICS_RETENTION_MODE=aggregate`,
  the default) or just deleted (`=delete`). Idempotent: re-running a month recomputes the rollup rather
  than double-counting.
- `analytics.reconcile` — weekly. Re-enqueues the last N days for every user with recent activity, to heal
  drift from crashes or a missed enqueue. Cheap because it reuses the normal queue.

Both are unit-tested with a frozen clock and a seeded dataset.

---

## 7. Realtime

Delete `pkg/jmp`, `pkg/jmp-client-js`, `internal/kosync/api_socket.go`, `websocket.go`, `rpc_util.go`, and
`docs/websocket.md`. The WebUI subscribes via the SDK:

```
pb.collection('documents').subscribe('*', handler)      // was JMP topic user.documents
pb.collection('reading_days').subscribe('*', handler)   // was JMP topic user.statistics
```

Access control is the collection `list` rule, so cross-user leakage is prevented by the same rule that
protects REST — one place to get right, and it's covered by tests. `DISABLE_WEBSOCKET_API` disappears as
a config option.

---

## 8. WebUI

### 8.1 Kept exactly

PrimeVue 4 with the Aura preset and the blue `KOsyncPreset`, `darkModeSelector: '.p-dark'`, Tailwind 4 +
`tailwindcss-primeui`, primeicons, Pinia, vue-router, chart.js via `primevue/chart`. `main.css`,
`main.ts` theme setup, and the visual structure of `TopBar`, `DashboardMetrics`, `ReadStatisticsChart`,
`DocumentsList` (grid/list toggle), `HistoryList` are carried over largely as-is. The bootstrap project's
current `webui/` scaffold (the Vite starter with `HelloWorld.vue`) is replaced by the legacy tree plus the
new stores; its eslint/oxlint/prettier config is kept, since it's an improvement over the legacy setup.

### 8.2 Rewritten (requirement 5)

| Legacy | v2 |
| --- | --- |
| `src/api.ts` (`fetchApi`, bearer JWT) | `src/pb.ts` — a single `new PocketBase(baseUrl)`; `pb.authStore` handles persistence and refresh |
| `stores/user.ts` (md5 login, JWT in localStorage, manual `isLoggedIn` polling) | `stores/auth.ts` — `pb.collection('users').authWithPassword`, reactive `pb.authStore.onChange` |
| `stores/sync.ts` (JMP client, RPC calls, manual pubsub reconciliation) | `stores/documents.ts` + `stores/stats.ts` — `getFullList` + `subscribe`, no hand-written merge logic |
| `?token=` URL bridge from `/api/auth.basic` | gone; a normal login form |
| `blueimp-md5` in the browser | gone; MD5 is computed server-side (§4.2) |

### 8.3 New screens

- **Register** — email + password, PocketBase verification mail.
- **Account → KOReader credentials** — list, create (label + password, shown once), rotate, enable/disable,
  delete; each row shows `last_used` and the setup snippet for KOReader.
- **Setup guide** — updated for the `/koreader` base URL and the register-in-WebUI-first flow.

### 8.4 Routing and serving

The SPA is served at `/` with `indexFallback: true`; PocketBase keeps `/_/` (superuser UI) and `/api/*`.
KOReader's base URL is `https://host/koreader`. `ENABLE_WEBUI=false` unregisters the static route (parity
with the legacy `--webui` flag).

### 8.5 Bundling (unchanged requirement 2 of "what stays")

`server/internal/webui/webui.go` keeps the `go:generate` → `bun build --outDir ../internal/webui/public`
→ `//go:embed public/*` chain and serves it with `apis.Static(webui.FS(), true)`. `go run build.go`
still produces one self-contained executable.

---

## 9. Configuration

PocketBase owns listen address (`--http`), data dir (`--dir`), SMTP, backups, rate limits, token
lifetimes. Only KOsync-specific settings stay in `kosync.env` (same `godotenv` + struct-tag mechanism,
`pkg/decode` ported with its tests):

| Variable | Default | Purpose |
| --- | --- | --- |
| `ENABLE_WEBUI` | `false` | serve the embedded SPA |
| `DISABLE_REGISTRATION` | `false` | block WebUI signup on private instances |
| `ANALYTICS_RETENTION_DAYS` | `90` | age at which daily rows are rolled up/deleted |
| `ANALYTICS_RETENTION_MODE` | `aggregate` | `aggregate` \| `delete` |
| `ANALYTICS_WORKER_INTERVAL` | `5s` | queue drain interval |
| `ANALYTICS_SESSION_GAP_SECONDS` | `300` | reading-time heuristic threshold |
| `KOREADER_AUTH_CACHE_TTL` | `300s` | credential cache TTL (§5.2), `0` disables |

Dropped: `DATABASE_FILE`, `LISTEN_ADDRESS`, `LOG_*`, `DISABLE_WEBSOCKET_API`, `PRINT_CRYPTO_KEYS`,
`CRYPTO_KEYS_SEED`, `JWT_DURATION`, `ENABLE_TRUSTED_PROXIES`/`TRUSTED_PROXIES`/`PROXY_IP_VALIDATION`
(PocketBase has its own trusted-proxy setting). `docs/config.md` documents the mapping for upgraders.

---

## 10. Legacy data import

A cobra command on `app.RootCmd`:

```
kosync import-legacy --file ./kosync.db [--dry-run] [--owner-email you@example.com] [--include-deleted]
```

Mapping:

| Legacy | v2 |
| --- | --- |
| `users` row | `koreader_accounts` (username kept, stored MD5 fed straight into `SetPassword`, so devices keep working untouched) |
| — | one `users` account per legacy user, unless `--owner-email` attaches everything to an existing account |
| `documents` | `documents`; `last_read_at / 10` → ms |
| `document_history` | `document_history`; `deleted_at IS NOT NULL` rows skipped unless `--include-deleted` |

Because legacy users have no email address, the default mode creates `users` records with
`<username>@invalid.local`, unverified, with a generated password printed **once** to stdout in a table
for the operator to distribute; the account holder then changes email and password in the WebUI. The
`--owner-email` mode is the right choice for single-user instances.

After import it enqueues every affected `(owner, date)` so analytics backfill through the normal worker.
The whole run is one transaction (`app.RunInTransaction`), idempotent on `(owner, document)`, and
`--dry-run` prints counts and conflicts without writing. Tested against a fixture legacy database built
in-test from the legacy DDL (`server/testdata/legacy.db`).

---

## 11. Testing strategy (requirement 8)

Project convention from the legacy `.junie/AGENTS.md` is kept: **standard library `testing` only, no
testify or other assertion libraries**, and every `source.go` has a `source_test.go`.

**Go — integration-first.** `tests.NewTestApp(testDataDir)` on a committed `server/testdata/pb_data`
fixture (generated once by running the migrations, refreshed by a `go run ./internal/migrations/gen`
helper), plus a `newTestApp(t)` helper seeding two users, their credentials, and documents.
`tests.ApiScenario` table tests per route:

- KOReader auth: valid, wrong MD5, unknown user, disabled account, missing headers, PocketBase JWT
  rejected on `/koreader/*`.
- Progress PUT/GET: create, update, history append, malformed body, unknown document → 404, and
  **cross-user isolation** (user B's credential must never read or overwrite user A's document).
- `/koreader/users/create` → 402.
- Custom routes: credential create/rotate (MD5 correctness verified by authenticating afterwards),
  history restore (asserting the previous state landed in history), ownership rejection.
- Collection rules: user B cannot list/view/update/delete user A's `documents`, `document_history`,
  `reading_days`; realtime subscription filtered identically.

**Go — units.** Analytics recompute against fixed timestamps with hand-computed expectations (including
the session-gap edge cases at exactly 0 s and exactly the threshold); retention/rollup with a frozen
clock; queue coalescing; credential cache (hit, expiry, invalidation, failures not cached); config
decoding; importer mapping and idempotency.

**Frontend.** vitest + @vue/test-utils + jsdom (already configured in the legacy project) with the
PocketBase SDK mocked: auth store, documents store incl. realtime event handling, stats store, and
component tests for `DashboardMetrics`, `DocumentsList`, `HistoryList`, `ReadStatisticsChart`, login and
credential management. Coverage reported to GitLab as today.

**End-to-end.** A Go test that boots the compiled binary on a temp data dir and drives it over `net/http`
— registers a user, creates a credential, pushes progress the way KOReader does, asserts the dashboard
data. No browser automation, so the Go+Bun-only constraint holds. (Playwright is explicitly *not* adopted.)

---

## 12. CI/CD

`.gitlab-ci.yml` ported from legacy and extended. Stages `validate → test → audit → build → analyze`:

- **validate**: `gofmt -l`, `go vet`, staticcheck, wwhrd (license check), bearer (SAST), and for the
  WebUI `oxlint`, `eslint`, `vue-tsc --build`.
- **test**: `go test -coverprofile -json ./...` with coverage regex; `bun run test --coverage`.
- **audit**: govulncheck, `bun audit`.
- **build**: `bun install && bun run build-only` → `go build -tags netgo`; artifact the binary. On tags,
  `docker buildx` → registry, using `deployment/ci.Dockerfile` adapted for PocketBase (`/pb_data` volume,
  `ENTRYPOINT ["/app/kosync", "serve", "--http=0.0.0.0:8080", "--dir=/pb_data"]`).
- **analyze**: sonarqube on `main`.

Go module and bun caches are added (the legacy config re-downloads everything each job).

---

## 13. Phases

Each phase ends with green tests and is independently reviewable — which matters under
`MACHINE_POLICY.md`, since you review and commit each one yourself.

| Phase | Deliverable | Notes |
| --- | --- | --- |
| **0. Scaffolding** | module layout, config package, embed+generate chain, `build.go`, CI skeleton, docs stubs | no behaviour yet; establishes file headers and conventions |
| **1. Schema** | all §3 collections as Go migrations, rules, indexes; rule tests | the schema review gate — worth stopping on |
| **2. KOReader API** | `/koreader/*`, MD5 auth middleware, credential cache, full route tests | **feature-complete for devices**; a KOReader can already sync |
| **3. Analytics** | queue, worker, recompute SQL, retention + reconcile crons, tests | replaces the legacy CTE-per-request |
| **4. Custom APIs** | credential create/rotate, history restore | |
| **5. WebUI** | full parity: dashboard, chart, documents grid/list, history, login/register, credential management, realtime | biggest single chunk; split per-view if desired |
| **6. Importer** | `import-legacy` + fixture tests + `docs/migration.md` | |
| **7. Release** | Dockerfile, compose, README/docs rewrite, tagged image | v2.0 |

Then the later ideas, in dependency order:

| Phase | Idea (from your list) | Depends on |
| --- | --- | --- |
| **8. Books** | EPUB upload, metadata + cover extraction, hash precomputation, library view | 5 |
| **9. Matching** | link pushes to books on arrival; unlinked pushes listed separately | 8 |
| **10. OPDS** | OPDS 2.0 catalog, Basic auth, authenticated acquisition | 8 |
| **11. Merge** | user-selected merge of several pushes into one document | 9 |
| **12. Book statistics** | per-book/day rows, book detail view, measured page counts | 9 + 3 |
| **13. Achievements** | pages/books/streak rules, SVG icons, repeatable tiers | 12 (page counts) + 3 (daily rows) |
| **14. Mail** | recovery + achievement notifications via PocketBase SMTP | 13 |
| **15. Library-first dashboard** | the library becomes the main component on `/`, below the statistics; the documents table moves to a page of its own | 8 |
| **16. Navigation** | left: home, library, later documents; right: address, account settings and sign out merged into one menu | 15 |
| **17. Device names** | editable display names per device, shown wherever a device is named | 12 |

Phases 9 and 10 are independent of each other and can be built in either order; §16 explains why doing
10 first makes 9 exact rather than heuristic for anything downloaded from the catalog.

Phases 8 to 17 are all built; §16.6, §18, §19, §21, §22, §23, §25 and §26 record what
each of them turned into, and §17 is now a description of what the interface does rather than what it should
do. Phase 10 landed after 9 rather than before it, so the exactness §16 wanted from that ordering
arrived as a third stored hash instead — see §22.1.

Phases 15 and 16 are interface work rather than new capability, and are described in §17. They are
listed last because nothing depends on them, but 15 is worth doing before the documents table grows
any more features, since several of them would only have to be moved again.

Two notes on those later phases: EPUB parsing should use a small pure-Go reader over `archive/zip` +
`encoding/xml` rather than a heavyweight dependency (cover extraction is `container.xml` → OPF →
`<meta name="cover">` → manifest href), and PocketBase's file-field `thumbs` generates the cover
thumbnails, so no image tooling is needed. Achievements are awarded by a cron over `reading_days` — the
streak rule ("3 consecutive days with >10 pages") is a window query over exactly that table, which is
another reason the daily rows are worth precomputing.

---

## 14. What the implementation changed about this plan

Phases 0 to 7 are built. These are the points where the code deliberately deviates from the plan
above; the rest was implemented as written.

1. **`/koreader/syncs/progress` answers with a body.** The plan said "byte-compatible with the legacy
   server", which returned an empty 200. The official server returns `{"document":…,"timestamp":…}`
   and `/users/auth` returns `{"authorized":"OK"}`, so those are what v2 returns.
2. **`koreader_accounts` cannot authenticate at all.** Beyond disabling password auth, the collection's
   `AuthRule` is nil, so PocketBase refuses any authentication against it. A device credential can
   never become an API session, belt and braces.
3. **`enabled` became `disabled`.** PocketBase booleans default to false, so a flag named `enabled`
   would have made every credential created outside the API dead on arrival.
4. **The restore endpoint consumes the entry it restores.** After a restore the entry *is* the current
   state, so leaving it in the history would show it twice. The state it replaced is archived, which
   is what keeps the restore undoable.
5. **`reading_days` rows are deleted when a day empties out.** Storing a row of zeroes for every day
   somebody did not read would grow the collection for no information.
6. **`progress_increase` is clamped per document.** The legacy query let a restarted book subtract from
   a day. Clamping is both more useful and required by the field's own bounds.
7. **A dry run of the importer is a rolled back transaction**, not a separate code path, so it exercises
   exactly what a real run does.
8. **The importer reuses an existing credential's owner** when it finds one, which is what makes a
   second run a no-op rather than a duplicate import under a fresh account.

## 15. Open risks

1. **bcrypt cost on every progress push** — mitigated by the credential cache (§5.2). If load ever
   demands more, the next step is a dedicated verifier with a lower cost factor for KOReader accounts
   only; the plan does *not* start there, because caching is simpler and reversible.
2. **`timestamp` unit change** — v2 returns Unix seconds where legacy returned 1/10000 s. Correct per the
   KOReader protocol, but any third-party client built against the legacy quirk breaks. Called out in
   `docs/migration.md`.
3. **Registration removal from KOReader** — existing users are used to registering on the device. The 402
   response, the WebUI setup guide, and the README all need to state the new flow clearly.
4. **PocketBase upgrades** — collections defined in Go migrations are the safe path; the superuser UI must
   not be used to edit the production schema by hand, or the next deploy will disagree with it.
5. **Analytics correctness during import** — a large backfill enqueues many days at once; the worker is
   rate-limited and the importer's own bulk path is tested against a hand-computed expectation.

---

## 16. The library: uploads, OPDS 2.0 and book-level statistics

Design for phases 8 to 12. Nothing here is built.

### 16.1 Why the server holds the file

Three reasons, and they reinforce each other:

1. **Backup.** The EPUB survives a lost or wiped device.
2. **Exact matching.** Holding the bytes means KOsync can compute the same document hash KOReader
   computes, so a progress push identifies itself instead of being guessed at (§16.3).
3. **Book-level statistics.** Metadata and a page count turn per-day totals into per-book ones: which
   books were finished, how long each took, which days went into it (§16.5).

The catalog is what ties 1 to 2: a book downloaded from KOsync is *byte-identical* to the one being
read, which is the only condition under which the binary hash can match.

### 16.2 Storage

A `books` file field (§3.8), so PocketBase handles storage, local disk or S3, and generates the cover
thumbnails. Two limits belong in config: a maximum upload size, and an optional per-owner quota.
`content_hash` (SHA-256 of the file) deduplicates re-uploads of the same file **within one owner**;
cross-owner dedup is deliberately not done, because it makes deletion and ownership ambiguous for a
saving that does not matter at this scale.

Parsing stays dependency-free: `archive/zip` + `encoding/xml`, `container.xml` → OPF → metadata, and
`<meta name="cover">` → manifest href for the cover.

### 16.3 The two KOReader hashes

KOReader identifies a document one of two ways, and `koreader_hashes` stores both:

| Hash | Computed from | Matches when |
| --- | --- | --- |
| filename | the file name only | the reader kept the name KOsync served |
| binary (partial MD5) | sampled chunks at increasing offsets | the file is byte-identical |

Consequences worth designing around:

- **Serve a deterministic filename.** The filename hash is only useful if KOsync knows the name on
  disk, so acquisition links must serve a name derived from the record, not from whatever the file was
  called at upload.
- **A different copy of the same book matches neither.** Another retailer's EPUB of the same title is
  different bytes and, usually, a different name. So the library UX has to say plainly: upload the file
  you actually read, or read the file you downloaded from here. This is the whole argument for the
  catalog.

**The binary hash is confirmed**, checked against five real production documents (the Witcher EPUBs
and their stored hashes) — 5 of 5. The algorithm is: MD5 over 1024-byte samples read at the offsets
below, stopping at the first read that returns nothing.

```
offset(i) = uint32(1024) << uint((2*i) & 31)    for i = -1 .. 10
          = 0, 1024, 4096, 16384, 65536, 262144, 1048576, ...
```

The `& 31` is the part that matters and the reason this had to be measured rather than read. KOReader
computes the offset with LuaJIT's `bit.lshift`, which masks the shift count to five bits, so the first
iteration is not `1024 >> 2 = 256` as the source reads — it is `1024 << 30` truncated to 32 bits, which
is **0**. The first sample is the file header. Implementing the loop the way it looks produces a hash
that matches nothing; that was the first attempt here, and it scored 0 of 5. A test must pin the
offsets literally, with the five known hashes as fixtures, so nobody later "fixes" the mask away.

### 16.4 OPDS 2.0

Client baseline: KOReader v2026.07 (commit `e72fe823d01b38351b7088168ba4559e3ed2e8bd`), the release
that added OPDS 2.0 support.

OPDS 2.0 is the Readium manifest model in JSON — a feed is `metadata` + `links` +
`navigation`/`groups`/`publications`, and a publication is `metadata` + `links` + `images`. Every field
it wants already exists on `books`, and the cover `thumbs` supply `images` directly, so the feed is a
projection over the library rather than a second data model. No XML, and no OpenSearch descriptor
document: 2.0 replaces it with a templated `rel: search` link.

**Route group `/opds`, not `/koreader/opds`.** The `/koreader` prefix exists to isolate the MD5 header
protocol. OPDS is a standard other readers speak, and the path should not claim otherwise.

**Auth: HTTP Basic against `koreader_accounts`.** Basic sends the plaintext password, and the stored
hash is bcrypt over the MD5 — so hashing the received password with MD5 and verifying against the
existing hash works unchanged. The credential a device already has for syncing also opens the catalog,
with no second thing to create, and the §5.2 cache keeps bcrypt off the hot path. Serve an
`application/opds-authentication+json` body on the 401 so conformant clients can discover the scheme.

**Acquisition must not use PocketBase's file URLs.** `/api/files/...` needs a short-lived token as a
query parameter, which an OPDS client will not know to fetch. Acquisition and cover links point at
`/opds/...` routes that re-check Basic auth and stream from the PocketBase filesystem.

**Build the feed as a tree behind a renderer interface.** The JSON renderer is the only one needed for
the baseline above, but older clients speak Atom 1.2, and with the tree already built that renderer is
a small addition rather than a second implementation. Do not build it up front.

### 16.5 Book-level statistics

This is the part that changes the analytics schema. Today's aggregation key is `(owner, date)`; book
statistics need `(owner, date, book)`, so phase 12 adds a `reading_book_days` collection with the same
measures plus a `book` relation, computed by the same worker from the same history rows.

Two subtleties:

**Reading time does not add up, and should not be forced to.** §6.3 estimates a day's reading time by
ordering every push in that day and summing the gaps below the session threshold. Grouping by book
first means a gap that spans a switch from one book to another belongs to neither, so
`sum(book rows) ≤ day row`. The plan keeps `reading_days` computed exactly as it is now — those numbers
were validated against the legacy query at 82 of 83 days — and computes the book rows independently.
The day total stays the authoritative one; the residual is switching time and is not displayed as a
discrepancy.

**Page size should be measured, not configured.** An EPUB is reflowable, so it has no page count, and
KOReader's own count changes with font size and screen. But the sync pushes give the answer away.
KOReader syncs every N pages, so the progress deltas between consecutive pushes quantize: on real data
from one device they cluster hard on one value, with clean half and double multiples around it.

Measured against real data, all five books on one device (`go7`), with the page counts the device
itself reports as ground truth:

Measured against real data, all five books on one device (`go7`), with the page counts the device
itself reports as ground truth:

| Book | derived pages | device pages | spine words | words/page |
| --- | --- | --- | --- | --- |
| Zeit des Sturms | **700** | 700 ✓ | 109 288 | 156.1 |
| Das Schwert der Vorsehung | **563** | 563 ✓ | 116 921 | 207.7 |
| Der letzte Wunsch | *no data* | 619 | 96 837 | 156.4 |
| Kreuzweg der Raben | *no data* | 446 | 68 856 | 154.4 |
| Die Witcher-Saga | *declined* | — | 638 847 | — |

So the device's page count for a book is `1 / page_fraction`, recovered from the deltas alone — **exact
on both books where it could be checked**, with no word count and no user input involved. The unit is
visible because partial pushes (a chapter end, closing the document) land on the half-step, which also
reveals N: here 2, matching the recommended "sync every 2 pages".

Getting the last percent right needs one more step. The median delta is a single noisy sample; once a
candidate unit has established how many pages each delta spans, the size follows from all of them at
once, as total progress over total pages. Skipping that refinement leaves the estimate about 1.5%
short — 690 and 556 instead of 700 and 563, which is wrong while looking entirely reasonable.

Three limits, all of them found by running the estimator against real data rather than by reasoning
about it.

**It needs pushes.** Two of the five books have one or two history rows each, both at the very end —
read before the server was in use. No amount of cleverness recovers a page count from that.

**It has a ceiling of roughly 1600 pages.** KOReader reports progress to four decimals — every one of
the 1803 values in the reference data sits exactly on a 0.0001 grid. A page in the 3562-page omnibus
spans 2.8 grid steps, which is below what the protocol can express, and the estimator's first version
duly reported the book as exactly 10000 pages: stable across every chunk of the series, and simply the
reporting grid wearing a page's clothes. The fix is a floor at six grid steps, and the cost is that
long omnibus editions can never be measured. `pages.FromProgress` refuses instead, which is the point.

**Words per page is not a device constant.** Three books sit at 153.6–155.7, and one sits at 206.0 —
33% denser — on the *same device* during an *overlapping period* (both were being read in
January 2026), so it is not a settings change between books. Two hypotheses were tested and killed:
counting non-spine files in the archive (spine-only counting moves it by 0.1%), and font settings
drifting over time (the reading periods overlap). The likely remainder is the book's own embedded CSS,
which is unconfirmed. Whatever the cause, the design consequence is settled: a per-device page size
the user configures once would have been ~25% wrong for one book in four, and they would have had no
way to notice. Measure; do not ask.

The resulting layering, best first:

1. **Measured** per `(book, device)` from a rolling window of recent deltas. Precise, and self-healing:
   change the font and the quantum shifts, and so does the estimate.
2. **Configured per device**, as `pages_per_sync` on `koreader_accounts` — an override for when the
   measurement is ambiguous, and expressed in the unit the user actually knows because they typed it
   into KOReader, rather than asking them to guess a page size.
3. **A global words-per-page constant** in config, for books that cannot be measured — three in five
   on this sample, so it will be used more than one would hope. Set it around 155 rather than the
   ~250 usually quoted for print; that is what an e-reader page actually holds here. If the density
   difference does turn out to be the book's own CSS, it is a property of the file rather than of the
   reader, and a measurement taken once could be stored on `books` and reused for every user with the
   same `content_hash`. Worth checking before settling for the constant.

Word counts must come from the **spine**, not from every XHTML file in the archive. On this sample it
happens to make almost no difference, which is exactly why it would survive review as a bug.

### 16.6 Built so far

Phase 8 has started. Two packages exist, both pure logic with no schema or API surface, so they can be
reviewed on their own:

- **`internal/epub`** — `PartialMD5` and `FilenameMD5` (the two document hashes), and `Open` for
  metadata, cover and spine word count. Verified against the five real books: 5 of 5 hashes, and all
  five parse.
- **`internal/pages`** — `FromProgress`, the page-size estimator described above.

44 test cases, 89.4% and 87.9% statement coverage. Both packages carry an opt-in test that runs
against real data when an environment variable points at it — `KOSYNC_REAL_EPUB_HASHES` for the
hashes, `KOSYNC_REAL_PROGRESS_CSV` for the estimator — and skips otherwise. The books are not ours to
ship and the reading history is personal, so neither can live in the repository; but synthetic
fixtures cannot show that the code agrees with KOReader rather than with itself, so the opt-in path
has to exist.

Two defects in the parser were found only by pointing it at the real files, and both are now covered
by tests: real titles arrive wrapped across indented lines, newlines and all, and EPUB 3 dropped the
scheme attribute on identifiers in favour of a `urn:isbn:` value, so every real EPUB 3 ISBN was being
filed as unknown.

The `books` collection and the upload path now exist too:

- **`internal/migrations/1786665600_books.go`** — the `books` collection, owner-scoped, with the file,
  the cover (thumbs 100x150 and 200x300), the metadata, and both document hashes as separate indexed
  columns rather than the JSON blob §3.8 planned. Matching a push means looking a book up by either
  hash, which JSON cannot index.
- **`internal/books`** — uploads go through the ordinary collection API, so rules, realtime and file
  serving come for free; a create hook reads the arriving file and fills in everything derived from
  it. A second hook refuses edits to the derived fields, while leaving title and authors editable.

Verified end to end against a real book through the running binary: uploading *Zeit des Sturms*
produced `hash_binary = 043f11771ef9d191364ac0ba08198d36`, which is exactly the document id the
production database recorded for it. A file uploaded here is therefore already identified as the thing
the device has been pushing progress for, which is the whole premise of phase 9. Cover extraction and
thumbnail generation work on the real file as well.

The fallback page count came out at 705 against the device's 700 — 0.7% out, which is far better than
the words-per-page spread in §16.5 deserves and should not be read as typical.

Three things were found only by running this, and all are fixed and covered by tests:

1. **A cover that is not a valid image failed the whole upload.** The field's own mime check rejected
   it and took the book with it. Covers are now sniffed by content, not by the href's extension, and
   an unusable one is dropped silently.
2. **`Config.Normalize` did not guard `BooksWordsPerPage`**, so a zero produced zero-page books.
3. **A file that is not a zip reported a zip error** rather than "not an EPUB". `epub.Open` now returns
   `ErrNotEPUB` for that case too, since something that is not a zip is not an EPUB.

The WebUI side is built too: a `books` store, a `BookLibrary` cover grid with multi-file upload,
rename and delete, and a `/library` route reachable from the top bar. Uploads go one file at a time,
so a failure names the book that failed rather than one of several parallel requests, and the store
sends nothing but the file and the owner — everything else would be untrustworthy coming from a
browser, and there is a test asserting the upload carries none of it.

**Phase 8 is complete.** All five reference books were uploaded through the running binary, and every
one produced the document hash the production database already had:

| Book | `hash_binary` | fallback pages | device pages |
| --- | --- | --- | --- |
| Zeit des Sturms | `043f1177…` ✓ | 705 | 700 |
| Das Schwert der Vorsehung | `06317bff…` ✓ | 754 | 563 |
| Der letzte Wunsch | `4d4ecfc4…` ✓ | 625 | 619 |
| Kreuzweg der Raben | `d3d6abff…` ✓ | 444 | 446 |
| Die Witcher-Saga | `00e7a0b5…` ✓ | 4122 | — |

Uploading the same files a second time was refused on the unique content hash, as intended.

That page-count column is the §16.5 argument made concrete. Three of the four land within 1% of what
the device reports, and *Das Schwert der Vorsehung* — the book that reads at 207 words per page rather
than 155 — comes out 34% too long. The fallback is genuinely good most of the time and occasionally
badly wrong, with nothing in the data to say which case you are in. That is why it stays the last
resort behind a measured count.

Measured values move as data accumulates, which would otherwise rewrite history. It does not, because
`user_achievements` rows are awarded records rather than a derived view (§3.8) — once granted, a tier
stays granted even if the underlying estimate later shifts.

With those in place the library view can show, per book: total reading time, days spent, first and last
read, percentage complete, and the distribution of reading across days.

---

## 17. Interface restructuring (phases 15 and 16)

Built. Neither added capability; both were about the shape the app presents once there is a library in
it, since the old layout was designed when documents were the only thing to show.

### 17.1 A library-first dashboard (phase 15)

`/` is now the statistics and then the library, and the documents table has moved to `/documents`.

The plan left two questions to settle while building rather than before, and both are now answered.

**How much of the library belongs on the dashboard: the recently read.** `BookLibrary` takes an
optional `limit`, and the dashboard passes six. Limited, it sorts by when each book was last read
rather than by title and links to the rest; unlimited, it is the full library by title, which is what
`/library` wants. A dashboard is a shelf, not a catalogue, and the distinction cost one prop.

**Whether documents and the library merge: no — the two have different jobs.** The library is where
books are read and interacted with. The documents page is the fallback that answers one question:
what have I read that is not on the server yet? So it is built around that question rather than
around the table it used to be:

- **The missing ones lead**, in their own "Not in your library" section with the count in the
  heading, an explanation of what they are missing out on, and an **EPUB upload right there** — the
  only fix, offered where the problem is stated rather than only on the library page.
- **The matched ones follow**, each with its cover and a link to its book, under a heading that says
  the library is the better place to read them.
- **Every row says which it is**, with a cover and link when there is a book and a "Not in library"
  tag when there is not.

That is phase 9's deferred "unlinked pushes listed separately", finally built, and held back for
exactly this restructure.

The library grid got a fix from the same review. A grid row is as tall as its tallest cell, so the
Witcher omnibus — whose title runs to six lines — stretched every card beside it and left them
floating above a gap. Each card now reserves exactly two lines of title and one of author whether it
fills them or not, clamping the overflow with the full text on hover, and pins its buttons to the
bottom with `mt-auto`. The page also printed the word "Library" twice, once as its own heading and
once on the card below it; the card's heading is now a prop the page can empty.

The first attempt at the documents page got something wrong in an instructive way. The marker and the link were added to the
**table** view — and the grid is the default, so the view most people see said nothing at all beyond a
count with no way to tell which rows it meant. The list component now takes the documents to render as
a prop instead of reading the store, so a page decides what a list contains, and both views are
asserted for the marker and the link in the tests.

### 17.2 Navigation (phase 16)

Built as described. Home, library and documents sit on the left as ordinary navigation with an active
state; the address, account settings and sign out are one menu on the right, with the address as its
first item rather than a label beside it — it says whose account the menu is about, which is the
question a person opening it has. The theme toggle is the only free-standing control left.

The labels collapse to icons below the `sm` breakpoint, so three navigation entries do not push the
account menu off a phone.

---

## 18. Matching (phase 9)

Built. `documents` gained a `book` relation, and `internal/books/matching.go` links the two.

The link is made in **both directions**, and the second is the common one:

- **on arrival** — a document is created on the first push for a hash, so a lookup at create time
  costs one indexed query per book rather than one per push; and
- **retroactively** — uploading a book claims the documents that were already recording progress
  through it. This is the case that actually happens, since people upload a book after reading it.

Either hash matches. KOReader sends whichever its checksum setting produces and the server cannot tell
which one it is looking at, so both columns are tried.

Three deliberate choices:

1. **A failed lookup never costs a device its push.** Matching errors are logged and swallowed. The
   book link is a convenience; the reading position is the thing the user would notice losing.
2. **The relation does not cascade.** Deleting an uploaded file clears the reference and leaves the
   document and its history exactly as they were before the book existed. There is a test for it,
   because that behaviour is PocketBase's rather than ours.
3. **Matching is owner-scoped.** Two accounts uploading identical bytes each match only their own.

Verified end to end through the running binary, in both directions: a push for an unknown hash arrived
unmatched, and uploading the EPUB linked it; a book uploaded first claimed the next push immediately.

### 18.1 Two bugs the first version had

Both were reported from a real instance, and neither showed up in the tests as written.

**Books uploaded before matching existed were never linked.** Both hooks fire on records *being
created*, so a pair that was already sitting in the database when the feature arrived would never be
brought together — permanently, with re-uploading the file the only way out. Adding the `book` field
was not enough; the data had to be repaired. `1786838400_backfill_document_book.go` does that once, and
its query is written out in the migration rather than calling `internal/books`, so what it did cannot
change under it later. Verified on a database put back into the pre-matching state: the link returned
on the next start.

**The library only saw the link after a full page reload.** `/library` subscribed to `books` but not to
`documents`, and the progress on a cover lives on the document — which is precisely what the server
moves when a book is uploaded. Uploading, or deleting and re-uploading, therefore changed nothing on
screen until the page was reloaded. `LibraryView` now subscribes to both.

One gap remained by choice for a while: if a match fails transiently, that document stays unlinked
until the book is re-uploaded, because nothing re-examines existing pairs. That is now closed by the
nightly reconcile in §27.1.

### 18.2 What phase 9 deliberately left out

The plan's phase 9 also said "unlinked pushes listed separately". That is interface work in the
documents table, which phase 15 moves to its own page — building it now means building it twice. It
belongs with that restructure, where the question of whether documents and the library remain separate
views gets settled anyway.

**Settled, after phase 15: the documents page marks an unmatched document "Not in library" and that is
enough.** A separate list would be a second place to look for the same rows, and the label already
answers the only question anybody was going to ask of it. Closed rather than deferred.

What did land in the interface is the more interesting half: the library shows reading progress on
each cover, so a matched book reads as "Reading 63%" or "Finished" at a glance. That is in the new
component, which is not going anywhere.

---

## 19. Book statistics (phase 12)

Built. It is the phase that made `internal/pages` real: the estimator had been written, tested and
validated in phase 8 and then had no callers at all, and `reading_days.pages_read` had been a
placeholder zero since phase 3. Both are now fed by the same worker pass.

### 19.1 What was added

- **`reading_book_days`** — the §3.5 measures keyed by `(owner, date, book)`, computed by a second
  query beside the day query rather than by grouping it, for the reason §16.5 gave.
- **`books.measured_pages` / `measured_device` / `measured_through`** — the page count recovered from
  the progress a device pushed, which device it came from, and how far into the reading the
  measurement looked.
- **A book page** at `/library/:id` — cover and metadata, time spent, pages read, days read, best day,
  and a per-day chart of pages and reading time. Reachable by clicking a cover.

### 19.2 The measurement, in production

Running the new migrations against a copy of the real database and letting the worker drain:

| Book | fallback pages | measured | device pages |
| --- | --- | --- | --- |
| Zeit des Sturms | 705 | **700** | 700 ✓ |
| Das Schwert der Vorsehung | 754 | **563** | 563 ✓ |
| Der letzte Wunsch | 625 | *declined* | 619 |
| Kreuzweg der Raben | 444 | *declined* | 446 |
| Die Witcher-Saga | 4122 | *declined* | — |

Both books that could be measured came out exactly right, end to end through the running binary rather
than in a unit test. The three that declined are the three §16.5 predicted would: two were read before
the server existed and have a handful of history rows, and the omnibus is past the ~1600 page ceiling
the four-decimal reporting grid imposes. They fall back to the word count, and the interface says which
of the two a number is.

### 19.3 The residual is real

§16.5 predicted that the book rows would sum to *less* than the day row, because a gap spanning a
switch between books belongs to neither. On the production data, on the days with more than one book:

| Day | books | day total | sum of books | residual |
| --- | --- | --- | --- | --- |
| 2026-01-25 | 3 | 3863 s | 3731 s | 132 s |
| 2026-02-21 | 2 | 4577 s | 4465 s | 112 s |
| 2026-02-01 | 2 | 7627 s | 7527 s | 100 s |

Around two minutes a day, which is what putting one book down and picking another up costs. The day
total stays authoritative and the residual is not displayed as a discrepancy. Pages, by contrast, add
up exactly: every page is read in one book.

### 19.4 Three things worth knowing

**The measurement is per file and per device, not per book.** Two devices paginate the same file
differently and both are right; the series with the most pushes behind it wins. Grouping by file as
well as device matters more than it looks: the same book stored twice under different names is two
files, and merging their series destroys the quantisation the estimate depends on.

**It looks at the recent end first.** A series spanning a font change fits neither pagination, so an
estimator that only ever saw the whole history would keep the old number for good. Trying the last 40
pushes before the whole series is what makes it self-healing, and widening when the window has nothing
to say is what keeps a rarely-read book measurable.

**`measured_through` is a reading timestamp, not a wall clock.** It records the newest push the last
measurement saw, so "has anything new been read" is answerable. The first version stored the moment the
measurement ran, which is always *after* every reading timestamp, so no book would ever have been
measured twice. It passed every test that did not specifically read the book again.

### 19.5 Upgrading an existing instance

Nothing recomputes a day that is not read on again, so an instance upgrading into this would have shown
zero pages for everything it had already recorded — the same shape of gap as §18.1, found by looking
for it this time rather than by a bug report. `1787011200_queue_book_statistics.go` enqueues every
stored day once, and the existing worker drains it in the background. On the production copy, 86 days
went through the queue and produced 88 per-book rows in a few seconds.

---

## 20. Device names (phase 17)

Phase 12 put a device in front of the reader for the first time — a measured page count says which
device it was measured on — and what it says is `865F46C0C0F4401D9A05768B6B0BF3AC`. That is the
`device_id` KOReader sends, and it tells nobody anything.

There are two separate problems behind that, and only the first is a bug:

**The name is already there and is not being used.** A push carries both fields, and `documents` stores
both: `last_device` is `go7`, `last_device_id` is the UUID. `books.measured_device` records the id
because the estimator groups by id — correctly, since the id is what identifies a device across a
rename. Displaying the id is simply the wrong end of the pair. Resolving it to the most recent
`last_device` for that id fixes the current display outright.

**`go7` is still not what the device is called.** It is a Boox Go 7. KOReader's own name is a short
identifier the reader chose once, or a model string a vendor chose for them, and neither is
necessarily what the owner would call the thing sitting on their desk. So the fix above is a floor,
not the feature.

The feature is a small owner-scoped registry keyed by `(owner, device_id)`, holding the last name the
device reported and a display name the owner can edit, with the reported name as the default. Written
by the same hook that already updates `last_device`, so a device appears the first time it pushes and
never needs adding by hand.

Note this is **not** `koreader_accounts.label`, which already exists. A credential and a device are not
the same thing: one credential can be used from several devices, and the analytics group by device id,
not by credential. Reusing the label would name the wrong thing and would be wrong most visibly for
exactly the person who set up one shared credential.

Where a device name appears once this exists: the measured page count on a book, the documents table's
"last read on", and — if phase 13 happens — anything that counts reading per device.

---

## 21. Device names (phase 17)

Built, and smaller than it looked, because both halves were already in the data.

**The display bug went first.** A push carries `last_device` and `last_device_id`, and `documents`
stored both all along; the book page was simply reading the wrong one. Both the documents table and the
book page now resolve the identifier to a name.

**The registry is the feature.** `devices` holds one row per `(owner, device_id)` with the name the
device reports and the name its owner chose. It is written by a hook on every push, after the write
rather than before it, on the same rule matching follows: registering a device must never cost a reader
their position.

Three decisions worth stating:

1. **The reported name is refreshed; the chosen one never is.** Renaming a device in KOReader shows up
   for anyone who has not overridden it, and a name typed into KOsync survives the very next sync.
   Getting this backwards would make the feature useless in the least obvious way — it would work, and
   then silently undo itself.
2. **`last_seen` only moves forwards.** History arrives out of order during an import, and a restored
   old state must not make a device look unused since last year.
3. **A device is not a credential.** `koreader_accounts.label` already exists and would have been the
   cheap place to put this, but one credential can be used from several devices and the statistics
   group by device id. Reusing the label would misname things worst for exactly the person who set up
   one shared credential.

Backfilled from the existing pushes, over both `documents` and `document_history`: a device that
finished a book months ago never pushes again, so a registry that only fills itself going forward would
never learn its name. On the production copy it registered three devices — including one neither of us
had thought about, a desktop KOReader reporting itself as `Flatpak`.

---

## 22. The OPDS catalog (phase 10)

Built. §16.4 planned it and every decision there survived contact; what follows is what the code does
and the two things the plan did not anticipate.

**The catalog is the other half of matching.** Phase 9 links a push to a book by hash, and §16.3 named
the consequence: another retailer's copy of the same title is different bytes and a different name, so
it matches nothing. The answer the plan gave was "read the file you downloaded from here" — which only
works once there is a here to download from. There now is, and the loop closes: a book acquired from
the catalog is byte-identical to the one the server holds, so its binary hash is known before the
device has read a word of it.

**Three shelves, and the third is the argument.** `/opds/books` by title and `/opds/recent` by upload
are what any catalog has. `/opds/reading` — started and not finished, most recently read first — is
the one a plain file share cannot offer, because it needs the progress this server exists to collect.
Setting up a second device and finding the book you are in the middle of, already at the right page,
is the whole product in one gesture. The join can return a book twice, when two document hashes point
at one book, so it groups and takes the later position of the two.

**Basic auth over the existing credential.** The stored value is bcrypt over MD5, and Basic delivers
the plain password, so hashing it with MD5 first verifies against what is already there. The catalog
is handed the KOReader handler rather than reaching for the credentials itself, which means one
verified-credential cache for both surfaces and no second thing for a person to create. A 401 carries
an `application/opds-authentication+json` body, so a reader prompts rather than guesses.

### 22.1 The field the plan did not have

§16.3 said acquisition "must serve a name derived from the record, not from whatever the file was
called at upload", and it was right, but it did not follow the consequence through: `hash_filename`
already holds the hash of the uploaded name, and the served name is a different string. One column
cannot hold both, and both are worth holding — one for "I uploaded the very file I read", the other
for "I read the file I downloaded from here".

So `books` gained a third indexed hash, `hash_catalog`, derived from the title and recomputed whenever
the title changes. Matching tries all three. A rename does leave a device that downloaded earlier
holding the old name, and that device falls back to the binary hash, which is the default setting
anyway.

### 22.2 Verified end to end

On a copy of the production database, ten books:

- The migration gave every one of them a catalog hash. `Zeit des Sturms` → `Zeit des Sturms.epub` →
  `5c75dcc791ef1ac8e5bde189a28a4999`, which is the MD5 of that name and nothing else.
- `/opds/reading` returned exactly the three books with a linked document between 0 and 1 — Metro, the
  Witcher omnibus at 83.8%, and Deutsche Sagen — and excluded the four finished ones.
- The downloaded EPUB was byte-identical to the stored file (SHA-256), and its binary hash came back
  `043f11771ef9d191364ac0ba08198d36`: the same string the production database has recorded as that
  document's identity since long before any of this was written.
- A push carrying the catalog filename hash created a document that was linked to the right book on
  arrival, with no upload and no manual step.
- The thumbnail route turned a 797x1240, 588 KB cover into 200x300 and 38 KB. Over a device's wifi
  that ratio is the difference between a catalog that opens and one that does not.

### 22.3 Deliberately not built

**The Atom 1.2 renderer.** §16.4 said to build the feed as a tree behind a renderer interface and not
to write the second renderer up front. Both halves were followed: `Feed`, `Publication` and `Link` are
what the catalog thinks in, `JSONRenderer` is what OPDS 2.0 puts on the wire, and `Feed.Id` exists
unused by the JSON renderer because Atom would need it.

**Uploading through the catalog.** OPDS 2.0 can describe it, and it would mean a device could push a
book up as well as pull one down. Nothing asks for it yet, and the metadata extraction on upload is a
request hook that assumes the web interface's multipart shape.

**A per-owner storage quota.** §16.2 listed it as belonging in config alongside the upload cap, and it
still does. A catalog makes the library more worth filling, so this gets more relevant, not less.

### 22.4 What the client actually did

The catalog worked on a real device on the first try — browsing, covers and downloading all behaved —
with three things wrong that only a client could reveal. All three came out of reading
`plugins/opds.koplugin/opdsbrowser.lua` rather than the spec, which is the lesson: OPDS 2.0 says what
is valid, and the client says what is useful.

**"Book information" was greyed out.** The button is `enabled = type(item.content) == "string"`, and
for an OPDS 2.0 feed `content` comes from exactly one place: `entry.metadata.description`. Nothing
else fills it. The catalog emitted no description, so the button was dead for every book.

Adding one raised the question of what to put in it. The obvious answer — the publisher's blurb from
`dc:description` — turned out to be worthless here: not one of the reference EPUBs carries that
element, so extracting it would have left the button exactly as grey while adding a field, a migration
and a backfill that re-reads every stored file. What the server does have is where the reading stands,
which on a shelf being browsed from a second device is the better answer anyway. So a description is
composed: the position and the device and date it was last read, then the page count and whether it
was measured or estimated, the word count and the ISBN. On the production copy all ten books got one.

Revisited since: `dc:description` is now extracted after all, and leads the composed description when
a book carries one. What changed is not the reference library — most files still carry no blurb — but
that the same question was being asked of the web interface, where "what is this one about" has no
answer at all short of opening the book. The field, the migration and the backfill were worth paying
for there, and the catalog gets the blurb for free once the column exists. The reading state still
follows it, for the books that have neither.

**The download button said "Download".** `text = url.unescape(acquisition.title or
string.upper(filetype))` — the link's title *replaces* the format name. Setting it to "Download" put a
redundant word where "EPUB" belonged, and would have made two formats indistinguishable if the library
ever holds more than one. The acquisition link no longer carries a title.

**Both cover sizes were the same size.** `getItemFromPublication` picks the thumbnail by relation and
falls back to `entry.images[1]` when it recognises none. The images carried no `rel`, so the fallback
made the full cover the thumbnail as well — 588 KB where 38 KB was on offer, on a device over wifi.
They now carry `http://opds-spec.org/image` and `.../image/thumbnail`.

**And one correction to §22.1.** KOReader names a downloaded file `Author - Title.epub` itself, and
only asks the server for a name when the catalog was added with "use server filenames" ticked, which
turns the download into a `HEAD` for the `Content-Disposition`. So `hash_catalog` is narrower than it
was written up as: it applies with that box ticked and filename matching selected. The binary hash
carries the ordinary case, and does so regardless of what the file ends up being called — which is
what makes it the right default and the one the setup guide recommends.

---

## 23. Merge (phase 11)

Built. §4.2 reserved `POST /api/kosync/documents/merge` for it and described it in one line — "fold N
documents into one, re-parent history, recompute analytics" — which turns out to be three quarters of
the work. The quarter it left out is the one that decides whether the feature holds.

**The case is real, and it took having the data to see it.** The library on the production instance
holds one book under two documents: "Metro Trilogie (2033,2034,2035)" at 0.07%, read on els-n39 and
matched to nothing, and an untitled document at 1.09% read in the Flatpak build and matched to
"Metro - Die Trilogie". Two copies of one file, two hashes, one book, and the reading split so that
neither the documents page nor the per-book statistics see the whole of it. Nothing about the schema
prevents this and nothing before phase 11 repaired it.

### 23.1 The hash has to survive its document

A merge that deletes the folded document and stops there undoes itself. The device that reported that
hash has not changed; it pushes the same string on its next sync, finds no document, and creates one —
and the merge is gone, silently, with no error anywhere. This is the same failure mode as a device
name that the next push overwrites, and it is the reason merge is more than a transaction.

So `document_aliases` maps a retired `(owner, document)` to the document it now resolves to, and both
the push and the pull go through `documents.Resolve` rather than `FindByHash`. The alias cascades with
its document, and deleting one by hand is the way back out — the collection's only writable operation.
The pull is still answered with the hash it asked about rather than with the survivor's, because the
device asked about its own file.

There is a second effect that was not the goal and is arguably the better half of the feature: once
two hashes resolve to one document, **the two devices sync with each other**. Reading Metro on the
Boox and picking it up in the Flatpak build now continues where it left off, which it never did before
however carefully both were configured.

### 23.2 Nothing is deleted that is not archived first

This version has no soft deletion. A folded document is removed outright, so whatever state it held is
gone unless the merge wrote it somewhere first — and the somewhere already exists, because every
progress push has been archiving superseded states into `document_history` since phase 2.

The rule is therefore: every state that loses is archived. The survivor's own position is archived when
it is superseded by a newer one, exactly as a push would supersede it. Every folded document's current
state is archived, except the one that has just become the survivor's current state, which would
otherwise appear twice — the same reasoning the history restore already uses. The history of each
folded document is moved across with a single `UPDATE` rather than record by record, because a
document can carry thousands of entries and every one of them would otherwise be loaded, revalidated
and announced over realtime to move one column.

That makes an unwanted merge recoverable rather than reversible, which is the trade the operator asked
for and the honest way to describe it: the positions are all in the history, one restore away, and
deleting the alias separates the devices again. What it does not do is put the two documents back as
two rows.

### 23.3 What the statistics need

Skipping the hooks on the history move means skipping the analytics enqueue they would have done, so
the merge queues the days itself — every day of the joined reading, not only the days that changed
hands. The reason is `reading_book_days`: if the survivor picked up a book here (and in the Metro case
it does, in whichever direction the merge is made), then every day it was ever read is attributed
differently now, including days that no record touched. The days are collected before anything moves,
because afterwards there is no way back to them.

### 23.4 The one decision left to the person

The document a merge is started from is the one that is kept. That is a per-row action in the web
interface rather than a multi-select with a "keep this one" control, because choosing the survivor is
the only judgement in the operation and clicking it is how the judgement is expressed. Everything else
follows: the most recent position wins, and the survivor takes on a book or a title only where it had
none, so that merging never quietly relabels the document the person chose to keep.

The dialog offers every other document, matched and unmatched alike. Filtering to the unmatched ones
would have been tidier and would have hidden exactly the pair this exists for, since the usual split
has one half in the library and one half out of it.

---

## 24. Timezones and document metadata

Two changes that arrived together because reading the KOReader sync plugin answered both at once.

### 24.1 The protocol has no clock

The question was whether KOReader tells the server what time the device thinks it is. It does not,
and this is worth writing down because it is the kind of thing that gets assumed rather than checked.

`KOSyncClient.lua` sends three headers — `accept`, `x-auth-user`, `x-auth-key` — and a body of
`document`, `progress`, `percentage`, `device`, `device_id`, plus an optional `metadata`. No
timestamp, no offset, no zone. An HTTP `Date` is defined to be GMT, so it would carry nothing even if
it were there. What KOsync stores as `last_read_at` is `time.Now().UTC()` at the moment the push
lands: not the device's reading time, the server's receive time.

So the zone cannot come from the device. It comes from the browser, at registration, via
`Intl.DateTimeFormat().resolvedOptions().timeZone` — no prompt, no library — and lives on the
account, editable afterwards.

### 24.2 Why this was not free

Everything computed in UTC was not a decision, it was the absence of one, and it had already spread.
`analytics.Enqueue` formatted the day in UTC and seven queries compared `substr(last_read_at, 1, 10)`
against a date. Giving an account a zone means all of that moves.

The replacement is a half-open range of UTC instants, `dayBounds`, rather than an offset applied
inside SQL. An offset is wrong twice a year — the last Sunday in March is 23 hours long in Vienna and
the last in October is 25 — and a range is not. It is also the faster form, because `last_read_at` is
indexed and a range reads the index while a substring cannot, so the correctness fix paid for itself.

Two places go the other way, turning instants back into day labels, and they do it in Go rather than
in SQL because SQLite has no notion of a zone that observes daylight saving. The zone database itself
is compiled in with `import _ "time/tzdata"`, because a container built from a minimal base has no
`/usr/share/zoneinfo` and every account would otherwise fall back to UTC silently — the same bug
arriving by a different route.

### 24.3 Changing a zone is a recomputation, not a reinterpretation

Moving the boundaries makes every stored day wrong at once, so the change requeues every day the
account has ever read, plus the days the old boundaries produced, which nothing else would ever
revisit. It is done in an update hook rather than at the one moment a person first picks a zone,
which means the first choice and every later one go through the same code — the only way the later
ones can be trusted.

Nothing is lost: it is all recomputed from `documents` and `document_history`, which this never
touches. But numbers move, and the interface says so before the change rather than after.

The retention cutoff is the one place left approximate. It compares a UTC-derived date against stored
local dates for every account at once, so it can be a day out at the boundary. For a window measured
in hundreds of days that is not worth a per-account pass, and saying so is better than a comment that
implies it is exact.

### 24.4 The metadata KOReader was willing to send all along

Found while reading the plugin for the timezone answer: a setting called "Send document metadata",
off by default, whose help text says the data "is ignored by the official sync server but may be used
by custom sync servers". That is this server exactly, and `ProgressRequest` had never read it.

It earns its place on the unmatched documents. A document that matches no uploaded book has no name
but its hash, and those are precisely the ones the documents page is for. So `documents` gained
`filename`, `authors` and `filename_hash`.

The three are treated differently on purpose. The filename is overwritten on every push, because it
describes the file as it is on the device now. The authors likewise. The title is only ever filled in
— it is the one thing on a document a person can edit, and a device that keeps sending the
publisher's title must not undo a rename on the next sync, which is the same rule the device names
already follow.

`filename_hash` is the KOReader filename hash of the reported name, which makes it a second exact key
to match a book by: a device identifying documents by content still says what the file is called, and
a book may be stored under that name. Exact comparison against an indexed column, not a guess at a
title — the project's stance on matching has not moved. The match also runs when a name first arrives
rather than only when a document is created, so turning the setting on links the documents that are
already there instead of only the next one.

---

## 25. Achievements (phase 13)

Built. §13 asked for "pages/books/streak rules, SVG icons, repeatable tiers", and every part of that
survived except the word *repeatable*, which turned out to mean something worth being precise about.

### 25.1 The tier is the ring, not a second cat

Eight rules, three tiers each, and the tier is a colour on the badge's ring rather than a new drawing.
That is what keeps the art budget finite: eight rules is eight cats, not twenty-four. The name does not
change with the tier either — the name is the badge's identity and the tier is how far it has been
taken, which is how a person reads a badge anyway.

The drawings are inline SVG in the web interface, mounted once as a hidden sprite and referenced with
`<use>`. They ship with the code rather than being uploaded, which means they are versioned with it,
recoloured by CSS, work in both themes and cost no request. The whole set is under 18 KB. The fur is
two custom properties, so a new coat costs two hex values and no new drawing.

### 25.2 Nothing is ever revoked, and that is a design

Every measure is recomputed from live data, and live data moves backwards. History can be deleted. A
re-read puts progress back to the start. The retention window eventually removes the very daily rows a
streak was counted from, so a hundred-day streak becomes unmeasurable a year later.

If achievements were derived on read, all three would take a badge away. So they are not derived: a
row is written the first time a threshold is crossed and never removed. An achievement records that
something happened, and it having happened does not stop being true.

This also disposes of the retention problem entirely rather than working around it, and it is why the
collection has no delete rule — one that could be deleted would make the claim a lie.

### 25.3 The rules are code, and the browser asks for them

Each rule is a question only code can ask. "How many nights did you read past midnight" is a timezone
conversion. "How many books have you finished" is a query over the current state *and* its history,
because progress goes backwards when a book is started again and having finished it once is the thing
being counted.

So the rules live in `internal/achievements` and the interface reads them from
`/api/kosync/achievements` with the progress in the same response. A copy of their names and
thresholds in TypeScript would have been a second place for the two to disagree from, and the one
thing worse than an achievement nobody can earn is one that says different things in two places.

Unearned rules are served too, drawn drained of colour. A badge nobody has yet is the only thing on
the card that says what there is to aim at.

### 25.4 Where it runs

After the statistics batch, once per account rather than once per day. The measures are whole-account
totals, so running them per day would ask the same question eighty times during a bulk requeue and get
the same answer eighty times. A batch is also exactly the moment the numbers an achievement is
measured from have finished moving.

Awarding is idempotent, which is what makes that safe: evaluating an account that has earned nothing
new writes nothing. And a first evaluation awards every tier the account already qualifies for at
once, because dribbling a year of history out one badge per day would be a lie about when it happened.

### 25.5 What phase 12 and the timezone made possible

*Night Prowler* could not have been built before §24. In UTC the midnight boundary falls somewhere in
the middle of a European evening, so "read past midnight" would have counted the wrong sessions for
everyone outside Greenwich. It is the first feature that actually needed the account timezone rather
than merely being tidier for having it.

A night is named after the day it began, so a session from 23:30 to 00:30 is one night and not two —
which is what a person means by "I was up reading".

*Page Turner* rests on phase 12's measured page counts, and sums `reading_days` and `reading_months`
together, because retention folds aged out days into months and deletes them. Counting only the days
would have quietly shrunk a lifetime total every time the retention job ran.

### 25.6 The second three

*Sunbeam Sitter* is the mirror of *Night Prowler* and cost almost nothing for it: the same distinct
moments, a different band of hours. The two share the boundary at 05:00 and do not overlap, so no
single reading can be credited as both a late night and an early morning. It is why the hour has a
name in the code rather than being written as a number in two places.

*The Long Sit* is the most pages read on any one day, from `reading_days` alone: a month holds a sum,
and the sum of a month is not a day anybody had. That means the record day eventually ages out of the
retention window and stops being measurable, which is the clearest case yet for §25.2 — the award
outlives the evidence on purpose.

*Nine Lives* counts books finished and then begun again, and is the first rule to ask the history a
question about *order*. A book being read for the second time looks exactly like one being read for
the first, so the test is a finish with a fresh start after it: the earliest moment the document stood
at the end, against the latest it stood near the beginning. A low position before the finish is just
where the first reading started, and comparing the two extremes that way excludes it in one pass.

Three of the four ideas that were turned down are recorded here because the reasons outlast them.
Anything that rewards an action rather than reading — a badge for merging, a badge for uploading —
turns a feature into a chore. Total reading time is not comparable across devices in the way pages
are (see [analytics.md](../analytics.md)). And distinct authors would need `books.authors` normalised
first, or "Sapkowski, Andrzej" and "Andrzej Sapkowski" are two people.

A rule built on `reading_days` shrinks as the retention window moves. Where a future measure can be
asked of `reading_months`, `documents` or `document_history` instead, it should be.

---

## 26. Mail (phase 14)

Built, and mostly by deciding what not to write. §14 asked for "recovery + achievement notifications
via PocketBase SMTP", and the first half was already there: verification, password reset and the
confirmation of an address change are PocketBase's, sent from its own templates and configured in the
superuser interface. There is nothing to reimplement and no reason to hold a second opinion about it.

What was left is the one message this server had to write itself, because nothing else knows what an
achievement is.

### 26.1 Off until asked, and asked twice

A boolean is false when nobody has ever set it, and for mail nobody asked for that is the safe end to
land on. So `users.achievement_mail` is positive — true means send — and an account created by a
script or in the superuser interface stays quiet until somebody ticks the box. Registration in the
browser sets it, because a person registering in a browser is a person who wants to hear about their
own reading; the accounts that predate the field are backfilled to on, for the same reason.

The operator has a separate switch, `ENABLE_ACHIEVEMENT_MAIL`, for a server that should send nothing
of its own at all. Both have to agree.

Nothing goes to an unconfirmed address. That is partly ordinary hygiene and partly this codebase's own
history: the legacy import parks every account it creates on `@invalid.local`, and mail to those
bounces forever without anybody being told.

### 26.2 One message per batch

Never one per badge. A first evaluation of an account that has been reading for years awards a dozen
tiers at once — §25.4 insists on that, because dribbling them out one a day would be a lie about when
they happened — and a dozen mails about it would be a bug rather than a celebration. The subject names
the badge when there is one and counts them when there are more.

The message says what was earned in words. The badges are cats and the cats are SVG, which mail
clients either strip or refuse to draw, so it links to the page they live on instead.

### 26.3 Off the drain loop

Sending is a network call to somebody else's server, and `net/smtp` has no dial timeout of its own: a
mail host that accepts the connection and then says nothing would stall the statistics queue for as
long as it felt like. So the notice goes out in its own goroutine, and nothing waits on the result —
the badge is stored and on the dashboard before this is reached.

`Worker.Stop` does wait for it, which is new: until now the worker had nothing outstanding when its
loop ended. A shutdown that closed the database underneath a message halfway out would produce an
error nobody could act on, about mail nobody would receive.

Nothing is retried. The mail is a courtesy about something already recorded, and a second attempt
risks sending it twice rather than not at all.

### 26.4 What the tests had to be told

The configuration in the analytics tests is a zero `Config` put through `Normalize`, which fixes
numbers that are out of range but cannot know that a boolean's documented default is `true` — those
come from the `env` struct tags, and only `config.New` reads those. The first version of the worker
test therefore watched for a mail that the worker had been told not to send. Both mail tests now set
the flag they are testing, in both directions, so neither can pass by accident.


---

## 27. After the plan (phases 18 to 20)

Everything in §13 is built. These three were not on the list: two were gaps the plan admitted to and
one is a feature that only became cheap once the mail existed.

### 27.1 The reconcile that §18.1 asked for

Matching happens three times as things arrive: on the push that creates a document, on the rename
that first gives it a filename, and on the upload that brings a book to documents already waiting for
it. All three can fail, and none of them can be retried — a device must never lose a reading position
over a link that is only a convenience — so a failed match used to be permanent, with re-uploading
the file the only way out.

`books.Reconcile` asks the whole question again, nightly at 03:30. It is one join rather than a
lookup per document, because the ordinary night is a library where nothing is missing: on an account
with two hundred documents and no gaps it is a single indexed scan that returns nothing.

Two details are load-bearing. The emptiness guard in the join — a book with no catalog hash and a
document no device ever reported a filename for both hold an empty string, and two empty strings are
equal, which is a match a naive join would happily make. And the repair goes through `app.Save`
rather than an `UPDATE`, because the link is what the statistics count pages by: saving the record
queues every day of that document for recomputation and puts the progress on the cover without a
reload. A run with nothing to do writes nothing at all, which the tests check by watching the
`updated` timestamp — a nightly job that rewrote every linked document would requeue every day of
every book, every night.

### 27.2 A quota, off by default

`BOOKS_QUOTA_MEGABYTES` defaults to `0`, meaning no limit. That is not indecision: this server was
written for one reader on their own disk, and a limit is a decision about somebody else's library.
The moment there is a second account, it is the thing to set.

The size had to be stored, because PocketBase keeps the name of a file on the record and its size
only on the filesystem, and a quota needs a sum over the whole library on every upload. `file_size`
is written as the upload is described, and the migration measured the books that predate the column
once, from the filesystem. A file that has gone missing stays at zero rather than failing the
migration: a library with one broken record in it must still start.

The check is on the request hook, not the record one, so the importer, the migrations and the tests
can still write books server side. That is the honest place for it — a quota is a rule about what
people may ask for, not an invariant of the data. An operator who lowers the limit below what an
account already holds has broken nothing: every book stays, no more can be added.

Superusers are exempt, because locking the administrator out of the instance they administer would be
absurd.

The interface shows a bar, fed by `GET /api/kosync/storage` rather than by summing the books in the
browser — half the answer is a server setting the browser cannot know, and sending both from one
place means the bar and the refusal can never disagree about what full means.

### 27.3 The summaries

The one new capability. It is small because everything it needs already existed: the daily rows are
precomputed, the per-book rows are precomputed, the achievements are recorded, and phase 14 built the
sending. What was left was three queries and a decision about time.

**The decision about time.** A summary covers the period that has *finished* — a report on a week
still being read would be wrong by the evening — and it arrives at eight in the morning in the
account's own zone, because the mail is about reading somebody has just done and there is no hurry.
That combination is why the job runs hourly rather than weekly: "Monday at eight" is a different
instant for every account, and on all but one run an hour this job finds nobody to write to.

**`summary_sent`.** The account records which period it was last written to about. That one column
turns an hourly job into a weekly message, and it means a server switched off over the weekend still
sends Monday's summary on Wednesday, once, rather than either skipping it or sending it three times.

The mark goes down *before* the message goes out, deliberately. A mail host that is refusing
connections would otherwise be retried on every hourly run for the rest of the week. A summary is a
courtesy about reading that is already on the dashboard: missing one is a smaller harm than a hundred
attempts, or than sending the same summary twice because the failure came after the message was
accepted.

**A period with no reading in it is not mailed.** The period is still marked, so this does not become
a weekly reminder of not having read.

**Only books are named, not documents.** A document nobody has uploaded the file for has no title
beyond its hash, and "you read 40 pages of 043f11771ef9d191364ac0ba08198d36" is not a sentence worth
mailing anybody. Those pages are still in the totals, which is where they belong.

**Not backfilled, unlike `achievement_mail`.** Those accounts were asked about milestone mail once;
nobody asked them about a weekly digest. `off` until somebody picks a cadence under **Account**.

### 27.4 What is left

- A release. `main.go` still says `26.08.0-dev` and there are no tags, which is now the only thing
  making a finished rewrite look unfinished.

### 27.5 The history that was fetched for nobody

Reported from the real instance: every page load lagged, and the network panel showed the history of
every document being fetched.

It was. `documents.load()` fetched the whole `document_history` collection alongside the documents,
joined the two in the browser, and did it on every view that shows a document — the dashboard, the
documents page, a book page. On the reference instance that is **10,890 rows for 11 documents**, and
`getFullList` pages at 500, so it was twenty-two sequential requests and ten thousand records parsed
into objects before anything could render.

All of it to fill a dialog that shows one document's history when somebody clicks the button.

The history is now fetched there instead, filtered server side by `document_ref` — which the index on
`(document_ref, last_read_at)` has always served. A page load is one request for eleven rows.

Two things had to come with it, and both are the kind of detail that makes a lazy load wrong when it
is left out:

- **Realtime events are ignored for a document whose history has not been fetched.** Folding a single
  live event into an empty array produces a list of one entry that looks like the whole story.
- **A reload keeps the histories already in hand.** Reloading happens after every merge and every
  restore, and it must not empty a dialog somebody is looking at.

The lesson is not about this query. It is that "load everything the page might need" and "load
everything the account has" stop being the same thing at a scale that arrives quietly.

---

## 28. The statistics sync target (phase 21)

### 28.1 The hole the protocol cannot see

The KOReader sync protocol carries no clock (§24.1), so `last_read_at` is the moment a push *arrives*.
A device that reads offline and reconnects therefore lands a fortnight of reading on one day, with no
reading time attached — a single push has no gap to measure — and the days actually read on hold no
rows at all.

Measured against the reference data, that is not a corner case:

| | from pushes | from KOReader's own statistics |
| --- | --- | --- |
| Reading time | 70.4 h | **130.9 h** |
| Days with reading | 91 | **105** |

The estimate is capturing 54% of the reading time, and fifteen days of reading are invisible to it.

### 28.2 Why WebDAV, and why it is the whole feature

KOReader's statistics plugin syncs its database to Dropbox, FTP or WebDAV. Only one of those is
something this server can be, and it happens to be the one that is HTTP with more verbs — so it is
served from the same port, behind the same device credential, as the catalog.

That is the entire reason this is worth building. A statistics database that has to be carried off an
e-reader by hand is one that gets carried once, in the first week, and never again. The value is not
in the file format; it is in nobody having to think about it.

`golang.org/x/net/webdav` does the protocol and `modernc.org/sqlite` reads the upload. Both were
already in the module graph as indirect dependencies of PocketBase, so this cost no new supply chain
at all.

### 28.3 What keeps a sync target from being a file host

Four things, and the first two are what matter:

- one directory per account, named by the authenticated owner and never by the request path;
- one permitted file name, so nothing anybody can choose ends up on the disk;
- a size cap, applied both to the request body and to the writes, because the second catches a client
  that lies about its length;
- and a schema check: the SQLite header first, then the tables and the columns the import will read.

Only the columns that will actually be read are required. A KOReader release that adds one must not
be refused by a server that has not heard of it yet — that would turn a routine device update into a
sync that silently stops.

The upload is written beside the stored copy and renamed over it only once all of that passes, so an
interrupted sync leaves the previous copy whole, and a reader downloading during an upload gets one
version or the other rather than the seam. What is stored is returned byte for byte, because the
plugin merges its own history into the copy it downloads: a server that rewrote the file would be
handing back something the device never wrote.

Everything refused is logged, deliberately. The shape of KOReader's own client is still being learned,
and a refusal that says nothing would mean a device that will not sync and a server with no opinion
about why.

### 28.3.1 The route that would not start

The first version bound the endpoint to every method at once, which is the obvious way to serve a
protocol with thirteen verbs. It made the server panic on boot.

To net/http, `"/webdav/{path...}"` bound to every method and the web interface's `"GET /{path...}"`
are two patterns where neither is more specific than the other: the first matches more methods, the
second a more general path. That is a conflict, and a conflict is a panic while the mux is being
built — after every test in the package had passed, because not one of them mounted a web interface
beside the endpoint. The fix is to name the thirteen verbs, which makes this route strictly the more
specific one where the two overlap and disjoint everywhere else.

The test that now guards it registers both, at the priority `registerWebUi` uses, and it was checked
the only way such a test is worth anything: by putting the old route back and watching it reproduce
the panic.

### 28.3.2 What the first real sync showed

A Boox Go 7 running KOReader for Android, configured against `/webdav/`:

```
PROPFIND /webdav/                  207
GET      /webdav/statistics.sqlite3 404   (no remote copy yet)
PUT      /webdav/statistics.sqlite3 201
```

Nothing was refused. No MKCOL, no upload under a temporary name followed by a MOVE, no LOCK — so the
strict allow-list is the right shape and the refusal log stayed empty, which is the outcome it was
built to be able to report.

Two defects did turn up, and only because the directory was looked at afterwards:

**SQLite left sidecars in the account's directory.** KOReader's database is in WAL mode, and opening a
WAL database — even read only — makes SQLite create `-shm` and `-wal` beside it. Validation opened the
temporary upload, so every sync left two files behind for ever, in the one directory that is supposed
to hold exactly one thing. Fixed twice over: the connection now says `immutable=1`, which is true of a
file that has already been written and closed and which makes SQLite read it in place and create
nothing; and the upload's cleanup removes any sidecar regardless.

**The listing showed them.** PROPFIND read the directory as it found it, so a client asking what was
there was told about files the server had promised not to keep. The directory handle now filters to
the one permitted name.

The first of those is why the regression test builds its fixture in WAL mode. A statistics database in
SQLite's default journal mode would never have provoked the sidecars, and the test would have passed
against the bug — which it did, on the first attempt, until both defences were removed together to
prove it could fail.

### 28.4 The import

Written after the endpoint had been given a real database rather than against a guess at one, which is
why it is a separate section and was a separate day's work.

`book.md5` is the same partial MD5 already stored in `documents.document` and `books.hash_binary`, so
the matching is exact rather than heuristic — 32 of the reference instance's 39 synced documents match
by string equality, and the reference database imported as 5,644 page turns over 107 days.

**Events, not summaries.** `page_reads` holds one row per page turn. Summarising on the way in would
bake in a day boundary that depends on a timezone the account can change afterwards, and the events
are also what makes the import idempotent: the unique index is KOReader's own key, so re-importing a
database that has grown by a week inserts exactly that week and nothing else. The reference database
imported twice adds 5,644 rows and then none.

**No stored link to a book.** The books are matched by a join at recomputation time, over the same
three hashes the catalog matches on. A link written at import time would have to be repaired when a
book was uploaded later — which is the bug this codebase has already had twice (§18.1), and declining
to introduce it a third time is cheaper than the cron that would have to clean up after it.

**The merge rule.** Where a day has measurements they win: reading time becomes the sum of the
durations, pages the count of distinct pages, and a measured day is kept even when no push ever landed
on it. What only the pushes can know — the update count, the progress through a document — stays
theirs. This is the same shape as `measured_pages` beating `page_count` (§19), one level up.

One invariant is deliberately broken by it. On a measured day the day's pages are no longer the sum of
its book rows, because a device measures pages in files that were never uploaded and there is no book
to attribute them to. Keeping the sum tidy would mean throwing that reading away.

**Off the request.** The import runs in its own goroutine after the upload is stored, like the
achievement mail, and a shutdown waits for it. Nothing is lost if it is interrupted: the file is
already on disk, and the next sync imports it again from the beginning, which is a no-op for
everything that was already there.

### 28.5 What does not change

Progress sync does not become secondary. `page_stat_data` has a page number and no xpointer, and a
page number is font-size dependent — which is why the plugin ships a view that rescales them. It
cannot put a reader back on the right line on another device. The division is clean: **the sync
protocol owns where you are, the statistics own what happened**, and where both describe a day the
measurement wins, exactly as `measured_pages` already beats `page_count` (§19).

---

## 29. Browsing a library that got big (phase 22)

Reported once the reference library passed about two hundred books: the OPDS catalog is three flat
shelves, and skimming it on an e-reader stopped working. What follows is what was measured before
anything was designed, so that the cost of each idea is known rather than assumed.

### 29.1 What the library actually holds

First counted at 121 books, then recounted with the finished parser over all **192**:

| | |
| --- | --- |
| Books with authors | **192 / 192**, 115 distinct spellings — 108 actual authors, see §29.3 |
| Books with a language | **192 / 192** — 9 stored spellings, 5 actual languages |
| Books with an ISBN | **28 / 121** at the time of measuring — the rest carry a Calibre UUID and nothing else |
| Books with a series | **30 / 192**, in 9 series |
| Books with subjects | **80 / 192**, 334 subjects, 202 of them distinct |

Two of those change the design.

**Series is the strongest facet and it is not stored.** The sampled files carry
`calibre:series` with a `calibre:series_index` — "A Song of Ice and Fire" at 1, 2 and 4 — which is
exactly the thing a reader wants to walk down, and `internal/epub` currently reads title, authors,
language and identifiers only.

**Subjects are present and mostly junk.** One sampled book declares six subjects that are all the
same phrase rearranged: "dark fantasy romance", "dark romance books", "dark fantasy romance books",
and so on. Others declare a clean single "Fantasy". Counted over the whole library with the
parser's own rules, **143 of the 202 distinct subjects belong to exactly one book** — a navigation
feed of 202 entries, most of them leading to a single title, is a worse way to find something than
the flat list it would be replacing.

### 29.2 The three pieces, in dependency order

**Series and subjects had to be extracted first** (task #34, built). `internal/epub` read title,
authors, language and identifiers only. It now also reads the series both ways it is written — EPUB 2
`calibre:series` with its `calibre:series_index`, and EPUB 3 `belongs-to-collection` refined by
`collection-type` and `group-position` — and the `dc:subject` list, deduplicated case-insensitively
and capped at 24. Migration `1787961600_book_series_subjects.go` adds `series`, `series_index` and
`subjects` with an index on `(owner, series)`, and backfills them by re-reading every stored EPUB the
way `BackfillFileSizes` already does. `series_index` is not an integer column: the reference library
holds volumes numbered 0.1, 16.5 and 17.5.

**The catalog grew a second kind of feed** (task #35, built). A `shelf` has a `list` function
returning books; a `facet` (`internal/opds/facets.go`) has a `groups` function returning values with
counts. The routing, pagination, authentication and rendering were already there. Three facets ship,
and the front page offers only the ones with something in them.

**Hand-made collections** (task #36, built) are independent of all of it and need no external data — an
owned record with a relation to books, exposed as its own feed. They are also the honest answer to
§29.1's subject problem: a shelf somebody curated beats a facet built from keyword spam. §29.5 has
what they turned into.

### 29.3 What the finished feeds do

Three facets, at `/opds/authors`, `/opds/series` and `/opds/languages`, each entry linking to
`/opds/by?facet=…&value=…`. On the reference library:

| Feed | Entries | What it looks like |
| --- | --- | --- |
| By author | 108 | `Lee Child (29)`, `William Shakespeare (39)` — three pages of 50 |
| By series | 9 | `Jack Reacher (19)`, `A Song of Ice and Fire (3)` |
| By language | 5 | `German (96)`, `English (92)`, `French (2)`, `Dutch (1)`, `Unknown (1)` |

Five decisions are worth keeping written down.

**No subject facet.** The column is stored and the web UI can show it, but 143 of 202 subjects lead
to one book. Curated collections are the answer to that problem, not a feed.

**One author is one shelf, however the name is spelled.** The 115 distinct strings in the author
field are 108 people. Publisher metadata writes a name either way round and punctuates initials as it
pleases, and the reference library holds both halves of five authors: `Lee Child` on 26 books and
`Child, Lee` on 3, `George R.R. Martin` beside `George R. R. Martin`, `J.K. Rowling` beside
`J. K. Rowling`, `Gabaldon, Diana`, `Publishing, Pottermore`. Split apart, the library's most-read
author sits in two places in the catalog and neither one is the whole of him.

`internal/books/authors.go` turns a name round at the *first* comma only, and refuses when what
follows is not a given name: more than three words is a list of people, not one person written
backwards (`Corinna Mieth, Simon Weber, Rainer Schäfer, Anna Schriefl` is four authors in one field
and is left exactly as found), and a trailing `LLC`, `Jr.` or `PhD` is a suffix. What remains is
lowercased down to its letters and digits, and that is the key. Nothing is transliterated and nothing
is decomposed: it is unnecessary for every merge the real library needs, it keeps `golang.org/x/text`
out of the build, and an ASCII-only key would reduce every name in a script without Latin letters to
the empty string — folding all of them into one nameless shelf, which is the bug this decision
exists to prevent. `Lincoln Child` stays separate from `Lee Child`. Ten further names simply stopped
displaying backwards. The grouping runs in Go over one row per name per book (224 rows for 192
books) because SQLite cannot say "the same letters ignoring punctuation" without a pile of nested
`REPLACE`s, and the URL carries a readable spelling rather than the key, so an address bookmarked
under the old spelling still resolves to the merged shelf.

**Languages are folded, not listed.** The library spells German four ways — `de`, `de-DE`, `DE`, and
English three — and shelving the spellings apart reproduces exactly the splitting the feature exists
to undo. The region and the case are dropped, and the tag is shown as a name: `und` is a file
declining to name a language, and "Unknown" is a better shelf to find that book on than "UND".

**A series is shelved in reading order**, by `series_index` and only then by title, because a series
read alphabetically is a series read in the wrong order. An unnumbered volume sorts to the front.

**The value travels in the query string.** Author names carry slashes, dots, ampersands and
apostrophes; a path segment that has to survive a reader, a proxy and a router without any of them
normalising it away is a much narrower target than a parameter. Shelves and facets also share one
route pattern — two patterns differing only in the name of their wildcard are the same pattern to
Go's router, and registering both panics at start up.

### 29.4 The same shelves in the browser

The catalog and the web library are two views of one shelf, and the operator asked for the second one
to group the way the first does. The library page grew a **Group by** control — nothing, author,
series or language — which turns the flat grid into headed sections, remembered in `localStorage`
because somebody who browses by series today will want to tomorrow. The dashboard's strip of recently
read books is never grouped: it is a shelf, not a catalogue, and six books in headed sections is
noise.

Two rules the browser has that the server did not need. **Nothing may vanish.** A facet feed simply
omits the books with no value for it, which is right for navigation and wrong for a page that claims
to be the whole library — so a grouping ends in "Without a series" or "Without an author" when it has
to, and a file that names no language is shelved with the ones that said `und`, since both mean
nobody knows. And **the name on the card is the name on the shelf**: the grid used to print the
authors exactly as the file wrote them, which under author grouping put "Child, Lee" beneath a shelf
headed "Lee Child". Both the grid and the book page now show the tidied form.

**The folding rule is written twice, and that is the risk this section exists to record.** The
grouping happens in the browser, over a library it has already loaded in full — an endpoint would be
a round trip to learn something the client is holding, and would go stale against the live
subscription that keeps the grid current as books arrive. So `webui/src/lib/grouping.ts` is a second
implementation of `internal/books/authors.go`, and two implementations of one rule drift. The guard
is `testdata/author-names.json`: every case the fold is held to lives there, and both
`server/internal/books/authors_test.go` and `webui/src/tests/lib/grouping.test.ts` read that same
file. Changing the rule on one side without the other fails the other side's tests. Adding a case
tests both.

### 29.5 Collections somebody made (task #36)

The one part of the library that is nobody's metadata: a named shelf, a description, and a list of
books in the order its owner put them there. `book_collections`, one migration, one request hook, a
fourth facet, two pages in the browser.

**The order is the whole content.** Everything else in the library can be reconstructed from the
files: titles, authors, series, languages, even the page counts. A reading list cannot — it is a
sequence somebody decided on, and the moment anything sorts it, it is gone. So the order the ids sit
in the relation field *is* the shelf, and it survives everywhere: `listCollection` loads the shelf's
owned books and re-orders them in Go by their position in the stored list before paging, because
there is no SQL sort that means "the order this list is written in"; the browser's grid takes a given
list and passes it through untouched, with the Group by control forced off, because grouping would
sort it. Two catalog tests pin it, one of them across a page boundary — a page-two query that sorted
by anything would quietly return the wrong three books.

**It is called `book_collections` and not `collections`.** PocketBase calls its own tables
collections. The API path would have read `/api/collections/collections/records`, and every sentence
in this document about either kind would have needed a qualifier.

**"The books must be yours" is a hook, not an access rule.** A rule can only see the record being
written, and the ids in `books` point at another table; without the check an account could name a
stranger's book id and read the title back out through relation expansion. `internal/collections`
counts how many of the submitted ids belong to the caller and refuses the write unless the counts
match. The hook reads `e.Record`, which is the *merged* list — so it also covers the `books+`
modifier, which is what the browser actually sends. The owner guard runs before the book check, in
that order deliberately: a changed owner makes every book on the shelf foreign, and the answer would
then be true but about the wrong thing.

**Deleting a book empties shelves; it does not delete them.** The books relation sets
`CascadeDelete: false`, and that single field is the difference between the two behaviours: PocketBase
deletes the referring record when a cascading relation would be left empty, so with `true`, deleting
the last book on a shelf would delete the shelf. A test exists for exactly this, because it is one
field option away from being wrong and nothing else would notice.

**The browser sends changes, not lists.** Putting a book on a shelf sends `{"books+": id}` and taking
it off sends `{"books-": id}`, so two open tabs cannot overwrite each other's shelf. Rearranging is
the one exception and has to send the whole list, because there is no modifier for "third, not
fifth".

**An empty shelf is a plan, and where it shows depends on what the page is for.** The catalog leaves
it out — `json_array_length(books) > 0`, the same rule that keeps an empty facet off the front page,
because a dead link costs a visible page turn on e-ink. The collections page in the browser lists it,
because that is where it gets filled.

Names are unique per account (`idx_book_collections_owner_name`), a shelf holds at most 2000 books —
the whole reference library is 192 — and the feed's value is the record id rather than the name,
which is stable across a rename and needs no escaping.

### 29.6 The ISBN lookup, and what limits it (deferred)

Filling in what the file does not carry — subjects, series, publisher, a better cover — by asking an
external catalogue (task #37). Three things bound it, and the first is the one that decides:

1. **Only 23% of the library has an ISBN.** An ISBN-keyed lookup reaches a quarter of these books.
   The rest would need a title-and-author search, which is a guess with a confidence attached rather
   than a lookup.
2. **It tells somebody else what is in a private library.** That has to be opt-in, off by default, and
   per-book rather than a sweep — and a self-hosted instance may have no outbound network at all.
3. **Licensing differs by source.** Open Library needs no key and its data is CC0; Google Books
   restricts what may be stored from its responses.

Whatever it fills, it must fill only what is empty and record where each value came from — the same
rule the upload already follows when publisher metadata meets a title the owner corrected.

Left out of this phase on the operator's call. It is recorded here rather than built.
