//
// File:        internal/statistics/pagination_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package statistics_test

import (
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/statistics"
)

// turns builds a run of page turns, one a minute, in a given pagination.
func turns(md5 string, first int, start time.Time, count, total int) []page {
	rows := make([]page, 0, count)
	for index := range count {
		rows = append(rows, page{
			md5:      md5,
			page:     first + index,
			start:    start.Add(time.Duration(index) * time.Minute),
			duration: 60,
			total:    total,
		})
	}

	return rows
}

// only returns the single pagination the file states, failing if it says
// anything else.
func only(t testing.TB, counts []statistics.Pagination) statistics.Pagination {
	t.Helper()

	if len(counts) != 1 {
		t.Fatalf("the file states %d page counts, want 1", len(counts))
	}

	return counts[0]
}

func TestThePageCountTheDeviceStatedIsRead(t *testing.T) {
	app, user := newApp(t)

	last := time.Date(2026, 8, 10, 20, 1, 0, 0, vienna)
	path := build(t, map[string]string{zeitDesSturms: "Zeit des Sturms"}, []page{
		{zeitDesSturms, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, vienna), 60, 700},
		{zeitDesSturms, 11, last, 45, 700},
	})

	result, err := statistics.Import(app, user.Id, path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	count := only(t, result.Pages)
	if count.Document != zeitDesSturms {
		t.Errorf("the count belongs to %q", count.Document)
	}
	if count.Pages != 700 {
		t.Errorf("the device stated %d pages, want the 700 it wrote down", count.Pages)
	}
	if count.Through != last.Unix() {
		t.Errorf("it looked through %d, want the newest turn at %d", count.Through, last.Unix())
	}
}

// The Witcher-Saga case. Four months of an omnibus were read at 3535 pages, and
// then it was reopened three times after finishing, in a pagination nothing was
// ever read in. The count worth keeping is the one the reading happened in — and
// it is also the only one the pages counted in this server's own statistics are
// comparable with.
func TestAReopeningDoesNotRepaginateABookThatWasReadInAnother(t *testing.T) {
	app, user := newApp(t)

	start := time.Date(2026, 8, 10, 9, 0, 0, 0, vienna)
	rows := turns(zeitDesSturms, 3400, start, 37, 3535)
	rows = append(rows, turns(zeitDesSturms, 1, start.Add(time.Hour*2), 3, 3851)...)

	result, err := statistics.Import(app, user.Id, build(t,
		map[string]string{zeitDesSturms: "Zeit des Sturms"}, rows))
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	count := only(t, result.Pages)
	if count.Pages != 3535 {
		t.Errorf("the book runs to %d pages, want the 3535 it was read in", count.Pages)
	}
	if count.Turns != 37 {
		t.Errorf("%d turns backed the count, want 37", count.Turns)
	}
	// How far it looked is the newest turn there is, whichever pagination that
	// one happened to be in: it is the mark that says this file has been read.
	newest := rows[len(rows)-1].start.Unix()
	if count.Through != newest {
		t.Errorf("it looked through %d, want %d", count.Through, newest)
	}
}

// The other half of the same rule: a change of font is a change of the book's
// page count, and it has to be followed rather than outvoted by the history.
func TestANewPaginationTakesOverOnceMostOfTheRecentReadingIsInIt(t *testing.T) {
	app, user := newApp(t)

	start := time.Date(2026, 8, 10, 9, 0, 0, 0, vienna)
	rows := turns(zeitDesSturms, 1, start, 60, 300)
	rows = append(rows, turns(zeitDesSturms, 1, start.Add(time.Hour*24), 25, 400)...)

	result, err := statistics.Import(app, user.Id, build(t,
		map[string]string{zeitDesSturms: "Zeit des Sturms"}, rows))
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if count := only(t, result.Pages); count.Pages != 400 {
		t.Errorf("the book runs to %d pages, want the 400 it is read in now", count.Pages)
	}
}

// Every document the file knows about is answered for, because each is a book
// somebody may have uploaded.
func TestEveryDocumentGetsItsOwnPageCount(t *testing.T) {
	app, user := newApp(t)

	other := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d"
	moment := time.Date(2026, 8, 10, 20, 0, 0, 0, vienna)
	path := build(t,
		map[string]string{zeitDesSturms: "Zeit des Sturms", other: "Der letzte Wunsch"},
		[]page{
			{zeitDesSturms, 10, moment, 60, 700},
			{other, 3, moment.Add(time.Hour), 60, 320},
		})

	result, err := statistics.Import(app, user.Id, path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(result.Pages) != 2 {
		t.Fatalf("the file states %d page counts, want 2", len(result.Pages))
	}

	stated := map[string]int{}
	for _, count := range result.Pages {
		stated[count.Document] = count.Pages
	}
	if stated[zeitDesSturms] != 700 || stated[other] != 320 {
		t.Errorf("the counts are %v, want 700 and 320", stated)
	}
}

// An older KOReader, or a format it never paginated, records no count. Saying
// nothing is the right answer: a book keeps whatever count it had.
func TestATurnWithNoCountStatesNothing(t *testing.T) {
	app, user := newApp(t)

	path := build(t, map[string]string{zeitDesSturms: "Zeit des Sturms"}, []page{
		{zeitDesSturms, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, vienna), 60, -1},
	})

	result, err := statistics.Import(app, user.Id, path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(result.Pages) != 0 {
		t.Errorf("a file with no page counts stated %v", result.Pages)
	}
}
