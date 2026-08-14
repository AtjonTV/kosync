//
// File:        internal/analytics/bookhooks.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// registerBookHooks queues the days whose page counts a book changes.
//
// Uploading a book is the moment its reading becomes countable in pages, and
// that reading is usually months old: the ordinary enqueue hooks only ever see
// the day of the push that triggered them, so without this the pages of every
// earlier day would stay at zero until someone read that book again.
//
// Deleting one is the same event in reverse, and is why this listens for the
// link changing rather than for the book arriving: matching sets the link on the
// document, and PocketBase clears it again when the book goes away.
func registerBookHooks(app core.App) {
	app.OnRecordAfterUpdateSuccess(schema.CollectionDocuments).BindFunc(func(e *core.RecordEvent) error {
		original := e.Record.Original()
		if original != nil && original.GetString(schema.FieldBook) == e.Record.GetString(schema.FieldBook) {
			return e.Next()
		}

		enqueueDocumentDays(e.App, e.Record)

		return e.Next()
	})

	// The per-book rows of a deleted book are cascaded away by PocketBase, which
	// leaves the day totals counting pages of a book that no longer exists. The
	// dates have to be read before the delete, because afterwards the rows that
	// name them are gone.
	app.OnRecordDelete(schema.CollectionBooks).BindFunc(func(e *core.RecordEvent) error {
		owner := e.Record.GetString(schema.FieldOwner)
		dates := bookDates(e.App, e.Record.Id)

		if err := e.Next(); err != nil {
			return err
		}

		for _, date := range dates {
			enqueueQuietly(e.App, owner, date)
		}

		return nil
	})
}

// enqueueDocumentDays queues every day a document has ever been read on.
func enqueueDocumentDays(app core.App, document *core.Record) {
	owner := document.GetString(schema.FieldOwner)
	if owner == "" {
		return
	}

	dates := []struct {
		Date string `db:"date"`
	}{}

	err := app.DB().
		NewQuery(`
			SELECT DISTINCT substr([[last_read_at]], 1, 10) AS date
			FROM {{` + schema.CollectionDocuments + `}}
			WHERE [[id]] = {:document}
			UNION
			SELECT DISTINCT substr([[last_read_at]], 1, 10) AS date
			FROM {{` + schema.CollectionDocumentHistory + `}}
			WHERE [[document_ref]] = {:document}
		`).
		Bind(dbx.Params{"document": document.Id}).
		All(&dates)
	if err != nil {
		app.Logger().Warn("failed to list the days of a document",
			"document", document.Id, "error", err)

		return
	}

	for _, row := range dates {
		enqueueQuietly(app, owner, row.Date)
	}
}

// bookDates returns the days a book has stored statistics for.
func bookDates(app core.App, bookId string) []string {
	rows := []struct {
		Date string `db:"date"`
	}{}

	err := app.DB().
		Select("date").
		Distinct(true).
		From(schema.CollectionReadingBookDays).
		Where(dbx.HashExp{"book": bookId}).
		All(&rows)
	if err != nil {
		app.Logger().Warn("failed to list the days of a book", "book", bookId, "error", err)

		return nil
	}

	dates := make([]string, 0, len(rows))
	for _, row := range rows {
		dates = append(dates, row.Date)
	}

	return dates
}

// enqueueQuietly queues a day, logging rather than failing: a missed enqueue
// costs a stale row until the weekly reconcile, while a failure here would cost
// the write that triggered it.
func enqueueQuietly(app core.App, owner, date string) {
	if err := Enqueue(app, owner, date); err != nil {
		app.Logger().Warn("failed to queue a statistics recomputation",
			"owner", owner, "date", date, "error", err)
	}
}
