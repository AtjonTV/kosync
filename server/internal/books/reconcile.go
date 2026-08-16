//
// File:        internal/books/reconcile.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books

import (
	"fmt"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// Cron job id and schedule for the reconcile below. Daily, in the quiet hour
// the other maintenance jobs already run in and far enough from them that two
// of them are never writing at once.
const (
	JobReconcile      = "kosync.books.reconcile"
	scheduleReconcile = "30 3 * * *"
)

// pair is one document and the book it should have been linked to.
type pair struct {
	Document string `db:"document"`
	Book     string `db:"book"`
}

// reconcileQuery finds the documents a book exists for that were never linked
// to it.
//
// It is a join rather than a lookup per document because the ordinary case is a
// library where nothing is missing: on an account with two hundred documents and
// no gaps this is one indexed scan that returns nothing, run once a day, instead
// of two hundred lookups that each find the answer that was already known.
//
// MIN over the book ids only matters when the same hash was uploaded twice,
// which the content hash mostly prevents. Picking the lower id is not a better
// answer than picking the other one, but it is the same answer every run, and a
// link that flickers between two books would be worse than either.
func reconcileQuery() string {
	matches := make([]string, 0, len(hashFields)*2)
	for _, field := range hashFields {
		// The emptiness guard is the one that matters: a book with no catalog
		// hash and a document with no reported filename would otherwise be
		// matched to each other by two empty strings being equal.
		for _, against := range []string{schema.FieldDocument, schema.FieldFilenameHash} {
			matches = append(matches,
				fmt.Sprintf("(b.[[%s]] != '' AND b.[[%s]] = d.[[%s]])", field, field, against))
		}
	}

	return `
		SELECT d.[[id]] AS document, MIN(b.[[id]]) AS book
		FROM {{` + schema.CollectionDocuments + `}} d
		JOIN {{` + schema.CollectionBooks + `}} b
			ON b.[[` + schema.FieldOwner + `]] = d.[[` + schema.FieldOwner + `]]
			AND (` + strings.Join(matches, " OR ") + `)
		WHERE d.[[` + schema.FieldBook + `]] = ''
		GROUP BY d.[[id]]
	`
}

// Reconcile links documents to the books that match them and returns how many
// it repaired.
//
// Matching normally happens as things arrive: on the push that creates a
// document, on the rename that gives it a filename, and on the upload that
// brings a book to documents already waiting for it. All three can fail — a
// locked database, a lookup that errors — and none of them can be retried,
// because a device must never lose a reading position over a link that is only
// a convenience. A failed match therefore used to be permanent, with
// re-uploading the file the only way out.
//
// This is the retry. It asks the whole question again on a schedule, which is
// cheap because the answer is almost always "nothing".
func Reconcile(app core.App) (int, error) {
	pairs := []pair{}
	if err := app.DB().NewQuery(reconcileQuery()).All(&pairs); err != nil {
		return 0, fmt.Errorf("find the unlinked documents: %w", err)
	}

	linked := 0
	for _, found := range pairs {
		document, err := app.FindRecordById(schema.CollectionDocuments, found.Document)
		if err != nil {
			return linked, fmt.Errorf("load the document %s: %w", found.Document, err)
		}

		// The query saw the row before this loop started saving, so a link made
		// in between — by an upload arriving now — is left as it is.
		if document.GetString(schema.FieldBook) != "" {
			continue
		}

		document.Set(schema.FieldBook, found.Book)

		// Saved as a record rather than updated in SQL, because the link is what
		// the statistics count pages by and what the library draws progress
		// from: the record hooks queue every day of that document for
		// recomputation, and the realtime event puts the progress on the cover
		// without a reload.
		if err := app.Save(document); err != nil {
			return linked, fmt.Errorf("link the document %s to the book %s: %w", found.Document, found.Book, err)
		}

		linked++
	}

	return linked, nil
}
