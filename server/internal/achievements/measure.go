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

// lateHour is when a night stops counting as the night before, and earlyHour is
// when a morning stops counting as an early one.
//
// Reading at 03:00 is still last night; reading at 06:00 is an early start.
// Five is where one turns into the other for most people, and eight is where
// early stops being early — both are judgements rather than facts. The two bands
// meet at five and do not overlap, so no single moment is both.
const (
	lateHour  = 5
	earlyHour = 8
)

// restartProgress is how far back into a book a position has to fall for it to
// count as having been started again rather than merely re-opened.
//
// A finished book that is opened once more sits at the last page; one that is
// being read again sits near the front. A tenth of the way in is far enough that
// a stray tap cannot reach it and near enough that it is unmistakably the start.
const restartProgress = 0.1

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

// booksReread counts the documents that were finished and then begun again.
//
// A re-read is the strongest thing the data has to say about a book mattering,
// and it is only visible in the history: the current position of a book being
// read for the second time looks exactly like one being read for the first.
//
// The test is a finish with a fresh start after it — the earliest moment the
// document stood at the end, against the latest it stood near the beginning.
// Comparing the two that way costs one pass and asks the right question, because
// a low position *before* the finish is just where the first reading began.
func booksReread(app core.App, ownerId string) (int, error) {
	total := 0

	err := app.DB().
		NewQuery(`
			SELECT COUNT(*) FROM (
				SELECT
					MIN(CASE WHEN progress >= 1 THEN moment END) AS finished,
					MAX(CASE WHEN progress <= {:restart} THEN moment END) AS restarted
				FROM (
					SELECT [[id]] AS document_id, [[progress]] AS progress,
						[[last_read_at]] AS moment
					FROM {{` + schema.CollectionDocuments + `}}
					WHERE [[owner]] = {:owner}
					UNION ALL
					SELECT [[document_ref]] AS document_id, [[progress]] AS progress,
						[[last_read_at]] AS moment
					FROM {{` + schema.CollectionDocumentHistory + `}}
					WHERE [[owner]] = {:owner}
				)
				GROUP BY document_id
			)
			WHERE finished IS NOT NULL AND restarted IS NOT NULL AND restarted > finished
		`).
		Bind(dbx.Params{"owner": ownerId, "restart": restartProgress}).
		Row(&total)

	return total, err
}

// bestDay returns the most pages read on any one day.
//
// Only the days, because a month holds a sum and the sum of a month is not a day
// anybody had. That means the record eventually ages out of the window and stops
// being measurable — which is what an award that is never taken away is for.
func bestDay(app core.App, ownerId string) (int, error) {
	total := 0

	err := app.DB().
		Select("COALESCE(MAX([[pages_read]]), 0)").
		From(schema.CollectionReadingDays).
		Where(dbx.HashExp{schema.FieldOwner: ownerId}).
		Row(&total)

	return total, err
}

// readingMoments returns every distinct instant the account has read at.
//
// Distinct rather than one per row: a single push writes the same instant to a
// document and to the history entry it superseded, and an hour of the day is
// being asked about here, not an amount of reading.
func readingMoments(app core.App, ownerId string) ([]types.DateTime, error) {
	rows := []struct {
		LastReadAt types.DateTime `db:"last_read_at"`
	}{}

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
		return nil, err
	}

	moments := make([]types.DateTime, 0, len(rows))
	for _, row := range rows {
		if !row.LastReadAt.IsZero() {
			moments = append(moments, row.LastReadAt)
		}
	}

	return moments, nil
}

// earlyMornings counts the days that began with reading before the day did.
//
// The mirror of lateNights, and the reason that one's hour band was worth naming
// rather than writing as a number: the two share a boundary, so an account
// cannot be credited with both ends of the same 05:00.
//
// A morning is simply the day it happens on. Nothing has to be moved, because
// unlike a night an early morning does not straddle anything.
func earlyMornings(app core.App, ownerId string) (int, error) {
	moments, err := readingMoments(app, ownerId)
	if err != nil {
		return 0, err
	}

	location := timezone.Of(app, ownerId)
	mornings := map[string]bool{}

	for _, moment := range moments {
		local := moment.Time().In(location)
		if local.Hour() < lateHour || local.Hour() >= earlyHour {
			continue
		}

		mornings[local.Format(timezone.DateLayout)] = true
	}

	return len(mornings), nil
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
	moments, err := readingMoments(app, ownerId)
	if err != nil {
		return 0, err
	}

	location := timezone.Of(app, ownerId)
	nights := map[string]bool{}

	for _, moment := range moments {
		local := moment.Time().In(location)
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
