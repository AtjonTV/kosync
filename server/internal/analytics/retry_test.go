//
// File:        internal/analytics/retry_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics_test

import (
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// queueAnUncomputableDay puts a row on the queue that no drain can ever finish.
//
// The date is not a date, so resolving the day fails every time, which is the
// only kind of failure a test can rely on. It goes in through a plain insert
// because the field's own pattern would refuse it — the point is to stand in for
// whatever real breakage puts a row beyond recomputation, not to claim this
// particular row can occur.
//
// It is created an hour before everything else so that the queue, which is
// drained oldest first, hands it out ahead of the good days behind it.
func queueAnUncomputableDay(t testing.TB, app core.App, ownerId string) string {
	t.Helper()

	id := core.GenerateDefaultRandomId()

	_, err := app.DB().
		NewQuery("INSERT INTO {{" + schema.CollectionAnalyticsQueue + "}}" +
			" ([[id]], [[owner]], [[date]], [[created]], [[attempts]], [[retry_after]])" +
			" VALUES ({:id}, {:owner}, 'not-a-date', {:created}, 0, '')").
		Bind(dbx.Params{
			"id":      id,
			"owner":   ownerId,
			"created": time.Now().UTC().Add(-time.Hour).Format("2006-01-02 15:04:05.000Z"),
		}).
		Execute()
	if err != nil {
		t.Fatalf("failed to queue the uncomputable day: %v", err)
	}

	return id
}

// queueItem reads one row of the queue back.
func queueItem(t testing.TB, app core.App, id string) *core.Record {
	t.Helper()

	record, err := app.FindRecordById(schema.CollectionAnalyticsQueue, id)
	if err != nil {
		t.Fatalf("failed to read the queued item %q: %v", id, err)
	}

	return record
}

// The bug this covers: the queue is drained oldest first and an item leaves it
// only once it has been recomputed, so a day that always fails used to be the
// day that was always tried — taking a place in every batch and hiding the days
// behind it for as long as the server ran.
func TestAFailingDayDoesNotBlockTheOnesBehindIt(t *testing.T) {
	app, user, worker := newRegisteredApp(t)

	stuck := queueAnUncomputableDay(t, app, user.Id)

	// Queued after it, and therefore behind it.
	testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 0))

	handled, err := worker.DrainAll()
	if err != nil {
		t.Fatalf("failed to drain the queue: %v", err)
	}
	if handled != 1 {
		t.Errorf("expected the good day to be recomputed, got %d handled", handled)
	}

	days, err := app.FindAllRecords(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("failed to list the stored days: %v", err)
	}
	if len(days) != 1 {
		t.Errorf("expected 1 stored statistics day, got %d", len(days))
	}

	// The stuck one is still owed, and is now waiting rather than spinning.
	item := queueItem(t, app, stuck)
	if got := item.GetInt(schema.FieldAttempts); got != 1 {
		t.Errorf("expected 1 recorded attempt, got %d", got)
	}
	if item.GetDateTime(schema.FieldRetryAfter).IsZero() {
		t.Errorf("expected the failed day to be put down until a later moment")
	}
}

// A deferred item is not a candidate until its moment comes, so the very next
// pass has nothing to do rather than trying it again.
func TestAPostponedDayIsNotClaimedAgainImmediately(t *testing.T) {
	app, user, worker := newRegisteredApp(t)

	stuck := queueAnUncomputableDay(t, app, user.Id)

	if _, err := worker.DrainOnce(); err != nil {
		t.Fatalf("failed to drain the queue: %v", err)
	}
	if got := queueLength(t, app); got != 1 {
		t.Fatalf("expected the failed day to stay queued, got %d items", got)
	}

	// A second pass right away must not touch it, which shows as the attempt
	// count standing still.
	if _, err := worker.DrainOnce(); err != nil {
		t.Fatalf("failed to drain the queue again: %v", err)
	}
	if got := queueItem(t, app, stuck).GetInt(schema.FieldAttempts); got != 1 {
		t.Errorf("expected the attempt count to stay at 1, got %d", got)
	}
}

// And once the backoff has run out it is tried again, because the failure is
// usually not the day's fault and the reading it describes is still owed.
func TestAPostponedDayIsTriedAgainWhenItsMomentComes(t *testing.T) {
	app, user, worker := newRegisteredApp(t)

	stuck := queueAnUncomputableDay(t, app, user.Id)

	if _, err := worker.DrainOnce(); err != nil {
		t.Fatalf("failed to drain the queue: %v", err)
	}

	// Bring the moment forward instead of waiting a minute for it. A plain
	// update again, because saving the record would validate the very date that
	// makes it uncomputable.
	_, err := app.DB().
		NewQuery("UPDATE {{" + schema.CollectionAnalyticsQueue + "}}" +
			" SET [[retry_after]] = {:past} WHERE [[id]] = {:id}").
		Bind(dbx.Params{
			"past": time.Now().UTC().Add(-time.Minute).Format("2006-01-02 15:04:05.000Z"),
			"id":   stuck,
		}).
		Execute()
	if err != nil {
		t.Fatalf("failed to bring the retry forward: %v", err)
	}

	if _, err := worker.DrainOnce(); err != nil {
		t.Fatalf("failed to drain the queue again: %v", err)
	}
	if got := queueItem(t, app, stuck).GetInt(schema.FieldAttempts); got != 2 {
		t.Errorf("expected a second attempt to have been made, got %d", got)
	}
}
