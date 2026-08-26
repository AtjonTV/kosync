//
// File:        internal/ownership/ownership_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package ownership_test

import (
	"net/http"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/ownership"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

var idBookOfA = testutil.PadId("booka")

// twoAccounts is the seeded two-user world with the guard registered over it,
// plus a book for account A so that all three frozen collections have something
// of A's to try to give away.
func twoAccounts(t testing.TB) *tests.TestApp {
	t.Helper()

	app := testutil.NewApp(t)
	ownership.Freeze(app,
		schema.CollectionKoreaderAccounts,
		schema.CollectionDocuments,
		schema.CollectionBooks,
	)

	fixture := testutil.Seed(t, app)
	testutil.CreateBook(t, app, fixture.UserA, idBookOfA, "Ambush", "hash-a", "")

	return app
}

// asUserA runs a scenario signed in as the account that owns everything above.
func asUserA(t *testing.T, scenario tests.ApiScenario) {
	t.Helper()

	headers := map[string]string{}
	scenario.Headers = headers
	scenario.TestAppFactory = twoAccounts
	scenario.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
		user, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserA)
		if err != nil {
			t.Fatalf("failed to load the fixture user: %v", err)
		}
		headers["Authorization"] = testutil.UserToken(t, user)
	}

	scenario.Test(t)
}

func recordUrl(collection, id string) string {
	return "/api/collections/" + collection + "/records/" + id
}

// The one that matters: a credential handed to another account keeps working,
// because the password did not change with it, and every device facing route
// would then answer about the new owner's reading.
func TestAKoreaderCredentialCannotBeGivenAway(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:            "handing a device credential to another account",
		Method:          http.MethodPatch,
		URL:             recordUrl(schema.CollectionKoreaderAccounts, testutil.IdAccountA),
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserB + `"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"cannot change owner"},
		ExpectedEvents:  map[string]int{"*": 0, "OnRecordUpdateRequest": 1},
	})
}

func TestADocumentCannotBeGivenAway(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:            "handing a document to another account",
		Method:          http.MethodPatch,
		URL:             recordUrl(schema.CollectionDocuments, testutil.IdDocumentA),
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserB + `"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"cannot change owner"},
		ExpectedEvents:  map[string]int{"*": 0, "OnRecordUpdateRequest": 1},
	})
}

func TestABookCannotBeGivenAway(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:            "handing a book to another account",
		Method:          http.MethodPatch,
		URL:             recordUrl(schema.CollectionBooks, idBookOfA),
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserB + `"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"cannot change owner"},
		ExpectedEvents:  map[string]int{"*": 0, "OnRecordUpdateRequest": 1},
	})
}

// The guard is about the owner changing, not about the word appearing in the
// payload: a client that sends the record back whole sends its owner along with
// everything else and has changed nothing.
func TestSendingTheOwnerBackUnchangedIsFine(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:            "a rename that repeats the owner",
		Method:          http.MethodPatch,
		URL:             recordUrl(schema.CollectionDocuments, testutil.IdDocumentA),
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserA + `","title":"Still mine"}`),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title":"Still mine"`, `"owner":"` + testutil.IdUserA + `"`},
	})
}

// A record of somebody else's is refused by the owner rule before the guard is
// ever reached, and must stay a 404 rather than becoming a 400 that confirms the
// record exists.
func TestAForeignRecordIsStillNotFound(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:            "taking another account's document",
		Method:          http.MethodPatch,
		URL:             recordUrl(schema.CollectionDocuments, testutil.IdDocumentB),
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserA + `"}`),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
	})
}
