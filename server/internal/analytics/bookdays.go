//
// File:        internal/analytics/bookdays.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"fmt"
	"math"
	"time"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// BookDayStats is one precomputed day of reading in one book.
type BookDayStats struct {
	Book             string  `db:"book"`
	UpdateCount      int     `db:"update_count"`
	ProgressIncrease float64 `db:"progress_increase"`
	ProgressFraction float64 `db:"progress_fraction"`
	ReadingTime      int     `db:"reading_time"`
	DocumentsTouched int     `db:"documents_touched"`

	// PagesRead is filled in afterwards, from the book's page count.
	PagesRead int
}

// bookDayStatsQuery is dayStatsQuery keyed by book as well as by day.
//
// The differences from the day query are worth stating, because they are the
// whole reason this is a second query rather than a GROUP BY on the first:
//
//   - Only documents matched to a book take part. Progress in a document nobody
//     has uploaded the file for cannot be attributed to a book, and is counted in
//     the day row alone.
//   - The reading time is summed per book, so a gap between a push in one book
//     and a push in another falls outside both windows and is counted in
//     neither. That is why the book rows do not add up to the day row, and why
//     forcing them to would mean inventing an owner for time spent switching.
//   - progress_fraction is the same number as progress_increase before it is
//     turned into percentage points; the page count is reckoned from it.
const bookDayStatsQuery = `
	WITH all_states AS (
		SELECT
			d.[[id]] AS document_id,
			d.[[book]] AS book_id,
			d.[[progress]] AS progress,
			d.[[last_read_at]] AS last_read_at
		FROM {{` + schema.CollectionDocuments + `}} d
		WHERE d.[[owner]] = {:owner} AND d.[[book]] != ''
		UNION ALL
		SELECT
			h.[[document_ref]] AS document_id,
			d.[[book]] AS book_id,
			h.[[progress]] AS progress,
			h.[[last_read_at]] AS last_read_at
		FROM {{` + schema.CollectionDocumentHistory + `}} h
		JOIN {{` + schema.CollectionDocuments + `}} d ON d.[[id]] = h.[[document_ref]]
		WHERE h.[[owner]] = {:owner} AND d.[[book]] != ''
	),
	day_states AS (
		SELECT * FROM all_states WHERE last_read_at >= {:start} AND last_read_at < {:end}
	),
	per_document AS (
		SELECT
			book_id,
			document_id,
			MAX(progress) AS max_progress,
			COUNT(DISTINCT last_read_at) AS update_count,
			MIN(last_read_at) AS first_of_day
		FROM day_states
		GROUP BY book_id, document_id
	),
	with_previous AS (
		SELECT
			p.book_id,
			p.max_progress,
			p.update_count,
			(
				SELECT MAX(a.progress)
				FROM all_states a
				WHERE a.document_id = p.document_id AND a.last_read_at < p.first_of_day
			) AS previous_progress
		FROM per_document p
	),
	per_book AS (
		SELECT
			book_id,
			SUM(update_count) AS update_count,
			SUM(MAX(max_progress - COALESCE(previous_progress, 0), 0)) AS progress_fraction,
			COUNT(*) AS documents_touched
		FROM with_previous
		GROUP BY book_id
	),
	moments AS (
		SELECT
			book_id,
			CAST(round((julianday(last_read_at) - 2440587.5) * 86400000.0) AS INTEGER) AS moment
		FROM day_states
	),
	deltas AS (
		SELECT
			book_id,
			moment - LAG(moment) OVER (PARTITION BY book_id ORDER BY moment) AS delta
		FROM moments
	),
	times AS (
		SELECT
			book_id,
			SUM(CASE WHEN delta > 0 AND delta < {:gap} THEN delta ELSE 0 END) AS milliseconds
		FROM deltas
		GROUP BY book_id
	)
	SELECT
		b.book_id AS book,
		b.update_count AS update_count,
		b.progress_fraction * 100 AS progress_increase,
		b.progress_fraction AS progress_fraction,
		b.documents_touched AS documents_touched,
		CAST(COALESCE(t.milliseconds, 0) / 1000 AS INTEGER) AS reading_time
	FROM per_book b
	LEFT JOIN times t ON t.book_id = b.book_id
	ORDER BY b.book_id
`

// ComputeBookDays calculates one day of reading, split by book, without storing
// anything. Books with no page count contribute no pages, not a guess.
func ComputeBookDays(app core.App, ownerId, date string, sessionGap time.Duration) ([]BookDayStats, error) {
	rows := []BookDayStats{}

	params, err := dayBounds(app, ownerId, date)
	if err != nil {
		return nil, fmt.Errorf("resolve the day %s of %s: %w", date, ownerId, err)
	}
	params["gap"] = sessionGap.Milliseconds()

	err = app.DB().
		NewQuery(bookDayStatsQuery).
		Bind(params).
		All(&rows)
	if err != nil {
		return nil, fmt.Errorf("compute book statistics for %s on %s: %w", ownerId, date, err)
	}

	for index := range rows {
		book, err := app.FindRecordById(schema.CollectionBooks, rows[index].Book)
		if err != nil {
			// The book went away between the query and here. Its rows are about
			// to be cascaded off anyway.
			continue
		}

		total, _ := books.EffectivePages(book)
		rows[index].PagesRead = int(math.Round(rows[index].ProgressFraction * float64(total)))
	}

	return rows, nil
}

// RecomputeBookDays recalculates one day of every book and stores the result,
// returning the pages read across all of them.
//
// Unlike the reading time, the pages do add up: every page is read in exactly
// one book, so the day's total is the sum of its books and nothing is lost
// between them.
func RecomputeBookDays(app core.App, ownerId, date string, sessionGap time.Duration) (int, error) {
	stats, err := ComputeBookDays(app, ownerId, date, sessionGap)
	if err != nil {
		return 0, err
	}

	existing, err := findBookDays(app, ownerId, date)
	if err != nil {
		return 0, err
	}

	collection, err := app.FindCollectionByNameOrId(schema.CollectionReadingBookDays)
	if err != nil {
		return 0, err
	}

	pagesRead := 0
	for _, row := range stats {
		record := existing[row.Book]
		delete(existing, row.Book)

		if record == nil {
			record = core.NewRecord(collection)
			record.Set(schema.FieldOwner, ownerId)
			record.Set(schema.FieldDate, date)
			record.Set(schema.FieldBook, row.Book)
		}

		record.Set(schema.FieldUpdateCount, row.UpdateCount)
		record.Set(schema.FieldProgressIncrease, row.ProgressIncrease)
		record.Set(schema.FieldReadingTime, row.ReadingTime)
		record.Set(schema.FieldDocumentsTouched, row.DocumentsTouched)
		record.Set(schema.FieldPagesRead, row.PagesRead)
		record.Set(schema.FieldComputedAt, time.Now().UTC())

		if err := app.Save(record); err != nil {
			return 0, err
		}

		pagesRead += row.PagesRead
	}

	// Whatever is left was read on this day the last time it was computed and is
	// not any more: a document unlinked from its book, or history deleted.
	for _, stale := range existing {
		if err := app.Delete(stale); err != nil {
			return 0, err
		}
	}

	return pagesRead, nil
}

// findBookDays loads the stored per-book rows of one day, keyed by book.
func findBookDays(app core.App, ownerId, date string) (map[string]*core.Record, error) {
	records, err := app.FindRecordsByFilter(
		schema.CollectionReadingBookDays,
		"owner = {:owner} && date = {:date}",
		"",
		0,
		0,
		dbx.Params{"owner": ownerId, "date": date},
	)
	if err != nil {
		return nil, err
	}

	byBook := make(map[string]*core.Record, len(records))
	for _, record := range records {
		byBook[record.GetString(schema.FieldBook)] = record
	}

	return byBook, nil
}

// MeasureBooksOfDay re-measures the page size of every book read on one day.
//
// It runs before the day is computed, so that a day's pages are reckoned in the
// page count the reading itself implies rather than in the word count fallback.
// Failures are logged and swallowed: a missing measurement costs precision in
// one number, while a failed recomputation costs the whole day.
func MeasureBooksOfDay(app core.App, ownerId, date string) {
	ids := []struct {
		Book string `db:"book"`
	}{}

	params, err := dayBounds(app, ownerId, date)
	if err != nil {
		app.Logger().Warn("failed to resolve a reading day",
			"owner", ownerId, "date", date, "error", err)

		return
	}

	err = app.DB().
		NewQuery(`
			SELECT DISTINCT d.[[book]] AS book
			FROM {{` + schema.CollectionDocuments + `}} d
			WHERE d.[[owner]] = {:owner} AND d.[[book]] != '' AND (
				(d.[[last_read_at]] >= {:start} AND d.[[last_read_at]] < {:end})
				OR EXISTS (
					SELECT 1 FROM {{` + schema.CollectionDocumentHistory + `}} h
					WHERE h.[[document_ref]] = d.[[id]]
					  AND h.[[last_read_at]] >= {:start} AND h.[[last_read_at]] < {:end}
				)
			)
		`).
		Bind(params).
		All(&ids)
	if err != nil {
		app.Logger().Warn("failed to list the books read on a day",
			"owner", ownerId, "date", date, "error", err)

		return
	}

	for _, id := range ids {
		book, err := app.FindRecordById(schema.CollectionBooks, id.Book)
		if err != nil {
			continue
		}
		if _, err := MeasurePageSize(app, book); err != nil {
			app.Logger().Warn("failed to measure a book's page size",
				"book", id.Book, "error", err)
		}
	}
}
