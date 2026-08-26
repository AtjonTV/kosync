//
// File:        internal/statistics/real_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package statistics_test

import (
	"os"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/statistics"
)

// realStatisticsEnv names a statistics database written by a real KOReader.
//
// A reading history is personal data and is not ours to keep in a repository,
// so this skips unless one is supplied. The databases the other tests build are
// this package's own idea of the schema; this is the only test that proves the
// import agrees with KOReader.
//
//	KOSYNC_REAL_STATISTICS_DB=/path/to/statistics.sqlite3 go test ./internal/statistics/ -v
const realStatisticsEnv = "KOSYNC_REAL_STATISTICS_DB"

func TestARealStatisticsDatabaseImports(t *testing.T) {
	path := os.Getenv(realStatisticsEnv)
	if path == "" {
		t.Skipf("set %s to a KOReader statistics database to run this", realStatisticsEnv)
	}

	app, user := newApp(t)

	result, err := statistics.Import(app, user.Id, path)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Rows == 0 {
		t.Fatal("the database held no page turns at all")
	}
	if result.Added != result.Rows {
		t.Errorf("added %d of %d rows into an empty account", result.Added, result.Rows)
	}
	if len(result.Dates) == 0 {
		t.Error("no days were queued for a database with reading in it")
	}

	t.Logf("%d page turns over %d days", result.Added, len(result.Dates))

	// The page counts are the other half of what the file is read for, and a
	// real one is the only place the pagination genuinely changes mid-book.
	if len(result.Pages) == 0 {
		t.Error("no page counts were stated for a database with reading in it")
	}
	for _, count := range result.Pages {
		if count.Pages <= 0 || count.Turns <= 0 || count.Through <= 0 {
			t.Errorf("document %s states %+v", count.Document, count)
		}
		t.Logf("%s runs to %d pages, from %d of its recent turns", count.Document, count.Pages, count.Turns)
	}

	// The second sync of a device that has not read since adds nothing at all.
	again, err := statistics.Import(app, user.Id, path)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if again.Added != 0 {
		t.Errorf("a second import of the same database added %d rows", again.Added)
	}
}
