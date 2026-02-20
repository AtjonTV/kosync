//
// File:        pkg/migrate/migrate_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrate_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"git.obth.eu/atjontv/kosync/pkg/migrate"
)

func getMockFs() fs.FS {
	getFS := func() fs.FS {
		return fstest.MapFS{
			"sql_test/001-test.sql": &fstest.MapFile{
				Data: []byte("-- test 1"),
			},
			"sql_test/002-test.sql": &fstest.MapFile{
				Data: []byte("-- test 2"),
			},
		}
	}
	return getFS()
}

func TestLoadMigrations(t *testing.T) {
	testFs := getMockFs()
	migs, newest, err := migrate.LoadFromFs(&testFs, "sql_test")
	if err != nil {
		t.Fatalf("Could not load migrations: %v", err.Error())
	}

	expected := 2
	if newest < expected {
		t.Errorf("Latest found version is %d, but it must be > %d", newest, expected)
	}

	expected = 1
	if migs == nil || len(*migs) < expected {
		t.Errorf("Should have found at least %d migrations, but found none", expected)
	}
}

func TestMigration_ReadMigration(t *testing.T) {
	testFs := getMockFs()
	migs, _, err := migrate.LoadFromFs(&testFs, "sql_test")
	if err != nil {
		t.Fatalf("Could not load migrations: %v", err.Error())
	}

	if migs == nil || len(*migs) < 1 {
		t.Fatalf("Did not find at least one migration, cant test if they can be read")
	}

	mig, err := (*migs)[0].ReadMigration()
	if err != nil {
		t.Errorf("Failed to read migration '%s': %v", (*migs)[0].Path, err.Error())
	}

	if len(mig) < 1 {
		t.Errorf("Read 0 bytes from migration '%s'", (*migs)[0].Path)
	}
}
