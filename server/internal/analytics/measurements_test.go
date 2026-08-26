//
// File:        internal/analytics/measurements_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/analytics"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	_ "modernc.org/sqlite"
)

// measuredPage is one row of a device's own record.
//
// total is the page count the book stood at when the turn was recorded — a
// device's own number, which moves with the font. Zero means measuredPages, so
// that a test about something else does not have to say.
type measuredPage struct {
	md5      string
	page     int
	start    time.Time
	duration int
	total    int
}

// measuredPages is the pagination a fixture is in unless it says otherwise.
const measuredPages = 668

// buildStatistics writes a KOReader statistics database and returns its path.
func buildStatistics(t testing.TB, hash string, pages []measuredPage) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "statistics.sqlite3")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	for _, statement := range []string{
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

	if _, err := db.Exec(`INSERT INTO book (id, title, md5, pages) VALUES (1, 'Zeit des Sturms', ?, ?)`,
		hash, measuredPages); err != nil {
		t.Fatalf("insert book: %v", err)
	}
	for _, one := range pages {
		total := one.total
		if total == 0 {
			total = measuredPages
		}

		if _, err := db.Exec(
			`INSERT INTO page_stat_data (id_book, page, start_time, duration, total_pages) VALUES (1, ?, ?, ?, ?)`,
			one.page, one.start.Unix(), one.duration, total); err != nil {
			t.Fatalf("insert page: %v", err)
		}
	}

	return path
}

// storedDay returns the stored statistics row of one day, if there is one.
func storedDay(t testing.TB, app core.App, owner, date string) *core.Record {
	t.Helper()

	record, err := app.FindFirstRecordByFilter(
		schema.CollectionReadingDays,
		"owner = {:owner} && date = {:date}",
		dbx.Params{"owner": owner, "date": date},
	)
	if err != nil {
		return nil
	}

	return record
}

// This is the bug the whole feature exists for. A device that read offline and
// synced later has no pushes on the days it read, so the statistics have no row
// for them at all: fifteen such days on the reference instance, invisible.
func TestADayWithNoPushesAtAllIsStillADayOfReading(t *testing.T) {
	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	path := buildStatistics(t, testutil.DocumentHashA, []measuredPage{
		{testutil.DocumentHashA, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, time.UTC), 600, 0},
		{testutil.DocumentHashA, 11, time.Date(2026, 8, 10, 20, 10, 0, 0, time.UTC), 540, 0},
	})

	if storedDay(t, app, user.Id, "2026-08-10") != nil {
		t.Fatal("the day exists before anything was imported")
	}

	result, err := analytics.ImportMeasurements(app, user.Id, path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(result.Dates) != 1 {
		t.Fatalf("queued %v, want one day", result.Dates)
	}

	if err := analytics.RecomputeDay(app, user.Id, "2026-08-10", 5*time.Minute); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	day := storedDay(t, app, user.Id, "2026-08-10")
	if day == nil {
		t.Fatal("a day the device measured produced no row")
	}
	if got := day.GetInt(schema.FieldReadingTime); got != 1140 {
		t.Errorf("reading time is %d, want 1140 seconds", got)
	}
	if got := day.GetInt(schema.FieldPagesRead); got != 2 {
		t.Errorf("pages read is %d, want 2", got)
	}
}

// Where both have an opinion, the measurement wins: it was counted, the other
// was deduced from when pushes happened to arrive.
func TestTheMeasurementBeatsTheInference(t *testing.T) {
	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	// Two pushes a minute apart: the inferred reading time is 60 seconds.
	base := time.Date(2026, 8, 10, 20, 0, 0, 0, time.UTC)
	document := testutil.CreateDocument(t, app, user, "", testutil.DocumentHashA, 0.20, base)
	testutil.CreateHistoryEntry(t, app, document, "", 0.10, base.Add(-time.Minute))

	if err := analytics.RecomputeDay(app, user.Id, "2026-08-10", 5*time.Minute); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	inferred := storedDay(t, app, user.Id, "2026-08-10")
	if inferred == nil {
		t.Fatal("the pushes produced no day")
	}
	if got := inferred.GetInt(schema.FieldReadingTime); got != 60 {
		t.Fatalf("inferred reading time is %d, want 60", got)
	}

	// The device says that hour was really half an hour of reading.
	path := buildStatistics(t, testutil.DocumentHashA, []measuredPage{
		{testutil.DocumentHashA, 10, base, 900, 0},
		{testutil.DocumentHashA, 11, base.Add(15 * time.Minute), 900, 0},
	})
	if _, err := analytics.ImportMeasurements(app, user.Id, path); err != nil {
		t.Fatalf("import: %v", err)
	}
	if err := analytics.RecomputeDay(app, user.Id, "2026-08-10", 5*time.Minute); err != nil {
		t.Fatalf("recompute: %v", err)
	}

	measured := storedDay(t, app, user.Id, "2026-08-10")
	if got := measured.GetInt(schema.FieldReadingTime); got != 1800 {
		t.Errorf("reading time is %d, want the measured 1800", got)
	}
	// The pushes still own everything they are the only witness to.
	if got := measured.GetInt(schema.FieldUpdateCount); got != 2 {
		t.Errorf("update count is %d, want the pushes' 2", got)
	}
	if got := measured.GetFloat(schema.FieldProgressIncrease); got <= 0 {
		t.Errorf("progress increase is %v, want the pushes' own", got)
	}
}

// A day nothing has anything to say about is still removed rather than stored
// as a row of zeroes.
func TestADayWithNeitherIsStillDeleted(t *testing.T) {
	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	if err := analytics.RecomputeDay(app, user.Id, "2026-08-10", 5*time.Minute); err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if storedDay(t, app, user.Id, "2026-08-10") != nil {
		t.Error("an empty day was stored")
	}
}

// The import queues the days it learned about, so that nothing has to know in
// advance which months a device has been reading in.
func TestTheImportQueuesTheDaysItLearnedAbout(t *testing.T) {
	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	path := buildStatistics(t, testutil.DocumentHashA, []measuredPage{
		{testutil.DocumentHashA, 10, time.Date(2026, 8, 10, 20, 0, 0, 0, time.UTC), 60, 0},
		{testutil.DocumentHashA, 11, time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC), 60, 0},
	})

	if _, err := analytics.ImportMeasurements(app, user.Id, path); err != nil {
		t.Fatalf("import: %v", err)
	}

	queued, err := app.FindAllRecords(schema.CollectionAnalyticsQueue)
	if err != nil {
		t.Fatalf("list the queue: %v", err)
	}
	if len(queued) != 2 {
		t.Fatalf("queued %d days, want 2", len(queued))
	}
}
