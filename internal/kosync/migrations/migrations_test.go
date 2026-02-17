//
// File:        internal/kosync/migrations/migrations_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations_test

import (
	"testing"

	"git.obth.eu/atjontv/kosync/internal/kosync/migrations"
)

func TestLoadMigrations(t *testing.T) {
	migs, newest, err := migrations.LoadMigrations()
	if err != nil {
		t.Errorf("Could not load migrations: %v", err.Error())
	}

	expected := 100
	if newest < expected {
		t.Errorf("Latest found version is %d, but it must be > %d", newest, expected)
	}

	expected = 1
	if migs == nil || len(*migs) < expected {
		t.Errorf("Should have found at least %d migrations, but found none", expected)
	}
}

func TestMigration_ReadMigration(t *testing.T) {
	migs, _, err := migrations.LoadMigrations()
	if err != nil {
		t.Errorf("Could not load migrations: %v", err.Error())
	}

	if migs == nil {
		t.Errorf("Did not find at least one migration, cant test if they can be read")
		return
	}

	mig, err := (*migs)[0].ReadMigration()
	if err != nil {
		t.Errorf("Failed to read migration '%s': %v", (*migs)[0].Path, err.Error())
	}

	if len(mig) < 1 {
		t.Errorf("Read 0 bytes from migration '%s'", (*migs)[0].Path)
	}
}
