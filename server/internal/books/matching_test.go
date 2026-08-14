//
// File:        internal/books/matching_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/koreader"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// The hash the reference EPUB actually produces, and the id the production
// database recorded for it. They are the same string, which is the whole point.
const zeitDesSturms = "043f11771ef9d191364ac0ba08198d36"

// matchingApp returns a migrated app with the matching hooks bound but no
// fixture data, so each test states exactly what exists.
func matchingApp(t testing.TB) *tests.TestApp {
	t.Helper()

	app := testutil.NewApp(t)
	conf := &config.Config{}
	conf.Normalize()
	books.Register(app, conf)

	return app
}

// createBook stores a book with the given hashes, bypassing the upload path.
func createBook(t testing.TB, app core.App, id, owner, binary, filename string) *core.Record {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("find books collection: %v", err)
	}

	file, err := filesystem.NewFileFromBytes([]byte("PK\x03\x04 not really an epub"), "book.epub")
	if err != nil {
		t.Fatalf("build file: %v", err)
	}

	record := core.NewRecord(collection)
	record.Id = id
	record.Set(schema.FieldOwner, owner)
	record.Set(schema.FieldFile, file)
	record.Set(schema.FieldTitle, "Zeit des Sturms")
	record.Set(schema.FieldContentHash, id)
	record.Set(schema.FieldHashBinary, binary)
	record.Set(schema.FieldHashFilename, filename)

	if err := app.Save(record); err != nil {
		t.Fatalf("save book: %v", err)
	}

	return record
}

func TestDocumentIsMatchedOnArrival(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	book := createBook(t, app, testutil.PadId("booka"), user.Id, zeitDesSturms, "0123456789abcdef0123456789abcdef")

	// A push arrives for a hash the library already knows.
	document := testutil.CreateDocument(t, app, user, "", zeitDesSturms, 0.25, time.Now())

	stored, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if got := stored.GetString(schema.FieldBook); got != book.Id {
		t.Errorf("document is linked to %q, want %q", got, book.Id)
	}
}

// The common case: the book is uploaded long after the reading was done.
func TestUploadingABookLinksExistingDocuments(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	document := testutil.CreateDocument(t, app, user, "", zeitDesSturms, 0.9, time.Now())
	if stored, _ := app.FindRecordById(schema.CollectionDocuments, document.Id); stored.GetString(schema.FieldBook) != "" {
		t.Fatal("expected the document to start unlinked")
	}

	book := createBook(t, app, testutil.PadId("booka"), user.Id, zeitDesSturms, "0123456789abcdef0123456789abcdef")

	stored, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if got := stored.GetString(schema.FieldBook); got != book.Id {
		t.Errorf("document is linked to %q, want %q", got, book.Id)
	}
}

// KOReader sends whichever hash its checksum setting produces, and the server
// cannot tell them apart, so a filename hash has to match too.
func TestMatchingUsesEitherHash(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	filenameHash := "915bd5f6d29f6038e88dad85acaf8958"
	book := createBook(t, app, testutil.PadId("booka"), user.Id, zeitDesSturms, filenameHash)

	document := testutil.CreateDocument(t, app, user, "", filenameHash, 0.1, time.Now())

	stored, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if got := stored.GetString(schema.FieldBook); got != book.Id {
		t.Errorf("a filename hash did not match: linked to %q, want %q", got, book.Id)
	}
}

// A book belongs to one account. Another account reading the same file gets no
// link from it, however identical the bytes are.
func TestMatchingIsOwnerScoped(t *testing.T) {
	app := matchingApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	bob := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)

	createBook(t, app, testutil.PadId("booka"), alice.Id, zeitDesSturms, "0123456789abcdef0123456789abcdef")

	document := testutil.CreateDocument(t, app, bob, "", zeitDesSturms, 0.4, time.Now())

	stored, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if got := stored.GetString(schema.FieldBook); got != "" {
		t.Errorf("another account's book was linked: %q", got)
	}
}

func TestDocumentWithoutAMatchStaysUnlinked(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	createBook(t, app, testutil.PadId("booka"), user.Id, zeitDesSturms, "0123456789abcdef0123456789abcdef")

	document := testutil.CreateDocument(t, app, user, "", "ffffffffffffffffffffffffffffffff", 0.4, time.Now())

	stored, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if got := stored.GetString(schema.FieldBook); got != "" {
		t.Errorf("an unrelated hash was linked to %q", got)
	}
}

// Deleting the file must not delete the reading it was progress through. The
// relation is cleared and the document survives.
func TestDeletingABookKeepsTheDocument(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := createBook(t, app, testutil.PadId("booka"), user.Id, zeitDesSturms, "0123456789abcdef0123456789abcdef")
	document := testutil.CreateDocument(t, app, user, "", zeitDesSturms, 0.75, time.Now())

	if err := app.Delete(book); err != nil {
		t.Fatalf("delete book: %v", err)
	}

	stored, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("the document did not survive its book: %v", err)
	}
	if got := stored.GetString(schema.FieldBook); got != "" {
		t.Errorf("the book reference was left dangling: %q", got)
	}
	if got := stored.GetFloat(schema.FieldProgress); got != 0.75 {
		t.Errorf("progress is %v, want 0.75", got)
	}
}

func TestFindForDocumentRejectsEmptyInput(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	createBook(t, app, testutil.PadId("booka"), user.Id, zeitDesSturms, "")

	// An empty hash must not match the book whose filename hash is also empty.
	found, err := books.FindForDocument(app, user.Id, "")
	if err != nil {
		t.Fatalf("FindForDocument: %v", err)
	}
	if found != nil {
		t.Errorf("an empty hash matched %q", found.Id)
	}
}

// The device path is the one that matters: a progress push arriving over the
// KOReader protocol must come out linked, without the device knowing anything
// about the library.
func TestProgressPushFromADeviceIsLinked(t *testing.T) {
	const pushed = zeitDesSturms

	scenario := tests.ApiScenario{
		Name:   "a device push is linked to the matching book",
		Method: http.MethodPut,
		URL:    "/koreader/syncs/progress",
		Body: strings.NewReader(`{"document":"` + pushed +
			`","progress":"/body/DocFragment[11]/body/div/p[5]/text().0","percentage":0.42,"device":"Kobo Clara","device_id":"abc"}`),
		Headers: map[string]string{
			koreader.HeaderAuthUser: testutil.KoUsernameA,
			koreader.HeaderAuthKey:  testutil.Md5Hex(testutil.KoPasswordA),
			"Content-Type":          "application/json",
		},
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			app := testutil.SeededApp(t)
			conf := &config.Config{}
			conf.Normalize()
			books.Register(app, conf)
			koreader.Register(app, conf)

			createBook(t, app, testutil.PadId("bookz"), testutil.IdUserA, pushed, "")

			return app
		},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"document":"` + pushed + `"`},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			document, err := app.FindFirstRecordByData(schema.CollectionDocuments, schema.FieldDocument, pushed)
			if err != nil {
				t.Fatalf("the push did not create a document: %v", err)
			}
			if got := document.GetString(schema.FieldBook); got != testutil.PadId("bookz") {
				t.Errorf("the pushed document is linked to %q, want the uploaded book", got)
			}
		},
	}
	scenario.Test(t)
}
