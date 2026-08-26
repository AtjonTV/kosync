//
// File:        internal/opds/collections_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds_test

import (
	"net/http"
	"net/url"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/tests"
)

// Fixture ids of the shelves, so that a link can be built without reading one
// out of a feed first.
var (
	idWinterReading = testutil.PadId("winter")
	idOneDay        = testutil.PadId("someday")
	idBobsShelf     = testutil.PadId("bobsss")
)

// aCuratedLibrary is a library with shelves somebody built.
//
// "Winter reading" is deliberately out of alphabetical order with the books on
// it, and out of reading order too, because the order a shelf is in is the one
// thing about it that no query could work out.
func aCuratedLibrary(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture) {
	ambush := addBook(t, app, fixture.UserA, "", "Ambush", []string{"Lee Child"}, false)
	betrayal := addBook(t, app, fixture.UserA, "", "Betrayal", []string{"Lee Child"}, false)
	choice := addBook(t, app, fixture.UserA, "", "Choice", []string{"Child, Lee"}, false)
	addBook(t, app, fixture.UserA, "", "Unshelved", []string{"Nobody"}, false)

	testutil.CreateBookCollection(t, app, fixture.UserA, idWinterReading,
		"Winter reading", choice, ambush, betrayal)

	// A shelf with nothing on it yet, which is a plan and not a place.
	testutil.CreateBookCollection(t, app, fixture.UserA, idOneDay, "One day")

	bobs := addBook(t, app, fixture.UserB, "", "Bob's Own Book", []string{"Bob"}, false)
	testutil.CreateBookCollection(t, app, fixture.UserB, idBobsShelf, "Bob's shelf", bobs)
}

// shelfUrl is the address of one shelf's books.
func shelfUrl(id string) string {
	return "/opds/by?" + url.Values{"facet": {"collections"}, "value": {id}}.Encode()
}

func TestTheRootFeedOffersTheCollectionsFirst(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "an account with a shelf of its own",
		Method:          http.MethodGet,
		URL:             "/opds",
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(50, aCuratedLibrary),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title":"Collections"`, `/opds/collections`},
		// Before the three feeds that are only the library described back to
		// itself: what somebody chose comes first.
		ExpectedEvents: map[string]int{"*": 0},
		AfterTestFunc:  inOrder(`"title":"Collections"`, `"title":"By author"`),
	}
	scenario.Test(t)
}

// A library nobody has curated says nothing about collections at all, rather
// than offering a feed that turns out to be empty a page turn later.
func TestTheRootFeedOffersNoCollectionsWhenThereAreNone(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:               "an account that has built nothing",
		Method:             http.MethodGet,
		URL:                "/opds",
		Headers:            basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:     newFactory(50, aBrowsableLibrary),
		ExpectedStatus:     http.StatusOK,
		ExpectedContent:    []string{`"title":"By author"`},
		NotExpectedContent: []string{`"title":"Collections"`, `/opds/collections`},
		ExpectedEvents:     map[string]int{"*": 0},
	}
	scenario.Test(t)
}

func TestTheCollectionsFeedListsTheShelvesWithTheirCounts(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "the shelves of one account",
		Method:          http.MethodGet,
		URL:             "/opds/collections",
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(50, aCuratedLibrary),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title":"Winter reading (3)"`, idWinterReading},
		NotExpectedContent: []string{
			// The empty shelf, and the other account's.
			"One day",
			"Bob's shelf",
			idBobsShelf,
		},
		ExpectedEvents: map[string]int{"*": 0},
	}
	scenario.Test(t)
}

// The reason a shelf is worth having: it is in the order its owner put it in,
// which is neither alphabetical nor anything else a query could reconstruct.
func TestAShelfIsServedInTheOrderItWasBuilt(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:               "one shelf, in its own order",
		Method:             http.MethodGet,
		URL:                shelfUrl(idWinterReading),
		Headers:            basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:     newFactory(50, aCuratedLibrary),
		ExpectedStatus:     http.StatusOK,
		ExpectedContent:    []string{`"title":"Winter reading"`},
		NotExpectedContent: []string{`"title":"Unshelved"`},
		ExpectedEvents:     map[string]int{"*": 0},
		AfterTestFunc:      inOrder(`"title":"Choice"`, `"title":"Ambush"`, `"title":"Betrayal"`),
	}
	scenario.Test(t)
}

// A shelf that is paged through keeps that order across the page boundary,
// which is the case a sort in the database would quietly get wrong.
func TestAShelfKeepsItsOrderAcrossPages(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:               "the second page of a shelf of three",
		Method:             http.MethodGet,
		URL:                shelfUrl(idWinterReading) + "&page=2",
		Headers:            basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:     newFactory(2, aCuratedLibrary),
		ExpectedStatus:     http.StatusOK,
		ExpectedContent:    []string{`"title":"Betrayal"`},
		NotExpectedContent: []string{`"title":"Choice"`, `"title":"Ambush"`},
		ExpectedEvents:     map[string]int{"*": 0},
	}
	scenario.Test(t)
}

// Somebody else's shelf answers the same as one that was never made, so the
// catalog cannot be used to find out what other people have put together.
func TestAForeignShelfIsNotThere(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "another account's shelf",
		Method:          http.MethodGet,
		URL:             shelfUrl(idBobsShelf),
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(50, aCuratedLibrary),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
		ExpectedEvents:  map[string]int{"*": 0},
	}
	scenario.Test(t)
}

func TestAnEmptyShelfIsNotThere(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "a shelf with nothing on it",
		Method:          http.MethodGet,
		URL:             shelfUrl(idOneDay),
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(50, aCuratedLibrary),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"data":{}`},
		ExpectedEvents:  map[string]int{"*": 0},
	}
	scenario.Test(t)
}
