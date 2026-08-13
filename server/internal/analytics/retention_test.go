//
// File:        internal/analytics/retention_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics_test

import (
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/analytics"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// now is the moment the retention tests pretend to run at.
var now = time.Date(2026, 6, 1, 4, 0, 0, 0, time.UTC)

// storeDay writes a statistics day directly, bypassing the computation.
func storeDay(t testing.TB, app core.App, owner *core.Record, date string, updates, readingTime int, increase float64) *core.Record {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("failed to find the reading_days collection: %v", err)
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldOwner, owner.Id)
	record.Set(schema.FieldDate, date)
	record.Set(schema.FieldUpdateCount, updates)
	record.Set(schema.FieldReadingTime, readingTime)
	record.Set(schema.FieldProgressIncrease, increase)

	if err := app.Save(record); err != nil {
		t.Fatalf("failed to store the statistics day %q: %v", date, err)
	}

	return record
}

func findMonthRecord(t testing.TB, app core.App, owner *core.Record, month string) *core.Record {
	t.Helper()

	record, err := app.FindFirstRecordByFilter(
		schema.CollectionReadingMonths,
		"owner = {:owner} && month = {:month}",
		dbx.Params{"owner": owner.Id, "month": month},
	)
	if err != nil {
		t.Fatalf("expected a rollup for %q: %v", month, err)
	}

	return record
}

func TestRetentionAggregatesAgedOutDays(t *testing.T) {
	app, user := newApp(t)

	conf := testConfig()
	conf.AnalyticsRetentionDays = 30
	conf.AnalyticsRetentionMode = config.RetentionModeAggregate

	// Two days in January, well outside the window.
	storeDay(t, app, user, "2026-01-05", 10, 600, 5)
	storeDay(t, app, user, "2026-01-06", 4, 300, 2.5)
	// One day inside the window.
	kept := storeDay(t, app, user, "2026-05-30", 7, 120, 1)

	removed, err := analytics.ApplyRetention(app, conf, now)
	if err != nil {
		t.Fatalf("failed to apply the retention: %v", err)
	}
	if removed != 2 {
		t.Errorf("expected 2 aged out days, got %d", removed)
	}

	days, err := app.FindAllRecords(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("failed to list the stored days: %v", err)
	}
	if len(days) != 1 || days[0].Id != kept.Id {
		t.Fatalf("expected only the recent day to remain, got %d rows", len(days))
	}

	month := findMonthRecord(t, app, user, "2026-01")
	if got := month.GetInt(schema.FieldUpdateCount); got != 14 {
		t.Errorf("expected 14 updates in the rollup, got %d", got)
	}
	if got := month.GetInt(schema.FieldReadingTime); got != 900 {
		t.Errorf("expected 900 seconds in the rollup, got %d", got)
	}
	if !almostEqual(month.GetFloat(schema.FieldProgressIncrease), 7.5) {
		t.Errorf("expected 7.5 percentage points in the rollup, got %v", month.GetFloat(schema.FieldProgressIncrease))
	}
	if got := month.GetInt(schema.FieldDaysActive); got != 2 {
		t.Errorf("expected 2 active days in the rollup, got %d", got)
	}
}

func TestRetentionAddsToAnExistingRollup(t *testing.T) {
	app, user := newApp(t)

	conf := testConfig()
	conf.AnalyticsRetentionDays = 30

	storeDay(t, app, user, "2026-01-05", 10, 600, 5)
	if _, err := analytics.ApplyRetention(app, conf, now); err != nil {
		t.Fatalf("failed to apply the retention: %v", err)
	}

	// A day of the same month ages out later, for example after an import.
	storeDay(t, app, user, "2026-01-20", 3, 60, 1)
	if _, err := analytics.ApplyRetention(app, conf, now); err != nil {
		t.Fatalf("failed to apply the retention: %v", err)
	}

	month := findMonthRecord(t, app, user, "2026-01")
	if got := month.GetInt(schema.FieldUpdateCount); got != 13 {
		t.Errorf("expected the second fold to add to the first, got %d updates", got)
	}
	if got := month.GetInt(schema.FieldDaysActive); got != 2 {
		t.Errorf("expected 2 active days, got %d", got)
	}

	months, err := app.FindAllRecords(schema.CollectionReadingMonths)
	if err != nil {
		t.Fatalf("failed to list the rollups: %v", err)
	}
	if len(months) != 1 {
		t.Errorf("expected a single rollup row per month, got %d", len(months))
	}
}

func TestRetentionInDeleteModeKeepsNoRollup(t *testing.T) {
	app, user := newApp(t)

	conf := testConfig()
	conf.AnalyticsRetentionDays = 30
	conf.AnalyticsRetentionMode = config.RetentionModeDelete

	storeDay(t, app, user, "2026-01-05", 10, 600, 5)

	removed, err := analytics.ApplyRetention(app, conf, now)
	if err != nil {
		t.Fatalf("failed to apply the retention: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 aged out day, got %d", removed)
	}

	months, err := app.FindAllRecords(schema.CollectionReadingMonths)
	if err != nil {
		t.Fatalf("failed to list the rollups: %v", err)
	}
	if len(months) != 0 {
		t.Errorf("expected no rollup in delete mode, got %d rows", len(months))
	}
}

func TestRetentionKeepsTheBoundaryDay(t *testing.T) {
	app, user := newApp(t)

	conf := testConfig()
	conf.AnalyticsRetentionDays = 30

	// Exactly 30 days before "now" is the oldest day still inside the window.
	boundary := now.AddDate(0, 0, -30).Format(analytics.DateLayout)
	storeDay(t, app, user, boundary, 1, 60, 1)

	removed, err := analytics.ApplyRetention(app, conf, now)
	if err != nil {
		t.Fatalf("failed to apply the retention: %v", err)
	}
	if removed != 0 {
		t.Errorf("expected the boundary day to be kept, %d days were removed", removed)
	}
}

func TestRetentionIsScopedPerUser(t *testing.T) {
	app, user := newApp(t)
	other := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)

	conf := testConfig()
	conf.AnalyticsRetentionDays = 30

	storeDay(t, app, user, "2026-01-05", 10, 600, 5)
	storeDay(t, app, other, "2026-01-05", 2, 60, 1)

	if _, err := analytics.ApplyRetention(app, conf, now); err != nil {
		t.Fatalf("failed to apply the retention: %v", err)
	}

	first := findMonthRecord(t, app, user, "2026-01")
	second := findMonthRecord(t, app, other, "2026-01")

	if got := first.GetInt(schema.FieldUpdateCount); got != 10 {
		t.Errorf("expected 10 updates for the first user, got %d", got)
	}
	if got := second.GetInt(schema.FieldUpdateCount); got != 2 {
		t.Errorf("expected 2 updates for the second user, got %d", got)
	}
}

func TestReconcileQueuesRecentDays(t *testing.T) {
	app, user := newApp(t)

	conf := testConfig()
	conf.AnalyticsReconcileDays = 7

	recent := now.AddDate(0, 0, -2)
	old := now.AddDate(0, 0, -40)
	testutil.CreateDocument(t, app, user, "", "hash-recent", 0.2, recent)
	testutil.CreateDocument(t, app, user, "", "hash-old", 0.4, old)

	queued, err := analytics.Reconcile(app, conf, now)
	if err != nil {
		t.Fatalf("failed to reconcile: %v", err)
	}
	if queued != 1 {
		t.Fatalf("expected only the recent day to be queued, got %d", queued)
	}

	items, err := app.FindAllRecords(schema.CollectionAnalyticsQueue)
	if err != nil {
		t.Fatalf("failed to read the queue: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 queued day, got %d", len(items))
	}
	if got := items[0].GetString(schema.FieldDate); got != recent.Format(analytics.DateLayout) {
		t.Errorf("expected the recent day %q to be queued, got %q", recent.Format(analytics.DateLayout), got)
	}
}
