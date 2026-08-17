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
package ownership

import (
	"git.obth.eu/atjontv/kosync/internal/schema"
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
