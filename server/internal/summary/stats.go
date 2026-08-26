//
// File:        internal/summary/stats.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package summary

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/timezone"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// maxBooks is how many books a summary names. A month of heavy reading can touch
// twenty, and a list that long stops being a summary.
const maxBooks = 5

// Stats is what an account read over one period.
type Stats struct {
	Period Period

	// DaysRead is how many of the period's days saw any reading at all, which is
	// the number a habit is actually visible in.
	DaysRead int
	Pages    int
	Seconds  int

	// BestDate and BestPages are the day the most was read on.
	BestDate  string
	BestPages int

	Books        []BookRead
	MoreBooks    int
	Achievements []Award
}

// BookRead is one book the period was spent in.
type BookRead struct {
	Id       string `db:"id"`
	Title    string `db:"title"`
	Pages    int    `db:"pages"`
	Seconds  int    `db:"seconds"`
	Finished bool
}

// Award is one achievement earned during the period.
type Award struct {
	Rule string `db:"rule"`
	Tier int    `db:"tier"`
}

// IsEmpty reports whether the period saw no reading.
//
// Worth having as its own question: a summary of nothing is not a summary, and
// the right thing to do with one is to send no mail rather than to send a page
// of zeroes.
func (s Stats) IsEmpty() bool {
	return s.DaysRead == 0 && s.Pages == 0 && s.Seconds == 0
}

// Hours returns the reading time as a phrase, since that is the only form it is
// ever shown in.
func (s Stats) Hours() string {
	return Duration(s.Seconds)
}

// Duration writes a number of seconds the way somebody would say it.
func Duration(seconds int) string {
	if seconds < 60 {
		return "under a minute"
	}

	minutes := seconds / 60
	if minutes == 1 {
		return "1 minute"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	}

	hours := minutes / 60
	rest := minutes % 60
	if rest == 0 {
		if hours == 1 {
			return "1 hour"
		}

		return fmt.Sprintf("%d hours", hours)
	}

	return fmt.Sprintf("%dh %02dm", hours, rest)
}

// For gathers what one account read over one period.
//
// It reads the precomputed daily rows rather than the progress history: those
// rows are what the dashboard shows, so a summary that disagreed with them would
// be a second opinion about the same week. It also means this is three small
// indexed queries rather than a walk through every push.
func For(app core.App, owner *core.Record, period Period) (Stats, error) {
	stats := Stats{Period: period}
	params := dbx.Params{"owner": owner.Id, "from": period.From, "to": period.To}

	totals := struct {
		Days    int `db:"days"`
		Pages   int `db:"pages"`
		Seconds int `db:"seconds"`
	}{}

	err := app.DB().
		NewQuery(`
			SELECT
				COUNT(*) AS days,
				COALESCE(SUM([[` + schema.FieldPagesRead + `]]), 0) AS pages,
				COALESCE(SUM([[` + schema.FieldReadingTime + `]]), 0) AS seconds
			FROM {{` + schema.CollectionReadingDays + `}}
			WHERE [[` + schema.FieldOwner + `]] = {:owner}
				AND [[` + schema.FieldDate + `]] >= {:from}
				AND [[` + schema.FieldDate + `]] <= {:to}
		`).
		Bind(params).
		One(&totals)
	if err != nil {
		return Stats{}, fmt.Errorf("total up the days of %s: %w", owner.Id, err)
	}

	stats.DaysRead = totals.Days
	stats.Pages = totals.Pages
	stats.Seconds = totals.Seconds

	if stats.IsEmpty() {
		// Nothing else can be true of a period nobody read in, and three more
		// queries would all come back empty.
		return stats, nil
	}

	best := struct {
		Date  string `db:"date"`
		Pages int    `db:"pages"`
	}{}

	err = app.DB().
		NewQuery(`
			SELECT [[` + schema.FieldDate + `]] AS date, [[` + schema.FieldPagesRead + `]] AS pages
			FROM {{` + schema.CollectionReadingDays + `}}
			WHERE [[` + schema.FieldOwner + `]] = {:owner}
				AND [[` + schema.FieldDate + `]] >= {:from}
				AND [[` + schema.FieldDate + `]] <= {:to}
			ORDER BY pages DESC, date ASC
			LIMIT 1
		`).
		Bind(params).
		One(&best)
	if err != nil {
		return Stats{}, fmt.Errorf("find the best day of %s: %w", owner.Id, err)
	}

	stats.BestDate = best.Date
	stats.BestPages = best.Pages

	if err := addBooks(app, owner, &stats, params); err != nil {
		return Stats{}, err
	}
	if err := addAchievements(app, owner, &stats); err != nil {
		return Stats{}, err
	}

	return stats, nil
}

// addBooks fills in which books the period was spent in.
//
// Only books, not documents: a document nobody has uploaded the file for has no
// title beyond its hash, and "you read 40 pages of
// 043f11771ef9d191364ac0ba08198d36" is not a sentence worth mailing anybody. The
// pages of such a document are still in the totals above, which is where they
// belong.
func addBooks(app core.App, owner *core.Record, stats *Stats, params dbx.Params) error {
	read := []BookRead{}

	err := app.DB().
		NewQuery(`
			SELECT
				d.[[` + schema.FieldBook + `]] AS id,
				b.[[` + schema.FieldTitle + `]] AS title,
				COALESCE(SUM(d.[[` + schema.FieldPagesRead + `]]), 0) AS pages,
				COALESCE(SUM(d.[[` + schema.FieldReadingTime + `]]), 0) AS seconds
			FROM {{` + schema.CollectionReadingBookDays + `}} d
			JOIN {{` + schema.CollectionBooks + `}} b ON b.[[id]] = d.[[` + schema.FieldBook + `]]
			WHERE d.[[` + schema.FieldOwner + `]] = {:owner}
				AND d.[[` + schema.FieldDate + `]] >= {:from}
				AND d.[[` + schema.FieldDate + `]] <= {:to}
			GROUP BY d.[[` + schema.FieldBook + `]]
			ORDER BY pages DESC, title ASC
		`).
		Bind(params).
		All(&read)
	if err != nil {
		return fmt.Errorf("list the books of %s: %w", owner.Id, err)
	}
	if len(read) == 0 {
		return nil
	}

	finished, err := finishedIn(app, owner, stats.Period)
	if err != nil {
		return err
	}

	for index := range read {
		read[index].Finished = finished[read[index].Id]
	}

	if len(read) > maxBooks {
		stats.MoreBooks = len(read) - maxBooks
		read = read[:maxBooks]
	}

	stats.Books = read

	return nil
}

// finishedIn returns the books that were read to the end during the period.
//
// The end is the current progress of a document rather than something in the
// history, because that is what finishing is: the furthest the reader got is
// where they still are. A book finished last month and not opened since does not
// turn up here, since its last push is outside the range.
func finishedIn(app core.App, owner *core.Record, period Period) (map[string]bool, error) {
	location := timezone.Load(owner.GetString(schema.FieldTimezone))

	start, _, err := timezone.DayRange(location, period.From)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", period.From, err)
	}
	_, end, err := timezone.DayRange(location, period.To)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", period.To, err)
	}

	rows := []struct {
		Book string `db:"book"`
	}{}

	err = app.DB().
		NewQuery(`
			SELECT DISTINCT [[` + schema.FieldBook + `]] AS book
			FROM {{` + schema.CollectionDocuments + `}}
			WHERE [[` + schema.FieldOwner + `]] = {:owner}
				AND [[` + schema.FieldBook + `]] != ''
				AND [[` + schema.FieldProgress + `]] >= 1
				AND [[` + schema.FieldLastReadAt + `]] >= {:start}
				AND [[` + schema.FieldLastReadAt + `]] < {:end}
		`).
		Bind(dbx.Params{"owner": owner.Id, "start": start, "end": end}).
		All(&rows)
	if err != nil {
		return nil, fmt.Errorf("find the finished books of %s: %w", owner.Id, err)
	}

	finished := make(map[string]bool, len(rows))
	for _, row := range rows {
		finished[row.Book] = true
	}

	return finished, nil
}

// addAchievements fills in what was earned during the period.
//
// Named again here rather than left to the mail, because a summary that
// mentioned the pages but not the badge would be missing the part somebody
// actually wants to be told.
func addAchievements(app core.App, owner *core.Record, stats *Stats) error {
	location := timezone.Load(owner.GetString(schema.FieldTimezone))

	start, _, err := timezone.DayRange(location, stats.Period.From)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", stats.Period.From, err)
	}
	_, end, err := timezone.DayRange(location, stats.Period.To)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", stats.Period.To, err)
	}

	awards := []Award{}

	err = app.DB().
		NewQuery(`
			SELECT [[` + schema.FieldRule + `]] AS rule, [[` + schema.FieldTier + `]] AS tier
			FROM {{` + schema.CollectionAchievements + `}}
			WHERE [[` + schema.FieldOwner + `]] = {:owner}
				AND [[` + schema.FieldEarnedAt + `]] >= {:start}
				AND [[` + schema.FieldEarnedAt + `]] < {:end}
			ORDER BY [[` + schema.FieldEarnedAt + `]] ASC
		`).
		Bind(dbx.Params{"owner": owner.Id, "start": start, "end": end}).
		All(&awards)
	if err != nil {
		return fmt.Errorf("list the achievements of %s: %w", owner.Id, err)
	}

	stats.Achievements = awards

	return nil
}
