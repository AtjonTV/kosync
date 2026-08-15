//
// File:        internal/migrations/1787529600_achievements.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(upAchievements, downAchievements)
}

func upAchievements(app core.App) error {
	return createAchievements(app)
}

func downAchievements(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionAchievements)
	if err != nil {
		return nil
	}

	return app.Delete(collection)
}

// createAchievements creates the record of what an account has earned.
//
// One row per owner, rule and tier, written by the server when a threshold is
// first crossed. A client can read them and nothing else: an achievement that
// could be granted from the browser would not be worth having, and one that
// could be deleted would make the "never revoked" rule a lie.
//
// Never revoked is the design, not an omission. The measures are recomputed from
// live data, and live data moves backwards: history can be deleted, a re-read
// puts progress back to the start, and the retention window eventually removes
// the daily rows a streak was counted from. An achievement records that
// something happened, so once it has happened it stays happened.
func createAchievements(app core.App) error {
	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	collection := core.NewBaseCollection(schema.CollectionAchievements)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  users.Id,
		CascadeDelete: true,
	})
	// The rule's slug rather than a relation: the rules are code, because each
	// one is a question only code can ask, and a table of their names would be a
	// second place for them to disagree from.
	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldRule,
		Required: true,
		Max:      64,
	})
	collection.Fields.Add(&core.NumberField{
		Name:     schema.FieldTier,
		Required: true,
		OnlyInt:  true,
		Min:      types.Pointer(1.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldValue,
		OnlyInt: true,
	})
	collection.Fields.Add(&core.DateField{
		Name:     schema.FieldEarnedAt,
		Required: true,
	})
	collection.Fields.Add(&core.AutodateField{
		Name:     schema.FieldCreated,
		OnCreate: true,
	})

	collection.AddIndex("idx_achievements_owner_rule_tier", true, "owner,rule,tier", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionAchievements, err)
	}

	return nil
}
