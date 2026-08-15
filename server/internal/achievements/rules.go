//
// File:        internal/achievements/rules.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package achievements

import "github.com/pocketbase/pocketbase/core"

// Rule is one thing an account can be recognised for.
//
// The rules live in code rather than in a table because each one is a question
// that only code can ask — "how many nights did you read past midnight" is a
// timezone conversion, not a column — and a table holding their names beside the
// code holding their meaning is a second place for the two to disagree.
type Rule struct {
	// Slug identifies the rule in the stored records and in the interface.
	Slug string
	// Name is what the badge is called. It does not change with the tier: the
	// name is the badge's identity and the tier is how far it has been taken.
	Name string
	// Summary says what earns it, in one line.
	Summary string
	// Unit names what the measure counts, for "37 of 100 nights".
	Unit string
	// Icon is the sprite symbol the web interface draws.
	Icon string
	// Fur is the coat that symbol is drawn in.
	Fur string
	// Tiers are the thresholds, in order. Crossing one awards that tier and
	// every lower one, so a first evaluation of an account that has been reading
	// for years does not award them one at a time.
	Tiers []int
	// Measure answers the rule for one account, from live data.
	Measure func(app core.App, ownerId string) (int, error)
}

// TierNames are what the three levels are called, lowest first.
var TierNames = []string{"bronze", "silver", "gold"}

// Rules are every achievement, in the order they are shown.
//
// The thresholds are meant to be reachable rather than flattering: the first
// tier should arrive early enough to explain what the row is for, and the last
// should take a year of real reading.
var Rules = []Rule{
	{
		Slug:    "first-pounce",
		Name:    "First Pounce",
		Summary: "Books you have read to the end.",
		Unit:    "books",
		Icon:    "ach-first",
		Fur:     "ginger",
		Tiers:   []int{1, 10, 50},
		Measure: booksFinished,
	},
	{
		Slug:    "page-turner",
		Name:    "Page Turner",
		Summary: "Pages read, counted from your own reading.",
		Unit:    "pages",
		Icon:    "ach-page",
		Fur:     "calico",
		Tiers:   []int{1000, 10000, 100000},
		Measure: pagesRead,
	},
	{
		Slug:    "shelf-inspector",
		Name:    "Shelf Inspector",
		Summary: "Books uploaded to your library.",
		Unit:    "books",
		Icon:    "ach-shelf",
		Fur:     "grey",
		Tiers:   []int{10, 50, 200},
		Measure: booksInLibrary,
	},
	{
		Slug:    "night-prowler",
		Name:    "Night Prowler",
		Summary: "Nights you were still reading after midnight.",
		Unit:    "nights",
		Icon:    "ach-night",
		Fur:     "soot",
		Tiers:   []int{1, 25, 100},
		Measure: lateNights,
	},
	{
		Slug:    "lap-warmer",
		Name:    "Lap Warmer",
		Summary: "Your longest run of days without missing one.",
		Unit:    "days",
		Icon:    "ach-streak",
		Fur:     "cream",
		Tiers:   []int{7, 30, 100},
		Measure: longestStreak,
	},
}

// FindRule returns the rule with the given slug.
func FindRule(slug string) (Rule, bool) {
	for _, rule := range Rules {
		if rule.Slug == slug {
			return rule, true
		}
	}

	return Rule{}, false
}

// EarnedTier returns the highest tier a measured value has reached, where zero
// means none of them.
func (r Rule) EarnedTier(value int) int {
	tier := 0
	for index, threshold := range r.Tiers {
		if value >= threshold {
			tier = index + 1
		}
	}

	return tier
}
