//
// File:        internal/achievements/achievements.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package achievements recognises what an account has read.
//
// The rules are in rules.go and the questions they ask are in measure.go; this
// is only the awarding. Two properties are worth stating because everything else
// follows from them:
//
// Awarding is idempotent. Evaluating an account that has earned nothing new
// writes nothing, which is what makes it safe to run after every statistics
// batch rather than at some carefully chosen moment.
//
// Nothing is ever revoked. Every measure is recomputed from live data, and live
// data moves backwards — history gets deleted, a re-read puts progress back to
// the start, and the retention window eventually removes the daily rows a streak
// was counted from. An achievement records that something happened, and it
// having happened does not stop being true.
package achievements

import (
	"database/sql"
	"errors"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Awarded is one tier an account has just earned.
type Awarded struct {
	Rule  Rule
	Tier  int
	Value int
}

// Evaluate measures every rule for one account and awards what it has reached.
//
// It returns only what was new, so a caller can tell somebody about it. A first
// evaluation of an account that has been reading for years awards every tier it
// qualifies for at once rather than one per day, because the alternative is
// dribbling out a year of history over a week.
func Evaluate(app core.App, ownerId string) ([]Awarded, error) {
	if ownerId == "" {
		return nil, nil
	}

	earned := []Awarded{}

	for _, rule := range Rules {
		value, err := rule.Measure(app, ownerId)
		if err != nil {
			// One rule failing must not cost the others. A missed award is
			// picked up by the next evaluation; an aborted pass is not.
			app.Logger().Warn("failed to measure an achievement",
				"owner", ownerId, "rule", rule.Slug, "error", err)

			continue
		}

		for tier := 1; tier <= rule.EarnedTier(value); tier++ {
			awarded, err := award(app, ownerId, rule, tier, value)
			if err != nil {
				app.Logger().Warn("failed to award an achievement",
					"owner", ownerId, "rule", rule.Slug, "tier", tier, "error", err)

				continue
			}
			if awarded {
				earned = append(earned, Awarded{Rule: rule, Tier: tier, Value: value})
			}
		}
	}

	return earned, nil
}

// award records one tier, and reports whether it was new.
func award(app core.App, ownerId string, rule Rule, tier, value int) (bool, error) {
	existing, err := app.FindFirstRecordByFilter(
		schema.CollectionAchievements,
		"owner = {:owner} && rule = {:rule} && tier = {:tier}",
		dbx.Params{"owner": ownerId, "rule": rule.Slug, "tier": tier},
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if existing != nil {
		return false, nil
	}

	collection, err := app.FindCollectionByNameOrId(schema.CollectionAchievements)
	if err != nil {
		return false, err
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldOwner, ownerId)
	record.Set(schema.FieldRule, rule.Slug)
	record.Set(schema.FieldTier, tier)
	record.Set(schema.FieldValue, value)
	// The moment it was noticed, not the moment it was reached. Those differ by
	// however long the queue took, and claiming otherwise would mean guessing.
	record.Set(schema.FieldEarnedAt, time.Now().UTC())

	if err := app.Save(record); err != nil {
		return false, err
	}

	return true, nil
}

// Progress is one rule as it stands for an account: what has been earned, and
// how far the next tier is.
type Progress struct {
	Rule  Rule
	Tier  int
	Value int
	Next  int
}

// Measure reports every rule for one account without awarding anything, which is
// what the interface needs to show a badge that has not been earned yet.
func Measure(app core.App, ownerId string) ([]Progress, error) {
	progress := make([]Progress, 0, len(Rules))

	for _, rule := range Rules {
		value, err := rule.Measure(app, ownerId)
		if err != nil {
			return nil, err
		}

		tier := rule.EarnedTier(value)
		next := 0
		if tier < len(rule.Tiers) {
			next = rule.Tiers[tier]
		}

		progress = append(progress, Progress{Rule: rule, Tier: tier, Value: value, Next: next})
	}

	return progress, nil
}
