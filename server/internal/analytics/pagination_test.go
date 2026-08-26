//
// File:        internal/analytics/pagination_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics_test

import (
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/analytics"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// omnibusPages is longer than the progress estimator can ever reach: progress is
// reported to four decimals and a page of a book this long is narrower than that
// grid. It is the length of the reference omnibus, and the reason the count a
// device states is worth reading at all.
const omnibusPages = 3543

// syncStatistics uploads a statistics database stating a page count.
func syncStatistics(t testing.TB, app core.App, owner *core.Record, total int) {
	t.Helper()

	path := buildStatistics(t, testutil.DocumentHashA, []measuredPage{
		{testutil.DocumentHashA, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, time.UTC), 600, total},
		{testutil.DocumentHashA, 11, time.Date(2026, 8, 10, 20, 10, 0, 0, time.UTC), 540, total},
	})

	if _, err := analytics.ImportMeasurements(app, owner.Id, path); err != nil {
		t.Fatalf("import: %v", err)
	}
}

// reload returns the book as it is stored.
func reload(t testing.TB, app *tests.TestApp, book *core.Record) *core.Record {
	t.Helper()

	stored, err := app.FindRecordById(schema.CollectionBooks, book.Id)
	if err != nil {
		t.Fatalf("reload the book: %v", err)
	}

	return stored
}

// The bug this exists for: a book uploaded before it was read keeps the word
// count estimate for ever, because it is longer than the estimator can measure.
// The device has written its own count down all along.
func TestTheCountTheDeviceStatedIsStoredOnTheBook(t *testing.T) {
	app, user := newApp(t)
	book := testutil.CreateBook(t, app, user, "", "Die Witcher-Saga", testutil.DocumentHashA, "")

	syncStatistics(t, app, user, omnibusPages)

	stored := reload(t, app, book)
	if got := stored.GetInt(schema.FieldMeasuredPages); got != omnibusPages {
		t.Errorf("the book runs to %d pages, want the %d the device stated", got, omnibusPages)
	}
	if got := stored.GetString(schema.FieldMeasuredSource); got != schema.MeasuredByDevice {
		t.Errorf("the measurement says it came from %q", got)
	}
	// Which device wrote the file is not in the file, and a count labelled with
	// the wrong device would be worse than one labelled with none.
	if got := stored.GetString(schema.FieldMeasuredDevice); got != "" {
		t.Errorf("the count names %q as its device", got)
	}
	if stored.GetDateTime(schema.FieldMeasuredThrough).IsZero() {
		t.Error("the measurement does not say how far it looked")
	}
}

// Reading in a file nobody has uploaded is real reading with no book to put a
// page count on, and must not be an error either.
func TestAStatedCountWithoutABookIsNotAProblem(t *testing.T) {
	app, user := newApp(t)

	syncStatistics(t, app, user, omnibusPages)

	books, err := app.FindAllRecords(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("list the books: %v", err)
	}
	if len(books) != 0 {
		t.Errorf("an import invented %d books", len(books))
	}
}

// The precedence between the two measurements. The estimator reconstructs what
// the device states outright, so where the device has spoken there is nothing
// left to reconstruct.
func TestAnEstimateDoesNotReplaceTheCountTheDeviceStated(t *testing.T) {
	app, user := newApp(t)
	book := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", testutil.DocumentHashA, "")

	syncStatistics(t, app, user, 660)
	recordReading(t, app, user, book, testutil.DocumentHashA, "go7", pushes(700, 30), at(9, 0))

	measured, err := analytics.MeasurePageSize(app, reload(t, app, book))
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	if measured {
		t.Error("the estimator overwrote a count the device had stated")
	}

	stored := reload(t, app, book)
	if got := stored.GetInt(schema.FieldMeasuredPages); got != 660 {
		t.Errorf("the book runs to %d pages, want the stated 660", got)
	}
	if got := stored.GetString(schema.FieldMeasuredSource); got != schema.MeasuredByDevice {
		t.Errorf("the measurement says it came from %q", got)
	}
}

// And the other way round: a book no device has stated a count for is still
// estimated, and the estimate says that is what it is.
func TestAnEstimateSaysItIsOne(t *testing.T) {
	app, user := newApp(t)
	book := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", testutil.DocumentHashA, "")

	recordReading(t, app, user, book, testutil.DocumentHashA, "go7", pushes(700, 30), at(9, 0))

	if _, err := analytics.MeasurePageSize(app, book); err != nil {
		t.Fatalf("measure: %v", err)
	}

	stored := reload(t, app, book)
	if got := stored.GetInt(schema.FieldMeasuredPages); got != 700 {
		t.Errorf("the estimate is %d pages, want 700", got)
	}
	if got := stored.GetString(schema.FieldMeasuredSource); got != schema.MeasuredByProgress {
		t.Errorf("the measurement says it came from %q", got)
	}
}

// A device syncs its statistics every day and states the same count every day.
// Storing it again would put the whole library at the top of "recently updated"
// once a day for nothing.
func TestAnUnchangedCountLeavesTheBookAlone(t *testing.T) {
	app, user := newApp(t)
	book := testutil.CreateBook(t, app, user, "", "Die Witcher-Saga", testutil.DocumentHashA, "")

	syncStatistics(t, app, user, omnibusPages)
	first := reload(t, app, book).GetDateTime(schema.FieldUpdated)

	syncStatistics(t, app, user, omnibusPages)
	second := reload(t, app, book).GetDateTime(schema.FieldUpdated)

	if second.String() != first.String() {
		t.Errorf("the book was written again at %s, having been written at %s", second, first)
	}
}

// A font change repaginates the book, and the number a reader is shown has to
// follow the book they are actually holding.
func TestANewStatedCountReplacesTheOldOne(t *testing.T) {
	app, user := newApp(t)
	book := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", testutil.DocumentHashA, "")

	syncStatistics(t, app, user, 660)
	syncStatistics(t, app, user, 700)

	if got := reload(t, app, book).GetInt(schema.FieldMeasuredPages); got != 700 {
		t.Errorf("the book runs to %d pages, want the 700 it is paginated into now", got)
	}
}
