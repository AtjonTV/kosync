//
// File:        internal/analytics/pagination.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"fmt"
	"time"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/statistics"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// StorePagination writes the page counts a device stated onto the books they
// belong to, and reports how many books that changed.
//
// This is the better half of the two page measurements. The estimator recovers a
// device's page count from the size of the steps its progress moves in, which
// works up to roughly 1600 pages and then stops: progress is reported to four
// decimals, and a page of a 3500 page omnibus is narrower than that grid. The
// statistics database has no such limit, because the device wrote the number
// down instead of leaving it to be inferred.
//
// The device is not recorded with it. Which reader uploaded a statistics database
// is not in the file — the WebDAV credential may be shared by several — and a
// count labelled with the wrong device would be worse than one labelled with
// none, so the field is cleared rather than guessed at or left stale.
func StorePagination(app core.App, ownerId string, counts []statistics.Pagination) (int, error) {
	changed := 0

	for _, count := range counts {
		if count.Pages <= 0 {
			continue
		}

		book, err := books.FindForDocument(app, ownerId, count.Document)
		if err != nil {
			return changed, fmt.Errorf("find the book of document %s: %w", count.Document, err)
		}
		if book == nil {
			// Reading in a file nobody has uploaded. Real reading, no book to
			// put a page count on.
			continue
		}

		moment, err := types.ParseDateTime(time.Unix(count.Through, 0).UTC())
		if err != nil {
			return changed, fmt.Errorf("read the newest page turn of document %s: %w", count.Document, err)
		}

		if !applyPagination(book, count, moment) {
			continue
		}

		if err := app.Save(book); err != nil {
			return changed, fmt.Errorf("store the page count of book %s: %w", book.Id, err)
		}

		changed++
	}

	return changed, nil
}

// applyPagination sets the fields and reports whether anything moved.
//
// A device that syncs its statistics every day states the same count every day,
// and saving a book that has not changed would put every book in the library at
// the top of "recently updated" once a day for nothing.
func applyPagination(book *core.Record, count statistics.Pagination, moment types.DateTime) bool {
	same := book.GetInt(schema.FieldMeasuredPages) == count.Pages &&
		book.GetString(schema.FieldMeasuredSource) == schema.MeasuredByDevice
	if same {
		return false
	}

	book.Set(schema.FieldMeasuredPages, count.Pages)
	book.Set(schema.FieldMeasuredSource, schema.MeasuredByDevice)
	book.Set(schema.FieldMeasuredDevice, "")
	book.Set(schema.FieldMeasuredThrough, moment)

	return true
}
