//
// File:        internal/summary/stats_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package summary_test

import (
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/summary"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/types"
)

// week is the period every stats test measures, so that the rows on either side
// of it are obviously outside.
var week = summary.Period{
	Kind:  schema.SummaryWeekly,
	Key:   "2026-W33",
	From:  "2026-08-10",
	To:    "2026-08-16",
	Title: "the week of 10 August",
}

// storeDay writes one precomputed reading day, the way the worker would.
func storeDay(t testing.TB, app core.App, owner, date string, pages, seconds int) {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("find reading_days: %v", err)
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldOwner, owner)
	record.Set(schema.FieldDate, date)
	record.Set(schema.FieldUpdateCount, 5)
	record.Set(schema.FieldProgressIncrease, 4.0)
	record.Set(schema.FieldReadingTime, seconds)
	record.Set(schema.FieldDocumentsTouched, 1)
	record.Set(schema.FieldPagesRead, pages)
	record.Set(schema.FieldComputedAt, types.NowDateTime())

	if err := app.Save(record); err != nil {
		t.Fatalf("save the day %s: %v", date, err)
	}
}

// storeBookDay writes one precomputed day of one book.
func storeBookDay(t testing.TB, app core.App, owner, book, date string, pages, seconds int) {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionReadingBookDays)
	if err != nil {
		t.Fatalf("find reading_book_days: %v", err)
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldOwner, owner)
	record.Set(schema.FieldBook, book)
	record.Set(schema.FieldDate, date)
	record.Set(schema.FieldUpdateCount, 3)
	record.Set(schema.FieldProgressIncrease, 2.0)
	record.Set(schema.FieldReadingTime, seconds)
	record.Set(schema.FieldPagesRead, pages)
	record.Set(schema.FieldComputedAt, types.NowDateTime())

	if err := app.Save(record); err != nil {
		t.Fatalf("save the book day %s: %v", date, err)
	}
}

// storeBook writes a library entry to hang the per-book days off.
func storeBook(t testing.TB, app core.App, id, owner, title string) *core.Record {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("find books: %v", err)
	}

	file, err := filesystem.NewFileFromBytes([]byte("PK\x03\x04 stand-in"), "book.epub")
	if err != nil {
		t.Fatalf("build file: %v", err)
	}

	record := core.NewRecord(collection)
	record.Id = id
	record.Set(schema.FieldOwner, owner)
	record.Set(schema.FieldFile, file)
	record.Set(schema.FieldTitle, title)
	record.Set(schema.FieldContentHash, id)

	if err := app.Save(record); err != nil {
		t.Fatalf("save the book: %v", err)
	}

	return record
}

func TestTheTotalsCoverThePeriodAndNothingElse(t *testing.T) {
	app := testutil.NewApp(t)
	user := wanting(t, app, schema.SummaryWeekly, "Europe/Vienna")
	other := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)

	storeDay(t, app, user.Id, "2026-08-09", 500, 9000) // the day before the week
	storeDay(t, app, user.Id, "2026-08-10", 40, 1800)
	storeDay(t, app, user.Id, "2026-08-13", 120, 5400)
	storeDay(t, app, user.Id, "2026-08-16", 30, 900)
	storeDay(t, app, user.Id, "2026-08-17", 700, 9000) // the day after
	storeDay(t, app, other.Id, "2026-08-13", 999, 9999)

	stats, err := summary.For(app, user, week)
	if err != nil {
		t.Fatalf("summarise: %v", err)
	}

	if stats.DaysRead != 3 {
		t.Errorf("days read is %d, want 3", stats.DaysRead)
	}
	if stats.Pages != 190 {
		t.Errorf("pages is %d, want 190", stats.Pages)
	}
	if stats.Seconds != 8100 {
		t.Errorf("seconds is %d, want 8100", stats.Seconds)
	}
	if stats.BestDate != "2026-08-13" || stats.BestPages != 120 {
		t.Errorf("best day is %s with %d pages", stats.BestDate, stats.BestPages)
	}
	if stats.IsEmpty() {
		t.Error("a week with reading in it reported itself empty")
	}
}

func TestAWeekWithNoReadingIsEmpty(t *testing.T) {
	app := testutil.NewApp(t)
	user := wanting(t, app, schema.SummaryWeekly, "Europe/Vienna")

	storeDay(t, app, user.Id, "2026-08-20", 300, 6000)

	stats, err := summary.For(app, user, week)
	if err != nil {
		t.Fatalf("summarise: %v", err)
	}
	if !stats.IsEmpty() {
		t.Errorf("expected an empty week, got %d pages over %d days", stats.Pages, stats.DaysRead)
	}
}

func TestTheBooksAreListedByHowMuchWasReadOfThem(t *testing.T) {
	app := testutil.NewApp(t)
	user := wanting(t, app, schema.SummaryWeekly, "Europe/Vienna")

	storeDay(t, app, user.Id, "2026-08-11", 200, 7200)

	sturm := storeBook(t, app, testutil.PadId("booka"), user.Id, "Zeit des Sturms")
	wunsch := storeBook(t, app, testutil.PadId("bookb"), user.Id, "Der letzte Wunsch")

	storeBookDay(t, app, user.Id, wunsch.Id, "2026-08-11", 40, 1800)
	storeBookDay(t, app, user.Id, sturm.Id, "2026-08-11", 100, 3600)
	storeBookDay(t, app, user.Id, sturm.Id, "2026-08-12", 60, 1800)
	// Outside the week, so it must not appear at all.
	storeBookDay(t, app, user.Id, wunsch.Id, "2026-08-30", 500, 9000)

	stats, err := summary.For(app, user, week)
	if err != nil {
		t.Fatalf("summarise: %v", err)
	}

	if len(stats.Books) != 2 {
		t.Fatalf("expected two books, got %d", len(stats.Books))
	}
	if stats.Books[0].Title != "Zeit des Sturms" || stats.Books[0].Pages != 160 {
		t.Errorf("first book is %q with %d pages", stats.Books[0].Title, stats.Books[0].Pages)
	}
	if stats.Books[1].Title != "Der letzte Wunsch" || stats.Books[1].Pages != 40 {
		t.Errorf("second book is %q with %d pages", stats.Books[1].Title, stats.Books[1].Pages)
	}
	if stats.Books[0].Finished {
		t.Error("a book nobody finished was reported as finished")
	}
}

// Finishing is worth saying, and it is the one thing in the summary that comes
// from the documents rather than from the precomputed rows.
func TestABookFinishedInThePeriodSaysSo(t *testing.T) {
	app := testutil.NewApp(t)
	user := wanting(t, app, schema.SummaryWeekly, "Europe/Vienna")

	storeDay(t, app, user.Id, "2026-08-11", 200, 7200)
	book := storeBook(t, app, testutil.PadId("booka"), user.Id, "Zeit des Sturms")
	storeBookDay(t, app, user.Id, book.Id, "2026-08-11", 100, 3600)

	// The last push of the week, at the end of the book.
	document := testutil.CreateDocument(t, app, user, testutil.PadId("docaa"),
		"043f11771ef9d191364ac0ba08198d36", 1.0,
		time.Date(2026, 8, 11, 20, 0, 0, 0, time.UTC))
	document.Set(schema.FieldBook, book.Id)
	if err := app.Save(document); err != nil {
		t.Fatalf("link the document: %v", err)
	}

	stats, err := summary.For(app, user, week)
	if err != nil {
		t.Fatalf("summarise: %v", err)
	}
	if len(stats.Books) != 1 || !stats.Books[0].Finished {
		t.Fatalf("expected the book to be reported as finished, got %+v", stats.Books)
	}
}

func TestTheAchievementsOfThePeriodAreIncluded(t *testing.T) {
	app := testutil.NewApp(t)
	user := wanting(t, app, schema.SummaryWeekly, "Europe/Vienna")

	storeDay(t, app, user.Id, "2026-08-11", 200, 7200)

	storeAward(t, app, user.Id, "night-prowler", 1, time.Date(2026, 8, 11, 22, 0, 0, 0, time.UTC))
	// Two days after the week ended, in the account's own zone.
	storeAward(t, app, user.Id, "first-pounce", 2, time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC))

	stats, err := summary.For(app, user, week)
	if err != nil {
		t.Fatalf("summarise: %v", err)
	}
	if len(stats.Achievements) != 1 {
		t.Fatalf("expected one achievement, got %d", len(stats.Achievements))
	}
	if stats.Achievements[0].Rule != "night-prowler" || stats.Achievements[0].Tier != 1 {
		t.Errorf("got %+v", stats.Achievements[0])
	}
}

// storeAward writes one earned tier.
func storeAward(t testing.TB, app core.App, owner, rule string, tier int, at time.Time) {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionAchievements)
	if err != nil {
		t.Fatalf("find achievements: %v", err)
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldOwner, owner)
	record.Set(schema.FieldRule, rule)
	record.Set(schema.FieldTier, tier)
	record.Set(schema.FieldValue, 3)
	record.Set(schema.FieldEarnedAt, at)

	if err := app.Save(record); err != nil {
		t.Fatalf("save the award: %v", err)
	}
}
