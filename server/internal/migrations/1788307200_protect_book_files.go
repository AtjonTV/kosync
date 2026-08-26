//
// File:        internal/migrations/1788307200_protect_book_files.go
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
	m.Register(upProtectBookFiles, downProtectBookFiles)
}

// bookFileTokenSeconds is how long a file access token stays good for.
//
// PocketBase's own default is three minutes, which is the right number for a
// link somebody clicks and a poor one for a wall of covers: the address of a
// protected file carries the token, so every renewal changes the address of
// every image on the page and the browser fetches the lot again. Half an hour
// is long enough that a library is not re-downloaded while it is being read
// through, and short enough that an address copied out of a browser's history
// stops working the same afternoon.
const bookFileTokenSeconds = 1800

func upProtectBookFiles(app core.App) error {
	if err := setBookFilesProtected(app, true); err != nil {
		return err
	}

	return setFileTokenDuration(app, bookFileTokenSeconds)
}

func downProtectBookFiles(app core.App) error {
	if err := setBookFilesProtected(app, false); err != nil {
		return err
	}

	// PocketBase's default, restored by hand because nothing records what it
	// was before this ran.
	return setFileTokenDuration(app, 180)
}

// setBookFilesProtected decides whether the books' EPUB and cover can be
// fetched by address alone.
//
// They could, until this ran. A file address is unguessable — a record id and a
// stored name with a random suffix — but unguessable is not the same as
// private: it never expires, it survives the account being deleted, and it is
// in every browser history and proxy log that has ever seen it. The collection
// says a book belongs to one account and nobody else may look at it, and that
// rule stopped at the record; a protected field carries it to the file, where
// PocketBase checks the same view rule against a short lived token.
//
// The catalog is unaffected: it serves its files itself, from
// internal/opds/files.go, behind the device authentication the feed already
// requires. That was written on the assumption this was already true.
func setBookFilesProtected(app core.App, protected bool) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionBooks, err)
	}

	for _, name := range []string{schema.FieldFile, schema.FieldCover} {
		field, ok := collection.Fields.GetByName(name).(*core.FileField)
		if !ok {
			return fmt.Errorf("%q of %q is not a file field", name, schema.CollectionBooks)
		}

		field.Protected = protected
	}

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("protect the files of %q: %w", schema.CollectionBooks, err)
	}

	return nil
}

// setFileTokenDuration sets how long the tokens that open a protected file last.
//
// The setting lives on the account collection rather than on the file, because
// the token stands for the reader and not for the thing being read.
func setFileTokenDuration(app core.App, seconds int64) error {
	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	users.FileToken.Duration = seconds

	if err := app.Save(users); err != nil {
		return fmt.Errorf("set the file token duration of %q: %w", schema.CollectionUsers, err)
	}

	return nil
}
