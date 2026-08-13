//
// File:        internal/migrations/1786579200_kosync_init.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package migrations defines the KOsync data model.
//
// The collections are created from Go code (instead of the superuser UI) so the
// schema is versioned in source and reviewable in a diff. Editing the schema of
// a running production instance through the UI will be overwritten by the next
// deployment, so all schema changes belong here.
package migrations

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(up, down)
}

func up(app core.App) error {
	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	// Account recovery and (later) achievement mails need a deliverable address.
	if emailField, ok := users.Fields.GetByName("email").(*core.EmailField); ok {
		emailField.Required = true
		if err := app.Save(users); err != nil {
			return fmt.Errorf("require %q email: %w", schema.CollectionUsers, err)
		}
	}

	koreaderAccounts, err := createKoreaderAccounts(app, users.Id)
	if err != nil {
		return err
	}

	documents, err := createDocuments(app, users.Id, koreaderAccounts.Id)
	if err != nil {
		return err
	}

	if err := createDocumentHistory(app, users.Id, documents.Id); err != nil {
		return err
	}

	if err := createReadingDays(app, users.Id); err != nil {
		return err
	}

	if err := createReadingMonths(app, users.Id); err != nil {
		return err
	}

	return createAnalyticsQueue(app, users.Id)
}

func down(app core.App) error {
	// Reverse creation order so relation targets outlive the collections
	// pointing at them.
	names := []string{
		schema.CollectionAnalyticsQueue,
		schema.CollectionReadingMonths,
		schema.CollectionReadingDays,
		schema.CollectionDocumentHistory,
		schema.CollectionDocuments,
		schema.CollectionKoreaderAccounts,
	}

	for _, name := range names {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			continue // already gone
		}
		if err := app.Delete(collection); err != nil {
			return fmt.Errorf("delete %q: %w", name, err)
		}
	}

	return nil
}

// createKoreaderAccounts creates the credentials KOReader devices sign in with.
//
// It is an auth collection purely for the password handling: the stored value is
// the MD5 hex that KOReader sends, hashed with bcrypt by PocketBase. Logging in
// through the PocketBase auth API is disabled (AuthRule nil, password auth off),
// because a device credential must never become an API session.
func createKoreaderAccounts(app core.App, usersId string) (*core.Collection, error) {
	collection := core.NewAuthCollection(schema.CollectionKoreaderAccounts)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.UpdateRule = types.Pointer(schema.OwnerRule)
	collection.DeleteRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = nil // created through /api/kosync/koreader-accounts

	collection.AuthRule = nil // no authentication through the PocketBase auth API
	collection.ManageRule = nil
	collection.PasswordAuth.Enabled = false
	collection.OAuth2.Enabled = false
	collection.MFA.Enabled = false
	collection.OTP.Enabled = false
	collection.AuthAlert.Enabled = false

	// KOReader accounts have no mailbox of their own; the owning user does.
	collection.Fields.Add(&core.EmailField{
		Name:     "email",
		System:   true,
		Required: false,
	})

	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldUsername,
		Required: true,
		Min:      1,
		Max:      255,
	})
	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  usersId,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldLabel,
		Max:  255,
	})
	// Negated on purpose: PocketBase booleans default to false, so a freshly
	// created credential is usable without having to remember a flag.
	collection.Fields.Add(&core.BoolField{
		Name: schema.FieldDisabled,
	})
	collection.Fields.Add(&core.DateField{
		Name: schema.FieldLastUsed,
	})
	addTimestamps(collection)

	collection.AddIndex("idx_koreader_accounts_username", true, "username", "")
	collection.AddIndex("idx_koreader_accounts_owner", false, "owner", "")

	if err := app.Save(collection); err != nil {
		return nil, fmt.Errorf("create %q: %w", schema.CollectionKoreaderAccounts, err)
	}

	return collection, nil
}

// createDocuments creates the current progress state per owner and document hash.
func createDocuments(app core.App, usersId, koreaderAccountsId string) (*core.Collection, error) {
	collection := core.NewBaseCollection(schema.CollectionDocuments)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.UpdateRule = types.Pointer(schema.OwnerRule)
	collection.DeleteRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = nil // documents are created by progress pushes

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  usersId,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldDocument,
		Required: true,
		Min:      1,
		Max:      255,
	})
	addProgressFields(collection)
	// Losing a device credential must not lose the reading progress it pushed,
	// so this relation deliberately does not cascade.
	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldSourceAccount,
		MaxSelect:     1,
		CollectionId:  koreaderAccountsId,
		CascadeDelete: false,
	})
	addTimestamps(collection)

	collection.AddIndex("idx_documents_owner_document", true, "owner,document", "")
	collection.AddIndex("idx_documents_owner_last_read_at", false, "owner,last_read_at", "")

	if err := app.Save(collection); err != nil {
		return nil, fmt.Errorf("create %q: %w", schema.CollectionDocuments, err)
	}

	return collection, nil
}

// createDocumentHistory creates the append-only log of superseded document states.
func createDocumentHistory(app core.App, usersId, documentsId string) error {
	collection := core.NewBaseCollection(schema.CollectionDocumentHistory)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.DeleteRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = nil // written by the server when a document is superseded
	collection.UpdateRule = nil // history is immutable

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  usersId,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldDocumentRef,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  documentsId,
		CascadeDelete: true,
	})
	addProgressFields(collection)
	collection.Fields.Add(&core.AutodateField{
		Name:     "created",
		OnCreate: true,
	})

	collection.AddIndex("idx_document_history_document_ref", false, "document_ref,last_read_at", "")
	collection.AddIndex("idx_document_history_owner_last_read_at", false, "owner,last_read_at", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionDocumentHistory, err)
	}

	return nil
}

// createReadingDays creates the precomputed daily statistics the dashboard reads.
func createReadingDays(app core.App, usersId string) error {
	collection := core.NewBaseCollection(schema.CollectionReadingDays)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  usersId,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldDate,
		Required: true,
		Pattern:  `^\d{4}-\d{2}-\d{2}$`,
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldUpdateCount,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	// Percentage points, so a full book read in one day is 100.
	collection.Fields.Add(&core.NumberField{
		Name: schema.FieldProgressIncrease,
		Min:  types.Pointer(0.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldReadingTime, // seconds
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldDocumentsTouched,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldPagesRead,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.DateField{
		Name: schema.FieldComputedAt,
	})
	addTimestamps(collection)

	collection.AddIndex("idx_reading_days_owner_date", true, "owner,date", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionReadingDays, err)
	}

	return nil
}

// createReadingMonths creates the rollup target for aged out daily statistics.
func createReadingMonths(app core.App, usersId string) error {
	collection := core.NewBaseCollection(schema.CollectionReadingMonths)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  usersId,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldMonth,
		Required: true,
		Pattern:  `^\d{4}-\d{2}$`,
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldUpdateCount,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name: schema.FieldProgressIncrease,
		Min:  types.Pointer(0.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldReadingTime,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldDaysActive,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldPagesRead,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	addTimestamps(collection)

	collection.AddIndex("idx_reading_months_owner_month", true, "owner,month", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionReadingMonths, err)
	}

	return nil
}

// createAnalyticsQueue creates the pending statistics recomputations.
//
// The unique (owner, date) index is what collapses a burst of progress pushes
// into a single recomputation.
func createAnalyticsQueue(app core.App, usersId string) error {
	collection := core.NewBaseCollection(schema.CollectionAnalyticsQueue)

	// Internal bookkeeping: superusers only.
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  usersId,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldDate,
		Required: true,
		Pattern:  `^\d{4}-\d{2}-\d{2}$`,
	})
	collection.Fields.Add(&core.AutodateField{
		Name:     "created",
		OnCreate: true,
	})

	collection.AddIndex("idx_analytics_queue_owner_date", true, "owner,date", "")
	collection.AddIndex("idx_analytics_queue_created", false, "created", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionAnalyticsQueue, err)
	}

	return nil
}

// addProgressFields adds the reading state shared by documents and their history.
func addProgressFields(collection *core.Collection) {
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldTitle,
		Max:  500,
	})
	// KOReader sends an xpointer or a page fragment, which can get long.
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldCurrentLocation,
		Max:  2000,
	})
	collection.Fields.Add(&core.NumberField{
		Name: schema.FieldProgress,
		Min:  types.Pointer(0.0),
		Max:  types.Pointer(1.0),
	})
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldLastDevice,
		Max:  255,
	})
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldLastDeviceId,
		Max:  255,
	})
	collection.Fields.Add(&core.DateField{
		Name:     schema.FieldLastReadAt,
		Required: true,
	})
}

// addTimestamps adds the standard PocketBase created/updated fields.
func addTimestamps(collection *core.Collection) {
	collection.Fields.Add(&core.AutodateField{
		Name:     "created",
		OnCreate: true,
	})
	collection.Fields.Add(&core.AutodateField{
		Name:     "updated",
		OnCreate: true,
		OnUpdate: true,
	})
}
