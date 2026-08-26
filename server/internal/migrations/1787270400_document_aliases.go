//
// File:        internal/migrations/1787270400_document_aliases.go
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
	m.Register(upDocumentAliases, downDocumentAliases)
}

func upDocumentAliases(app core.App) error {
	return createDocumentAliases(app)
}

func downDocumentAliases(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionDocumentAliases)
	if err != nil {
		return nil
	}

	return app.Delete(collection)
}

// createDocumentAliases creates the map from a retired document hash to the
// document it was merged into.
//
// Two devices can read the same book from two different copies of the file and
// report two different hashes, which arrive here as two documents with the
// reading split between them. Merging folds them into one — and the moment the
// second document is gone, the device that reported it would push its hash again
// and get a fresh document back, undoing the merge on the next sync. This is
// what stops that: the hash survives its document, pointing at the survivor.
//
// The rows are written by the merge, never by a client, so there is no create
// rule and no update rule. Deleting one is allowed, and is the way back: drop
// the alias and the next push from that device makes its own document again.
func createDocumentAliases(app core.App) error {
	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	documents, err := app.FindCollectionByNameOrId(schema.CollectionDocuments)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionDocuments, err)
	}

	collection := core.NewBaseCollection(schema.CollectionDocumentAliases)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = nil // written by the merge
	collection.UpdateRule = nil // a hash points where it points
	collection.DeleteRule = types.Pointer(schema.OwnerRule)

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  users.Id,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldDocument,
		Required: true,
		Min:      1,
		Max:      255,
	})
	// An alias without its document means nothing, so it goes when the document
	// does — and the hash is free to become a document of its own again.
	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldDocumentRef,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  documents.Id,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.AutodateField{
		Name:     schema.FieldCreated,
		OnCreate: true,
	})

	// The same uniqueness the documents themselves have: one owner, one meaning
	// per hash.
	collection.AddIndex("idx_document_aliases_owner_document", true, "owner,document", "")
	collection.AddIndex("idx_document_aliases_document_ref", false, "document_ref", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionDocumentAliases, err)
	}

	return nil
}
