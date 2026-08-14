//
// File:        internal/analytics/compute.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// DayStats is one precomputed day of reading.
type DayStats struct {
	UpdateCount      int     `db:"update_count"`
	ProgressIncrease float64 `db:"progress_increase"`
	ReadingTime      int     `db:"reading_time"`
	DocumentsTouched int     `db:"documents_touched"`
}

// IsEmpty reports whether the day saw no reading at all.
func (s DayStats) IsEmpty() bool {
	return s.UpdateCount == 0 && s.DocumentsTouched == 0 && s.ReadingTime == 0 && s.ProgressIncrease == 0
}

// dayStatsQuery computes one day for one user from the raw progress records.
//
// The day is a half-open range of UTC instants rather than a text prefix,
// because a reading day belongs to the reader's zone and not to the one the
// timestamps happen to be stored in. See dayBounds.
//
// The three numbers it produces:
//   - update_count: how many distinct progress moments the day contains.
//   - progress_increase: per document, how much further into it the reader got
//     compared to the furthest point reached before that day, in percentage
//     points. Clamped at zero per document so that restarting a book does not
//     subtract from a day's reading.
//   - reading_time: the sum of the gaps between consecutive progress moments,
//     counting only gaps shorter than the configured session gap. This is a
//     heuristic: KOReader reports positions, never durations. The gaps are
//     measured in whole milliseconds and only the total is truncated to
//     seconds, so a day of short intervals does not lose a fraction each time.
const dayStatsQuery = `
	WITH all_states AS (
		SELECT [[id]] AS document_id, [[progress]] AS progress, [[last_read_at]] AS last_read_at
		FROM {{` + schema.CollectionDocuments + `}}
		WHERE [[owner]] = {:owner}
		UNION ALL
		SELECT [[document_ref]] AS document_id, [[progress]] AS progress, [[last_read_at]] AS last_read_at
		FROM {{` + schema.CollectionDocumentHistory + `}}
		WHERE [[owner]] = {:owner}
	),
	day_states AS (
		SELECT * FROM all_states WHERE last_read_at >= {:start} AND last_read_at < {:end}
	),
	per_document AS (
		SELECT
			document_id,
			MAX(progress) AS max_progress,
			COUNT(DISTINCT last_read_at) AS update_count,
			MIN(last_read_at) AS first_of_day
		FROM day_states
		GROUP BY document_id
	),
	with_previous AS (
		SELECT
			p.document_id,
			p.max_progress,
			p.update_count,
			(
				SELECT MAX(a.progress)
				FROM all_states a
				WHERE a.document_id = p.document_id AND a.last_read_at < p.first_of_day
			) AS previous_progress
		FROM per_document p
	),
	moments AS (
		-- Whole milliseconds, so the gaps below are exact integers rather than
		-- floating point differences that lose a second here and there.
		SELECT CAST(round((julianday(last_read_at) - 2440587.5) * 86400000.0) AS INTEGER) AS moment
		FROM day_states
	),
	deltas AS (
		SELECT moment - LAG(moment) OVER (ORDER BY moment) AS delta
		FROM moments
	)
	SELECT
		COALESCE((SELECT SUM(update_count) FROM per_document), 0) AS update_count,
		COALESCE((SELECT SUM(MAX(max_progress - COALESCE(previous_progress, 0), 0)) FROM with_previous), 0) * 100 AS progress_increase,
		COALESCE((SELECT COUNT(*) FROM per_document), 0) AS documents_touched,
		CAST(COALESCE((
			SELECT SUM(CASE WHEN delta > 0 AND delta < {:gap} THEN delta ELSE 0 END) FROM deltas
		), 0) / 1000 AS INTEGER) AS reading_time
`

// ComputeDay calculates the statistics of a single day without storing them.
func ComputeDay(app core.App, ownerId, date string, sessionGap time.Duration) (DayStats, error) {
	stats := DayStats{}

	params, err := dayBounds(app, ownerId, date)
	if err != nil {
		return DayStats{}, fmt.Errorf("resolve the day %s of %s: %w", date, ownerId, err)
	}
	params["gap"] = sessionGap.Milliseconds()

	err = app.DB().
		NewQuery(dayStatsQuery).
		Bind(params).
		One(&stats)
	if err != nil {
		return DayStats{}, fmt.Errorf("compute statistics for %s on %s: %w", ownerId, date, err)
	}

	return stats, nil
}

// RecomputeDay recalculates a day and stores the result.
//
// A day without any reading is removed instead of being stored as a row of
// zeroes, so that the statistics collection stays proportional to the days a
// user actually read.
//
// The per-book rows are computed in the same pass, and the day's pages read is
// their sum: a page belongs to exactly one book, so unlike the reading time
// nothing falls between them. Progress in a document that has no uploaded book
// therefore counts towards the day's reading but not towards its pages, which is
// the honest answer — nothing knows how long that document is.
func RecomputeDay(app core.App, ownerId, date string, sessionGap time.Duration) error {
	stats, err := ComputeDay(app, ownerId, date, sessionGap)
	if err != nil {
		return err
	}

	// Before the books are counted, not after: a page count measured from this
	// very day's pushes is the unit the day should be reckoned in.
	MeasureBooksOfDay(app, ownerId, date)

	pagesRead, err := RecomputeBookDays(app, ownerId, date, sessionGap)
	if err != nil {
		return err
	}

	existing, err := findDay(app, ownerId, date)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if stats.IsEmpty() {
		if existing == nil {
			return nil
		}
		return app.Delete(existing)
	}

	record := existing
	if record == nil {
		collection, err := app.FindCollectionByNameOrId(schema.CollectionReadingDays)
		if err != nil {
			return err
		}
		record = core.NewRecord(collection)
		record.Set(schema.FieldOwner, ownerId)
		record.Set(schema.FieldDate, date)
	}

	record.Set(schema.FieldUpdateCount, stats.UpdateCount)
	record.Set(schema.FieldProgressIncrease, stats.ProgressIncrease)
	record.Set(schema.FieldReadingTime, stats.ReadingTime)
	record.Set(schema.FieldDocumentsTouched, stats.DocumentsTouched)
	record.Set(schema.FieldPagesRead, pagesRead)
	record.Set(schema.FieldComputedAt, time.Now().UTC())

	return app.Save(record)
}

// findDay loads the stored statistics of a single day.
func findDay(app core.App, ownerId, date string) (*core.Record, error) {
	return app.FindFirstRecordByFilter(
		schema.CollectionReadingDays,
		"owner = {:owner} && date = {:date}",
		dbx.Params{"owner": ownerId, "date": date},
	)
}
