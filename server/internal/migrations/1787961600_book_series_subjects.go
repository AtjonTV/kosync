//
// File:        internal/migrations/1787961600_book_series_subjects.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(upBookSeriesSubjects, downBookSeriesSubjects)
}

func upBookSeriesSubjects(app core.App) error {
	if err := addBookSeriesSubjects(app); err != nil {
		return err
	}

	return BackfillSeriesAndSubjects(app)
}

func downBookSeriesSubjects(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return nil
	}

	for _, name := range []string{schema.FieldSeries, schema.FieldSeriesIndex, schema.FieldSubjects} {
		collection.Fields.RemoveByName(name)
	}
	collection.RemoveIndex("idx_books_owner_series")

	return app.Save(collection)
}

// addBookSeriesSubjects makes room for the two things a big library is browsed
// by that were being thrown away.
//
// A library of two hundred books is not skimmable as one list, and the series is
// the strongest thing to break it up by: thirty of the reference library's books
// belong to one, and their reader wants volume two after volume one rather than
// whatever sorts next alphabetically. The subjects come along because they are
// in the same file and cost one column — but see the field comment for what they
// are and are not good for.
func addBookSeriesSubjects(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionBooks, err)
	}

	collection.Fields.Add(&core.TextField{
		Name: schema.FieldSeries,
		Max:  500,
	})
	// Not an integer. Half-numbered volumes are a real thing publishers do —
	// the novella between books two and three is 2.5 — and rounding them to
	// the volume they sit beside puts them in an arbitrary order next to it.
	collection.Fields.Add(&core.NumberField{
		Name: schema.FieldSeriesIndex,
		Min:  types.Pointer(0.0),
	})
	collection.Fields.Add(&core.JSONField{
		Name:    schema.FieldSubjects,
		MaxSize: 8000,
	})

	// Grouping a library by series and listing one series both start here.
	collection.AddIndex("idx_books_owner_series", false, "owner,series", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add the series fields to %q: %w", schema.CollectionBooks, err)
	}

	return nil
}

// maxBackfillBytes caps how much of one book is read into memory to re-examine
// it. The field itself allows more, but a book that large is not going to be an
// EPUB whose package document is worth reading, and a migration should not be
// the thing that decides how much memory the server needs.
const maxBackfillBytes = 256 << 20

// BackfillSeriesAndSubjects reads the books that were uploaded before the
// columns existed.
//
// Every book has to be opened again, because the series is nowhere else: unlike
// the file size, which the filesystem knew all along, this was parsed once and
// discarded. That makes it the most expensive migration here — the reference
// library is 192 books and about two gigabytes — but it is paid once, and the
// alternative is a shelf that only works for books uploaded from today on.
//
// Nothing here is fatal. A book whose file has gone missing, or which is no
// longer readable as an EPUB, keeps its empty columns and is logged; a library
// with one broken record in it must still start.
//
// Written with plain statements rather than app.Save for the same reason the
// file sizes were: these fields describe the file, and a library should not
// report every book as edited today because a column was added.
func BackfillSeriesAndSubjects(app core.App) error {
	records, err := app.FindAllRecords(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("load %q: %w", schema.CollectionBooks, err)
	}
	if len(records) == 0 {
		return nil
	}

	system, err := app.NewFilesystem()
	if err != nil {
		app.Logger().Warn("could not read the stored books, their series stay unknown", "error", err)

		return nil
	}
	defer system.Close()

	described := 0
	for _, record := range records {
		filename := record.GetString(schema.FieldFile)
		if filename == "" {
			continue
		}

		metadata, err := readStoredBook(system, record.BaseFilesPath()+"/"+filename)
		if err != nil {
			app.Logger().Warn("could not re-read a stored book",
				"book", record.Id, "file", filename, "error", err)

			continue
		}
		if metadata.Series == "" && len(metadata.Subjects) == 0 {
			continue
		}

		// A book with no subjects is left NULL rather than given the string
		// "null" or an empty array, so that "has none" has one spelling in the
		// column and every query can ask about it the same way.
		var subjects any
		if len(metadata.Subjects) > 0 {
			encoded, err := json.Marshal(metadata.Subjects)
			if err != nil {
				continue
			}
			subjects = string(encoded)
		}

		_, err = app.DB().
			NewQuery("UPDATE {{" + schema.CollectionBooks + "}}" +
				" SET [[" + schema.FieldSeries + "]] = {:series}," +
				" [[" + schema.FieldSeriesIndex + "]] = {:index}," +
				" [[" + schema.FieldSubjects + "]] = {:subjects}" +
				" WHERE [[id]] = {:id}").
			Bind(dbx.Params{
				"series":   metadata.Series,
				"index":    metadata.SeriesIndex,
				"subjects": subjects,
				"id":       record.Id,
			}).
			Execute()
		if err != nil {
			return fmt.Errorf("describe %q: %w", record.Id, err)
		}
		described++
	}

	if described > 0 {
		app.Logger().Info("read the series and subjects of the stored books", "books", described)
	}

	return nil
}

// readStoredBook opens one stored EPUB and returns what it says about itself.
//
// The whole file goes through memory, one book at a time. The storage backend
// may be an object store on the other side of a network, where the seeking that
// archive/zip does would be a request per read; buying that back with one
// sequential read of a file already capped at MaxBookBytes is the better trade.
func readStoredBook(system *filesystem.System, key string) (epub.Metadata, error) {
	reader, err := system.GetReader(key)
	if err != nil {
		return epub.Metadata{}, err
	}
	defer reader.Close()

	raw, err := io.ReadAll(io.LimitReader(reader, maxBackfillBytes))
	if err != nil {
		return epub.Metadata{}, err
	}

	book, err := epub.Open(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return epub.Metadata{}, err
	}

	return book.Metadata(), nil
}
