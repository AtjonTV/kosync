//
// File:        internal/statistics/pagination.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package statistics

import (
	"database/sql"
	"fmt"
)

// Pagination is how many pages a device paginated one document into.
//
// It is the number this server otherwise has to reconstruct from the size of the
// steps a device's progress moves in — stated outright, by the reader that did
// the paginating. Document is KOReader's md5, the same identity a page turn
// carries.
//
// Turns is how many of the recent page turns were recorded in this pagination,
// and Through the moment of the newest page turn the document has, which is how
// far into the reading this looked.
type Pagination struct {
	Document string
	Pages    int
	Turns    int
	Through  int64
}

// paginationWindow is how many of a document's most recent page turns decide
// which pagination is the current one.
//
// A device's page count changes with the font, the margins and the screen, and
// the statistics database keeps every page turn under whatever count was in force
// when it happened. The most recent turns are therefore the only ones that
// describe the book as the reader has it now.
//
// Forty for the same reason the progress estimator uses forty: it is a few
// evenings of reading, long enough that three stray turns from reopening a
// finished book cannot outvote a week of reading it, and short enough that a
// change of font is followed within a sitting rather than outvoted by a year of
// history.
const paginationWindow = 40

// paginationQuery reads the pagination each document's recent reading was done
// in, most-used first.
//
// The count comes from page_stat_data rather than from book.pages, though both
// are the device's own number. book.pages is whatever pagination the book was
// last opened in, which is set by opening it — three turns after reopening a
// finished book to check a name would state a count nothing was ever read in.
// The per-turn total is the count the reading actually happened in, which is the
// one the pages in this server's own statistics are counted in.
//
// The ordering is what makes the fold below trivial: the first row of each
// document is the pagination most of its recent turns were recorded in, ties
// going to the more recent one.
const paginationQuery = `
	SELECT md5, pages, COUNT(*) AS turns, MAX(through) AS through
	FROM (
		SELECT
			b.md5 AS md5,
			p.total_pages AS pages,
			p.start_time AS through,
			ROW_NUMBER() OVER (PARTITION BY b.md5 ORDER BY p.start_time DESC) AS recency
		FROM page_stat_data p
		JOIN book b ON b.id = p.id_book
		WHERE b.md5 IS NOT NULL AND b.md5 != '' AND p.start_time > 0 AND p.total_pages > 0
	)
	WHERE recency <= ?
	GROUP BY md5, pages
	ORDER BY md5 ASC, turns DESC, through DESC
`

// paginations reads what the device says its documents run to.
//
// A file with no total_pages anywhere in it — an old KOReader, or a format that
// never reported one — simply yields nothing, and the books keep whatever count
// they had.
func paginations(source *sql.DB) ([]Pagination, error) {
	rows, err := source.Query(paginationQuery, paginationWindow)
	if err != nil {
		return nil, fmt.Errorf("read the page counts: %w", err)
	}
	defer rows.Close()

	found := map[string]Pagination{}
	order := []string{}

	for rows.Next() {
		var (
			document string
			pages    int
			turns    int
			through  int64
		)
		if err := rows.Scan(&document, &pages, &turns, &through); err != nil {
			return nil, fmt.Errorf("read a page count: %w", err)
		}

		one, seen := found[document]
		if !seen {
			// The first row of a document is the winning pagination; the rest
			// are only here to say how far the reading goes.
			one = Pagination{Document: document, Pages: pages, Turns: turns}
			order = append(order, document)
		}
		if through > one.Through {
			one.Through = through
		}
		found[document] = one
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the page counts: %w", err)
	}

	counts := make([]Pagination, 0, len(order))
	for _, document := range order {
		counts = append(counts, found[document])
	}

	return counts, nil
}
