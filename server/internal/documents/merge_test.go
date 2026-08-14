//
// File:        internal/documents/merge_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package documents_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/documents"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// base is the moment the fixtures are read at, so that "newer" and "older" in
// these tests mean something a reader would recognise.
var base = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// twoCopies is the case the merge exists for: one book read on two devices from
// two different files, so KOReader reports two hashes and the reading ends up
// split between two documents. The reader is at 40% on the newer one and left
// the older one at 7% two days ago.
func twoCopies(t testing.TB, app core.App) (owner, older, newer *core.Record) {
	t.Helper()

	owner = testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	older = testutil.CreateDocument(t, app, owner, "", "aaaa1111", 0.07, base.Add(-48*time.Hour))
	newer = testutil.CreateDocument(t, app, owner, "", "bbbb2222", 0.4, base)

	return owner, older, newer
}

// historyOf loads a document's archived states, oldest first.
func historyOf(t testing.TB, app core.App, documentId string) []*core.Record {
	t.Helper()

	records, err := app.FindRecordsByFilter(
		schema.CollectionDocumentHistory,
		"document_ref = {:document}",
		schema.FieldLastReadAt,
		0, 0,
		dbx.Params{"document": documentId},
	)
	if err != nil {
		t.Fatalf("failed to load the history of %q: %v", documentId, err)
	}

	return records
}

func TestMergeKeepsTheMostRecentPosition(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)

	// Merging into the older document: the one that survives is the caller's
	// choice, the position it carries is not.
	merged, err := documents.Merge(app, owner.Id, older.Id, []string{newer.Id})
	if err != nil {
		t.Fatalf("failed to merge: %v", err)
	}
	if merged != 1 {
		t.Errorf("expected 1 document to be folded in, got %d", merged)
	}

	survivor, err := app.FindRecordById(schema.CollectionDocuments, older.Id)
	if err != nil {
		t.Fatalf("failed to reload the surviving document: %v", err)
	}
	if got := survivor.GetFloat(schema.FieldProgress); got != 0.4 {
		t.Errorf("expected the more recent progress of 0.4, got %v", got)
	}
	if got := survivor.GetString(schema.FieldDocument); got != "aaaa1111" {
		t.Errorf("expected the survivor to keep its own hash, got %q", got)
	}

	if _, err := app.FindRecordById(schema.CollectionDocuments, newer.Id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected the merged document to be gone, got %v", err)
	}
}

func TestMergeArchivesEveryStateItReplaces(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)

	if _, err := documents.Merge(app, owner.Id, older.Id, []string{newer.Id}); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	// The documents are deleted outright, so what the merge did not archive is
	// gone for good. The survivor's own superseded position has to be there, and
	// the winner's must not be there twice.
	history := historyOf(t, app, older.Id)
	if len(history) != 1 {
		t.Fatalf("expected exactly one archived state, got %d", len(history))
	}
	if got := history[0].GetFloat(schema.FieldProgress); got != 0.07 {
		t.Errorf("expected the replaced position of 0.07 to be archived, got %v", got)
	}
}

func TestMergeArchivesTheStateOfEveryDocumentItFoldsIn(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)

	// This way round the survivor's own state is the newest and stays put, so
	// the only state at risk is the one being folded in.
	if _, err := documents.Merge(app, owner.Id, newer.Id, []string{older.Id}); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	survivor, err := app.FindRecordById(schema.CollectionDocuments, newer.Id)
	if err != nil {
		t.Fatalf("failed to reload the surviving document: %v", err)
	}
	if got := survivor.GetFloat(schema.FieldProgress); got != 0.4 {
		t.Errorf("expected the survivor to keep its own newer progress, got %v", got)
	}

	history := historyOf(t, app, newer.Id)
	if len(history) != 1 {
		t.Fatalf("expected the folded document's state to be archived, got %d entries", len(history))
	}
	if got := history[0].GetFloat(schema.FieldProgress); got != 0.07 {
		t.Errorf("expected the folded position of 0.07 to be archived, got %v", got)
	}
}

func TestMergeMovesTheHistoryAcross(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)

	testutil.CreateHistoryEntry(t, app, older, "", 0.01, base.Add(-72*time.Hour))
	testutil.CreateHistoryEntry(t, app, newer, "", 0.2, base.Add(-24*time.Hour))

	if _, err := documents.Merge(app, owner.Id, newer.Id, []string{older.Id}); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	// Two entries that were already there, plus the folded document's current
	// state, all in reading order under one document.
	history := historyOf(t, app, newer.Id)
	if len(history) != 3 {
		t.Fatalf("expected three archived states after the merge, got %d", len(history))
	}

	want := []float64{0.01, 0.07, 0.2}
	for index, entry := range history {
		if got := entry.GetFloat(schema.FieldProgress); got != want[index] {
			t.Errorf("entry %d: expected progress %v, got %v", index, want[index], got)
		}
	}
}

func TestMergeLeavesTheHashPointingAtTheSurvivor(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)

	if _, err := documents.Merge(app, owner.Id, newer.Id, []string{older.Id}); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	// This is what stops the device that reported the folded hash from
	// recreating it on its next push and undoing the merge.
	resolved, err := documents.Resolve(app, owner.Id, "aaaa1111")
	if err != nil {
		t.Fatalf("failed to resolve the retired hash: %v", err)
	}
	if resolved.Id != newer.Id {
		t.Errorf("expected the retired hash to resolve to the survivor, got %q", resolved.Id)
	}
}

func TestDeletingAnAliasSeparatesTheDocumentsAgain(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)

	if _, err := documents.Merge(app, owner.Id, newer.Id, []string{older.Id}); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	aliases, err := app.FindAllRecords(schema.CollectionDocumentAliases,
		dbx.HashExp{schema.FieldDocument: "aaaa1111"})
	if err != nil {
		t.Fatalf("failed to load the aliases: %v", err)
	}
	if len(aliases) != 1 {
		t.Fatalf("expected one alias for the retired hash, got %d", len(aliases))
	}
	if err := app.Delete(aliases[0]); err != nil {
		t.Fatalf("failed to delete the alias: %v", err)
	}

	// The way back out of an unwanted merge: the hash means nothing again, so
	// the next push from that device makes a document of its own.
	if _, err := documents.Resolve(app, owner.Id, "aaaa1111"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected the retired hash to mean nothing again, got %v", err)
	}
}

func TestMergingTheSurvivorAgainKeepsTheOlderAliases(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)
	third := testutil.CreateDocument(t, app, owner, "", "cccc3333", 0.5, base.Add(time.Hour))

	if _, err := documents.Merge(app, owner.Id, newer.Id, []string{older.Id}); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}
	// The survivor of the first merge is folded into a third document. The hash
	// that was already retired has to follow, or the first device falls off.
	if _, err := documents.Merge(app, owner.Id, third.Id, []string{newer.Id}); err != nil {
		t.Fatalf("failed to merge again: %v", err)
	}

	for _, hash := range []string{"aaaa1111", "bbbb2222"} {
		resolved, err := documents.Resolve(app, owner.Id, hash)
		if err != nil {
			t.Fatalf("failed to resolve %q: %v", hash, err)
		}
		if resolved.Id != third.Id {
			t.Errorf("expected %q to resolve to the last survivor, got %q", hash, resolved.Id)
		}
	}
}

func TestMergeTakesABookOnlyWhereThereIsNone(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)

	book := testutil.CreateBook(t, app, owner, "", "Metro 2033", "hashbinary", "hashfilename")
	newer.Set(schema.FieldBook, book.Id)
	if err := app.Save(newer); err != nil {
		t.Fatalf("failed to link the book: %v", err)
	}

	// The unmatched document is the one being kept, so the match comes with the
	// reading rather than being lost with the document that carried it.
	if _, err := documents.Merge(app, owner.Id, older.Id, []string{newer.Id}); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	survivor, err := app.FindRecordById(schema.CollectionDocuments, older.Id)
	if err != nil {
		t.Fatalf("failed to reload the surviving document: %v", err)
	}
	if got := survivor.GetString(schema.FieldBook); got != book.Id {
		t.Errorf("expected the survivor to inherit the book, got %q", got)
	}
}

func TestMergeKeepsTheSurvivorsOwnBook(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)

	kept := testutil.CreateBook(t, app, owner, "", "Metro 2033", "keptbinary", "keptfilename")
	other := testutil.CreateBook(t, app, owner, "", "Metro 2034", "otherbinary", "otherfilename")
	older.Set(schema.FieldBook, kept.Id)
	newer.Set(schema.FieldBook, other.Id)
	if err := app.Save(older); err != nil {
		t.Fatalf("failed to link the kept book: %v", err)
	}
	if err := app.Save(newer); err != nil {
		t.Fatalf("failed to link the other book: %v", err)
	}

	if _, err := documents.Merge(app, owner.Id, older.Id, []string{newer.Id}); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	survivor, err := app.FindRecordById(schema.CollectionDocuments, older.Id)
	if err != nil {
		t.Fatalf("failed to reload the surviving document: %v", err)
	}
	// Merging must not silently relabel the document the caller chose to keep.
	if got := survivor.GetString(schema.FieldBook); got != kept.Id {
		t.Errorf("expected the survivor to keep its own book, got %q", got)
	}
}

func TestMergeQueuesTheDaysItTouched(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)

	testutil.CreateHistoryEntry(t, app, older, "", 0.01, base.Add(-72*time.Hour))

	if _, err := documents.Merge(app, owner.Id, newer.Id, []string{older.Id}); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	queued, err := app.FindAllRecords(schema.CollectionAnalyticsQueue,
		dbx.HashExp{schema.FieldOwner: owner.Id})
	if err != nil {
		t.Fatalf("failed to load the analytics queue: %v", err)
	}

	// Reading that was attributed to nothing is now attributed to the survivor's
	// book, so every day it happened on has to be worked out again.
	days := map[string]bool{}
	for _, item := range queued {
		days[item.GetString(schema.FieldDate)] = true
	}
	for _, want := range []string{"2026-02-26", "2026-02-27", "2026-03-01"} {
		if !days[want] {
			t.Errorf("expected %s to be queued for recomputation, queue holds %v", want, days)
		}
	}
}

func TestMergeRefusesSomebodyElsesDocument(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, _ := twoCopies(t, app)

	stranger := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)
	theirs := testutil.CreateDocument(t, app, stranger, "", "dddd4444", 0.9, base)

	if _, err := documents.Merge(app, owner.Id, older.Id, []string{theirs.Id}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected a foreign document to be reported as missing, got %v", err)
	}

	// A refused merge leaves nothing half done.
	if _, err := app.FindRecordById(schema.CollectionDocuments, theirs.Id); err != nil {
		t.Errorf("the foreign document was touched: %v", err)
	}
	if entries := historyOf(t, app, older.Id); len(entries) != 0 {
		t.Errorf("expected the failed merge to archive nothing, got %d entries", len(entries))
	}
}

func TestMergeRefusesADocumentIntoItself(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, _ := twoCopies(t, app)

	if _, err := documents.Merge(app, owner.Id, older.Id, []string{older.Id}); !errors.Is(err, documents.ErrNothingToMerge) {
		t.Fatalf("expected ErrNothingToMerge, got %v", err)
	}
}

// A named document merged into an unnamed one keeps the name: the title is the
// only thing that tells the two apart in a list, and losing it would make the
// merge look like a deletion.
func TestMergeFillsInAMissingTitle(t *testing.T) {
	app := testutil.NewApp(t)
	owner, older, newer := twoCopies(t, app)

	older.Set(schema.FieldTitle, "Metro Trilogie")
	if err := app.Save(older); err != nil {
		t.Fatalf("failed to set the title: %v", err)
	}

	if _, err := documents.Merge(app, owner.Id, newer.Id, []string{older.Id}); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	survivor, err := app.FindRecordById(schema.CollectionDocuments, newer.Id)
	if err != nil {
		t.Fatalf("failed to reload the surviving document: %v", err)
	}
	if got := survivor.GetString(schema.FieldTitle); got != "Metro Trilogie" {
		t.Errorf("expected the survivor to inherit the title, got %q", got)
	}
}
