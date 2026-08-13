//
// File:        internal/migrations/migrations_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations_test

import (
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
)

func TestAllCollectionsAreCreated(t *testing.T) {
	app := testutil.NewApp(t)

	names := []string{
		schema.CollectionKoreaderAccounts,
		schema.CollectionDocuments,
		schema.CollectionDocumentHistory,
		schema.CollectionReadingDays,
		schema.CollectionReadingMonths,
		schema.CollectionAnalyticsQueue,
	}

	for _, name := range names {
		if _, err := app.FindCollectionByNameOrId(name); err != nil {
			t.Errorf("expected collection %q to exist: %v", name, err)
		}
	}
}

func TestUsersEmailIsRequired(t *testing.T) {
	app := testutil.NewApp(t)

	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		t.Fatalf("failed to find the users collection: %v", err)
	}

	field, ok := users.Fields.GetByName("email").(*core.EmailField)
	if !ok {
		t.Fatalf("expected the users collection to have an email field")
	}
	if !field.Required {
		t.Errorf("expected the users email field to be required")
	}
}

func TestKoreaderAccountsCannotAuthenticateThroughPocketbase(t *testing.T) {
	app := testutil.NewApp(t)

	collection, err := app.FindCollectionByNameOrId(schema.CollectionKoreaderAccounts)
	if err != nil {
		t.Fatalf("failed to find the koreader_accounts collection: %v", err)
	}

	if collection.AuthRule != nil {
		t.Errorf("expected AuthRule to be nil so device credentials cannot become API sessions")
	}
	if collection.PasswordAuth.Enabled {
		t.Errorf("expected password auth to be disabled")
	}
	if collection.OAuth2.Enabled {
		t.Errorf("expected OAuth2 to be disabled")
	}
	if collection.CreateRule != nil {
		t.Errorf("expected create to be superuser only, credentials are created through /api/kosync")
	}
}

func TestKoreaderAccountStoresTheMd5Hashed(t *testing.T) {
	app := testutil.NewApp(t)

	user := testutil.CreateUser(t, app, "", "reader@example.com", "a-long-enough-password")
	account := testutil.CreateKoreaderAccount(t, app, user, "", "reader", "device-secret")

	md5hex := testutil.Md5Hex("device-secret")

	if !account.ValidatePassword(md5hex) {
		t.Errorf("expected the stored credential to validate against the MD5 digest")
	}
	if account.ValidatePassword("device-secret") {
		t.Errorf("expected the plain password to be rejected, KOReader only ever sends the digest")
	}
	if got := account.GetString(schema.FieldPassword); got == md5hex {
		t.Errorf("expected the MD5 digest not to be readable back from the record")
	}
}

func TestKoreaderAccountUsernameIsUnique(t *testing.T) {
	app := testutil.NewApp(t)

	first := testutil.CreateUser(t, app, "", "first@example.com", "a-long-enough-password")
	second := testutil.CreateUser(t, app, "", "second@example.com", "a-long-enough-password")
	testutil.CreateKoreaderAccount(t, app, first, "", "shared", "secret-one")

	collection, err := app.FindCollectionByNameOrId(schema.CollectionKoreaderAccounts)
	if err != nil {
		t.Fatalf("failed to find the koreader_accounts collection: %v", err)
	}

	duplicate := core.NewRecord(collection)
	duplicate.Set(schema.FieldUsername, "shared")
	duplicate.Set(schema.FieldOwner, second.Id)
	duplicate.SetPassword(testutil.Md5Hex("secret-two"))

	if err := app.Save(duplicate); err == nil {
		t.Errorf("expected a duplicate KOReader username to be rejected")
	}
}

func TestDocumentIsUniquePerOwnerAndHash(t *testing.T) {
	app := testutil.NewApp(t)

	user := testutil.CreateUser(t, app, "", "reader@example.com", "a-long-enough-password")
	other := testutil.CreateUser(t, app, "", "other@example.com", "a-long-enough-password")
	now := time.Now()

	testutil.CreateDocument(t, app, user, "", "hash-a", 0.25, now)

	// The same hash for a different owner is a different book on a different shelf.
	testutil.CreateDocument(t, app, other, "", "hash-a", 0.5, now)

	collection, err := app.FindCollectionByNameOrId(schema.CollectionDocuments)
	if err != nil {
		t.Fatalf("failed to find the documents collection: %v", err)
	}

	duplicate := core.NewRecord(collection)
	duplicate.Set(schema.FieldOwner, user.Id)
	duplicate.Set(schema.FieldDocument, "hash-a")
	duplicate.Set(schema.FieldProgress, 0.9)
	duplicate.Set(schema.FieldLastReadAt, now)

	if err := app.Save(duplicate); err == nil {
		t.Errorf("expected a second document with the same hash and owner to be rejected")
	}
}

func TestProgressIsBoundToTheUnitInterval(t *testing.T) {
	app := testutil.NewApp(t)

	user := testutil.CreateUser(t, app, "", "reader@example.com", "a-long-enough-password")
	collection, err := app.FindCollectionByNameOrId(schema.CollectionDocuments)
	if err != nil {
		t.Fatalf("failed to find the documents collection: %v", err)
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldOwner, user.Id)
	record.Set(schema.FieldDocument, "hash-a")
	record.Set(schema.FieldProgress, 1.5)
	record.Set(schema.FieldLastReadAt, time.Now())

	if err := app.Save(record); err == nil {
		t.Errorf("expected a progress above 1.0 to be rejected")
	}
}

func TestDeletingAUserRemovesTheirData(t *testing.T) {
	app := testutil.NewApp(t)

	user := testutil.CreateUser(t, app, "", "reader@example.com", "a-long-enough-password")
	account := testutil.CreateKoreaderAccount(t, app, user, "", "reader", "device-secret")
	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.25, time.Now())
	testutil.CreateHistoryEntry(t, app, document, "", 0.1, time.Now().Add(-time.Hour))

	if err := app.Delete(user); err != nil {
		t.Fatalf("failed to delete the user: %v", err)
	}

	if _, err := app.FindRecordById(schema.CollectionKoreaderAccounts, account.Id); err == nil {
		t.Errorf("expected the KOReader credential to be deleted with its owner")
	}
	if _, err := app.FindRecordById(schema.CollectionDocuments, document.Id); err == nil {
		t.Errorf("expected the document to be deleted with its owner")
	}

	history, err := app.FindAllRecords(schema.CollectionDocumentHistory)
	if err != nil {
		t.Fatalf("failed to list the document history: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected the document history to be deleted with its owner, got %d entries", len(history))
	}
}

func TestDeletingACredentialKeepsTheProgressItPushed(t *testing.T) {
	app := testutil.NewApp(t)

	user := testutil.CreateUser(t, app, "", "reader@example.com", "a-long-enough-password")
	account := testutil.CreateKoreaderAccount(t, app, user, "", "reader", "device-secret")

	document := testutil.CreateDocument(t, app, user, "", "hash-a", 0.25, time.Now())
	document.Set(schema.FieldSourceAccount, account.Id)
	if err := app.Save(document); err != nil {
		t.Fatalf("failed to link the document to its source credential: %v", err)
	}

	if err := app.Delete(account); err != nil {
		t.Fatalf("failed to delete the KOReader credential: %v", err)
	}

	kept, err := app.FindRecordById(schema.CollectionDocuments, document.Id)
	if err != nil {
		t.Fatalf("expected the document to survive the deletion of its credential: %v", err)
	}
	if kept.GetString(schema.FieldSourceAccount) != "" {
		t.Errorf("expected the dangling source_account reference to be cleared")
	}
}
