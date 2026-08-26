//
// File:        internal/books/covers_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books_test

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// storeBookWithoutCover writes a book with its file and no cover, which is the
// state a book uploaded before the reader could follow its cover is left in.
func storeBookWithoutCover(t testing.TB, app core.App, id, owner string, content []byte) *core.Record {
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
	if got := record.GetString(schema.FieldCover); got != "" {
		t.Fatalf("the seeded book already has a cover %q", got)
	}

	return record
}

// storedBook returns a book as it stands in the database now.
func storedBook(t testing.TB, app core.App, id string) *core.Record {
	t.Helper()

	stored, err := app.FindRecordById(schema.CollectionBooks, id)
	if err != nil {
		t.Fatalf("reload book: %v", err)
	}

	return stored
}

// coverOf returns the cover a stored book has now.
func coverOf(t testing.TB, app core.App, id string) string {
	t.Helper()

	return storedBook(t, app, id).GetString(schema.FieldCover)
}

// coverlessEPUB builds a book that has no cover to find: no pointer at one, and
// no picture in the archive to guess at.
func coverlessEPUB(t testing.TB) []byte {
	t.Helper()

	const pkg = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Zeit des Sturms</dc:title>
  </metadata>
  <manifest>
    <item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="one"/></spine>
</package>`

	chapter := `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>` +
		strings.TrimSpace(strings.Repeat("wort ", 40)) + `</p></body></html>`

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, item := range []struct{ name, content string }{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", container},
		{"OEBPS/content.opf", pkg},
		{"OEBPS/text/one.xhtml", chapter},
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

func TestBackfillCoversFillsInABookThatHasNone(t *testing.T) {
	app := matchingApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	book := storeBookWithoutCover(t, app, testutil.PadId("booka"), alice.Id, epubBytes(t, 40))

	filled, err := books.BackfillCovers(app)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if filled != 1 {
		t.Fatalf("filled %d covers, want 1", filled)
	}

	// The extension follows the sniffed content, as it does on upload.
	if got := coverOf(t, app, book.Id); !strings.HasSuffix(got, ".jpg") {
		t.Errorf("cover is %q", got)
	}
}

// A book with nothing to find must not be counted, and must not be written to
// on every pass for the rest of its life.
func TestBackfillCoversLeavesABookWithNoneAlone(t *testing.T) {
	app := matchingApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	book := storeBookWithoutCover(t, app, testutil.PadId("booka"), alice.Id, coverlessEPUB(t))

	// Reloaded, because the stored timestamp is truncated to milliseconds and
	// the one held in memory is not.
	updated := storedBook(t, app, book.Id).GetDateTime(schema.FieldUpdated)

	filled, err := books.BackfillCovers(app)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if filled != 0 {
		t.Errorf("filled %d covers, want 0", filled)
	}
	if got := coverOf(t, app, book.Id); got != "" {
		t.Errorf("cover is %q, want none", got)
	}

	if got := storedBook(t, app, book.Id).GetDateTime(schema.FieldUpdated); !got.Equal(updated) {
		t.Errorf("the book was written to: %v, want %v", got, updated)
	}
}

// The pass looks at the books that have no cover, so a second one has nothing
// left to do — and cannot replace a cover that is already there.
func TestBackfillCoversKeepsTheCoverABookAlreadyHas(t *testing.T) {
	app := matchingApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	book := storeBookWithoutCover(t, app, testutil.PadId("booka"), alice.Id, epubBytes(t, 40))

	if _, err := books.BackfillCovers(app); err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	first := coverOf(t, app, book.Id)

	filled, err := books.BackfillCovers(app)
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if filled != 0 {
		t.Errorf("filled %d covers on the second pass, want 0", filled)
	}
	if got := coverOf(t, app, book.Id); got != first {
		t.Errorf("cover is %q, want the %q it already had", got, first)
	}
}

// One unreadable file must not stop the shelf being looked at: the whole point
// is the books that can be repaired.
func TestBackfillCoversCarriesOnPastABookItCannotRead(t *testing.T) {
	app := matchingApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	broken := storeBookWithoutCover(t, app, testutil.PadId("bookb"), alice.Id,
		[]byte("PK\x03\x04 not really an epub"))
	good := storeBookWithoutCover(t, app, testutil.PadId("booka"), alice.Id, epubBytes(t, 40))

	filled, err := books.BackfillCovers(app)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if filled != 1 {
		t.Fatalf("filled %d covers, want 1", filled)
	}
	if got := coverOf(t, app, good.Id); got == "" {
		t.Error("the readable book has no cover")
	}
	if got := coverOf(t, app, broken.Id); got != "" {
		t.Errorf("the unreadable book has a cover %q", got)
	}
}

func TestCoverBackfillIsScheduled(t *testing.T) {
	app := matchingApp(t)

	for _, job := range app.Cron().Jobs() {
		if job.Id() == books.JobCovers {
			return
		}
	}

	t.Errorf("%q is not among the scheduled jobs", books.JobCovers)
}
