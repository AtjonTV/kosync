package kosyncng

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"git.obth.eu/atjontv/kosync/internal/app/kosync"
)

func (app *KosyncNg) PersistDatabase() error {
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

func (app *KosyncNg) AddUser(username, password string) error {
	app.DbLock.Lock()
	defer app.DbLock.Unlock()

	_, found := app.Db.Users[username]
	if found {
		return fmt.Errorf("username is already taken")
	}

	// Create user
	app.Db.Users[username] = kosync.UserData{
		Username:  username,
		Password:  password,
		Documents: make(map[string]kosync.FileData),
		History:   make(map[string]kosync.HistoryData),
	}

	// Persist new user
	return app.PersistDatabase()
}

func (app *KosyncNg) AddOrUpdateDocument(username string, document kosync.DocumentData) error {
	app.DbLock.Lock()
	defer app.DbLock.Unlock()

	if app.Db.Config.StoreHistory {
		var currentVersion = app.Db.Users[username].Documents[document.Document]
		var previousData = app.Db.Users[username].History[document.Document].DocumentHistory
		app.Db.Users[username].History[document.Document] = kosync.HistoryData{
			DocumentHistory: append(previousData, currentVersion),
		}
		app.DebugPrint("DB", "-", fmt.Sprintf("[user: %s]: Document '%s' progress went from %.2f %% to %.2f %%", username, document.Document, currentVersion.Percentage*100, document.Percentage*100))
	}

	// Create document state
	app.Db.Users[username].Documents[document.Document] = kosync.FileData{
		ProgressData: document.ProgressData,
		Timestamp:    time.Now().Unix(),
	}

	// Persist new user
	return app.PersistDatabase()
}
