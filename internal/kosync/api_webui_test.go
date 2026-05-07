//
// File:        internal/kosync/api_webui_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
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
