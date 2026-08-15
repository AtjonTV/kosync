//
// File:        internal/achievements/achievements_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package achievements_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/achievements"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// newApp returns a migrated app with one account and nothing read.
func newApp(t testing.TB) (*tests.TestApp, *core.Record) {
	t.Helper()

	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	return app, user
}

// valueOf returns what one rule measures for an account.
func valueOf(t testing.TB, app core.App, ownerId, slug string) int {
	t.Helper()

	progress, err := achievements.Measure(app, ownerId)
	if err != nil {
		t.Fatalf("failed to measure: %v", err)
	}

	for _, entry := range progress {
		if entry.Rule.Slug == slug {
			return entry.Value
		}
	}

	t.Fatalf("no rule named %q", slug)

	return 0
}

// readingDay stores a precomputed day, which is what the streak and the page
// count are measured from.
func readingDay(t testing.TB, app core.App, owner *core.Record, date string, pages int) {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("failed to find the reading days collection: %v", err)
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldOwner, owner.Id)
	record.Set(schema.FieldDate, date)
	record.Set(schema.FieldUpdateCount, 1)
	record.Set(schema.FieldPagesRead, pages)
	record.Set(schema.FieldComputedAt, time.Now().UTC())

	if err := app.Save(record); err != nil {
		t.Fatalf("failed to store the reading day %s: %v", date, err)
	}
}

// A book counts as finished because it was finished once, not because it still
// reads as finished: starting it again puts the progress back to the beginning.
func TestBooksFinishedCountsWhatWasEverFinished(t *testing.T) {
	app, user := newApp(t)
	now := time.Now().UTC()

	// Finished, then started again — the current position says 3%.
	restarted := testutil.CreateDocument(t, app, user, "", "hash-restarted", 0.03, now)
	testutil.CreateHistoryEntry(t, app, restarted, "", 1, now.Add(-48*time.Hour))

	// Finished and left alone.
	testutil.CreateDocument(t, app, user, "", "hash-done", 1, now)

	// Halfway, and never finished.
	testutil.CreateDocument(t, app, user, "", "hash-midway", 0.5, now)

	if got := valueOf(t, app, user.Id, "first-pounce"); got != 2 {
		t.Errorf("expected 2 finished books, got %d", got)
	}
}

func TestPagesReadSumsDaysAndTheMonthsTheyWereFoldedInto(t *testing.T) {
	app, user := newApp(t)

	readingDay(t, app, user, "2026-08-01", 120)
	readingDay(t, app, user, "2026-08-02", 80)

	// Retention folds aged out days into months and deletes them. Counting only
	// the days would quietly shrink a lifetime total every time it ran.
	months, err := app.FindCollectionByNameOrId(schema.CollectionReadingMonths)
	if err != nil {
		t.Fatalf("failed to find the reading months collection: %v", err)
	}
	rolled := core.NewRecord(months)
	rolled.Set(schema.FieldOwner, user.Id)
	rolled.Set(schema.FieldMonth, "2025-11")
	rolled.Set(schema.FieldPagesRead, 900)
	rolled.Set(schema.FieldDaysActive, 20)
	if err := app.Save(rolled); err != nil {
		t.Fatalf("failed to store the rolled up month: %v", err)
	}

	if got := valueOf(t, app, user.Id, "page-turner"); got != 1100 {
		t.Errorf("expected 1100 pages across days and months, got %d", got)
	}
}

func TestLongestStreakFindsTheLongestRun(t *testing.T) {
	app, user := newApp(t)

	// Three in a row, a gap, then four in a row.
	for _, date := range []string{
		"2026-07-01", "2026-07-02", "2026-07-03",
		"2026-07-10", "2026-07-11", "2026-07-12", "2026-07-13",
	} {
		readingDay(t, app, user, date, 10)
	}

	if got := valueOf(t, app, user.Id, "lap-warmer"); got != 4 {
		t.Errorf("expected the longest run of 4 days, got %d", got)
	}
}

// A night is named after the day it began, so one session that crosses midnight
// is one night and not two.
func TestLateNightsAreCountedInTheAccountsZone(t *testing.T) {
	app, user := newApp(t)
	user.Set(schema.FieldTimezone, "Europe/Vienna")
	if err := app.Save(user); err != nil {
		t.Fatalf("failed to set the timezone: %v", err)
	}

	// 22:30 UTC on the 14th is 00:30 on the 15th in Vienna: the night of the
	// 14th. 23:30 UTC is 01:30, the same night.
	document := testutil.CreateDocument(t, app, user, "", "hash-night", 0.5,
		time.Date(2026, 8, 14, 23, 30, 0, 0, time.UTC))
	testutil.CreateHistoryEntry(t, app, document, "", 0.4,
		time.Date(2026, 8, 14, 22, 30, 0, 0, time.UTC))

	// An afternoon, which is nobody's late night.
	testutil.CreateHistoryEntry(t, app, document, "", 0.2,
		time.Date(2026, 8, 12, 14, 0, 0, 0, time.UTC))

	if got := valueOf(t, app, user.Id, "night-prowler"); got != 1 {
		t.Errorf("expected one late night, got %d", got)
	}
}

// The same data in UTC is not a late night at all, which is why this rule could
// not be built before accounts had a zone.
func TestLateNightsInUTCAreDifferent(t *testing.T) {
	app, user := newApp(t)

	testutil.CreateDocument(t, app, user, "", "hash-night", 0.5,
		time.Date(2026, 8, 14, 23, 30, 0, 0, time.UTC))

	if got := valueOf(t, app, user.Id, "night-prowler"); got != 0 {
		t.Errorf("expected 23:30 UTC not to be past midnight in UTC, got %d", got)
	}
}

// The mirror of the late night, sharing its boundary: 05:00 belongs to the
// morning and 04:59 to the night before, and neither counts twice.
func TestEarlyMorningsAreCountedInTheAccountsZone(t *testing.T) {
	app, user := newApp(t)
	user.Set(schema.FieldTimezone, "Europe/Vienna")
	if err := app.Save(user); err != nil {
		t.Fatalf("failed to set the timezone: %v", err)
	}

	// 04:30 UTC is 06:30 in Vienna: an early morning. 05:10 UTC is 07:10, the
	// same morning, and one morning is what that should be.
	document := testutil.CreateDocument(t, app, user, "", "hash-morning", 0.5,
		time.Date(2026, 8, 14, 5, 10, 0, 0, time.UTC))
	testutil.CreateHistoryEntry(t, app, document, "", 0.4,
		time.Date(2026, 8, 14, 4, 30, 0, 0, time.UTC))

	// 07:30 in Vienna on another day: early enough.
	testutil.CreateHistoryEntry(t, app, document, "", 0.3,
		time.Date(2026, 8, 12, 5, 30, 0, 0, time.UTC))

	// 09:00 in Vienna, which is nobody's early start.
	testutil.CreateHistoryEntry(t, app, document, "", 0.2,
		time.Date(2026, 8, 10, 7, 0, 0, 0, time.UTC))

	// 02:00 in Vienna, which is the night before and the other rule's business.
	testutil.CreateHistoryEntry(t, app, document, "", 0.1,
		time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC))

	if got := valueOf(t, app, user.Id, "sunbeam-sitter"); got != 2 {
		t.Errorf("expected two early mornings, got %d", got)
	}
	if got := valueOf(t, app, user.Id, "night-prowler"); got != 1 {
		t.Errorf("expected the 02:00 reading to be a late night, got %d", got)
	}
}

func TestBestDayTakesTheDayAndNotTheTotal(t *testing.T) {
	app, user := newApp(t)

	readingDay(t, app, user, "2026-08-01", 40)
	readingDay(t, app, user, "2026-08-02", 210)
	readingDay(t, app, user, "2026-08-03", 90)

	if got := valueOf(t, app, user.Id, "the-long-sit"); got != 210 {
		t.Errorf("expected the best single day of 210 pages, got %d", got)
	}
}

// A book counts as re-read when it was begun again after it was finished. A low
// position before the finish is just where the first reading started.
func TestBooksRereadNeedsTheRestartToComeAfterTheFinish(t *testing.T) {
	app, user := newApp(t)
	start := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	// Started, finished, and started again: a re-read.
	reread := testutil.CreateDocument(t, app, user, "", "hash-reread", 0.04, start.Add(72*time.Hour))
	testutil.CreateHistoryEntry(t, app, reread, "", 0.02, start)
	testutil.CreateHistoryEntry(t, app, reread, "", 1, start.Add(48*time.Hour))

	// Finished and left alone at the last page.
	finished := testutil.CreateDocument(t, app, user, "", "hash-finished", 1, start.Add(24*time.Hour))
	testutil.CreateHistoryEntry(t, app, finished, "", 0.05, start)

	// Only just begun, and never finished at all.
	testutil.CreateDocument(t, app, user, "", "hash-begun", 0.03, start)

	if got := valueOf(t, app, user.Id, "nine-lives"); got != 1 {
		t.Errorf("expected one re-read book, got %d", got)
	}
}

func TestEvaluateAwardsEveryTierReached(t *testing.T) {
	app, user := newApp(t)

	// Well past the first two thresholds of the streak rule at once. A first
	// evaluation of an account that has been reading for years must not dribble
	// the tiers out one per day.
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for day := 0; day < 31; day++ {
		readingDay(t, app, user, start.AddDate(0, 0, day).Format("2006-01-02"), 5)
	}

	earned, err := achievements.Evaluate(app, user.Id)
	if err != nil {
		t.Fatalf("failed to evaluate: %v", err)
	}

	tiers := map[int]bool{}
	for _, award := range earned {
		if award.Rule.Slug == "lap-warmer" {
			tiers[award.Tier] = true
		}
	}
	if !tiers[1] || !tiers[2] {
		t.Errorf("expected both the 7 and 30 day tiers, got %v", tiers)
	}
	if tiers[3] {
		t.Errorf("expected the 100 day tier not to be awarded at 31 days")
	}
}

func TestEvaluateIsIdempotent(t *testing.T) {
	app, user := newApp(t)
	testutil.CreateDocument(t, app, user, "", "hash-done", 1, time.Now().UTC())

	first, err := achievements.Evaluate(app, user.Id)
	if err != nil {
		t.Fatalf("failed the first evaluation: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected the first evaluation to award something")
	}

	second, err := achievements.Evaluate(app, user.Id)
	if err != nil {
		t.Fatalf("failed the second evaluation: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("expected the second evaluation to award nothing, got %d", len(second))
	}

	stored, err := app.FindAllRecords(schema.CollectionAchievements,
		dbx.HashExp{schema.FieldOwner: user.Id})
	if err != nil {
		t.Fatalf("failed to load the achievements: %v", err)
	}
	if len(stored) != len(first) {
		t.Errorf("expected %d stored achievements, got %d", len(first), len(stored))
	}
}

// The measures are recomputed from live data and live data moves backwards.
// What was earned stays earned.
func TestAnAchievementIsNeverRevoked(t *testing.T) {
	app, user := newApp(t)

	document := testutil.CreateDocument(t, app, user, "", "hash-done", 1, time.Now().UTC())
	if _, err := achievements.Evaluate(app, user.Id); err != nil {
		t.Fatalf("failed to evaluate: %v", err)
	}

	// The reading is deleted, so nothing measures as finished any more.
	if err := app.Delete(document); err != nil {
		t.Fatalf("failed to delete the document: %v", err)
	}
	if got := valueOf(t, app, user.Id, "first-pounce"); got != 0 {
		t.Fatalf("expected the measure to fall back to 0, got %d", got)
	}

	if _, err := achievements.Evaluate(app, user.Id); err != nil {
		t.Fatalf("failed to re-evaluate: %v", err)
	}

	kept, err := app.FindAllRecords(schema.CollectionAchievements,
		dbx.HashExp{schema.FieldOwner: user.Id, schema.FieldRule: "first-pounce"})
	if err != nil {
		t.Fatalf("failed to load the achievements: %v", err)
	}
	if len(kept) != 1 {
		t.Errorf("expected the achievement to survive the reading being deleted, got %d", len(kept))
	}
}

func TestMeasureReportsTheNextThreshold(t *testing.T) {
	app, user := newApp(t)
	testutil.CreateDocument(t, app, user, "", "hash-done", 1, time.Now().UTC())

	progress, err := achievements.Measure(app, user.Id)
	if err != nil {
		t.Fatalf("failed to measure: %v", err)
	}

	for _, entry := range progress {
		if entry.Rule.Slug != "first-pounce" {
			continue
		}
		if entry.Tier != 1 {
			t.Errorf("expected the first tier at one finished book, got %d", entry.Tier)
		}
		if entry.Next != 10 {
			t.Errorf("expected the next threshold to be 10, got %d", entry.Next)
		}

		return
	}

	t.Fatal("first-pounce was not measured")
}

// The drawings live in the web interface and the names of them live here, so
// nothing in either tree fails when they disagree: a <use> of a symbol that does
// not exist renders as nothing at all, silently, exactly like a mistyped icon
// class. This is the only place the two halves can be checked against each other.
//
// It skips rather than fails when the interface is not beside the server, so a
// source tree with only the Go half still tests clean.
func TestEveryRuleIsDrawnByTheInterface(t *testing.T) {
	sprite, err := os.ReadFile("../../../webui/src/components/AchievementSprite.vue")
	if err != nil {
		t.Skipf("the web interface is not beside the server: %v", err)
	}
	badge, err := os.ReadFile("../../../webui/src/components/AchievementBadge.vue")
	if err != nil {
		t.Skipf("the web interface is not beside the server: %v", err)
	}

	for _, rule := range achievements.Rules {
		if !strings.Contains(string(sprite), `id="`+rule.Icon+`"`) {
			t.Errorf("rule %q names the drawing %q, which the sprite does not have",
				rule.Slug, rule.Icon)
		}
		if !strings.Contains(string(badge), ".fur-"+rule.Fur+" {") {
			t.Errorf("rule %q names the coat %q, which the badge does not have",
				rule.Slug, rule.Fur)
		}
	}
}

// Every rule has to have somewhere to draw itself from, or a badge renders as
// an empty circle and nothing says why.
func TestEveryRuleNamesADrawing(t *testing.T) {
	seen := map[string]bool{}

	for _, rule := range achievements.Rules {
		if rule.Slug == "" || rule.Name == "" || rule.Icon == "" || rule.Fur == "" {
			t.Errorf("rule %q is missing part of its identity", rule.Slug)
		}
		if len(rule.Tiers) != len(achievements.TierNames) {
			t.Errorf("rule %q has %d tiers, want %d", rule.Slug, len(rule.Tiers), len(achievements.TierNames))
		}
		if seen[rule.Slug] {
			t.Errorf("rule %q appears twice", rule.Slug)
		}
		seen[rule.Slug] = true

		// Ascending, or EarnedTier would report the wrong level.
		for index := 1; index < len(rule.Tiers); index++ {
			if rule.Tiers[index] <= rule.Tiers[index-1] {
				t.Errorf("rule %q has thresholds out of order: %v", rule.Slug, rule.Tiers)
			}
		}
	}
}
