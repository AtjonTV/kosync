# Contributing to KOsync

Thank you for considering a contribution. This document is the project's own conventions; the rules
that govern contributions themselves — in particular how AI assistance is attributed — live in
[MACHINE_POLICY.md](MACHINE_POLICY.md) and apply to every commit.

## 1. Getting started

### 1.1 Prerequisites

- **Go** 1.26 or newer, for the server
- **Bun** 1.3 or newer, for the web interface
- **Node** on the `PATH`, for the type check only — see [docs/build.md](docs/build.md) for why

Nothing else. No Redis, no Nginx, no database server: PocketBase brings SQLite with it.

### 1.2 The layout

| Directory | What is in it |
| --- | --- |
| `server/` | the Go module: PocketBase application, KOReader protocol, analytics, OPDS, WebDAV |
| `webui/` | the Vue 3 web interface, built with Bun and embedded into the server binary |
| `docs/` | the documentation, including [docs/rewrite-plan.md](docs/rewrite-plan.md) |
| `deployment/` | Dockerfiles |

### 1.3 Building

```bash
cd server
go run build.go          # builds the web interface, embeds it, writes ./kosync
go run build.go -run     # and starts it afterwards
go run build.go -web=false   # server only, reusing what is already embedded
```

For the dev servers, Docker, and the parts that need Node, see [docs/build.md](docs/build.md).

## 2. Testing standards

### 2.1 Go

- For each `source.go` a corresponding `source_test.go` **must** exist in the same directory.
- Tests use Go's built-in `testing` package **ONLY**. **External assertion or mocking frameworks
  (e.g. `testify`) are FORBIDDEN.**
- Do **not** mock the database. `testutil.NewApp(t)` returns a throwaway PocketBase instance built
  from an empty data directory and migrated, so a test runs against the schema the migrations really
  produce — including the collection access rules. `testutil.NewUnmigratedApp(t)` is the same thing
  before the first migration, for testing the migrations themselves.
- Fixtures come from `internal/testutil` (`CreateUser`, `CreateKoreaderAccount`, `PadId`, `Md5Hex`).
  Add to it rather than hand-rolling records in a test.
- Never point a test at real data. The reference EPUBs and any production database stay outside the
  repository.

```bash
cd server
go test ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

### 2.2 Web interface

Tests live under `webui/src/tests/`, mirroring `src/`, and run on Vitest with `@vue/test-utils`.
PocketBase is mocked through `src/tests/mocks/pb`, never called for real.

```bash
cd webui
bun run test
bun run test --coverage
```

## 3. Code style

### 3.1 File headers

Every source file (Go, TypeScript, Vue, and the configuration files that allow comments) **MUST**
start with:

```go
//
// File:        ${path}
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © ${year} ${legal_name}. Licensed under the EUPL-1.2 or later
//
```

In a `.vue` file the same four lines go into an HTML comment (`<!-- ... -->`) above the
`<script setup>` block. The path is written the way the file is reached from its own project: Go
files under `server/` drop that prefix (`internal/opds/feed.go`, `main.go`), web interface files
keep theirs (`webui/src/pb.ts`).

**Updating copyrights:**
- When you edit a file for the first time, add another copyright line below the last existing one.
- When you edit it again in a later year, extend your own year range (e.g. "2026" to "2026-2027").

### 3.2 Go

- `gofmt` decides the formatting; tabs for indentation. CI fails on anything `gofmt -l` prints.
- `go vet ./...` and `staticcheck ./...` must be clean.
- Modern idioms: `any` over `interface{}`, `errors.Is`/`errors.As` over comparison, the `slices` and
  `maps` packages, `for i := range n`.
- **Naming:** `PascalCase` exported, `camelCase` internal. A package is a noun and its name is not
  repeated in its symbols (`opds.Handler`, not `opds.OpdsHandler`).
- **Errors:** last return value, wrapped with `fmt.Errorf("...: %w", err)`. An error handed to a
  client goes through PocketBase's `apis` helpers so the status code is right.
- **Comments** explain why, not what. Where something looks wrong but is deliberate — KOReader's
  shift-masked hash offsets, the MD5 the protocol demands — the comment says so and why it must not
  be "fixed". Suppressions (`#nosec`, `// bearer:disable`) are per site, never a blanket skip, so a
  new occurrence elsewhere is still reported.

### 3.3 Schema and migrations

- Collections are defined in `server/internal/migrations/`, written as Go migrations. Changes made
  in the superuser interface at `/_/` are written out there automatically, so they can be reviewed
  and shipped like any other code. Never edit a migration that has already been released.
- Collection and field names are constants in `internal/schema`. Use them; a string literal for a
  field name is a typo waiting to happen.
- Access rules belong on the collection, not in a handler, whenever the rule can express it. A hook
  is for what a rule cannot say — see `internal/collections` for an example and its reasoning.
- Document the change in [docs/database.md](docs/database.md).

### 3.4 TypeScript and Vue

- Vue 3 with `<script setup lang="ts">`, Pinia for state, PrimeVue 4 components, PrimeIcons and
  Tailwind 4 for layout.
- Formatting is Prettier's: two spaces, no semicolons, single quotes, 100 columns. Run
  `bun run format`.
- `bun run lint` (oxlint, then ESLint) must pass with no warnings; CI runs both with warnings as
  errors.
- **Naming:** `camelCase` for variables and functions, `PascalCase` for components
  (`LibraryGrid.vue`), `use[Name]Store` for Pinia stores.
- Talk to the server through the shared client in `src/pb.ts`. Collection names live in
  `Collections`, custom endpoints in `KosyncApi`.

### 3.5 Documentation

Markdown wraps at 100 columns. Anything a user or an operator can observe — a field, an endpoint, an
environment variable, an error message — belongs in `docs/`, and a new environment variable also
belongs in `server/kosync.env.example` with the reasoning next to it.

## 4. Changelog

Add an entry to the `## [Unreleased]` section of [CHANGELOG.md](CHANGELOG.md) for anything a user or
an operator would notice. Write it for the person running the server, not for the person who wrote
the patch, and mark anything that requires action on upgrade with **BREAKING**.

## 5. Git commit messages

Enable the template once per clone:

```bash
git config commit.template .gitmessage
```

- **Summary:** one short line describing the change, then an empty line.
- **Context:** what cannot be inferred from the diff, including why. Explain acronyms that a new
  contributor would not know ("OPDS" needs a word, "JSON" does not).
- **AI assistance:** if an AI agent or LLM wrote part or all of a commit, attribute it with the
  `AI-Agent` and `AI-Model` trailers, one pair per tool used:

  ```text
  AI-Agent: Anthropic Claude Code (v2.1/CLI)
  AI-Model: Anthropic Claude Opus 5 (claude-opus-5)
  ```

  Using `Co-Authored-By` for an AI is **forbidden** — that trailer is for human co-authors
  ([MACHINE_POLICY.md](MACHINE_POLICY.md) §7.2.5 and §7.2.6). Reviewing, testing and committing AI
  written changes remains the contributor's own responsibility (§4.2).

## 6. What CI checks

Nothing below needs a pipeline to run; all of it runs locally.

| Stage | Job |
| --- | --- |
| validate | `gofmt`, `go vet`, `staticcheck`, Bearer, `wwhrd` (licenses), webui types and lint |
| test | `go test` with coverage, `bun run test` with coverage |
| audit | `govulncheck`, `bun audit` |
| build | the static binary, and a Docker image on a tag |
| analyze | SonarQube, on `main` |

`wwhrd` only allows MIT, BSD-2-Clause, BSD-3-Clause and Apache-2.0 dependencies
(`server/.wwhrd.yml`). A dependency under anything else needs a different dependency.

## 7. Code of conduct

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## 8. License

KOsync is licensed under the **EUPL-1.2 or later**. Every new file carries that header, and by
contributing you agree that your contribution is licensed the same way.
