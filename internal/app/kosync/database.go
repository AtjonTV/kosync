//
// File:        internal/app/kosync/database.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"
)

func LoadOrInitDatabase() (string, Database, error) {
	searchPaths := []string{
		"/data/database.json",
		"database.json",
	}

	var db Database
	foundDbFile := searchPaths[0] // Default to /data

	// Handle reading database
	createEmptyDatabase := true
	for _, path := range searchPaths {
		stat, _ := os.Stat(path)
		if stat != nil && stat.Size() > 0 {
			data, err := os.ReadFile(path)
			if err != nil {
				return "", Database{}, err
			}

			if len(data) > 1 {
				err = json.Unmarshal(data, &db)
				if err != nil {
					return "", Database{}, err
				}

				foundDbFile = path
				createEmptyDatabase = false
				break
			}
		}
	}

	// Fallback to empty
	if createEmptyDatabase {
		db = Database{
			Config: ConfigData{
				ListenAddress:       ":8080",
				DisableRegistration: false,
				DebugLog:            false,
				StoreHistory:        false,
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

func (app *Kosync) BackupDatabase() error {
	if err := app.PersistDatabase(); err != nil {
		return err
	}

	binaryData, err := json.Marshal(app.Db)
	if err != nil {
		return err
	}

	app.DbMutex.Lock()
	defer app.DbMutex.Unlock()

	now := time.Now()
	block := pem.Block{
		Type: "KOSYNC BACKUP",
		Headers: map[string]string{
			"App":          "https://git.obth.eu/atjontv/kosync",
			"Content-Type": "application/json",
			"Created-At":   now.Format(time.RFC3339),
			"Schema":       fmt.Sprintf("%d", app.Db.Schema),
		},
		Bytes: binaryData,
	}

	backupFileName := fmt.Sprintf("%s_%s-%s.bak",
		strings.Replace(app.DatabaseFile, ".json", "", 1),
		now.Format(time.DateOnly),
		now.Format(time.TimeOnly),
	)
	backupFile, err := os.OpenFile(backupFileName, os.O_CREATE+os.O_RDWR, fs.FileMode(0644))
	defer func(backupFile *os.File) {
		err := backupFile.Close()
		if err != nil {
			app.DebugPrint(fmt.Sprintf("[Backup]: Failed to close backup file '%s': %v", backupFileName, err))
		}
	}(backupFile)
	if err != nil {
		return err
	}
	// Encode and write to file
	err = pem.Encode(backupFile, &block)
	if err != nil {
		return err
	}
	app.DebugPrint(fmt.Sprintf("[Backup]: Created backup file '%s'", backupFileName))
	return nil
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
