//
// File:        internal/kosync/database_migration.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import "fmt"

const (
	SchemaVersion = 4
)

func (app *Kosync) MigrateSchema() error {
	app.PrintDebug("DB", "-", "Checking for Database schema migrations.")

	migrations := map[int]interface{}{
		1: func() {
			for id, user := range app.Db.Users {
				app.Db.Users[id] = UserData{
					Username:  user.Username,
					Password:  user.Password,
					Documents: user.Documents,
					History:   make(map[string]HistoryData),
				}
			}
		},
		2: func() {
			app.Db.Config.BackupEncodingType = BackupEncodingTypeMsgpack
		},
		3: func() {
			app.Db.Config.BackupOnStartup = false
		},
		4: func() {
			// Add document id to documents
			for userId, user := range app.Db.Users {
				for docId, doc := range user.Documents {
					app.Db.Users[userId].Documents[docId] = FileData{
						DocumentId:   docId,
						ProgressData: doc.ProgressData,
						Timestamp:    doc.Timestamp,
					}
				}
			}
		},
	}

	if app.Db.Schema < SchemaVersion {
		app.PrintDebug("DB", "-", "Migrations are available, performing backup.")
		if err := app.BackupDatabase(); err != nil {
			return err
		}
	} else {
		app.PrintDebug("DB", "-", "No Migrations to do.")
		return nil
	}

	for ver, migrate := range migrations {
		if app.Db.Schema < ver {
			app.PrintDebug("DB", "-", fmt.Sprintf("Migrating Schema from %d to %d", app.Db.Schema, ver))
			migrate.(func())()
			app.Db.Schema = ver
		}
	}

	return nil
}
