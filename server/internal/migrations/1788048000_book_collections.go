//
// File:        internal/migrations/1788048000_book_collections.go
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
	m.Register(upBookCollections, downBookCollections)
}

func upBookCollections(app core.App) error {
	return createBookCollections(app)
}

func downBookCollections(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBookCollections)
	if err != nil {
		return nil
	}

	return app.Delete(collection)
}

// maxCollectionBooks is how many books one shelf may hold.
//
// A relation field has to be told a ceiling to be a list at all — PocketBase
// reads a maximum of one as a single value — so the number exists because the
// field needs one rather than because anybody wants a limit. It is set far past
// any library this serves: the reference library is 192 books, and a shelf of
// two thousand hand-picked ones is not a shelf.
const maxCollectionBooks = 2000

// createBookCollections creates the shelves an account puts together by hand.
//
// This is the one collection whose contents are entirely somebody's own opinion.
// Everything else in KOsync is read out of a file or reported by a device, and
// so is created by the server; a shelf is made, renamed, filled and thrown away
// by its owner, which is why all five rules are the owner rule and none of them
// is nil.
func createBookCollections(app core.App) error {
	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	books, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionBooks, err)
	}

	collection := core.NewBaseCollection(schema.CollectionBookCollections)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = types.Pointer(schema.OwnerRule)
	collection.UpdateRule = types.Pointer(schema.OwnerRule)
	collection.DeleteRule = types.Pointer(schema.OwnerRule)

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  users.Id,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldName,
		Required: true,
		Max:      100,
	})
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldDescription,
		Max:  1000,
	})
	// Not a cascade delete: PocketBase removes a deleted book's id from every
	// list that named it, and only deletes the record holding the list when it
	// empties. Deleting the last book of a shelf must not delete the shelf —
	// somebody who clears out a reading list still has the reading list.
	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldBooks,
		MaxSelect:     maxCollectionBooks,
		CollectionId:  books.Id,
		CascadeDelete: false,
	})
	addTimestamps(collection)

	// Two shelves of one name are two answers to the same question. The index
	// makes that a validation error on the name, which the browser can say
	// something useful about, rather than a puzzle to be found later.
	collection.AddIndex("idx_book_collections_owner_name", true, "owner,name", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionBookCollections, err)
	}

	return nil
}
