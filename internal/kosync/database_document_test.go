//
// File:        internal/kosync/database_document_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"testing"
)

func TestCreateDocument(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create database for testing: %+v", err)
	}

	doc := Document{
		Id:                 "valid_id",
		OwnerId:            "owner_123",
		Title:              "Test Document",
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
		t.Fatalf("Failed to create database for testing: %+v", err)
	}

	doc := Document{
		Id:                 "valid_id",
		OwnerId:            "owner_123",
		Title:              "Test Document",
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

func TestDocumentCRUD(t *testing.T) {
	// Setup
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create database for testing: %+v", err)
	}
	// Test CreateOrUpdateDocument
	testDoc := &Document{
		Id:                 "valid_id",
		OwnerId:            "owner_123",
		Title:              "Test Document",
		CurrentLocation:    "location_1",
		Progress:           0.5,
		LastReadOnDevice:   "device_abc",
		LastReadOnDeviceId: "device_id_123",
		LastReadAt:         1609459200000,
	}
	// Test
	err = db.CreateOrUpdateDocument(testDoc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}
	// Verify document exists
	doc, found, err := db.FindDocumentById("owner_123", "valid_id")
	if err != nil {
		t.Fatalf("Failed to find document: %v", err)
	}
	if !found {
		t.Fatalf("Document not found after creation")
	}
	if doc.Title != "Test Document" {
		t.Fatalf("Title mismatch: expected '%s', got '%s'", "Test Document", doc.Title)
	}
}
func TestFindDocumentById(t *testing.T) {
	// Setup
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create database for testing: %+v", err)
	}
	// Create a test document
	testDoc := &Document{
		Id:                 "valid_id",
		OwnerId:            "owner_123",
		Title:              "Test Title",
		CurrentLocation:    "location_1",
		Progress:           0.5,
		LastReadOnDevice:   "device_abc",
		LastReadOnDeviceId: "device_id_123",
		LastReadAt:         1609459200000,
	}
	err = db.CreateOrUpdateDocument(testDoc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}
	// Test Case 1: Document found
	doc, found, err := db.FindDocumentById("owner_123", "valid_id")
	if err != nil {
		t.Fatalf("Failed to find document: %v", err)
	}
	if !found || doc == nil {
		t.Fatal("Document not found")
	}
	if doc.Title != "Test Title" {
		t.Fatalf("Title mismatch: expected '%s', got '%s'", "Test Title", doc.Title)
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
		t.Fatalf("Failed to create database for testing: %+v", err)
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
		t.Fatalf("Failed to create database for testing: %+v", err)
	}
	// Create a test document
	testDoc := &Document{
		Id:                 "valid_id",
		OwnerId:            "owner_123",
		Title:              "Test Title",
		CurrentLocation:    "location_1",
		Progress:           0.5,
		LastReadOnDevice:   "device_abc",
		LastReadOnDeviceId: "device_id_123",
		LastReadAt:         1609459200000,
	}
	err = db.CreateOrUpdateDocument(testDoc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}
	err = db.CreateOrUpdateDocument(testDoc)
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
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
