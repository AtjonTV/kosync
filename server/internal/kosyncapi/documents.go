//
// File:        internal/kosyncapi/documents.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi

import (
	"git.obth.eu/atjontv/kosync/internal/documents"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// restoreHistory puts a document back into an earlier state.
//
// The state that is being replaced is archived first, so restoring never loses
// the position the reader is at right now. This mirrors what a progress push
// does and is what makes the restore itself undoable.
func (h *Handler) restoreHistory(e *core.RequestEvent) error {
	document, err := e.App.FindRecordById(schema.CollectionDocuments, e.Request.PathValue("id"))
	if err != nil {
		return notFoundOrError(e, err, "document")
	}
	if document.GetString(schema.FieldOwner) != e.Auth.Id {
		return e.NotFoundError("The requested document was not found.", nil)
	}

	entry, err := e.App.FindRecordById(schema.CollectionDocumentHistory, e.Request.PathValue("historyId"))
	if err != nil {
		return notFoundOrError(e, err, "history entry")
	}
	if entry.GetString(schema.FieldOwner) != e.Auth.Id || entry.GetString(schema.FieldDocumentRef) != document.Id {
		return e.NotFoundError("The requested history entry was not found.", nil)
	}

	err = e.App.RunInTransaction(func(txApp core.App) error {
		current, err := txApp.FindRecordById(schema.CollectionDocuments, document.Id)
		if err != nil {
			return err
		}

		if err := documents.Archive(txApp, current); err != nil {
			return err
		}

		// The restored state keeps its original timestamp: the document really
		// was last read at that moment, and the statistics of that day should
		// not move because of a restore.
		current.Set(schema.FieldCurrentLocation, entry.GetString(schema.FieldCurrentLocation))
		current.Set(schema.FieldProgress, entry.GetFloat(schema.FieldProgress))
		current.Set(schema.FieldLastDevice, entry.GetString(schema.FieldLastDevice))
		current.Set(schema.FieldLastDeviceId, entry.GetString(schema.FieldLastDeviceId))
		current.Set(schema.FieldLastReadAt, entry.GetDateTime(schema.FieldLastReadAt))
		if title := entry.GetString(schema.FieldTitle); title != "" {
			current.Set(schema.FieldTitle, title)
		}

		if err := txApp.Save(current); err != nil {
			return err
		}

		// The restored entry has become the current state, so keeping it in the
		// history as well would duplicate it in the list.
		return txApp.Delete(entry)
	})
	if err != nil {
		return e.InternalServerError("Failed to restore the document.", err)
	}

	return ok(e, "The document was restored.")
}
