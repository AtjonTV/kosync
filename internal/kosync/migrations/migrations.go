//
// File:        internal/kosync/migrations/migrations.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"embed"
	"io/fs"
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

	migrations = new([]Migration)
	err = fs.WalkDir(migrationsFs, sqlDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		fileName := filepath.Base(path)
		nameParts := strings.Split(fileName, "-")
		if len(nameParts) != 2 {
			return nil
		}

		var parsedInt int64
		parsedInt, err = strconv.ParseInt(nameParts[0], 10, 0)
		if err != nil {
			return err
		}
		version := int(parsedInt)
		if version > newest {
			newest = version
		}

		newMig := Migration{
			Version: version,
			Title:   strings.ReplaceAll(strings.ReplaceAll(nameParts[1], ".sql", ""), "_", " "),
			Path:    path,
		}
		*migrations = append(*migrations, newMig)
		return nil
	})
	if err != nil {
		return
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
