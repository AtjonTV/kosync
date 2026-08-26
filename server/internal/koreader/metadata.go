//
// File:        internal/koreader/metadata.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package koreader

import (
	"strings"

	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// applyMetadata records what the device says the file is.
//
// Everything here is optional twice over: the reader only sends it when "send
// document metadata" is on, and any one of the three can be empty when it is.
// So nothing is ever cleared — an absent field means the device did not say,
// not that the answer is nothing.
func applyMetadata(document *core.Record, metadata *ProgressMetadata) {
	if metadata == nil {
		return
	}

	if filename := strings.TrimSpace(metadata.Filename); filename != "" {
		// Overwritten on every push, because it describes the file as it is on
		// the device now and a rename there should show here.
		document.Set(schema.FieldFilename, filename)
		document.Set(schema.FieldFilenameHash, epub.FilenameMD5(filename))
	}

	if authors := strings.TrimSpace(metadata.Authors); authors != "" {
		document.Set(schema.FieldDocumentAuthors, authors)
	}

	// The title is the one thing on a document a person can edit, so it is only
	// ever filled in, never replaced. A device that keeps sending the publisher's
	// title must not undo a rename on the next sync.
	if title := strings.TrimSpace(metadata.Title); title != "" &&
		strings.TrimSpace(document.GetString(schema.FieldTitle)) == "" {
		document.Set(schema.FieldTitle, title)
	}
}
