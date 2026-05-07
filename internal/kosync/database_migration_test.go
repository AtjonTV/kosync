//
// File:        internal/kosync/database_migration_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"testing"

	"git.obth.eu/atjontv/kosync/internal/kosync/migrations"
)

const migrationErrFormat = "Migrations failed. Expected version %d, got %d."

func TestMigrationsApplied(t *testing.T) {
	migs, i, _ := migrations.LoadMigrations()

	{
		db, err := NewTemporaryDatabase(true)
		if err != nil {
			t.Fatalf("Failed to create temporary database: %v", err)
		}

		if db.SchemaVersion() != i {
			t.Fatalf(migrationErrFormat, i, db.SchemaVersion())
		}
	}

	{
		db, err := NewTemporaryDatabase(false)
		if err != nil {
			t.Fatalf("Failed to create temporary database: %v", err)
		}
		err = db.MigrateToTargetVersion(migs, (*migs)[0].Version)
		if err != nil {
			t.Fatalf("Failed to migrate database: %v", err)
		}

		if db.SchemaVersion() != (*migs)[0].Version {
			t.Fatalf(migrationErrFormat, (*migs)[0].Version, db.SchemaVersion())
		}

		err = db.MigrateToTargetVersion(migs, i)
		if err != nil {
			t.Fatalf("Failed to migrate database: %v", err)
		}

		if db.SchemaVersion() != i {
			t.Fatalf(migrationErrFormat, i, db.SchemaVersion())
		}
	}
}

func TestGetCurrentSchemaVersion_Error(t *testing.T) {
	db, _ := NewTemporaryDatabase(true)
	db.Close()
	_, err := db.getCurrentSchemaVersion()
	if err == nil {
		t.Error("Expected error when database is closed")
	}
}
