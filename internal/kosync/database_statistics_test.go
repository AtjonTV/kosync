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

	now := float64(time.Now().UnixMicro() / 100.0)

	// 1. New document (no history yet)
	doc := &Document{
		Id:                 "doc1",
		OwnerId:            user.Id,
		Title:              "Document 1",
		Progress:           0.1,
		LastReadAt:         now - 10000*60*60*24*2, // 2 days ago in 100us units
		LastReadOnDevice:   "Device 1",
		LastReadOnDeviceId: "d1",
	}
	if err := db.CreateOrUpdateDocument(doc); err != nil {
		t.Fatal(err)
	}

	// 2. Update today (will create history entry with 0.1 progress)
	doc.Progress = 0.2
	doc.LastReadAt = now
	if err := db.CreateOrUpdateDocument(doc); err != nil {
		t.Fatal(err)
	}

	// 3. Another update today (will create history entry with 0.2 progress)
	doc.Progress = 0.35
	doc.LastReadAt = now + 10000 // +1 second in 100us units
	if err := db.CreateOrUpdateDocument(doc); err != nil {
		t.Fatal(err)
	}

	stats, err := db.GetReadStatistics(user.Id, 14)
	if err != nil {
		t.Fatal(err)
	}

	if len(stats) != 14 {
		t.Fatalf("Expected 14 days of statistics, got %d", len(stats))
	}

	stats7, err := db.GetReadStatistics(user.Id, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats7) != 7 {
		t.Fatalf("Expected 7 days of statistics, got %d", len(stats7))
	}

	// Today is the last entry in both
	todayStats := stats[13]
	todayStats7 := stats7[6]
	expectedDate := time.Now().Format("2006-01-02")
	if todayStats.Date != expectedDate {
		t.Fatalf("Expected today's date %s, got %s", expectedDate, todayStats.Date)
	}
	if todayStats7.Date != expectedDate {
		t.Fatalf("Expected today's date %s in 7-day stats, got %s", expectedDate, todayStats7.Date)
	}

	// We sent 2 updates today (the first creation 2 days ago didn't create history)
	// Update 1 (0.2): History gets 0.1. Document becomes 0.2.
	// Update 2 (0.35): History gets 0.2. Document becomes 0.35.
	// Update count = 2.
	// Progress increase = 0.35 - 0.1 = 0.25 (which is 25.0 percentage points)
	if todayStats.UpdateCount != 2 {
		t.Fatalf("Expected 2 updates today, got %d", todayStats.UpdateCount)
	}

	if todayStats.ProgressIncrease < 24.9 || todayStats.ProgressIncrease > 25.1 {
		t.Fatalf("Expected ~25.0%% progress increase, got %f", todayStats.ProgressIncrease)
	}
}

func TestGetReadStatisticsByDay(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	user, err := db.CreateUser("testuser", "testpass")
	if err != nil {
		t.Fatal(err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	yesterdayTimestamp := float64(time.Now().UTC().AddDate(0, 0, -1).UnixMicro() / 100.0)

	// 1. Update yesterday
	doc := &Document{
		Id:                 "doc1",
		OwnerId:            user.Id,
		Title:              "Document 1",
		Progress:           0.5,
		LastReadAt:         yesterdayTimestamp,
		LastReadOnDevice:   "Device 1",
		LastReadOnDeviceId: "d1",
	}
	if err := db.CreateOrUpdateDocument(doc); err != nil {
		t.Fatal(err)
	}

	// 2. Fetch yesterday's stats
	// At this point, doc is in 'documents' table with yesterday's timestamp.
	stats, err := db.GetReadStatisticsByDay(user.Id, yesterday)
	if err != nil {
		t.Fatal(err)
	}

	if stats.Date != yesterday {
		t.Errorf("Expected date %s, got %s", yesterday, stats.Date)
	}
	if stats.UpdateCount != 1 {
		t.Errorf("Expected 1 update yesterday, got %d", stats.UpdateCount)
	}
	if stats.ProgressIncrease < 49.9 || stats.ProgressIncrease > 50.1 {
		t.Errorf("Expected ~50.0%% increase yesterday (0 to 0.5), got %f", stats.ProgressIncrease)
	}

	// 3. Update again today
	doc.Progress = 0.8
	doc.LastReadAt = float64(time.Now().UTC().UnixMicro() / 100.0)
	if err := db.CreateOrUpdateDocument(doc); err != nil {
		t.Fatal(err)
	}

	// 4. Update once more today to see progress increase
	doc.Progress = 1.0
	doc.LastReadAt = float64(time.Now().UTC().UnixMicro()/100.0 + 10000) // +1 second
	if err := db.CreateOrUpdateDocument(doc); err != nil {
		t.Fatal(err)
	}

	// 5. Fetch today's stats
	statsToday, err := db.GetReadStatisticsByDay(user.Id, today)
	if err != nil {
		t.Fatal(err)
	}

	if statsToday.UpdateCount != 2 {
		t.Errorf("Expected 2 updates today, got %d", statsToday.UpdateCount)
	}
	// Progress increase today: 1.0 - 0.5 = 0.5 (50.0 percentage points)
	if statsToday.ProgressIncrease < 49.9 || statsToday.ProgressIncrease > 50.1 {
		t.Errorf("Expected ~50.0%% increase today, got %f", statsToday.ProgressIncrease)
	}
}
