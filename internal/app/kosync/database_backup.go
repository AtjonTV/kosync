//
// File:        internal/app/kosync/database_backup.go
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
