//
// File:        internal/statistics/statistics.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package statistics reads what a device measured about its own reading.
//
// KOReader keeps a database of page turns: which page of which file, from when,
// for how long. It is the only record anywhere of when reading actually
// happened, because the sync protocol carries no clock — everything else in this
// server infers a day and a duration from the moments pushes arrived, which is
// wrong by half on the reference data and blind altogether to reading done
// offline.
//
// This package turns that file into rows. It does not decide what they mean: the
// statistics worker aggregates them into days, in the account's own timezone,
// alongside everything else.
package statistics

import (
	"database/sql"
	"fmt"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/timezone"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	// The SQLite driver PocketBase already uses.
	_ "modernc.org/sqlite"
)

// Result is what one import did.
type Result struct {
	// Rows is how many page reads the file held, Added how many of them were
	// new. On a second sync of the same device those differ by a week.
	Rows  int
	Added int

	// Dates are the local days the new rows fall on, which are the days whose
	// statistics are now out of date.
	Dates []string
}

// pageRead is one row of the device's own record.
type pageRead struct {
	Document  string `db:"md5"`
	Page      int    `db:"page"`
	StartTime int64  `db:"start_time"`
	Duration  int    `db:"duration"`
}

// readQuery pulls the page turns out of a statistics database.
//
// Rows with no md5 are skipped in the query rather than later: a book KOReader
// could not hash is a book nothing here could ever match to a document, and
// keeping the rows would mean storing reading that can never be attributed.
//
// page_stat_data is read directly rather than through KOReader's page_stat view.
// The view rescales page numbers to the book's current page count, which is the
// right thing for a chart of one book and the wrong thing to store: the rescaling
// depends on a total that changes with the font size, so what it produces is not
// a fact but a rendering of one. The raw page number is what the device recorded.
const readQuery = `
	SELECT b.md5 AS md5, p.page AS page, p.start_time AS start_time, p.duration AS duration
	FROM page_stat_data p
	JOIN book b ON b.id = p.id_book
	WHERE b.md5 IS NOT NULL AND b.md5 != '' AND p.start_time > 0
	ORDER BY p.start_time
`

// Import reads a statistics database and stores what is new in it.
//
// The file is opened read only and immutable — it is an upload, not something
// this server chose to have, and immutable also stops SQLite writing its journal
// sidecars into the directory the file lives in.
//
// Nothing is ever updated or deleted, only inserted. A page turn is something
// that happened; a second sync of the same device says so again, and saying it
// twice does not make it two.
func Import(app core.App, ownerId, path string) (Result, error) {
	result := Result{}

	if ownerId == "" {
		return result, fmt.Errorf("no account to import for")
	}

	source, err := sql.Open("sqlite",
		"file:"+path+"?mode=ro&immutable=1&_pragma=trusted_schema(off)&_pragma=query_only(true)")
	if err != nil {
		return result, fmt.Errorf("open the statistics database: %w", err)
	}
	defer source.Close()

	rows, err := source.Query(readQuery)
	if err != nil {
		return result, fmt.Errorf("read the page turns: %w", err)
	}
	defer rows.Close()

	reads := []pageRead{}
	for rows.Next() {
		one := pageRead{}
		if err := rows.Scan(&one.Document, &one.Page, &one.StartTime, &one.Duration); err != nil {
			return result, fmt.Errorf("read a page turn: %w", err)
		}
		reads = append(reads, one)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("read the page turns: %w", err)
	}

	result.Rows = len(reads)
	if len(reads) == 0 {
		return result, nil
	}

	location := timezone.Of(app, ownerId)
	days := map[string]bool{}

	// One transaction for the lot: a half-imported file would leave days whose
	// statistics disagree with the rows they were computed from, and the next
	// import would not know to fix them.
	err = app.RunInTransaction(func(txApp core.App) error {
		for _, one := range reads {
			added, err := insert(txApp, ownerId, one)
			if err != nil {
				return err
			}
			if !added {
				continue
			}

			result.Added++
			days[timezone.DayOf(location, time.Unix(one.StartTime, 0))] = true
		}

		return nil
	})
	if err != nil {
		return Result{}, err
	}

	for day := range days {
		result.Dates = append(result.Dates, day)
	}

	return result, nil
}

// insert stores one page turn unless it is already here.
//
// The conflict clause does the deciding rather than a lookup per row: the unique
// index is KOReader's own key, and a second sync of a database that has grown by
// a week presents several thousand rows that are all already stored.
func insert(app core.App, ownerId string, one pageRead) (bool, error) {
	result, err := app.DB().
		NewQuery(`
			INSERT INTO {{` + schema.CollectionPageReads + `}}
				([[id]], [[owner]], [[document]], [[page]], [[started_at]], [[duration]], [[created]], [[updated]])
			VALUES ({:id}, {:owner}, {:document}, {:page}, {:started}, {:duration}, {:now}, {:now})
			ON CONFLICT ([[owner]], [[document]], [[page]], [[started_at]]) DO NOTHING
		`).
		Bind(dbx.Params{
			"id":       core.GenerateDefaultRandomId(),
			"owner":    ownerId,
			"document": one.Document,
			"page":     one.Page,
			"started":  time.Unix(one.StartTime, 0).UTC().Format(dateTimeLayout),
			"duration": one.Duration,
			"now":      time.Now().UTC().Format(dateTimeLayout),
		}).
		Execute()
	if err != nil {
		return false, fmt.Errorf("store a page turn: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, nil //nolint:nilerr // a driver that will not count is not a failed insert
	}

	return affected > 0, nil
}

// dateTimeLayout is how PocketBase stores a date column, and therefore how a
// timestamp has to be written to be comparable against one.
const dateTimeLayout = "2006-01-02 15:04:05.000Z"
