//
// File:        internal/importer/importer.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package importer moves the data of a legacy KOsync server into this one.
//
// The legacy server stored everything in a single SQLite file with one table of
// users whose password was the MD5 digest KOReader sends. Here those users
// become KOReader credentials, and every credential needs an account to belong
// to, which is what most of the mapping below is about.
package importer

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
)

// legacyTimeUnit converts a legacy timestamp to milliseconds.
//
// The legacy schema counts 1/10000 of a second since the epoch (see its
// migration "103-unify_timestamps.sql"), PocketBase stores milliseconds.
const legacyTimeUnit = 10

// generatedEmailDomain is used for the accounts created for legacy users. They
// had no email address of their own, and the domain is reserved for exactly
// this purpose so no mail can accidentally leave the server.
const generatedEmailDomain = "invalid.local"

// maxEmailAttempts is how many legacy users may reduce to the same local part
// before the import gives up on finding an address for the next one. Two or
// three is already unusual; a hundred means the names differ only in characters
// the address cannot carry, and inventing a hundred and first would not help.
const maxEmailAttempts = 100

// unsafeEmailChars matches everything that may not appear in the local part of
// a generated address.
var unsafeEmailChars = regexp.MustCompile(`[^a-z0-9._-]`)

// Options configures an import run.
type Options struct {
	// File is the path to the legacy kosync.db.
	File string

	// OwnerEmail attaches everything to this existing account instead of
	// creating one account per legacy user.
	OwnerEmail string

	// IncludeDeleted also imports the rows the legacy server soft deleted.
	IncludeDeleted bool

	// DryRun reports what would happen without writing anything.
	DryRun bool
}

// CreatedAccount is an account that was created for a legacy user, together
// with the password that was generated for it.
type CreatedAccount struct {
	LegacyUsername string
	Email          string
	Password       string
}

// Report summarises an import run.
type Report struct {
	Accounts    []CreatedAccount
	Credentials int
	Documents   int
	History     int
	Skipped     []string
}

// legacyUser is a row of the legacy "users" table.
type legacyUser struct {
	Id       string `db:"id"`
	Username string `db:"username"`
	Password string `db:"password"`
}

// legacyDocument is a row of the legacy "documents" or "document_history" table.
type legacyDocument struct {
	DocumentId string  `db:"document_id"`
	OwnerId    string  `db:"owner_id"`
	Title      string  `db:"title"`
	Location   string  `db:"current_location"`
	Progress   float64 `db:"progress"`
	Device     string  `db:"last_read_on_device"`
	DeviceId   string  `db:"last_read_on_device_id"`
	LastReadAt int64   `db:"last_read_at"`
}

// Run imports a legacy database.
//
// Everything happens in one transaction, so a failure half way through leaves
// no half imported account behind. Running it twice is safe: legacy users whose
// credential already exists and documents that were already imported are
// skipped instead of duplicated.
func Run(app core.App, options Options) (*Report, error) {
	legacy, err := openLegacy(options.File)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = legacy.Close()
	}()

	report := &Report{}

	run := func(txApp core.App) error {
		users, err := readUsers(legacy)
		if err != nil {
			return err
		}

		var fixedOwner *core.Record
		if options.OwnerEmail != "" {
			fixedOwner, err = txApp.FindAuthRecordByEmail(schema.CollectionUsers, options.OwnerEmail)
			if err != nil {
				return fmt.Errorf("no account with the email %q: %w", options.OwnerEmail, err)
			}
		}

		owners := map[string]*core.Record{} // legacy user id -> account

		for _, user := range users {
			// A credential that is already there means this legacy user was
			// imported before. Reusing its owner is what makes a second run a
			// no-op instead of a duplicate.
			if existing, err := findCredential(txApp, user.Username); err == nil {
				owner, err := txApp.FindRecordById(schema.CollectionUsers, existing.GetString(schema.FieldOwner))
				if err != nil {
					return fmt.Errorf("the credential %q has no owner: %w", user.Username, err)
				}
				owners[user.Id] = owner
				report.Skipped = append(report.Skipped,
					fmt.Sprintf("credential %q already exists", user.Username))
				continue
			}

			owner := fixedOwner
			if owner == nil {
				owner, err = createAccount(txApp, user, report)
				if err != nil {
					return err
				}
			}
			owners[user.Id] = owner

			if err := importCredential(txApp, user, owner); err != nil {
				return err
			}
			report.Credentials++
		}

		return importProgress(txApp, legacy, owners, options, report)
	}

	if options.DryRun {
		// A rolled back transaction is the most honest dry run there is: the
		// same code path, the same validations, no lasting effect.
		err = app.RunInTransaction(func(txApp core.App) error {
			if err := run(txApp); err != nil {
				return err
			}
			return errDryRun
		})
		if errors.Is(err, errDryRun) {
			err = nil
		}
	} else {
		err = app.RunInTransaction(run)
	}
	if err != nil {
		return nil, err
	}

	return report, nil
}

// errDryRun rolls back the transaction of a dry run.
var errDryRun = errors.New("dry run")

// openLegacy opens the legacy database read only.
func openLegacy(path string) (*dbx.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("no legacy database given")
	}

	db, err := dbx.Open("sqlite", path+"?mode=ro&_pragma=busy_timeout(10000)")
	if err != nil {
		return nil, fmt.Errorf("open the legacy database %q: %w", path, err)
	}

	// Fail here rather than half way through the import.
	if _, err := db.NewQuery("SELECT 1 FROM {{users}} LIMIT 1").Execute(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%q does not look like a KOsync database: %w", path, err)
	}

	return db, nil
}

// readUsers loads the legacy users that were not deleted.
func readUsers(legacy *dbx.DB) ([]legacyUser, error) {
	users := []legacyUser{}

	err := legacy.
		NewQuery("SELECT [[id]], [[username]], [[password]] FROM {{users}} WHERE [[deleted_at]] IS NULL ORDER BY [[username]]").
		All(&users)
	if err != nil {
		return nil, fmt.Errorf("read the legacy users: %w", err)
	}

	return users, nil
}

// createAccount creates the WebUI account a legacy user's credential belongs to.
func createAccount(txApp core.App, user legacyUser, report *Report) (*core.Record, error) {
	collection, err := txApp.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return nil, err
	}

	email, err := uniqueEmail(txApp, user.Username)
	if err != nil {
		return nil, err
	}

	password := security.RandomString(16)

	record := core.NewRecord(collection)
	record.SetEmail(email)
	record.SetPassword(password)
	record.Set("name", user.Username)

	if err := txApp.Save(record); err != nil {
		return nil, fmt.Errorf("create an account for the legacy user %q: %w", user.Username, err)
	}

	report.Accounts = append(report.Accounts, CreatedAccount{
		LegacyUsername: user.Username,
		Email:          email,
		Password:       password,
	})

	return record, nil
}

// uniqueEmail derives an unused address from a legacy username.
func uniqueEmail(txApp core.App, username string) (string, error) {
	local := unsafeEmailChars.ReplaceAllString(strings.ToLower(username), "-")
	local = strings.Trim(local, "-._")
	if local == "" {
		local = "reader"
	}

	// The bare name first, then the numbered forms. The loop tests the candidate
	// it is holding and only then builds the next one, so every address it can
	// name is one it has actually asked about.
	candidate := local + "@" + generatedEmailDomain
	for attempt := 2; ; attempt++ {
		_, err := txApp.FindAuthRecordByEmail(schema.CollectionUsers, candidate)
		if errors.Is(err, sql.ErrNoRows) {
			return candidate, nil // free
		}
		if err != nil {
			// Anything else is the database failing to answer, and reading that
			// as "the address is free" would hand two legacy users the same one.
			return "", fmt.Errorf("check whether %q is free: %w", candidate, err)
		}
		if attempt > maxEmailAttempts {
			return "", fmt.Errorf("could not find a free address for the legacy user %q", username)
		}

		candidate = fmt.Sprintf("%s-%d@%s", local, attempt, generatedEmailDomain)
	}
}

// findCredential looks up an already imported KOReader credential.
func findCredential(txApp core.App, username string) (*core.Record, error) {
	return txApp.FindFirstRecordByData(schema.CollectionKoreaderAccounts, schema.FieldUsername, username)
}

// importCredential turns a legacy user into a KOReader credential.
//
// The legacy password column already holds the MD5 digest KOReader sends, so it
// is stored as is (hashed by PocketBase) and every device keeps working without
// the owner having to touch it.
func importCredential(txApp core.App, user legacyUser, owner *core.Record) error {
	collection, err := txApp.FindCollectionByNameOrId(schema.CollectionKoreaderAccounts)
	if err != nil {
		return err
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldUsername, user.Username)
	record.Set(schema.FieldOwner, owner.Id)
	record.Set(schema.FieldLabel, "Imported from legacy KOsync")
	record.SetPassword(user.Password)

	if err := txApp.Save(record); err != nil {
		return fmt.Errorf("import the credential %q: %w", user.Username, err)
	}

	return nil
}

// importProgress imports the documents and their history.
func importProgress(txApp core.App, legacy *dbx.DB, owners map[string]*core.Record, options Options, report *Report) error {
	documentsCollection, err := txApp.FindCollectionByNameOrId(schema.CollectionDocuments)
	if err != nil {
		return err
	}
	historyCollection, err := txApp.FindCollectionByNameOrId(schema.CollectionDocumentHistory)
	if err != nil {
		return err
	}

	rows, err := readDocuments(legacy, "documents", "id", options.IncludeDeleted)
	if err != nil {
		return err
	}

	// legacy owner id + legacy document id -> new document record
	imported := map[string]*core.Record{}

	for _, row := range rows {
		owner, ok := owners[row.OwnerId]
		if !ok {
			report.Skipped = append(report.Skipped,
				fmt.Sprintf("document %q of the unknown legacy user %q", row.DocumentId, row.OwnerId))
			continue
		}

		existing, err := txApp.FindFirstRecordByFilter(
			schema.CollectionDocuments,
			"owner = {:owner} && document = {:document}",
			dbx.Params{"owner": owner.Id, "document": row.DocumentId},
		)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if existing != nil {
			imported[key(row.OwnerId, row.DocumentId)] = existing
			report.Skipped = append(report.Skipped,
				fmt.Sprintf("document %q of %q already exists", row.DocumentId, owner.Email()))
			continue
		}

		record := core.NewRecord(documentsCollection)
		record.Set(schema.FieldOwner, owner.Id)
		record.Set(schema.FieldDocument, row.DocumentId)
		applyProgress(record, row)

		if err := txApp.Save(record); err != nil {
			return fmt.Errorf("import the document %q: %w", row.DocumentId, err)
		}

		imported[key(row.OwnerId, row.DocumentId)] = record
		report.Documents++
	}

	historyRows, err := readDocuments(legacy, "document_history", "document_id", options.IncludeDeleted)
	if err != nil {
		return err
	}

	for _, row := range historyRows {
		owner, ok := owners[row.OwnerId]
		if !ok {
			continue
		}

		document, ok := imported[key(row.OwnerId, row.DocumentId)]
		if !ok {
			// Usually the document was deleted in the legacy server, so it
			// was not imported and its history has nothing to hang on.
			report.Skipped = append(report.Skipped,
				fmt.Sprintf("history of the document %q, which was not imported", row.DocumentId))
			continue
		}

		record := core.NewRecord(historyCollection)
		record.Set(schema.FieldOwner, owner.Id)
		record.Set(schema.FieldDocumentRef, document.Id)
		applyProgress(record, row)

		if err := txApp.Save(record); err != nil {
			return fmt.Errorf("import a history entry of %q: %w", row.DocumentId, err)
		}

		report.History++
	}

	return nil
}

// readDocuments loads the legacy progress rows of one table.
func readDocuments(legacy *dbx.DB, table, idColumn string, includeDeleted bool) ([]legacyDocument, error) {
	rows := []legacyDocument{}

	query := fmt.Sprintf(`
		SELECT
			[[%s]] AS document_id,
			[[owner_id]] AS owner_id,
			COALESCE([[title]], '') AS title,
			COALESCE([[current_location]], '') AS current_location,
			COALESCE([[progress]], 0) AS progress,
			COALESCE([[last_read_on_device]], '') AS last_read_on_device,
			COALESCE([[last_read_on_device_id]], '') AS last_read_on_device_id,
			COALESCE([[last_read_at]], 0) AS last_read_at
		FROM {{%s}}
	`, idColumn, table)
	if !includeDeleted {
		query += " WHERE [[deleted_at]] IS NULL"
	}
	query += " ORDER BY [[last_read_at]] ASC"

	if err := legacy.NewQuery(query).All(&rows); err != nil {
		return nil, fmt.Errorf("read the legacy %s: %w", table, err)
	}

	return rows, nil
}

// applyProgress copies a legacy row onto a new record.
func applyProgress(record *core.Record, row legacyDocument) {
	record.Set(schema.FieldTitle, row.Title)
	record.Set(schema.FieldCurrentLocation, row.Location)
	record.Set(schema.FieldProgress, min(max(row.Progress, 0), 1))
	record.Set(schema.FieldLastDevice, row.Device)
	record.Set(schema.FieldLastDeviceId, row.DeviceId)
	record.Set(schema.FieldLastReadAt, legacyTime(row.LastReadAt))
}

// legacyTime converts a legacy timestamp into a time.
func legacyTime(value int64) time.Time {
	if value <= 0 {
		return time.Now().UTC()
	}

	return time.UnixMilli(value / legacyTimeUnit).UTC()
}

// key identifies a legacy document.
func key(ownerId, documentId string) string {
	return ownerId + "\x00" + documentId
}
