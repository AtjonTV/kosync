//
// File:        internal/opds/files.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds

import (
	"net/url"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// download streams the EPUB itself.
//
// This is why the catalog cannot simply link to PocketBase's own file URLs:
// those want a short lived token as a query parameter, obtained from an endpoint
// an OPDS client has never heard of. So the file is served from here, behind the
// same Basic authentication as the feed that pointed at it.
//
// The name in the path is not used to find anything — the id does that — but it
// is the name the file is served under, so a reader that takes its filename from
// the URL and one that takes it from the header end up with the same book on
// disk. A title edited after a device downloaded the book leaves that device
// holding the old name, which is a case the binary hash still covers.
func (h *Handler) download(e *core.RequestEvent) error {
	book, err := h.findBook(e)
	if err != nil {
		return err
	}

	stored := book.GetString(schema.FieldFile)
	if stored == "" {
		return e.NotFoundError("This book has no file.", nil)
	}

	name := books.CatalogFilename(book)
	e.Response.Header().Set("Content-Type", MediaEpub)
	e.Response.Header().Set("Content-Disposition", attachment(name))

	return h.serve(e, book.BaseFilesPath()+"/"+stored, name)
}

// cover streams the full sized cover image.
func (h *Handler) cover(e *core.RequestEvent) error {
	book, err := h.findBook(e)
	if err != nil {
		return err
	}

	cover := book.GetString(schema.FieldCover)
	if cover == "" {
		return e.NotFoundError("This book has no cover.", nil)
	}

	return h.serve(e, book.BaseFilesPath()+"/"+cover, cover)
}

// thumbnail streams the small cover, generating it on first request.
//
// PocketBase generates thumbnails lazily behind its own file endpoint, and that
// endpoint is unreachable from here, so the same generation happens on the way
// past. A failure falls back to the full sized image: a cover that is larger
// than it needs to be is better than a hole in the reader's grid.
func (h *Handler) thumbnail(e *core.RequestEvent) error {
	book, err := h.findBook(e)
	if err != nil {
		return err
	}

	cover := book.GetString(schema.FieldCover)
	if cover == "" {
		return e.NotFoundError("This book has no cover.", nil)
	}

	base := book.BaseFilesPath()
	original := base + "/" + cover
	thumb := base + "/thumbs_" + cover + "/" + thumbnailSize + "_" + cover

	fsys, err := h.app.NewFilesystem()
	if err != nil {
		return e.InternalServerError("Failed to reach the file storage.", err)
	}
	defer fsys.Close()

	if exists, _ := fsys.Exists(thumb); !exists {
		if err := fsys.CreateThumb(original, thumb, thumbnailSize); err != nil {
			h.app.Logger().Warn("could not build a catalog thumbnail",
				"book", book.Id, "error", err)
			thumb = original
		}
	}

	if err := fsys.Serve(e.Response, e.Request, thumb, cover); err != nil {
		return e.NotFoundError("That file is not stored here.", err)
	}

	return nil
}

// serve streams one stored file.
func (h *Handler) serve(e *core.RequestEvent, path, name string) error {
	fsys, err := h.app.NewFilesystem()
	if err != nil {
		return e.InternalServerError("Failed to reach the file storage.", err)
	}
	defer fsys.Close()

	if err := fsys.Serve(e.Response, e.Request, path, name); err != nil {
		return e.NotFoundError("That file is not stored here.", err)
	}

	return nil
}

// attachment builds a Content-Disposition header that survives a title with
// spaces or umlauts in it.
//
// The quoted form is the one every client understands and holds an ASCII
// approximation; the encoded form carries the real name for the clients that
// read it. Sending both is what RFC 6266 asks for.
func attachment(name string) string {
	var ascii strings.Builder
	for _, r := range name {
		switch {
		case r < 32 || r > 126:
			ascii.WriteRune('_')
		case r == '"' || r == '\\':
			ascii.WriteRune('_')
		default:
			ascii.WriteRune(r)
		}
	}

	return `attachment; filename="` + ascii.String() + `"; filename*=UTF-8''` + url.PathEscape(name)
}
