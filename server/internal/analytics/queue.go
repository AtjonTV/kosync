//
// File:        internal/analytics/queue.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// DateLayout is how a statistics day is written, in UTC.
const DateLayout = "2006-01-02"

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

// EnqueueTime marks the day the given moment falls on, in UTC.
func EnqueueTime(app core.App, ownerId string, moment time.Time) error {
	return Enqueue(app, ownerId, moment.UTC().Format(DateLayout))
}

// queueItem is one pending recomputation.
type queueItem struct {
	Id    string `db:"id"`
	Owner string `db:"owner"`
	Date  string `db:"date"`
}

// claim returns up to limit pending items, oldest first.
func claim(app core.App, limit int) ([]queueItem, error) {
	items := []queueItem{}

	err := app.DB().
		Select("id", "owner", "date").
		From(schema.CollectionAnalyticsQueue).
		OrderBy("created ASC").
		Limit(int64(limit)).
		All(&items)

	return items, err
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
