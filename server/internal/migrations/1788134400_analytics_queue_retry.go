//
// File:        internal/migrations/1788134400_analytics_queue_retry.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(upAnalyticsQueueRetry, downAnalyticsQueueRetry)
}

// upAnalyticsQueueRetry gives a failing recomputation somewhere to wait.
//
// The queue is drained oldest first and an item is removed only once it has been
// recomputed, so before this a day that failed every time was also the day that
// was tried every time: it stayed at the head, took a place in every batch, and
// with a batch's worth of such days nothing behind them was ever reached again.
// The statistics would then stop updating for every account on the instance,
// with nothing to show for it but the same error logged every few seconds.
//
// Two columns are enough to fix that. A failure is counted and the item is put
// down for a while, growing with each attempt, so a genuinely stuck day costs
// one attempt an hour instead of one per tick and everything behind it moves on.
// Nothing is ever dropped: the failure may be a locked database or a bad
// deployment, and the day itself is still owed.
func upAnalyticsQueueRetry(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionAnalyticsQueue)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionAnalyticsQueue, err)
	}

	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldAttempts,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.DateField{
		Name: schema.FieldRetryAfter,
	})

	// The drain claims by "due now, oldest first", so both columns belong in the
	// index it walks. Without the retry column in front, every deferred item is
	// still a row the query has to read and discard.
	collection.AddIndex("idx_analytics_queue_retry_after", false, "retry_after,created", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add the retry columns to %q: %w", schema.CollectionAnalyticsQueue, err)
	}

	return nil
}

func downAnalyticsQueueRetry(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionAnalyticsQueue)
	if err != nil {
		return nil
	}

	collection.RemoveIndex("idx_analytics_queue_retry_after")
	collection.Fields.RemoveByName(schema.FieldAttempts)
	collection.Fields.RemoveByName(schema.FieldRetryAfter)

	return app.Save(collection)
}
