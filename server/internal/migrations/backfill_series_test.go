//
// File:        internal/migrations/backfill_series_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/migrations"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

const seriesPackage = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>A Clash of Kings</dc:title>
    <dc:language>en</dc:language>
    <dc:subject>Fantasy</dc:subject>
    <meta name="calibre:series" content="A Song of Ice and Fire"/>
    <meta name="calibre:series_index" content="2.0"/>
  </metadata>
  <manifest><item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="one"/></spine>
</package>`

// seriesEPUB builds a small but structurally real EPUB around a package
// document.
func seriesEPUB(t testing.TB, pkg string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)

	for name, content := range map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`,
		"OEBPS/content.opf": pkg,
		"OEBPS/text/one.xhtml": `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml">` +
			`<body><p>wort wort wort</p></body></html>`,
	} {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	return buffer.Bytes()
}

// storeUndescribedBook writes a book with its file but none of what the file
// says, which is the state every book uploaded before the columns existed is in.
func storeUndescribedBook(t testing.TB, app core.App, id, owner string, content []byte) *core.Record {
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
	record.Set(schema.FieldTitle, "A Clash of Kings")
	record.Set(schema.FieldContentHash, id)

	if err := app.Save(record); err != nil {
		t.Fatalf("save book: %v", err)
	}

	return record
}

func TestBackfillReadsTheSeriesOfStoredBooks(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := storeUndescribedBook(t, app, testutil.PadId("booka"), alice.Id, seriesEPUB(t, seriesPackage))
	if book.GetString(schema.FieldSeries) != "" {
		t.Fatal("the book already had a series, the fixture proves nothing")
	}

	if err := migrations.BackfillSeriesAndSubjects(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	stored, err := app.FindRecordById(schema.CollectionBooks, book.Id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := stored.GetString(schema.FieldSeries); got != "A Song of Ice and Fire" {
		t.Errorf("series is %q", got)
	}
	if got := stored.GetFloat(schema.FieldSeriesIndex); got != 2 {
		t.Errorf("series index is %v, want 2", got)
	}

	var subjects []string
	if err := json.Unmarshal([]byte(stored.GetString(schema.FieldSubjects)), &subjects); err != nil {
		t.Fatalf("subjects are not JSON: %v", err)
	}
	if len(subjects) != 1 || subjects[0] != "Fantasy" {
		t.Errorf("subjects are %v", subjects)
	}
}

// The library is re-read in place, so the books have to come out of it with the
// same modification time they went in with. A shelf of two hundred books that
// all claim to have been edited today is a worse answer than no shelf.
func TestBackfillDoesNotTouchTheTimestamps(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := storeUndescribedBook(t, app, testutil.PadId("booka"), alice.Id, seriesEPUB(t, seriesPackage))

	// Compared as they are stored: the record in hand still carries the
	// nanoseconds the database rounded away when it was written.
	before := book.GetDateTime(schema.FieldUpdated).String()

	if err := migrations.BackfillSeriesAndSubjects(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	stored, err := app.FindRecordById(schema.CollectionBooks, book.Id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := stored.GetDateTime(schema.FieldUpdated).String(); got != before {
		t.Errorf("updated moved from %v to %v", before, got)
	}
}

// A record whose file has gone missing, or was never an EPUB, must not stop a
// server from starting.
func TestBackfillSurvivesAnUnreadableBook(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	broken := storeUndescribedBook(t, app, testutil.PadId("bookb"), alice.Id, []byte("PK\x03\x04 not really"))
	good := storeUndescribedBook(t, app, testutil.PadId("booka"), alice.Id, seriesEPUB(t, seriesPackage))

	if err := migrations.BackfillSeriesAndSubjects(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	stored, err := app.FindRecordById(schema.CollectionBooks, good.Id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.GetString(schema.FieldSeries) == "" {
		t.Error("the readable book was skipped along with the broken one")
	}

	stored, err = app.FindRecordById(schema.CollectionBooks, broken.Id)
	if err != nil {
		t.Fatalf("reload the broken one: %v", err)
	}
	if stored.GetString(schema.FieldSeries) != "" {
		t.Error("the broken book was given a series out of nowhere")
	}
}
