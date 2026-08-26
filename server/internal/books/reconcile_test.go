//
// File:        internal/books/reconcile_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books_test

import (
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// unlink clears a document's book behind the hooks' back, which is the state a
// failed match leaves behind and the only way to reach it deliberately.
func unlink(t testing.TB, app core.App, documentId string) {
	t.Helper()

	_, err := app.DB().
		NewQuery(`UPDATE {{` + schema.CollectionDocuments + `}} SET [[` + schema.FieldBook + `]] = '' WHERE [[id]] = {:id}`).
		Bind(dbx.Params{"id": documentId}).
		Execute()
	if err != nil {
		t.Fatalf("unlink the document: %v", err)
	}
}

// bookOf returns the book a stored document is linked to.
func bookOf(t testing.TB, app core.App, documentId string) string {
	t.Helper()

	stored, err := app.FindRecordById(schema.CollectionDocuments, documentId)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}

	return stored.GetString(schema.FieldBook)
}

func TestReconcileLinksADocumentAMatchWasMissedFor(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	book := createBook(t, app, testutil.PadId("booka"), user.Id, zeitDesSturms, "0123456789abcdef0123456789abcdef")

	document := testutil.CreateDocument(t, app, user, "", zeitDesSturms, 0.25, time.Now())
	unlink(t, app, document.Id)

	linked, err := books.Reconcile(app)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if linked != 1 {
		t.Fatalf("linked %d documents, want 1", linked)
	}
	if got := bookOf(t, app, document.Id); got != book.Id {
		t.Errorf("document is linked to %q, want %q", got, book.Id)
	}
}

// The reported filename is the second hash a document can be recognised by, and
// the reconcile has to try it for the same reason the hooks do.
func TestReconcileTriesTheReportedFilenameToo(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	filenameHash := "915bd5f6d29f6038e88dad85acaf8958"
	book := createBook(t, app, testutil.PadId("booka"), user.Id, zeitDesSturms, filenameHash)

	// A document identified by a hash no book carries, which later reported a
	// filename that hashes to one that does.
	document := testutil.CreateDocument(t, app, user, "", "ffffffffffffffffffffffffffffffff", 0.4, time.Now())
	document.Set(schema.FieldFilenameHash, filenameHash)
	if err := app.Save(document); err != nil {
		t.Fatalf("set the filename hash: %v", err)
	}
	unlink(t, app, document.Id)

	if _, err := books.Reconcile(app); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := bookOf(t, app, document.Id); got != book.Id {
		t.Errorf("document is linked to %q, want %q", got, book.Id)
	}
}

// A book nobody has a catalog name for and a document no device has reported a
// filename for both carry an empty string, and two empty strings are equal.
// That is a match a naive join would make and a wrong one.
func TestReconcileDoesNotMatchOnEmptyHashes(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)

	book := createBook(t, app, testutil.PadId("booka"), user.Id, zeitDesSturms, "")
	// Written directly: the create hook fills the catalog hash in from the title.
	_, err := app.DB().
		NewQuery(`UPDATE {{` + schema.CollectionBooks + `}}
			SET [[` + schema.FieldHashFilename + `]] = '', [[` + schema.FieldHashCatalog + `]] = ''
			WHERE [[id]] = {:id}`).
		Bind(dbx.Params{"id": book.Id}).
		Execute()
	if err != nil {
		t.Fatalf("empty the hashes: %v", err)
	}

	document := testutil.CreateDocument(t, app, user, "", "ffffffffffffffffffffffffffffffff", 0.4, time.Now())

	linked, err := books.Reconcile(app)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if linked != 0 {
		t.Errorf("linked %d documents, want none", linked)
	}
	if got := bookOf(t, app, document.Id); got != "" {
		t.Errorf("document is linked to %q, want nothing", got)
	}
}

// One account's book is not another's, however identical the file.
func TestReconcileStaysWithinTheOwner(t *testing.T) {
	app := matchingApp(t)
	owner := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	other := testutil.CreateUser(t, app, testutil.IdUserB, testutil.EmailUserB, testutil.PasswordUsers)

	createBook(t, app, testutil.PadId("booka"), other.Id, zeitDesSturms, "0123456789abcdef0123456789abcdef")
	document := testutil.CreateDocument(t, app, owner, "", zeitDesSturms, 0.25, time.Now())

	linked, err := books.Reconcile(app)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if linked != 0 {
		t.Errorf("linked %d documents, want none", linked)
	}
	if got := bookOf(t, app, document.Id); got != "" {
		t.Errorf("document is linked to %q, want nothing", got)
	}
}

// The ordinary night: everything is already linked and the run does nothing.
func TestReconcileLeavesLinkedDocumentsAlone(t *testing.T) {
	app := matchingApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	book := createBook(t, app, testutil.PadId("booka"), user.Id, zeitDesSturms, "0123456789abcdef0123456789abcdef")
	document := testutil.CreateDocument(t, app, user, "", zeitDesSturms, 0.25, time.Now())

	before, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}

	linked, err := books.Reconcile(app)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if linked != 0 {
		t.Errorf("linked %d documents, want none", linked)
	}

	after, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if after.GetString(schema.FieldBook) != book.Id {
		t.Errorf("the link was lost")
	}
	// Nothing was written, so nothing was touched: a run that saved every
	// already-linked document would requeue every day of every book, nightly.
	if !after.GetDateTime(schema.FieldUpdated).Equal(before.GetDateTime(schema.FieldUpdated)) {
		t.Errorf("the document was rewritten by a reconcile that had nothing to do")
	}
}
