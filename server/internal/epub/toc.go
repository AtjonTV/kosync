//
// File:        internal/epub/toc.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub

import (
	"bytes"
	"encoding/xml"
	"path"
	"strings"
)

// maxTocBytes caps the table of contents that is read out of an archive.
//
// A book's contents are a list of names, and a list of names is small: the
// largest in the reference library is fifteen kilobytes for eighty-four
// entries. This is far past anything real, and a file that runs into it is
// asking for the rest of the book to be parsed as a list of chapter titles.
const maxTocBytes = 2 << 20

// tocEntry is what the book's own table of contents calls one of its documents.
type tocEntry struct {
	// Label is the name the book gives it: "Prologue", or "1".
	Label string
	// Section is the entry it is nested under, empty at the top level. A
	// trilogy in one file numbers its chapters from one three times over, and
	// the number alone is then not an answer to "where am I".
	Section string
}

// tocLink is one link of a table of contents, before it is matched to a spine
// document. Depth counts the nesting, starting at one.
type tocLink struct {
	Href  string
	Label string
	Depth int
}

// contents returns the book's own names for its documents, keyed by archive
// path, and an empty map for a book that names none.
//
// Read once. Both callers of it — listing the spine and reading one document
// out of it — want the same answer, and a book that has to be asked twice would
// pay for parsing its contents on every page turn.
func (r *Reader) contents() map[string]tocEntry {
	r.tocOnce.Do(func() { r.toc = r.readContents() })

	return r.toc
}

// readContents parses whichever table of contents the archive carries.
//
// The EPUB 3 navigation document first and the EPUB 2 NCX second, which is the
// order of preference the format itself states — a file holding both, and this
// is common, has the NCX only so that older readers can open it at all.
func (r *Reader) readContents() map[string]tocEntry {
	links, from := r.readNav()
	if len(links) == 0 {
		links, from = r.readNCX()
	}

	entries := make(map[string]tocEntry, len(links))
	sections := make(map[int]string)
	dir := path.Dir(from)

	for _, link := range links {
		if link.Label == "" {
			continue
		}
		sections[link.Depth] = link.Label

		name := r.resolveFrom(dir, link.Href)
		if name == "" {
			continue
		}

		// The first mention wins. Several entries pointing into one document
		// is how a book with a chapter per file marks its sections, and the
		// document is named after the first thing in it, not the last.
		if _, taken := entries[name]; !taken {
			entries[name] = tocEntry{Label: link.Label, Section: sections[link.Depth-1]}
		}
	}

	return entries
}

// readNav reads the EPUB 3 navigation document, and says where it was read from
// so that its links can be resolved against it.
func (r *Reader) readNav() ([]tocLink, string) {
	name := ""
	for _, item := range r.pkg.Manifest.Items {
		for _, property := range strings.Fields(item.Properties) {
			if property == "nav" {
				name = r.resolve(item.Href)
			}
		}
	}

	if name == "" {
		return nil, ""
	}

	raw, err := r.readFileLimit(name, maxTocBytes)
	if err != nil {
		return nil, ""
	}

	return navLinks(raw), name
}

// readNCX reads the EPUB 2 table of contents.
func (r *Reader) readNCX() ([]tocLink, string) {
	name := ""
	for _, item := range r.pkg.Manifest.Items {
		if item.MediaType == "application/x-dtbncx+xml" {
			name = r.resolve(item.Href)
		}
	}

	if name == "" {
		return nil, ""
	}

	raw, err := r.readFileLimit(name, maxTocBytes)
	if err != nil {
		return nil, ""
	}

	var doc ncxDocument
	if err := unmarshal(raw, &doc); err != nil {
		return nil, ""
	}

	return flattenNCX(doc.Points, 1, nil), name
}

type ncxDocument struct {
	Points []ncxPoint `xml:"navMap>navPoint"`
}

type ncxPoint struct {
	Label string `xml:"navLabel>text"`
	// An attribute cannot be reached through a nested path, so the element
	// carrying it is described rather than the attribute itself.
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Points []ncxPoint `xml:"navPoint"`
}

// flattenNCX walks the nesting into the flat list the rest of this works with.
func flattenNCX(points []ncxPoint, depth int, into []tocLink) []tocLink {
	for _, point := range points {
		into = append(into, tocLink{
			Href:  strings.TrimSpace(point.Content.Src),
			Label: normalize(point.Label),
			Depth: depth,
		})
		into = flattenNCX(point.Points, depth+1, into)
	}

	return into
}

// navLinks reads the links out of an EPUB 3 navigation document.
//
// The document is XHTML, so this walks it rather than unmarshalling it: the
// structure that matters is the nesting of the lists, which a struct cannot
// describe. The <nav> marked as the table of contents is the one wanted; a file
// that marks none has its first one read, because a navigation document with a
// single unmarked nav in it is still a table of contents.
func navLinks(raw []byte) []tocLink {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity

	var found, first []tocLink
	var current []tocLink

	inNav, isTOC, depth := false, false, 0
	href, collecting := "", false
	var text strings.Builder

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch element := token.(type) {
		case xml.StartElement:
			switch strings.ToLower(element.Name.Local) {
			case "nav":
				inNav, isTOC, depth = true, navIsTOC(element), 0
				current = nil
			case "ol", "ul":
				if inNav {
					depth++
				}
			case "a":
				if inNav {
					href, collecting = attr(element, "href"), true
					text.Reset()
				}
			}
		case xml.CharData:
			if collecting {
				text.Write(element)
			}
		case xml.EndElement:
			switch strings.ToLower(element.Name.Local) {
			case "a":
				if collecting {
					current = append(current, tocLink{
						Href:  strings.TrimSpace(href),
						Label: normalize(text.String()),
						Depth: max(depth, 1),
					})
					collecting = false
				}
			case "ol", "ul":
				if inNav && depth > 0 {
					depth--
				}
			case "nav":
				if isTOC && found == nil {
					found = current
				}
				if first == nil {
					first = current
				}
				inNav, isTOC, current = false, false, nil
			}
		}
	}

	if found != nil {
		return found
	}

	return first
}

// navIsTOC says whether a <nav> is the table of contents rather than the
// landmarks or the page list. The attribute is epub:type, and the prefix is
// whatever the file declared it as, so only the local name is checked.
func navIsTOC(element xml.StartElement) bool {
	for _, candidate := range element.Attr {
		if strings.EqualFold(candidate.Name.Local, "type") {
			for _, word := range strings.Fields(candidate.Value) {
				if strings.EqualFold(word, "toc") {
					return true
				}
			}
		}
	}

	return false
}

// attr returns one attribute of an element, by local name.
func attr(element xml.StartElement, name string) string {
	for _, candidate := range element.Attr {
		if strings.EqualFold(candidate.Name.Local, name) {
			return candidate.Value
		}
	}

	return ""
}
