//
// File:        internal/migrations/1788393600_book_description.go
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
	m.Register(upBookDescription, downBookDescription)
}

func upBookDescription(app core.App) error {
	if err := addBookDescription(app); err != nil {
		return err
	}

	return BackfillDescriptions(app)
}

func downBookDescription(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return nil
	}

	collection.Fields.RemoveByName(schema.FieldDescription)

	return app.Save(collection)
}

// maxDescriptionLength is what the column will hold.
//
// The reader cuts what it extracts at four thousand characters, so this is that
// with room over it: the field is the owner's to correct, like the title and the
// series, and a hand-written description should not run into a limit set by how
// much of a publisher's blurb was worth keeping.
const maxDescriptionLength = 5000

// addBookDescription makes room for the one piece of a book's own metadata the
// library was throwing away.
//
// The question a shelf is browsed with is "what is this one about", and until now
// nothing in KOsync could answer it: the cover, the title and the series say who
// wrote it and where it sits, not what happens in it. The file often knows —
// dc:description is where a publisher puts the blurb — and the reader was walking
// straight past it.
func addBookDescription(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionBooks, err)
	}

	collection.Fields.Add(&core.TextField{
		Name: schema.FieldDescription,
		Max:  maxDescriptionLength,
	})

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add %q to %q: %w", schema.FieldDescription, schema.CollectionBooks, err)
	}

	return nil
}

// BackfillDescriptions reads the blurb out of the books that were uploaded
// before the column existed.
//
// Every book has to be opened again, for the same reason the series did: the
// description was parsed past and never stored, so there is nowhere else to get
// it from. This is the expensive kind of migration — it reads every file in the
// library once — and it is paid once, on the upgrade that adds the column.
//
// It is that or a library where only the books uploaded from today on say
// anything about themselves, which for a shelf that has been filling up for
// months is no feature at all. The alternative the operator was left with before
// is worse still: download the book, delete it, upload it again, and lose the
// reading progress linked to it.
//
// Nothing here is fatal. A book whose file has gone missing, or which is no
// longer readable as an EPUB, keeps its empty description and is logged; a
// library with one broken record in it must still start.
//
// Written with a plain statement rather than app.Save, like the backfills before
// it: the description describes the file, and a library should not report every
// book as edited today because a column was added.
func BackfillDescriptions(app core.App) error {
	records, err := app.FindAllRecords(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("load %q: %w", schema.CollectionBooks, err)
	}
	if len(records) == 0 {
		return nil
	}

	system, err := app.NewFilesystem()
	if err != nil {
		app.Logger().Warn("could not read the stored books, their descriptions stay unknown", "error", err)

		return nil
	}
	defer system.Close()

	described := 0
	for _, record := range records {
		filename := record.GetString(schema.FieldFile)
		// A description already there is one somebody typed, and a file the
		// publisher wrote is not a reason to overwrite it.
		if filename == "" || record.GetString(schema.FieldDescription) != "" {
			continue
		}

		metadata, err := readStoredBook(system, record.BaseFilesPath()+"/"+filename)
		if err != nil {
			app.Logger().Warn("could not re-read a stored book",
				"book", record.Id, "file", filename, "error", err)

			continue
		}
		if metadata.Description == "" {
			continue
		}

		_, err = app.DB().
			NewQuery("UPDATE {{" + schema.CollectionBooks + "}}" +
				" SET [[" + schema.FieldDescription + "]] = {:description}" +
				" WHERE [[id]] = {:id}").
			Bind(dbx.Params{"description": metadata.Description, "id": record.Id}).
			Execute()
		if err != nil {
			return fmt.Errorf("describe %q: %w", record.Id, err)
		}
		described++
	}

	if described > 0 {
		app.Logger().Info("read the descriptions of the stored books", "books", described)
	}

	return nil
}
