//
// File:        internal/kosync/database_backup_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupAndRestoreDatabase(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create a database for testing: %v", err)
	}
	defer func(db *Database) {
		_ = db.Close()
	}(db)

	// Create some data to backup
	_, err = db.CreateUser("backupuser", "backuppass")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "test.db")

	// We need a real file on disk for the backup logic as it uses filepath.Dir(cfg.DatabaseFile)
	// NewTemporaryDatabase uses os.TempDir() + "kotest.db"
	// Let's create a custom config for backup test
	cfg := &Config{
		DatabaseFile: dbFile,
	}

	// Backup
	err = BackupDatabase(cfg, db.rawDb)
	if err != nil {
		t.Fatalf("BackupDatabase failed: %v", err)
	}

	// Find the backup file
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}

	var backupFile string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "test_") && strings.HasSuffix(f.Name(), ".db") {
			backupFile = filepath.Join(tempDir, f.Name())
			break
		}
	}

	if backupFile == "" {
		t.Fatal("Backup file not found")
	}

	// Now try to restore into a NEW database
	db2, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create a second database: %v", err)
	}
	defer func(db2 *Database) {
		_ = db2.Close()
	}(db2)

	// Verify user does NOT exist in db2
	_, found, _ := db2.FindUserByUsername("backupuser")
	if found {
		t.Fatal("User should not exist in a new database before restore")
	}

	// Restore
	err = RestoreDatabase(db2.rawDb, backupFile)
	if err != nil {
		t.Fatalf("RestoreDatabase failed: %v", err)
	}

	// Verify user DOES exist in db2 after restore
	_, found, _ = db2.FindUserByUsername("backupuser")
	if !found {
		t.Fatal("User should exist in the database after restore")
	}
}

func TestRestoreDatabaseNonExistent(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create a database for testing: %v", err)
	}
	defer func(db *Database) {
		_ = db.Close()
	}(db)

	err = RestoreDatabase(db.rawDb, "non-existent-file.db")
	if !errors.Is(err, ErrRestoreSourceDoesNotExist) {
		t.Fatalf("Expected ErrRestoreSourceDoesNotExist, got %v", err)
	}
}
