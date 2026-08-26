//
// File:        internal/migrations/backfill_description_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations_test

import (
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/migrations"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
)

// blurbPackage carries a description in the shape most publishers ship: HTML,
// escaped by the XML around it.
const blurbPackage = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>A Clash of Kings</dc:title>
    <dc:language>en</dc:language>
    <dc:description>&lt;p&gt;Sieben Königslande.&lt;/p&gt;&lt;p&gt;Ein Krieg.&lt;/p&gt;</dc:description>
  </metadata>
  <manifest><item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="one"/></spine>
</package>`

func TestBackfillReadsTheDescriptionOfStoredBooks(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := storeUndescribedBook(t, app, testutil.PadId("booka"), alice.Id, seriesEPUB(t, blurbPackage))
	if book.GetString(schema.FieldDescription) != "" {
		t.Fatal("the book already had a description, the fixture proves nothing")
	}

	if err := migrations.BackfillDescriptions(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	stored := reload(t, app, book.Id)
	if got := stored.GetString(schema.FieldDescription); got != "Sieben Königslande.\n\nEin Krieg." {
		t.Errorf("description is %q", got)
	}
}

// A book whose file says nothing about itself is left alone rather than given an
// empty string to display.
func TestBackfillLeavesABookWithNoDescriptionAlone(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := storeUndescribedBook(t, app, testutil.PadId("booka"), alice.Id, seriesEPUB(t, seriesPackage))

	if err := migrations.BackfillDescriptions(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	if got := reload(t, app, book.Id).GetString(schema.FieldDescription); got != "" {
		t.Errorf("description is %q, want empty", got)
	}
}

// The blurb is the publisher's and the column is the owner's. A description
// somebody typed is not overwritten by one the file happens to carry.
func TestBackfillKeepsADescriptionThatIsAlreadyThere(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := storeUndescribedBook(t, app, testutil.PadId("booka"), alice.Id, seriesEPUB(t, blurbPackage))
	book.Set(schema.FieldDescription, "Was tatsächlich passiert.")
	if err := app.Save(book); err != nil {
		t.Fatalf("save the typed description: %v", err)
	}

	if err := migrations.BackfillDescriptions(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	if got := reload(t, app, book.Id).GetString(schema.FieldDescription); got != "Was tatsächlich passiert." {
		t.Errorf("description is %q, want the typed one", got)
	}
}

// The library is re-read in place, so the books have to come out of it with the
// same modification time they went in with.
func TestBackfillDescriptionsDoesNotTouchTheTimestamps(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := storeUndescribedBook(t, app, testutil.PadId("booka"), alice.Id, seriesEPUB(t, blurbPackage))
	before := book.GetDateTime(schema.FieldUpdated).String()

	if err := migrations.BackfillDescriptions(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	if got := reload(t, app, book.Id).GetDateTime(schema.FieldUpdated).String(); got != before {
		t.Errorf("updated moved from %v to %v", before, got)
	}
}

// A record whose file has gone missing, or was never an EPUB, must not stop a
// server from starting.
func TestBackfillDescriptionsSurvivesAnUnreadableBook(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	broken := storeUndescribedBook(t, app, testutil.PadId("bookb"), alice.Id, []byte("PK\x03\x04 not really"))
	good := storeUndescribedBook(t, app, testutil.PadId("booka"), alice.Id, seriesEPUB(t, blurbPackage))

	if err := migrations.BackfillDescriptions(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	if reload(t, app, good.Id).GetString(schema.FieldDescription) == "" {
		t.Error("the readable book was skipped along with the broken one")
	}
	if got := reload(t, app, broken.Id).GetString(schema.FieldDescription); got != "" {
		t.Errorf("the broken book was given the description %q out of nowhere", got)
	}
}

// The column has to hold what the reader is willing to extract, or a book with a
// long blurb fails to save at all.
func TestTheColumnHoldsTheLongestDescriptionTheReaderWillKeep(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	endless := strings.TrimSpace(strings.Repeat("wort ", 6000))
	pkg := strings.Replace(blurbPackage,
		"&lt;p&gt;Sieben Königslande.&lt;/p&gt;&lt;p&gt;Ein Krieg.&lt;/p&gt;", endless, 1)

	book := storeUndescribedBook(t, app, testutil.PadId("booka"), alice.Id, seriesEPUB(t, pkg))

	if err := migrations.BackfillDescriptions(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	stored := reload(t, app, book.Id)
	if got := stored.GetString(schema.FieldDescription); got == "" {
		t.Fatal("the long description was not stored at all")
	}

	// Saved through the record API as well, which is where the field's own
	// length check lives: the backfill writes SQL and would not have met it.
	stored.Set(schema.FieldTitle, "A Clash of Kings, corrected")
	if err := app.Save(stored); err != nil {
		t.Fatalf("the stored description is longer than the column allows: %v", err)
	}
}

// reload reads a book back out of the database.
func reload(t testing.TB, app core.App, id string) *core.Record {
	t.Helper()

	record, err := app.FindRecordById(schema.CollectionBooks, id)
	if err != nil {
		t.Fatalf("reload %s: %v", id, err)
	}

	return record
}
