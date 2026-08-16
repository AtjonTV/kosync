//
// File:        internal/migrations/1787875200_page_reads.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(upPageReads, downPageReads)
}

func upPageReads(app core.App) error {
	return createPageReads(app)
}

func downPageReads(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionPageReads)
	if err != nil {
		return nil
	}

	return app.Delete(collection)
}

// createPageReads holds what a device measured about its own reading.
//
// Every other number in this database is inferred. The sync protocol carries no
// clock, so a reading day is worked out from when pushes arrived and a reading
// time from the gaps between them — which is why a device that reads offline for
// a week appears to have read nothing and then everything in one instant. These
// rows are the other thing entirely: KOReader's own record of which page it was
// showing, from when, for how long.
//
// Stored as the events rather than as a daily summary, for two reasons. A day is
// the reader's day and depends on a timezone that can be changed afterwards, so
// summarising on the way in would bake in an answer that has to be revisable.
// And the events are what makes the import idempotent: KOReader's own unique key
// is (book, page, start time), so re-importing a database that has grown by a
// week inserts exactly that week.
func createPageReads(app core.App) error {
	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	collection := core.NewBaseCollection(schema.CollectionPageReads)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	// Nothing writes these through the API. They arrive by import, from a file
	// the account uploaded, and a row somebody typed in would be a measurement
	// nothing measured.
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  users.Id,
		CascadeDelete: true,
	})

	// KOReader's hash of the file, which is the same string the sync protocol
	// calls the document. No relation to the documents collection: a device
	// measures reading in books it has never pushed progress for, and that
	// reading is no less real for it.
	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldDocument,
		Required: true,
		Max:      32,
	})

	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldPage,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})
	collection.Fields.Add(&core.DateField{
		Name:     schema.FieldStartedAt,
		Required: true,
	})
	// Seconds the page stayed open, as the device counted them.
	collection.Fields.Add(&core.NumberField{
		Name:    schema.FieldDuration,
		OnlyInt: true,
		Min:     types.Pointer(0.0),
	})

	addTimestamps(collection)

	// KOReader's own unique key, which is what makes a re-import a no-op for
	// everything that was already here.
	collection.AddIndex("idx_page_reads_unique", true, "owner,document,page,started_at", "")
	// Every question asked of these rows is "what happened between these two
	// instants", for one account.
	collection.AddIndex("idx_page_reads_owner_started", false, "owner,started_at", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionPageReads, err)
	}

	return nil
}
