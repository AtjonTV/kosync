//
// File:        internal/devices/devices.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package devices keeps a named record of everything that has pushed progress.
//
// KOReader sends two things about itself: an identifier that survives a rename,
// and a name. Everything in KOsync groups by the identifier, because that is the
// only stable one — but an identifier is a hex string, and showing it to the
// person who owns the device tells them nothing. This package keeps the pair
// together and lets the name be changed to whatever the thing is actually
// called, which is rarely what KOReader calls it.
package devices

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Register wires device registration into the application lifecycle.
func Register(app core.App) {
	registerSeenHooks(app)

	// A device row describes a device that exists. Only the display name is the
	// owner's to choose; the rest is what the device said about itself, and a
	// row invented here would name nothing at all.
	app.OnRecordUpdateRequest(schema.CollectionDevices).BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil {
			return err
		}

		for _, field := range []string{
			schema.FieldOwner,
			schema.FieldDeviceId,
			schema.FieldReportedName,
			schema.FieldLastSeen,
		} {
			if _, present := info.Body[field]; present {
				return e.BadRequestError(
					fmt.Sprintf("%q is reported by the device and cannot be changed.", field),
					nil,
				)
			}
		}

		return e.Next()
	})
}

// registerSeenHooks records the device behind every progress push.
//
// It runs after the write rather than before it: a device that cannot be
// registered must not cost the reader their position, which is the same rule
// matching follows.
func registerSeenHooks(app core.App) {
	seen := func(e *core.RecordEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		if err := Seen(e.App, e.Record); err != nil {
			e.App.Logger().Warn("failed to register a device",
				"device", e.Record.GetString(schema.FieldLastDeviceId), "error", err)
		}

		return nil
	}

	for _, collection := range []string{schema.CollectionDocuments, schema.CollectionDocumentHistory} {
		app.OnRecordAfterCreateSuccess(collection).BindFunc(seen)
		app.OnRecordAfterUpdateSuccess(collection).BindFunc(seen)
	}
}

// Seen records that a device pushed, creating its row the first time.
//
// The reported name is refreshed on every push so that renaming the device in
// KOReader is visible to anyone who has not overridden it. The display name is
// only ever set once, at creation: overwriting it would undo the rename on the
// reader's very next sync.
func Seen(app core.App, push *core.Record) error {
	owner := push.GetString(schema.FieldOwner)
	deviceId := push.GetString(schema.FieldLastDeviceId)
	if owner == "" || deviceId == "" {
		return nil
	}

	name := push.GetString(schema.FieldLastDevice)
	lastRead := push.GetDateTime(schema.FieldLastReadAt)

	record, err := Find(app, owner, deviceId)
	if err != nil {
		return err
	}

	if record == nil {
		collection, err := app.FindCollectionByNameOrId(schema.CollectionDevices)
		if err != nil {
			return err
		}

		record = core.NewRecord(collection)
		record.Set(schema.FieldOwner, owner)
		record.Set(schema.FieldDeviceId, deviceId)
		record.Set(schema.FieldName, name)
	}

	if name != "" {
		record.Set(schema.FieldReportedName, name)
	}
	// Only ever forwards: history arrives out of order, and a restored old state
	// must not make a device look unused since last year.
	if !lastRead.IsZero() && lastRead.String() > record.GetDateTime(schema.FieldLastSeen).String() {
		record.Set(schema.FieldLastSeen, lastRead)
	}

	return app.Save(record)
}

// Find returns the owner's device with the given identifier, or nil.
func Find(app core.App, owner, deviceId string) (*core.Record, error) {
	records, err := app.FindRecordsByFilter(
		schema.CollectionDevices,
		"owner = {:owner} && device_id = {:device}",
		"", 1, 0,
		dbx.Params{"owner": owner, "device": deviceId},
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}

	return records[0], nil
}

// DisplayName is what a device should be called, best first.
//
// The identifier is the last resort rather than an error: a device that pushed
// without a name is still a device, and its hex string at least tells two of
// them apart.
func DisplayName(record *core.Record) string {
	if record == nil {
		return ""
	}

	for _, field := range []string{schema.FieldName, schema.FieldReportedName, schema.FieldDeviceId} {
		if value := record.GetString(field); value != "" {
			return value
		}
	}

	return ""
}
