//
// File:        internal/migrations/1786924800_book_statistics.go
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
	m.Register(upBookStatistics, downBookStatistics)
}

func upBookStatistics(app core.App) error {
	if err := addMeasuredPages(app); err != nil {
		return err
	}

	return createReadingBookDays(app)
}

func downBookStatistics(app core.App) error {
	if collection, err := app.FindCollectionByNameOrId(schema.CollectionReadingBookDays); err == nil {
		if err := app.Delete(collection); err != nil {
			return err
		}
	}

	books, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return nil
	}

	for _, field := range []string{schema.FieldMeasuredPages, schema.FieldMeasuredDevice, schema.FieldMeasuredThrough} {
		books.Fields.RemoveByName(field)
	}

	return app.Save(books)
}

// addMeasuredPages gives a book somewhere to keep the page count measured from
// the progress its devices pushed.
//
// It sits beside page_count rather than replacing it: page_count is what the
// word count implies and is always available, while a measurement needs a device
// that pushed often enough, which two of the five reference books never did.
func addMeasuredPages(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionBooks, err)
	}

	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldMeasuredPages,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	// Which device the measurement came from. A book read on a phone and on an
	// e-reader has two page counts, both correct; this records whose it is.
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldMeasuredDevice,
		Max:  200,
	})
	// How far into the reading the measurement looked, so that a book nobody has
	// read since is not measured again on every recomputation of every day it
	// was read on.
	collection.Fields.Add(&core.DateField{
		Name: schema.FieldMeasuredThrough,
	})

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add the measured page count to %q: %w", schema.CollectionBooks, err)
	}

	return nil
}

// createReadingBookDays creates the per-book daily statistics.
//
// The measures are the ones reading_days carries, keyed by book as well as by
// day. They are computed independently rather than by regrouping the day rows,
// because reading time is estimated from the gaps between pushes and a gap that
// spans a switch from one book to another belongs to neither: the book rows add
// up to at most the day row, and the difference is the switching.
func createReadingBookDays(app core.App) error {
	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	books, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionBooks, err)
	}

	collection := core.NewBaseCollection(schema.CollectionReadingBookDays)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  users.Id,
		CascadeDelete: true,
	})
	// Deleting a book takes its per-book rows with it, unlike the documents,
	// which keep their reading. These rows describe the book; without it there
	// is nothing for them to describe, and the day totals still hold the reading.
	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldBook,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  books.Id,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldDate,
		Required: true,
		Pattern:  `^\d{4}-\d{2}-\d{2}$`,
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldUpdateCount,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	// Percentage points of this book, so finishing it in a day is 100.
	collection.Fields.Add(&core.NumberField{
		Name: schema.FieldProgressIncrease,
		Min:  types.Pointer(0.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldReadingTime, // seconds
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldPagesRead,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.DateField{
		Name: schema.FieldComputedAt,
	})
	addTimestamps(collection)

	collection.AddIndex("idx_reading_book_days_owner_date_book", true, "owner,date,book", "")
	// The book detail page reads one book's whole series at once.
	collection.AddIndex("idx_reading_book_days_book_date", false, "book,date", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionReadingBookDays, err)
	}

	return nil
}
