# Testing in kosync

This document outlines the testing conventions and requirements for the kosync project.

## General Conventions

This is a Go project that adheres to standard Go testing conventions. Every Go source file should have a corresponding test file to ensure maintainability and code quality.

- For each `source.go` file, a `source_test.go` file must exist in the same directory.
- Tests must be written using Go's built-in `testing` package.

## Dependencies

To keep the project lightweight and maintainable:
- External testing frameworks or assertion libraries (such as `testify`) are **forbidden**.
- Stick to standard library features for assertions and test logic.

## Database Tests

Tests that require a database connection should not use mocks. Instead, they should utilize an in-memory database instance to ensure tests are fast, isolated, and realistic.

- Use `NewTemporaryDatabase(true)` to obtain a fresh, in-memory database instance for your tests.
- This approach removes the need for complex database mocking and ensures that the database logic is actually exercised during tests.

## Coding Style

New code, including test files, must follow the existing project coding style. 

### File Header

Every new file must include a header containing the file path, project link, copyright, and license notice. Refer to existing files for the exact format:

```go
//
// File:        path/to/file.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 >Your legal Name<. Licensed under the EUPL-1.2 or later
//
```

## Running Tests

You can run all tests in the project using the standard Go command:

```bash
go test ./...
```

### Test Coverage

To collect test coverage data, run:

```bash
go test -coverprofile=coverage.out ./...
```

To view the coverage report in your browser, run:

```bash
go tool cover -html=coverage.out
```

To view the coverage report in your terminal, run:

```bash
go tool cover -func=coverage.out
```
