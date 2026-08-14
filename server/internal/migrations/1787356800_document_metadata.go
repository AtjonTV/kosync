//
// File:        internal/migrations/1787356800_document_metadata.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(upDocumentMetadata, downDocumentMetadata)
}

func upDocumentMetadata(app core.App) error {
	return addDocumentMetadata(app)
}

func downDocumentMetadata(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionDocuments)
	if err != nil {
		return nil
	}

	for _, name := range []string{schema.FieldFilename, schema.FieldFilenameHash, schema.FieldDocumentAuthors} {
		collection.Fields.RemoveByName(name)
	}
	collection.RemoveIndex("idx_documents_owner_filename_hash")

	return app.Save(collection)
}

// addDocumentMetadata makes room for what KOReader can say about a file.
//
// The sync plugin has a setting called "send document metadata", off by default,
// which adds the filename, title and authors to every progress push. Its own
// help text says the official server ignores it and a custom one may use it —
// which is this server exactly.
//
// It is worth having because of the documents that never match a book. Those are
// the ones the documents page exists for, and until now the only thing they
// could be called was a 32 character hash.
func addDocumentMetadata(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionDocuments)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionDocuments, err)
	}

	collection.Fields.Add(&core.TextField{
		Name: schema.FieldFilename,
		Max:  512,
	})
	// Not a relation to anything and not unique: two devices reading two copies
	// of one book under the same name is exactly the case the merge exists for,
	// and this is evidence about it rather than an identity.
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldFilenameHash,
		Max:  32,
	})
	// Plain text rather than the JSON list that books carry: KOReader sends
	// whatever its own metadata reader produced, and pretending that is
	// structured would be inventing a shape nobody promised.
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldDocumentAuthors,
		Max:  512,
	})

	collection.AddIndex("idx_documents_owner_filename_hash", false, "owner,filename_hash", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add the metadata fields to %q: %w", schema.CollectionDocuments, err)
	}

	return nil
}
