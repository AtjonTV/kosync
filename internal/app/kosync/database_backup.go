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
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shamaton/msgpack/v3"
)

const (
	BackupFileType            = "KOSYNC BACKUP"
	BackupEncodingTypeJson    = "json"
	BackupEncodingTypeMsgpack = "msgpack"
)

func (app *Kosync) BackupDatabase() error {
	if err := app.PersistDatabase(); err != nil {
		return err
	}

	app.DbMutex.Lock()
	defer app.DbMutex.Unlock()

	var contentType = ""
	var binaryData []byte
	var err error

	if app.Db.Config.BackupEncodingType == BackupEncodingTypeJson || app.Db.Schema < 2 {
		binaryData, err = json.Marshal(app.Db)
		contentType = "application/json"
	} else if app.Db.Config.BackupEncodingType == BackupEncodingTypeMsgpack {
		binaryData, err = msgpack.Marshal(app.Db)
		contentType = "application/vnd.msgpack"
	} else {
		return fmt.Errorf("can not create database backup for unknown content type '%s'", app.Db.Config.BackupEncodingType)
	}
	if err != nil {
		return err
	}

	now := time.Now()
	block := pem.Block{
		Type: BackupFileType,
		Headers: map[string]string{
			"App":          "https://git.obth.eu/atjontv/kosync",
			"Content-Type": contentType,
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

func RestoreDatabase(backupFile string) error {
	log.Println(fmt.Sprintf("[Restore]: Trying to restore database from file '%s'", backupFile))

	dbFile, err := FindDatabaseFile()
	if err != nil {
		return err
	}

	backupData, err := os.ReadFile(backupFile)
	if err != nil {
		return err
	}

	pemData, _ := pem.Decode(backupData)

	if pemData.Type != BackupFileType {
		return fmt.Errorf("the backup file does not contain a KOsync backup. It contains: '%s'", pemData.Type)
	}

	contentType, found := pemData.Headers["Content-Type"]
	if !found {
		return fmt.Errorf("the backup file does not specify a content type and cant be decoded")
	}

	_, found = pemData.Headers["Schema"]
	if !found {
		return fmt.Errorf("the backup file does not specify a schema version and cant be restored")
	}

	var db Database
	if contentType == "application/json" {
		if err := json.Unmarshal(pemData.Bytes, &db); err != nil {
			return err
		}
	} else if contentType == "application/vnd.msgpack" {
		if err := msgpack.Unmarshal(pemData.Bytes, &db); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("content type of backup file is not supported '%s'", contentType)
	}

	if db.Schema > SchemaVersion {
		return fmt.Errorf("can not restore a backup from a newer version. The backup has schema version %d while the server has %d", db.Schema, SchemaVersion)
	}

	log.Println("[Restore]: Restoring the database file")
	tmpKosync := Kosync{
		DatabaseFile: dbFile,
		Db:           db,
		DbMutex:      sync.Mutex{},
	}

	if err := tmpKosync.PersistDatabase(); err != nil {
		return err
	}

	log.Println("[Restore]: Restore complete.")
	return nil
}
