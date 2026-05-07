//
// File:        internal/kosync/database_document_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"testing"
)

const (
	testDbCreateError = "Failed to create database for testing: %+v"
	testDocTitle      = "Test Document"
	testDocCreateErr  = "Failed to create document: %v"
	testTitle         = "Test Title"
)

func TestCreateDocument(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateError, err)
	}

	doc := Document{
		Id:                 "valid_id",
		OwnerId:            "owner_123",
		Title:              testDocTitle,
		CurrentLocation:    "location_1",
		Progress:           0.5,
		LastReadOnDevice:   "device_abc",
		LastReadOnDeviceId: "device_id_123",
		LastReadAt:         1609459200000,
	}

	err = db.CreateOrUpdateDocument(&doc)
	if err != nil {
		t.Fatalf("Failed to create the first document: %v", err)
	}
}

func TestUpdateDocument(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateError, err)
	}

	doc := Document{
		Id:                 "valid_id",
		OwnerId:            "owner_123",
		Title:              testDocTitle,
		CurrentLocation:    "location_1",
		Progress:           0.5,
		LastReadOnDevice:   "device_abc",
		LastReadOnDeviceId: "device_id_123",
		LastReadAt:         1609459200000,
	}

	err = db.CreateOrUpdateDocument(&doc)
	if err != nil {
		t.Fatalf("Failed to create the first document: %v", err)
	}

	dbDoc, _, err := db.FindDocumentById("owner_123", "valid_id")
	if err != nil {
		t.Fatalf("Failed to get document that was just created: %v", err)
	}

	if dbDoc == nil {
		t.Fatalf("Document was nil after creation")
		return
	}

	if !doc.Equals(dbDoc) {
		t.Fatalf("Document was different from expected.\nExpected: %+v\nActual: %+v", doc, dbDoc)
	}

	doc.Progress = 0.99

	err = db.CreateOrUpdateDocument(&doc)
	if err != nil {
		t.Fatalf("Failed to update document: %v", err)
	}

	newDbDoc, _, err := db.FindDocumentById("owner_123", "valid_id")
	if err != nil {
		t.Fatalf("Failed to get document that was just updated: %v", err)
	}

	if !doc.Equals(newDbDoc) {
		t.Fatalf("Document was different from expected.\nExpected: %+v\nActual: %+v", doc, newDbDoc)
	}
}

func TestUpdateDocumentEmptyTitle(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateError, err)
	}

	doc := Document{
		Id:      "empty_title_test",
		OwnerId: "owner_123",
		Title:   "Original Title",
	}

	err = db.CreateOrUpdateDocument(&doc)
	if err != nil {
		t.Fatalf("Failed to create the first document: %v", err)
	}

	// Update with empty title
	doc.Title = ""
	err = db.CreateOrUpdateDocument(&doc)
	if err != nil {
		t.Fatalf("Failed to update document: %v", err)
	}

	dbDoc, _, err := db.FindDocumentById("owner_123", "empty_title_test")
	if err != nil {
		t.Fatalf("Failed to get document: %v", err)
	}

	if dbDoc.Title != "Original Title" {
		t.Fatalf("Title should have remained 'Original Title', but got '%s'", dbDoc.Title)
	}
}

func TestDocumentCRUD(t *testing.T) {
	// Setup
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateError, err)
	}
	// Test CreateOrUpdateDocument
	testDoc := &Document{
		Id:                 "valid_id",
		OwnerId:            "owner_123",
		Title:              testDocTitle,
		CurrentLocation:    "location_1",
		Progress:           0.5,
		LastReadOnDevice:   "device_abc",
		LastReadOnDeviceId: "device_id_123",
		LastReadAt:         1609459200000,
	}
	// Test
	err = db.CreateOrUpdateDocument(testDoc)
	if err != nil {
		t.Fatalf(testDocCreateErr, err)
	}
	// Verify document exists
	doc, found, err := db.FindDocumentById("owner_123", "valid_id")
	if err != nil {
		t.Fatalf("Failed to find document: %v", err)
	}
	if !found {
		t.Fatalf("Document not found after creation")
	}
	if doc.Title != testDocTitle {
		t.Fatalf("Title mismatch: expected '%s', got '%s'", testDocTitle, doc.Title)
	}
}
func TestFindDocumentById(t *testing.T) {
	// Setup
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateError, err)
	}
	// Create a test document
	testDoc := &Document{
		Id:                 "valid_id",
		OwnerId:            "owner_123",
		Title:              testTitle,
		CurrentLocation:    "location_1",
		Progress:           0.5,
		LastReadOnDevice:   "device_abc",
		LastReadOnDeviceId: "device_id_123",
		LastReadAt:         1609459200000,
	}
	err = db.CreateOrUpdateDocument(testDoc)
	if err != nil {
		t.Fatalf(testDocCreateErr, err)
	}
	// Test Case 1: Document found
	doc, found, err := db.FindDocumentById("owner_123", "valid_id")
	if err != nil {
		t.Fatalf("Failed to find document: %v", err)
	}
	if !found || doc == nil {
		t.Fatal("Document not found")
	}
	if doc.Title != testTitle {
		t.Fatalf("Title mismatch: expected '%s', got '%s'", testTitle, doc.Title)
	}
	// Test Case 2: Document not found
	_, found, err = db.FindDocumentById("owner_123", "invalid_id")
	if err != nil {
		t.Fatalf("Unexpected error finding non-existent document: %v", err)
	}
	if found {
		t.Fatal("Document found when it shouldn't be")
	}
}
func TestAllDocumentsOfUser(t *testing.T) {
	// Setup
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateError, err)
	}
	// Create multiple documents
	docs := []*Document{
		{
			Id:                 "doc1",
			OwnerId:            "owner_123",
			Title:              "Doc 1",
			CurrentLocation:    "location_1",
			Progress:           0.5,
			LastReadOnDevice:   "device_abc",
			LastReadOnDeviceId: "device_id_123",
			LastReadAt:         1609459200000,
		},
		{
			Id:                 "doc2",
			OwnerId:            "owner_123",
			Title:              "Doc 2",
			CurrentLocation:    "location_2",
			Progress:           0.75,
			LastReadOnDevice:   "device_xyz",
			LastReadOnDeviceId: "device_id_456",
			LastReadAt:         1609459200001,
		},
	}
	for _, doc := range docs {
		err := db.CreateOrUpdateDocument(doc)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}
	}
	// Test
	docsResult, err := db.AllDocumentsOfUser("owner_123")
	if err != nil {
		t.Fatalf("Failed to get documents: %v", err)
	}
	if len(docsResult) != 2 {
		t.Fatalf("Expected 2 documents, got %d", len(docsResult))
	}
}
func TestGetDocumentHistory(t *testing.T) {
	// Setup
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateError, err)
	}
	// Create a test document
	testDoc := &Document{
		Id:                 "valid_id",
		OwnerId:            "owner_123",
		Title:              testTitle,
		CurrentLocation:    "location_1",
		Progress:           0.5,
		LastReadOnDevice:   "device_abc",
		LastReadOnDeviceId: "device_id_123",
		LastReadAt:         1609459200000,
	}
	err = db.CreateOrUpdateDocument(testDoc)
	if err != nil {
		t.Fatalf(testDocCreateErr, err)
	}
	err = db.CreateOrUpdateDocument(testDoc)
	if err != nil {
		t.Fatalf(testDocCreateErr, err)
	}
	// Test
	history, err := db.GetDocumentHistory("owner_123", "valid_id")
	if err != nil {
		t.Fatalf("Failed to get document history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("Expected 1 history entry, got %d", len(history))
	}
}

func TestAllDocumentsOfUserWithHistory(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create a database for testing: %v", err)
	}

	doc := Document{
		Id:      "doc1",
		OwnerId: "user1",
		Title:   "Title 1",
	}

	err = db.CreateOrUpdateDocument(&doc)
	if err != nil {
		t.Fatalf("Failed to create a document: %v", err)
	}

	// Update to create history
	doc.Title = "Title 1 Updated"
	err = db.CreateOrUpdateDocument(&doc)
	if err != nil {
		t.Fatalf("Failed to update document: %v", err)
	}

	results, err := db.AllDocumentsOfUserWithHistory("user1")
	if err != nil {
		t.Fatalf("Failed to get documents with history: %v", err)
	}

	if len(*results) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(*results))
	}

	if (*results)[0].Document.Id != "doc1" {
		t.Errorf("Expected doc1, got %s", (*results)[0].Document.Id)
	}

	if len((*results)[0].History) != 1 {
		t.Fatalf("Expected 1 history entry, got %d", len((*results)[0].History))
	}
}

func TestSoftDeleteDocumentHistory(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateError, err)
	}

	ownerId := "user_h"
	docId := "doc_h"

	doc := &Document{
		Id:      docId,
		OwnerId: ownerId,
		Title:   "Original Title",
	}

	// Create document
	err = db.CreateOrUpdateDocument(doc)
	if err != nil {
		t.Fatalf(testDocCreateErr, err)
	}

	// Update document to create history entry
	doc.Title = "Updated Title"
	err = db.CreateOrUpdateDocument(doc)
	if err != nil {
		t.Fatalf("Failed to update document: %v", err)
	}

	// Get history
	history, err := db.GetDocumentHistory(ownerId, docId)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("Expected 1 history entry, got %d", len(history))
	}
	historyItem := history[0]

	// Verify we can delete it (this will fail compilation until we add the method)
	// We'll use the last_read_at as identifier for the history item since document_id + owner_id + last_read_at is a typical way to identify them
	err = db.DeleteDocumentHistoryItem(ownerId, docId, int64(historyItem.LastReadAt))
	if err != nil {
		t.Fatalf("Failed to delete history item: %v", err)
	}

	// Verify it's gone from history
	history, err = db.GetDocumentHistory(ownerId, docId)
	if err != nil {
		t.Fatalf("Failed to get history after deletion: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("Expected 0 history entries after deletion, got %d", len(history))
	}
}

func TestDeleteDocumentById(t *testing.T) {
	// Setup
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf(testDbCreateError, err)
	}

	doc := &Document{
		Id:      "delete_me",
		OwnerId: "owner_123",
		Title:   "Delete Me",
	}

	err = db.CreateOrUpdateDocument(doc)
	if err != nil {
		t.Fatalf(testDocCreateErr, err)
	}

	// Verify it exists
	_, found, err := db.FindDocumentById("owner_123", "delete_me")
	if err != nil || !found {
		t.Fatalf("Document should exist before deletion")
	}

	// Delete
	err = db.DeleteDocumentById("owner_123", "delete_me")
	if err != nil {
		t.Fatalf("Failed to delete document: %v", err)
	}

	// Verify it's gone from FindDocumentById
	_, found, err = db.FindDocumentById("owner_123", "delete_me")
	if err != nil {
		t.Fatalf("Error finding document: %v", err)
	}
	if found {
		t.Fatalf("Document should NOT be found after deletion")
	}

	// Verify it's gone from AllDocumentsOfUser
	docs, err := db.AllDocumentsOfUser("owner_123")
	if err != nil {
		t.Fatalf("Error getting all documents: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("Expected 0 documents, got %d", len(docs))
	}
}

func TestCreateOrUpdateDocument_TransactionFail(t *testing.T) {
	// Trigger an error by closing the DB
	db, _ := NewTemporaryDatabase(true)
	db.Close()
	err := db.CreateOrUpdateDocument(&Document{Id: "test", OwnerId: "user"})
	if err == nil {
		t.Error("Expected error when database is closed")
	}
}
