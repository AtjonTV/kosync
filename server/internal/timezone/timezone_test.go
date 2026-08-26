//
// File:        internal/timezone/timezone_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package timezone_test

import (
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/timezone"
)

// The zone database has to be in the binary, or every one of these would pass
// by falling back to UTC on a machine without /usr/share/zoneinfo.
func TestTheZoneDatabaseIsAvailable(t *testing.T) {
	if !timezone.Valid("Europe/Vienna") {
		t.Fatal("expected Europe/Vienna to be a known zone; is time/tzdata imported?")
	}
	if timezone.Valid("Middle/Earth") {
		t.Error("expected an invented zone to be rejected")
	}
	if timezone.Valid("") {
		t.Error("expected an empty name to be rejected")
	}
}

func TestAnUnknownZoneFallsBackToUTC(t *testing.T) {
	// Not an error: this is reached from a stored field, and a statistics run is
	// the wrong place to discover a name was mistyped months ago.
	if got := timezone.Load("Middle/Earth"); got != time.UTC {
		t.Errorf("expected UTC for an unknown zone, got %v", got)
	}
	if got := timezone.Load(""); got != time.UTC {
		t.Errorf("expected UTC for an empty name, got %v", got)
	}
}

// The case the whole feature exists for: an hour past midnight in Vienna is
// still the previous day in UTC, so bucketing by UTC files that reading under
// the wrong date.
func TestALateNightBelongsToTheLocalDay(t *testing.T) {
	vienna := timezone.Load("Europe/Vienna")

	// 2026-08-14 23:00 UTC is 2026-08-15 01:00 in Vienna.
	moment := time.Date(2026, 8, 14, 23, 0, 0, 0, time.UTC)

	if got := timezone.DayOf(vienna, moment); got != "2026-08-15" {
		t.Errorf("expected the reading to fall on 2026-08-15 in Vienna, got %s", got)
	}
	if got := timezone.DayOf(time.UTC, moment); got != "2026-08-14" {
		t.Errorf("expected the same moment to be 2026-08-14 in UTC, got %s", got)
	}
}

func TestDayRangeCoversTheLocalDay(t *testing.T) {
	vienna := timezone.Load("Europe/Vienna")

	start, end, err := timezone.DayRange(vienna, "2026-08-15")
	if err != nil {
		t.Fatalf("failed to resolve the day: %v", err)
	}

	// Vienna is UTC+2 in August, so the local day starts at 22:00 the day before.
	if start != "2026-08-14 22:00:00.000Z" {
		t.Errorf("expected the day to start at 2026-08-14 22:00 UTC, got %s", start)
	}
	if end != "2026-08-15 22:00:00.000Z" {
		t.Errorf("expected the day to end at 2026-08-15 22:00 UTC, got %s", end)
	}
}

// A fixed offset would be wrong twice a year. These two days are the reason the
// range is computed rather than added.
func TestDaylightSavingDaysAreNotTwentyFourHours(t *testing.T) {
	vienna := timezone.Load("Europe/Vienna")

	scenarios := []struct {
		date  string
		hours float64
		why   string
	}{
		{"2026-03-29", 23, "the spring forward loses an hour"},
		{"2026-10-25", 25, "the autumn back gains one"},
		{"2026-08-15", 24, "an ordinary day is unchanged"},
	}

	for _, scenario := range scenarios {
		start, end, err := timezone.DayRange(vienna, scenario.date)
		if err != nil {
			t.Fatalf("%s: failed to resolve: %v", scenario.date, err)
		}

		from, err := time.Parse("2006-01-02 15:04:05.000Z", start)
		if err != nil {
			t.Fatalf("%s: unparseable start %q: %v", scenario.date, start, err)
		}
		to, err := time.Parse("2006-01-02 15:04:05.000Z", end)
		if err != nil {
			t.Fatalf("%s: unparseable end %q: %v", scenario.date, end, err)
		}

		if got := to.Sub(from).Hours(); got != scenario.hours {
			t.Errorf("%s: expected %v hours because %s, got %v",
				scenario.date, scenario.hours, scenario.why, got)
		}
	}
}

func TestDayRangeRejectsSomethingThatIsNotADate(t *testing.T) {
	if _, _, err := timezone.DayRange(time.UTC, "the fifteenth"); err == nil {
		t.Error("expected an unparseable date to be refused")
	}
}
