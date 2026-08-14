//
// File:        internal/books/filename.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books

import (
	"strings"
	"unicode"

	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// CatalogExtension is the only format the library holds.
const CatalogExtension = ".epub"

// maxCatalogNameRunes caps the derived name. Long titles exist — the reference
// library has an omnibus whose title runs past 90 characters — and a file name
// has to survive being written to a device's filesystem, several of which stop
// at 255 bytes for the whole name.
const maxCatalogNameRunes = 120

// CatalogFilename is the name the OPDS catalog serves a book under.
//
// It is derived from the record rather than from the upload because the name a
// file happened to have on the uploader's disk is not something a downloading
// reader can know. Deriving it makes the filename hash predictable, which is
// what lets a book downloaded from here be recognised by a reader that
// identifies documents by name.
//
// The title is used rather than the id because this name ends up in the file
// list of an e-reader, where "Zeit des Sturms.epub" is worth having and
// "u0kq83nfm2lz5xd.epub" is not.
func CatalogFilename(book *core.Record) string {
	name := sanitizeFilename(book.GetString(schema.FieldTitle))
	if name == "" {
		name = book.Id
	}

	return name + CatalogExtension
}

// CatalogHash is the KOReader filename hash of CatalogFilename.
func CatalogHash(book *core.Record) string {
	return epub.FilenameMD5(CatalogFilename(book))
}

// sanitizeFilename reduces a title to something safe to write to a disk and to
// put in a URL path segment.
//
// Everything outside the kept set becomes a space rather than being dropped, so
// that "Feuertaufe/Der Schwalbenturm" does not collapse into one run-on word,
// and runs of spaces are then folded. A leading dot is removed as well: it would
// hide the book on the reader.
func sanitizeFilename(title string) string {
	var builder strings.Builder

	for _, r := range title {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		case strings.ContainsRune(" -_,.()'", r):
			builder.WriteRune(r)
		default:
			builder.WriteRune(' ')
		}
	}

	name := strings.Join(strings.Fields(builder.String()), " ")
	name = strings.TrimLeft(name, ".")
	name = strings.TrimSpace(name)

	runes := []rune(name)
	if len(runes) > maxCatalogNameRunes {
		name = strings.TrimSpace(string(runes[:maxCatalogNameRunes]))
	}

	return name
}
