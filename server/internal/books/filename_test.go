//
// File:        internal/books/filename_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books_test

import (
	"strings"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
)

// titled stores a book with the given title and nothing else derived.
func titled(t testing.TB, app core.App, owner, title string) *core.Record {
	t.Helper()

	record := createBook(t, app, testutil.PadId("booka"), owner, "", "")
	record.Set(schema.FieldTitle, title)
	if err := app.Save(record); err != nil {
		t.Fatalf("save book: %v", err)
	}

	return record
}

// draft is a book record that is never stored, because the name is derived from
// the title and the id alone and storing a dozen of them proves nothing extra.
func draft(t testing.TB, app core.App, title string) *core.Record {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("find books collection: %v", err)
	}

	record := core.NewRecord(collection)
	record.Id = testutil.PadId("booka")
	record.Set(schema.FieldTitle, title)

	return record
}

func TestCatalogFilenameIsDerivedFromTheTitle(t *testing.T) {
	app := matchingApp(t)

	cases := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "an ordinary title",
			title: "Zeit des Sturms",
			want:  "Zeit des Sturms.epub",
		},
		{
			// A slash would open a second path segment in the acquisition URL,
			// and a colon is refused outright by some filesystems.
			name:  "a title with path characters in it",
			title: "Feuertaufe/Der Schwalbenturm: Teil 2",
			want:  "Feuertaufe Der Schwalbenturm Teil 2.epub",
		},
		{
			// Replaced with a space rather than dropped, or the two words on
			// either side would run together.
			name:  "runs of removed characters collapse",
			title: "A  ///  B",
			want:  "A B.epub",
		},
		{
			name:  "umlauts are kept, because a reader can write them",
			title: "Über allem",
			want:  "Über allem.epub",
		},
		{
			// A leading dot would hide the book in the reader's file list.
			name:  "a leading dot is not a hidden file",
			title: ".hidden",
			want:  "hidden.epub",
		},
		{
			name:  "a title of nothing falls back to the id",
			title: "   ",
			want:  testutil.PadId("booka") + ".epub",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			book := draft(t, app, testCase.title)

			if got := books.CatalogFilename(book); got != testCase.want {
				t.Errorf("expected the name %q, got %q", testCase.want, got)
			}
		})
	}
}

// A file name has to survive being written to a device, several of which stop
// at 255 bytes for the whole name.
func TestALongTitleIsCutToALengthADeviceAccepts(t *testing.T) {
	app := matchingApp(t)

	book := draft(t, app, strings.Repeat("Witcher ", 40))

	name := books.CatalogFilename(book)
	if len([]rune(name)) > 130 {
		t.Errorf("expected a name a filesystem accepts, got one of %d runes", len([]rune(name)))
	}
	if !strings.HasSuffix(name, ".epub") {
		t.Errorf("expected the extension to survive the cut, got %q", name)
	}
}

// The whole argument for the catalog: a book downloaded from here is recognised
// by the device that downloaded it, before any progress has ever been pushed.
func TestAPushForACatalogDownloadIsMatched(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := titled(t, app, user.Id, "Zeit des Sturms")

	// The reader is set to identify documents by name, so it sends the hash of
	// the name the catalog served the file under.
	hash := epub.FilenameMD5(books.CatalogFilename(book))
	document := testutil.CreateDocument(t, app, user, "", hash, 0.25, time.Now())

	stored, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if got := stored.GetString(schema.FieldBook); got != book.Id {
		t.Errorf("document is linked to %q, want %q", got, book.Id)
	}
}

// The stored hash follows the title, or a renamed book would keep claiming to
// be served under a name it no longer is.
func TestRenamingABookMovesItsCatalogHash(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := titled(t, app, user.Id, "Zeit des Sturms")
	before := book.GetString(schema.FieldHashCatalog)

	book.Set(schema.FieldTitle, "Season of Storms")
	if err := app.Save(book); err != nil {
		t.Fatalf("rename book: %v", err)
	}

	stored, err := app.FindRecordById(schema.CollectionBooks, book.Id)
	if err != nil {
		t.Fatalf("reload book: %v", err)
	}

	after := stored.GetString(schema.FieldHashCatalog)
	if after == before {
		t.Error("expected the catalog hash to follow the new title")
	}
	if want := epub.FilenameMD5("Season of Storms.epub"); after != want {
		t.Errorf("expected the hash %q, got %q", want, after)
	}
}

// The hash is stored when the book is, so that matching is an indexed lookup
// rather than a name derived for every book on every push.
func TestTheCatalogHashIsStoredOnUpload(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := titled(t, app, user.Id, "Zeit des Sturms")

	if want := epub.FilenameMD5("Zeit des Sturms.epub"); book.GetString(schema.FieldHashCatalog) != want {
		t.Errorf("expected the hash %q, got %q", want, book.GetString(schema.FieldHashCatalog))
	}
}
