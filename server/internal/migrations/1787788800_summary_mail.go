//
// File:        internal/migrations/1787788800_summary_mail.go
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
	m.Register(upSummaryMail, downSummaryMail)
}

func upSummaryMail(app core.App) error {
	return addSummaryMail(app)
}

func downSummaryMail(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return nil
	}

	collection.Fields.RemoveByName(schema.FieldSummaryMail)
	collection.Fields.RemoveByName(schema.FieldSummarySent)

	return app.Save(collection)
}

// addSummaryMail lets an account ask for a report on its own reading.
//
// A cadence rather than a switch, because weekly and monthly are not degrees of
// the same thing: a week says something about a habit and a month says something
// about a book. Unset is off, and unlike the achievement notice this is not
// backfilled for the accounts that predate it — those accounts asked for
// milestone mail once and nobody asked them about this. It is one click in the
// interface for anybody who wants it.
func addSummaryMail(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	collection.Fields.Add(&core.SelectField{
		Name:      schema.FieldSummaryMail,
		MaxSelect: 1,
		Values:    []string{schema.SummaryOff, schema.SummaryWeekly, schema.SummaryMonthly},
	})

	// The period the last summary covered, which is what keeps a server that
	// was switched off over the weekend from either skipping Monday's mail or
	// sending it three times when it comes back.
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldSummarySent,
		Max:  16,
	})

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add %q to %q: %w", schema.FieldSummaryMail, schema.CollectionUsers, err)
	}

	return nil
}
