//
// File:        internal/kosync/database.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Database struct {
	rawDb         *sql.DB
	currentSchema int
}

func NewDatabase(config *Config) (*Database, error) {
	log := NewKlog("database")
	log.Debug("Trying to open SQLite database at '%s'", config.DatabaseFile)
	rawDb, err := sql.Open("sqlite", config.DatabaseFile)
	if err != nil {
		log.Error("Failed to open SQLite database: %v", err.Error())
		return nil, err
	}

	db := &Database{
		rawDb:         rawDb,
		currentSchema: 0,
	}
	if err := db.checkAndRunMigrations(config); err != nil {
		log.Error("Failed to open SQLite database: %v", err.Error())
		return nil, err
	}

	return db, nil
}

func NewDatabaseWithoutMigrate(config *Config) (*Database, error) {
	log := NewKlog("database")
	log.Debug("Trying to open SQLite database at '%s'", config.DatabaseFile)
	rawDb, err := sql.Open("sqlite", config.DatabaseFile)
	if err != nil {
		log.Error("Failed to open SQLite database: %v", err.Error())
		return nil, err
	}

	db := &Database{
		rawDb:         rawDb,
		currentSchema: 0,
	}
	return db, nil
}

func (db *Database) Close() error {
	return db.rawDb.Close()
}

func (db *Database) SchemaVersion() int {
	return db.currentSchema
}
