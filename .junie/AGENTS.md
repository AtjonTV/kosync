# Junie Guidelines for KOsync

This document outlines the project-specific guidelines, conventions, and architectural details to ensure consistent and high-quality contributions to the KOsync project.

## Project Overview

KOsync is a synchronization server for KOReader, written in Go. It provides a backend for syncing reading progress and a WebUI for managing documents.

## Coding Style & Conventions

### File Headers
Every source file (Go, TypeScript, etc.) MUST include the following header:
```go
//
// File:        path/to/file.ext
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//
```
*Note: The year range and copyright holder may vary based on the original author, but the format must remain consistent.*

### Naming Conventions
- Follow standard Go naming conventions (PascalCase for exported, camelCase for internal).
- Database related functions often use `Find[Entity]By[Field]`, `Create[Entity]`, `Update[Entity]`, etc.

### Soft Deletion
The project uses soft deletion for documents. Instead of removing records, the `deleted_at` field (a Unix timestamp in milliseconds) is set. Always ensure that `SELECT` queries filter for `deleted_at IS NULL`.

## Testing

### General Rules
- Standard library `testing` package ONLY.
- **External testing frameworks or assertion libraries (e.g., `testify`) are FORBIDDEN.**
- Every `source.go` file must have a corresponding `source_test.go` file.

### Database Tests
- Do NOT use mocks for database tests.
- Use `NewTemporaryDatabase(true)` to create a fresh, in-memory SQLite instance.
- Ensure `db.Close()` is called (typically via `defer`).

### Run Tests
```bash
go test ./...
```

## Database Management

### SQLite
KOsync uses SQLite. The production database is typically `kosync.db`.

### Migrations
- Migrations are stored as SQL files in `internal/kosync/migrations/sql/`.
- They are managed by a custom migration logic in `internal/kosync/database_migration.go`.
- The `schema_versions` table tracks applied migrations.
- When adding a new migration, create a new `.sql` file with the next sequential number.

## Backend Architecture

### Web Framework
- Uses [Fiber v3](https://github.com/gofiber/fiber).
- Routes are registered in `internal/kosync/kosync.go`.

### Authentication & Middleware
- Authentication is handled via `NewAuthMiddleware` in `internal/kosync/middleware.go`.
- It supports Bearer Tokens (JWT) and custom headers (`x-auth-user`, `x-auth-key`).
- **CRITICAL:** When adding new API routes, always verify if they should be protected by adding them to the `enableUrl` list in the middleware.

### Logging
- Use the `Klog` wrapper defined in `internal/kosync/log.go`.
- Tags should be descriptive (e.g., `api/webui`, `db/document`).
- Log levels: `Debug`, `Info`, `Error`.

## WebUI

### Technology Stack
- Vue 3 with TypeScript.
- Pinia for state management.
- PrimeVue for UI components.
- PrimeIcons for icons.
- Tailwind CSS for styling.

### API Consumption
- Use `fetchApi` from `webui/src/api.ts`. It automatically handles the Authorization header.

### Icons
- Always use `primeicons`. Ensure they are imported in `webui/src/main.ts` as `import 'primeicons/primeicons.css'`.

## Documentation & AI Usage

### AI-Usage.md
- Whenever you (the AI Agent) create or significantly modify a file, you MUST update the table in `AI-Usage.md`.
- Every file MUST only be listed once.
- Include the file path, interaction type, and the agent plus model name.

### KDoc / Comments
- Follow the existing comment style. Do not over-comment if the surrounding code is sparse, but ensure complex logic is explained.

## Common Pitfalls
- **Nil Context Checks:** When accessing user data from `fiber.Ctx.Locals`, always check if it's nil before type assertion to avoid panics.
- **MD5 Passwords:** KOReader plugins often use MD5 for passwords. Ensure compatibility where required.
