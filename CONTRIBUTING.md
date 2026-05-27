# Contributing to KOsync

First off, thank you for considering contributing to KOsync! This document outlines the project-specific guidelines, conventions, and architectural details to ensure consistent and high-quality contributions.

## 1. Getting Started & Local Development

### 1.1 Prerequisites
- **Go Toolchain** (Go 1.25+)
- **Bun** (for WebUI development)

### 1.2 Building Locally
There are multiple ways to build KOsync depending on your needs. For local development without Docker, a static executable is recommended.

To build the static executable (including the WebUI):
```bash
go run build.go
```

If you want to build and immediately run it:
```bash
go run build.go -run -web
```

To build without the WebUI (and without Bun):
```bash
go run build.go -web=false
```

When building with the WebUI, it must be enabled at runtime by passing `--webui` via the CLI or by setting the `ENABLE_WEBUI` environment variable.

See `docs/build.md` for more deployment and build options (e.g., Docker, dynamic executables).

## 2. Testing Standards

KOsync adheres to standard Go testing conventions to keep the project lightweight and maintainable.

### 2.1 General Rules
- For each `source.go` file, a corresponding `source_test.go` file **must** exist in the same directory.
- Tests must be written using Go's built-in `testing` package **ONLY**.
- **External testing frameworks or assertion libraries (e.g., `testify`) are FORBIDDEN.**

### 2.2 Database Tests
- Do **not** use mocks for database tests.
- Use `NewTemporaryDatabase(true)` to obtain a fresh, in-memory SQLite database instance. This ensures tests are fast, isolated, and realistic.
- Always ensure `db.Close()` is called (typically via `defer`).

### 2.3 Running Tests
Run all tests using the standard Go command:
```bash
go test ./...
```

To run tests with coverage:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 3. Code Style Guidelines

### 3.1 File Headers
Every source file (Go, TypeScript, Vue, etc.) **MUST** include the following header:
```go
//
// File:        ${path}
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © ${year} ${legal_name}. Licensed under the EUPL-1.2 or later
//
```
**Note:**
- Ensure the file path is correct for each file and that the year and legal name are filled in.
- The legal name corresponds to the person who creates the contribution themselves or lets an AI create the contribution for them.

**Updating Copyrights:**
- When editing a file for the first time, you must add another copyright line below the last existing one (see template above)
- When editing a file again, you must check if your copyright year needs updating (e.g., from "2026" to "2026-2027").

### 3.2 Go Conventions
- Use standard `gofmt` for formatting with **Tabs** for indentation.
- Follow modern Go idioms (Go 1.25+):
  - Use `any` instead of `interface{}`.
  - Use `errors.Is(err, target)` instead of `err == target`.
  - Use `slices` and `maps` packages for common operations.
  - Use `for i := range n` for simple loops.
- **Naming:** `PascalCase` for exported symbols, `camelCase` for internal symbols. Database functions prefer `Find[Entity]By[Field]`, `Create[Entity]`, `Update[Entity]`.
- **Error Handling:** Return errors as the last return value, use `fmt.Errorf` with `%w` for wrapping, and log errors using `Klog` or `LogError`.
- **Logging:** Use the `Klog` wrapper from `internal/kosync/log.go`. Tags should be descriptive (e.g., `db/document`, `api/webui`). Levels: `Debug`, `Info`, `Error`.

### 3.3 Database & Models
- **Soft Deletion:** KOsync uses soft deletion for documents. Instead of removing records, the `deleted_at` field (Unix timestamp in milliseconds) is set. Always filter `SELECT` queries with `deleted_at IS NULL`.
- **Migrations:** Stored in `internal/kosync/migrations/sql/`. Use sequential numbering for new `.sql` files.

### 3.4 TypeScript & Vue (WebUI) Conventions
- **Technology Stack:** Vue 3 (Composition API with `<script setup lang="ts">`), Pinia, PrimeVue & PrimeIcons, Tailwind CSS.
- **Formatting:** 2 spaces indentation.
- **Naming:** `camelCase` for variables & functions, `PascalCase` for components (e.g., `DocumentsList.vue`), `use[Name]Store` for Pinia stores.
- **API Interaction:** The preferred method is using `JMPClient` (via WebSocket) as it allows for RPC and PubSub. The `fetchApi` from `webui/src/api.ts` is still available and automatically handles authentication headers for standard HTTP requests.
- **Components:** Always use `<script setup lang="ts">`, prefer PrimeVue components, and use `primeicons` classes for icons.

## 4. Git Commit Messages

When committing code, please configure and use the `.gitmessage` template (`git config commit.template .gitmessage`).
- **Summary:** Write a short summary of the changes you have made on the first line.
- **Context:** Add additional context on subsequent lines. Explain acronyms when they are not obvious for new users (e.g., "JMP" needs an explanation, "JSON" does not).
- **AI Agents:** If AI Agents, LLMs, or similar were used to create parts or the whole commit, specify the agent and model used. Do **not** include the Agent via "Co-Authored-By". Use specific tags:
  ```text
  AI-Agent: JetBrains Junie (v1588/IntelliJ IDEA Ultimate)
  AI-Model: Google Gemini 3 Flash (gemini-3-flash-preview)
  ```

## 5. Documentation & AI Usage

### 5.1 AI-Usage.md
Whenever a file is created or significantly modified using AI, you **MUST** update `AI-Usage.md`.
- Every file must be listed only once.
- Include the file path, interaction type, and the agent/model name.

### 5.2 KDoc / Comments
Follow the existing comment style. Explain complex logic, but avoid over-commenting obvious code. Use clear headings and lists for Markdown documentation files.

## 6. License
The project is licensed under the **EUPL-1.2 or later**. All new files must adhere to this license.
