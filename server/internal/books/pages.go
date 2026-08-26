//
// File:        internal/books/pages.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books

import (
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// Where a book's page count came from.
const (
	// PageSourceMeasured means it is a device's own count: either stated by the
	// statistics database a device synced here, or recovered from the progress
	// it pushed. Which of the two is on the record — see
	// schema.FieldMeasuredSource — matters when deciding whether to measure
	// again, and not at all when reckoning in the number.
	PageSourceMeasured = "measured"

	// PageSourceWords means it was derived from the word count at the configured
	// density, because no measurement was possible.
	PageSourceWords = "words"

	// PageSourceNone means the book has no usable page count at all.
	PageSourceNone = "none"
)

// EffectivePages returns the page count to reckon in, and where it came from.
//
// A measurement wins whenever there is one: it is what the reader itself
// paginated the book into, while the word count fallback assumes a density that
// varies by a third between books on the same device.
func EffectivePages(book *core.Record) (int, string) {
	if book == nil {
		return 0, PageSourceNone
	}

	if measured := book.GetInt(schema.FieldMeasuredPages); measured > 0 {
		return measured, PageSourceMeasured
	}

	if notional := book.GetInt(schema.FieldPageCount); notional > 0 {
		return notional, PageSourceWords
	}

	return 0, PageSourceNone
}
