//
// File:        internal/analytics/measurements.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"sync"

	"git.obth.eu/atjontv/kosync/internal/statistics"
	"github.com/pocketbase/pocketbase/core"
)

// Uploads is something that can say when a device has left its statistics
// behind. It is satisfied by the WebDAV handler.
//
// An interface rather than the handler itself so that this package does not
// depend on the endpoint: what to do with a measured day is a statistics
// question, and it would be the same question if the file arrived some other
// way.
type Uploads interface {
	OnStored(handle func(ownerId, path string))
}

// imports counts the imports still running, so that a shutdown can wait for
// them rather than pulling the database out from under one.
var imports sync.WaitGroup

// RegisterMeasurements imports a device's own statistics whenever one arrives.
//
// Off the request that carried it, for the same reason the achievement mail is:
// a device waiting for its upload to be acknowledged should not also be waiting
// for several thousand rows to be examined. Nothing is lost if the server stops
// half way — the file is already stored, and the next sync imports it again from
// the beginning, which is a no-op for everything that was already here.
func RegisterMeasurements(app core.App, uploads Uploads) {
	if uploads == nil {
		return
	}

	uploads.OnStored(func(ownerId, path string) {
		imports.Add(1)

		go func() {
			defer imports.Done()

			result, err := ImportMeasurements(app, ownerId, path)
			if err != nil {
				app.Logger().Error("failed to import a statistics database",
					"owner", ownerId, "error", err)

				return
			}

			app.Logger().Info("imported a statistics database",
				"owner", ownerId, "rows", result.Rows, "added", result.Added, "days", len(result.Dates))
		}()
	})

	app.OnTerminate().BindFunc(func(te *core.TerminateEvent) error {
		imports.Wait()
		return te.Next()
	})
}

// ImportMeasurements reads an uploaded statistics database, records the page
// counts it states and queues every day it has something new to say about.
//
// Queued rather than recomputed here: the days it names may be months old and
// there may be hundreds of them, and the queue is what already knows how to work
// through that a batch at a time without holding anything up.
//
// The page counts are stored before the days are queued, because a day's reading
// is counted in pages and the count they should be reckoned in has just arrived.
// A failure to store them is logged rather than returned: the reading is already
// imported, and losing a day's worth of statistics over a page count would be the
// wrong way round.
func ImportMeasurements(app core.App, ownerId, path string) (statistics.Result, error) {
	result, err := statistics.Import(app, ownerId, path)
	if err != nil {
		return result, err
	}

	if changed, err := StorePagination(app, ownerId, result.Pages); err != nil {
		app.Logger().Warn("could not store the page counts a device stated",
			"owner", ownerId, "error", err)
	} else if changed > 0 {
		app.Logger().Info("took the page count of some books from the device",
			"owner", ownerId, "books", changed)
	}

	for _, date := range result.Dates {
		enqueueQuietly(app, ownerId, date)
	}

	return result, nil
}
