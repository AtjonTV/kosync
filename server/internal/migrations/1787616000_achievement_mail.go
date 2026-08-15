//
// File:        internal/migrations/1787616000_achievement_mail.go
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
	m.Register(upAchievementMail, downAchievementMail)
}

func upAchievementMail(app core.App) error {
	return addAchievementMail(app)
}

func downAchievementMail(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return nil
	}

	collection.Fields.RemoveByName(schema.FieldAchievementMail)

	return app.Save(collection)
}

// addAchievementMail lets an account say whether it wants to hear about what it
// has earned.
//
// The field is positive — true means send — because a boolean is false when it
// has never been set, and for unsolicited mail the safe end of that is silence.
// An account created through the API or the superuser interface therefore gets
// no mail until somebody ticks the box, which is the right way for that mistake
// to fall.
func addAchievementMail(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	collection.Fields.Add(&core.BoolField{
		Name: schema.FieldAchievementMail,
	})

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("add %q to %q: %w", schema.FieldAchievementMail, schema.CollectionUsers, err)
	}

	return backfillAchievementMail(app)
}

// backfillAchievementMail turns it on for the accounts that predate the field.
//
// These are people who were already using the server when this arrived, so the
// mail is about their own reading and they can turn it off in one click. That is
// a different situation from an account created by a script, which has nobody
// waiting to read anything.
func backfillAchievementMail(app core.App) error {
	_, err := app.DB().
		NewQuery(`
			UPDATE {{` + schema.CollectionUsers + `}}
			SET [[` + schema.FieldAchievementMail + `]] = TRUE
		`).
		Execute()

	return err
}
