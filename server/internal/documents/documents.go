//
// File:        internal/documents/documents.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package documents holds the reading progress operations that both the device
// API and the WebUI API need.
package documents

import (
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// FindByHash loads the current progress of one document of one owner.
//
// It returns a wrapped sql.ErrNoRows when the owner has no such document.
func FindByHash(app core.App, ownerId, documentHash string) (*core.Record, error) {
	return app.FindFirstRecordByFilter(
		schema.CollectionDocuments,
		"owner = {:owner} && document = {:document}",
		dbx.Params{"owner": ownerId, "document": documentHash},
	)
}

// Archive copies the state a document is about to leave behind into its history.
//
// Every write that replaces the current state calls this first, which is what
// makes the per document history in the WebUI complete.
func Archive(app core.App, document *core.Record) error {
	collection, err := app.FindCollectionByNameOrId(schema.CollectionDocumentHistory)
	if err != nil {
		return err
	}

	entry := core.NewRecord(collection)
	entry.Set(schema.FieldOwner, document.GetString(schema.FieldOwner))
	entry.Set(schema.FieldDocumentRef, document.Id)
	entry.Set(schema.FieldTitle, document.GetString(schema.FieldTitle))
	entry.Set(schema.FieldCurrentLocation, document.GetString(schema.FieldCurrentLocation))
	entry.Set(schema.FieldProgress, document.GetFloat(schema.FieldProgress))
	entry.Set(schema.FieldLastDevice, document.GetString(schema.FieldLastDevice))
	entry.Set(schema.FieldLastDeviceId, document.GetString(schema.FieldLastDeviceId))
	entry.Set(schema.FieldLastReadAt, document.GetDateTime(schema.FieldLastReadAt))

	return app.Save(entry)
}
