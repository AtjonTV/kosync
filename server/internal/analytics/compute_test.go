//
// File:        internal/analytics/compute_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics_test

import (
	"math"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/analytics"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

const sessionGap = 5 * time.Minute

// day is the date every fixture in this file is written on.
var day = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

// at returns a moment on the fixture day.
func at(hour, minute int) time.Time {
	return day.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

// dateOf formats a moment the way the statistics are keyed.
func dateOf(moment time.Time) string {
	return moment.UTC().Format(analytics.DateLayout)
}

// newApp returns a migrated app with a single user and no reading data.
func newApp(t testing.TB) (*tests.TestApp, *core.Record) {
	t.Helper()

	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	return app, user
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.000001
}

func TestComputeDayCountsUpdatesAndDocuments(t *testing.T) {
	app, user := newApp(t)

	// One document read three times, another read once.
	first := testutil.CreateDocument(t, app, user, "", "hash-a", 0.3, at(10, 20))
	testutil.CreateHistoryEntry(t, app, first, "", 0.1, at(10, 0))
	testutil.CreateHistoryEntry(t, app, first, "", 0.2, at(10, 10))
	testutil.CreateDocument(t, app, user, "", "hash-b", 0.5, at(20, 0))

	stats, err := analytics.ComputeDay(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}

	if stats.UpdateCount != 4 {
		t.Errorf("expected 4 progress moments, got %d", stats.UpdateCount)
	}
	if stats.DocumentsTouched != 2 {
		t.Errorf("expected 2 documents touched, got %d", stats.DocumentsTouched)
	}
}

func TestComputeDayMeasuresProgressAgainstThePreviousDay(t *testing.T) {
	app, user := newApp(t)

	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.6, at(10, 0))
	// Yesterday the reader was already at 40%.
	testutil.CreateHistoryEntry(t, app, document, "", 0.4, at(10, 0).AddDate(0, 0, -1))

	stats, err := analytics.ComputeDay(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}

	if !almostEqual(stats.ProgressIncrease, 20) {
		t.Errorf("expected an increase of 20 percentage points, got %v", stats.ProgressIncrease)
	}
}

func TestComputeDayCountsAFreshDocumentFromZero(t *testing.T) {
	app, user := newApp(t)

	testutil.CreateDocument(t, app, user, "", "hash-a", 0.15, at(10, 0))

	stats, err := analytics.ComputeDay(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}

	if !almostEqual(stats.ProgressIncrease, 15) {
		t.Errorf("expected an increase of 15 percentage points, got %v", stats.ProgressIncrease)
	}
}

func TestComputeDayDoesNotSubtractWhenABookIsRestarted(t *testing.T) {
	app, user := newApp(t)

	// Finished yesterday, started over today.
	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.05, at(9, 0))
	testutil.CreateHistoryEntry(t, app, document, "", 1.0, at(9, 0).AddDate(0, 0, -1))
	// A second book genuinely advanced today.
	testutil.CreateDocument(t, app, user, "", "hash-b", 0.3, at(11, 0))

	stats, err := analytics.ComputeDay(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}

	// The restarted book contributes 0, not -95 percentage points.
	if !almostEqual(stats.ProgressIncrease, 30) {
		t.Errorf("expected an increase of 30 percentage points, got %v", stats.ProgressIncrease)
	}
}

func TestComputeDayReadingTimeSumsShortGaps(t *testing.T) {
	app, user := newApp(t)

	// 10:00 -> 10:02 -> 10:05 counts as 5 minutes of reading.
	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.3, at(10, 5))
	testutil.CreateHistoryEntry(t, app, document, "", 0.1, at(10, 0))
	testutil.CreateHistoryEntry(t, app, document, "", 0.2, at(10, 2))

	stats, err := analytics.ComputeDay(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}

	if stats.ReadingTime != 300 {
		t.Errorf("expected 300 seconds of reading time, got %d", stats.ReadingTime)
	}
}

func TestComputeDayReadingTimeIgnoresLongGaps(t *testing.T) {
	app, user := newApp(t)

	// Two sessions of 2 minutes each, hours apart. The gap in between is not
	// reading, it is the rest of the day.
	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.4, at(20, 2))
	testutil.CreateHistoryEntry(t, app, document, "", 0.1, at(8, 0))
	testutil.CreateHistoryEntry(t, app, document, "", 0.2, at(8, 2))
	testutil.CreateHistoryEntry(t, app, document, "", 0.3, at(20, 0))

	stats, err := analytics.ComputeDay(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}

	if stats.ReadingTime != 240 {
		t.Errorf("expected 240 seconds of reading time, got %d", stats.ReadingTime)
	}
}

func TestComputeDayGapExactlyAtTheThresholdIsNotCounted(t *testing.T) {
	app, user := newApp(t)

	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 5))
	testutil.CreateHistoryEntry(t, app, document, "", 0.1, at(10, 0))

	stats, err := analytics.ComputeDay(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}

	if stats.ReadingTime != 0 {
		t.Errorf("expected a gap of exactly the session limit to be excluded, got %d seconds", stats.ReadingTime)
	}
}

func TestComputeDayIsScopedToOneUser(t *testing.T) {
	app, user := newApp(t)
	other := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)

	testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 0))
	testutil.CreateDocument(t, app, other, "", "hash-b", 0.9, at(11, 0))

	stats, err := analytics.ComputeDay(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}

	if stats.DocumentsTouched != 1 {
		t.Errorf("expected only the user's own document, got %d", stats.DocumentsTouched)
	}
	if !almostEqual(stats.ProgressIncrease, 20) {
		t.Errorf("expected 20 percentage points from the user's own reading, got %v", stats.ProgressIncrease)
	}
}

func TestComputeDayIsEmptyForADayWithoutReading(t *testing.T) {
	app, user := newApp(t)

	testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 0))

	stats, err := analytics.ComputeDay(app, user.Id, "2026-02-14", sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}

	if !stats.IsEmpty() {
		t.Errorf("expected an empty day, got %+v", stats)
	}
}

func TestRecomputeDayStoresAndUpdatesTheRow(t *testing.T) {
	app, user := newApp(t)

	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 0))

	if err := analytics.RecomputeDay(app, user.Id, dateOf(day), sessionGap); err != nil {
		t.Fatalf("failed to recompute: %v", err)
	}

	stored, err := app.FindFirstRecordByData(schema.CollectionReadingDays, schema.FieldDate, dateOf(day))
	if err != nil {
		t.Fatalf("expected the day to be stored: %v", err)
	}
	if got := stored.GetInt(schema.FieldUpdateCount); got != 1 {
		t.Errorf("expected 1 update, got %d", got)
	}
	if stored.GetDateTime(schema.FieldComputedAt).IsZero() {
		t.Errorf("expected computed_at to be set")
	}

	// Read some more and recompute the same day.
	testutil.CreateHistoryEntry(t, app, document, "", 0.2, at(10, 30))
	if err := analytics.RecomputeDay(app, user.Id, dateOf(day), sessionGap); err != nil {
		t.Fatalf("failed to recompute: %v", err)
	}

	days, err := app.FindAllRecords(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("failed to list the stored days: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("expected the day to be updated in place, got %d rows", len(days))
	}
	if got := days[0].GetInt(schema.FieldUpdateCount); got != 2 {
		t.Errorf("expected 2 updates after the second push, got %d", got)
	}
}

func TestRecomputeDayRemovesADayThatLostItsData(t *testing.T) {
	app, user := newApp(t)

	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 0))
	if err := analytics.RecomputeDay(app, user.Id, dateOf(day), sessionGap); err != nil {
		t.Fatalf("failed to recompute: %v", err)
	}

	if err := app.Delete(document); err != nil {
		t.Fatalf("failed to delete the document: %v", err)
	}
	if err := analytics.RecomputeDay(app, user.Id, dateOf(day), sessionGap); err != nil {
		t.Fatalf("failed to recompute: %v", err)
	}

	days, err := app.FindAllRecords(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("failed to list the stored days: %v", err)
	}
	if len(days) != 0 {
		t.Errorf("expected the empty day to be removed, got %d rows", len(days))
	}
}
