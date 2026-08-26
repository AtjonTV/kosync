//
// File:        internal/migrations/1787011200_queue_book_statistics.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"fmt"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(upQueueBookStatistics, downQueueBookStatistics)
}

// upQueueBookStatistics asks for every stored day to be computed again.
//
// The per-book rows and the page counts are produced by the recomputation, and
// nothing recomputes a day that is not read on again. Without this, an instance
// upgrading into book statistics would show zero pages for everything it had
// already recorded — the same shape of gap that left uploaded books unmatched
// before, and the same fix: do the work once, on the way in.
//
// The queue is drained in the background a batch at a time, so this stays a
// cheap insert rather than a migration that recomputes months of reading while
// the server is still starting.
func upQueueBookStatistics(app core.App) error {
	return QueueStoredDays(app)
}

// QueueStoredDays enqueues a recomputation for every day already on record.
func QueueStoredDays(app core.App) error {
	query := fmt.Sprintf(`
		INSERT INTO {{%s}} ([[id]], [[owner]], [[date]], [[created]])
		SELECT
			substr(lower(hex(randomblob(16))), 1, 15),
			d.[[owner]],
			d.[[date]],
			{:created}
		FROM {{%s}} d
		WHERE true
		ON CONFLICT ([[owner]], [[date]]) DO NOTHING
	`, schema.CollectionAnalyticsQueue, schema.CollectionReadingDays)

	// The random id is generated in SQL so this stays one statement. PocketBase
	// only requires 15 characters that match its id pattern, which hex does.
	//
	// "WHERE true" is not decoration: without it SQLite parses the ON CONFLICT as
	// part of the SELECT's own table reference.
	if _, err := app.DB().NewQuery(query).Bind(dbx.Params{
		"created": time.Now().UTC().Format("2006-01-02 15:04:05.000Z"),
	}).Execute(); err != nil {
		return fmt.Errorf("queue the stored days: %w", err)
	}

	return nil
}

// downQueueBookStatistics does nothing: the queue drains itself, and by the time
// anything rolls back there is nothing left to undo.
func downQueueBookStatistics(_ core.App) error {
	return nil
}
