//
// File:        internal/opds/facets_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// describedBook stores a book carrying the fields the navigation feeds group by.
//
// Two saves rather than one because the description hooks run on the request
// events and not on a plain save, so nothing here overwrites what the test says
// the book is.
func describedBook(
	t testing.TB,
	app core.App,
	owner *core.Record,
	title string,
	authors []string,
	language string,
	series string,
	index float64,
) *core.Record {
	t.Helper()

	record := addBook(t, app, owner, "", title, authors, false)
	record.Set(schema.FieldLanguage, language)
	record.Set(schema.FieldSeries, series)
	record.Set(schema.FieldSeriesIndex, index)

	if err := app.Save(record); err != nil {
		t.Fatalf("failed to describe the book %q: %v", title, err)
	}

	return record
}

// aBrowsableLibrary is a library big enough to be worth breaking up: two series,
// one of them with an unnumbered volume, two languages under five spellings, and
// an author who appears only as a translator on one book.
//
// The volumes are titled so that reading order and alphabetical order disagree,
// which is the only way an ordering test proves anything.
func aBrowsableLibrary(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture) {
	describedBook(t, app, fixture.UserA, "Ambush", []string{"Lee Child"}, "en", "Jack Reacher", 3)
	describedBook(t, app, fixture.UserA, "Betrayal", []string{"Lee Child"}, "en", "Jack Reacher", 1)
	describedBook(t, app, fixture.UserA, "Choice", []string{"Lee Child"}, "en", "Jack Reacher", 2)

	describedBook(t, app, fixture.UserA, "Zeit des Sturms", []string{"Andrzej Sapkowski"}, "de-DE", "", 0)
	describedBook(t, app, fixture.UserA, "Etwas endet", []string{"Andrzej Sapkowski"}, "de", "", 0)
	describedBook(t, app, fixture.UserA, "Der Schwalbenturm", []string{"Andrzej Sapkowski"}, "DE", "", 0)

	describedBook(t, app, fixture.UserA, "Der letzte Wunsch",
		[]string{"Andrzej Sapkowski", "Erik Simon"}, "de", "Die Hexer-Saga", 1)
	describedBook(t, app, fixture.UserA, "Der Weg, von dem niemand zurückkehrt",
		[]string{"Andrzej Sapkowski"}, "de", "Die Hexer-Saga", 0)

	// A book with nobody's name on it and a language tag that is the file
	// refusing to say. One of the reference library's 192 is exactly this, and
	// the authors of it are not an empty array but no value at all, which is
	// what the guard around json_each is for.
	describedBook(t, app, fixture.UserA, "Anonymus", nil, "und", "", 0)

	describedBook(t, app, fixture.UserB, "Bob's Own Book", []string{"Bob"}, "en", "Bob's Trilogy", 1)
}

// inOrder fails unless the given pieces appear in the body in the given order,
// which is what an ordering claim needs and a set of substring checks cannot say.
func inOrder(pieces ...string) func(t testing.TB, app *tests.TestApp, res *http.Response) {
	return func(t testing.TB, app *tests.TestApp, res *http.Response) {
		t.Helper()

		raw, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("failed to read the feed: %v", err)
		}

		body := string(raw)
		at := 0
		for _, piece := range pieces {
			index := strings.Index(body[at:], piece)
			if index < 0 {
				t.Fatalf("expected %q after the piece before it, in\n%s", piece, body)
			}
			at += index + len(piece)
		}
	}
}

func TestTheRootFeedOffersTheNavigationFeeds(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "the entry point of a library worth browsing",
		Method:         http.MethodGet,
		URL:            "/opds",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, aBrowsableLibrary),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"title":"By author"`,
			`"title":"By series"`,
			`"title":"By language"`,
			`/opds/authors`,
			`/opds/series`,
			`/opds/languages`,
		},
		// The shelves come first: the navigation feeds are for finding a book,
		// and the shelves are for carrying on with one.
		AfterTestFunc: inOrder(`"title":"All books"`, `"title":"By author"`, `"title":"By language"`),
	}

	scenario.Test(t)
}

// A library with no series in it should not be offered a series shelf, and
// finding that out should not cost a reader a round trip.
func TestAFacetWithNothingInItIsNotOffered(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "a library of undescribed books",
		Method:         http.MethodGet,
		URL:            "/opds",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, twoLibraries),
		ExpectedStatus: http.StatusOK,
		// The books do carry authors, so that one facet stays.
		ExpectedContent:    []string{`"title":"By author"`},
		NotExpectedContent: []string{`"title":"By series"`, `"title":"By language"`},
	}

	scenario.Test(t)
}

func TestTheAuthorFeedNamesEveryAuthorWithACount(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "every author in the library",
		Method:         http.MethodGet,
		URL:            "/opds/authors",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, aBrowsableLibrary),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"navigation"`,
			`"title":"Andrzej Sapkowski (5)"`,
			`"title":"Lee Child (3)"`,
			// A book with two names on it appears under both of them.
			`"title":"Erik Simon (1)"`,
			`"numberOfItems":3`,
			`facet=authors`,
		},
		// A navigation feed lists names, not books, and it lists nobody else's.
		NotExpectedContent: []string{`"publications"`, `Bob`},
		AfterTestFunc: inOrder(
			`"title":"Andrzej Sapkowski (5)"`,
			`"title":"Erik Simon (1)"`,
			`"title":"Lee Child (3)"`,
		),
	}

	scenario.Test(t)
}

func TestTheSeriesFeedListsOnlyTheSeries(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "the series in the library",
		Method:         http.MethodGet,
		URL:            "/opds/series",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, aBrowsableLibrary),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"title":"Die Hexer-Saga (2)"`,
			`"title":"Jack Reacher (3)"`,
			`"numberOfItems":2`,
		},
		// The three standalone books belong to no series and so belong nowhere
		// in this feed, and neither does the other account's series.
		NotExpectedContent: []string{`Zeit des Sturms`, `Bob's Trilogy`},
	}

	scenario.Test(t)
}

// A series read alphabetically is a series read in the wrong order, which is the
// whole reason the shelf exists.
func TestASeriesIsShelvedInReadingOrder(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "one series, in order",
		Method:         http.MethodGet,
		URL:            "/opds/by?facet=series&value=Jack+Reacher",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, aBrowsableLibrary),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"title":"Jack Reacher"`,
			`"publications"`,
			`"numberOfItems":3`,
		},
		NotExpectedContent: []string{`Zeit des Sturms`},
		AfterTestFunc:      inOrder(`"title":"Betrayal"`, `"title":"Choice"`, `"title":"Ambush"`),
	}

	scenario.Test(t)
}

// A volume the publisher gave no number sorts to the front rather than being
// scattered through the numbered ones.
func TestAnUnnumberedVolumeComesFirst(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "a series with an unnumbered volume",
		Method:          http.MethodGet,
		URL:             "/opds/by?facet=series&value=Die+Hexer-Saga",
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(50, aBrowsableLibrary),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"numberOfItems":2`},
		AfterTestFunc: inOrder(
			`"title":"Der Weg, von dem niemand zurückkehrt"`,
			`"title":"Der letzte Wunsch"`,
		),
	}

	scenario.Test(t)
}

// The reference library spells one language four ways. Shelving the spellings
// apart would reproduce the splitting this feature exists to undo.
func TestTheLanguagesAreFoldedAndNamed(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "every language in the library",
		Method:         http.MethodGet,
		URL:            "/opds/languages",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, aBrowsableLibrary),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"title":"German (5)"`,
			`"title":"English (3)"`,
			// "und" is not a language, it is the file declining to name one.
			`"title":"Unknown (1)"`,
			`"numberOfItems":3`,
		},
		// "de", "de-DE" and "DE" are one shelf, not three, and the tag is not
		// what a reader is shown.
		NotExpectedContent: []string{`"title":"de`, `"title":"DE`, `de-DE (`, `UND`},
		// The most common language first: there are usually two or three of
		// them, and the one somebody wants is the one most of their books is in.
		AfterTestFunc: inOrder(`"title":"German (5)"`, `"title":"English (3)"`, `"title":"Unknown (1)"`),
	}

	scenario.Test(t)
}

func TestALanguageShelfHoldsEverySpellingOfIt(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "the German books",
		Method:         http.MethodGet,
		URL:            "/opds/by?facet=languages&value=de",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, aBrowsableLibrary),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			// The feed is titled with the name, not the tag that was asked for.
			`"title":"German"`,
			`"title":"Zeit des Sturms"`,
			`"title":"Der Schwalbenturm"`,
			`"title":"Etwas endet"`,
			`"numberOfItems":5`,
		},
		NotExpectedContent: []string{`"title":"Ambush"`},
	}

	scenario.Test(t)
}

func TestAnAuthorShelfHoldsWhatTheyWrote(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "one author's books",
		Method:         http.MethodGet,
		URL:            "/opds/by?facet=authors&value=Lee+Child",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, aBrowsableLibrary),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"title":"Lee Child"`,
			`"title":"Ambush"`,
			`"title":"Betrayal"`,
			`"title":"Choice"`,
			`"numberOfItems":3`,
		},
		NotExpectedContent: []string{`"title":"Etwas endet"`},
	}

	scenario.Test(t)
}

// Everything below the catalog is scoped to the account that asked, and a
// navigation feed is no exception: it would otherwise say how many books other
// people own without ever handing one over.
func TestANavigationFeedShowsOnlyTheAccountsOwn(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name:            "the other account's authors",
			Method:          http.MethodGet,
			URL:             "/opds/authors",
			Headers:         basicAuth(testutil.KoUsernameB, testutil.KoPasswordB),
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"title":"Bob (1)"`, `"numberOfItems":1`},
			NotExpectedContent: []string{
				`Lee Child`,
				`Andrzej Sapkowski`,
			},
		},
		{
			Name:               "the other account's series",
			Method:             http.MethodGet,
			URL:                "/opds/series",
			Headers:            basicAuth(testutil.KoUsernameB, testutil.KoPasswordB),
			ExpectedStatus:     http.StatusOK,
			ExpectedContent:    []string{`"title":"Bob's Trilogy (1)"`},
			NotExpectedContent: []string{`Jack Reacher`, `Die Hexer-Saga`},
		},
	}

	for _, scenario := range scenarios {
		scenario.TestAppFactory = newFactory(50, aBrowsableLibrary)
		scenario.Test(t)
	}
}

func TestANavigationFeedIsPaginated(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name:           "the first page of the authors",
			URL:            "/opds/authors",
			ExpectedStatus: http.StatusOK,
			ExpectedContent: []string{
				`"title":"Andrzej Sapkowski (5)"`,
				`"title":"Erik Simon (1)"`,
				`"numberOfItems":3`,
				`"itemsPerPage":2`,
				`"currentPage":1`,
				`"rel":"next"`,
				`/opds/authors?page=2`,
			},
			NotExpectedContent: []string{`Lee Child`, `"rel":"previous"`},
		},
		{
			Name:           "the last page of the authors",
			URL:            "/opds/authors?page=2",
			ExpectedStatus: http.StatusOK,
			ExpectedContent: []string{
				`"title":"Lee Child (3)"`,
				`"currentPage":2`,
				`"rel":"previous"`,
			},
			NotExpectedContent: []string{`Andrzej Sapkowski`, `"rel":"next"`},
		},
		{
			Name:           "the second page of a series",
			URL:            "/opds/by?facet=series&value=Jack+Reacher&page=2",
			ExpectedStatus: http.StatusOK,
			ExpectedContent: []string{
				`"title":"Ambush"`,
				`"numberOfItems":3`,
				`"currentPage":2`,
				`"rel":"previous"`,
			},
			NotExpectedContent: []string{`"title":"Betrayal"`},
		},
	}

	for _, scenario := range scenarios {
		scenario.Method = http.MethodGet
		scenario.Headers = basicAuth(testutil.KoUsernameA, testutil.KoPasswordA)
		scenario.TestAppFactory = newFactory(2, aBrowsableLibrary)
		scenario.Test(t)
	}
}

func TestAGroupThatLeadsNowhereIsNotFound(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name: "a facet that does not exist",
			URL:  "/opds/by?facet=subjects&value=Fantasy",
		},
		{
			Name: "a facet feed that does not exist",
			URL:  "/opds/subjects",
		},
		{
			Name: "no value at all",
			URL:  "/opds/by?facet=series",
		},
		{
			Name: "an empty value",
			URL:  "/opds/by?facet=authors&value=",
		},
		{
			// A link somebody bookmarked before renaming the series.
			Name: "a value nothing is under",
			URL:  "/opds/by?facet=series&value=Gone",
		},
		{
			// The other account's series is not a 403 either, because saying
			// which of the two it is would answer the question.
			Name: "another account's value",
			URL:  "/opds/by?facet=series&value=Bob%27s+Trilogy",
		},
	}

	for _, scenario := range scenarios {
		scenario.Method = http.MethodGet
		scenario.Headers = basicAuth(testutil.KoUsernameA, testutil.KoPasswordA)
		scenario.TestAppFactory = newFactory(50, aBrowsableLibrary)
		scenario.ExpectedStatus = http.StatusNotFound
		scenario.ExpectedContent = []string{`"status":404`}
		scenario.Test(t)
	}
}
