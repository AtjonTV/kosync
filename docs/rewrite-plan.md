<!--
File:        docs/rewrite-plan.md
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
| `pages_read` | number (0 until books provide page counts — phase 12, see §16.5) |
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
| POST | `/api/kosync/documents/merge` | **phase 7** — fold N documents into one, re-parent history, recompute analytics |

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

1. Progress arrives (KOReader push, WebUI edit, history restore, later: merge).
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
| **12. Book statistics** | per-book/day rows, book detail view, notional page counts | 9 + 3 |
| **13. Achievements** | pages/books/streak rules, SVG icons, repeatable tiers | 12 (page counts) + 3 (daily rows) |
| **14. Mail** | recovery + achievement notifications via PocketBase SMTP | 13 |
| **15. Library-first dashboard** | the library becomes the main component on `/`, below the statistics; the documents table moves to a page of its own | 8 |
| **16. Navigation** | left: home, library, later documents; right: address, account settings and sign out merged into one menu | 15 |

Phases 9 and 10 are independent of each other and can be built in either order; §16 explains why doing
10 first makes 9 exact rather than heuristic for anything downloaded from the catalog.

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

Neither of these adds capability; both are about the shape the app presents once there is a library in
it. Recorded now because the current layout was designed when documents were the only thing to show.

### 17.1 A library-first dashboard (phase 15)

Today `/` is the statistics plus the documents table, and `/library` is a separate page. That has it
backwards: the library is the thing with covers, and the covers are what make a dashboard worth
looking at. The intended shape is

- `/` — statistics first, then the library as the main component below them,
- and the documents table moved to a page of its own.

Worth doing before the documents table gains anything else, since each new feature there is one more
thing to move.

Two questions to settle when building it, not before: how much of the library belongs on the dashboard
(everything, or the recently read), and whether the documents page and the library end up merged once
matching (phase 9) means most documents *are* books. The second question is the more interesting one,
and the answer probably depends on how many pushes stay unmatched in practice.

### 17.2 Navigation (phase 16)

The top bar grew one control at a time and now reads as a row of unrelated icons. The intended shape is

- **left** — home, library, and later documents, as ordinary navigation,
- **right** — the address, account settings and sign out collapsed into a single account menu.

That leaves the theme toggle as the only free-standing control on the right, which is where it belongs.

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

One gap remains by choice: if a match fails transiently, that document stays unlinked until the book is
re-uploaded, because nothing re-examines existing pairs. A periodic reconcile would close it. It is not
built, on the grounds that the failure is logged and the cost is a page of missing progress rather than
lost data — but if unmatched documents ever show up in practice, that is the thing to add.

### 18.2 What phase 9 deliberately left out

The plan's phase 9 also said "unlinked pushes listed separately". That is interface work in the
documents table, which phase 15 moves to its own page — building it now means building it twice. It
belongs with that restructure, where the question of whether documents and the library remain separate
views gets settled anyway.

What did land in the interface is the more interesting half: the library shows reading progress on
each cover, so a matched book reads as "Reading 63%" or "Finished" at a glance. That is in the new
component, which is not going anywhere.
