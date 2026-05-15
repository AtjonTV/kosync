//
// File:        internal/kosync/database_statistics_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"testing"
	"time"
)

func TestGetReadStatistics(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	user, err := db.CreateUser("testuser", "testpass")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().Unix() * 1000

	// 1. New document (no history yet)
	doc := &Document{
		Id:                 "doc1",
		OwnerId:            user.Id,
		Title:              "Document 1",
		Progress:           0.1,
		LastReadAt:         float64(now - 1000*60*60*24*2), // 2 days ago
		LastReadOnDevice:   "Device 1",
		LastReadOnDeviceId: "d1",
	}
	if err := db.CreateOrUpdateDocument(doc); err != nil {
		t.Fatal(err)
	}

	// 2. Update today (will create history entry with 0.1 progress)
	doc.Progress = 0.2
	doc.LastReadAt = float64(now)
	if err := db.CreateOrUpdateDocument(doc); err != nil {
		t.Fatal(err)
	}

	// 3. Another update today (will create history entry with 0.2 progress)
	doc.Progress = 0.35
	doc.LastReadAt = float64(now + 1000)
	if err := db.CreateOrUpdateDocument(doc); err != nil {
		t.Fatal(err)
	}

	stats, err := db.GetReadStatistics(user.Id, 14)
	if err != nil {
		t.Fatal(err)
	}

	if len(stats) != 14 {
		t.Errorf("Expected 14 days of statistics, got %d", len(stats))
	}

	stats7, err := db.GetReadStatistics(user.Id, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats7) != 7 {
		t.Errorf("Expected 7 days of statistics, got %d", len(stats7))
	}

	// Today is the last entry in both
	todayStats := stats[13]
	todayStats7 := stats7[6]
	expectedDate := time.Now().Format("2006-01-02")
	if todayStats.Date != expectedDate {
		t.Errorf("Expected today's date %s, got %s", expectedDate, todayStats.Date)
	}
	if todayStats7.Date != expectedDate {
		t.Errorf("Expected today's date %s in 7-day stats, got %s", expectedDate, todayStats7.Date)
	}

	// We sent 2 updates today (the first creation 2 days ago didn't create history)
	// Update 1 (0.2): History gets 0.1. Document becomes 0.2.
	// Update 2 (0.35): History gets 0.2. Document becomes 0.35.
	// Update count = 2.
	// Progress increase = 0.35 - 0.1 = 0.25 (which is 25.0 percentage points)
	if todayStats.UpdateCount != 2 {
		t.Errorf("Expected 2 updates today, got %d", todayStats.UpdateCount)
	}

	if todayStats.ProgressIncrease < 24.9 || todayStats.ProgressIncrease > 25.1 {
		t.Errorf("Expected ~25.0%% progress increase, got %f", todayStats.ProgressIncrease)
	}
}
