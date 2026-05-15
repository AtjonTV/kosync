# KOsync Code Style Guidelines

This document defines the coding standards and conventions for the KOsync project. All contributors must follow these guidelines to ensure consistency and maintainability across the codebase.

## 1. General Requirements

### 1.1 File Headers
Every source file (Go, TypeScript, Vue, etc.) **MUST** include the following header:

```go
//
// File:        path/to/file.ext
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © ${year} ${legal_name}. Licensed under the EUPL-1.2 or later
//
```
*Note: Ensure the file path is correct for each file and that year and legal name are filled in/u.*

### 1.2 License
The project is licensed under the **EUPL-1.2 or later**. All new files must adhere to this license.

### 1.3 AI Usage Documentation
Whenever a file is created or significantly modified using AI, you **MUST** update `AI-Usage.md`.
- Every file must be listed only once.
- Include file path, interaction type, and the agent/model name.

---

## 2. Go Coding Style

### 2.1 Formatting & Tools
- Use standard `gofmt` for formatting.
- Use **Tabs** for indentation (Go standard).
- Follow modern Go idioms (Go 1.25+):
    - Use `any` instead of `interface{}`.
    - Use `errors.Is(err, target)` instead of `err == target`.
    - Use `slices` and `maps` packages for common operations.
    - Use `for i := range n` for simple loops.

### 2.2 Naming Conventions
- **Exported symbols:** `PascalCase`.
- **Internal symbols:** `camelCase`.
- **Database functions:** Prefer `Find[Entity]By[Field]`, `Create[Entity]`, `Update[Entity]`.

### 2.3 Error Handling
- Return errors as the last return value.
- Use `fmt.Errorf` with `%w` for error wrapping where appropriate.
- Log errors using `Klog` or `LogError`.

### 2.4 Database & Models
- **Soft Deletion:** KOsync uses soft deletion for documents.
    - Use the `deleted_at` field (Unix timestamp in milliseconds).
    - Always filter `SELECT` queries with `deleted_at IS NULL`.
- **Migrations:**
    - Stored in `internal/kosync/migrations/sql/`.
    - Use sequential numbering for new `.sql` files.

### 2.5 Testing
- Use the standard `testing` package **ONLY**.
- **Prohibited:** External assertion libraries (e.g., `testify`) or mock frameworks.
- Every `file.go` must have a corresponding `file_test.go`.
- **Database Tests:**
    - Do not use mocks.
    - Use `NewTemporaryDatabase(true)` for a fresh in-memory SQLite instance.
    - Always `defer db.Close()`.

### 2.6 Logging
- Use the `Klog` wrapper from `internal/kosync/log.go`.
- Tags should be descriptive (e.g., `db/document`, `api/webui`).
- Levels: `Debug`, `Info`, `Error`.

---

## 3. TypeScript & Vue Coding Style

### 3.1 Technology Stack
- **Framework:** Vue 3 (Composition API with `<script setup lang="ts">`).
- **State Management:** Pinia.
- **UI Components:** PrimeVue & PrimeIcons.
- **Styling:** Tailwind CSS.

### 3.2 Formatting
- **Indentation:** 2 spaces.
- **Naming:**
    - Variables & Functions: `camelCase`.
    - Components: `PascalCase` (e.g., `DocumentsList.vue`).
    - Stores: `use[Name]Store` (e.g., `useUserStore`).

### 3.3 Components
- Always use `<script setup lang="ts">`.
- Use PrimeVue components where possible to maintain UI consistency.
- Icons must use `primeicons` classes.

### 3.4 API Interaction
- Use `fetchApi` from `webui/src/api.ts`.
- It automatically handles authentication headers.
- Always handle potential `null` data or error responses.

### 3.5 State Management
- Logic related to shared state should reside in Pinia stores (`webui/src/stores/`).
- Use `Ref<T>` for reactive state where necessary.

---

## 4. Documentation

- **KDoc / Comments:** Follow the existing style. Explain complex logic, but avoid over-commenting obvious code.
- **Markdown:** Use clear headings and lists for documentation files.
