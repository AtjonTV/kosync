//
// File:        internal/pages/estimate_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package pages_test

import (
	"math"
	"math/rand"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/pages"
)

// grid is the resolution KOReader reports progress at.
const grid = 0.0001

// series builds a run of progress values the way a device produces them: a
// push every syncPages pages, occasional single-page pushes where a chapter
// ends, occasional jumps, and everything rounded to the reporting grid.
type series struct {
	pages     int
	syncPages int
	pushes    int
	partials  float64 // share of pushes that cover one page instead of syncPages
	jumps     float64 // share of pushes that skip ahead
	random    *rand.Rand
}

func (s series) build() []float64 {
	page := 1 / float64(s.pages)
	progress := []float64{0}
	current := 0.0

	for index := 0; index < s.pushes; index++ {
		step := s.syncPages
		switch {
		case s.random.Float64() < s.jumps:
			step = 20 + s.random.Intn(200)
		case s.random.Float64() < s.partials:
			step = 1
		}

		current += float64(step) * page
		if current >= 1 {
			break
		}
		progress = append(progress, math.Round(current/grid)*grid)
	}

	return progress
}

func TestFromProgressRecoversPageCount(t *testing.T) {
	cases := []struct {
		name      string
		pages     int
		syncPages int
	}{
		{name: "novel, sync every 2 pages", pages: 700, syncPages: 2},
		{name: "short book, sync every 2 pages", pages: 280, syncPages: 2},
		{name: "sync every page", pages: 500, syncPages: 1},
		{name: "sync every 5 pages", pages: 900, syncPages: 5},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			built := series{
				pages:     testCase.pages,
				syncPages: testCase.syncPages,
				pushes:    240,
				partials:  0.09,
				jumps:     0.02,
				random:    rand.New(rand.NewSource(1)), // #nosec G404 -- fixture data
			}.build()

			estimate, ok := pages.FromProgress(built)
			if !ok {
				t.Fatalf("no estimate from %d pushes", len(built))
			}

			// One page either way is the honest expectation: pages are rounded
			// onto a coarse reporting grid.
			if difference := estimate.Pages - testCase.pages; difference < -1 || difference > 1 {
				t.Errorf("estimated %d pages, want %d", estimate.Pages, testCase.pages)
			}
			if estimate.SyncPages != testCase.syncPages {
				t.Errorf("estimated sync every %d pages, want %d", estimate.SyncPages, testCase.syncPages)
			}
		})
	}
}

// A book whose pages are narrower than the reporting grid can resolve must be
// refused rather than guessed at. Without the resolution floor the estimator
// returns the grid itself as the page size, which looks entirely plausible.
func TestFromProgressDeclinesBelowReportingResolution(t *testing.T) {
	built := series{
		pages:     3562,
		syncPages: 2,
		pushes:    600,
		partials:  0.09,
		jumps:     0.01,
		random:    rand.New(rand.NewSource(2)), // #nosec G404 -- fixture data
	}.build()

	if estimate, ok := pages.FromProgress(built); ok {
		t.Errorf("estimated %d pages from a book below the reporting resolution", estimate.Pages)
	}
}

// Single-page pushes are the signal that reveals the page unit. A threshold
// loose enough to write them off as noise reports a book half its real length.
func TestFromProgressUsesPartialPushesToFindTheUnit(t *testing.T) {
	built := series{
		pages:     700,
		syncPages: 2,
		pushes:    300,
		partials:  0.05,
		jumps:     0.0,
		random:    rand.New(rand.NewSource(3)), // #nosec G404 -- fixture data
	}.build()

	estimate, ok := pages.FromProgress(built)
	if !ok {
		t.Fatalf("no estimate")
	}
	if estimate.Pages < 690 || estimate.Pages > 710 {
		t.Errorf("estimated %d pages, want about 700", estimate.Pages)
	}
}

func TestFromProgressNeedsEnoughData(t *testing.T) {
	cases := []struct {
		name     string
		progress []float64
	}{
		{name: "empty", progress: nil},
		{name: "one push", progress: []float64{0.5}},
		{name: "read before syncing", progress: []float64{0.9962, 0.9962, 1.0}},
		{name: "all backwards", progress: []float64{0.9, 0.8, 0.7, 0.6, 0.5, 0.4, 0.3, 0.2, 0.1}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if estimate, ok := pages.FromProgress(testCase.progress); ok {
				t.Errorf("estimated %d pages from %d values", estimate.Pages, len(testCase.progress))
			}
		})
	}
}

// Jumping around a book must not move the estimate: those deltas are not
// reading and are dropped as outliers.
func TestFromProgressIgnoresJumps(t *testing.T) {
	clean := series{
		pages: 700, syncPages: 2, pushes: 300, partials: 0.09, jumps: 0,
		random: rand.New(rand.NewSource(4)), // #nosec G404 -- fixture data
	}.build()
	jumpy := series{
		pages: 700, syncPages: 2, pushes: 300, partials: 0.09, jumps: 0.06,
		random: rand.New(rand.NewSource(4)), // #nosec G404 -- fixture data
	}.build()

	first, ok := pages.FromProgress(clean)
	if !ok {
		t.Fatalf("no estimate without jumps")
	}
	second, ok := pages.FromProgress(jumpy)
	if !ok {
		t.Fatalf("no estimate with jumps")
	}

	if difference := first.Pages - second.Pages; difference < -2 || difference > 2 {
		t.Errorf("jumps moved the estimate from %d to %d pages", first.Pages, second.Pages)
	}
}
