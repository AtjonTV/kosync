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
)

const SchemaVersion = 101

var logDbMigrate = NewKlog("db/migrate")

func (db *Database) checkAndRunMigrations() error {
	logDbMigrate.Debug("Checking and running database migrations")
	currentVersion, err := db.getCurrentSchemaVersion()
	if err != nil {
		return err
	}

	logDbMigrate.Debug("Current database schema version: %d", currentVersion)
	if currentVersion < SchemaVersion {
		logDbMigrate.Debug("Running database migrations")
		db.currentSchema = currentVersion

		for ver := currentVersion + 1; ver <= SchemaVersion; ver++ {
			if err := db.migrateTo(ver); err != nil {
				return err
			}
		}
	}

	return nil
}

func (db *Database) migrateTo(targetVersion int) error {
	logDbMigrate.Debug("Migrating to target %d", targetVersion)
	var insertSchemaVersion = `INSERT INTO schema_versions (version, installed_at) VALUES (?, ?)`

	// Create schema
	if targetVersion == 100 {
		logDbMigrate.Debug("Migrating to version 100")
		var createSchemaVersionTable = `
            CREATE TABLE IF NOT EXISTS schema_versions (
                version INTEGER PRIMARY KEY,
                installed_at INTEGER NOT NULL
            );
        `
		if _, err := db.rawDb.Exec(createSchemaVersionTable); err != nil {
			logDbMigrate.Error("Failed to create schema_versions table: %v", err.Error())
			return err
		}

		var createUsersTable = `
            CREATE TABLE IF NOT EXISTS users (
                id TEXT PRIMARY KEY,
                username TEXT UNIQUE,
                password TEXT,
                created_at INTEGER NOT NULL,
                updated_at INTEGER,
                deleted_at INTEGER
            );
        `
		if _, err := db.rawDb.Exec(createUsersTable); err != nil {
			logDbMigrate.Error("Failed to create users table: %v", err.Error())
			return err
		}

		var createDocumentsTable = `
            CREATE TABLE IF NOT EXISTS documents (
                id TEXT NOT NULL,
                owner_id TEXT NOT NULL,
                
                title TEXT,
                current_location TEXT,
                progress FLOAT,
                last_read_on_device TEXT,
                last_read_on_device_id TEXT,
                last_read_at INTEGER,
                
                created_at INTEGER NOT NULL,
                updated_at INTEGER,
                deleted_at INTEGER,
                PRIMARY KEY (id, owner_id)
            );
        `
		if _, err := db.rawDb.Exec(createDocumentsTable); err != nil {
			logDbMigrate.Error("Failed to create documents table: %v", err.Error())
			return err
		}

		var createDocumentHistoryTable = `
            CREATE TABLE IF NOT EXISTS document_history (
                id TEXT NOT NULL,
                owner_id TEXT NOT NULL,
                last_read_at INTEGER NOT NULL,
                
                title TEXT,
                current_location TEXT,
                progress FLOAT,
                last_read_on_device TEXT,
                last_read_on_device_id TEXT,
                
                created_at INTEGER NOT NULL,
                updated_at INTEGER,
                deleted_at INTEGER,
                PRIMARY KEY (id, owner_id, last_read_at)
            );
        `
		if _, err := db.rawDb.Exec(createDocumentHistoryTable); err != nil {
			logDbMigrate.Error("Failed to create document_history table: %v", err.Error())
			return err
		}

		if _, err := db.rawDb.Exec(insertSchemaVersion, 100, time.Now().Unix()); err != nil {
			logDbMigrate.Error("Failed to insert schema version: %v", err.Error())
			return err
		}
	}

	// Convert last_read_at to float
	if targetVersion == 101 {
		logDbMigrate.Debug("Migrating to version 101")
		var migrateLastReadAtToFloat = `
			ALTER TABLE documents RENAME COLUMN last_read_at TO last_read_at_old;
			ALTER TABLE documents ADD COLUMN last_read_at FLOAT NOT NULL DEFAULT 0;
			UPDATE documents SET last_read_at = (last_read_at_old * 10000) WHERE true;
			ALTER TABLE documents DROP COLUMN last_read_at_old;
		`
		_, err := db.rawDb.Exec(migrateLastReadAtToFloat)
		if err != nil {
			logDbMigrate.Error("Failed to migrate last_read_at to float: %v", err.Error())
			return err
		}

		var rebuildDocumentHistory = `
			CREATE TABLE IF NOT EXISTS document_history_old AS SELECT * FROM document_history;
			DROP TABLE document_history;

            CREATE TABLE IF NOT EXISTS document_history (
                id TEXT NOT NULL,
                owner_id TEXT NOT NULL,
                last_read_at FLOAT NOT NULL,

                title TEXT,
                current_location TEXT,
                progress FLOAT,
                last_read_on_device TEXT,
                last_read_on_device_id TEXT,

                created_at INTEGER NOT NULL,
                updated_at INTEGER,
                deleted_at INTEGER,
                PRIMARY KEY (id, owner_id, last_read_at)
            );

			INSERT INTO document_history (id, owner_id, last_read_at, title, current_location, progress, last_read_on_device, last_read_on_device_id, created_at, updated_at, deleted_at)
			SELECT id, owner_id, (last_read_at * 10000), title, current_location, progress, last_read_on_device, last_read_on_device_id, created_at, unixepoch(), deleted_at
			FROM document_history_old;

			DROP TABLE document_history_old;
		`
		_, err = db.rawDb.Exec(rebuildDocumentHistory)
		if err != nil {
			logDbMigrate.Error("Failed to rebuild document_history: %v", err.Error())
			return err
		}

		if _, err := db.rawDb.Exec(insertSchemaVersion, 101, time.Now().Unix()); err != nil {
			logDbMigrate.Error("Failed to insert schema version: %v", err.Error())
			return err
		}
	}

	db.currentSchema = targetVersion
	return nil
}

func (db *Database) getCurrentSchemaVersion() (vers int, err error) {
	versionsRes, err := db.rawDb.Query("SELECT version FROM schema_versions ORDER BY version DESC LIMIT 1;")
	if err != nil && !strings.Contains(err.Error(), "no such table: schema_versions") {
		logDbMigrate.Error("Failed to check database schema version: %v", err.Error())
		return
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
