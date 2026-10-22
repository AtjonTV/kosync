//
// File:        internal/ownership/ownership.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package ownership keeps a record from changing hands.
//
// Every KOsync collection an account can write to is scoped by schema.OwnerRule,
// and that rule is a filter over the record as it is stored. PocketBase checks
// it once, against the row it loaded, and then applies the request body — so the
// rule that decides whether a record may be written says nothing about whose it
// is afterwards. Left alone, an account can hand its own record to somebody else
// simply by naming them in the payload.
//
// On a KOReader credential that is not a gift but a theft: the account still
// knows the device password, so a credential moved to another owner keeps
// authenticating and now resolves to that owner's documents, books and
// statistics on every device facing route.
//
// Freeze is registered from main for the collections that have nothing else to
// say on the subject. Two collections state it themselves instead: devices,
// where the owner is one of several fields the device reports and the account
// may not touch at all, and book_collections, where the check has to run before
// the books on the shelf are validated against it.
//
// Confine is the same sentence about a record's relations rather than its own
// owner: a rule filters the record being written, and says nothing about the
// records its relation fields point at.
package ownership

import (
	"slices"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Freeze refuses every request that would move a record of the given
// collections from one account to another.
//
// The stored owner is compared against the submitted one rather than looked for
// in the request body, so a client that sends the whole record back — as an edit
// form usually does — is not refused for a change it did not make.
func Freeze(app core.App, collections ...string) {
	for _, collection := range collections {
		app.OnRecordUpdateRequest(collection).BindFunc(func(e *core.RecordRequestEvent) error {
			if e.Record.GetString(schema.FieldOwner) != e.Record.Original().GetString(schema.FieldOwner) {
				return e.BadRequestError("A record cannot change owner.", nil)
			}

			return e.Next()
		})
	}
}

// Confine refuses every request that would point a record of the given
// collection at a record somebody else owns.
//
// The relations are given as field name to the collection the field points
// into, and every collection named there has to carry an owner. Without this an
// account can link its own document to another account's book by id, and
// everything that follows the link afterwards — the page count measured from the
// document's progress, the titles in a summary mail — reaches across into the
// other account.
//
// Only a value that changed is checked, for the same reason Freeze compares
// rather than looks: a client sending the whole record back has not linked
// anything.
func Confine(app core.App, collection string, relations map[string]string) {
	check := func(e *core.RecordRequestEvent) error {
		owner := e.Record.GetString(schema.FieldOwner)
		original := e.Record.Original()

		for field, target := range relations {
			ids := e.Record.GetStringSlice(field)
			if len(ids) == 0 || slices.Equal(ids, original.GetStringSlice(field)) {
				continue
			}

			wanted := make([]any, 0, len(ids))
			for _, id := range ids {
				wanted = append(wanted, id)
			}

			found, err := e.App.CountRecords(target,
				dbx.HashExp{schema.FieldOwner: owner},
				dbx.In("id", wanted...),
			)
			if err != nil {
				return e.InternalServerError("Failed to check the linked records.", err)
			}

			// GetStringSlice has dropped the duplicates, so a count says whether
			// every id resolved to one of the owner's own.
			if owner == "" || int(found) != len(ids) {
				return e.BadRequestError("A record can only be linked to your own records.", nil)
			}
		}

		return e.Next()
	}

	app.OnRecordCreateRequest(collection).BindFunc(check)
	app.OnRecordUpdateRequest(collection).BindFunc(check)
}
