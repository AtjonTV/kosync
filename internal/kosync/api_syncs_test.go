//
// File:        internal/kosync/api_syncs_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestSyncsPostProgress(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	koapp := &Kosync{
		Db:     db,
		Config: &Config{EnableWebSocketApi: false},
	}

	app := fiber.New()
	app.Put("/syncs/progress", func(c fiber.Ctx) error {
		c.Locals(CtxContextUserId, "user123")
		c.Locals(CtxContextUserName, "testuser")
		return koapp.SyncsPostProgress(c)
	})

	progress := KoProgress{
		Document:   "test_doc",
		Progress:   "page 10",
		Percentage: 0.5,
		Device:     "test_device",
		DeviceId:   "device123",
	}
	body, _ := json.Marshal(progress)

	req := httptest.NewRequest(http.MethodPut, "/syncs/progress", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %v", resp.StatusCode)
	}

	// Verify DB state
	doc, found, err := db.FindDocumentById("user123", "test_doc")
	if err != nil {
		t.Fatalf("DB error: %v", err)
	}
	if !found {
		t.Errorf("Document not found in DB")
	}
	if doc.Progress != 0.5 {
		t.Errorf("Expected progress 0.5, got %v", doc.Progress)
	}
}

func TestSyncsGetProgress(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	koapp := &Kosync{
		Db: db,
	}

	doc := &Document{
		Id:              "test_doc",
		OwnerId:         "user123",
		CurrentLocation: "page 20",
		Progress:        0.8,
	}
	_ = db.CreateOrUpdateDocument(doc)

	app := fiber.New()
	app.Get("/syncs/progress/:document", func(c fiber.Ctx) error {
		c.Locals(CtxContextUserId, "user123")
		c.Locals(CtxContextUserName, "testuser")
		return koapp.SyncsGetProgress(c)
	})

	// 1. Success
	req := httptest.NewRequest(http.MethodGet, "/syncs/progress/test_doc", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %v", resp.StatusCode)
	}

	var result KoProgressWithTime
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Document != "test_doc" || result.Percentage != 0.8 {
		t.Errorf("Unexpected result: %+v", result)
	}

	// 2. Not found
	req = httptest.NewRequest(http.MethodGet, "/syncs/progress/unknown_doc", nil)
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %v", resp.StatusCode)
	}
}
