//
// File:        internal/kosyncapi/preview.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/preview"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// previewCacheControl is how long a browser may keep a chapter.
//
// Private, because the answer is one account's book. Short, because the point
// of the caching is a page turn back to where the reader just was, not keeping
// the book: the ETag is what makes the second visit free, and it is derived
// from the file's own hash, so a book that is replaced is never served stale.
const previewCacheControl = "private, max-age=300"

// jsonOutline is the shape of a book: what to call it and what it is made of.
type jsonOutline struct {
	Title    string        `json:"title"`
	Chapters []jsonChapter `json:"chapters"`
}

// jsonChapter is one entry of the list a reader jumps around with.
type jsonChapter struct {
	Index int    `json:"index"`
	Title string `json:"title"`

	// Section is the part of the book the entry belongs to, left out when the
	// book has no parts. A trilogy in one file numbers its chapters from one
	// three times over, and the list has to say which of the three.
	Section string `json:"section,omitempty"`
}

// previewOutline lists the chapters of one of the account's books.
//
// Nothing here writes: no document, no reading day, no achievement. That is the
// feature rather than a precaution — looking inside a book to see what it is
// must not turn into having read it — and it is why the preview is a pair of
// GETs over the stored file instead of anything that goes near the progress a
// device syncs.
func (h *Handler) previewOutline(e *core.RequestEvent) error {
	record, err := h.ownBook(e)
	if err != nil {
		return err
	}

	book, err := openForPreview(e, record)
	if err != nil {
		return err
	}
	defer book.Close()

	documents := book.Spine()
	chapters := make([]jsonChapter, 0, len(documents))
	for _, document := range documents {
		chapters = append(chapters, jsonChapter{
			Index:   document.Index,
			Title:   document.Title,
			Section: document.Section,
		})
	}

	if done := cached(e, etag(record, "outline")); done {
		return nil
	}

	return e.JSON(http.StatusOK, jsonOutline{
		Title:    record.GetString(schema.FieldTitle),
		Chapters: chapters,
	})
}

// previewChapter returns one chapter of one of the account's books.
func (h *Handler) previewChapter(e *core.RequestEvent) error {
	record, err := h.ownBook(e)
	if err != nil {
		return err
	}

	index, err := strconv.Atoi(e.Request.PathValue("index"))
	if err != nil {
		return e.BadRequestError("The chapter must be a number.", err)
	}

	book, err := openForPreview(e, record)
	if err != nil {
		return err
	}
	defer book.Close()

	chapter, err := preview.Read(book.Reader, index)
	if errors.Is(err, preview.ErrNoChapter) {
		return e.NotFoundError("The requested chapter was not found.", err)
	}
	if err != nil {
		return e.InternalServerError("Failed to read the chapter.", err)
	}

	if done := cached(e, etag(record, strconv.Itoa(index))); done {
		return nil
	}

	return e.JSON(http.StatusOK, chapter)
}

// ownBook loads a book of the signed in account.
//
// Somebody else's book is reported as missing, exactly like one that was never
// there. A custom route is not covered by the collection's view rule, so this
// is the check — and answering "forbidden" for it would confirm which ids exist.
func (h *Handler) ownBook(e *core.RequestEvent) (*core.Record, error) {
	record, err := e.App.FindRecordById(schema.CollectionBooks, e.Request.PathValue("id"))
	if err != nil {
		return nil, notFoundOrError(e, err, "book")
	}
	if record.GetString(schema.FieldOwner) != e.Auth.Id {
		return nil, e.NotFoundError("The requested book was not found.", nil)
	}

	return record, nil
}

// openForPreview opens the stored archive, turning the ways it can fail into
// answers rather than into a stack trace.
func openForPreview(e *core.RequestEvent, record *core.Record) (*books.Stored, error) {
	book, err := books.Open(e.App, record)
	switch {
	case errors.Is(err, books.ErrNoFile):
		return nil, e.NotFoundError("This book has no stored file to preview.", err)
	case errors.Is(err, epub.ErrNotEPUB):
		return nil, e.BadRequestError("This book cannot be previewed: the stored file is not a readable EPUB.", err)
	case err != nil:
		return nil, e.InternalServerError("Failed to open the book.", err)
	}

	return book, nil
}

// etag identifies one answer by the file it was read out of.
//
// The content hash rather than the record's timestamps: renaming a book or
// putting it on a shelf changes the row and changes nothing inside the file,
// and a reader paging back should not have to fetch the chapter again for it.
func etag(record *core.Record, part string) string {
	version := record.GetString(schema.FieldContentHash)
	if version == "" {
		version = record.Id + "-" + record.GetDateTime(schema.FieldUpdated).String()
	}

	return fmt.Sprintf("%q", version+"-"+part)
}

// cached sets the caching headers and reports whether the answer was already
// sent, because the browser turned out to have it.
func cached(e *core.RequestEvent, tag string) bool {
	e.Response.Header().Set("Cache-Control", previewCacheControl)
	e.Response.Header().Set("ETag", tag)

	if e.Request.Header.Get("If-None-Match") != tag {
		return false
	}

	_ = e.NoContent(http.StatusNotModified)

	return true
}
