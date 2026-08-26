//
// File:        internal/opds/files_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds_test

import (
	"net/http"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/tests"
)

// Ids the file scenarios address the books by.
var (
	idBookA   = testutil.PadId("booka")
	idBookB   = testutil.PadId("bookb")
	idNoCover = testutil.PadId("bookc")
)

// The title carries a space, an umlaut and a slash, which is every part of the
// derived name that could go wrong at once: the slash would open a second path
// segment, and the umlaut has to survive both the URL and the header.
const awkwardTitle = "Zeit des Sturms / Über allem"

// threeBooks gives the first user a book with a cover and one without, and the
// second user one of their own.
func threeBooks(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture) {
	addBook(t, app, fixture.UserA, idBookA, awkwardTitle, []string{"Andrzej Sapkowski"}, true)
	addBook(t, app, fixture.UserA, idNoCover, "No Cover", nil, false)
	addBook(t, app, fixture.UserB, idBookB, "Bob's Own Book", nil, true)
}

func TestAcquisitionStreamsTheBook(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "downloading a book",
		Method:          http.MethodGet,
		URL:             "/opds/books/" + idBookA + "/download/anything.epub",
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(50, threeBooks),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{"stand-in for " + awkwardTitle},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			if got := res.Header.Get("Content-Type"); got != "application/epub+zip" {
				t.Errorf("expected an EPUB content type, got %q", got)
			}

			// The name in the header is the derived one, not the name in the
			// URL and not the mangled name the file is stored under. Both forms
			// are sent so that a client reading either lands on the same name.
			disposition := res.Header.Get("Content-Disposition")
			for _, want := range []string{
				// The quoted form is ASCII only, so the umlaut is replaced
				// rather than transliterated; the encoded form beside it is the
				// one that carries the real name.
				`filename="Zeit des Sturms _ber allem.epub"`,
				`filename*=UTF-8''Zeit%20des%20Sturms%20%C3%9Cber%20allem.epub`,
			} {
				if !strings.Contains(disposition, want) {
					t.Errorf("expected %q in the disposition %q", want, disposition)
				}
			}
		},
	}

	scenario.Test(t)
}

// The name is decoration on the way in — the id finds the book — so a device
// that kept a link from before a rename still gets its book.
func TestAcquisitionIgnoresAStaleNameInTheUrl(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "a name that no longer matches",
		Method:          http.MethodGet,
		URL:             "/opds/books/" + idBookA + "/download/The%20Old%20Name.epub",
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(50, threeBooks),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{"stand-in for " + awkwardTitle},
	}

	scenario.Test(t)
}

// Somebody else's book answers exactly as one that does not exist, so the
// catalog cannot be used to find out what other people own.
func TestAnotherAccountsBookIsNotFound(t *testing.T) {
	for _, path := range []string{"/download/x.epub", "/cover", "/thumbnail"} {
		scenario := tests.ApiScenario{
			Name:            "reaching for " + path,
			Method:          http.MethodGet,
			URL:             "/opds/books/" + idBookB + path,
			Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
			TestAppFactory:  newFactory(50, threeBooks),
			ExpectedStatus:  http.StatusNotFound,
			ExpectedContent: []string{`"status":404`},
		}

		scenario.Test(t)
	}
}

func TestCoversAreServedInTwoSizes(t *testing.T) {
	scenarios := []coverScenario{
		{
			Name:   "the full cover",
			URL:    "/opds/books/" + idBookA + "/cover",
			status: http.StatusOK,
		},
		{
			// PocketBase generates thumbnails behind its own file endpoint,
			// which needs a token no reader knows how to fetch, so the catalog
			// generates this one on the way past.
			Name:   "the thumbnail",
			URL:    "/opds/books/" + idBookA + "/thumbnail",
			status: http.StatusOK,
		},
		{
			Name:   "a book with no cover at all",
			URL:    "/opds/books/" + idNoCover + "/cover",
			status: http.StatusNotFound,
		},
		{
			Name:   "a thumbnail of nothing",
			URL:    "/opds/books/" + idNoCover + "/thumbnail",
			status: http.StatusNotFound,
		},
	}

	for _, scenario := range scenarios {
		run(t, scenario)
	}
}

// coverScenario is an ApiScenario with the parts every cover case shares left
// out, because four copies of the same six fields hide the one line that differs.
type coverScenario struct {
	Name   string
	URL    string
	status int
}

func run(t *testing.T, from coverScenario) {
	t.Helper()

	scenario := tests.ApiScenario{
		Name:           from.Name,
		Method:         http.MethodGet,
		URL:            from.URL,
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, threeBooks),
		ExpectedStatus: from.status,
	}

	if from.status == http.StatusOK {
		// An image is not text, so there is no keyword to look for; the check is
		// that something arrived and that it is an image.
		scenario.AfterTestFunc = func(t testing.TB, app *tests.TestApp, res *http.Response) {
			if got := res.Header.Get("Content-Type"); !strings.HasPrefix(got, "image/") {
				t.Errorf("expected an image, got %q", got)
			}
		}
		scenario.ExpectedContent = []string{"PNG"}
	} else {
		scenario.ExpectedContent = []string{`"status":404`}
	}

	scenario.Test(t)
}
