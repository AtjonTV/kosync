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
	// Try to get a mutex lock, so that two gorountines cant write at the same time
	app.DbMutex.Lock()
	defer app.DbMutex.Unlock()

	// marshal to json
	data, err := json.MarshalIndent(app.Db, "", "  ")
	if err != nil {
		app.DebugPrint(fmt.Sprintf("Failed to marshel the Database into JSON: %e", err))
		return err
	}
	// write to disk
	err = os.WriteFile(app.DatabaseFile, data, 0644)
	if err != nil {
		app.DebugPrint(fmt.Sprintf("Failed to save the Database to disk: %e", err))
		return err
	}
	return nil
}
