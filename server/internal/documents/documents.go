//
// File:        internal/documents/documents.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package documents holds the reading progress operations that both the device
// API and the WebUI API need.
package documents

import (
	"database/sql"
	"errors"

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

// Resolve loads the document a hash belongs to, following a merge.
//
// A device keeps sending the hash it has always sent, and after a merge that
// hash is no longer a document of its own. Every read and every write of
// progress goes through here rather than through FindByHash, so the push lands
// on the document the reading was folded into instead of quietly rebuilding the
// one that was merged away.
//
// Like FindByHash it returns a wrapped sql.ErrNoRows when the hash means nothing
// to this owner.
func Resolve(app core.App, ownerId, documentHash string) (*core.Record, error) {
	record, err := FindByHash(app, ownerId, documentHash)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return record, err
	}

	alias, aliasErr := app.FindFirstRecordByFilter(
		schema.CollectionDocumentAliases,
		"owner = {:owner} && document = {:document}",
		dbx.Params{"owner": ownerId, "document": documentHash},
	)
	if aliasErr != nil {
		// No document and no alias: report the missing document, which is what
		// the caller asked about.
		return nil, err
	}

	return app.FindRecordById(schema.CollectionDocuments, alias.GetString(schema.FieldDocumentRef))
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
