//
// File:        internal/migrations/1788220800_book_measured_source.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(upBookMeasuredSource, downBookMeasuredSource)
}

func upBookMeasuredSource(app core.App) error {
	if err := addMeasuredSource(app); err != nil {
		return err
	}

	return markExistingMeasurements(app)
}

func downBookMeasuredSource(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return nil
	}

	collection.Fields.RemoveByName(schema.FieldMeasuredSource)

	return app.Save(collection)
}

// addMeasuredSource records which of the two page measurements a book holds.
//
// There are two now: the count a device's statistics database states outright,
// and the count the estimator reconstructs from progress pushes. Both are stored
// in the same column because both are the device's own pagination and everything
// downstream reckons in it either way — but they are not equally good, and
// without knowing which one is stored the estimator would overwrite a stated
// number with a reconstructed one on the next day it recomputes.
func addMeasuredSource(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionBooks, err)
	}

	collection.Fields.Add(&core.TextField{
		Name: schema.FieldMeasuredSource,
		Max:  20,
	})

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add %q to %q: %w", schema.FieldMeasuredSource, schema.CollectionBooks, err)
	}

	return nil
}

// markExistingMeasurements says where the measurements already stored came from.
//
// Only the estimator could have written them, so they are all progress ones. A
// plain statement rather than a save per record for the reason the file sizes
// were backfilled that way: this describes what a column already held, and a
// library should not report every book as edited today because a column was
// added.
func markExistingMeasurements(app core.App) error {
	_, err := app.DB().
		NewQuery(`
			UPDATE {{` + schema.CollectionBooks + `}}
			SET [[` + schema.FieldMeasuredSource + `]] = {:source}
			WHERE [[` + schema.FieldMeasuredPages + `]] > 0
		`).
		Bind(dbx.Params{"source": schema.MeasuredByProgress}).
		Execute()
	if err != nil {
		return fmt.Errorf("mark the existing measurements of %q: %w", schema.CollectionBooks, err)
	}

	return nil
}
