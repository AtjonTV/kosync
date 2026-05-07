//
// File:        internal/kosync/api_webui_test.go
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

func TestApiDeleteDocument(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	koapp := &Kosync{
		Db:     db,
		Config: &Config{EnableWebSocketApi: false},
	}

	doc := &Document{
		Id:      "doc_to_delete",
		OwnerId: "user123",
		Title:   "Delete Me",
	}
	_ = db.CreateOrUpdateDocument(doc)

	app := fiber.New()
	app.Delete("/api/documents.delete", func(c fiber.Ctx) error {
		c.Locals(CtxContextUserId, "user123")
		return koapp.ApiDeleteDocument(c)
	})

	// 1. Missing ID
	req := httptest.NewRequest(http.MethodDelete, "/api/documents.delete", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing ID, got %v", resp.StatusCode)
	}

	// 2. Successful delete
	req = httptest.NewRequest(http.MethodDelete, "/api/documents.delete?id=doc_to_delete", nil)
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204 for successful delete, got %v", resp.StatusCode)
	}

	// 3. Verify it is deleted in DB
	_, found, _ := db.FindDocumentById("user123", "doc_to_delete")
	if found {
		t.Errorf("Document should be deleted")
	}
}

func TestApiGetDocumentsAll(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	user, _ := db.CreateUser("testuser", "pass")
	doc := &Document{
		Id:      "doc1",
		OwnerId: user.Id,
		Title:   "Doc 1",
	}
	_ = db.CreateOrUpdateDocument(doc)

	koapp := &Kosync{
		Db: db,
	}

	app := fiber.New()
	app.Get("/api/documents.all", func(c fiber.Ctx) error {
		c.Locals(CtxContextUserName, "testuser")
		return koapp.ApiGetDocumentsAll(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/documents.all", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %v", resp.StatusCode)
	}

	var results []DocumentWithHistory
	_ = json.NewDecoder(resp.Body).Decode(&results)
	if len(results) != 1 || results[0].Id != "doc1" {
		t.Errorf("Unexpected results: %+v", results)
	}
}

func TestApiPutDocument(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	user, _ := db.CreateUser("testuser", "pass")

	koapp := &Kosync{
		Db:     db,
		Config: &Config{EnableWebSocketApi: false},
	}

	app := fiber.New()
	app.Put("/api/documents.update", func(c fiber.Ctx) error {
		c.Locals(CtxContextUserName, "testuser")
		c.Locals(CtxContextUserId, user.Id)
		return koapp.ApiPutDocument(c)
	})

	doc := Document{
		Id:      "doc_to_update",
		OwnerId: user.Id,
		Title:   "Updated Title",
	}
	body, _ := json.Marshal(doc)

	req := httptest.NewRequest(http.MethodPut, "/api/documents.update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204, got %v", resp.StatusCode)
	}

	// Verify DB state
	savedDoc, found, _ := db.FindDocumentById(user.Id, "doc_to_update")
	if !found || savedDoc.Title != "Updated Title" {
		t.Errorf("Document not updated correctly")
	}
}

func TestApiDeleteDocumentHistory(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	user, _ := db.CreateUser("testuser", "pass")
	doc := &Document{
		Id:         "doc_h",
		OwnerId:    user.Id,
		LastReadAt: 123456789,
	}
	_ = db.CreateOrUpdateDocument(doc)
	// Update it again to create history
	doc.Progress = 0.5
	doc.LastReadAt = 123456790
	_ = db.CreateOrUpdateDocument(doc)

	koapp := &Kosync{
		Db:     db,
		Config: &Config{EnableWebSocketApi: false},
	}

	app := fiber.New()
	app.Delete("/api/documents.history.delete", func(c fiber.Ctx) error {
		c.Locals(CtxContextUserId, user.Id)
		return koapp.ApiDeleteDocumentHistory(c)
	})

	// Missing params
	req := httptest.NewRequest(http.MethodDelete, "/api/documents.history.delete", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %v", resp.StatusCode)
	}

	// Success
	req = httptest.NewRequest(http.MethodDelete, "/api/documents.history.delete?id=doc_h&last_read_at=123456789", nil)
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204, got %v", resp.StatusCode)
	}

	// Verify history deleted
	history, _ := db.GetDocumentHistory(user.Id, "doc_h")
	if len(history) != 0 {
		t.Errorf("History should be empty, got %d items", len(history))
	}
}
