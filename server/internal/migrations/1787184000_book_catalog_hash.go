//
// File:        internal/migrations/1787184000_book_catalog_hash.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(upBookCatalogHash, downBookCatalogHash)
}

func upBookCatalogHash(app core.App) error {
	if err := addCatalogHash(app); err != nil {
		return err
	}

	return BackfillCatalogHashes(app)
}

func downBookCatalogHash(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return nil
	}

	collection.Fields.RemoveByName(schema.FieldHashCatalog)
	collection.RemoveIndex("idx_books_owner_hash_catalog")

	return app.Save(collection)
}

// addCatalogHash adds the hash of the name the catalog serves a book under.
func addCatalogHash(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionBooks, err)
	}

	collection.Fields.Add(&core.TextField{
		Name: schema.FieldHashCatalog,
		Max:  32,
	})

	// Matching a push means looking a book up by whichever of the three hashes
	// the reader sent, so this one is indexed like the other two.
	collection.AddIndex("idx_books_owner_hash_catalog", false, "owner,hash_catalog", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add %q to %q: %w", schema.FieldHashCatalog, schema.CollectionBooks, err)
	}

	return nil
}

// BackfillCatalogHashes fills the new hash for books that were uploaded before
// the catalog existed.
//
// It writes with a plain statement rather than through app.Save on purpose: the
// hash is derived, and a library's worth of records should not all report
// themselves as edited today because a column was added.
func BackfillCatalogHashes(app core.App) error {
	records, err := app.FindAllRecords(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("load %q: %w", schema.CollectionBooks, err)
	}

	for _, record := range records {
		hash := books.CatalogHash(record)
		if hash == record.GetString(schema.FieldHashCatalog) {
			continue
		}

		_, err := app.DB().
			NewQuery("UPDATE {{" + schema.CollectionBooks + "}} SET [[hash_catalog]] = {:hash} WHERE [[id]] = {:id}").
			Bind(dbx.Params{"hash": hash, "id": record.Id}).
			Execute()
		if err != nil {
			return fmt.Errorf("set the catalog hash of %q: %w", record.Id, err)
		}
	}

	return nil
}
