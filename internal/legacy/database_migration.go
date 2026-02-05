//
// File:        internal/legacy/database_migration.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package legacy

import (
	"fmt"
)

const (
	SchemaVersion = 6
)

func (db *LegacyDb) MigrateSchema() error {
	db.PrintDebug("DB", "-", "Checking for Database schema migrations.")

	migrations := map[int]interface{}{
		1: func() {
			// Add history to users
			for id, user := range db.Db.Users {
				db.Db.Users[id] = UserData{
					Username:  user.Username,
					Password:  user.Password,
					Documents: user.Documents,
					History:   make(map[string]HistoryData),
				}
			}
		},
		2: func() {
			// Default backup encoding to msgpack
			db.Db.Config.BackupEncodingType = BackupEncodingTypeMsgpack
		},
		3: func() {
			// Disable backup on startup
			db.Db.Config.BackupOnStartup = false
		},
		4: func() {
			// Add document id to documents
			for userId, user := range db.Db.Users {
				for docId, doc := range user.Documents {
					db.Db.Users[userId].Documents[docId] = FileData{
						DocumentId:   docId,
						ProgressData: doc.ProgressData,
						Timestamp:    doc.Timestamp,
					}
				}
			}
		},
		5: func() {
			// Disable webui
			db.Db.Config.WebUi = false
		},
		6: func() {
			// Set an empty pretty name to documents (because string can't be nil)
			for userId, user := range db.Db.Users {
				for docId, doc := range user.Documents {
					db.Db.Users[userId].Documents[docId] = FileData{
						DocumentId:   docId,
						ProgressData: doc.ProgressData,
						Timestamp:    doc.Timestamp,
						PrettyName:   "",
					}
				}
			}
		},
	}

	if db.Db.Schema < SchemaVersion {
		db.PrintDebug("DB", "-", "Migrations are available, performing backup.")
		if err := db.BackupDatabase(); err != nil {
			return err
		}
	} else {
		db.PrintDebug("DB", "-", "No Migrations to do.")
		return nil
	}

	for ver, migrate := range migrations {
		if db.Db.Schema < ver {
			db.PrintDebug("DB", "-", fmt.Sprintf("Migrating Schema from %d to %d", db.Db.Schema, ver))
			migrate.(func())()
			db.Db.Schema = ver
		}
	}

	return nil
}
