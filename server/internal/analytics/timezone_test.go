//
// File:        internal/analytics/timezone_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/analytics"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// inVienna sets the account's zone without going through the request hooks, so
// that a test can arrange the world before asking a question about it.
func inVienna(t testing.TB, app core.App, user *core.Record) {
	t.Helper()

	user.Set(schema.FieldTimezone, "Europe/Vienna")
	if err := app.Save(user); err != nil {
		t.Fatalf("failed to set the timezone: %v", err)
	}
}

// queuedDays returns the days waiting to be recomputed for an account.
func queuedDays(t testing.TB, app core.App, ownerId string) map[string]bool {
	t.Helper()

	items, err := app.FindAllRecords(schema.CollectionAnalyticsQueue,
		dbx.HashExp{schema.FieldOwner: ownerId})
	if err != nil {
		t.Fatalf("failed to read the analytics queue: %v", err)
	}

	days := map[string]bool{}
	for _, item := range items {
		days[item.GetString(schema.FieldDate)] = true
	}

	return days
}

// An hour past midnight in Vienna is still yesterday in UTC. Counting it as
// yesterday is what breaks a streak for someone who reads late.
func TestADayIsTheAccountsDayNotUTC(t *testing.T) {
	app, user := newApp(t)
	inVienna(t, app, user)

	// 21:00 and 23:00 UTC on the 14th are 23:00 on the 14th and 01:00 on the
	// 15th in Vienna: one reading each, on two different local days.
	document := testutil.CreateDocument(t, app, user, "", "hash-late", 0.4,
		time.Date(2026, 8, 14, 23, 0, 0, 0, time.UTC))
	testutil.CreateHistoryEntry(t, app, document, "", 0.2,
		time.Date(2026, 8, 14, 21, 0, 0, 0, time.UTC))

	fourteenth, err := analytics.ComputeDay(app, user.Id, "2026-08-14", sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the 14th: %v", err)
	}
	fifteenth, err := analytics.ComputeDay(app, user.Id, "2026-08-15", sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the 15th: %v", err)
	}

	if fourteenth.UpdateCount != 1 {
		t.Errorf("expected the 23:00 local reading on the 14th, got %d updates", fourteenth.UpdateCount)
	}
	if fifteenth.UpdateCount != 1 {
		t.Errorf("expected the 01:00 local reading on the 15th, got %d updates", fifteenth.UpdateCount)
	}
}

// The same data in UTC is one day with both readings on it, which is the
// behaviour every account keeps until it chooses a zone.
func TestAnAccountWithoutAZoneIsUnchanged(t *testing.T) {
	app, user := newApp(t)

	document := testutil.CreateDocument(t, app, user, "", "hash-late", 0.4,
		time.Date(2026, 8, 14, 23, 0, 0, 0, time.UTC))
	testutil.CreateHistoryEntry(t, app, document, "", 0.2,
		time.Date(2026, 8, 14, 21, 0, 0, 0, time.UTC))

	stats, err := analytics.ComputeDay(app, user.Id, "2026-08-14", sessionGap)
	if err != nil {
		t.Fatalf("failed to compute the day: %v", err)
	}
	if stats.UpdateCount != 2 {
		t.Errorf("expected both readings on the UTC day, got %d updates", stats.UpdateCount)
	}
}

func TestAPushIsQueuedUnderTheAccountsDay(t *testing.T) {
	app, user := newApp(t)
	analytics.Register(app, normalisedConfig())
	inVienna(t, app, user)

	// 23:30 UTC on the 14th is 01:30 on the 15th in Vienna.
	testutil.CreateDocument(t, app, user, "", "hash-late", 0.4,
		time.Date(2026, 8, 14, 23, 30, 0, 0, time.UTC))

	days := queuedDays(t, app, user.Id)
	if !days["2026-08-15"] {
		t.Errorf("expected the local day to be queued, the queue holds %v", days)
	}
}

// Changing the zone moves every boundary, so every day has to be worked out
// again — including the ones the old boundaries produced, which nothing else
// would ever revisit.
func TestChangingTheTimezoneRequeuesEverything(t *testing.T) {
	app, user := newApp(t)
	analytics.Register(app, normalisedConfig())

	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.4,
		time.Date(2026, 8, 14, 23, 0, 0, 0, time.UTC))
	testutil.CreateHistoryEntry(t, app, document, "", 0.2,
		time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC))

	// Clear what the writes above queued, so the assertion is about the change.
	for _, item := range mustAll(t, app, schema.CollectionAnalyticsQueue) {
		if err := app.Delete(item); err != nil {
			t.Fatalf("failed to clear the queue: %v", err)
		}
	}

	user.Set(schema.FieldTimezone, "Europe/Vienna")
	if err := app.Save(user); err != nil {
		t.Fatalf("failed to change the timezone: %v", err)
	}

	days := queuedDays(t, app, user.Id)
	for _, want := range []string{"2026-06-02", "2026-08-15"} {
		if !days[want] {
			t.Errorf("expected %s to be requeued, the queue holds %v", want, days)
		}
	}
}

// A zone the database does not know must be refused where a person can be told
// about it, not swallowed into a silent fallback months later.
func TestAnUnknownTimezoneIsRefused(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "an invented zone name",
		Method: http.MethodPatch,
		URL:    "/api/collections/users/records/" + testutil.IdUserA,
		Body:   strings.NewReader(`{"timezone":"Middle/Earth"}`),
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			fresh := testutil.NewApp(t)
			testutil.CreateUser(t, fresh, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
			analytics.Register(fresh, normalisedConfig())

			return fresh
		},
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			record, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserA)
			if err != nil {
				t.Fatalf("failed to load the fixture user: %v", err)
			}
			scenarioHeaders["Authorization"] = testutil.UserToken(t, record)
		},
		Headers:         scenarioHeaders,
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"not a known timezone"},
	}

	scenario.Test(t)
}

// scenarioHeaders is filled in by BeforeTestFunc, which is the only place the
// token can be minted: the signing key belongs to the app the scenario built.
var scenarioHeaders = map[string]string{"Content-Type": "application/json"}

func normalisedConfig() *config.Config {
	conf := &config.Config{}
	conf.Normalize()

	return conf
}

func mustAll(t testing.TB, app core.App, collection string) []*core.Record {
	t.Helper()

	records, err := app.FindAllRecords(collection)
	if err != nil {
		t.Fatalf("failed to read %q: %v", collection, err)
	}

	return records
}
