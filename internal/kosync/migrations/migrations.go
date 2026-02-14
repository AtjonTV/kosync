//
// File:        internal/kosync/migrations/migrations.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"embed"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

//go:embed sql/*
var migrationsFs embed.FS

type Migration struct {
	Version int
	Title   string
	Path    string
}

func LoadMigrations() (migrations *[]Migration, newest int, err error) {
	sqlDir := "sql"
	dir, err := migrationsFs.ReadDir(sqlDir)
	if err != nil {
		return
	}

	tmpMig := make([]Migration, 0)
	migrations = &tmpMig
	for i := range dir {
		ent := dir[i]
		if ent.Type().IsRegular() {
			name := ent.Name()
			nameParts := strings.Split(name, "-")
			if len(nameParts) != 2 {
				continue
			}
			var parsedInt int64
			parsedInt, err = strconv.ParseInt(nameParts[0], 10, 0)
			if err != nil {
				return
			}
			version := int(parsedInt)
			if version > newest {
				newest = version
			}

			newMig := Migration{
				Version: version,
				Title:   strings.ReplaceAll(strings.ReplaceAll(nameParts[1], ".sql", ""), "_", " "),
				Path:    filepath.Join(sqlDir, ent.Name()),
			}
			*migrations = append(*migrations, newMig)
		}
	}

	slices.SortFunc(*migrations, func(a, b Migration) int {
		return a.Compare(&b)
	})

	return
}

func (m *Migration) ReadMigration() (string, error) {
	data, err := migrationsFs.ReadFile(m.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (m *Migration) Compare(b *Migration) int {
	return m.Version - b.Version
}
