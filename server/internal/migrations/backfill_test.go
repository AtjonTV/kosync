//
// File:        internal/migrations/backfill_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations_test

import (
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/migrations"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// storeBook writes a book straight to the collection. The matching hooks are
// not registered here, which is exactly the situation the backfill exists for:
// records that came into being without anything linking them.
func storeBook(t testing.TB, app core.App, id, owner, binary, filename string) *core.Record {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionBooks)
	if err != nil {
		t.Fatalf("find books collection: %v", err)
	}

	file, err := filesystem.NewFileFromBytes([]byte("PK\x03\x04 stand-in"), "book.epub")
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

// An instance that uploaded books before matching existed has documents and
// books that no hook will ever bring together, because both hooks only fire on
// records being created.
func TestBackfillLinksPreExistingPairs(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	bob := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)

	const binaryHash = "043f11771ef9d191364ac0ba08198d36"
	const filenameHash = "915bd5f6d29f6038e88dad85acaf8958"

	book := storeBook(t, app, testutil.PadId("booka"), alice.Id, binaryHash, filenameHash)
	other := storeBook(t, app, testutil.PadId("bookb"), bob.Id, binaryHash, "")

	onBinary := testutil.CreateDocument(t, app, alice, testutil.PadId("docaa"), binaryHash, 0.63, time.Now())
	onFilename := testutil.CreateDocument(t, app, alice, testutil.PadId("docab"), filenameHash, 0.20, time.Now())
	unrelated := testutil.CreateDocument(t, app, alice, testutil.PadId("docac"),
		"ffffffffffffffffffffffffffffffff", 0.10, time.Now())
	bobsOwn := testutil.CreateDocument(t, app, bob, testutil.PadId("docba"), binaryHash, 0.44, time.Now())

	for _, document := range []*core.Record{onBinary, onFilename, unrelated, bobsOwn} {
		stored, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
		if err != nil {
			t.Fatalf("reload %s: %v", document.Id, err)
		}
		if stored.GetString(schema.FieldBook) != "" {
			t.Fatalf("%s was already linked, the fixture proves nothing", document.Id)
		}
	}

	if err := migrations.BackfillDocumentBook(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	expected := map[string]string{
		onBinary.Id:   book.Id,
		onFilename.Id: book.Id,
		unrelated.Id:  "",
		bobsOwn.Id:    other.Id,
	}

	for id, want := range expected {
		stored, err := app.FindRecordById(schema.CollectionDocuments, id)
		if err != nil {
			t.Fatalf("reload %s: %v", id, err)
		}
		if got := stored.GetString(schema.FieldBook); got != want {
			t.Errorf("%s is linked to %q, want %q", id, got, want)
		}
	}
}

// Running it twice must not disturb links that are already right.
func TestBackfillIsIdempotent(t *testing.T) {
	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	const hash = "043f11771ef9d191364ac0ba08198d36"
	book := storeBook(t, app, testutil.PadId("booka"), user.Id, hash, "")
	document := testutil.CreateDocument(t, app, user, testutil.PadId("docaa"), hash, 0.5, time.Now())

	for range 2 {
		if err := migrations.BackfillDocumentBook(app); err != nil {
			t.Fatalf("backfill: %v", err)
		}
	}

	stored, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if got := stored.GetString(schema.FieldBook); got != book.Id {
		t.Errorf("document is linked to %q, want %q", got, book.Id)
	}
}

// A book with no filename hash stores an empty string there. That must not
// match anything, and documents always carry a real hash.
func TestBackfillIgnoresEmptyHashes(t *testing.T) {
	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	storeBook(t, app, testutil.PadId("booka"), user.Id, "", "")
	document := testutil.CreateDocument(t, app, user, testutil.PadId("docaa"),
		"043f11771ef9d191364ac0ba08198d36", 0.5, time.Now())

	if err := migrations.BackfillDocumentBook(app); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	stored, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if got := stored.GetString(schema.FieldBook); got != "" {
		t.Errorf("a book with no hashes matched: %q", got)
	}
}

// An instance upgrading into book statistics has days on record that nothing
// will ever recompute, so every one of them would report zero pages read.
func TestQueueStoredDaysEnqueuesEveryDayOnRecord(t *testing.T) {
	app := testutil.NewApp(t)
	alice := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	bob := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)

	storeReadingDay(t, app, alice.Id, "2026-01-05")
	storeReadingDay(t, app, alice.Id, "2026-02-11")
	storeReadingDay(t, app, bob.Id, "2026-01-05")

	if err := migrations.QueueStoredDays(app); err != nil {
		t.Fatalf("queue the stored days: %v", err)
	}

	queued, err := app.FindAllRecords(schema.CollectionAnalyticsQueue)
	if err != nil {
		t.Fatalf("list the queue: %v", err)
	}
	if len(queued) != 3 {
		t.Fatalf("expected one queue item per stored day, got %d", len(queued))
	}

	for _, item := range queued {
		if len(item.Id) != 15 {
			t.Errorf("generated id %q is not a valid record id", item.Id)
		}
	}
}

// It runs on every deployment that has not run it yet, and the unique index on
// the queue is what makes that harmless.
func TestQueueStoredDaysIsIdempotent(t *testing.T) {
	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	storeReadingDay(t, app, user.Id, "2026-01-05")

	for range 2 {
		if err := migrations.QueueStoredDays(app); err != nil {
			t.Fatalf("queue the stored days: %v", err)
		}
	}

	queued, err := app.FindAllRecords(schema.CollectionAnalyticsQueue)
	if err != nil {
		t.Fatalf("list the queue: %v", err)
	}
	if len(queued) != 1 {
		t.Errorf("expected the day to be queued once, got %d items", len(queued))
	}
}

// storeReadingDay writes a statistics day the way the worker would have.
func storeReadingDay(t testing.TB, app core.App, owner, date string) {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("find the reading_days collection: %v", err)
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldOwner, owner)
	record.Set(schema.FieldDate, date)
	record.Set(schema.FieldUpdateCount, 3)

	if err := app.Save(record); err != nil {
		t.Fatalf("store the day %q: %v", date, err)
	}
}
