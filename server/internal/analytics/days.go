//
// File:        internal/analytics/days.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/timezone"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// OwnerLocation returns the zone an account's reading days are reckoned in.
//
// An account that cannot be loaded gets UTC rather than an error. A statistics
// run is the wrong place to fail over a missing user: the numbers would simply
// stop being computed, silently, which is worse than computing them in the zone
// the timestamps are already in.
func OwnerLocation(app core.App, ownerId string) *time.Location {
	if ownerId == "" {
		return time.UTC
	}

	user, err := app.FindRecordById(schema.CollectionUsers, ownerId)
	if err != nil {
		return time.UTC
	}

	return timezone.Load(user.GetString(schema.FieldTimezone))
}

// dayBounds returns the query parameters that select one account's reading day.
//
// Every statistics query asks the same question — which progress moments fall
// on this day — and every one of them asks it as a half-open range of UTC
// instants rather than as a substring of the stored text. Two reasons, and the
// second is the one that bites:
//
//   - The stored text is UTC, so its first ten characters are a UTC day. Once a
//     reader has a zone, that is not their day.
//   - A fixed offset applied in SQL would be wrong twice a year. The last Sunday
//     in March is 23 hours long in Vienna and the last in October is 25, and
//     only a range says so.
//
// It is also the faster form, because last_read_at is indexed and a range reads
// the index while a substring cannot.
func dayBounds(app core.App, ownerId, date string) (dbx.Params, error) {
	start, end, err := timezone.DayRange(OwnerLocation(app, ownerId), date)
	if err != nil {
		return nil, err
	}

	return dbx.Params{"owner": ownerId, "start": start, "end": end}, nil
}

// LocalDays turns stored UTC timestamps into the distinct local dates they fall
// on, in the order they were read.
//
// This is the other half of dayBounds: where that turns a date into a range,
// this turns instants back into dates. It happens in Go because SQLite cannot
// be told about a zone that observes daylight saving.
func LocalDays(location *time.Location, moments []types.DateTime) []string {
	seen := map[string]bool{}
	days := []string{}

	for _, moment := range moments {
		if moment.IsZero() {
			continue
		}

		day := timezone.DayOf(location, moment.Time())
		if seen[day] {
			continue
		}
		seen[day] = true
		days = append(days, day)
	}

	return days
}
