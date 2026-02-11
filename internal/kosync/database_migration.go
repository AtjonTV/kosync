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

const SchemaVersion = 102

var logDbMigrate = NewKlog("db/migrate")

func (db *Database) checkAndRunMigrations(config *Config) error {
	logDbMigrate.Debug("Checking and running database migrations")
	currentVersion, err := db.getCurrentSchemaVersion()
	if err != nil {
		return err
	}

	logDbMigrate.Debug("Current database schema version: %d", currentVersion)
	if currentVersion < SchemaVersion {
		logDbMigrate.Debug("Creating backup of database before migrations")
		err := BackupDatabase(config, db.rawDb)
		if err != nil {
			logDbMigrate.Error("Failed to backup database: %v", err.Error())
			return err
		}
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

	// Convert last_read_at to sub-second precision
	if targetVersion == 101 {
		logDbMigrate.Debug("Migrating to version 101")
		var changeReadTimestampToSubSeconds = `
			UPDATE documents SET last_read_at = (last_read_at * 10000) WHERE true;
			UPDATE document_history SET last_read_at = (last_read_at * 10000) WHERE true;
		`
		_, err := db.rawDb.Exec(changeReadTimestampToSubSeconds)
		if err != nil {
			logDbMigrate.Error("Failed to update last_read_at: %v", err.Error())
			return err
		}

		if _, err := db.rawDb.Exec(insertSchemaVersion, 101, time.Now().Unix()); err != nil {
			logDbMigrate.Error("Failed to insert schema version: %v", err.Error())
			return err
		}
	}

	if targetVersion == 102 {
		logDbMigrate.Debug("Migrating to version 102")
		var changeHistoryPrimaryToCreatedAt = `
			CREATE TABLE document_history_old AS SELECT * FROM document_history;

			DROP TABLE document_history;
            CREATE TABLE document_history (
                document_id TEXT NOT NULL,
                owner_id TEXT NOT NULL,

                title TEXT,
                current_location TEXT,
                progress FLOAT,
                last_read_on_device TEXT,
                last_read_on_device_id TEXT,
                last_read_at INTEGER,

                created_at INTEGER,
                updated_at INTEGER,
                deleted_at INTEGER
            );

			INSERT INTO document_history (document_id, owner_id, title, current_location, progress, last_read_on_device, last_read_on_device_id, last_read_at, created_at, updated_at, deleted_at)
			SELECT id as document_id, owner_id, title, current_location, progress, last_read_on_device, last_read_on_device_id, last_read_at, created_at, updated_at, deleted_at  FROM document_history_old;

			DROP TABLE document_history_old;
		`
		_, err := db.rawDb.Exec(changeHistoryPrimaryToCreatedAt)
		if err != nil {
			logDbMigrate.Error("Failed to update document_history primary key: %v", err.Error())
			return err
		}

		if _, err := db.rawDb.Exec(insertSchemaVersion, 102, time.Now().Unix()); err != nil {
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
