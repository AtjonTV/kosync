//
// File:        internal/summary/summary.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package summary works out what an account read over a week or a month.
//
// It is separate from the mail that carries it for the ordinary reason: the
// question "what did this account read last week" has an answer whether or not
// anybody is sending it anywhere, and it is worth being able to test that answer
// without a mailbox.
//
// Everything here is reckoned in the account's own timezone. A week that ended
// on Sunday ended when the reader's Sunday ended, not when UTC's did, and the
// daily statistics this reads are already stored under local dates for exactly
// that reason.
package summary

import (
	"fmt"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/timezone"
	"github.com/pocketbase/pocketbase/core"
)

// SendHour is the local hour a summary may first go out at.
//
// Eight in the morning, because the mail is about the reading somebody has just
// finished doing and there is no hurry: an account whose week ended at midnight
// is not owed an email at midnight.
const SendHour = 8

// Period is the stretch of days a summary covers.
//
// From and To are inclusive local dates, which is what the daily statistics are
// keyed by. Key is what gets written down once the summary has been sent, and is
// the reason a server that was switched off all weekend still sends the right
// thing on Monday. Title is how the period is named to a reader: "week 32
// (3. - 9. August)" or "July 2026".
type Period struct {
	Kind  string
	Key   string
	From  string
	To    string
	Title string
}

// weekKey and monthKey are how a period is written down. The week is an ISO
// week, which is the only numbering where "week 1" means the same thing to
// everybody.
const (
	weekKey   = "%d-W%02d"
	monthKey  = "2006-01"
	dayLayout = timezone.DateLayout
)

// LastCompleted returns the most recent period of the given kind that has
// finished, as of the given local time.
//
// Finished is the point: a summary of a week still being read would be wrong by
// the evening. The second return is false for a cadence that is not a cadence,
// which is what an account that has never asked for any of this holds.
func LastCompleted(kind string, local time.Time) (Period, bool) {
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())

	switch kind {
	case schema.SummaryWeekly:
		// Monday is the first day, so Sunday's index has to be six rather than
		// zero. Go counts from Sunday and readers do not.
		offset := (int(midnight.Weekday()) + 6) % 7
		end := midnight.AddDate(0, 0, -offset-1)
		start := end.AddDate(0, 0, -6)
		year, week := start.ISOWeek()

		return Period{
			Kind:  kind,
			Key:   fmt.Sprintf(weekKey, year, week),
			From:  start.Format(dayLayout),
			To:    end.Format(dayLayout),
			Title: weekTitle(week, start, end),
		}, true

	case schema.SummaryMonthly:
		end := time.Date(midnight.Year(), midnight.Month(), 1, 0, 0, 0, 0, midnight.Location()).
			AddDate(0, 0, -1)
		start := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, end.Location())

		return Period{
			Kind:  kind,
			Key:   start.Format(monthKey),
			From:  start.Format(dayLayout),
			To:    end.Format(dayLayout),
			Title: start.Format("January 2006"),
		}, true
	}

	return Period{}, false
}

// weekTitle names a week both ways somebody might think of one: by its ISO
// number, and by the days it covers.
//
// Either half alone is a poor name. "Week 33" assumes the reader counts weeks,
// which most do not; "the week of 10 August" and "the week of 3 August" are the
// same sentence twice, and two of them arriving close together read as the same
// mail sent twice. Together they say which week this is and prove it.
//
// The month is written once when both ends share one, and the year only when
// they do not — a week that runs from December into January is the one place
// where leaving the year out would be genuinely ambiguous.
func weekTitle(week int, start, end time.Time) string {
	switch {
	case start.Year() != end.Year():
		return fmt.Sprintf("week %d (%d. %s %d - %d. %s %d)", week,
			start.Day(), start.Month(), start.Year(), end.Day(), end.Month(), end.Year())

	case start.Month() != end.Month():
		return fmt.Sprintf("week %d (%d. %s - %d. %s)", week,
			start.Day(), start.Month(), end.Day(), end.Month())

	default:
		return fmt.Sprintf("week %d (%d. - %d. %s)", week, start.Day(), end.Day(), end.Month())
	}
}

// Due returns the period an account is owed a summary for, if it is owed one.
//
// Four things have to be true, and the order they are checked in is the order
// they are cheap in: the account asked for a cadence, it is a civilised hour
// where the account is, that cadence has a completed period, and no summary has
// gone out for it yet.
//
// The last of those is what makes this safe to run every hour, and safe to have
// missed for three days.
func Due(owner *core.Record, now time.Time) (Period, bool) {
	kind := owner.GetString(schema.FieldSummaryMail)
	if kind == "" || kind == schema.SummaryOff {
		return Period{}, false
	}

	local := now.In(timezone.Load(owner.GetString(schema.FieldTimezone)))
	if local.Hour() < SendHour {
		return Period{}, false
	}

	period, ok := LastCompleted(kind, local)
	if !ok {
		return Period{}, false
	}

	if owner.GetString(schema.FieldSummarySent) == period.Key {
		return Period{}, false
	}

	return period, true
}

// Wanted lists the accounts that have asked for a summary of some cadence.
func Wanted(app core.App) ([]*core.Record, error) {
	records, err := app.FindRecordsByFilter(
		schema.CollectionUsers,
		fmt.Sprintf("%s != '' && %s != '%s'", schema.FieldSummaryMail, schema.FieldSummaryMail, schema.SummaryOff),
		schema.FieldCreated, 0, 0,
	)
	if err != nil {
		return nil, fmt.Errorf("list the accounts wanting a summary: %w", err)
	}

	return records, nil
}
