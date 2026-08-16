//
// File:        internal/collections/collections.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package collections guards the shelves an account puts together by hand.
//
// The shelves themselves need no server: they are made, renamed and filled
// through the ordinary PocketBase collection API, and the owner rule keeps one
// account's out of another's. What the rule cannot say is that the books on a
// shelf have to be the owner's own — a rule is a filter over the record being
// written, and the books are a list of ids pointing somewhere else. So that one
// sentence is written here.
//
// The name is unfortunate and unavoidable: PocketBase calls its own tables
// collections too. Everything in this package means the KOsync kind — a shelf
// somebody built — and the PocketBase kind only ever appears as core.Collection.
package collections

import (
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Register wires the shelf checks into the application lifecycle.
func Register(app core.App) {
	app.OnRecordCreateRequest(schema.CollectionBookCollections).BindFunc(func(e *core.RecordRequestEvent) error {
		if err := ownsEveryBook(e); err != nil {
			return err
		}

		return e.Next()
	})

	app.OnRecordUpdateRequest(schema.CollectionBookCollections).BindFunc(func(e *core.RecordRequestEvent) error {
		// A shelf cannot be handed to somebody else. The owner rule already
		// refuses to load another account's shelf, so this is about the other
		// direction: giving one's own away, which nothing asks for and which
		// would leave a record its former owner can no longer see.
		//
		// Asked before the books are checked, because a changed owner makes
		// every book on the shelf somebody else's and the answer would then be
		// true but about the wrong thing.
		//
		// Compared against the stored value rather than looked for in the body,
		// so that a client sending the whole record back — the owner included and
		// unchanged — is not refused for a change it did not make.
		if e.Record.GetString(schema.FieldOwner) != e.Record.Original().GetString(schema.FieldOwner) {
			return e.BadRequestError("A collection cannot change owner.", nil)
		}

		if err := ownsEveryBook(e); err != nil {
			return err
		}

		return e.Next()
	})
}

// ownsEveryBook refuses a shelf holding books the account does not own.
//
// Without it an account could put any book id at all on a shelf and read the
// titles back through the relation's expansion, which is a way of asking what
// somebody else uploaded. The catalog is safe from this on its own — every feed
// starts from the books of one owner — but the collection API is not, and the
// answer belongs where the record is written rather than in each reader.
func ownsEveryBook(e *core.RecordRequestEvent) error {
	ids := e.Record.GetStringSlice(schema.FieldBooks)
	if len(ids) == 0 {
		return nil
	}

	owner := e.Record.GetString(schema.FieldOwner)
	if owner == "" {
		// Whose shelf this is has not been said, so no book can belong on it.
		// The owner rule turns this into the refusal it deserves; there is
		// simply nothing here to check against.
		return nil
	}

	wanted := make([]any, 0, len(ids))
	for _, id := range ids {
		wanted = append(wanted, id)
	}

	// GetStringSlice has already dropped the duplicates, so a plain count
	// answers the question: every id resolved, or some of them did not.
	found, err := e.App.CountRecords(schema.CollectionBooks,
		dbx.HashExp{schema.FieldOwner: owner},
		dbx.In("id", wanted...),
	)
	if err != nil {
		return e.InternalServerError("Failed to check the books.", err)
	}

	if int(found) != len(ids) {
		return e.BadRequestError("A collection can only hold books from your own library.", nil)
	}

	return nil
}
