//
// File:        internal/kosync/migrations/migrations.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"embed"
	"io/fs"

	"git.obth.eu/atjontv/kosync/pkg/migrate"
)

//go:embed sql/*
var migrationsFs embed.FS

func LoadMigrations() (migrations *[]migrate.Migration, newest int, err error) {
	asFs := fs.FS(migrationsFs)
	m, n, e := migrate.LoadFromFs(&asFs, "sql")
	return m, n, e
}
