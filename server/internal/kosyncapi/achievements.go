//
// File:        internal/kosyncapi/achievements.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi

import (
	"net/http"

	"git.obth.eu/atjontv/kosync/internal/achievements"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// jsonAchievement is one rule as it stands for the signed in account.
type jsonAchievement struct {
	Rule    string `json:"rule"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Unit    string `json:"unit"`
	Icon    string `json:"icon"`
	Fur     string `json:"fur"`
	Tiers   []int  `json:"tiers"`

	// Value is the measure right now, Tier the highest threshold it has
	// reached, and Next the one after that — zero when there is no next.
	Value int `json:"value"`
	Tier  int `json:"tier"`
	Next  int `json:"next"`

	// Earned is when each tier was first noticed, which can be earlier than the
	// current measure suggests: nothing is ever taken away.
	Earned []jsonEarned `json:"earned"`
}

// jsonEarned is one tier that has been awarded.
type jsonEarned struct {
	Tier  int    `json:"tier"`
	Value int    `json:"value"`
	At    string `json:"at"`
}

// listAchievements returns every rule with the account's standing in it.
//
// The rules themselves are served rather than duplicated in the browser. They
// are code — "how many nights did you read past midnight" is a timezone
// conversion, not a column — and a copy of their names and thresholds in the
// interface would be a second place for the two to disagree from.
//
// Unearned rules are included, because a badge nobody has yet is the one worth
// showing: it is the only thing that says what there is to aim at.
func (h *Handler) listAchievements(e *core.RequestEvent) error {
	progress, err := achievements.Measure(e.App, e.Auth.Id)
	if err != nil {
		return e.InternalServerError("Failed to measure your achievements.", err)
	}

	earned, err := e.App.FindAllRecords(schema.CollectionAchievements,
		dbx.HashExp{schema.FieldOwner: e.Auth.Id})
	if err != nil {
		return e.InternalServerError("Failed to load your achievements.", err)
	}

	byRule := map[string][]jsonEarned{}
	for _, record := range earned {
		slug := record.GetString(schema.FieldRule)
		byRule[slug] = append(byRule[slug], jsonEarned{
			Tier:  record.GetInt(schema.FieldTier),
			Value: record.GetInt(schema.FieldValue),
			At:    record.GetDateTime(schema.FieldEarnedAt).String(),
		})
	}

	list := make([]jsonAchievement, 0, len(progress))
	for _, entry := range progress {
		// The measure is live and live data moves backwards, so on its own it
		// would take a badge away — history deleted, a book started again, the
		// retention window removing the days a streak was counted from. What was
		// awarded is the durable answer and wins wherever the two disagree.
		tier, next := entry.Tier, entry.Next
		for _, one := range byRule[entry.Rule.Slug] {
			if one.Tier > tier {
				tier = one.Tier
				next = 0
				if tier < len(entry.Rule.Tiers) {
					next = entry.Rule.Tiers[tier]
				}
			}
		}

		list = append(list, jsonAchievement{
			Rule:    entry.Rule.Slug,
			Name:    entry.Rule.Name,
			Summary: entry.Rule.Summary,
			Unit:    entry.Rule.Unit,
			Icon:    entry.Rule.Icon,
			Fur:     entry.Rule.Fur,
			Tiers:   entry.Rule.Tiers,
			Value:   entry.Value,
			Tier:    tier,
			Next:    next,
			Earned:  byRule[entry.Rule.Slug],
		})
	}

	return e.JSON(http.StatusOK, map[string]any{"achievements": list})
}
