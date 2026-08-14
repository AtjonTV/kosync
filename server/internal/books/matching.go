//
// File:        internal/books/matching.go
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

// hashFields are the columns a document hash may be found in. A reader sends
// whichever hash its checksum method produces and the server cannot tell which
// kind it is looking at — they are 32 hex characters either way — so all three
// are tried.
var hashFields = []string{schema.FieldHashBinary, schema.FieldHashFilename, schema.FieldHashCatalog}

// FindForDocument returns the owner's book that a document hash identifies, or
// nil when there is none.
func FindForDocument(app core.App, owner, documentHash string) (*core.Record, error) {
	if owner == "" || documentHash == "" {
		return nil, nil
	}

	matches := make([]string, 0, len(hashFields))
	for _, field := range hashFields {
		matches = append(matches, field+" = {:hash}")
	}

	records, err := app.FindRecordsByFilter(
		schema.CollectionBooks,
		fmt.Sprintf("%s = {:owner} && (%s)", schema.FieldOwner, strings.Join(matches, " || ")),
		"", 1, 0,
		map[string]any{"owner": owner, "hash": documentHash},
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}

	return records[0], nil
}

// registerMatching links documents to books, in both directions.
//
// Both are needed, and the second is the common one: people upload a book after
// they have been reading it, so the link has to be made retroactively as well
// as on arrival.
func registerMatching(app core.App) {
	// A document is created once, on the first push for that hash, and updated
	// on every push afterwards. Matching on create alone therefore costs one
	// indexed lookup per book rather than one per push.
	app.OnRecordCreate(schema.CollectionDocuments).BindFunc(func(e *core.RecordEvent) error {
		if err := linkDocument(e.App, e.Record); err != nil {
			// A failed lookup must not cost a device its progress push. The
			// book link is a convenience; the reading position is not.
			e.App.Logger().Warn("could not match a document to a book",
				"document", e.Record.GetString(schema.FieldDocument), "error", err)
		}

		return e.Next()
	})

	app.OnRecordAfterCreateSuccess(schema.CollectionBooks).BindFunc(func(e *core.RecordEvent) error {
		if err := linkExistingDocuments(e.App, e.Record); err != nil {
			e.App.Logger().Warn("could not link existing documents to an uploaded book",
				"book", e.Record.Id, "error", err)
		}

		return e.Next()
	})
}

// linkDocument sets the book of a document that does not have one yet.
func linkDocument(app core.App, document *core.Record) error {
	if document.GetString(schema.FieldBook) != "" {
		return nil
	}

	book, err := FindForDocument(app, document.GetString(schema.FieldOwner), document.GetString(schema.FieldDocument))
	if err != nil || book == nil {
		return err
	}

	document.Set(schema.FieldBook, book.Id)

	return nil
}

// linkExistingDocuments attaches a freshly uploaded book to the documents that
// were already recording progress through it.
func linkExistingDocuments(app core.App, book *core.Record) error {
	owner := book.GetString(schema.FieldOwner)
	params := map[string]any{"owner": owner}
	matches := []string{}
	for index, field := range hashFields {
		hash := book.GetString(field)
		if hash == "" {
			continue
		}
		name := fmt.Sprintf("hash%d", index)
		params[name] = hash
		matches = append(matches, fmt.Sprintf("%s = {:%s}", schema.FieldDocument, name))
	}
	if owner == "" || len(matches) == 0 {
		return nil
	}

	documents, err := app.FindRecordsByFilter(
		schema.CollectionDocuments,
		fmt.Sprintf("%s = {:owner} && %s = '' && (%s)",
			schema.FieldOwner, schema.FieldBook, strings.Join(matches, " || ")),
		"", 0, 0,
		params,
	)
	if err != nil {
		return err
	}

	for _, document := range documents {
		document.Set(schema.FieldBook, book.Id)
		if err := app.Save(document); err != nil {
			return err
		}
	}

	return nil
}
