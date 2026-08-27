//
// File:        internal/kosyncapi/preview_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

const previewContainer = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

const previewPackage = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Zeit des Sturms</dc:title></metadata>
  <manifest>
    <item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/>
    <item id="two" href="text/two.xhtml" media-type="application/xhtml+xml"/>
    <item id="part" href="text/part.xhtml" media-type="application/xhtml+xml"/>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
  </manifest>
  <spine><itemref idref="one"/><itemref idref="two"/></spine>
</package>`

// previewEPUB is a two chapter book whose first chapter carries everything the
// preview has to take away before it is shown.
func previewEPUB(t testing.TB) []byte {
	t.Helper()

	entries := []struct{ name, content string }{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", previewContainer},
		{"OEBPS/content.opf", previewPackage},
		{"OEBPS/text/one.xhtml", `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml">` +
			`<head><title>Der Anfang</title></head><body><p onclick="steal()">Ein Sturm zieht auf.</p>` +
			`<script>steal()</script></body></html>`},
		{"OEBPS/text/two.xhtml", `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml">` +
			`<head><title>Die Fortsetzung</title></head><body><p>Und weiter.</p></body></html>`},
		{"OEBPS/text/part.xhtml", `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml">` +
			`<head><title>Erster Teil</title></head><body><h1>ERSTER TEIL</h1></body></html>`},
		// The book's own contents put the second chapter in a part and say nothing
		// about the first, which is how a book treats its front matter. The part's
		// own title page is in the manifest but not in the spine, so it is a name
		// the list can group under and not a chapter to page through.
		{"OEBPS/nav.xhtml", `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body>` +
			`<nav xmlns:epub="http://www.idpf.org/2007/ops" epub:type="toc"><ol>` +
			`<li><a href="text/part.xhtml">ERSTER TEIL</a><ol>` +
			`<li><a href="text/two.xhtml">Die Fortsetzung</a></li></ol></li>` +
			`</ol></nav></body></html>`},
	}

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, item := range entries {
		writer, err := archive.Create(item.name)
		if err != nil {
			t.Fatalf("create %s: %v", item.name, err)
		}
		if _, err := writer.Write([]byte(item.content)); err != nil {
			t.Fatalf("write %s: %v", item.name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	return buffer.Bytes()
}

// storeEPUB writes a book with the given file, without going through upload.
func storeEPUB(t testing.TB, app core.App, id, owner string, content []byte) {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("find books collection: %v", err)
	}

	file, err := filesystem.NewFileFromBytes(content, "book.epub")
	if err != nil {
		t.Fatalf("build file: %v", err)
	}

	record := core.NewRecord(collection)
	record.Id = id
	record.Set(schema.FieldOwner, owner)
	record.Set(schema.FieldFile, file)
	record.Set(schema.FieldTitle, "Zeit des Sturms")
	record.Set(schema.FieldContentHash, id)

	if err := app.Save(record); err != nil {
		t.Fatalf("save book: %v", err)
	}
}

// previewApp mounts the API over a library holding one book of each account.
func previewApp(t testing.TB) *tests.TestApp {
	t.Helper()

	app := newApp(t)
	storeEPUB(t, app, testutil.PadId("booka"), testutil.IdUserA, previewEPUB(t))
	storeEPUB(t, app, testutil.PadId("bookb"), testutil.IdUserB, previewEPUB(t))

	return app
}

func TestPreviewListsTheChapters(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:           "the shape of a book the account owns",
		Method:         http.MethodGet,
		URL:            "/api/kosync/books/" + testutil.PadId("booka") + "/preview",
		TestAppFactory: previewApp,
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"title":"Zeit des Sturms"`,
			// The first chapter is not in the book's contents and is named by
			// itself; the second is, and comes back under the part it belongs to.
			`{"index":0,"title":"Der Anfang"}`,
			`{"index":1,"title":"Die Fortsetzung","section":"ERSTER TEIL"}`,
		},
	})
}

// A chapter carries the part it belongs to as well, because the reader shows it
// in the header and gets there without asking for the outline again.
func TestAChapterSaysWhichPartOfTheBookItIsIn(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "the second chapter, under its part",
		Method:          http.MethodGet,
		URL:             "/api/kosync/books/" + testutil.PadId("booka") + "/preview/1",
		TestAppFactory:  previewApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"section":"ERSTER TEIL"`},
	})
}

// A book with no parts says nothing rather than saying nothing at length: the
// first chapter is outside the contents entirely, and the field is left out.
func TestAChapterOutsideThePartsSaysSoByOmission(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:               "the first chapter, in no part",
		Method:             http.MethodGet,
		URL:                "/api/kosync/books/" + testutil.PadId("booka") + "/preview/0",
		TestAppFactory:     previewApp,
		ExpectedStatus:     http.StatusOK,
		ExpectedContent:    []string{`"title":"Der Anfang"`},
		NotExpectedContent: []string{`"section"`},
	})
}

// The endpoint is the whole security boundary of the preview's markup on the
// server side; the frame around it is the other one.
func TestPreviewReturnsAChapterWithoutItsScript(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "one chapter, rebuilt",
		Method:          http.MethodGet,
		URL:             "/api/kosync/books/" + testutil.PadId("booka") + "/preview/0",
		TestAppFactory:  previewApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"index":0`, `"title":"Der Anfang"`, `\u003cp\u003eEin Sturm zieht auf.\u003c/p\u003e`, `"truncated":false`},
		NotExpectedContent: []string{
			"script", "steal", "onclick",
		},
	})
}

// A preview must never become reading. Nothing it does writes, and the way to
// keep that true is to notice when it stops being true.
func TestPreviewRecordsNothing(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "looking inside a book is not reading it",
		Method:          http.MethodGet,
		URL:             "/api/kosync/books/" + testutil.PadId("booka") + "/preview/1",
		TestAppFactory:  previewApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title":"Die Fortsetzung"`},
		ExpectedEvents:  map[string]int{"*": 0},
	})
}

func TestPreviewOfSomebodyElsesBookIsNotFound(t *testing.T) {
	for name, url := range map[string]string{
		"the outline": "/api/kosync/books/" + testutil.PadId("bookb") + "/preview",
		"a chapter":   "/api/kosync/books/" + testutil.PadId("bookb") + "/preview/0",
	} {
		asUser(t, testutil.IdUserA, tests.ApiScenario{
			Name:            name + " of another account's book",
			Method:          http.MethodGet,
			URL:             url,
			TestAppFactory:  previewApp,
			ExpectedStatus:  http.StatusNotFound,
			ExpectedContent: []string{`"status":404`},
		})
	}
}

func TestPreviewOfABookThatDoesNotExistIsNotFound(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "an id nobody has",
		Method:          http.MethodGet,
		URL:             "/api/kosync/books/" + testutil.PadId("nope") + "/preview",
		TestAppFactory:  previewApp,
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"status":404`},
	})
}

func TestPreviewOfAChapterPastTheEndIsNotFound(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a chapter the book does not have",
		Method:          http.MethodGet,
		URL:             "/api/kosync/books/" + testutil.PadId("booka") + "/preview/9",
		TestAppFactory:  previewApp,
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"status":404`},
	})
}

func TestPreviewOfSomethingThatIsNotAChapterNumberIsRefused(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a chapter that is not a number",
		Method:          http.MethodGet,
		URL:             "/api/kosync/books/" + testutil.PadId("booka") + "/preview/erstes",
		TestAppFactory:  previewApp,
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{`"status":400`},
	})
}

// A file that is not a readable EPUB is a bad book, not a broken server.
func TestPreviewOfAnUnreadableFileIsAnAnswer(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:   "a stored file that is not an EPUB",
		Method: http.MethodGet,
		URL:    "/api/kosync/books/" + testutil.PadId("bookc") + "/preview",
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			app := previewApp(t)

			var buffer bytes.Buffer
			archive := zip.NewWriter(&buffer)
			writer, err := archive.Create("notes.txt")
			if err != nil {
				t.Fatalf("create entry: %v", err)
			}
			if _, err := writer.Write([]byte("no book in here")); err != nil {
				t.Fatalf("write entry: %v", err)
			}
			if err := archive.Close(); err != nil {
				t.Fatalf("close archive: %v", err)
			}
			storeEPUB(t, app, testutil.PadId("bookc"), testutil.IdUserA, buffer.Bytes())

			return app
		},
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{`"status":400`},
	})
}

func TestPreviewNeedsAnAccount(t *testing.T) {
	asUser(t, "", tests.ApiScenario{
		Name:            "a signed out request is refused",
		Method:          http.MethodGet,
		URL:             "/api/kosync/books/" + testutil.PadId("booka") + "/preview",
		TestAppFactory:  previewApp,
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{`"status":401`},
	})
}

// Paging back to the chapter before is the commonest thing a reader does, and
// the tag is what makes it cost nothing.
func TestAChapterIsTaggedSoPagingBackIsFree(t *testing.T) {
	tag := ""

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "the first read carries a tag",
		Method:          http.MethodGet,
		URL:             "/api/kosync/books/" + testutil.PadId("booka") + "/preview/0",
		TestAppFactory:  previewApp,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`\u003cp\u003eEin Sturm zieht auf.\u003c/p\u003e`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, response *http.Response) {
			tag = response.Header.Get("ETag")
			if tag == "" {
				t.Fatal("the chapter carries no ETag")
			}
			if cache := response.Header.Get("Cache-Control"); cache != "private, max-age=300" {
				t.Errorf("the chapter is cached as %q", cache)
			}
		},
	})

	if tag == "" {
		t.Fatal("no tag to ask with")
	}

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:           "the second read is answered with nothing",
		Method:         http.MethodGet,
		URL:            "/api/kosync/books/" + testutil.PadId("booka") + "/preview/0",
		Headers:        map[string]string{"If-None-Match": tag},
		TestAppFactory: previewApp,
		ExpectedStatus: http.StatusNotModified,
	})
}
