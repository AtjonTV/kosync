//
// File:        internal/statistics/measured.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package statistics

import (
	"database/sql"
	"errors"
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Day is what a device measured on one of the reader's days.
type Day struct {
	// Seconds is the time the device had a page open, summed. It owes nothing to
	// the session gap heuristic: there is no guessing where a session ended,
	// because every page carries how long it was looked at.
	Seconds int `db:"seconds"`

	// Pages is how many distinct pages of how many documents were open. Distinct
	// because turning back to re-read a paragraph is not another page read, and
	// the day would otherwise reward flicking back and forth.
	Pages int `db:"pages"`

	// Documents is how many files were open that day, matched to a book or not.
	Documents int `db:"documents"`
}

// IsEmpty reports whether anything was measured at all.
func (d Day) IsEmpty() bool {
	return d.Seconds == 0 && d.Pages == 0 && d.Documents == 0
}

// BookDay is the same for one book.
type BookDay struct {
	Book    string `db:"book"`
	Seconds int    `db:"seconds"`
	Pages   int    `db:"pages"`
}

// bookJoin matches a page read to a book by any of the hashes a book is known
// by, written once because both queries below need exactly the same rule.
//
// It is a join rather than a stored link on purpose. A book uploaded next month
// should make last month's measurements count towards it, and a link written at
// import time would have to be repaired afterwards — which is the one bug this
// codebase has already had twice.
const bookJoin = `
	JOIN {{` + schema.CollectionBooks + `}} b
		ON b.[[` + schema.FieldOwner + `]] = r.[[` + schema.FieldOwner + `]]
		AND (
			(b.[[` + schema.FieldHashBinary + `]] != '' AND b.[[` + schema.FieldHashBinary + `]] = r.[[` + schema.FieldDocument + `]])
			OR (b.[[` + schema.FieldHashFilename + `]] != '' AND b.[[` + schema.FieldHashFilename + `]] = r.[[` + schema.FieldDocument + `]])
			OR (b.[[` + schema.FieldHashCatalog + `]] != '' AND b.[[` + schema.FieldHashCatalog + `]] = r.[[` + schema.FieldDocument + `]])
		)
`

// MeasuredDay returns what the device measured for one of the reader's days.
//
// The bounds are the same half-open range of UTC instants every other statistics
// query uses, for the same reason: a day belongs to the reader's zone, and an
// offset applied in SQL is wrong twice a year.
func MeasuredDay(app core.App, ownerId, start, end string) (Day, error) {
	day := Day{}

	err := app.DB().
		NewQuery(`
			SELECT
				COALESCE(SUM(r.[[` + schema.FieldDuration + `]]), 0) AS seconds,
				COUNT(DISTINCT r.[[` + schema.FieldDocument + `]] || '/' || r.[[` + schema.FieldPage + `]]) AS pages,
				COUNT(DISTINCT r.[[` + schema.FieldDocument + `]]) AS documents
			FROM {{` + schema.CollectionPageReads + `}} r
			WHERE r.[[` + schema.FieldOwner + `]] = {:owner}
				AND r.[[` + schema.FieldStartedAt + `]] >= {:start}
				AND r.[[` + schema.FieldStartedAt + `]] < {:end}
		`).
		Bind(dbx.Params{"owner": ownerId, "start": start, "end": end}).
		One(&day)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Day{}, fmt.Errorf("read the measured day of %s: %w", ownerId, err)
	}

	return day, nil
}

// MeasuredBookDays is the same day, split by the books the reading can be
// attributed to.
//
// Reading in a document nobody has uploaded the file for appears in MeasuredDay
// and not here, which is the honest split: the time is real, the book it belongs
// to is not something this server has.
func MeasuredBookDays(app core.App, ownerId, start, end string) ([]BookDay, error) {
	rows := []BookDay{}

	err := app.DB().
		NewQuery(`
			SELECT
				b.[[id]] AS book,
				COALESCE(SUM(r.[[` + schema.FieldDuration + `]]), 0) AS seconds,
				COUNT(DISTINCT r.[[` + schema.FieldDocument + `]] || '/' || r.[[` + schema.FieldPage + `]]) AS pages
			FROM {{` + schema.CollectionPageReads + `}} r
			` + bookJoin + `
			WHERE r.[[` + schema.FieldOwner + `]] = {:owner}
				AND r.[[` + schema.FieldStartedAt + `]] >= {:start}
				AND r.[[` + schema.FieldStartedAt + `]] < {:end}
			GROUP BY b.[[id]]
			ORDER BY b.[[id]]
		`).
		Bind(dbx.Params{"owner": ownerId, "start": start, "end": end}).
		All(&rows)
	if err != nil {
		return nil, fmt.Errorf("read the measured books of %s: %w", ownerId, err)
	}

	return rows, nil
}

// Days returns every local day an account has measurements for, which is what a
// timezone change has to requeue.
func Days(app core.App, ownerId string) ([]string, error) {
	rows := []struct {
		StartedAt string `db:"started_at"`
	}{}

	err := app.DB().
		NewQuery(`
			SELECT DISTINCT [[` + schema.FieldStartedAt + `]] AS started_at
			FROM {{` + schema.CollectionPageReads + `}}
			WHERE [[` + schema.FieldOwner + `]] = {:owner}
		`).
		Bind(dbx.Params{"owner": ownerId}).
		All(&rows)
	if err != nil {
		return nil, fmt.Errorf("list the measured days of %s: %w", ownerId, err)
	}

	moments := make([]string, 0, len(rows))
	for _, row := range rows {
		moments = append(moments, row.StartedAt)
	}

	return moments, nil
}
