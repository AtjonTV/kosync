//
// File:        internal/pages/real_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package pages_test

import (
	"encoding/csv"
	"os"
	"sort"
	"strconv"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/pages"
)

// realSeriesEnv names a CSV of real progress pushes to check the estimator
// against: label, expected page count (0 if unknown), timestamp, progress.
//
// Real reading data cannot be committed — it is personal, and the books it
// describes are not ours to ship — so this test skips unless the file is
// supplied. It exists because the estimator's whole claim is that it recovers
// a device's real page count, and synthetic data cannot test that claim.
//
//	KOSYNC_REAL_PROGRESS_CSV=/path/to/progress.csv go test ./internal/pages/
const realSeriesEnv = "KOSYNC_REAL_PROGRESS_CSV"

func TestEstimateAgainstRealProgress(t *testing.T) {
	path := os.Getenv(realSeriesEnv)
	if path == "" {
		t.Skipf("set %s to check against real reading data", realSeriesEnv)
	}

	type sample struct {
		moment   int64
		progress float64
	}
	series := map[string][]sample{}
	expected := map[string]int{}

	file, err := os.Open(path) // #nosec G304 -- test-only path from the developer's environment
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	for _, row := range rows {
		if len(row) < 4 {
			continue
		}
		label := row[0]
		want, _ := strconv.Atoi(row[1])
		moment, _ := strconv.ParseInt(row[2], 10, 64)
		progress, err := strconv.ParseFloat(row[3], 64)
		if err != nil {
			continue
		}
		expected[label] = want
		series[label] = append(series[label], sample{moment: moment, progress: progress})
	}

	if len(series) == 0 {
		t.Fatalf("%s contained no usable rows", path)
	}

	labels := make([]string, 0, len(series))
	for label := range series {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	for _, label := range labels {
		samples := series[label]
		sort.Slice(samples, func(a, b int) bool { return samples[a].moment < samples[b].moment })

		progress := make([]float64, len(samples))
		for index, entry := range samples {
			progress[index] = entry.progress
		}

		want := expected[label]

		estimate, ok := pages.FromProgress(progress)
		if !ok {
			// Refusing is a valid answer — a book longer than the reporting
			// resolution can resolve cannot be measured at all.
			if want > 0 {
				t.Errorf("%s: no estimate from %d pushes, device reports %d pages",
					label, len(progress), want)
			} else {
				t.Logf("%-28s declined, from %d pushes", label, len(progress))
			}

			continue
		}

		t.Logf("%-28s %5d pages, sync every %d, from %d deltas",
			label, estimate.Pages, estimate.SyncPages, estimate.Samples)

		if want == 0 {
			continue
		}
		if estimate.Pages != want {
			t.Errorf("%s: estimated %d pages, device reports %d", label, estimate.Pages, want)
		}
	}
}
