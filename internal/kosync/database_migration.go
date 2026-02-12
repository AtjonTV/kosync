//
// File:        internal/kosync/database_migration.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"database/sql"
	"strings"
	"time"

	"git.obth.eu/atjontv/kosync/internal/kosync/migrations"
)

var logDbMigrate = NewKlog("db/migrate")

func (db *Database) checkAndRunMigrations(config *Config) error {
	logDbMigrate.Debug("Checking and running database migrations")
	currentVersion, err := db.getCurrentSchemaVersion()
	if err != nil {
		return err
	}

	logDbMigrate.Debug("Current database schema version: %d", currentVersion)

	migs, newestVersion, err := migrations.LoadMigrations()
	if err != nil {
		return err
	}
	logDbMigrate.Debug("Latest database schema version: %d", newestVersion)

	db.currentSchema = currentVersion
	if currentVersion < newestVersion {
		if currentVersion > 0 { // no need to backup an empty database, so we skip == 0
			logDbMigrate.Debug("Creating backup of database before applying migrations")
			err := BackupDatabase(config, db.rawDb)
			if err != nil {
				logDbMigrate.Error("Failed to backup database: %v", err.Error())
				return err
			}
		}
		logDbMigrate.Debug("Running database migrations")

		for ver := currentVersion + 1; ver <= newestVersion; ver++ {
			if err := db.migrateTo(migs, ver); err != nil {
				return err
			}
		}

		logDbMigrate.Debug("All migrations applied")
	} else {
		logDbMigrate.Debug("No migrations to apply")
	}

	return nil
}

func (db *Database) migrateTo(migs *[]migrations.Migration, targetVersion int) error {
	var insertSchemaVersion = `INSERT INTO schema_versions (version, installed_at) VALUES (?, ?)`

	for i := range *migs {
		if (*migs)[i].Version == targetVersion {
			logDbMigrate.Debug("Migrating to version %d", targetVersion)
			mig := (*migs)[i]

			migFile, err := mig.ReadMigration()
			if err != nil {
				logDbMigrate.Error("Failed to read migration %d from file %s", targetVersion, mig.Path)
			}

			if _, err := db.rawDb.Exec(migFile); err != nil {
				logDbMigrate.Error("Failed to run migration %d: %v", targetVersion, err.Error())
				return err
			}

			if _, err := db.rawDb.Exec(insertSchemaVersion, targetVersion, time.Now().Unix()); err != nil {
				logDbMigrate.Error("Failed to insert schema version: %v", err.Error())
				return err
			}
			logDbMigrate.Debug("Successfully applied migration %d", targetVersion)
			db.currentSchema = targetVersion
		}
	}

	return nil
}

func (db *Database) getCurrentSchemaVersion() (vers int, err error) {
	versionsRes, err := db.rawDb.Query("SELECT version FROM schema_versions ORDER BY version DESC LIMIT 1;")
	if err != nil && !strings.Contains(err.Error(), "no such table: schema_versions") {
		logDbMigrate.Error("Failed to check database schema version: %v", err.Error())
		return
	} else if err != nil {
		return 0, nil
	}
	defer func(versionsRes *sql.Rows) {
		if versionsRes != nil {
			_ = versionsRes.Close()
		}
	}(versionsRes)
	if versionsRes != nil && versionsRes.Next() {
		err = versionsRes.Scan(&vers)
		if err != nil {
			logDbMigrate.Error("Failed to scan database schema version: %v", err.Error())
			return
		}
	}
	return
}
