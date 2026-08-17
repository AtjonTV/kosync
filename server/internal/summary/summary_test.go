//
// File:        internal/summary/summary_test.go
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
)

// vienna is a zone that is neither UTC nor a whole number of hours from
// everything else, and that observes daylight saving.
var vienna = mustLoad("Europe/Vienna")

func mustLoad(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}

	return location
}

func TestTheWeekIsTheOneThatHasFinished(t *testing.T) {
	cases := []struct {
		name  string
		local time.Time
		key   string
		from  string
		to    string
	}{
		// Monday morning: the week that ended yesterday.
		{"monday", time.Date(2026, 8, 17, 9, 0, 0, 0, vienna), "2026-W33", "2026-08-10", "2026-08-16"},
		// Later in the same week, the answer must not move: the current week is
		// still being read and summarising it would be wrong by the evening.
		{"thursday", time.Date(2026, 8, 20, 9, 0, 0, 0, vienna), "2026-W33", "2026-08-10", "2026-08-16"},
		// Sunday night is still the same week, right up to midnight.
		{"sunday", time.Date(2026, 8, 23, 23, 30, 0, 0, vienna), "2026-W33", "2026-08-10", "2026-08-16"},
		// And then it moves on.
		{"next monday", time.Date(2026, 8, 24, 8, 0, 0, 0, vienna), "2026-W34", "2026-08-17", "2026-08-23"},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			period, ok := summary.LastCompleted(schema.SummaryWeekly, one.local)
			if !ok {
				t.Fatal("expected a completed week")
			}
			if period.Key != one.key || period.From != one.from || period.To != one.to {
				t.Errorf("got %s (%s to %s), want %s (%s to %s)",
					period.Key, period.From, period.To, one.key, one.from, one.to)
			}
		})
	}
}

func TestAWeekIsNamedByItsNumberAndItsDays(t *testing.T) {
	cases := []struct {
		name  string
		local time.Time
		title string
	}{
		// The ordinary case: one month, so it is named once.
		{"within a month", time.Date(2026, 8, 17, 9, 0, 0, 0, vienna), "week 33 (10. - 16. August)"},
		// A week that straddles two months has to name both.
		{"across a month", time.Date(2026, 8, 3, 9, 0, 0, 0, vienna), "week 31 (27. July - 2. August)"},
		// And one that straddles two years has to name those as well. Its ISO
		// number is 53 of 2026, the year it started in, even though most of it
		// is in 2027 — which is exactly why the days are spelled out.
		{"across a year", time.Date(2027, 1, 4, 9, 0, 0, 0, vienna),
			"week 53 (28. December 2026 - 3. January 2027)"},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			period, ok := summary.LastCompleted(schema.SummaryWeekly, one.local)
			if !ok {
				t.Fatal("expected a completed week")
			}
			if period.Title != one.title {
				t.Errorf("title is %q, want %q", period.Title, one.title)
			}
		})
	}
}

func TestTheMonthIsTheOneThatHasFinished(t *testing.T) {
	period, ok := summary.LastCompleted(schema.SummaryMonthly, time.Date(2026, 8, 1, 8, 0, 0, 0, vienna))
	if !ok {
		t.Fatal("expected a completed month")
	}
	if period.Key != "2026-07" || period.From != "2026-07-01" || period.To != "2026-07-31" {
		t.Errorf("got %s (%s to %s), want 2026-07 (2026-07-01 to 2026-07-31)",
			period.Key, period.From, period.To)
	}
	if period.Title != "July 2026" {
		t.Errorf("title is %q", period.Title)
	}

	// The first of January looks back into the previous year, which is the one
	// place the arithmetic could quietly produce month zero.
	period, _ = summary.LastCompleted(schema.SummaryMonthly, time.Date(2027, 1, 1, 8, 0, 0, 0, vienna))
	if period.Key != "2026-12" || period.From != "2026-12-01" || period.To != "2026-12-31" {
		t.Errorf("across the new year: got %s (%s to %s)", period.Key, period.From, period.To)
	}
}

func TestACadenceNobodyChoseIsNotACadence(t *testing.T) {
	for _, kind := range []string{"", schema.SummaryOff, "fortnightly"} {
		if _, ok := summary.LastCompleted(kind, time.Now()); ok {
			t.Errorf("%q produced a period", kind)
		}
	}
}

// wanting returns an account with the given cadence and zone.
func wanting(t testing.TB, app core.App, cadence, zone string) *core.Record {
	t.Helper()

	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	user.Set(schema.FieldSummaryMail, cadence)
	user.Set(schema.FieldTimezone, zone)
	if err := app.Save(user); err != nil {
		t.Fatalf("save the account: %v", err)
	}

	return user
}

func TestNothingIsDueBeforeBreakfast(t *testing.T) {
	app := testutil.NewApp(t)
	user := wanting(t, app, schema.SummaryWeekly, "Europe/Vienna")

	// 03:00 UTC on a Monday is 05:00 in Vienna, which is nobody's idea of when
	// to be told about last week.
	if _, due := summary.Due(user, time.Date(2026, 8, 17, 3, 0, 0, 0, time.UTC)); due {
		t.Error("a summary was due at five in the morning")
	}

	// 07:00 UTC is 09:00 in Vienna, and the week is over.
	period, due := summary.Due(user, time.Date(2026, 8, 17, 7, 0, 0, 0, time.UTC))
	if !due {
		t.Fatal("expected the summary to be due")
	}
	if period.Key != "2026-W33" {
		t.Errorf("period is %q", period.Key)
	}
}

// The whole point of writing the period down: the hourly job must not send the
// same summary again an hour later.
func TestASummaryIsOnlyDueOnce(t *testing.T) {
	app := testutil.NewApp(t)
	user := wanting(t, app, schema.SummaryWeekly, "Europe/Vienna")

	now := time.Date(2026, 8, 17, 7, 0, 0, 0, time.UTC)
	period, due := summary.Due(user, now)
	if !due {
		t.Fatal("expected the summary to be due")
	}

	user.Set(schema.FieldSummarySent, period.Key)

	if _, due := summary.Due(user, now.Add(time.Hour)); due {
		t.Error("the same summary came due a second time")
	}
	// But the next week's does.
	if _, due := summary.Due(user, now.AddDate(0, 0, 7)); !due {
		t.Error("the following week never came due")
	}
}

// A server that was off all weekend still owes Monday's summary on Wednesday.
func TestALateServerStillOwesTheSummary(t *testing.T) {
	app := testutil.NewApp(t)
	user := wanting(t, app, schema.SummaryWeekly, "Europe/Vienna")

	period, due := summary.Due(user, time.Date(2026, 8, 19, 7, 0, 0, 0, time.UTC))
	if !due {
		t.Fatal("expected the missed summary to still be due")
	}
	if period.Key != "2026-W33" {
		t.Errorf("period is %q, want the week that was missed", period.Key)
	}
}

func TestAnAccountThatAskedForNothingIsNeverDue(t *testing.T) {
	app := testutil.NewApp(t)
	user := wanting(t, app, schema.SummaryOff, "Europe/Vienna")

	if _, due := summary.Due(user, time.Date(2026, 8, 17, 7, 0, 0, 0, time.UTC)); due {
		t.Error("an account that wants no summary was sent one")
	}
}

func TestWantedListsOnlyTheAccountsThatAsked(t *testing.T) {
	app := testutil.SeededApp(t)

	alice, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserA)
	if err != nil {
		t.Fatalf("load alice: %v", err)
	}
	alice.Set(schema.FieldSummaryMail, schema.SummaryMonthly)
	if err := app.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	bob, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserB)
	if err != nil {
		t.Fatalf("load bob: %v", err)
	}
	bob.Set(schema.FieldSummaryMail, schema.SummaryOff)
	if err := app.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}

	wanted, err := summary.Wanted(app)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(wanted) != 1 || wanted[0].Id != testutil.IdUserA {
		t.Fatalf("expected only alice, got %d accounts", len(wanted))
	}
}

func TestDurationsAreWrittenTheWayPeopleSayThem(t *testing.T) {
	cases := []struct {
		seconds int
		want    string
	}{
		{0, "under a minute"},
		{59, "under a minute"},
		{60, "1 minute"},
		{45 * 60, "45 minutes"},
		{3600, "1 hour"},
		{2 * 3600, "2 hours"},
		{3600 + 5*60, "1h 05m"},
	}

	for _, one := range cases {
		if got := summary.Duration(one.seconds); got != one.want {
			t.Errorf("Duration(%d) = %q, want %q", one.seconds, got, one.want)
		}
	}
}
