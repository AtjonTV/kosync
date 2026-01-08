//
// File:        main.go
// Project:     https://git.obth.eu/atjontv/kosync
// License:     EUPL-1.2 or later
// Description: KOsync is a simple KOReader progress sync server,
//				contained in a single Go file, it compiles to a single static binary.
// Build:       go build -tags netgo main.go && strip main
//

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Database struct {
	Users               map[string]UserData `json:"users"`
	ListenAddress       string              `json:"listen_address"`
	DisableRegistration bool                `json:"disable_registration"`
}

type UserData struct {
	Username  string              `json:"username"`
	Password  string              `json:"password"`
	Documents map[string]FileData `json:"documents"`
}

type FileData struct {
	Progress   string  `json:"progress"`
	Percentage float32 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceId   string  `json:"device_id"`
	Timestamp  int64   `json:"timestamp"`
}

func (db *Database) FindUser(username string) (bool, *UserData) {
	if _, ok := db.Users[username]; !ok {
		return false, nil
	}
	user := db.Users[username]
	return true, &user
}

func (db *Database) AuthUser(username, password string) (bool, bool) {
	found, user := db.FindUser(username)
	if !found {
		return false, false
	}
	return true, user.Password == password
}

func (db *Database) SyncToDisk(databaseFileName string) error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(databaseFileName, data, 0644)
}

func (user *UserData) FindDocument(fileId string) (bool, *FileData) {
	if _, ok := user.Documents[fileId]; !ok {
		return false, nil
	}
	file := user.Documents[fileId]
	return true, &file
}

func doAuth(db *Database, r *http.Request, w http.ResponseWriter) (bool, *UserData) {
	username := r.Header.Get("x-auth-user")
	password := r.Header.Get("x-auth-key")

	found, authenticated := db.AuthUser(username, password)
	if !found || !authenticated {
		w.WriteHeader(http.StatusUnauthorized)
		log.Printf("[user: %s]: Unauthorized access for user with password '%s'", username, password)
		return false, nil
	}

	log.Printf("[user: %s]: Authorized access for user", username)
	_, user := db.FindUser(username)
	return true, user
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

func main() {
	dbFileName := "database.json"
	var db Database

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

	if createEmptyDatabase {
		db = Database{
			ListenAddress:       ":8080",
			DisableRegistration: false,
			Users:               make(map[string]UserData),
		}
	}

	if len(db.ListenAddress) == 0 {
		db.ListenAddress = ":8080"
	}

	dbLock := sync.Mutex{}
	saveDb := func() {
		dbLock.Lock()
		defer dbLock.Unlock()
		err := db.SyncToDisk(dbFileName)
		if err != nil {
			log.Printf("Error saving database: %v", err)
		}
	}

	go saveDb()

	http.HandleFunc("/users/create", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Register request")

		if db.DisableRegistration {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		var data RegisterRequest
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
			// return the same status as expected by the kosync plugin, which is 402 for some reason?
			w.WriteHeader(http.StatusPaymentRequired)
			return
		}

		db.Users[data.Username] = UserData{
			Username:  data.Username,
			Password:  data.Password,
			Documents: make(map[string]FileData),
		}

		go saveDb()

		w.WriteHeader(http.StatusCreated)
	})

	http.HandleFunc("/users/auth", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Auth request")
		authed, _ := doAuth(&db, r, w)
		if !authed {
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/syncs/progress", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Sync request")

		authed, user := doAuth(&db, r, w)
		if !authed {
			return
		}

		var data ProgressRequest
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			log.Printf("[user: %s]: Error reading request body: %v", user.Username, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		log.Printf("[user: %s]: Received progress update for document '%s'", user.Username, data.Document)
		db.Users[user.Username].Documents[data.Document] = FileData{
			Progress:   data.Progress,
			Percentage: data.Percentage,
			Device:     data.Device,
			DeviceId:   data.DeviceId,
			Timestamp:  time.Now().Unix(),
		}

		go saveDb()

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/syncs/progress/{document}", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Get Progress request")

		authed, user := doAuth(&db, r, w)
		if !authed {
			return
		}

		documentId := r.PathValue("document")
		log.Printf("[user: %s]: Trying to find document '%s'", user.Username, documentId)

		found, docData := user.FindDocument(documentId)
		if !found {
			log.Printf("[user: %s]: Document '%s' not found", user.Username, documentId)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		wireFormat := ProgressResponse{
			Document:   documentId,
			Progress:   docData.Progress,
			Percentage: docData.Percentage,
			Device:     docData.Device,
			DeviceId:   docData.DeviceId,
			Timestamp:  docData.Timestamp,
		}

		data, err := json.Marshal(wireFormat)
		if err != nil {
			log.Printf("[user: %s]: Error encoding response: %v", user.Username, err)
			return
		}

		log.Printf("[user: %s]: Sending response for document '%s': %s", user.Username, documentId, string(data))
		w.Header().Set("Content-Type", "application/json")
		bytesWritten, err := w.Write(data)
		if err != nil {
			log.Printf("[user: %s]: Error sending response: %v", user.Username, err)
			return
		}
		if bytesWritten != len(data) {
			log.Printf("[user: %s]: Error sending response: only %d bytes written", user.Username, bytesWritten)
			return
		}
	})

	log.Printf("Starting KOsync server on '%s'", db.ListenAddress)
	log.Fatal(http.ListenAndServe(db.ListenAddress, nil))
}
