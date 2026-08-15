//
// File:        internal/achievements/measure.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package achievements

import (
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/timezone"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// lateHour is when a night stops counting as the night before.
//
// Reading at 03:00 is still last night; reading at 06:00 is an early start, and
// that is a different achievement nobody has asked for. Five is where one turns
// into the other for most people, and it is a judgement rather than a fact.
const lateHour = 5

// booksFinished counts the documents that have ever reached the end.
//
// Ever, not currently: progress goes backwards when a book is started again, and
// having finished it once is the thing being counted. The history is where that
// is written down.
//
// It counts documents rather than books because a book nobody uploaded is still
// a book that was read to the end — and since a merge folds two documents of one
// title into one, the double counting that would otherwise threaten this has
// somewhere to be fixed.
func booksFinished(app core.App, ownerId string) (int, error) {
	total := 0

	err := app.DB().
		NewQuery(`
			SELECT COUNT(DISTINCT document_id) FROM (
				SELECT [[id]] AS document_id, [[progress]] AS progress
				FROM {{` + schema.CollectionDocuments + `}}
				WHERE [[owner]] = {:owner}
				UNION ALL
				SELECT [[document_ref]] AS document_id, [[progress]] AS progress
				FROM {{` + schema.CollectionDocumentHistory + `}}
				WHERE [[owner]] = {:owner}
			)
			WHERE progress >= 1
		`).
		Bind(dbx.Params{"owner": ownerId}).
		Row(&total)

	return total, err
}

// pagesRead sums every page the statistics have attributed to a book.
//
// Both tables, because retention folds aged out days into months and deletes
// them: counting only the days would quietly reduce a lifetime total every time
// the retention job ran.
func pagesRead(app core.App, ownerId string) (int, error) {
	total := 0

	err := app.DB().
		NewQuery(`
			SELECT COALESCE((
				SELECT SUM([[pages_read]]) FROM {{` + schema.CollectionReadingDays + `}}
				WHERE [[owner]] = {:owner}
			), 0) + COALESCE((
				SELECT SUM([[pages_read]]) FROM {{` + schema.CollectionReadingMonths + `}}
				WHERE [[owner]] = {:owner}
			), 0)
		`).
		Bind(dbx.Params{"owner": ownerId}).
		Row(&total)

	return total, err
}

// booksInLibrary counts the uploaded EPUBs.
func booksInLibrary(app core.App, ownerId string) (int, error) {
	total, err := app.CountRecords(schema.CollectionBooks, dbx.HashExp{schema.FieldOwner: ownerId})

	return int(total), err
}

// lateNights counts the nights on which reading carried past midnight.
//
// A night is named after the day it began, so reading at 00:30 on the 15th is
// counted as the night of the 14th — which is what a person means by "I was up
// reading". Without that, one late session would count as two nights whenever it
// straddled midnight.
//
// The hour is local, which is the whole reason this could not be built before
// accounts had a timezone: in UTC the boundary falls somewhere in the middle of
// an evening and the count is meaningless.
func lateNights(app core.App, ownerId string) (int, error) {
	rows := []struct {
		LastReadAt types.DateTime `db:"last_read_at"`
	}{}

	// Distinct moments rather than rows: one push writes the same instant to a
	// document and to the history entry it superseded.
	err := app.DB().
		NewQuery(`
			SELECT DISTINCT [[last_read_at]] AS last_read_at
			FROM {{` + schema.CollectionDocuments + `}}
			WHERE [[owner]] = {:owner}
			UNION
			SELECT DISTINCT [[last_read_at]] AS last_read_at
			FROM {{` + schema.CollectionDocumentHistory + `}}
			WHERE [[owner]] = {:owner}
		`).
		Bind(dbx.Params{"owner": ownerId}).
		All(&rows)
	if err != nil {
		return 0, err
	}

	location := timezone.Of(app, ownerId)
	nights := map[string]bool{}

	for _, row := range rows {
		if row.LastReadAt.IsZero() {
			continue
		}

		local := row.LastReadAt.Time().In(location)
		if local.Hour() >= lateHour {
			continue
		}

		nights[local.AddDate(0, 0, -1).Format(timezone.DateLayout)] = true
	}

	return len(nights), nil
}

// longestStreak returns the longest run of consecutive days with reading on them.
//
// It can only see the days the retention window has kept, so a streak from
// before it is no longer measurable. That is exactly why an awarded achievement
// is never taken away: it was true when it was measured, and the evidence aging
// out does not make it untrue.
func longestStreak(app core.App, ownerId string) (int, error) {
	rows := []struct {
		Date string `db:"date"`
	}{}

	err := app.DB().
		Select("[[date]] AS date").
		From(schema.CollectionReadingDays).
		Where(dbx.HashExp{schema.FieldOwner: ownerId}).
		AndWhere(dbx.NewExp("[[update_count]] > 0")).
		OrderBy("[[date]] ASC").
		All(&rows)
	if err != nil {
		return 0, err
	}

	longest, run := 0, 0
	previous := time.Time{}

	for _, row := range rows {
		day, err := time.Parse(timezone.DateLayout, row.Date)
		if err != nil {
			continue
		}

		if !previous.IsZero() && day.Equal(previous.AddDate(0, 0, 1)) {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}

		previous = day
	}

	return longest, nil
}
