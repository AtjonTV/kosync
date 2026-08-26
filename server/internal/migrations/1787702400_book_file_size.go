//
// File:        internal/migrations/1787702400_book_file_size.go
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
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(upBookFileSize, downBookFileSize)
}

func upBookFileSize(app core.App) error {
	if err := addBookFileSize(app); err != nil {
		return err
	}

	return BackfillFileSizes(app)
}

func downBookFileSize(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return nil
	}

	collection.Fields.RemoveByName(schema.FieldFileSize)

	return app.Save(collection)
}

// addBookFileSize records how much room a book takes.
//
// The size is on the record rather than read from the filesystem when it is
// wanted, because a quota is checked on every upload and the answer it needs is
// a sum over the whole library. One column and one SUM is a question the
// database can answer; a stat call per book is a question that gets slower with
// every book somebody adds.
func addBookFileSize(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionBooks, err)
	}

	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldFileSize,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add %q to %q: %w", schema.FieldFileSize, schema.CollectionBooks, err)
	}

	return nil
}

// BackfillFileSizes measures the books that were uploaded before the column
// existed.
//
// This is the one place the filesystem is asked, and it is asked once per book
// ever. A file that has gone missing is left at zero rather than failing the
// migration: a library with a broken record in it must still start, and a quota
// that undercounts by one book is a smaller problem than a server that will not
// boot.
//
// Written with a plain statement for the same reason the catalog hashes were:
// the size describes the file, and a library should not report every record as
// edited today because a column was added.
func BackfillFileSizes(app core.App) error {
	records, err := app.FindAllRecords(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("load %q: %w", schema.CollectionBooks, err)
	}
	if len(records) == 0 {
		return nil
	}

	system, err := app.NewFilesystem()
	if err != nil {
		// Object storage that cannot be reached is an operator's problem, not a
		// reason to refuse the schema change. The sizes stay at zero and the
		// next upload of each book fills its own in.
		app.Logger().Warn("could not measure the stored books, their sizes stay unknown", "error", err)

		return nil
	}
	defer system.Close()

	for _, record := range records {
		filename := record.GetString(schema.FieldFile)
		if filename == "" {
			continue
		}

		attributes, err := system.Attributes(record.BaseFilesPath() + "/" + filename)
		if err != nil {
			app.Logger().Warn("could not measure a stored book",
				"book", record.Id, "file", filename, "error", err)

			continue
		}

		_, err = app.DB().
			NewQuery("UPDATE {{" + schema.CollectionBooks + "}} SET [[" + schema.FieldFileSize + "]] = {:size} WHERE [[id]] = {:id}").
			Bind(dbx.Params{"size": attributes.Size, "id": record.Id}).
			Execute()
		if err != nil {
			return fmt.Errorf("set the size of %q: %w", record.Id, err)
		}
	}

	return nil
}
