//
// File:        internal/migrations/1787097600_devices.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package migrations

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(upDevices, downDevices)
}

func upDevices(app core.App) error {
	if err := createDevices(app); err != nil {
		return err
	}

	return BackfillDevices(app)
}

func downDevices(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionDevices)
	if err != nil {
		return nil
	}

	return app.Delete(collection)
}

// createDevices creates the registry of devices that have pushed progress.
//
// The rows are made by the server as pushes arrive, never by a client: a device
// exists because it synced, and one typed in by hand would name nothing. Only
// the display name can be edited, which the hook in internal/devices enforces.
func createDevices(app core.App) error {
	users, err := app.FindCollectionByNameOrId(schema.CollectionUsers)
	if err != nil {
		return fmt.Errorf("find %q collection: %w", schema.CollectionUsers, err)
	}

	collection := core.NewBaseCollection(schema.CollectionDevices)

	collection.ListRule = types.Pointer(schema.OwnerRule)
	collection.ViewRule = types.Pointer(schema.OwnerRule)
	collection.CreateRule = nil
	collection.UpdateRule = types.Pointer(schema.OwnerRule)
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:          schema.FieldOwner,
		Required:      true,
		MaxSelect:     1,
		CollectionId:  users.Id,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{
		Name:     schema.FieldDeviceId,
		Required: true,
		Max:      200,
	})
	// What KOReader last called itself. Kept up to date so that a device renamed
	// on the device shows its new name to anyone who has not overridden it.
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldReportedName,
		Max:  200,
	})
	collection.Fields.Add(&core.TextField{
		Name: schema.FieldName,
		Max:  200,
	})
	collection.Fields.Add(&core.DateField{
		Name: schema.FieldLastSeen,
	})
	addTimestamps(collection)

	collection.AddIndex("idx_devices_owner_device_id", true, "owner,device_id", "")

	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create %q: %w", schema.CollectionDevices, err)
	}

	return nil
}

// BackfillDevices registers every device that has already pushed.
//
// Without it the registry starts empty and a device only appears the next time
// it syncs, which for a book finished months ago is never — and that book's
// measured page count would go on naming a bare identifier.
//
// The name taken is the one from the most recent push, which is the same rule
// the hook applies from then on.
func BackfillDevices(app core.App) error {
	type row struct {
		Owner    string `db:"owner"`
		DeviceId string `db:"device_id"`
		Name     string `db:"name"`
		LastSeen string `db:"last_seen"`
	}

	rows := []row{}

	// Both tables, because the current state of a document only holds the last
	// device to touch it: a device used earlier and then retired appears in the
	// history alone.
	query := fmt.Sprintf(`
		WITH pushes AS (
			SELECT [[owner]] AS owner, [[last_device_id]] AS device_id,
			       [[last_device]] AS name, [[last_read_at]] AS last_read_at
			FROM {{%s}}
			WHERE [[last_device_id]] != ''
			UNION ALL
			SELECT [[owner]] AS owner, [[last_device_id]] AS device_id,
			       [[last_device]] AS name, [[last_read_at]] AS last_read_at
			FROM {{%s}}
			WHERE [[last_device_id]] != ''
		)
		SELECT owner, device_id,
		       (SELECT p.name FROM pushes p
		         WHERE p.owner = pushes.owner AND p.device_id = pushes.device_id
		         ORDER BY p.last_read_at DESC LIMIT 1) AS name,
		       MAX(last_read_at) AS last_seen
		FROM pushes
		GROUP BY owner, device_id
	`, schema.CollectionDocuments, schema.CollectionDocumentHistory)

	if err := app.DB().NewQuery(query).All(&rows); err != nil {
		return fmt.Errorf("list the devices that have pushed: %w", err)
	}

	collection, err := app.FindCollectionByNameOrId(schema.CollectionDevices)
	if err != nil {
		return err
	}

	for _, entry := range rows {
		existing, err := app.FindFirstRecordByFilter(
			schema.CollectionDevices,
			"owner = {:owner} && device_id = {:device}",
			dbx.Params{"owner": entry.Owner, "device": entry.DeviceId},
		)
		if err == nil && existing != nil {
			continue
		}

		record := core.NewRecord(collection)
		record.Set(schema.FieldOwner, entry.Owner)
		record.Set(schema.FieldDeviceId, entry.DeviceId)
		record.Set(schema.FieldReportedName, entry.Name)
		record.Set(schema.FieldName, entry.Name)
		if moment, err := types.ParseDateTime(entry.LastSeen); err == nil {
			record.Set(schema.FieldLastSeen, moment)
		}

		if err := app.Save(record); err != nil {
			return fmt.Errorf("register device %q: %w", entry.DeviceId, err)
		}
	}

	return nil
}
