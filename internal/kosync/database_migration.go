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

func (db *Database) checkAndRunMigrations() error {
	versionsRes, err := db.rawDb.Query("SELECT version FROM schema_versions ORDER BY version DESC LIMIT 1;")
	if err != nil && !strings.Contains(err.Error(), "no such table: schema_versions") {
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
			return err
		}
	} else {
		currentVersion = 0
	}
	if currentVersion < SchemaVersion {
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
	var insertSchemaVersion = `INSERT INTO schema_versions (version, installed_at) VALUES (?, ?)`

	if targetVersion == 100 {
		var createSchemaVersionTable = `
            CREATE TABLE IF NOT EXISTS schema_versions (
                version INTEGER PRIMARY KEY,
                installed_at INTEGER NOT NULL
            );
        `
		if _, err := db.rawDb.Exec(createSchemaVersionTable); err != nil {
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
			return err
		}

		if _, err := db.rawDb.Exec(insertSchemaVersion, 100, time.Now().Unix()); err != nil {
			return err
		}
	}

	db.currentSchema = targetVersion
	return nil
}
