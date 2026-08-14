//
// File:        internal/migrations/1786838400_backfill_document_book.go
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
	m.Register(upBackfillDocumentBook, downBackfillDocumentBook)
}

// upBackfillDocumentBook links the documents and books that already existed.
//
// Matching runs when a document is created and when a book is uploaded, so
// neither hook ever sees a pair that was already in the database when matching
// was added. Without this, an instance that uploaded books before the feature
// existed would show no reading progress on any of them, permanently, and
// re-uploading the file would be the only way to fix it.
//
// The query is written out here rather than calling internal/books, so that
// what this migration does cannot change under it later.
func upBackfillDocumentBook(app core.App) error {
	return BackfillDocumentBook(app)
}

// BackfillDocumentBook attaches every unlinked document to the owner's book
// carrying its hash, on either of the two hash columns.
func BackfillDocumentBook(app core.App) error {
	documents := "{{" + schema.CollectionDocuments + "}}"
	books := "{{" + schema.CollectionBooks + "}}"

	// A correlated subquery rather than a join, because SQLite's UPDATE ... FROM
	// is newer than the oldest builds this may run on, and the row counts here
	// are small either way.
	match := fmt.Sprintf(
		`SELECT b.[[id]] FROM %s b
		 WHERE b.[[owner]] = d.[[owner]]
		   AND (b.[[hash_binary]] = d.[[document]] OR b.[[hash_filename]] = d.[[document]])
		 LIMIT 1`, books)

	query := fmt.Sprintf(
		`UPDATE %s AS d
		 SET [[book]] = (%s)
		 WHERE d.[[book]] = '' AND (%s) IS NOT NULL`,
		documents, match, match)

	if _, err := app.DB().NewQuery(query).Execute(); err != nil {
		return fmt.Errorf("backfill %q: %w", schema.FieldBook, err)
	}

	return nil
}

// downBackfillDocumentBook does nothing: the links it made are indistinguishable
// from the ones the hooks make, and undoing them would throw away good data.
func downBackfillDocumentBook(_ core.App) error {
	return nil
}
