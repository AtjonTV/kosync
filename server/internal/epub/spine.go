//
// File:        internal/epub/spine.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

// maxTitleBytes is how much of a document is read to find out what it is called,
// and maxOutlineBytes how much of that is read across the whole book.
//
// A title lives in the head and a first heading a line or two after it, so the
// answer is always near the front of the file; reading the rest of a chapter to
// find something in its first paragraph would make listing a book's contents
// cost as much as reading it. Past the total, the remaining documents are simply
// numbered, which is what a book with several thousand of them would show
// anyway.
const (
	maxTitleBytes   = 16 << 10
	maxOutlineBytes = 8 << 20
)

// Document is one entry of the spine.
//
// Index is the position in the list Spine returns, not in the spine element:
// an entry naming a file the archive does not hold is dropped, because there is
// nothing to show for it and a gap in the numbering would only be a way for a
// reader to ask for it anyway.
type Document struct {
	Index int
	Path  string
	Title string

	// Section is the part of the book the entry is nested under, empty when it
	// is not nested under anything. A trilogy in one file numbers its chapters
	// from one three times over, and "3" alone is then not an answer to the
	// question the header asks.
	Section string
}

// Spine lists the documents the book is read in, in reading order.
//
// The names are the book's own where it gives any — the EPUB 3 navigation
// document or the EPUB 2 NCX — and derived from each document otherwise. Both
// halves are needed: a book's contents cover the chapters and leave out the
// cover, the title page and the imprint, while the documents themselves are
// named by whatever produced them, which for a great many publishers means the
// book's own title in all eighty-four of them.
func (r *Reader) Spine() []Document {
	paths := r.spinePaths()
	documents := make([]Document, 0, len(paths))

	budget := maxOutlineBytes
	for index, name := range paths {
		var raw []byte
		if budget > 0 {
			if read, err := r.readFileLimit(name, maxTitleBytes); err == nil {
				budget -= len(read)
				raw = read
			}
		}

		title, section := r.describe(name, raw, index)
		documents = append(documents, Document{
			Index:   index,
			Path:    name,
			Title:   title,
			Section: section,
		})
	}

	return documents
}

// ReadDocument returns one spine document as the archive stores it, along with
// the entry it belongs to.
//
// The entry comes back with it rather than being looked up separately, because
// the two things a caller needs afterwards — where the document sits in the
// archive, so that what it references can be resolved, and what it is called —
// are both answered by the bytes that were just read. Asking Spine instead
// would read the front of every other document in the book to answer about one.
func (r *Reader) ReadDocument(index int) ([]byte, Document, error) {
	paths := r.spinePaths()
	if index < 0 || index >= len(paths) {
		return nil, Document{}, fmt.Errorf("epub: no document %d in the spine", index)
	}

	raw, err := r.readFile(paths[index])
	if err != nil {
		return nil, Document{}, err
	}

	title, section := r.describe(paths[index], raw, index)

	return raw, Document{
		Index:   index,
		Path:    paths[index],
		Title:   title,
		Section: section,
	}, nil
}

// Resource returns one file the archive holds, along with what it is.
//
// The href is read the way the document that wrote it meant it: relative to
// that document, and never leaving the archive. resolveFrom refuses anything
// carrying a scheme, so an http URL or a data: URI resolves to nothing rather
// than to a request this server would make on a book's behalf.
func (r *Reader) Resource(from, href string) ([]byte, string, error) {
	name := r.resolveFrom(path.Dir(from), href)
	if name == "" {
		return nil, "", fmt.Errorf("epub: %q leads outside the archive", href)
	}

	raw, err := r.readFile(name)
	if err != nil {
		return nil, "", err
	}

	return raw, r.mediaType(name), nil
}

// spinePaths lists the archive paths of the spine, dropping what is not there.
func (r *Reader) spinePaths() []string {
	items := r.pkg.Spine.Items
	if len(items) > maxSpineItems {
		items = items[:maxSpineItems]
	}

	paths := make([]string, 0, len(items))
	for _, ref := range items {
		href, found := r.hrefIDs[ref.IDRef]
		if !found {
			continue
		}

		name := r.resolve(href)
		if _, held := r.byName[name]; !held {
			continue
		}

		paths = append(paths, name)
	}

	return paths
}

// mediaType names what a file in the archive is.
//
// The manifest is asked first, because it is the book's own statement and the
// only one that can tell an SVG image from an SVG that is a page. The extension
// answers for anything the manifest forgot, which is common enough in the fonts
// and images generators add after the fact.
func (r *Reader) mediaType(name string) string {
	for _, item := range r.pkg.Manifest.Items {
		if item.MediaType != "" && r.resolve(item.Href) == name {
			return item.MediaType
		}
	}

	return extensionTypes[strings.ToLower(path.Ext(name))]
}

// extensionTypes covers what a document draws, for archives whose manifest does
// not say. Anything not named here has no media type, and the preview will not
// inline what it cannot name.
var extensionTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

// readFileLimit reads at most the given number of bytes of an archive entry.
func (r *Reader) readFileLimit(name string, limit int64) ([]byte, error) {
	file, found := r.byName[name]
	if !found {
		return nil, fmt.Errorf("epub: %s not in archive", name)
	}

	handle, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	return io.ReadAll(io.LimitReader(handle, limit))
}

// describe names one spine document, and says which part of the book it belongs
// to. A preview needs every entry to be nameable, because the name is what the
// list of them is picked from.
//
// The book's own table of contents first, because it is the only one of these
// that was written by a person. Then what the document says it is, unless that
// is the title of the book itself: publishers' generators write the book's name
// into the head of every file, and eighty-four chapters all called "Metro - Die
// Trilogie" is a list nobody can pick from. Then its first heading, for the
// files that carry one. Then its number, which at least tells them apart.
func (r *Reader) describe(name string, raw []byte, index int) (string, string) {
	if entry, found := r.contents()[name]; found && entry.Label != "" {
		return entry.Label, entry.Section
	}

	title, heading := documentTitles(raw)
	if title != "" && !strings.EqualFold(title, r.bookTitle()) {
		return title, ""
	}
	if heading != "" {
		return heading, ""
	}

	return numberedTitle(index), ""
}

// bookTitle is what the package document calls the whole book.
func (r *Reader) bookTitle() string {
	if len(r.pkg.Metadata.Titles) == 0 {
		return ""
	}

	return normalize(r.pkg.Metadata.Titles[0])
}

func numberedTitle(index int) string {
	return "Chapter " + strconv.Itoa(index+1)
}

// documentTitles returns what a document calls itself: the <title> of the file,
// and the first heading in it. They come back separately because neither is
// reliably the better one — a document that says two different things about
// itself is not something this can arbitrate, and the caller decides.
func documentTitles(raw []byte) (string, string) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity

	title := ""
	heading := ""
	collecting := ""
	var text strings.Builder

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch element := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(element.Name.Local)
			if collecting != "" {
				continue
			}
			switch name {
			case "title", "h1", "h2", "h3":
				collecting = name
				text.Reset()
			}
		case xml.CharData:
			if collecting != "" {
				text.Write(element)
			}
		case xml.EndElement:
			if collecting == "" || strings.ToLower(element.Name.Local) != collecting {
				continue
			}

			found := normalize(text.String())
			if collecting == "title" {
				if title == "" {
					title = found
				}
			} else if heading == "" {
				heading = found
			}
			collecting = ""

			if title != "" && heading != "" {
				return title, heading
			}
		}
	}

	return title, heading
}
