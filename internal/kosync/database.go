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

const DatabaseFileName = "./kosync.db"
const SchemaVersion = 100

type Database struct {
	rawDb         *sql.DB
	currentSchema int
}

func NewDatabase(config *Config) (*Database, error) {
	rawDb, err := sql.Open("sqlite", config.DatabaseFile)
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
