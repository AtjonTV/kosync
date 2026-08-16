//
// File:        internal/statistics/statistics_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package statistics_test

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/statistics"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"git.obth.eu/atjontv/kosync/internal/timezone"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"

	_ "modernc.org/sqlite"
)

// The hash the reference EPUB produces, which is what KOReader stores as md5
// and what this server stores as the document.
const zeitDesSturms = "043f11771ef9d191364ac0ba08198d36"

// page is one row of a device's own record.
type page struct {
	md5      string
	page     int
	start    time.Time
	duration int
}

// build writes a statistics database of the shape KOReader keeps, in WAL mode
// as a real one is, and returns its path.
func build(t testing.TB, books map[string]string, pages []page) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "statistics.sqlite3")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	for _, statement := range []string{
		`PRAGMA journal_mode=WAL`,
		`CREATE TABLE book (id integer PRIMARY KEY autoincrement, title text, authors text,
			notes integer, last_open integer, highlights integer, pages integer,
			series text, language text, md5 text, total_read_time integer, total_read_pages integer)`,
		`CREATE TABLE page_stat_data (id_book integer, page integer NOT NULL DEFAULT 0,
			start_time integer NOT NULL DEFAULT 0, duration integer NOT NULL DEFAULT 0,
			total_pages integer NOT NULL DEFAULT 0, UNIQUE (id_book, page, start_time))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("build: %v", err)
		}
	}

	ids := map[string]int{}
	next := 1
	for hash, title := range books {
		if _, err := db.Exec(`INSERT INTO book (id, title, md5, pages) VALUES (?, ?, ?, 668)`,
			next, title, hash); err != nil {
			t.Fatalf("insert book: %v", err)
		}
		ids[hash] = next
		next++
	}

	for _, one := range pages {
		if _, err := db.Exec(
			`INSERT INTO page_stat_data (id_book, page, start_time, duration, total_pages) VALUES (?, ?, ?, ?, 668)`,
			ids[one.md5], one.page, one.start.Unix(), one.duration); err != nil {
			t.Fatalf("insert page: %v", err)
		}
	}

	return path
}

// vienna is the zone the fixture account reads in.
var vienna = func() *time.Location {
	location, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		panic(err)
	}

	return location
}()

// newApp returns an app with one account reading in Vienna.
func newApp(t testing.TB) (*tests.TestApp, *core.Record) {
	t.Helper()

	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	user.Set(schema.FieldTimezone, "Europe/Vienna")
	if err := app.Save(user); err != nil {
		t.Fatalf("set the zone: %v", err)
	}

	return app, user
}

// bounds returns the UTC range of one of the account's days.
func bounds(t testing.TB, date string) (string, string) {
	t.Helper()

	start, end, err := timezone.DayRange(vienna, date)
	if err != nil {
		t.Fatalf("resolve %s: %v", date, err)
	}

	return start, end
}

func TestThePageTurnsAreImported(t *testing.T) {
	app, user := newApp(t)

	path := build(t, map[string]string{zeitDesSturms: "Zeit des Sturms"}, []page{
		{zeitDesSturms, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, vienna), 60},
		{zeitDesSturms, 11, time.Date(2026, 8, 10, 20, 1, 0, 0, vienna), 45},
	})

	result, err := statistics.Import(app, user.Id, path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Rows != 2 || result.Added != 2 {
		t.Errorf("read %d rows and added %d, want 2 and 2", result.Rows, result.Added)
	}
	if len(result.Dates) != 1 || result.Dates[0] != "2026-08-10" {
		t.Errorf("dates are %v, want [2026-08-10]", result.Dates)
	}
}

// A device syncs the same database again next week, grown by a week. Only the
// week is new.
func TestImportingTheSameDatabaseTwiceAddsNothing(t *testing.T) {
	app, user := newApp(t)

	path := build(t, map[string]string{zeitDesSturms: "Zeit des Sturms"}, []page{
		{zeitDesSturms, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, vienna), 60},
		{zeitDesSturms, 11, time.Date(2026, 8, 10, 20, 1, 0, 0, vienna), 45},
	})

	if _, err := statistics.Import(app, user.Id, path); err != nil {
		t.Fatalf("first import: %v", err)
	}

	result, err := statistics.Import(app, user.Id, path)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if result.Rows != 2 {
		t.Errorf("read %d rows, want 2", result.Rows)
	}
	if result.Added != 0 {
		t.Errorf("added %d rows on a second import, want none", result.Added)
	}
	if len(result.Dates) != 0 {
		t.Errorf("queued %v, want nothing", result.Dates)
	}

	stored, err := app.FindAllRecords(schema.CollectionPageReads)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(stored) != 2 {
		t.Errorf("stored %d rows, want 2", len(stored))
	}
}

// The day a page turn belongs to is the reader's day, not UTC's. Late on a
// Vienna evening it is already tomorrow in UTC, and a summary that said so
// would move an evening's reading to the next morning.
func TestTheDayIsTheReadersOwn(t *testing.T) {
	app, user := newApp(t)

	path := build(t, map[string]string{zeitDesSturms: "Zeit des Sturms"}, []page{
		// 23:30 in Vienna on the 10th is 21:30 UTC on the 10th…
		{zeitDesSturms, 10, time.Date(2026, 8, 10, 23, 30, 0, 0, vienna), 60},
		// …and 00:30 on the 11th is 22:30 UTC on the 10th.
		{zeitDesSturms, 11, time.Date(2026, 8, 11, 0, 30, 0, 0, vienna), 60},
	})

	result, err := statistics.Import(app, user.Id, path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	sort.Strings(result.Dates)
	if len(result.Dates) != 2 || result.Dates[0] != "2026-08-10" || result.Dates[1] != "2026-08-11" {
		t.Errorf("dates are %v, want the 10th and the 11th", result.Dates)
	}
}

// A book KOReader could not hash is a book nothing here could ever attribute
// reading to.
func TestBooksWithoutAHashAreSkipped(t *testing.T) {
	app, user := newApp(t)

	path := build(t, map[string]string{"": "Unknown"}, []page{
		{"", 10, time.Date(2026, 8, 10, 20, 0, 0, 0, vienna), 60},
	})

	result, err := statistics.Import(app, user.Id, path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Rows != 0 || result.Added != 0 {
		t.Errorf("imported %d of %d rows, want none", result.Added, result.Rows)
	}
}

func TestTheMeasuredDayIsSummedAndCounted(t *testing.T) {
	app, user := newApp(t)

	other := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d"
	path := build(t,
		map[string]string{zeitDesSturms: "Zeit des Sturms", other: "Der letzte Wunsch"},
		[]page{
			{zeitDesSturms, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, vienna), 60},
			{zeitDesSturms, 11, time.Date(2026, 8, 10, 20, 1, 0, 0, vienna), 45},
			// The same page again in the evening: re-reading a paragraph is not
			// another page.
			{zeitDesSturms, 11, time.Date(2026, 8, 10, 22, 0, 0, 0, vienna), 30},
			{other, 3, time.Date(2026, 8, 10, 23, 0, 0, 0, vienna), 120},
			// The next day, which must not be counted in this one.
			{zeitDesSturms, 12, time.Date(2026, 8, 11, 9, 0, 0, 0, vienna), 999},
		})

	if _, err := statistics.Import(app, user.Id, path); err != nil {
		t.Fatalf("import: %v", err)
	}

	start, end := bounds(t, "2026-08-10")
	day, err := statistics.MeasuredDay(app, user.Id, start, end)
	if err != nil {
		t.Fatalf("measure: %v", err)
	}

	if day.Seconds != 255 {
		t.Errorf("seconds are %d, want 255", day.Seconds)
	}
	if day.Pages != 3 {
		t.Errorf("pages are %d, want 3 distinct", day.Pages)
	}
	if day.Documents != 2 {
		t.Errorf("documents are %d, want 2", day.Documents)
	}
}

// The books are matched by the hashes a book is known by, which is what makes
// this exact rather than a guess at a title.
func TestTheMeasuredBooksAreMatchedByHash(t *testing.T) {
	app, user := newApp(t)

	book := storeBook(t, app, user.Id, "Zeit des Sturms", zeitDesSturms)

	path := build(t, map[string]string{zeitDesSturms: "Zeit des Sturms"}, []page{
		{zeitDesSturms, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, vienna), 60},
		{zeitDesSturms, 11, time.Date(2026, 8, 10, 20, 1, 0, 0, vienna), 45},
	})

	if _, err := statistics.Import(app, user.Id, path); err != nil {
		t.Fatalf("import: %v", err)
	}

	start, end := bounds(t, "2026-08-10")
	rows, err := statistics.MeasuredBookDays(app, user.Id, start, end)
	if err != nil {
		t.Fatalf("measure: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("got %d books, want 1", len(rows))
	}
	if rows[0].Book != book.Id {
		t.Errorf("matched %q, want %q", rows[0].Book, book.Id)
	}
	if rows[0].Seconds != 105 || rows[0].Pages != 2 {
		t.Errorf("got %d seconds and %d pages, want 105 and 2", rows[0].Seconds, rows[0].Pages)
	}
}

// Reading in a file nobody has uploaded is real reading with no book to put it
// against. It counts in the day and not in the books.
func TestReadingWithoutABookCountsInTheDayOnly(t *testing.T) {
	app, user := newApp(t)

	path := build(t, map[string]string{zeitDesSturms: "Zeit des Sturms"}, []page{
		{zeitDesSturms, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, vienna), 60},
	})

	if _, err := statistics.Import(app, user.Id, path); err != nil {
		t.Fatalf("import: %v", err)
	}

	start, end := bounds(t, "2026-08-10")

	day, err := statistics.MeasuredDay(app, user.Id, start, end)
	if err != nil {
		t.Fatalf("measure the day: %v", err)
	}
	if day.Seconds != 60 {
		t.Errorf("the day lost the reading: %d seconds", day.Seconds)
	}

	rows, err := statistics.MeasuredBookDays(app, user.Id, start, end)
	if err != nil {
		t.Fatalf("measure the books: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("got %d books for a document nobody uploaded", len(rows))
	}
}

// One account's device never speaks for another's days.
func TestMeasurementsAreOwnedByOneAccount(t *testing.T) {
	app, user := newApp(t)
	other := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)

	path := build(t, map[string]string{zeitDesSturms: "Zeit des Sturms"}, []page{
		{zeitDesSturms, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, vienna), 60},
	})

	if _, err := statistics.Import(app, user.Id, path); err != nil {
		t.Fatalf("import: %v", err)
	}

	start, end := bounds(t, "2026-08-10")
	day, err := statistics.MeasuredDay(app, other.Id, start, end)
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	if !day.IsEmpty() {
		t.Errorf("the other account was given %d seconds of somebody else's reading", day.Seconds)
	}
}

func TestImportingNeedsAnAccount(t *testing.T) {
	app, _ := newApp(t)

	path := build(t, map[string]string{zeitDesSturms: "Zeit des Sturms"}, []page{
		{zeitDesSturms, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, vienna), 60},
	})

	if _, err := statistics.Import(app, "", path); err == nil {
		t.Error("an import without an account was allowed")
	}
}

// storeBook puts a book in the library under a given binary hash.
func storeBook(t testing.TB, app core.App, owner, title, hash string) *core.Record {
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
	record.Set(schema.FieldOwner, owner)
	record.Set(schema.FieldFile, file)
	record.Set(schema.FieldTitle, title)
	record.Set(schema.FieldContentHash, fmt.Sprintf("%064s", hash))
	record.Set(schema.FieldHashBinary, hash)

	if err := app.Save(record); err != nil {
		t.Fatalf("save the book: %v", err)
	}

	return record
}
