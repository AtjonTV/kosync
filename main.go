//
// File:        main.go
// Project:     https://git.obth.eu/atjontv/kosync
// License:     Copyright 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
// Description: KOsync is a simple KOReader progress sync server,
//				contained in a single Go file, it compiles to a single static binary.
// Build:       go build -tags netgo main.go && strip main
//

package main

import (
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Kosync struct {
	DatabaseFile string
	Db           Database
	DbMutex      sync.Mutex
}

type Database struct {
	Schema int                 `json:"schema"`
	Config ConfigData          `json:"config"`
	Users  map[string]UserData `json:"users"`
}

type ConfigData struct {
	ListenAddress       string `json:"listen_address"`
	DisableRegistration bool   `json:"disable_registration"`
	DebugLog            bool   `json:"enable_debug_log"`
	StoreHistory        bool   `json:"store_history"`
}

type UserData struct {
	Username  string                 `json:"username"`
	Password  string                 `json:"password"`
	Documents map[string]FileData    `json:"documents"`
	History   map[string]HistoryData `json:"history"`
}

type FileData struct {
	Progress   string  `json:"progress"`
	Percentage float32 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceId   string  `json:"device_id"`
	Timestamp  int64   `json:"timestamp"`
}

type HistoryData struct {
	DocumentHistory []FileData `json:"document_history"`
}

func (app *Kosync) MigrateSchema() error {
	app.DebugPrint("[DB] Checking for Database schema migrations.")
	latestVer := 1

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

	if app.Db.Schema < latestVer {
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
			"App":        "https://git.obth.eu/atjontv/kosync",
			"Created-At": now.Format(time.RFC3339),
			"Schema":     fmt.Sprintf("%d", app.Db.Schema),
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

func (app *Kosync) DoAuth(r *http.Request, w http.ResponseWriter) (bool, *UserData) {
	// get the headers
	username := r.Header.Get("x-auth-user")
	password := r.Header.Get("x-auth-key")

	// try to find the user
	user, found := app.Db.Users[username]
	if !found {
		return false, nil
	}

	// verify the passwords match (both are md5 hashed)
	if user.Password != password {
		w.WriteHeader(http.StatusUnauthorized)
		app.DebugPrint(fmt.Sprintf("[user: %s]: Unauthorized access for user with password '%s'", username, password))
		return false, nil
	}

	app.DebugPrint(fmt.Sprintf("[user: %s]: Authorized access for user", username))
	return true, &user
}

func (app *Kosync) DebugPrint(s string) {
	// Only print debugs when enabled
	if app.Db.Config.DebugLog {
		log.Println(s)
	}
}

type ProgressRequest struct {
	Document   string  `json:"document"`
	Progress   string  `json:"progress"`
	Percentage float32 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceId   string  `json:"device_id"`
}

type ProgressResponse struct {
	Document   string  `json:"document"`
	Progress   string  `json:"progress"`
	Percentage float32 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceId   string  `json:"device_id"`
	Timestamp  int64   `json:"timestamp"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (app *Kosync) HandleUsersCreate(w http.ResponseWriter, r *http.Request) {
	log.Println("Register request")

	// Return with forbidden when registration is disabled
	if app.Db.Config.DisableRegistration {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Parse payload
	var data RegisterRequest
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		app.DebugPrint(fmt.Sprintf("Error reading request body: %v", err))
		// return the same status as expected by the kosync plugin, which is 402 for some reason?
		w.WriteHeader(http.StatusPaymentRequired)
		return
	}

	// Create user
	app.Db.Users[data.Username] = UserData{
		Username:  data.Username,
		Password:  data.Password,
		Documents: make(map[string]FileData),
	}

	// Persist new user
	go func() {
		_ = app.PersistDatabase()
	}()

	w.WriteHeader(http.StatusCreated)
}

func (app *Kosync) HandleUsersAuth(w http.ResponseWriter, r *http.Request) {
	app.DebugPrint("Auth request")

	// Verify credentials
	authed, _ := app.DoAuth(r, w)
	if !authed {
		return
	}

	w.WriteHeader(http.StatusOK)
}
func (app *Kosync) HandleSyncsProgress(w http.ResponseWriter, r *http.Request) {
	app.DebugPrint("Sync request")

	// Verify credentials
	authed, user := app.DoAuth(r, w)
	if !authed {
		return
	}

	// Parse payload
	var data ProgressRequest
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		app.DebugPrint(fmt.Sprintf("[user: %s]: Error reading request body: %v", user.Username, err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	app.DebugPrint(fmt.Sprintf("[user: %s]: Received progress update for document '%s'", user.Username, data.Document))

	if app.Db.Config.StoreHistory {
		var currentVersion = app.Db.Users[user.Username].Documents[data.Document]
		var previousData = app.Db.Users[user.Username].History[data.Document].DocumentHistory
		app.Db.Users[user.Username].History[data.Document] = HistoryData{
			DocumentHistory: append(previousData, currentVersion),
		}
		app.DebugPrint(fmt.Sprintf("[user: %s]: Document '%s' progress went from %.2f %% to %.2f %%", user.Username, data.Document, currentVersion.Percentage*100, data.Percentage*100))
	}

	// Create document state
	app.Db.Users[user.Username].Documents[data.Document] = FileData{
		Progress:   data.Progress,
		Percentage: data.Percentage,
		Device:     data.Device,
		DeviceId:   data.DeviceId,
		Timestamp:  time.Now().Unix(),
	}

	// Persist
	go func() {
		_ = app.PersistDatabase()
	}()

	w.WriteHeader(http.StatusOK)
}

func (app *Kosync) HandleSyncsProgressGetDocument(w http.ResponseWriter, r *http.Request) {
	app.DebugPrint("Get Progress request")

	// Verify credentials
	authed, user := app.DoAuth(r, w)
	if !authed {
		return
	}

	// Get document id
	documentId := r.PathValue("document")
	app.DebugPrint(fmt.Sprintf("[user: %s]: Trying to find document '%s'", user.Username, documentId))

	// Find document
	docData, found := user.Documents[documentId]
	if !found {
		app.DebugPrint(fmt.Sprintf("[user: %s]: Document '%s' not found", user.Username, documentId))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Create DTO
	wireFormat := ProgressResponse{
		Document:   documentId,
		Progress:   docData.Progress,
		Percentage: docData.Percentage,
		Device:     docData.Device,
		DeviceId:   docData.DeviceId,
		Timestamp:  docData.Timestamp,
	}

	// Json encode
	data, err := json.Marshal(wireFormat)
	if err != nil {
		app.DebugPrint(fmt.Sprintf("[user: %s]: Error encoding response: %v", user.Username, err))
		return
	}

	// Send JSON
	app.DebugPrint(fmt.Sprintf("[user: %s]: Sending response for document '%s': %s", user.Username, documentId, string(data)))
	w.Header().Set("Content-Type", "application/json")
	bytesWritten, err := w.Write(data)
	if err != nil {
		app.DebugPrint(fmt.Sprintf("[user: %s]: Error sending response: %v", user.Username, err))
		return
	}
	if bytesWritten != len(data) {
		app.DebugPrint(fmt.Sprintf("[user: %s]: Error sending response: only %d bytes written", user.Username, bytesWritten))
		return
	}
}

func main() {
	log.Println("[KOsync] KOsync Server v2026.02.0 by Thomas Obernosterer (https://obth.eu)")
	log.Println("[KOsync] Copyright 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later.")
	log.Println("[KOsync] Obtain the Source Code at https://git.obth.eu/atjontv/kosync")
	log.Println("[KOsync] ")
	dbFileName := "database.json"
	var db Database

	// Handle reading database
	createEmptyDatabase := false
	stat, _ := os.Stat(dbFileName)
	if stat != nil && stat.Size() > 0 {
		data, err := os.ReadFile(dbFileName)
		if err != nil {
			panic(err)
		}

		if len(data) > 1 {
			err = json.Unmarshal(data, &db)
			if err != nil {
				panic(err)
			}
		} else {
			createEmptyDatabase = true
		}
	} else {
		createEmptyDatabase = true
	}

	// Fallback to empty
	if createEmptyDatabase {
		db = Database{
			Config: ConfigData{
				ListenAddress:       ":8080",
				DisableRegistration: false,
			},
			Users: make(map[string]UserData),
		}
	}

	// Enforce required defaults
	if len(db.Config.ListenAddress) == 0 {
		db.Config.ListenAddress = ":8080"
	}

	// New app instance
	kosync := Kosync{
		DatabaseFile: dbFileName,
		Db:           db,
		DbMutex:      sync.Mutex{},
	}

	// Perform schema migrations
	if err := kosync.MigrateSchema(); err != nil {
		panic(err)
	}

	// Persist database
	if err := kosync.PersistDatabase(); err != nil {
		panic(err)
	}

	// Register route handlers
	http.HandleFunc("/users/create", kosync.HandleUsersCreate)
	http.HandleFunc("/users/auth", kosync.HandleUsersAuth)
	http.HandleFunc("/syncs/progress", kosync.HandleSyncsProgress)
	http.HandleFunc("/syncs/progress/{document}", kosync.HandleSyncsProgressGetDocument)

	// Start server
	log.Printf("Starting KOsync server on '%s'", db.Config.ListenAddress)
	log.Fatal(http.ListenAndServe(db.Config.ListenAddress, nil))
}
