//
// File:        internal/kosyncapi/documents.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi

import (
	"database/sql"
	"errors"
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/documents"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// mergeRequest asks for several documents to be folded into one.
type mergeRequest struct {
	// Into is the document that survives and keeps its hash.
	Into string `json:"into"`
	// From are the documents folded into it, which cease to exist.
	From []string `json:"from"`
}

// mergeDocuments joins documents that are the same reading under different
// hashes.
//
// The work is in internal/documents; what is here is who is allowed to ask.
// Anything belonging to somebody else is reported as missing, the same as
// something that never existed.
func (h *Handler) mergeDocuments(e *core.RequestEvent) error {
	request := mergeRequest{}
	if err := e.BindBody(&request); err != nil {
		return e.BadRequestError("Failed to read the merge payload.", err)
	}
	if request.Into == "" {
		return e.BadRequestError("Field 'into' is required: name the document to keep.", nil)
	}
	if len(request.From) == 0 {
		return e.BadRequestError("Field 'from' is required: name at least one document to merge.", nil)
	}

	merged, err := documents.Merge(e.App, e.Auth.Id, request.Into, request.From)
	switch {
	case errors.Is(err, documents.ErrNothingToMerge):
		return e.BadRequestError("A document cannot be merged into itself.", err)
	case errors.Is(err, sql.ErrNoRows):
		return e.NotFoundError("One of the requested documents was not found.", err)
	case err != nil:
		return e.InternalServerError("Failed to merge the documents.", err)
	}

	return ok(e, fmt.Sprintf("%s merged into one.", plural(merged+1, "document")))
}

// plural writes a count with its noun, so a message reads as a sentence rather
// than as a template.
func plural(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}

	return fmt.Sprintf("%d %ss", count, noun)
}

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
