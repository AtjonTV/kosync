//
// File:        internal/analytics/days.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"git.obth.eu/atjontv/kosync/internal/timezone"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

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
	start, end, err := timezone.DayRange(timezone.Of(app, ownerId), date)
	if err != nil {
		return nil, err
	}

	return dbx.Params{"owner": ownerId, "start": start, "end": end}, nil
}
