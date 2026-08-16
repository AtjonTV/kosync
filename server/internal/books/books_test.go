//
// File:        internal/books/books_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

const booksURL = "/api/collections/books/records"

// newApp returns a seeded app with the upload processing mounted.
//
// Each scenario gets its own app: PocketBase registers its routes when the
// serve event fires, and firing that twice on one app collides.
func newApp(t testing.TB) *tests.TestApp {
	t.Helper()

	app := testutil.SeededApp(t)
	conf := &config.Config{}
	conf.Normalize()
	books.Register(app, conf)

	return app
}

// asUser runs a scenario authenticated as the given fixture user.
func asUser(t *testing.T, userId string, scenario tests.ApiScenario) {
	t.Helper()

	if scenario.TestAppFactory == nil {
		scenario.TestAppFactory = newApp
	}

	headers := map[string]string{}
	for key, value := range scenario.Headers {
		headers[key] = value
	}
	scenario.Headers = headers

	before := scenario.BeforeTestFunc
	scenario.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
		user, err := app.FindRecordById(schema.CollectionUsers, userId)
		if err != nil {
			t.Fatalf("failed to load the fixture user %q: %v", userId, err)
		}
		headers["Authorization"] = testutil.UserToken(t, user)

		if before != nil {
			before(t, app, e)
		}
	}

	scenario.Test(t)
}

const container = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

const packageDocument = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Zeit des Sturms</dc:title>
    <dc:creator>Andrzej Sapkowski</dc:creator>
    <dc:language>de</dc:language>
    <dc:identifier>urn:isbn:9783423426091</dc:identifier>
    <meta name="cover" content="cover-img"/>
  </metadata>
  <manifest>
    <item id="cover-img" href="images/cover.jpg" media-type="image/jpeg"/>
    <item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="one"/></spine>
</package>`

// jpegBytes is the shortest byte sequence that sniffs as a JPEG. The cover is
// validated by content, not by the name the archive gave it.
func jpegBytes() []byte {
	header := []byte{0xFF, 0xD8, 0xFF, 0xE0}

	return append(header, bytes.Repeat([]byte{0x00}, 60)...)
}

// epubBytes builds a small but structurally real EPUB with the given number of
// words in its single spine document.
func epubBytes(t testing.TB, words int) []byte {
	t.Helper()

	return epubBytesWith(t, words, packageDocument)
}

// epubBytesWith builds an EPUB around a specific package document.
//
// The document cannot be patched into the finished archive afterwards: zip
// deflates it, so the XML is not present in the bytes as written.
func epubBytesWith(t testing.TB, words int, pkg string) []byte {
	t.Helper()

	return epubBytesFull(t, words, pkg, jpegBytes())
}

// epubBytesFull builds an EPUB with a specific package document and cover.
func epubBytesFull(t testing.TB, words int, pkg string, cover []byte) []byte {
	t.Helper()

	body := strings.TrimSpace(strings.Repeat("wort ", words))
	chapter := `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>` +
		body + `</p></body></html>`

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, item := range []struct{ name, content string }{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", container},
		{"OEBPS/content.opf", pkg},
		{"OEBPS/text/one.xhtml", chapter},
		{"OEBPS/images/cover.jpg", string(cover)},
	} {
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

// upload builds a multipart body for a book upload.
func upload(t testing.TB, owner, filename string, content []byte, extra map[string]string) (*bytes.Reader, string) {
	t.Helper()

	var buffer bytes.Buffer
	form := multipart.NewWriter(&buffer)

	if err := form.WriteField(schema.FieldOwner, owner); err != nil {
		t.Fatalf("write owner: %v", err)
	}
	for field, value := range extra {
		if err := form.WriteField(field, value); err != nil {
			t.Fatalf("write %s: %v", field, err)
		}
	}

	part, err := form.CreateFormFile(schema.FieldFile, filename)
	if err != nil {
		t.Fatalf("create file part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}

	return bytes.NewReader(buffer.Bytes()), form.FormDataContentType()
}

func TestUploadDescribesTheBook(t *testing.T) {
	body, contentType := upload(t, testutil.IdUserA, "Sapkowski_Zeit-des-Sturms.epub", epubBytes(t, 310), nil)

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "an upload is described from the file itself",
		Method:          http.MethodPost,
		URL:             booksURL,
		Body:            body,
		Headers:         map[string]string{"Content-Type": contentType},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title":"Zeit des Sturms"`, `"language":"de"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			book, err := app.FindFirstRecordByData(schema.CollectionBooks, schema.FieldOwner, testutil.IdUserA)
			if err != nil {
				t.Fatalf("expected the book to be stored: %v", err)
			}

			// The hash a device would compute for this very file. Nothing about
			// it is supplied by the client.
			if got := book.GetString(schema.FieldHashBinary); len(got) != 32 {
				t.Errorf("binary hash is %q", got)
			}
			if got := book.GetString(schema.FieldHashFilename); len(got) != 32 {
				t.Errorf("filename hash is %q", got)
			}
			if got := book.GetString(schema.FieldContentHash); len(got) != 64 {
				t.Errorf("content hash is %q", got)
			}
			if got := book.GetInt(schema.FieldWordCount); got != 310 {
				t.Errorf("word count is %d, want 310", got)
			}
			// 310 words at the default 155 words per page.
			if got := book.GetInt(schema.FieldPageCount); got != 2 {
				t.Errorf("page count is %d, want 2", got)
			}
			if got := book.GetString(schema.FieldCover); got == "" {
				t.Error("expected the cover to be extracted from the archive")
			}

			var authors []string
			if err := json.Unmarshal([]byte(book.GetString(schema.FieldAuthors)), &authors); err != nil {
				t.Fatalf("authors are not JSON: %v", err)
			}
			if len(authors) != 1 || authors[0] != "Andrzej Sapkowski" {
				t.Errorf("authors are %v", authors)
			}

			var identifiers map[string]string
			if err := json.Unmarshal([]byte(book.GetString(schema.FieldIdentifiers)), &identifiers); err != nil {
				t.Fatalf("identifiers are not JSON: %v", err)
			}
			if identifiers["ISBN"] != "9783423426091" {
				t.Errorf("identifiers are %v", identifiers)
			}
		},
	})
}

func TestUploadRejectsNonEPUB(t *testing.T) {
	body, contentType := upload(t, testutil.IdUserA, "notes.epub", []byte("this is not a zip at all"), nil)

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a file that is not an EPUB is refused",
		Method:          http.MethodPost,
		URL:             booksURL,
		Body:            body,
		Headers:         map[string]string{"Content-Type": contentType},
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"not an EPUB"},
	})
}

// A zip without an EPUB container is the more interesting negative: it parses
// as an archive and only fails on the container lookup.
func TestUploadRejectsPlainZip(t *testing.T) {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	writer, err := archive.Create("readme.txt")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := writer.Write([]byte("just a zip")); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	body, contentType := upload(t, testutil.IdUserA, "book.epub", buffer.Bytes(), nil)

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a plain zip is refused",
		Method:          http.MethodPost,
		URL:             booksURL,
		Body:            body,
		Headers:         map[string]string{"Content-Type": contentType},
		ExpectedStatus:  http.StatusBadRequest,
		ExpectedContent: []string{"not an EPUB"},
	})
}

// A title typed by the uploader is theirs, and must not be overwritten by the
// publisher's metadata.
func TestUploadKeepsASuppliedTitle(t *testing.T) {
	body, contentType := upload(t, testutil.IdUserA, "book.epub", epubBytes(t, 100),
		map[string]string{schema.FieldTitle: "My own title"})

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "a supplied title survives",
		Method:          http.MethodPost,
		URL:             booksURL,
		Body:            body,
		Headers:         map[string]string{"Content-Type": contentType},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title":"My own title"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			book, err := app.FindFirstRecordByData(schema.CollectionBooks, schema.FieldOwner, testutil.IdUserA)
			if err != nil {
				t.Fatalf("expected the book to be stored: %v", err)
			}
			// The language still comes from the file, so this is not simply
			// "the client wins".
			if book.GetString(schema.FieldLanguage) != "de" {
				t.Errorf("language is %q, want de", book.GetString(schema.FieldLanguage))
			}
		},
	})
}

// A book with no title of its own falls back to the file name rather than
// showing up as a blank row.
func TestUploadFallsBackToTheFileName(t *testing.T) {
	stripped := strings.Replace(packageDocument, "<dc:title>Zeit des Sturms</dc:title>", "", 1)

	body, contentType := upload(t, testutil.IdUserA, "Some Untitled Book.epub",
		epubBytesWith(t, 50, stripped), nil)

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "an untitled book is named after its file",
		Method:          http.MethodPost,
		URL:             booksURL,
		Body:            body,
		Headers:         map[string]string{"Content-Type": contentType},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title":"Some Untitled Book"`},
	})
}

// A book whose artwork cannot be stored is still a book. The cover is sniffed
// by content, so an archive whose "cover.jpg" is not an image at all must be
// dropped quietly rather than failing the field's mime check and taking the
// upload down with it.
func TestUploadSurvivesAnUnusableCover(t *testing.T) {
	content := epubBytesFull(t, 90, packageDocument, []byte("this is not an image at all"))

	body, contentType := upload(t, testutil.IdUserA, "book.epub", content, nil)

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:            "an unusable cover does not fail the upload",
		Method:          http.MethodPost,
		URL:             booksURL,
		Body:            body,
		Headers:         map[string]string{"Content-Type": contentType},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"cover":""`, `"word_count":90`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			book, err := app.FindFirstRecordByData(schema.CollectionBooks, schema.FieldOwner, testutil.IdUserA)
			if err != nil {
				t.Fatalf("expected the book to be stored: %v", err)
			}
			if got := book.GetString(schema.FieldCover); got != "" {
				t.Errorf("expected no cover, got %q", got)
			}
			if book.GetInt(schema.FieldWordCount) != 90 {
				t.Errorf("the rest of the book was not described")
			}
		},
	})
}

// The same file twice is the same book, whatever it is named the second time.
func TestUploadRejectsTheSameFileTwice(t *testing.T) {
	content := epubBytes(t, 120)
	digest := sha256.Sum256(content)

	body, contentType := upload(t, testutil.IdUserA, "a-different-name.epub", content, nil)

	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:           "the same file cannot be uploaded twice",
		Method:         http.MethodPost,
		URL:            booksURL,
		Body:           body,
		Headers:        map[string]string{"Content-Type": contentType},
		ExpectedStatus: http.StatusBadRequest,
		// Not "Failed to create record.", which is what the unique index alone
		// produces: it is true of a dozen different problems, and this one is
		// not a problem at all — the book is already there. The title says
		// which one, because the second upload can carry a different file name.
		ExpectedContent:    []string{"Zeit des Sturms", "already in your library"},
		NotExpectedContent: []string{"Failed to create record"},
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			seedBookWith(t, app, hex.EncodeToString(digest[:]))
		},
	})
}

// The library is per account: two people owning the same file each keep their
// own copy, and neither is told about the other's.
func TestTheSameFileCanBeUploadedByAnotherAccount(t *testing.T) {
	content := epubBytes(t, 120)
	digest := sha256.Sum256(content)

	body, contentType := upload(t, testutil.IdUserB, "same-book.epub", content, nil)

	asUser(t, testutil.IdUserB, tests.ApiScenario{
		Name:            "another account uploads the same file",
		Method:          http.MethodPost,
		URL:             booksURL,
		Body:            body,
		Headers:         map[string]string{"Content-Type": contentType},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title"`},
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			seedBookWith(t, app, hex.EncodeToString(digest[:]))
		},
	})
}

func TestDerivedFieldsCannotBeEdited(t *testing.T) {
	for _, field := range []string{
		schema.FieldHashBinary,
		schema.FieldContentHash,
		schema.FieldWordCount,
	} {
		asUser(t, testutil.IdUserA, tests.ApiScenario{
			Name:   "editing " + field + " is refused",
			Method: http.MethodPatch,
			URL:    booksURL + "/" + testutil.PadId("booka"),
			Body:   strings.NewReader(`{"` + field + `":"0"}`),
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			BeforeTestFunc:  seedBook,
			ExpectedStatus:  http.StatusBadRequest,
			ExpectedContent: []string{"describes the uploaded file"},
		})
	}
}

// The page count everything is reckoned in comes out of the reading itself.
// A number typed in here would sit in front of every statistic without anything
// having produced it.
func TestTheMeasurementCannotBeSetByHand(t *testing.T) {
	for _, field := range []string{
		schema.FieldMeasuredPages,
		schema.FieldMeasuredDevice,
		schema.FieldMeasuredThrough,
	} {
		asUser(t, testutil.IdUserA, tests.ApiScenario{
			Name:   "editing " + field + " is refused",
			Method: http.MethodPatch,
			URL:    booksURL + "/" + testutil.PadId("booka"),
			Body:   strings.NewReader(`{"` + field + `":"1"}`),
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			BeforeTestFunc:  seedBook,
			ExpectedStatus:  http.StatusBadRequest,
			ExpectedContent: []string{"measured from your reading"},
		})
	}
}

// Correcting the publisher's metadata is the owner's business, so the fields
// that are not derived from the file stay editable.
func TestTitleRemainsEditable(t *testing.T) {
	asUser(t, testutil.IdUserA, tests.ApiScenario{
		Name:   "the title can be corrected",
		Method: http.MethodPatch,
		URL:    booksURL + "/" + testutil.PadId("booka"),
		Body:   strings.NewReader(`{"title":"A better title"}`),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		BeforeTestFunc:  seedBook,
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"title":"A better title"`},
	})
}

func TestBooksAreOwnerScoped(t *testing.T) {
	asUser(t, testutil.IdUserB, tests.ApiScenario{
		Name:            "another user's book is not visible",
		Method:          http.MethodGet,
		URL:             booksURL + "/" + testutil.PadId("booka"),
		BeforeTestFunc:  seedBook,
		ExpectedStatus:  http.StatusNotFound,
		ExpectedContent: []string{`"status":404`},
	})
}

// seedBook creates a book for user A directly, bypassing the upload path.
func seedBook(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
	seedBookWith(t, app, strings.Repeat("a", 64))
}

// seedBookWith creates a book carrying a specific content hash.
func seedBookWith(t testing.TB, app *tests.TestApp, contentHash string) {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("find books collection: %v", err)
	}

	file, err := filesystem.NewFileFromBytes(epubBytes(t, 40), "seeded.epub")
	if err != nil {
		t.Fatalf("build seed file: %v", err)
	}

	record := core.NewRecord(collection)
	record.Id = testutil.PadId("booka")
	record.Set(schema.FieldOwner, testutil.IdUserA)
	record.Set(schema.FieldFile, file)
	record.Set(schema.FieldTitle, "Zeit des Sturms")
	record.Set(schema.FieldWordCount, 108755)
	record.Set(schema.FieldPageCount, 700)
	record.Set(schema.FieldContentHash, contentHash)
	record.Set(schema.FieldHashBinary, "043f11771ef9d191364ac0ba08198d36")

	if err := app.Save(record); err != nil {
		t.Fatalf("seed book: %v", err)
	}
}
