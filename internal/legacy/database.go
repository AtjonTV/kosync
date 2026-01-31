//
// File:        internal/legacy/database.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package legacy

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/log"
)

type LegacyDb struct {
	Db     Database
	DbLock sync.Mutex
	DbFile string
}

type LegacyConfig struct {
	RestoreFile *string
	MakeBackup  *bool
}

func New(cfg LegacyConfig) *LegacyDb {
	if cfg.RestoreFile != nil && len(*cfg.RestoreFile) > 0 {
		if err := RestoreDatabase(*cfg.RestoreFile); err != nil {
			panic(err)
		}
	}

	// Try to find the database or create a new one
	foundDbFile, db, err := LoadOrInitDatabase()
	if err != nil {
		panic(err)
	}

	ldb := LegacyDb{
		Db:     db,
		DbLock: sync.Mutex{},
		DbFile: foundDbFile,
	}

	if err := ldb.MigrateSchema(); err != nil {
		panic(err)
	}

	// Persist migrated database
	if err := ldb.PersistDatabase(); err != nil {
		panic(err)
	}

	if cfg.MakeBackup != nil && *cfg.MakeBackup {
		if err := ldb.BackupDatabase(); err != nil {
			ldb.PrintError("Backup", "-", fmt.Sprintf("Failed to create backup, continuing startup: %v", err))
		}
	}
	return &ldb
}

func (db *LegacyDb) CheckMigrationToSqlite() bool {
	return db.Db.Schema < 99
}

func (db *LegacyDb) PrintDebug(marker, requestId, s string) {
	// Only print debugs when enabled
	if db.Db.Config.DebugLog {
		log.Debugf("RequestId=%s, Module=%s: %s\n", requestId, marker, s)
	}
}
func (db *LegacyDb) PrintError(marker, requestId, s string) {
	log.Errorf("RequestId=%s, Module=%s: %s\n", requestId, marker, s)
}

func FindDatabaseFile() (bool, string, error) {
	searchPaths := []string{
		"/data/database.json",
		"database.json",
	}

	foundDbFile := searchPaths[1] // Default to ./database.json
	if _, err := os.ReadFile(searchPaths[0]); os.IsExist(err) {
		foundDbFile = searchPaths[0] // Default to /data/database.json when inside a docker container
	}

	for _, path := range searchPaths {
		stat, _ := os.Stat(path)
		if stat != nil && stat.Size() > 0 {
			return true, path, nil
		}
	}
	return false, foundDbFile, nil
}

func LoadOrInitDatabase() (string, Database, error) {
	var db Database

	found, foundDbFile, err := FindDatabaseFile()
	if err != nil {
		return "", Database{}, err
	}

	createEmptyDatabase := true
	if found {
		// Handle reading database
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
	} else {
		f, err := os.Create(foundDbFile)
		if err != nil {
			return "", Database{}, err
		}
		if err := f.Close(); err != nil {
			return "", Database{}, err
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

func (db *LegacyDb) PersistDatabase() error {
	// marshal to json
	data, err := json.MarshalIndent(db.Db, "", "  ")
	if err != nil {
		db.PrintDebug("DB", "-", fmt.Sprintf("Failed to marshel the Database into JSON: %e", err))
		return err
	}
	// write to disk
	err = os.WriteFile(db.DbFile, data, 0600)
	if err != nil {
		db.PrintDebug("DB", "-", fmt.Sprintf("Failed to save the Database to disk: %e", err))
		return err
	}
	db.PrintDebug("DB", "-", fmt.Sprintf("Wrote %d bytes to disk", len(data)))
	return nil
}

func (db *LegacyDb) FindUser(username string) (*UserData, bool) {
	user, found := db.Db.Users[username]
	if found {
		return &user, true
	}
	return nil, false
}

func (db *LegacyDb) GetUser(username string) *UserData {
	user, found := db.FindUser(username)
	if found {
		return user
	}
	return nil
}

func (db *LegacyDb) AddUser(username, password string) error {
	db.DbLock.Lock()
	defer db.DbLock.Unlock()

	_, found := db.Db.Users[username]
	if found {
		return fmt.Errorf("username is already taken")
	}

	// Create user
	db.Db.Users[username] = UserData{
		Username:  username,
		Password:  password,
		Documents: make(map[string]FileData),
		History:   make(map[string]HistoryData),
	}

	// Persist new user
	return db.PersistDatabase()
}

func (db *LegacyDb) AddOrUpdateDocument(username string, document DocumentData) error {
	db.DbLock.Lock()
	defer db.DbLock.Unlock()

	var currentVersion, hasCurrent = db.Db.Users[username].Documents[document.Document]
	if db.Db.Config.StoreHistory {
		var previousData = db.Db.Users[username].History[document.Document].DocumentHistory
		db.Db.Users[username].History[document.Document] = HistoryData{
			DocumentHistory: append(previousData, currentVersion),
		}
		db.PrintDebug("DB", "-", fmt.Sprintf("[user: %s]: Document '%s' progress went from %.2f %% to %.2f %%", username, document.Document, currentVersion.Percentage*100, document.Percentage*100))
	}

	// Special handling to keep pretty name persistent
	var prettyName = ""
	if hasCurrent {
		prettyName = currentVersion.PrettyName
	}

	// Create document state
	db.Db.Users[username].Documents[document.Document] = FileData{
		DocumentId:   document.Document,
		ProgressData: document.ProgressData,
		Timestamp:    time.Now().Unix(),
		PrettyName:   prettyName,
	}

	// Persist new user
	return db.PersistDatabase()
}

func (db *LegacyDb) UpdateDocumentPrettyName(userId, documentId, prettyName string) error {
	db.DbLock.Lock()
	defer db.DbLock.Unlock()

	origDoc := db.Db.Users[userId].Documents[documentId]
	db.Db.Users[userId].Documents[documentId] = FileData{
		ProgressData: origDoc.ProgressData,
		DocumentId:   origDoc.DocumentId,
		Timestamp:    origDoc.Timestamp,
		PrettyName:   prettyName,
	}

	return db.PersistDatabase()
}
