//
// File:        internal/migrations/1787443200_account_timezone.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/timezone"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(upAccountTimezone, downAccountTimezone)
}

func upAccountTimezone(app core.App) error {
	return addAccountTimezone(app)
}

func downAccountTimezone(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return nil
	}

	collection.Fields.RemoveByName(schema.FieldTimezone)

	return app.Save(collection)
}

// addAccountTimezone gives an account the zone its reading days are reckoned in.
//
// Every timestamp KOsync holds is UTC, because the sync protocol carries no
// clock at all: the body of a progress push is the document, the position and
// the device, and the three headers are authentication. There is nothing in it
// to tell the server what time the reader thinks it is. So the zone has to come
// from the only other place a person meets this server — the browser, which
// knows it and will say so for free.
//
// Existing accounts keep UTC, which is what their statistics were already
// computed in, so this migration changes no number. Choosing a zone does, and
// that is handled where the choice is made: the change requeues every day the
// account has ever read.
func addAccountTimezone(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	collection.Fields.Add(&core.TextField{
		Name: schema.FieldTimezone,
		Max:  64,
	})

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add %q to %q: %w", schema.FieldTimezone, schema.CollectionUsers, err)
	}

	return backfillTimezone(app)
}

// backfillTimezone writes the default into the accounts that predate the field,
// so that "not set" is never something the rest of the code has to think about.
func backfillTimezone(app core.App) error {
	_, err := app.DB().
		NewQuery(`
			UPDATE {{` + schema.CollectionUsers + `}}
			SET [[` + schema.FieldTimezone + `]] = {:zone}
			WHERE [[` + schema.FieldTimezone + `]] = '' OR [[` + schema.FieldTimezone + `]] IS NULL
		`).
		Bind(map[string]any{"zone": timezone.Default}).
		Execute()

	return err
}
