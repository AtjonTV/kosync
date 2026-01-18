//
// File:        internal/app/kosync/api_users.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func (app *Kosync) RegisterApiUsers() {
	http.HandleFunc("/users/create", app.handleUsersCreate)
	http.HandleFunc("/users/auth", app.handleUsersAuth)
}

func (app *Kosync) handleUsersCreate(w http.ResponseWriter, r *http.Request) {
	log.Println("Register request")

	// Return with forbidden when registration is disabled
	if app.Db.Config.DisableRegistration {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Parse payload
	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
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

func (app *Kosync) handleUsersAuth(w http.ResponseWriter, r *http.Request) {
	app.DebugPrint("Auth request")

	// Verify credentials
	authed, _ := app.DoAuth(r, w)
	if !authed {
		return
	}

	w.WriteHeader(http.StatusOK)
}
