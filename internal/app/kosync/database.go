//
// File:        internal/app/kosync/database.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func FindDatabaseFile() (string, error) {
	searchPaths := []string{
		"/data/database.json",
		"database.json",
	}

	foundDbFile := searchPaths[0] // Default to /data

	for _, path := range searchPaths {
		stat, _ := os.Stat(path)
		if stat != nil && stat.Size() > 0 {
			return path, nil
		}
	}
	return foundDbFile, nil
}

func LoadOrInitDatabase() (string, Database, error) {
	var db Database

	foundDbFile, err := FindDatabaseFile()
	if err != nil {
		return "", Database{}, err
	}

	// Handle reading database
	createEmptyDatabase := true
	data, err := os.ReadFile(foundDbFile)
	if err != nil {
		return "", Database{}, err
	}

	if len(data) > 1 {
		err = json.Unmarshal(data, &db)
		if err != nil {
			return "", Database{}, err
		}

		createEmptyDatabase = false
	}

	// Fallback to empty
	if createEmptyDatabase {
		db = Database{
			Config: ConfigData{
				ListenAddress:       ":8080",
				DisableRegistration: false,
				DebugLog:            false,
				StoreHistory:        false,
				BackupEncodingType:  "msgpack",
			},
			Users: make(map[string]UserData),
		}
	}

	// Enforce required defaults
	if len(db.Config.ListenAddress) == 0 {
		db.Config.ListenAddress = ":8080"
	}

	return foundDbFile, db, nil
}

func (app *Kosync) PersistDatabase() error {
	// marshal to json
	data, err := json.MarshalIndent(app.Db, "", "  ")
	if err != nil {
		app.DebugPrint("DB", "-", fmt.Sprintf("Failed to marshel the Database into JSON: %e", err))
		return err
	}
	// write to disk
	err = os.WriteFile(app.DbFile, data, 0644)
	if err != nil {
		app.DebugPrint("DB", "-", fmt.Sprintf("Failed to save the Database to disk: %e", err))
		return err
	}
	app.DebugPrint("DB", "-", fmt.Sprintf("Wrote %d bytes to disk", len(data)))
	return nil
}

func (app *Kosync) AddUser(username, password string) error {
	app.DbLock.Lock()
	defer app.DbLock.Unlock()

	_, found := app.Db.Users[username]
	if found {
		return fmt.Errorf("username is already taken")
	}

	// Create user
	app.Db.Users[username] = UserData{
		Username:  username,
		Password:  password,
		Documents: make(map[string]FileData),
		History:   make(map[string]HistoryData),
	}

	// Persist new user
	return app.PersistDatabase()
}

func (app *Kosync) AddOrUpdateDocument(username string, document DocumentData) error {
	app.DbLock.Lock()
	defer app.DbLock.Unlock()

	if app.Db.Config.StoreHistory {
		var currentVersion = app.Db.Users[username].Documents[document.Document]
		var previousData = app.Db.Users[username].History[document.Document].DocumentHistory
		app.Db.Users[username].History[document.Document] = HistoryData{
			DocumentHistory: append(previousData, currentVersion),
		}
		app.DebugPrint("DB", "-", fmt.Sprintf("[user: %s]: Document '%s' progress went from %.2f %% to %.2f %%", username, document.Document, currentVersion.Percentage*100, document.Percentage*100))
	}

	// Create document state
	app.Db.Users[username].Documents[document.Document] = FileData{
		ProgressData: document.ProgressData,
		Timestamp:    time.Now().Unix(),
	}

	// Persist new user
	return app.PersistDatabase()
}
