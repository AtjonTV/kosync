//
// File:        internal/kosync/database_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync_test

import (
	"crypto/rand"
	rand2 "math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/kosync"
	"git.obth.eu/atjontv/kosync/internal/kosync/migrations"
)

func newMemDb(t *testing.T) *kosync.Database {
	conf := kosync.Config{
		DatabaseFile: filepath.Join(os.TempDir(), "kotest.db"),
	}
	db, err := kosync.NewDatabase(&conf)
	if err != nil {
		t.Errorf("Could not create in-memory database for testing: %v", err)
	}
	return db
}

func TestMigrationsApplied(t *testing.T) {
	_, i, _ := migrations.LoadMigrations()

	db := newMemDb(t)

	if db.SchemaVersion() != i {
		t.FailNow()
	}
}

func TestCreateDocument(t *testing.T) {
	db := newMemDb(t)

	doc := kosync.Document{
		Id:                 "1",
		OwnerId:            "1",
		Title:              "",
		CurrentLocation:    "",
		Progress:           0,
		LastReadOnDevice:   "",
		LastReadOnDeviceId: "",
		LastReadAt:         0,
	}

	err := db.CreateOrUpdateDocument(&doc)
	if err != nil {
		t.Errorf("Failed to create the first document: %v", err)
	}
}

func TestUpdateDocument(t *testing.T) {
	db := newMemDb(t)

	doc := kosync.Document{
		Id:                 "1",
		OwnerId:            "1",
		Title:              rand.Text(),
		CurrentLocation:    rand.Text(),
		Progress:           rand2.Float32(),
		LastReadOnDevice:   rand.Text(),
		LastReadOnDeviceId: rand.Text(),
		LastReadAt:         float64(time.Now().UnixMicro()),
	}

	err := db.CreateOrUpdateDocument(&doc)
	if err != nil {
		t.Errorf("Failed to create the first document: %v", err)
	}

	dbDoc, _, err := db.FindDocumentById("1", "1")
	if err != nil {
		t.Errorf("Failed to get document that was just created: %v", err)
	}

	if dbDoc == nil {
		t.Errorf("Document was nil after creation")
		return
	}

	if !doc.Equals(dbDoc) {
		t.Errorf("Document was different from expected.\nExpected: %+v\nActual: %+v", doc, dbDoc)
	}

	doc.Progress = 0.99

	err = db.CreateOrUpdateDocument(&doc)
	if err != nil {
		t.Errorf("Failed to update document: %v", err)
	}

	newDbDoc, _, err := db.FindDocumentById("1", "1")
	if err != nil {
		t.Errorf("Failed to get document that was just updated: %v", err)
	}

	if !doc.Equals(newDbDoc) {
		t.Errorf("Document was different from expected.\nExpected: %+v\nActual: %+v", doc, newDbDoc)
	}
}
