//
// File:        internal/analytics/bookdays_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics_test

import (
	"testing"

	"git.obth.eu/atjontv/kosync/internal/analytics"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// linkTo attaches a document to a book, the way matching does on a push.
func linkTo(t testing.TB, app core.App, document, book *core.Record) *core.Record {
	t.Helper()

	document.Set(schema.FieldBook, book.Id)
	if err := app.Save(document); err != nil {
		t.Fatalf("failed to link a document to a book: %v", err)
	}

	return document
}

// withPages gives a book a notional page count, as an upload would from the
// word count.
func withPages(t testing.TB, app core.App, book *core.Record, count int) *core.Record {
	t.Helper()

	book.Set(schema.FieldPageCount, count)
	if err := app.Save(book); err != nil {
		t.Fatalf("failed to set a page count: %v", err)
	}

	return book
}

// bookDayOf loads the stored row of one book on the fixture day.
func bookDayOf(t testing.TB, app *tests.TestApp, book *core.Record) *core.Record {
	t.Helper()

	records, err := app.FindAllRecords(schema.CollectionReadingBookDays)
	if err != nil {
		t.Fatalf("failed to list the per-book rows: %v", err)
	}

	for _, record := range records {
		if record.GetString(schema.FieldBook) == book.Id && record.GetString(schema.FieldDate) == dateOf(day) {
			return record
		}
	}

	return nil
}

func TestComputeBookDaysSplitsTheDayByBook(t *testing.T) {
	app, user := newApp(t)

	first := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", "")
	second := testutil.CreateBook(t, app, user, "", "Kreuzweg der Raben", "hash-b", "")

	linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-a", 0.30, at(10, 0)), first)
	linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-b", 0.10, at(20, 0)), second)

	stats, err := analytics.ComputeBookDays(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the book days: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected one row per book, got %d", len(stats))
	}

	byBook := map[string]analytics.BookDayStats{}
	for _, row := range stats {
		byBook[row.Book] = row
	}

	if got := byBook[first.Id].ProgressIncrease; !almostEqual(got, 30) {
		t.Errorf("expected 30 percentage points in the first book, got %v", got)
	}
	if got := byBook[second.Id].ProgressIncrease; !almostEqual(got, 10) {
		t.Errorf("expected 10 percentage points in the second book, got %v", got)
	}
}

// Progress in a document nobody has uploaded the file for still counts towards
// the day, but there is no book to attribute it to.
func TestComputeBookDaysIgnoresUnmatchedDocuments(t *testing.T) {
	app, user := newApp(t)

	testutil.CreateDocument(t, app, user, "", "hash-a", 0.30, at(10, 0))

	stats, err := analytics.ComputeBookDays(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the book days: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected no rows for an unmatched document, got %d", len(stats))
	}

	// The day itself is unaffected.
	dayStats, err := analytics.ComputeDay(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}
	if !almostEqual(dayStats.ProgressIncrease, 30) {
		t.Errorf("expected the day to still count the reading, got %v", dayStats.ProgressIncrease)
	}
}

// This is the §16.5 subtlety made into a test: the gap across a switch from one
// book to the other belongs to neither, so the book rows are allowed to add up
// to less than the day.
func TestComputeBookDaysReadingTimeDoesNotSpanASwitch(t *testing.T) {
	app, user := newApp(t)

	first := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", "")
	second := testutil.CreateBook(t, app, user, "", "Kreuzweg der Raben", "hash-b", "")

	// 10:00, 10:02 in the first book, then 10:04, 10:06 in the second. Every gap
	// is two minutes and under the session limit, so the day counts six minutes.
	one := linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-a", 0.2, at(10, 2)), first)
	testutil.CreateHistoryEntry(t, app, one, "", 0.1, at(10, 0))
	two := linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-b", 0.2, at(10, 6)), second)
	testutil.CreateHistoryEntry(t, app, two, "", 0.1, at(10, 4))

	dayStats, err := analytics.ComputeDay(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}
	if dayStats.ReadingTime != 360 {
		t.Fatalf("expected the day to count 360 seconds, got %d", dayStats.ReadingTime)
	}

	stats, err := analytics.ComputeBookDays(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the book days: %v", err)
	}

	total := 0
	for _, row := range stats {
		if row.ReadingTime != 120 {
			t.Errorf("expected 120 seconds in book %s, got %d", row.Book, row.ReadingTime)
		}
		total += row.ReadingTime
	}
	if total != 240 {
		t.Errorf("expected the books to account for 240 of the day's 360 seconds, got %d", total)
	}
}

func TestComputeBookDaysCountsPagesFromThePageCount(t *testing.T) {
	app, user := newApp(t)

	// A 500 page book read from 20% to 45% is 125 pages.
	book := withPages(t, app, testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", ""), 500)
	document := linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-a", 0.45, at(11, 0)), book)
	testutil.CreateHistoryEntry(t, app, document, "", 0.20, at(10, 0).AddDate(0, 0, -1))

	stats, err := analytics.ComputeBookDays(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the book days: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected one row, got %d", len(stats))
	}
	if stats[0].PagesRead != 125 {
		t.Errorf("expected 125 pages, got %d", stats[0].PagesRead)
	}
}

// A measured count is the reader's own pagination and beats the one the word
// count implies.
func TestComputeBookDaysPrefersTheMeasuredPageCount(t *testing.T) {
	app, user := newApp(t)

	book := withPages(t, app, testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", ""), 500)
	book.Set(schema.FieldMeasuredPages, 700)
	if err := app.Save(book); err != nil {
		t.Fatalf("failed to store the measurement: %v", err)
	}

	linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-a", 0.10, at(11, 0)), book)

	stats, err := analytics.ComputeBookDays(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the book days: %v", err)
	}
	if stats[0].PagesRead != 70 {
		t.Errorf("expected 70 measured pages rather than 50 notional ones, got %d", stats[0].PagesRead)
	}
}

func TestRecomputeDayStoresTheBookRowsAndSumsThePages(t *testing.T) {
	app, user := newApp(t)

	first := withPages(t, app, testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", ""), 500)
	second := withPages(t, app, testutil.CreateBook(t, app, user, "", "Kreuzweg der Raben", "hash-b", ""), 400)

	linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-a", 0.20, at(10, 0)), first)  // 100 pages
	linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-b", 0.25, at(20, 0)), second) // 100 pages

	if err := analytics.RecomputeDay(app, user.Id, dateOf(day), sessionGap); err != nil {
		t.Fatalf("failed to recompute: %v", err)
	}

	rows, err := app.FindAllRecords(schema.CollectionReadingBookDays)
	if err != nil {
		t.Fatalf("failed to list the per-book rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected one row per book, got %d", len(rows))
	}

	stored, err := app.FindFirstRecordByData(schema.CollectionReadingDays, schema.FieldDate, dateOf(day))
	if err != nil {
		t.Fatalf("expected the day to be stored: %v", err)
	}
	if got := stored.GetInt(schema.FieldPagesRead); got != 200 {
		t.Errorf("expected the day to sum both books to 200 pages, got %d", got)
	}
}

// A document that loses its book, or a book that loses its reading, must not
// leave a row behind claiming the pages are still there.
func TestRecomputeBookDaysRemovesRowsThatLostTheirReading(t *testing.T) {
	app, user := newApp(t)

	book := withPages(t, app, testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", ""), 500)
	document := linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-a", 0.20, at(10, 0)), book)

	if err := analytics.RecomputeDay(app, user.Id, dateOf(day), sessionGap); err != nil {
		t.Fatalf("failed to recompute: %v", err)
	}
	if bookDayOf(t, app, book) == nil {
		t.Fatalf("expected a per-book row to exist, the rest of the test proves nothing")
	}

	document.Set(schema.FieldBook, "")
	if err := app.Save(document); err != nil {
		t.Fatalf("failed to unlink the document: %v", err)
	}

	if err := analytics.RecomputeDay(app, user.Id, dateOf(day), sessionGap); err != nil {
		t.Fatalf("failed to recompute: %v", err)
	}

	if row := bookDayOf(t, app, book); row != nil {
		t.Errorf("expected the stale per-book row to be removed, got %v pages",
			row.GetInt(schema.FieldPagesRead))
	}

	stored, err := app.FindFirstRecordByData(schema.CollectionReadingDays, schema.FieldDate, dateOf(day))
	if err != nil {
		t.Fatalf("expected the day to survive: %v", err)
	}
	if got := stored.GetInt(schema.FieldPagesRead); got != 0 {
		t.Errorf("expected the day to fall back to 0 pages, got %d", got)
	}
}

// Nothing knows how long an unmatched document is, so its reading counts towards
// the day without inventing pages for it.
func TestRecomputeDayCountsNoPagesWithoutABook(t *testing.T) {
	app, user := newApp(t)

	testutil.CreateDocument(t, app, user, "", "hash-a", 0.40, at(10, 0))

	if err := analytics.RecomputeDay(app, user.Id, dateOf(day), sessionGap); err != nil {
		t.Fatalf("failed to recompute: %v", err)
	}

	stored, err := app.FindFirstRecordByData(schema.CollectionReadingDays, schema.FieldDate, dateOf(day))
	if err != nil {
		t.Fatalf("expected the day to be stored: %v", err)
	}
	if got := stored.GetInt(schema.FieldPagesRead); got != 0 {
		t.Errorf("expected no pages without a book, got %d", got)
	}
	if !almostEqual(stored.GetFloat(schema.FieldProgressIncrease), 40) {
		t.Errorf("expected the reading itself to still be counted")
	}
}

func TestComputeBookDaysIsScopedToOneUser(t *testing.T) {
	app, user := newApp(t)
	other := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)

	mine := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", "")
	theirs := testutil.CreateBook(t, app, other, "", "Zeit des Sturms", "hash-a", "")

	linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-a", 0.20, at(10, 0)), mine)
	linkTo(t, app, testutil.CreateDocument(t, app, other, "", "hash-a", 0.90, at(10, 0)), theirs)

	stats, err := analytics.ComputeBookDays(app, user.Id, dateOf(day), sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the book days: %v", err)
	}
	if len(stats) != 1 || stats[0].Book != mine.Id {
		t.Fatalf("expected only the user's own book, got %+v", stats)
	}
}

// Deleting a book takes its per-book rows with it. The reading itself lives in
// the documents and survives, which is why the day is requeued rather than
// rewritten in place.
func TestDeletingABookRemovesItsRowsAndQueuesTheDay(t *testing.T) {
	app, user := newApp(t)
	analytics.Register(app, testConfig())

	book := withPages(t, app, testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", ""), 500)
	linkTo(t, app, testutil.CreateDocument(t, app, user, "", "hash-a", 0.20, at(10, 0)), book)

	if err := analytics.RecomputeDay(app, user.Id, dateOf(day), sessionGap); err != nil {
		t.Fatalf("failed to recompute: %v", err)
	}
	clearQueue(t, app)

	if err := app.Delete(book); err != nil {
		t.Fatalf("failed to delete the book: %v", err)
	}

	rows, err := app.FindAllRecords(schema.CollectionReadingBookDays)
	if err != nil {
		t.Fatalf("failed to list the per-book rows: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected the per-book rows to be cascaded away, got %d", len(rows))
	}

	if !queued(t, app, user.Id, dateOf(day)) {
		t.Errorf("expected the day to be queued so its page count stops counting a deleted book")
	}
}

// Uploading a book is usually the last thing that happens to reading that is
// already months old, so every day it touched has to be recomputed.
func TestLinkingABookQueuesEveryDayTheDocumentWasReadOn(t *testing.T) {
	app, user := newApp(t)
	analytics.Register(app, testConfig())

	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.30, at(10, 0))
	testutil.CreateHistoryEntry(t, app, document, "", 0.10, at(10, 0).AddDate(0, 0, -3))
	testutil.CreateHistoryEntry(t, app, document, "", 0.20, at(10, 0).AddDate(0, 0, -1))
	clearQueue(t, app)

	book := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", "")
	linkTo(t, app, document, book)

	for _, offset := range []int{-3, -1, 0} {
		date := dateOf(day.AddDate(0, 0, offset))
		if !queued(t, app, user.Id, date) {
			t.Errorf("expected %s to be queued after the book was linked", date)
		}
	}
}

// queued reports whether a day is waiting to be recomputed.
func queued(t testing.TB, app *tests.TestApp, owner, date string) bool {
	t.Helper()

	records, err := app.FindAllRecords(schema.CollectionAnalyticsQueue)
	if err != nil {
		t.Fatalf("failed to list the queue: %v", err)
	}

	for _, record := range records {
		if record.GetString(schema.FieldOwner) == owner && record.GetString(schema.FieldDate) == date {
			return true
		}
	}

	return false
}

// clearQueue empties the queue so that a test can assert on what one action put
// back into it.
func clearQueue(t testing.TB, app *tests.TestApp) {
	t.Helper()

	records, err := app.FindAllRecords(schema.CollectionAnalyticsQueue)
	if err != nil {
		t.Fatalf("failed to list the queue: %v", err)
	}
	for _, record := range records {
		if err := app.Delete(record); err != nil {
			t.Fatalf("failed to empty the queue: %v", err)
		}
	}
}
