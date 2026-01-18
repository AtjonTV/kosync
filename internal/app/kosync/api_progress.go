//
// File:        internal/app/kosync/api_progress.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (app *Kosync) RegisterApiProgress() {
	http.HandleFunc("/syncs/progress", app.handleSyncsProgress)
	http.HandleFunc("/syncs/progress/{document}", app.handleSyncsProgressGetDocument)
}

func (app *Kosync) handleSyncsProgress(w http.ResponseWriter, r *http.Request) {
	app.DebugPrint("Sync request")

	// Verify credentials
	authed, user := app.DoAuth(r, w)
	if !authed {
		return
	}

	// Parse payload
	var data struct {
		Document   string  `json:"document"`
		Progress   string  `json:"progress"`
		Percentage float32 `json:"percentage"`
		Device     string  `json:"device"`
		DeviceId   string  `json:"device_id"`
	}
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

func (app *Kosync) handleSyncsProgressGetDocument(w http.ResponseWriter, r *http.Request) {
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

	type ProgressResponse struct {
		Document   string  `json:"document"`
		Progress   string  `json:"progress"`
		Percentage float32 `json:"percentage"`
		Device     string  `json:"device"`
		DeviceId   string  `json:"device_id"`
		Timestamp  int64   `json:"timestamp"`
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
