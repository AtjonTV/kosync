//
// File:        internal/kosync/koreader_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"math"
	"testing"
	"time"
)

func TestDocumentToKoProgressWithTime(t *testing.T) {
	// Test case 1: Normal input
	d := &Document{
		Id:                 "test-doc",
		CurrentLocation:    "start",
		Progress:           50.0,
		LastReadOnDevice:   "device1",
		LastReadOnDeviceId: "device1-id",
		LastReadAt:         1609459200000000000,
	}
	result := DocumentToKoProgressWithTime(d)
	if result.Document != "test-doc" {
		t.Errorf("Expected document 'test-doc', got %s", result.Document)
	}
	if result.Percentage != 50.0 {
		t.Errorf("Expected percentage 50.0, got %f", result.Percentage)
	}
	if result.Device != "device1" {
		t.Errorf("Expected device 'device1', got %s", result.Device)
	}
	if result.Timestamp != 1609459200000000000 {
		t.Errorf("Expected timestamp %d, got %f", 1609459200000000000, result.Timestamp)
	}
}
func TestKoProgressToDocument(t *testing.T) {
	// Test case 1: Normal input
	ownerId := "owner1"
	doc := KoProgress{
		Document:   "test-doc",
		Progress:   "start",
		Percentage: 50.0,
		Device:     "device1",
		DeviceId:   "device1-id",
	}
	result := KoProgressToDocument(&doc, ownerId)
	if result.Id != "test-doc" {
		t.Errorf("Expected document 'test-doc', got %s", result.Id)
	}
	if result.CurrentLocation != "start" {
		t.Errorf("Expected current location 'start', got %s", result.CurrentLocation)
	}
	if result.Progress != 50.0 {
		t.Errorf("Expected progress 50.0, got %f", result.Progress)
	}
	// Use a rounded comparison to avoid precision issues with dynamic timestamps
	roundedNow := float64(time.Now().UnixMicro() / 100.0)
	if math.Abs(result.LastReadAt-roundedNow) > 1e-6 {
		t.Errorf("Expected timestamp ~%f, got %f", roundedNow, result.LastReadAt)
	}
}
