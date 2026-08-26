//
// File:        internal/opds/opds_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/devices"
	"git.obth.eu/atjontv/kosync/internal/koreader"
	"git.obth.eu/atjontv/kosync/internal/opds"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// basicAuth returns the header a catalog client sends.
func basicAuth(username, password string) map[string]string {
	return map[string]string{
		"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password)),
	}
}

// seeder fills a freshly built app with the books a scenario needs.
type seeder func(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture)

// newFactory builds a fresh app per scenario.
//
// Fresh per scenario rather than shared: PocketBase registers its routes when a
// scenario starts, so two scenarios sharing one app collide on the first route.
func newFactory(pageSize int, seed seeder) func(testing.TB) *tests.TestApp {
	return func(t testing.TB) *tests.TestApp {
		app := testutil.NewApp(t)

		conf := &config.Config{KoreaderAuthCacheTtl: 300, EnableOpds: true}
		conf.Normalize()
		conf.OpdsPageSize = pageSize

		sync := koreader.Register(app, conf)
		books.Register(app, conf)
		devices.Register(app)
		opds.Register(app, conf, sync)

		fixture := testutil.Seed(t, app)
		if seed != nil {
			seed(t, app, fixture)
		}

		return app
	}
}

// addBook stores a book of the given owner, with a cover when one is asked for.
// An empty id means the record gets one of its own.
func addBook(t testing.TB, app core.App, owner *core.Record, id, title string, authors []string, withCover bool) *core.Record {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("failed to find the %q collection: %v", schema.CollectionBooks, err)
	}

	file, err := filesystem.NewFileFromBytes([]byte("PK\x03\x04 stand-in for "+title), "book.epub")
	if err != nil {
		t.Fatalf("failed to build a stand-in file: %v", err)
	}

	record := core.NewRecord(collection)
	if id != "" {
		record.Id = id
	}
	record.Set(schema.FieldOwner, owner.Id)
	record.Set(schema.FieldFile, file)
	record.Set(schema.FieldTitle, title)
	record.Set(schema.FieldContentHash, title)
	record.Set(schema.FieldPageCount, 320)

	if len(authors) > 0 {
		record.Set(schema.FieldAuthors, authors)
	}
	if withCover {
		record.Set(schema.FieldCover, coverFile(t))
	}

	if err := app.Save(record); err != nil {
		t.Fatalf("failed to store the book %q: %v", title, err)
	}

	return record
}

// coverFile is a real image, because the thumbnail route generates one from it.
func coverFile(t testing.TB) *filesystem.File {
	t.Helper()

	source := image.NewRGBA(image.Rect(0, 0, 400, 600))
	for x := range 400 {
		for y := range 600 {
			source.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}

	buffer := &bytes.Buffer{}
	if err := png.Encode(buffer, source); err != nil {
		t.Fatalf("failed to encode a cover: %v", err)
	}

	file, err := filesystem.NewFileFromBytes(buffer.Bytes(), "cover.png")
	if err != nil {
		t.Fatalf("failed to build a cover file: %v", err)
	}

	return file
}

// twoLibraries gives each fixture user a book, which is what proves that the
// catalog shows one account its own.
func twoLibraries(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture) {
	addBook(t, app, fixture.UserA, "", "Zeit des Sturms", []string{"Andrzej Sapkowski"}, true)
	addBook(t, app, fixture.UserB, "", "Bob's Own Book", []string{"Bob"}, false)
}

func TestTheCatalogRequiresACredential(t *testing.T) {
	scenarios := []tests.ApiScenario{
		{
			Name:           "no credentials at all",
			Method:         http.MethodGet,
			URL:            "/opds",
			ExpectedStatus: http.StatusUnauthorized,
			// A bare 401 leaves a reader guessing, so the body says how to
			// authenticate and what to call the two fields.
			ExpectedContent: []string{
				`"type":"http://opds-spec.org/auth/basic"`,
				`"login":"KOReader username"`,
			},
		},
		{
			Name:            "the wrong password",
			Method:          http.MethodGet,
			URL:             "/opds",
			Headers:         basicAuth(testutil.KoUsernameA, "not-the-password"),
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"authentication"`},
		},
		{
			Name:            "an unknown account",
			Method:          http.MethodGet,
			URL:             "/opds",
			Headers:         basicAuth("nobody", testutil.KoPasswordA),
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"authentication"`},
		},
		{
			Name:            "the digest is not the password",
			Method:          http.MethodGet,
			URL:             "/opds",
			Headers:         basicAuth(testutil.KoUsernameA, testutil.Md5Hex(testutil.KoPasswordA)),
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"authentication"`},
		},
		{
			Name:            "the credential the device syncs with",
			Method:          http.MethodGet,
			URL:             "/opds",
			Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"title":"KOsync library"`},
		},
		{
			// An address typed into a device by hand arrives with a trailing
			// slash about as often as not.
			Name:            "the address with a trailing slash",
			Method:          http.MethodGet,
			URL:             "/opds/",
			Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"title":"KOsync library"`},
		},
		{
			Name:            "a path below the catalog that is nothing",
			Method:          http.MethodGet,
			URL:             "/opds/books/deeper/still/nonsense",
			Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
			ExpectedStatus:  http.StatusNotFound,
			ExpectedContent: []string{`"status":404`},
		},
	}

	for _, scenario := range scenarios {
		scenario.TestAppFactory = newFactory(50, nil)
		scenario.Test(t)
	}
}

func TestTheRootFeedOffersTheShelvesAndASearch(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "the entry point",
		Method:         http.MethodGet,
		URL:            "/opds",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, twoLibraries),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"navigation"`,
			`"title":"Currently reading"`,
			`"title":"Recently added"`,
			`"title":"All books"`,
			// OPDS 2.0 has no search description document: the template is the
			// whole of it, so it has to be marked as one.
			`"rel":"search"`,
			`"templated":true`,
			`/opds/search{?query}`,
		},
		NotExpectedContent: []string{`"publications"`},
	}

	scenario.Test(t)
}

func TestAShelfShowsOnlyTheAccountsOwnBooks(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "one account's library",
		Method:         http.MethodGet,
		URL:            "/opds/books",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, twoLibraries),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"title":"Zeit des Sturms"`,
			`"name":"Andrzej Sapkowski"`,
			`"@type":"http://schema.org/Book"`,
			`"rel":"http://opds-spec.org/acquisition/open-access"`,
			`"type":"application/epub+zip"`,
			`"numberOfPages":320`,
			`"numberOfItems":1`,
		},
		NotExpectedContent: []string{"Bob's Own Book"},
	}

	scenario.Test(t)
}

func TestAnUnknownShelfIsNotFound(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:            "a shelf that does not exist",
		Method:          http.MethodGet,
		URL:             "/opds/nonsense",
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(50, twoLibraries),
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"status":404`},
	}

	scenario.Test(t)
}

// The shelf is what makes this catalog worth having over a file share: the
// server knows which books are in the middle of being read, and that is exactly
// the list somebody setting up a second device wants.
func TestCurrentlyReadingHoldsTheStartedAndUnfinishedBooks(t *testing.T) {
	seed := func(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture) {
		started := addBook(t, app, fixture.UserA, "", "Started", nil, false)
		finished := addBook(t, app, fixture.UserA, "", "Finished", nil, false)
		addBook(t, app, fixture.UserA, "", "Untouched", nil, false)

		read(t, app, fixture.UserA, started, "hash-started", 0.4)
		read(t, app, fixture.UserA, finished, "hash-finished", 1)
	}

	scenario := tests.ApiScenario{
		Name:               "the books in progress",
		Method:             http.MethodGet,
		URL:                "/opds/reading",
		Headers:            basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:     newFactory(50, seed),
		ExpectedStatus:     http.StatusOK,
		ExpectedContent:    []string{`"title":"Started"`, `"numberOfItems":1`},
		NotExpectedContent: []string{`"title":"Finished"`, `"title":"Untouched"`},
	}

	scenario.Test(t)
}

// Two documents can point at one book, when one device identified the file by
// name and another by content. The shelf has to show the book once.
func TestCurrentlyReadingCountsABookReadOnTwoDevicesOnce(t *testing.T) {
	seed := func(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture) {
		book := addBook(t, app, fixture.UserA, "", "Read Twice", nil, false)
		read(t, app, fixture.UserA, book, "hash-binary", 0.4)
		read(t, app, fixture.UserA, book, "hash-filename", 0.6)
	}

	scenario := tests.ApiScenario{
		Name:            "one book, two documents",
		Method:          http.MethodGet,
		URL:             "/opds/reading",
		Headers:         basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:  newFactory(50, seed),
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"numberOfItems":1`},
	}

	scenario.Test(t)
}

// read records that a device pushed progress through a book.
func read(t testing.TB, app core.App, owner, book *core.Record, hash string, progress float64) {
	t.Helper()

	document := testutil.CreateDocument(t, app, owner, "", hash, progress, time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC))
	document.Set(schema.FieldBook, book.Id)
	if err := app.Save(document); err != nil {
		t.Fatalf("failed to link a document to a book: %v", err)
	}
}

func TestAShelfIsPaginated(t *testing.T) {
	seed := func(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture) {
		for _, title := range []string{"Alpha", "Bravo", "Delta", "Echo", "Foxtrot"} {
			addBook(t, app, fixture.UserA, "", title, nil, false)
		}
	}

	scenarios := []tests.ApiScenario{
		{
			Name:   "the first page",
			Method: http.MethodGet,
			URL:    "/opds/books",
			ExpectedContent: []string{
				`"title":"Alpha"`, `"title":"Bravo"`,
				`"numberOfItems":5`, `"itemsPerPage":2`, `"currentPage":1`,
				`"rel":"next"`, `/opds/books?page=2`, `/opds/books?page=3`,
			},
			// There is nowhere before the first page, and saying so lets a
			// client stop without counting.
			NotExpectedContent: []string{`"rel":"previous"`, `"title":"Delta"`},
		},
		{
			Name:   "a middle page",
			Method: http.MethodGet,
			URL:    "/opds/books?page=2",
			ExpectedContent: []string{
				`"title":"Delta"`, `"title":"Echo"`,
				`"currentPage":2`, `"rel":"next"`, `"rel":"previous"`,
			},
			NotExpectedContent: []string{`"title":"Alpha"`},
		},
		{
			Name:               "the last page",
			Method:             http.MethodGet,
			URL:                "/opds/books?page=3",
			ExpectedContent:    []string{`"title":"Foxtrot"`, `"rel":"previous"`},
			NotExpectedContent: []string{`"rel":"next"`},
		},
		{
			Name:   "past the end",
			Method: http.MethodGet,
			URL:    "/opds/books?page=9",
			// An empty page rather than an error: a client that overshoots gets
			// a feed it can render and links back to somewhere real.
			ExpectedContent:    []string{`"currentPage":9`, `"rel":"first"`},
			NotExpectedContent: []string{`"publications"`},
		},
		{
			Name:            "a page number that is not one",
			Method:          http.MethodGet,
			URL:             "/opds/books?page=nonsense",
			ExpectedContent: []string{`"currentPage":1`, `"title":"Alpha"`},
		},
	}

	for _, scenario := range scenarios {
		scenario.Headers = basicAuth(testutil.KoUsernameA, testutil.KoPasswordA)
		scenario.TestAppFactory = newFactory(2, seed)
		scenario.ExpectedStatus = http.StatusOK
		scenario.Test(t)
	}
}

func TestSearchLooksAtTitlesAndAuthors(t *testing.T) {
	seed := func(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture) {
		addBook(t, app, fixture.UserA, "", "Zeit des Sturms", []string{"Andrzej Sapkowski"}, false)
		addBook(t, app, fixture.UserA, "", "Nineteen Eighty-Four", []string{"George Orwell"}, false)
		addBook(t, app, fixture.UserB, "", "Sapkowski For Bob", []string{"Andrzej Sapkowski"}, false)
		// A pair only an escaped search can tell apart. Underscores in a title
		// are not contrived: they are what a download leaves behind when the
		// site it came from could not put spaces in a file name.
		addBook(t, app, fixture.UserA, "", "Vol_1 Beginnings", []string{"Anon"}, false)
		addBook(t, app, fixture.UserA, "", "Vol 1 Endings", []string{"Anon"}, false)
	}

	scenarios := []tests.ApiScenario{
		{
			Name:               "by title",
			URL:                "/opds/search?query=sturms",
			ExpectedContent:    []string{`"title":"Zeit des Sturms"`, `"numberOfItems":1`},
			NotExpectedContent: []string{"Orwell"},
		},
		{
			Name:               "by author",
			URL:                "/opds/search?query=orwell",
			ExpectedContent:    []string{`"title":"Nineteen Eighty-Four"`},
			NotExpectedContent: []string{"Sturms"},
		},
		{
			Name:               "still only this account's books",
			URL:                "/opds/search?query=sapkowski",
			ExpectedContent:    []string{`"title":"Zeit des Sturms"`, `"numberOfItems":1`},
			NotExpectedContent: []string{"Sapkowski For Bob"},
		},
		{
			// The wildcard is escaped, so it searches for the character rather
			// than matching everything.
			Name:               "a percent sign is a character, not a wildcard",
			URL:                "/opds/search?query=%25",
			ExpectedContent:    []string{`"numberOfItems":0`},
			NotExpectedContent: []string{`"publications"`},
		},
		{
			// The other wildcard, and the one a reader types by accident: an
			// underscore stands for any single character unless it is escaped,
			// which would quietly turn a search for a file name into a search
			// for everything shaped like it.
			Name:               "an underscore is a character, not a wildcard",
			URL:                "/opds/search?query=vol_1",
			ExpectedContent:    []string{`"title":"Vol_1 Beginnings"`, `"numberOfItems":1`},
			NotExpectedContent: []string{"Vol 1 Endings"},
		},
		{
			Name:            "nothing matches",
			URL:             "/opds/search?query=zzzz",
			ExpectedContent: []string{`"numberOfItems":0`, `"title":"Search: zzzz"`},
		},
	}

	for _, scenario := range scenarios {
		scenario.Method = http.MethodGet
		scenario.Headers = basicAuth(testutil.KoUsernameA, testutil.KoPasswordA)
		scenario.TestAppFactory = newFactory(50, seed)
		scenario.ExpectedStatus = http.StatusOK
		scenario.Test(t)
	}
}

func TestAnEmptySearchGoesBackToTheCatalog(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "a template followed without filling it in",
		Method:         http.MethodGet,
		URL:            "/opds/search?query=",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, nil),
		ExpectedStatus: http.StatusFound,
	}

	scenario.Test(t)
}

// KOReader greys out its "book information" button unless the publication
// carries a description, and most EPUBs carry none of their own — not one of
// the reference books does. So the catalog writes what this server does know.
func TestEveryPublicationCarriesADescription(t *testing.T) {
	seed := func(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture) {
		started := addBook(t, app, fixture.UserA, "", "Started", nil, false)
		addBook(t, app, fixture.UserA, "", "Untouched", nil, false)

		read(t, app, fixture.UserA, started, "hash-started", 0.63)
	}

	scenario := tests.ApiScenario{
		Name:           "book information",
		Method:         http.MethodGet,
		URL:            "/opds/books",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, seed),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`63% read, last opened on 1 March 2026.`,
			`Not started on any of your devices yet.`,
			// The page count says which kind it is, because a measurement and a
			// guess from the word count are worth telling apart.
			`320 pages, estimated from the word count.`,
		},
	}

	scenario.Test(t)
}

// A device identifier is not a name, and the description is prose.
func TestTheDescriptionNamesTheDevice(t *testing.T) {
	seed := func(t testing.TB, app *tests.TestApp, fixture *testutil.Fixture) {
		book := addBook(t, app, fixture.UserA, "", "Started", nil, false)
		read(t, app, fixture.UserA, book, "hash-started", 0.63)

		document, err := app.FindFirstRecordByData(schema.CollectionDocuments, schema.FieldDocument, "hash-started")
		if err != nil {
			t.Fatalf("failed to load the document: %v", err)
		}
		document.Set(schema.FieldLastDevice, "go7")
		document.Set(schema.FieldLastDeviceId, "865F46C0C0F4401D9A05768B6B0BF3AC")
		if err := app.Save(document); err != nil {
			t.Fatalf("failed to record the device: %v", err)
		}

		device, err := devices.Find(app, fixture.UserA.Id, "865F46C0C0F4401D9A05768B6B0BF3AC")
		if err != nil || device == nil {
			t.Fatalf("expected the device to be registered: %v", err)
		}
		device.Set(schema.FieldName, "Boox Go 7")
		if err := app.Save(device); err != nil {
			t.Fatalf("failed to rename the device: %v", err)
		}
	}

	scenario := tests.ApiScenario{
		Name:               "the chosen device name",
		Method:             http.MethodGet,
		URL:                "/opds/books",
		Headers:            basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:     newFactory(50, seed),
		ExpectedStatus:     http.StatusOK,
		ExpectedContent:    []string{`last opened on Boox Go 7 on 1 March 2026.`},
		NotExpectedContent: []string{"865F46C0C0F4401D9A05768B6B0BF3AC"},
	}

	scenario.Test(t)
}

// A reader labels the download button with the link's title when there is one,
// so a title here replaces the format with a word that says nothing.
func TestTheAcquisitionIsNotGivenATitle(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:               "the download link",
		Method:             http.MethodGet,
		URL:                "/opds/books",
		Headers:            basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory:     newFactory(50, twoLibraries),
		ExpectedStatus:     http.StatusOK,
		ExpectedContent:    []string{`"rel":"http://opds-spec.org/acquisition/open-access"`},
		NotExpectedContent: []string{`"title":"Download"`},
	}

	scenario.Test(t)
}

// Without these relations a reader cannot tell the two covers apart and falls
// back to the first, which is the large one it did not want.
func TestTheTwoCoverSizesAreDistinguishable(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:           "cover relations",
		Method:         http.MethodGet,
		URL:            "/opds/books",
		Headers:        basicAuth(testutil.KoUsernameA, testutil.KoPasswordA),
		TestAppFactory: newFactory(50, twoLibraries),
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"rel":"http://opds-spec.org/image"`,
			`"rel":"http://opds-spec.org/image/thumbnail"`,
			`"width":200`,
		},
	}

	scenario.Test(t)
}
