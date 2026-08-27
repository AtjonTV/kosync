//
// File:        internal/books/storage_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
)

// manyChapters builds a book of the given number of spine documents, so that
// opening it is a walk over a central directory rather than one entry.
func manyChapters(t testing.TB, chapters int) []byte {
	t.Helper()

	manifest := strings.Builder{}
	spine := strings.Builder{}
	for chapter := range chapters {
		manifest.WriteString(fmt.Sprintf(
			`<item id="c%d" href="text/%d.xhtml" media-type="application/xhtml+xml"/>`, chapter, chapter))
		spine.WriteString(fmt.Sprintf(`<itemref idref="c%d"/>`, chapter))
	}

	pkg := `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Zeit des Sturms</dc:title></metadata>
  <manifest>` + manifest.String() + `</manifest>
  <spine>` + spine.String() + `</spine>
</package>`

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	write := func(name, content string) {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	write("mimetype", "application/epub+zip")
	write("META-INF/container.xml", container)
	write("OEBPS/content.opf", pkg)
	for chapter := range chapters {
		write(fmt.Sprintf("OEBPS/text/%d.xhtml", chapter), fmt.Sprintf(
			`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml">`+
				`<head><title>Kapitel %d</title></head><body><p>%s</p></body></html>`,
			chapter+1, strings.TrimSpace(strings.Repeat("wort ", 200))))
	}

	if err := archive.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	return buffer.Bytes()
}

// storedApp returns an app holding one book of the given bytes.
func storedApp(t testing.TB, content []byte) (core.App, *core.Record) {
	t.Helper()

	app := matchingApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	return app, storeBookWithoutCover(t, app, testutil.PadId("booka"), alice.Id, content)
}

// The point of this path is that a book is read where it lies: the archive's
// central directory and the one entry asked for, not the whole file through
// memory on the way to showing one chapter of it.
func TestOpenReadsAStoredBookInPlace(t *testing.T) {
	app, record := storedApp(t, manyChapters(t, 40))

	book, err := books.Open(app, record)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer book.Close()

	documents := book.Spine()
	if len(documents) != 40 {
		t.Fatalf("the book has %d chapters, want 40", len(documents))
	}
	if documents[39].Title != "Kapitel 40" {
		t.Errorf("the last chapter is called %q", documents[39].Title)
	}

	// The last document sits at the far end of the file, past the point every
	// earlier read left the handle at.
	raw, _, err := book.ReadDocument(39)
	if err != nil {
		t.Fatalf("ReadDocument: %v", err)
	}
	if !strings.Contains(string(raw), "Kapitel 40") {
		t.Errorf("the last chapter is %.60s", raw)
	}
}

// Reading the same entries again after the handle has been moved to the end of
// the file is the case the seeking adapter exists for.
func TestAStoredBookCanBeReadOutOfOrder(t *testing.T) {
	app, record := storedApp(t, manyChapters(t, 40))

	book, err := books.Open(app, record)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer book.Close()

	for _, index := range []int{39, 0, 20, 0, 39} {
		raw, document, err := book.ReadDocument(index)
		if err != nil {
			t.Fatalf("ReadDocument(%d): %v", index, err)
		}
		if want := fmt.Sprintf("Kapitel %d", index+1); document.Title != want {
			t.Errorf("chapter %d is called %q, want %q", index, document.Title, want)
		}
		if !strings.Contains(string(raw), "wort") {
			t.Errorf("chapter %d came back empty", index)
		}
	}
}

// The collection requires a file, so this is not a state an upload can leave
// behind — but a row can lose one to a storage backend that was cleaned up
// underneath it, and a request for that book must answer rather than fail.
func TestOpeningABookWithoutAFileSaysSo(t *testing.T) {
	app := matchingApp(t)

	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("find books collection: %v", err)
	}

	record := core.NewRecord(collection)
	record.Id = testutil.PadId("bookb")
	record.Set(schema.FieldTitle, "Zeit des Sturms")

	if _, err := books.Open(app, record); !errors.Is(err, books.ErrNoFile) {
		t.Errorf("opening a fileless book returned %v", err)
	}
}

// A zip that is not an EPUB passes the upload's own check on the file type, so
// this is the shape a stored file that cannot be previewed really arrives in.
func TestOpeningSomethingThatIsNotAnEPUBSaysSo(t *testing.T) {
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

	app, record := storedApp(t, buffer.Bytes())

	if _, err := books.Open(app, record); !errors.Is(err, epub.ErrNotEPUB) {
		t.Errorf("opening a stored non-EPUB returned %v", err)
	}
}
