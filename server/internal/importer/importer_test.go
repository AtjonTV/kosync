//
// File:        internal/importer/importer_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package importer_test

import (
	"io"
	"path/filepath"
	"testing"
	"time"

	"git.obth.eu/atjontv/kosync/internal/analytics"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/importer"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/dbx"
)

// legacySchema is the schema of a fully migrated legacy KOsync database,
// copied from its migration files so a change there shows up as a failure here.
const legacySchema = `
CREATE TABLE schema_versions (version INTEGER PRIMARY KEY, installed_at INTEGER NOT NULL);

CREATE TABLE users (
	id TEXT PRIMARY KEY,
	username TEXT UNIQUE,
	password TEXT,
	created_at INTEGER NOT NULL,
	updated_at INTEGER,
	deleted_at INTEGER
);

CREATE TABLE documents (
	id TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	title TEXT,
	current_location TEXT,
	progress FLOAT,
	last_read_on_device TEXT,
	last_read_on_device_id TEXT,
	last_read_at INTEGER,
	created_at INTEGER NOT NULL,
	updated_at INTEGER,
	deleted_at INTEGER,
	PRIMARY KEY (id, owner_id)
);

CREATE TABLE document_history (
	document_id TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	title TEXT,
	current_location TEXT,
	progress FLOAT,
	last_read_on_device TEXT,
	last_read_on_device_id TEXT,
	last_read_at INTEGER,
	created_at INTEGER,
	updated_at INTEGER,
	deleted_at INTEGER
);
`

// legacyUnit converts a time into the legacy 1/10000 second timestamp.
func legacyUnit(moment time.Time) int64 {
	return moment.UnixMicro() / 100
}

// legacyFixture writes a legacy database and returns its path.
func legacyFixture(t *testing.T, build func(db *dbx.DB)) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "kosync.db")

	db, err := dbx.Open("sqlite", path)
	if err != nil {
		t.Fatalf("failed to create the legacy fixture: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	if _, err := db.NewQuery(legacySchema).Execute(); err != nil {
		t.Fatalf("failed to create the legacy schema: %v", err)
	}

	build(db)

	return path
}

// insertLegacyUser adds a user to the fixture.
func insertLegacyUser(t *testing.T, db *dbx.DB, id, username, md5password string) {
	t.Helper()

	_, err := db.Insert("users", dbx.Params{
		"id":         id,
		"username":   username,
		"password":   md5password,
		"created_at": legacyUnit(time.Now()),
	}).Execute()
	if err != nil {
		t.Fatalf("failed to insert the legacy user %q: %v", username, err)
	}
}

// insertLegacyDocument adds a document to the fixture.
func insertLegacyDocument(t *testing.T, db *dbx.DB, id, ownerId, title string, progress float64, lastReadAt time.Time, deleted bool) {
	t.Helper()

	params := dbx.Params{
		"id":                     id,
		"owner_id":               ownerId,
		"title":                  title,
		"current_location":       "/body/DocFragment[1]",
		"progress":               progress,
		"last_read_on_device":    "Kobo",
		"last_read_on_device_id": "AAA",
		"last_read_at":           legacyUnit(lastReadAt),
		"created_at":             legacyUnit(lastReadAt),
	}
	if deleted {
		params["deleted_at"] = legacyUnit(time.Now())
	}

	if _, err := db.Insert("documents", params).Execute(); err != nil {
		t.Fatalf("failed to insert the legacy document %q: %v", id, err)
	}
}

// insertLegacyHistory adds a history entry to the fixture.
func insertLegacyHistory(t *testing.T, db *dbx.DB, documentId, ownerId string, progress float64, lastReadAt time.Time) {
	t.Helper()

	_, err := db.Insert("document_history", dbx.Params{
		"document_id":            documentId,
		"owner_id":               ownerId,
		"title":                  "",
		"current_location":       "/body",
		"progress":               progress,
		"last_read_on_device":    "Kobo",
		"last_read_on_device_id": "AAA",
		"last_read_at":           legacyUnit(lastReadAt),
		"created_at":             legacyUnit(lastReadAt),
	}).Execute()
	if err != nil {
		t.Fatalf("failed to insert the legacy history of %q: %v", documentId, err)
	}
}

// standardFixture is one legacy user with one document and two earlier states.
func standardFixture(t *testing.T) string {
	t.Helper()

	readAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	return legacyFixture(t, func(db *dbx.DB) {
		insertLegacyUser(t, db, "legacy-user-1", "alice", testutil.Md5Hex("device-secret"))
		insertLegacyDocument(t, db, "hash-a", "legacy-user-1", "Some Book", 0.42, readAt, false)
		insertLegacyHistory(t, db, "hash-a", "legacy-user-1", 0.1, readAt.Add(-2*time.Hour))
		insertLegacyHistory(t, db, "hash-a", "legacy-user-1", 0.2, readAt.Add(-time.Hour))
	})
}

func TestImportCreatesAccountsCredentialsAndProgress(t *testing.T) {
	app := testutil.NewApp(t)
	file := standardFixture(t)

	report, err := importer.Run(app, importer.Options{File: file})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if report.Credentials != 1 || report.Documents != 1 || report.History != 2 {
		t.Errorf("unexpected report: %+v", report)
	}
	if len(report.Accounts) != 1 {
		t.Fatalf("expected one generated account, got %d", len(report.Accounts))
	}
	if report.Accounts[0].LegacyUsername != "alice" {
		t.Errorf("expected the generated account to name its legacy user")
	}

	// The generated password has to actually work, otherwise the user is locked
	// out of the account that owns their reading history.
	user, err := app.FindAuthRecordByEmail(schema.CollectionUsers, report.Accounts[0].Email)
	if err != nil {
		t.Fatalf("failed to load the generated account: %v", err)
	}
	if !user.ValidatePassword(report.Accounts[0].Password) {
		t.Errorf("the printed password does not open the generated account")
	}
}

func TestImportedCredentialKeepsWorkingOnTheDevice(t *testing.T) {
	app := testutil.NewApp(t)
	file := standardFixture(t)

	if _, err := importer.Run(app, importer.Options{File: file}); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	account, err := app.FindFirstRecordByData(schema.CollectionKoreaderAccounts, schema.FieldUsername, "alice")
	if err != nil {
		t.Fatalf("expected the credential to be imported: %v", err)
	}

	// This is the promise of the import: nobody has to touch their devices.
	if !account.ValidatePassword(testutil.Md5Hex("device-secret")) {
		t.Errorf("the imported credential does not accept the password the device still sends")
	}
}

func TestImportConvertsTheLegacyTimestamps(t *testing.T) {
	app := testutil.NewApp(t)
	readAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	file := legacyFixture(t, func(db *dbx.DB) {
		insertLegacyUser(t, db, "legacy-user-1", "alice", testutil.Md5Hex("device-secret"))
		insertLegacyDocument(t, db, "hash-a", "legacy-user-1", "Some Book", 0.42, readAt, false)
	})

	if _, err := importer.Run(app, importer.Options{File: file}); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	document, err := app.FindFirstRecordByData(schema.CollectionDocuments, schema.FieldDocument, "hash-a")
	if err != nil {
		t.Fatalf("expected the document to be imported: %v", err)
	}

	got := document.GetDateTime(schema.FieldLastReadAt).Time().UTC()
	if !got.Equal(readAt) {
		t.Errorf("expected %v, got %v", readAt, got)
	}
	if document.GetFloat(schema.FieldProgress) != 0.42 {
		t.Errorf("expected the progress to survive, got %v", document.GetFloat(schema.FieldProgress))
	}
	if document.GetString(schema.FieldTitle) != "Some Book" {
		t.Errorf("expected the title to survive, got %q", document.GetString(schema.FieldTitle))
	}
}

func TestImportAttachesEverythingToOneAccountWhenAsked(t *testing.T) {
	app := testutil.NewApp(t)
	owner := testutil.CreateUser(t, app, "", "owner@example.com", "a-long-enough-password")
	file := standardFixture(t)

	report, err := importer.Run(app, importer.Options{File: file, OwnerEmail: "owner@example.com"})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if len(report.Accounts) != 0 {
		t.Errorf("expected no accounts to be generated, got %d", len(report.Accounts))
	}

	account, err := app.FindFirstRecordByData(schema.CollectionKoreaderAccounts, schema.FieldUsername, "alice")
	if err != nil {
		t.Fatalf("expected the credential to be imported: %v", err)
	}
	if account.GetString(schema.FieldOwner) != owner.Id {
		t.Errorf("expected the credential to belong to the named account")
	}

	document, err := app.FindFirstRecordByData(schema.CollectionDocuments, schema.FieldDocument, "hash-a")
	if err != nil {
		t.Fatalf("expected the document to be imported: %v", err)
	}
	if document.GetString(schema.FieldOwner) != owner.Id {
		t.Errorf("expected the document to belong to the named account")
	}
}

func TestImportFailsOnAnUnknownOwnerEmail(t *testing.T) {
	app := testutil.NewApp(t)
	file := standardFixture(t)

	if _, err := importer.Run(app, importer.Options{File: file, OwnerEmail: "nobody@example.com"}); err == nil {
		t.Errorf("expected the import to refuse an account that does not exist")
	}
}

func TestImportSkipsDeletedRowsUnlessAsked(t *testing.T) {
	readAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	build := func(db *dbx.DB) {
		insertLegacyUser(t, db, "legacy-user-1", "alice", testutil.Md5Hex("device-secret"))
		insertLegacyDocument(t, db, "hash-a", "legacy-user-1", "Kept", 0.42, readAt, false)
		insertLegacyDocument(t, db, "hash-b", "legacy-user-1", "Deleted", 0.1, readAt, true)
	}

	app := testutil.NewApp(t)
	report, err := importer.Run(app, importer.Options{File: legacyFixture(t, build)})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if report.Documents != 1 {
		t.Errorf("expected the deleted document to be skipped, imported %d", report.Documents)
	}

	other := testutil.NewApp(t)
	report, err = importer.Run(other, importer.Options{File: legacyFixture(t, build), IncludeDeleted: true})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if report.Documents != 2 {
		t.Errorf("expected the deleted document to be included, imported %d", report.Documents)
	}
}

func TestImportIsIdempotent(t *testing.T) {
	app := testutil.NewApp(t)
	file := standardFixture(t)

	if _, err := importer.Run(app, importer.Options{File: file}); err != nil {
		t.Fatalf("first import failed: %v", err)
	}

	report, err := importer.Run(app, importer.Options{File: file})
	if err != nil {
		t.Fatalf("second import failed: %v", err)
	}

	if report.Credentials != 0 || report.Documents != 0 {
		t.Errorf("expected the second run to import nothing, got %+v", report)
	}
	if len(report.Skipped) == 0 {
		t.Errorf("expected the second run to report what it skipped")
	}

	documents, err := app.FindAllRecords(schema.CollectionDocuments)
	if err != nil {
		t.Fatalf("failed to list the documents: %v", err)
	}
	if len(documents) != 1 {
		t.Errorf("expected the document not to be duplicated, got %d", len(documents))
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	app := testutil.NewApp(t)
	file := standardFixture(t)

	report, err := importer.Run(app, importer.Options{File: file, DryRun: true})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if report.Credentials != 1 || report.Documents != 1 || report.History != 2 {
		t.Errorf("expected the dry run to report the real numbers, got %+v", report)
	}

	for _, collection := range []string{
		schema.CollectionKoreaderAccounts,
		schema.CollectionDocuments,
		schema.CollectionDocumentHistory,
	} {
		records, err := app.FindAllRecords(collection)
		if err != nil {
			t.Fatalf("failed to list %q: %v", collection, err)
		}
		if len(records) != 0 {
			t.Errorf("expected %q to stay empty after a dry run, got %d records", collection, len(records))
		}
	}
}

func TestImportRefusesSomethingThatIsNotAKosyncDatabase(t *testing.T) {
	app := testutil.NewApp(t)

	path := filepath.Join(t.TempDir(), "not-kosync.db")
	db, err := dbx.Open("sqlite", path)
	if err != nil {
		t.Fatalf("failed to create the fixture: %v", err)
	}
	if _, err := db.NewQuery("CREATE TABLE something (id TEXT)").Execute(); err != nil {
		t.Fatalf("failed to create the fixture: %v", err)
	}
	_ = db.Close()

	if _, err := importer.Run(app, importer.Options{File: path}); err == nil {
		t.Errorf("expected the import to refuse an unrelated database")
	}
}

func TestImportGeneratesDistinctAddressesForSimilarUsernames(t *testing.T) {
	app := testutil.NewApp(t)

	file := legacyFixture(t, func(db *dbx.DB) {
		insertLegacyUser(t, db, "u1", "Alice", testutil.Md5Hex("one"))
		insertLegacyUser(t, db, "u2", "alice", testutil.Md5Hex("two"))
		insertLegacyUser(t, db, "u3", "a lice!", testutil.Md5Hex("three"))
	})

	report, err := importer.Run(app, importer.Options{File: file})
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if len(report.Accounts) != 3 {
		t.Fatalf("expected three accounts, got %d", len(report.Accounts))
	}

	// The exact addresses, not merely three different ones: the numbering walks
	// a candidate at a time and every step of it has to be one that was actually
	// asked about, or two legacy users end up sharing an account.
	want := map[string]bool{
		"alice@invalid.local":   true,
		"alice-2@invalid.local": true,
		"a-lice@invalid.local":  true,
	}

	seen := map[string]bool{}
	for _, account := range report.Accounts {
		if seen[account.Email] {
			t.Errorf("the address %q was handed out twice", account.Email)
		}
		if !want[account.Email] {
			t.Errorf("unexpected address %q", account.Email)
		}
		seen[account.Email] = true
	}
}

func TestImportedProgressProducesStatistics(t *testing.T) {
	app := testutil.NewApp(t)
	conf := &config.Config{}
	conf.Normalize()
	worker := analytics.Register(app, conf)

	file := standardFixture(t)
	if _, err := importer.Run(app, importer.Options{File: file}); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if _, err := worker.DrainAll(); err != nil {
		t.Fatalf("failed to compute the statistics: %v", err)
	}

	days, err := app.FindAllRecords(schema.CollectionReadingDays)
	if err != nil {
		t.Fatalf("failed to list the statistics: %v", err)
	}
	if len(days) != 1 {
		t.Fatalf("expected the imported day to be computed, got %d rows", len(days))
	}
	if got := days[0].GetString(schema.FieldDate); got != "2026-03-01" {
		t.Errorf("expected the statistics of 2026-03-01, got %q", got)
	}
	if got := days[0].GetInt(schema.FieldUpdateCount); got != 3 {
		t.Errorf("expected 3 progress moments, got %d", got)
	}
}

func TestImportReportsAProgressOutsideTheAllowedRange(t *testing.T) {
	app := testutil.NewApp(t)
	readAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	// The legacy schema had no bounds on the progress column.
	file := legacyFixture(t, func(db *dbx.DB) {
		insertLegacyUser(t, db, "legacy-user-1", "alice", testutil.Md5Hex("device-secret"))
		insertLegacyDocument(t, db, "hash-a", "legacy-user-1", "Broken", 1.5, readAt, false)
	})

	if _, err := importer.Run(app, importer.Options{File: file}); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	document, err := app.FindFirstRecordByData(schema.CollectionDocuments, schema.FieldDocument, "hash-a")
	if err != nil {
		t.Fatalf("expected the document to be imported: %v", err)
	}
	if got := document.GetFloat(schema.FieldProgress); got != 1 {
		t.Errorf("expected an out of range progress to be clamped to 1, got %v", got)
	}
}

func TestCommandMigratesAFreshDataDirectory(t *testing.T) {
	// Only "serve" applies the migrations by itself. Importing into a data
	// directory that was never served has to work all the same, which is how
	// most people will do their migration.
	app := testutil.NewUnmigratedApp(t)

	conf := &config.Config{}
	conf.Normalize()

	command := importer.NewCommand(app, conf)
	command.SetArgs([]string{"--file", standardFixture(t)})
	command.SetOut(io.Discard)

	if err := command.Execute(); err != nil {
		t.Fatalf("the import command failed on a fresh data directory: %v", err)
	}

	if _, err := app.FindFirstRecordByData(schema.CollectionKoreaderAccounts, schema.FieldUsername, "alice"); err != nil {
		t.Errorf("expected the credential to be imported: %v", err)
	}
}
