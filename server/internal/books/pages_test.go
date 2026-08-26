//
// File:        internal/books/pages_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books_test

import (
	"testing"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
)

// bookWith returns an unsaved book carrying the two page counts.
func bookWith(t testing.TB, app core.App, notional, measured int) *core.Record {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("failed to find the books collection: %v", err)
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldPageCount, notional)
	record.Set(schema.FieldMeasuredPages, measured)

	return record
}

func TestEffectivePagesPrefersTheMeasurement(t *testing.T) {
	app := testutil.NewApp(t)

	// On the reference books the word count fallback and the device's own count
	// differed by up to a third, and the device was the one that was right.
	count, source := books.EffectivePages(bookWith(t, app, 500, 700))

	if count != 700 {
		t.Errorf("expected the measured count, got %d", count)
	}
	if source != books.PageSourceMeasured {
		t.Errorf("expected the source to be %q, got %q", books.PageSourceMeasured, source)
	}
}

func TestEffectivePagesFallsBackToTheWordCount(t *testing.T) {
	app := testutil.NewApp(t)

	count, source := books.EffectivePages(bookWith(t, app, 500, 0))

	if count != 500 {
		t.Errorf("expected the notional count, got %d", count)
	}
	if source != books.PageSourceWords {
		t.Errorf("expected the source to be %q, got %q", books.PageSourceWords, source)
	}
}

func TestEffectivePagesReportsWhenThereIsNothing(t *testing.T) {
	app := testutil.NewApp(t)

	count, source := books.EffectivePages(bookWith(t, app, 0, 0))

	if count != 0 || source != books.PageSourceNone {
		t.Errorf("expected no page count, got %d from %q", count, source)
	}
}

func TestEffectivePagesHandlesAMissingBook(t *testing.T) {
	count, source := books.EffectivePages(nil)

	if count != 0 || source != books.PageSourceNone {
		t.Errorf("expected no page count, got %d from %q", count, source)
	}
}
