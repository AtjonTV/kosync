//
// File:        internal/migrations/1786752000_document_book.go
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
	m.Register(upDocumentBook, downDocumentBook)
}

// upDocumentBook links a document to the book it is progress through.
//
// The relation deliberately does not cascade: deleting an uploaded file must
// not delete the reading progress made in it. PocketBase clears the reference
// instead, which leaves the document exactly as it was before the book was
// uploaded.
func upDocumentBook(app core.App) error {
	documents, err := app.FindCollectionByNameOrId(schema.CollectionDocuments)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionDocuments, err)
	}

	books, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionBooks, err)
	}

	documents.Fields.Add(&core.RelationField{
		Name:          schema.FieldBook,
		MaxSelect:     1,
		CollectionId:  books.Id,
		CascadeDelete: false,
	})

	// The library view asks for the documents of a book, and the documents view
	// asks which of them have no book at all.
	documents.AddIndex("idx_documents_owner_book", false, "owner,book", "")

	if err := app.Save(documents); err != nil {
		return fmt.Errorf("add %q to %q: %w", schema.FieldBook, schema.CollectionDocuments, err)
	}

	return nil
}

func downDocumentBook(app core.App) error {
	documents, err := app.FindCollectionByNameOrId(schema.CollectionDocuments)
	if err != nil {
		return nil
	}

	documents.Fields.RemoveByName(schema.FieldBook)
	documents.RemoveIndex("idx_documents_owner_book")

	return app.Save(documents)
}
