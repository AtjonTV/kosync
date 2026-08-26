//
// File:        internal/pages/estimate.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package pages estimates how many pages a book has on the device that read
// it, from nothing but the progress values that device pushed.
//
// An EPUB is reflowable, so it has no page count of its own, and KOReader's
// count moves with font size and screen. But KOReader syncs every N pages, so
// the progress deltas between consecutive pushes are near-integer multiples of
// one page. Recovering that unit gives the page count directly, and it is a
// measurement rather than a guess: on real data it reproduced two device-
// reported page counts exactly.
package pages

import (
	"math"
	"sort"
)

const (
	// MinSamples is the fewest deltas worth estimating from.
	MinSamples = 12

	// maxSyncPages caps how many pages one push may represent. KOReader's own
	// setting does not go anywhere near this.
	maxSyncPages = 16

	// outlierFactor drops deltas larger than this multiple of the median. Those
	// are jumps — skipping ahead, following a link — not reading.
	outlierFactor = 4.0

	// ReportingResolution is the precision KOReader sends progress at: four
	// decimals. Every one of the 1803 values in the reference data sits exactly
	// on this grid.
	ReportingResolution = 0.0001

	// tolerance is how far a delta may sit from a whole number of pages, as a
	// fraction of one page.
	//
	// Rounding onto the reporting grid moves a delta by at most half a grid
	// step, which for a normal book is a few percent of a page; the reference
	// data sits within 4% of whole pages. The value has to stay tight, because
	// a loose one lets a candidate page size of 4/5 of the truth match
	// everything: five-page steps land on it exactly, and only the occasional
	// single-page push contradicts it.
	tolerance = 0.15

	// requiredFit is the share of deltas that must land on a whole number of
	// pages before a candidate page size is accepted.
	//
	// It has to be strict. Single-page pushes at chapter ends were about 9% of
	// deltas in the reference data, and a threshold loose enough to ignore them
	// accepts a page size of exactly twice the truth.
	requiredFit = 0.98

	// minGridRatio is how many steps of the protocol's own resolution one page
	// must span before the page size is believable.
	//
	// KOReader reports progress to four decimals, so a page is only visible if
	// it is comfortably wider than 0.0001. Without this floor the estimator
	// happily "finds" a page size equal to the reporting grid: a 3562-page
	// omnibus in the reference data came out as exactly 10000 pages, stable
	// across every chunk of the series, because that is 1/0.0001.
	//
	// Six steps leaves margin over the rounding error, and puts the ceiling at
	// roughly 1600 pages — above that a book cannot be measured and has to
	// fall back.
	minGridRatio = 6
)

// Estimate is a measured page size for one book on one device.
type Estimate struct {
	// PageFraction is the share of the book one page covers.
	PageFraction float64

	// Pages is the book's length on that device.
	Pages int

	// SyncPages is how many pages one push represents — KOReader's "sync every
	// N pages" setting, recovered rather than configured.
	SyncPages int

	// Samples is how many deltas the estimate is based on.
	Samples int
}

// FromProgress estimates the page size from progress values ordered by time.
// It reports false when the data cannot support an estimate, which is the
// normal case for a book that was read before syncing was set up.
func FromProgress(progress []float64) (Estimate, bool) {
	deltas := forwardDeltas(progress)
	if len(deltas) < MinSamples {
		return Estimate{}, false
	}

	sort.Float64s(deltas)
	median := medianOf(deltas)
	if median <= 0 {
		return Estimate{}, false
	}

	kept := make([]float64, 0, len(deltas))
	for _, delta := range deltas {
		if delta <= median*outlierFactor {
			kept = append(kept, delta)
		}
	}
	if len(kept) < MinSamples {
		return Estimate{}, false
	}

	// The median is a whole number of pages; which number is what the loop
	// finds. Trying the largest candidate page size first means the answer is
	// the coarsest unit that still explains the data.
	// A page narrower than this cannot be told apart from the reporting grid
	// itself, so anything below it is refused rather than guessed at.
	median = medianOf(kept)
	floor := float64(minGridRatio) * ReportingResolution

	for sync := 1; sync <= maxSyncPages; sync++ {
		fraction := median / float64(sync)
		if fraction <= 0 || fraction < floor {
			// Every later candidate is smaller still, so nothing below the
			// reporting resolution is worth trying.
			break
		}
		if fit(kept, fraction) < requiredFit {
			continue
		}

		// The median is one noisy sample of a two-page step. Now that the
		// candidate has told us how many pages each delta spans, the page size
		// follows from all of them at once: total progress over total pages.
		// Skipping this leaves the estimate about 1.5% short.
		fraction = refine(kept, fraction)

		pages := int(math.Round(1 / fraction))
		if pages <= 0 {
			break
		}

		return Estimate{
			PageFraction: fraction,
			Pages:        pages,
			SyncPages:    sync,
			Samples:      len(kept),
		}, true
	}

	return Estimate{}, false
}

// forwardDeltas returns the positive differences between consecutive values.
// Backwards moves are re-reads, not reading, and contribute nothing.
func forwardDeltas(progress []float64) []float64 {
	deltas := make([]float64, 0, len(progress))
	for index := 1; index < len(progress); index++ {
		if delta := progress[index] - progress[index-1]; delta > 0 {
			deltas = append(deltas, delta)
		}
	}

	return deltas
}

// fit is the share of deltas that sit within tolerance of a whole number of
// pages of the given size.
func fit(sorted []float64, fraction float64) float64 {
	matched := 0
	for _, delta := range sorted {
		steps := math.Round(delta / fraction)
		if steps < 1 {
			continue
		}
		if math.Abs(delta-steps*fraction) <= tolerance*fraction {
			matched++
		}
	}

	return float64(matched) / float64(len(sorted))
}

// refine re-derives the page size from every delta that fits: the progress
// they cover, divided by the number of pages they span.
func refine(deltas []float64, fraction float64) float64 {
	var progress, spanned float64

	for _, delta := range deltas {
		steps := math.Round(delta / fraction)
		if steps < 1 {
			continue
		}
		if math.Abs(delta-steps*fraction) > tolerance*fraction {
			continue
		}
		progress += delta
		spanned += steps
	}

	if spanned == 0 {
		return fraction
	}

	return progress / spanned
}

// medianOf returns the median of an already sorted slice.
func medianOf(sorted []float64) float64 {
	count := len(sorted)
	if count == 0 {
		return 0
	}
	if count%2 == 1 {
		return sorted[count/2]
	}

	return (sorted[count/2-1] + sorted[count/2]) / 2
}
