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

var logDbMigrate = NewKlog("db/migrate")

func (db *Database) checkAndRunMigrations() error {
	logDbMigrate.Debug("Checking and running database migrations")
	versionsRes, err := db.rawDb.Query("SELECT version FROM schema_versions ORDER BY version DESC LIMIT 1;")
	if err != nil && !strings.Contains(err.Error(), "no such table: schema_versions") {
		logDbMigrate.Error("Failed to check database schema version: %v", err.Error())
		return err
	}
	defer func(versionsRes *sql.Rows) {
		if versionsRes != nil {
			_ = versionsRes.Close()
		}
	}(versionsRes)
	var currentVersion int
	if versionsRes != nil && versionsRes.Next() {
		if err := versionsRes.Scan(&currentVersion); err != nil {
			logDbMigrate.Error("Failed to scan database schema version: %v", err.Error())
			return err
		}
	} else {
		currentVersion = 0
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

	db.currentSchema = targetVersion
	return nil
}
