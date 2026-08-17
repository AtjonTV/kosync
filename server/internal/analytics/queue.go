//
// File:        internal/analytics/queue.go
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
)

// DateLayout is how a statistics day is written. The day it names belongs to the
// owner's timezone, not to UTC — see internal/timezone.
const DateLayout = timezone.DateLayout

// dateTimeLayout is how PocketBase stores a date column.
const dateTimeLayout = "2006-01-02 15:04:05.000Z"

// Enqueue marks one day of one user as in need of a recomputation.
//
// The insert is a no-op when the day is already queued, which is what turns a
// burst of progress pushes into a single recomputation.
func Enqueue(app core.App, ownerId string, date string) error {
	if ownerId == "" || date == "" {
		return nil
	}

	_, err := app.DB().
		NewQuery(`
			INSERT INTO {{` + schema.CollectionAnalyticsQueue + `}} ([[id]], [[owner]], [[date]], [[created]])
			VALUES ({:id}, {:owner}, {:date}, {:created})
			ON CONFLICT ([[owner]], [[date]]) DO NOTHING
		`).
		Bind(dbx.Params{
			"id":      core.GenerateDefaultRandomId(),
			"owner":   ownerId,
			"date":    date,
			"created": time.Now().UTC().Format(dateTimeLayout),
		}).
		Execute()

	return err
}

// EnqueueTime marks the day the given moment falls on, in the owner's zone.
//
// The zone is what makes this a lookup rather than a format call: an evening in
// Vienna is already tomorrow in UTC, and queueing the UTC day would recompute a
// day the reader has not lived yet while leaving the one they just read on
// stale.
func EnqueueTime(app core.App, ownerId string, moment time.Time) error {
	return Enqueue(app, ownerId, timezone.DayOf(timezone.Of(app, ownerId), moment))
}

// Retry pacing for a day that could not be recomputed.
//
// The first failure is worth trying again almost at once — a locked database
// clears in seconds — and each one after that is worth waiting longer for. The
// cap is what keeps a permanently broken day down to a line in the log every
// hour instead of one every tick.
const (
	retryBackoffBase = time.Minute
	retryBackoffMax  = time.Hour
)

// queueItem is one pending recomputation.
type queueItem struct {
	Id       string `db:"id"`
	Owner    string `db:"owner"`
	Date     string `db:"date"`
	Attempts int    `db:"attempts"`
}

// claim returns up to limit items that are due, oldest first.
//
// An item that has failed carries the moment it may be tried again, and until
// then it is not a candidate. That is the whole reason this is a filter and not
// a plain read: the queue is drained oldest first, so without it the day that
// always fails is also the day that is always tried, and a batch's worth of them
// hides everything behind them forever.
func claim(app core.App, limit int) ([]queueItem, error) {
	items := []queueItem{}

	err := app.DB().
		Select("id", "owner", "date", "attempts").
		From(schema.CollectionAnalyticsQueue).
		// PocketBase writes an unset date as the empty string rather than NULL,
		// which is also what every row created before the column existed holds.
		AndWhere(dbx.NewExp(
			"[[retry_after]] = '' OR [[retry_after]] <= {:now}",
			dbx.Params{"now": time.Now().UTC().Format(dateTimeLayout)},
		)).
		OrderBy("created ASC").
		Limit(int64(limit)).
		All(&items)

	return items, err
}

// postpone records a failed attempt and puts the item down until it is worth
// another one.
//
// The item is deliberately not deleted however often it fails. What stopped the
// recomputation is usually not the day itself, and the reading it describes is
// still on the server waiting to be counted.
func postpone(app core.App, item queueItem) error {
	attempts := item.Attempts + 1

	// One, two, four ... thirty-two minutes, and an hour from then on.
	backoff := retryBackoffMax
	if attempts >= 1 && attempts <= 6 {
		backoff = retryBackoffBase << (attempts - 1)
	}

	_, err := app.DB().
		NewQuery("UPDATE {{" + schema.CollectionAnalyticsQueue + "}}" +
			" SET [[attempts]] = {:attempts}, [[retry_after]] = {:retryAfter}" +
			" WHERE [[id]] = {:id}").
		Bind(dbx.Params{
			"attempts":   attempts,
			"retryAfter": time.Now().UTC().Add(backoff).Format(dateTimeLayout),
			"id":         item.Id,
		}).
		Execute()

	return err
}

// release removes processed items from the queue.
func release(app core.App, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	values := make([]any, len(ids))
	for i, id := range ids {
		values[i] = id
	}

	_, err := app.DB().
		Delete(schema.CollectionAnalyticsQueue, dbx.In("id", values...)).
		Execute()

	return err
}

// registerEnqueueHooks queues a recomputation whenever reading data changes,
// no matter whether it arrived from a device, the WebUI or the importer.
func registerEnqueueHooks(app core.App) {
	enqueueFromRecord := func(e *core.RecordEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		owner := e.Record.GetString(schema.FieldOwner)
		day := e.Record.GetDateTime(schema.FieldLastReadAt)
		if owner == "" || day.IsZero() {
			return nil
		}

		if err := EnqueueTime(e.App, owner, day.Time()); err != nil {
			// A missed enqueue costs a stale statistics row until the weekly
			// reconcile job picks the day up again, so it must not fail the
			// write that triggered it.
			e.App.Logger().Warn("failed to queue a statistics recomputation",
				"owner", owner, "date", day.String(), "error", err)
		}

		return nil
	}

	for _, collection := range []string{schema.CollectionDocuments, schema.CollectionDocumentHistory} {
		app.OnRecordAfterCreateSuccess(collection).BindFunc(enqueueFromRecord)
		app.OnRecordAfterUpdateSuccess(collection).BindFunc(enqueueFromRecord)
		app.OnRecordAfterDeleteSuccess(collection).BindFunc(enqueueFromRecord)
	}
}
