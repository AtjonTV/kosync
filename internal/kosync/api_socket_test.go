//
// File:        internal/kosync/api_socket_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/pkg/jmp"
	"github.com/gofiber/fiber/v3"
)

func TestHandleOpenWebsocket(t *testing.T) {
	app := fiber.New()
	koapp := &Kosync{
		Crypt: NewDefaultCryptState(),
	}

	app.Get("/api/ws", koapp.HandleOpenWebsocket)

	// Case 1: Without auth
	req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status 303 (See Other), got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "/api/ws/") {
		t.Errorf("Expected location to contain /api/ws/, got %s", location)
	}

	// Case 2: With auth
	app = fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(CtxContextUserId, "user1")
		c.Locals(CtxContextUserName, "testuser")
		return c.Next()
	})
	app.Get("/api/ws", koapp.HandleOpenWebsocket)

	req = httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status 303 (See Other), got %d", resp.StatusCode)
	}

	location = resp.Header.Get("Location")
	if !strings.Contains(location, "/api/ws/") {
		t.Errorf("Expected location to contain /api/ws/, got %s", location)
	}

	// Should contain token
	parts := strings.Split(location, "/api/ws/")
	if len(parts) < 2 || parts[1] == "" {
		t.Errorf("Expected token in location, got %s", location)
	}
}

func TestRpcDocumentsDelete(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateErr, err)
	}
	defer func(db *Database) {
		_ = db.Close()
	}(db)

	app := &Kosync{Db: db, Jmp: jmp.New(), Config: &Config{DisableWebSocketApi: true}}

	// Create doc
	doc := &Document{Id: "d1", OwnerId: "u1"}
	err = db.CreateOrUpdateDocument(doc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	ctx := jmp.NewContext()
	ctx.Data[CtxContextUserId] = "u1"

	rpc := &jmp.RpcRequestPayload{
		Arguments: map[string]any{"document_id": "d1"},
	}

	res := app.RpcDocumentsDelete(ctx, rpc)
	if len(res.Errors) > 0 {
		t.Errorf("Expected success, got errors: %v", res.Errors)
	}

	// Verify deleted
	_, found, err := db.FindDocumentById("u1", "d1")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("Expected document to be deleted")
	}
}

func TestRpcDocumentsHistoryDelete(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateErr, err)
	}
	defer func(db *Database) {
		_ = db.Close()
	}(db)

	app := &Kosync{Db: db, Jmp: jmp.New(), Config: &Config{DisableWebSocketApi: true}}

	// Create doc with history
	doc := &Document{Id: "d1", OwnerId: "u1"}
	err = db.CreateOrUpdateDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	doc.Title = "Updated"
	err = db.CreateOrUpdateDocument(doc)
	if err != nil {
		t.Fatal(err)
	}

	history, err := db.GetDocumentHistory("u1", "d1")
	if err != nil {
		t.Fatal(err)
	}
	lastReadAt := int64(history[0].LastReadAt)

	ctx := jmp.NewContext()
	ctx.Data[CtxContextUserId] = "u1"

	rpc := &jmp.RpcRequestPayload{
		Arguments: map[string]any{
			"document_id":  "d1",
			"last_read_at": lastReadAt,
		},
	}

	res := app.RpcDocumentsHistoryDelete(ctx, rpc)
	if len(res.Errors) > 0 {
		t.Errorf("Expected success, got errors: %v", res.Errors)
	}

	// Verify history gone
	history, err = db.GetDocumentHistory("u1", "d1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Errorf("Expected 0 history items, got %d", len(history))
	}
}

func TestRpcDocumentsHistoryDelete_Float64(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateErr, err)
	}
	defer func(db *Database) {
		_ = db.Close()
	}(db)

	app := &Kosync{Db: db, Jmp: jmp.New(), Config: &Config{DisableWebSocketApi: true}}

	// Create doc with history
	doc := &Document{Id: "d1", OwnerId: "u1"}
	err = db.CreateOrUpdateDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	doc.Title = "Updated"
	err = db.CreateOrUpdateDocument(doc)
	if err != nil {
		t.Fatal(err)
	}

	history, err := db.GetDocumentHistory("u1", "d1")
	if err != nil {
		t.Fatal(err)
	}
	lastReadAt := float64(history[0].LastReadAt)

	ctx := jmp.NewContext()
	ctx.Data[CtxContextUserId] = "u1"

	rpc := &jmp.RpcRequestPayload{
		Arguments: map[string]any{
			"document_id":  "d1",
			"last_read_at": lastReadAt,
		},
	}

	res := app.RpcDocumentsHistoryDelete(ctx, rpc)
	if len(res.Errors) > 0 {
		t.Errorf("Expected success, got errors: %v", res.Errors)
	}

	// Verify history gone
	history, err = db.GetDocumentHistory("u1", "d1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Errorf("Expected 0 history items, got %d", len(history))
	}
}

func TestRpcDocumentsUpdateOwnershipEnforcement(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateErr, err)
	}
	defer func(db *Database) {
		_ = db.Close()
	}(db)

	app := &Kosync{Db: db, Jmp: jmp.New(), Config: &Config{DisableWebSocketApi: true}}

	ctx := jmp.NewContext()
	ctx.Data[CtxContextUserId] = "u2" // User 2 is logged in

	// Malicious RPC: User 2 tries to update a document for User 1
	rpc := &jmp.RpcRequestPayload{
		Arguments: map[string]any{
			"document": map[string]any{
				"id":       "doc1",
				"owner_id": "u1", // Claims it belongs to User 1
				"title":    "Pwned",
			},
		},
	}

	res := app.RpcDocumentsUpdate(ctx, rpc)
	if len(res.Errors) > 0 {
		t.Errorf("Expected success, got errors: %v", res.Errors)
	}

	// Verify that the document was saved for USER 2, not USER 1
	_, found1, _ := db.FindDocumentById("u1", "doc1")
	if found1 {
		t.Errorf("Document should NOT have been created for User 1")
	}

	savedDoc2, found2, _ := db.FindDocumentById("u2", "doc1")
	if !found2 {
		t.Errorf("Document should have been created for User 2")
	}
	if savedDoc2.OwnerId != "u2" {
		t.Errorf("Expected OwnerId u2, got %s", savedDoc2.OwnerId)
	}
}
