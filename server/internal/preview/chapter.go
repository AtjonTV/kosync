//
// File:        internal/preview/chapter.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package preview

import (
	"encoding/base64"
	"errors"
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/epub"
)

// maxImageBytes caps one image and maxChapterImageBytes everything a chapter
// draws.
//
// The images travel inside the answer, so both of these are paid twice over: as
// base64 they grow by a third, and the whole chapter arrives before its first
// word is shown. The caps are set where an illustrated chapter still works and
// a book of full-page scans stops before it makes a tablet unhappy.
const (
	maxImageBytes        = 2 << 20
	maxChapterImageBytes = 8 << 20
)

// ErrNoChapter is returned for a chapter the book does not have.
var ErrNoChapter = errors.New("preview: no such chapter")

// Chapter is one document of a book, ready to be shown.
type Chapter struct {
	Index int    `json:"index"`
	Title string `json:"title"`

	// Section is the part of the book the chapter belongs to, empty when the
	// book has no parts. Left out of the response when it is empty.
	Section string `json:"section,omitempty"`

	HTML string `json:"html"`

	// Truncated says the chapter was longer than the preview will render, so
	// the interface can say so rather than let the text appear to stop.
	Truncated bool `json:"truncated"`
}

// drawable are the images that may be inlined.
//
// SVG is among them, and only ever as the source of an <img>. An SVG drawn that
// way is an image and nothing else: browsers disable script and external
// references in that context, which is the whole reason it is not treated here
// as the markup it otherwise is.
var drawable = map[string]bool{
	"image/jpeg":    true,
	"image/png":     true,
	"image/gif":     true,
	"image/webp":    true,
	"image/svg+xml": true,
}

// Read returns one chapter of a book as markup the interface may render.
func Read(book *epub.Reader, index int) (Chapter, error) {
	raw, document, err := book.ReadDocument(index)
	if err != nil {
		return Chapter{}, fmt.Errorf("%w: %d", ErrNoChapter, index)
	}

	markup, truncated := Clean(raw, images(book, document.Path))

	return Chapter{
		Index:     index,
		Title:     document.Title,
		Section:   document.Section,
		HTML:      markup,
		Truncated: truncated,
	}, nil
}

// images builds the resolver that inlines what a document draws.
//
// Inlined rather than served from an endpoint of their own, because the preview
// is authenticated and an <img> inside a sandboxed frame sends no credentials —
// the same problem the library's covers answer with a token in the address. A
// second token-carrying endpoint is the escape hatch if books turn out to be
// too heavily illustrated for this, and only this function changes.
//
// The budget is shared across the chapter and each image is fetched once: a
// decorative rule repeated forty times costs what one costs.
func images(book *epub.Reader, from string) func(string) (string, bool) {
	budget := maxChapterImageBytes
	seen := map[string]string{}

	return func(href string) (string, bool) {
		if href == "" {
			return "", false
		}
		if inlined, known := seen[href]; known {
			return inlined, inlined != ""
		}

		inlined, ok := inline(book, from, href, &budget)
		seen[href] = ""
		if ok {
			seen[href] = inlined
		}

		return inlined, ok
	}
}

// inline reads one image out of the archive and spends it from the budget.
func inline(book *epub.Reader, from, href string, budget *int) (string, bool) {
	raw, kind, err := book.Resource(from, href)
	if err != nil || !drawable[kind] {
		return "", false
	}
	if len(raw) == 0 || len(raw) > maxImageBytes || len(raw) > *budget {
		return "", false
	}
	*budget -= len(raw)

	return "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(raw), true
}
