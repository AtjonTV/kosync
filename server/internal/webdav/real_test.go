//
// File:        internal/webdav/real_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package webdav_test

import (
	"bytes"
	"net/http"
	"os"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/testutil"
	"git.obth.eu/atjontv/kosync/internal/webdav"
	"github.com/pocketbase/pocketbase/tests"
)

// realStatisticsEnv names a statistics database written by a real KOReader.
//
// A reading history is personal data and is not ours to keep in a repository,
// so this skips unless one is supplied. It is the only test that proves the
// gate agrees with KOReader rather than with the database the other tests build
// to their own idea of the schema — which is exactly the way a validator comes
// to accept only its own imagination.
//
//	KOSYNC_REAL_STATISTICS_DB=/path/to/statistics.sqlite3 go test ./internal/webdav/
const realStatisticsEnv = "KOSYNC_REAL_STATISTICS_DB"

func TestARealStatisticsDatabaseIsAccepted(t *testing.T) {
	path := os.Getenv(realStatisticsEnv)
	if path == "" {
		t.Skipf("set %s to a KOReader statistics database to run this", realStatisticsEnv)
	}

	if err := webdav.Validate(path); err != nil {
		t.Fatalf("a real statistics database was refused: %v", err)
	}

	content, err := os.ReadFile(path) // #nosec G304 -- supplied by the operator
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	scenario := tests.ApiScenario{
		Name:            "a real statistics database goes all the way through",
		Method:          http.MethodPut,
		URL:             syncURL,
		Body:            bytes.NewReader(content),
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(),
		ExpectedStatus:  http.StatusCreated,
		ExpectedContent: []string{"Created"},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			kept, ok := stored(t, app, testutil.IdUserA)
			if !ok {
				t.Fatal("expected the upload to be kept")
			}
			if !bytes.Equal(kept, content) {
				t.Error("the stored file is not byte for byte what was uploaded")
			}
		},
	}
	scenario.Test(t)
}
