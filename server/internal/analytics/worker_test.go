//
// File:        internal/analytics/worker_test.go
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
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// testConfig returns a configuration with the documented defaults.
func testConfig() *config.Config {
	conf := &config.Config{}
	conf.Normalize()

	return conf
}

// newRegisteredApp returns an app with the statistics hooks wired up.
func newRegisteredApp(t testing.TB) (*tests.TestApp, *core.Record, *analytics.Worker) {
	t.Helper()

	app, user := newApp(t)
	worker := analytics.Register(app, testConfig())

	return app, user, worker
}

func queueLength(t testing.TB, app core.App) int {
	t.Helper()

	items, err := app.FindAllRecords(schema.CollectionAnalyticsQueue)
	if err != nil {
		t.Fatalf("failed to read the queue: %v", err)
	}

	return len(items)
}

func TestEnqueueIsIdempotentPerDay(t *testing.T) {
	app, user := newApp(t)

	for range 5 {
		if err := analytics.Enqueue(app, user.Id, "2026-03-01"); err != nil {
			t.Fatalf("failed to enqueue: %v", err)
		}
	}
	if err := analytics.Enqueue(app, user.Id, "2026-03-02"); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	if got := queueLength(t, app); got != 2 {
		t.Errorf("expected repeated pushes for one day to collapse into one item, got %d", got)
	}
}

func TestWritingProgressQueuesItsDay(t *testing.T) {
	app, user, _ := newRegisteredApp(t)

	testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 0))

	items, err := app.FindAllRecords(schema.CollectionAnalyticsQueue)
	if err != nil {
		t.Fatalf("failed to read the queue: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected the write to queue its day, got %d items", len(items))
	}
	if got := items[0].GetString(schema.FieldDate); got != dateOf(day) {
		t.Errorf("expected the queued day to be %q, got %q", dateOf(day), got)
	}
	if got := items[0].GetString(schema.FieldOwner); got != user.Id {
		t.Errorf("expected the queued owner to be %q, got %q", user.Id, got)
	}
}

func TestWorkerComputesQueuedDaysAndClearsTheQueue(t *testing.T) {
	app, user, worker := newRegisteredApp(t)

	testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 0))
	testutil.CreateDocument(t, app, user, "", "hash-b", 0.4, at(10, 0).AddDate(0, 0, -1))

	handled, err := worker.DrainAll()
	if err != nil {
		t.Fatalf("failed to drain the queue: %v", err)
	}
	if handled != 2 {
		t.Errorf("expected 2 queued days to be handled, got %d", handled)
	}

	if got := queueLength(t, app); got != 0 {
		t.Errorf("expected the queue to be empty afterwards, got %d items", got)
	}

	days, err := app.FindAllRecords(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("failed to list the stored days: %v", err)
	}
	if len(days) != 2 {
		t.Errorf("expected 2 stored statistics days, got %d", len(days))
	}
}

func TestDrainingAnEmptyQueueDoesNothing(t *testing.T) {
	app, _, worker := newRegisteredApp(t)

	handled, err := worker.DrainOnce()
	if err != nil {
		t.Fatalf("failed to drain the queue: %v", err)
	}
	if handled != 0 {
		t.Errorf("expected nothing to be handled, got %d", handled)
	}
	if got := queueLength(t, app); got != 0 {
		t.Errorf("expected the queue to stay empty, got %d items", got)
	}
}

func TestDeletingProgressQueuesTheDayAgain(t *testing.T) {
	app, user, worker := newRegisteredApp(t)

	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 0))
	if _, err := worker.DrainAll(); err != nil {
		t.Fatalf("failed to drain the queue: %v", err)
	}

	if err := app.Delete(document); err != nil {
		t.Fatalf("failed to delete the document: %v", err)
	}
	if got := queueLength(t, app); got != 1 {
		t.Fatalf("expected the deletion to queue its day, got %d items", got)
	}

	if _, err := worker.DrainAll(); err != nil {
		t.Fatalf("failed to drain the queue: %v", err)
	}

	days, err := app.FindAllRecords(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("failed to list the stored days: %v", err)
	}
	if len(days) != 0 {
		t.Errorf("expected the statistics of the emptied day to be removed, got %d rows", len(days))
	}
}

func TestWorkerStartAndStopAreSafeToRepeat(t *testing.T) {
	_, _, worker := newRegisteredApp(t)

	worker.Start()
	worker.Start() // must not start a second loop
	worker.Stop()
	worker.Stop() // must not block or panic

	// Starting again after a stop has to work, because that is what a restart
	// of the serve command does.
	worker.Start()
	worker.Stop()
}

func TestWorkerDrainsWhileRunning(t *testing.T) {
	app, user := newApp(t)

	conf := testConfig()
	conf.AnalyticsWorkerInterval = 1
	worker := analytics.Register(app, conf)

	testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 0))

	worker.Start()
	defer worker.Stop()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		days, err := app.FindAllRecords(schema.CollectionReadingDays)
		if err != nil {
			t.Fatalf("failed to list the stored days: %v", err)
		}
		if len(days) == 1 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Errorf("expected the running worker to compute the queued day within 10s")
}
