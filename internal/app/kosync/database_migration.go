//
// File:        internal/app/kosync/database_migration.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import "fmt"

const (
	SchemaVersion = 1
)

func (app *Kosync) MigrateSchema() error {
	app.DebugPrint("[DB] Checking for Database schema migrations.")

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
	}

	if app.Db.Schema < SchemaVersion {
		app.DebugPrint("[DB] Migrations are available, performing backup.")
		err := app.BackupDatabase()
		if err != nil {
			return err
		}
	} else {
		app.DebugPrint("[DB] No Migrations to do.")
		return nil
	}

	for ver, migrate := range migrations {
		if app.Db.Schema < ver {
			app.DebugPrint(fmt.Sprintf("[DB] Migrating Schema from %d to %d", app.Db.Schema, ver))
			migrate.(func())()
			app.Db.Schema = ver
		}
	}

	return nil
}
