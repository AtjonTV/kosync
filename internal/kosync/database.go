package kosync

import (
	"database/sql"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const SchemaVersion = 100

type Database struct {
	rawDb         *sql.DB
	currentSchema int
}

func NewDatabase() (*Database, error) {
	rawDb, err := sql.Open("sqlite", GetEnv("DATABASE_FILE", "./kosync.db"))
	if err != nil {
		return nil, err
	}

	db := &Database{
		rawDb:         rawDb,
		currentSchema: 0,
	}
	if err := db.checkAndRunMigrations(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *Database) Close() error {
	return db.rawDb.Close()
}

func (db *Database) checkAndRunMigrations() error {
	versionsRes, err := db.rawDb.Query("SELECT version FROM schema_versions ORDER BY version DESC LIMIT 1;")
	if err != nil && !strings.Contains(err.Error(), "no such table: schema_versions") {
		return err
	}
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

func (db *Database) migrateTo(version int) error {
	if version == 100 {
		if _, err := db.rawDb.Exec(`
            CREATE TABLE IF NOT EXISTS schema_versions (
                version INTEGER PRIMARY KEY,
                installed_at INTEGER NOT NULL
            );
        `); err != nil {
			return err
		}
		if _, err := db.rawDb.Exec(`
            CREATE TABLE IF NOT EXISTS users (
                id TEXT PRIMARY KEY,
                username TEXT UNIQE,
                password TEXT,
                created_at INTEGER NOT NULL,
                updated_at INTEGER,
                deleted_at INTEGER
            );
        `); err != nil {
			return err
		}
		if _, err := db.rawDb.Exec(`
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
        `); err != nil {
			return err
		}
		if _, err := db.rawDb.Exec(`
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
        `); err != nil {
			return err
		}
		if _, err := db.rawDb.Exec(`
            INSERT INTO schema_versions (version, installed_at) VALUES (100, ?);
        `, time.Now().Unix()); err != nil {
			return err
		}
	}

	return nil
}
