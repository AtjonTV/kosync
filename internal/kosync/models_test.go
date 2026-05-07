//
// File:        internal/kosync/models_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"testing"
)

func TestDocument_ProgressAsString(t *testing.T) {
	doc := &Document{
		Progress: 0.1234,
	}
	expected := "12.34%"
	if actual := doc.ProgressAsString(); actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}

	doc.Progress = 1.0
	expected = "100.00%"
	if actual := doc.ProgressAsString(); actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}

	doc.Progress = 0.0
	expected = "0.00%"
	if actual := doc.ProgressAsString(); actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestDocumentFromMap(t *testing.T) {
	m := map[string]interface{}{
		"id":                     "doc123",
		"owner_id":               "user1",
		"title":                  "Test Book",
		"current_location":       "page 10",
		"progress":               float32(0.45),
		"last_read_on_device":    "Phone",
		"last_read_on_device_id": "phone1",
		"last_read_at":           float64(123456789.0),
	}

	doc := DocumentFromMap(m)

	if doc.Id != "doc123" {
		t.Errorf("expected doc123, got %s", doc.Id)
	}
	if doc.OwnerId != "user1" {
		t.Errorf("expected user1, got %s", doc.OwnerId)
	}
	if doc.Title != "Test Book" {
		t.Errorf("expected Test Book, got %s", doc.Title)
	}
	if doc.Progress != 0.45 {
		t.Errorf("expected 0.45, got %f", doc.Progress)
	}
}

func TestDocument_Equals(t *testing.T) {
	doc1 := &Document{
		Id:    "1",
		Title: "Book 1",
	}
	doc2 := &Document{
		Id:    "1",
		Title: "Book 1",
	}
	doc3 := &Document{
		Id:    "2",
		Title: "Book 1",
	}

	if !doc1.Equals(doc2) {
		t.Errorf("doc1 and doc2 should be equal")
	}
	if doc1.Equals(doc3) {
		t.Errorf("doc1 and doc3 should not be equal")
	}
}
