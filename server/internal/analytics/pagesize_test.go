//
// File:        internal/analytics/pagesize_test.go
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
)

// pushes returns the progress values a device would report while reading a book
// of the given length.
//
// Every push covers two pages, which is KOReader's recommended setting, except
// for every seventh, which covers one. Those single steps are what a chapter
// ending mid-page produces on real data, and they are what tells a two-page
// quantum apart from a book of half the length: without them the series is
// equally well explained by pages twice as long.
//
// The values are rounded to four decimals because that is all the sync protocol
// carries, and that rounding is the largest source of noise the estimator has to
// survive.
func pushes(pages, count int) []float64 {
	series := make([]float64, 0, count)
	read := 0

	for index := range count {
		series = append(series, math.Round(float64(read)/float64(pages)*10000)/10000)
		if index%7 == 6 {
			read++
		} else {
			read += 2
		}
	}

	return series
}

// shifted moves a series to start where another one left off, which is what a
// setting changed part way through a book looks like: the position carries on,
// only the step size changes.
func shifted(series []float64, offset float64) []float64 {
	moved := make([]float64, 0, len(series))
	for _, progress := range series {
		moved = append(moved, progress+offset)
	}

	return moved
}

// recordReading stores a series of progress values as one document's history,
// pushed by the named device.
func recordReading(t testing.TB, app core.App, user, book *core.Record, hash, device string, series []float64, start time.Time) *core.Record {
	t.Helper()

	last := len(series) - 1
	document := testutil.CreateDocument(t, app, user, "", hash, series[last], start.Add(time.Duration(last)*time.Minute))
	document.Set(schema.FieldBook, book.Id)
	document.Set(schema.FieldLastDeviceId, device)
	if err := app.Save(document); err != nil {
		t.Fatalf("failed to store the document: %v", err)
	}

	storeHistory(t, app, document, device, series[:last], start)

	return document
}

// continueReading appends more reading to a document that already has some.
func continueReading(t testing.TB, app core.App, document *core.Record, device string, series []float64, start time.Time) {
	t.Helper()

	// What the document holds now is no longer the newest position, so it moves
	// into the history the way a push would move it.
	storeHistory(t, app, document, device,
		[]float64{document.GetFloat(schema.FieldProgress)},
		document.GetDateTime(schema.FieldLastReadAt).Time())

	last := len(series) - 1
	storeHistory(t, app, document, device, series[:last], start)

	document.Set(schema.FieldProgress, series[last])
	document.Set(schema.FieldLastReadAt, start.Add(time.Duration(last)*time.Minute))
	if err := app.Save(document); err != nil {
		t.Fatalf("failed to advance the document: %v", err)
	}
}

// storeHistory writes a series as superseded positions, a minute apart.
func storeHistory(t testing.TB, app core.App, document *core.Record, device string, series []float64, start time.Time) {
	t.Helper()

	for index, progress := range series {
		entry := testutil.CreateHistoryEntry(t, app, document, "", progress, start.Add(time.Duration(index)*time.Minute))
		entry.Set(schema.FieldLastDeviceId, device)
		if err := app.Save(entry); err != nil {
			t.Fatalf("failed to store a history entry: %v", err)
		}
	}
}

// The number the estimator is asked for is the device's own, and this is the
// same measurement that reproduced 700 and 563 exactly on real books.
func TestMeasurePageSizeRecoversTheDevicePageCount(t *testing.T) {
	app, user := newApp(t)

	book := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", "")
	recordReading(t, app, user, book, "hash-a", "go7", pushes(700, 30), at(9, 0))

	measured, err := analytics.MeasurePageSize(app, book)
	if err != nil {
		t.Fatalf("failed to measure: %v", err)
	}
	if !measured {
		t.Fatalf("expected a measurement from 30 pushes")
	}

	stored, err := app.FindRecordById(schema.CollectionBooks, book.Id)
	if err != nil {
		t.Fatalf("failed to reload the book: %v", err)
	}
	if got := stored.GetInt(schema.FieldMeasuredPages); got != 700 {
		t.Errorf("expected 700 pages, got %d", got)
	}
	if got := stored.GetString(schema.FieldMeasuredDevice); got != "go7" {
		t.Errorf("expected the measurement to name the device, got %q", got)
	}
	if stored.GetDateTime(schema.FieldMeasuredThrough).IsZero() {
		t.Errorf("expected measured_at to be set")
	}
}

// A book read on a phone and on an e-reader has two page counts, both right for
// their own device. The statistics need one, and the device that pushed most is
// the one whose pages were actually turned.
func TestMeasurePageSizePrefersTheDeviceWithMoreEvidence(t *testing.T) {
	app, user := newApp(t)

	book := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", "")
	recordReading(t, app, user, book, "hash-a", "go7", pushes(700, 30), at(9, 0))
	recordReading(t, app, user, book, "hash-b", "els-n39", pushes(400, 16), at(14, 0))

	if _, err := analytics.MeasurePageSize(app, book); err != nil {
		t.Fatalf("failed to measure: %v", err)
	}

	stored, err := app.FindRecordById(schema.CollectionBooks, book.Id)
	if err != nil {
		t.Fatalf("failed to reload the book: %v", err)
	}
	if got := stored.GetString(schema.FieldMeasuredDevice); got != "go7" {
		t.Errorf("expected the device with more pushes, got %q", got)
	}
	if got := stored.GetInt(schema.FieldMeasuredPages); got != 700 {
		t.Errorf("expected that device's page count, got %d", got)
	}
}

// Two of the five reference books were read before the server existed and have
// a single history row each. Nothing recovers a page count from that, and
// guessing one would be worse than the word count fallback.
func TestMeasurePageSizeDeclinesWithoutEnoughPushes(t *testing.T) {
	app, user := newApp(t)

	book := testutil.CreateBook(t, app, user, "", "Der letzte Wunsch", "hash-a", "")
	recordReading(t, app, user, book, "hash-a", "go7", pushes(619, 4), at(9, 0))

	measured, err := analytics.MeasurePageSize(app, book)
	if err != nil {
		t.Fatalf("failed to measure: %v", err)
	}
	if measured {
		t.Errorf("expected no measurement from four pushes")
	}

	stored, err := app.FindRecordById(schema.CollectionBooks, book.Id)
	if err != nil {
		t.Fatalf("failed to reload the book: %v", err)
	}
	if got := stored.GetInt(schema.FieldMeasuredPages); got != 0 {
		t.Errorf("expected no stored page count, got %d", got)
	}
}

// Recomputing forty days of one book's reading must not measure it forty times.
func TestMeasurePageSizeSkipsWhenNothingNewWasRead(t *testing.T) {
	app, user := newApp(t)

	book := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", "")
	recordReading(t, app, user, book, "hash-a", "go7", pushes(700, 30), at(9, 0))

	if measured, err := analytics.MeasurePageSize(app, book); err != nil || !measured {
		t.Fatalf("expected the first measurement to be taken: %v %v", measured, err)
	}

	measured, err := analytics.MeasurePageSize(app, book)
	if err != nil {
		t.Fatalf("failed to measure: %v", err)
	}
	if measured {
		t.Errorf("expected the second pass to leave the measurement alone")
	}
}

// The whole point of measuring rather than configuring: change the font, the
// quantum shifts, and so does the estimate. A series spanning the change fits
// neither pagination, so the recent end has to be tried on its own or the book
// would keep its old number for good.
func TestMeasurePageSizeFollowsAChangedFont(t *testing.T) {
	app, user := newApp(t)

	book := testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", "")
	early := pushes(700, 30)
	document := recordReading(t, app, user, book, "hash-a", "go7", early, at(9, 0))

	if measured, err := analytics.MeasurePageSize(app, book); err != nil || !measured {
		t.Fatalf("expected the first measurement: %v %v", measured, err)
	}
	if got := book.GetInt(schema.FieldMeasuredPages); got != 700 {
		t.Fatalf("expected 700 pages before the change, got %d", got)
	}

	// A month later, same book and same device, in a larger font: more pages, so
	// smaller steps, carrying on from where the reading stopped.
	continueReading(t, app, document, "go7",
		shifted(pushes(900, 40), early[len(early)-1]), at(9, 0).AddDate(0, 0, 30))

	if measured, err := analytics.MeasurePageSize(app, book); err != nil || !measured {
		t.Fatalf("expected a fresh measurement after the change: %v %v", measured, err)
	}

	stored, err := app.FindRecordById(schema.CollectionBooks, book.Id)
	if err != nil {
		t.Fatalf("failed to reload the book: %v", err)
	}
	if got := stored.GetInt(schema.FieldMeasuredPages); got != 900 {
		t.Errorf("expected the new pagination, got %d", got)
	}
}

// RecomputeDay takes the measurement itself, so a day's pages are reckoned in
// the count that day's own pushes imply.
func TestRecomputeDayMeasuresTheBooksItCounts(t *testing.T) {
	app, user := newApp(t)

	// The word count would put this at 500 pages; the device says 700.
	book := withPages(t, app, testutil.CreateBook(t, app, user, "", "Zeit des Sturms", "hash-a", ""), 500)
	series := pushes(700, 30)
	recordReading(t, app, user, book, "hash-a", "go7", series, at(9, 0))

	if err := analytics.RecomputeDay(app, user.Id, dateOf(day), sessionGap); err != nil {
		t.Fatalf("failed to recompute: %v", err)
	}

	stored, err := app.FindRecordById(schema.CollectionBooks, book.Id)
	if err != nil {
		t.Fatalf("failed to reload the book: %v", err)
	}
	if got := stored.GetInt(schema.FieldMeasuredPages); got != 700 {
		t.Fatalf("expected the recomputation to measure the book, got %d", got)
	}

	row := bookDayOf(t, app, book)
	if row == nil {
		t.Fatalf("expected a per-book row")
	}

	// The series ends short of the last page, so the count is what the progress
	// covers at 700 pages rather than the whole book.
	want := int(math.Round(series[len(series)-1] * 700))
	if got := row.GetInt(schema.FieldPagesRead); got != want {
		t.Errorf("expected %d pages at the measured length, got %d", want, got)
	}
}
