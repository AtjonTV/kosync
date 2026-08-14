//
// File:        internal/timezone/timezone.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package timezone turns an account's timezone into the day boundaries the
// statistics are reckoned by.
//
// A reading day has to be the reader's day. KOsync stores every timestamp in
// UTC — it has to, because the devices never say what time they think it is —
// so somewhere the two have to be reconciled, and this is that somewhere.
//
// The conversion is deliberately expressed as a half-open range of UTC instants
// rather than as an offset applied inside SQL. An offset is wrong twice a year,
// and a range is not: 2026-03-29 in Vienna is 23 hours long, and asking for
// "the instants between these two" is the only phrasing that stays true through
// a daylight saving change. It also happens to be the faster question, because
// last_read_at is indexed and a range reads the index while a substring does
// not.
package timezone

import (
	"sync"
	"time"

	// The zone database compiled into the binary. Without it a container built
	// from a minimal base has no /usr/share/zoneinfo, and every account would
	// silently fall back to UTC — which is exactly the bug this package exists
	// to fix, arriving by a different route.
	_ "time/tzdata"
)

// Default is the zone an account has until it says otherwise. UTC is the honest
// choice for "not answered yet": it is what the timestamps already are, so
// nothing is silently shifted.
const Default = "UTC"

// DateLayout is how a statistics day is written.
const DateLayout = "2006-01-02"

// dateTimeLayout is how PocketBase stores a date column, and therefore how a
// boundary has to be written to be comparable against one.
const dateTimeLayout = "2006-01-02 15:04:05.000Z"

// loaded caches the locations that have been asked for.
//
// time.LoadLocation parses the zone file on every call, and the statistics
// worker asks for the same handful of zones over and over.
var loaded sync.Map

// Load returns the named location, or UTC when the name is empty or unknown.
//
// An unknown name is not an error here on purpose. It reaches this package from
// a stored field, and a statistics run is the wrong place to discover that a
// zone was mistyped months ago; the validation belongs where the name is set.
func Load(name string) *time.Location {
	if name == "" {
		return time.UTC
	}

	if cached, ok := loaded.Load(name); ok {
		return cached.(*time.Location)
	}

	location, err := time.LoadLocation(name)
	if err != nil {
		location = time.UTC
	}

	loaded.Store(name, location)

	return location
}

// Valid reports whether a name is one the zone database knows.
func Valid(name string) bool {
	if name == "" {
		return false
	}

	_, err := time.LoadLocation(name)

	return err == nil
}

// DayOf returns the local date a UTC instant falls on.
func DayOf(location *time.Location, moment time.Time) string {
	return moment.In(location).Format(DateLayout)
}

// DayRange returns the UTC instants a local day spans, as the half-open
// interval [start, end) in the format a stored timestamp is written in.
//
// A day that does not exist in the given zone — the hour a spring-forward skips
// can make midnight itself skip — is resolved by time.Date to the moment just
// after the gap, which is the right answer: the day still starts, it just starts
// an hour later.
func DayRange(location *time.Location, date string) (start, end string, err error) {
	day, err := time.ParseInLocation(DateLayout, date, location)
	if err != nil {
		return "", "", err
	}

	// AddDate rather than Add(24h): a day is not always 24 hours long, and this
	// is the whole reason the package exists.
	next := day.AddDate(0, 0, 1)

	return day.UTC().Format(dateTimeLayout), next.UTC().Format(dateTimeLayout), nil
}
