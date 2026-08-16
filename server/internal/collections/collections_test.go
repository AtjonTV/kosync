//
// File:        internal/collections/collections_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package collections_test

import (
	"net/http"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/collections"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// Fixture ids, readable so that a failing assertion names what it tripped over.
var (
	idBookOfA       = testutil.PadId("booka")
	idBookOfOwnA    = testutil.PadId("bookaa")
	idBookOfB       = testutil.PadId("bookb")
	idCollectionOfA = testutil.PadId("colla")
)

// aLibraryWithTwoOwners gives each account a book and account A a shelf holding
// its own, which is the only arrangement in which the check has anything to say.
func aLibraryWithTwoOwners(t testing.TB) *tests.TestApp {
	t.Helper()

	app := testutil.NewApp(t)
	collections.Register(app)

	fixture := testutil.Seed(t, app)
	bookOfA := testutil.CreateBook(t, app, fixture.UserA, idBookOfA, "Ambush", "hash-a", "")
	testutil.CreateBook(t, app, fixture.UserA, idBookOfOwnA, "Betrayal", "hash-aa", "")
	testutil.CreateBook(t, app, fixture.UserB, idBookOfB, "Bob's Own Book", "hash-b", "")
	testutil.CreateBookCollection(t, app, fixture.UserA, idCollectionOfA, "To read", bookOfA)

	return app
}

// asUserA runs a scenario against that world, signed in as the account that owns
// the shelf.
func asUserA(t *testing.T, scenario tests.ApiScenario) {
	t.Helper()

	headers := map[string]string{}
	scenario.Headers = headers
	scenario.TestAppFactory = aLibraryWithTwoOwners
	scenario.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
		user, err := app.FindRecordById(schema.CollectionUsers, testutil.IdUserA)
		if err != nil {
			t.Fatalf("failed to load the fixture user: %v", err)
		}
		headers["Authorization"] = testutil.UserToken(t, user)
	}

	scenario.Test(t)
}

const collectionsUrl = "/api/collections/" + schema.CollectionBookCollections + "/records"

func TestAShelfMayHoldTheOwnersOwnBooks(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:   "a shelf of one's own books",
		Method: http.MethodPost,
		URL:    collectionsUrl,
		Body: strings.NewReader(`{"owner":"` + testutil.IdUserA + `",` +
			`"name":"Favourites","books":["` + idBookOfA + `"]}`),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"name":"Favourites"`, `"` + idBookOfA + `"`},
		ExpectedEvents: map[string]int{
			"*":                          0,
			"OnRecordCreateRequest":      1,
			"OnRecordCreate":             1,
			"OnRecordCreateExecute":      1,
			"OnRecordAfterCreateSuccess": 1,
			"OnModelCreate":              1,
			"OnModelCreateExecute":       1,
			"OnModelAfterCreateSuccess":  1,
			"OnModelValidate":            1,
			"OnRecordValidate":           1,
			"OnRecordEnrich":             1,
		},
	})
}

// The check exists for this: a shelf is a list of ids, and an id is somebody
// else's book as easily as it is one's own. Reading the titles back through the
// relation would then be a way of asking what another account uploaded.
func TestAShelfMayNotHoldSomebodyElsesBooks(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:   "a shelf holding a foreign book",
		Method: http.MethodPost,
		URL:    collectionsUrl,
		Body: strings.NewReader(`{"owner":"` + testutil.IdUserA + `",` +
			`"name":"Stolen","books":["` + idBookOfB + `"]}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"your own library"},
		ExpectedEvents:  map[string]int{"*": 0, "OnRecordCreateRequest": 1},
	})

	asUserA(t, tests.ApiScenario{
		Name:            "a foreign book added to an existing shelf",
		Method:          http.MethodPatch,
		URL:             collectionsUrl + "/" + idCollectionOfA,
		Body:            strings.NewReader(`{"books":["` + idBookOfA + `","` + idBookOfB + `"]}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"your own library"},
		ExpectedEvents:  map[string]int{"*": 0, "OnRecordUpdateRequest": 1},
	})
}

// A book that no longer exists is refused the same way, which is the case that
// arrives on its own: two browser tabs, one of them deleting.
func TestAShelfMayNotHoldABookThatIsGone(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:            "a shelf holding a book nobody has",
		Method:          http.MethodPatch,
		URL:             collectionsUrl + "/" + idCollectionOfA,
		Body:            strings.NewReader(`{"books":["` + testutil.PadId("gone") + `"]}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"your own library"},
		ExpectedEvents:  map[string]int{"*": 0, "OnRecordUpdateRequest": 1},
	})
}

func TestAShelfCannotBeGivenAway(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:            "handing a shelf to another account",
		Method:          http.MethodPatch,
		URL:             collectionsUrl + "/" + idCollectionOfA,
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserB + `"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"cannot change owner"},
		ExpectedEvents:  map[string]int{"*": 0, "OnRecordUpdateRequest": 1},
	})
}

// The guard is about the owner changing, not about the word appearing in the
// payload: a client that sends the record back whole — as a form usually does —
// sends its owner along with everything else and has changed nothing.
func TestSendingTheOwnerBackUnchangedIsFine(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:            "a rename that repeats the owner",
		Method:          http.MethodPatch,
		URL:             collectionsUrl + "/" + idCollectionOfA,
		Body:            strings.NewReader(`{"owner":"` + testutil.IdUserA + `","name":"Read next"}`),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"name":"Read next"`},
		ExpectedEvents: map[string]int{
			"*":                          0,
			"OnRecordUpdateRequest":      1,
			"OnRecordUpdate":             1,
			"OnRecordUpdateExecute":      1,
			"OnRecordAfterUpdateSuccess": 1,
			"OnModelUpdate":              1,
			"OnModelUpdateExecute":       1,
			"OnModelAfterUpdateSuccess":  1,
			"OnModelValidate":            1,
			"OnRecordValidate":           1,
			"OnRecordEnrich":             1,
		},
	})
}

// Emptying a shelf is not deleting it: somebody who clears out a reading list
// still has the reading list.
func TestEmptyingAShelfKeepsIt(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:            "the last book taken off",
		Method:          http.MethodPatch,
		URL:             collectionsUrl + "/" + idCollectionOfA,
		Body:            strings.NewReader(`{"books":[]}`),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"name":"To read"`, `"books":[]`},
		ExpectedEvents: map[string]int{
			"*":                          0,
			"OnRecordUpdateRequest":      1,
			"OnRecordUpdate":             1,
			"OnRecordUpdateExecute":      1,
			"OnRecordAfterUpdateSuccess": 1,
			"OnModelUpdate":              1,
			"OnModelUpdateExecute":       1,
			"OnModelAfterUpdateSuccess":  1,
			"OnModelValidate":            1,
			"OnRecordValidate":           1,
			"OnRecordEnrich":             1,
		},
	})
}

// Deleting a book takes it off every shelf it stood on, and leaves the shelf
// where it was. That is PocketBase's own doing rather than this package's, which
// is exactly why it is worth a test: the behaviour is a field option away from
// deleting the shelf along with its last book.
func TestDeletingABookTakesItOffTheShelf(t *testing.T) {
	app := aLibraryWithTwoOwners(t)

	book, err := app.FindRecordById(schema.CollectionBooks, idBookOfA)
	if err != nil {
		t.Fatalf("failed to load the book: %v", err)
	}
	if err := app.Delete(book); err != nil {
		t.Fatalf("failed to delete the book: %v", err)
	}

	shelf, err := app.FindRecordById(schema.CollectionBookCollections, idCollectionOfA)
	if err != nil {
		t.Fatalf("expected the collection to outlive its last book: %v", err)
	}
	if got := shelf.GetStringSlice(schema.FieldBooks); len(got) != 0 {
		t.Errorf("expected the deleted book to be off the shelf, got %v", got)
	}
}

// The browser adds and removes one book at a time with PocketBase's own list
// modifiers, so that two open tabs cannot overwrite each other's shelf. The
// check has to see the merged list rather than what was sent, which is what
// these two say.
func TestABookIsAddedAndTakenOffOneAtATime(t *testing.T) {
	asUserA(t, tests.ApiScenario{
		Name:            "another of one's own books appended",
		Method:          http.MethodPatch,
		URL:             collectionsUrl + "/" + idCollectionOfA,
		Body:            strings.NewReader(`{"books+":"` + idBookOfOwnA + `"}`),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{idBookOfA, idBookOfOwnA},
		ExpectedEvents: map[string]int{
			"*":                          0,
			"OnRecordUpdateRequest":      1,
			"OnRecordUpdate":             1,
			"OnRecordUpdateExecute":      1,
			"OnRecordAfterUpdateSuccess": 1,
			"OnModelUpdate":              1,
			"OnModelUpdateExecute":       1,
			"OnModelAfterUpdateSuccess":  1,
			"OnModelValidate":            1,
			"OnRecordValidate":           1,
			"OnRecordEnrich":             1,
		},
	})

	asUserA(t, tests.ApiScenario{
		Name:            "a foreign book appended",
		Method:          http.MethodPatch,
		URL:             collectionsUrl + "/" + idCollectionOfA,
		Body:            strings.NewReader(`{"books+":"` + idBookOfB + `"}`),
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"your own library"},
		ExpectedEvents:  map[string]int{"*": 0, "OnRecordUpdateRequest": 1},
	})
}
