//
// File:        internal/mail/summary_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package mail_test

import (
	"strings"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/mail"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/types"
)

// mondayMorning is 09:00 in Vienna, on the first Monday after the week the
// fixture reads in.
var mondayMorning = time.Date(2026, 8, 17, 7, 0, 0, 0, time.UTC)

// readingApp returns an app with one account that wants a summary of the given
// cadence and has read during the week of 10 August.
func readingApp(t testing.TB, cadence string) (*tests.TestApp, *core.Record) {
	t.Helper()

	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	user.Set(schema.FieldSummaryMail, cadence)
	user.Set(schema.FieldTimezone, "Europe/Vienna")
	if err := app.Save(user); err != nil {
		t.Fatalf("ask for the summary: %v", err)
	}

	return app, user
}

// storeDay writes one precomputed reading day.
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

// storeBookWithDay puts a book in the library and a day of reading in it.
func storeBookWithDay(t testing.TB, app core.App, owner, title, date string, pages int) {
	t.Helper()

	books, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("find books: %v", err)
	}

	file, err := filesystem.NewFileFromBytes([]byte("PK\x03\x04 stand-in"), "book.epub")
	if err != nil {
		t.Fatalf("build file: %v", err)
	}

	book := core.NewRecord(books)
	book.Id = testutil.PadId("booka")
	book.Set(schema.FieldOwner, owner)
	book.Set(schema.FieldFile, file)
	book.Set(schema.FieldTitle, title)
	book.Set(schema.FieldContentHash, book.Id)
	if err := app.Save(book); err != nil {
		t.Fatalf("save the book: %v", err)
	}

	days, err := app.FindCollectionByNameOrId(schema.CollectionReadingBookDays)
	if err != nil {
		t.Fatalf("find reading_book_days: %v", err)
	}

	day := core.NewRecord(days)
	day.Set(schema.FieldOwner, owner)
	day.Set(schema.FieldBook, book.Id)
	day.Set(schema.FieldDate, date)
	day.Set(schema.FieldUpdateCount, 3)
	day.Set(schema.FieldProgressIncrease, 2.0)
	day.Set(schema.FieldReadingTime, 3600)
	day.Set(schema.FieldPagesRead, pages)
	day.Set(schema.FieldComputedAt, types.NowDateTime())

	if err := app.Save(day); err != nil {
		t.Fatalf("save the book day: %v", err)
	}
}

func TestAWeeklySummaryLeadsWithThePages(t *testing.T) {
	app, user := readingApp(t, schema.SummaryWeekly)
	storeDay(t, app, user.Id, "2026-08-11", 190, 8100)
	storeBookWithDay(t, app, user.Id, "Zeit des Sturms", "2026-08-11", 160)

	sent, err := mail.Summaries(app, mondayMorning)
	if err != nil {
		t.Fatalf("send the summaries: %v", err)
	}
	if sent != 1 {
		t.Fatalf("sent %d summaries, want 1", sent)
	}
	if app.TestMailer.TotalSend() != 1 {
		t.Fatalf("expected one message, got %d", app.TestMailer.TotalSend())
	}

	message := app.TestMailer.FirstMessage()
	if !strings.Contains(message.Subject, "190 pages last week") {
		t.Errorf("subject is %q", message.Subject)
	}
	if len(message.To) != 1 || message.To[0].Address != testutil.EmailUserA {
		t.Errorf("the summary went to %v", message.To)
	}
	for _, want := range []string{"Zeit des Sturms", "160 pages", "one day"} {
		if !strings.Contains(message.Text, want) {
			t.Errorf("expected %q in the body, got %q", want, message.Text)
		}
	}
	if message.HTML == "" {
		t.Error("expected an HTML body as well")
	}

	// The period is written down, which is what stops the next hourly run
	// sending it again.
	stored, err := app.FindRecordById(schema.CollectionUsers, user.Id)
	if err != nil {
		t.Fatalf("reload the account: %v", err)
	}
	if got := stored.GetString(schema.FieldSummarySent); got != "2026-W33" {
		t.Errorf("recorded period is %q, want 2026-W33", got)
	}
}

// The job runs every hour. It must send one summary a week.
func TestTheSameSummaryIsNotSentTwice(t *testing.T) {
	app, user := readingApp(t, schema.SummaryWeekly)
	storeDay(t, app, user.Id, "2026-08-11", 190, 8100)

	for hour := range 5 {
		if _, err := mail.Summaries(app, mondayMorning.Add(time.Duration(hour)*time.Hour)); err != nil {
			t.Fatalf("send the summaries: %v", err)
		}
	}

	if app.TestMailer.TotalSend() != 1 {
		t.Errorf("five runs sent %d messages, want 1", app.TestMailer.TotalSend())
	}
}

// A week nobody read in is not worth a message, and must not become one next
// hour either.
func TestAWeekWithNoReadingIsNotMailed(t *testing.T) {
	app, user := readingApp(t, schema.SummaryWeekly)

	if _, err := mail.Summaries(app, mondayMorning); err != nil {
		t.Fatalf("send the summaries: %v", err)
	}
	if app.TestMailer.TotalSend() != 0 {
		t.Errorf("expected no message, got %d", app.TestMailer.TotalSend())
	}

	stored, err := app.FindRecordById(schema.CollectionUsers, user.Id)
	if err != nil {
		t.Fatalf("reload the account: %v", err)
	}
	if got := stored.GetString(schema.FieldSummarySent); got != "2026-W33" {
		t.Errorf("the empty week was not marked as covered, got %q", got)
	}
}

func TestNothingIsSentBeforeTheLocalMorning(t *testing.T) {
	app, user := readingApp(t, schema.SummaryWeekly)
	storeDay(t, app, user.Id, "2026-08-11", 190, 8100)

	// 03:00 UTC is 05:00 in Vienna.
	if _, err := mail.Summaries(app, time.Date(2026, 8, 17, 3, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("send the summaries: %v", err)
	}
	if app.TestMailer.TotalSend() != 0 {
		t.Errorf("a summary arrived at five in the morning")
	}
}

func TestAMonthlySummaryNamesTheMonth(t *testing.T) {
	app, user := readingApp(t, schema.SummaryMonthly)
	storeDay(t, app, user.Id, "2026-07-14", 640, 20000)

	// The first of August, in the morning.
	if _, err := mail.Summaries(app, time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("send the summaries: %v", err)
	}
	if app.TestMailer.TotalSend() != 1 {
		t.Fatalf("expected one message, got %d", app.TestMailer.TotalSend())
	}

	message := app.TestMailer.FirstMessage()
	if !strings.Contains(message.Subject, "640 pages in July 2026") {
		t.Errorf("subject is %q", message.Subject)
	}

	stored, err := app.FindRecordById(schema.CollectionUsers, user.Id)
	if err != nil {
		t.Fatalf("reload the account: %v", err)
	}
	if got := stored.GetString(schema.FieldSummarySent); got != "2026-07" {
		t.Errorf("recorded period is %q, want 2026-07", got)
	}
}

// An account that never chose a cadence hears nothing, which is what makes this
// safe to add to a server people are already using.
func TestAnAccountThatAskedForNothingGetsNothing(t *testing.T) {
	app, user := readingApp(t, schema.SummaryOff)
	storeDay(t, app, user.Id, "2026-08-11", 190, 8100)

	if _, err := mail.Summaries(app, mondayMorning); err != nil {
		t.Fatalf("send the summaries: %v", err)
	}
	if app.TestMailer.TotalSend() != 0 {
		t.Errorf("expected nothing to be sent, got %d", app.TestMailer.TotalSend())
	}
}

func TestNoSummaryGoesToAnUnverifiedAddress(t *testing.T) {
	app, user := readingApp(t, schema.SummaryWeekly)
	storeDay(t, app, user.Id, "2026-08-11", 190, 8100)

	user.SetVerified(false)
	if err := app.Save(user); err != nil {
		t.Fatalf("unverify: %v", err)
	}

	if _, err := mail.Summaries(app, mondayMorning); err != nil {
		t.Fatalf("send the summaries: %v", err)
	}
	if app.TestMailer.TotalSend() != 0 {
		t.Errorf("expected nothing to be sent, got %d", app.TestMailer.TotalSend())
	}
}
