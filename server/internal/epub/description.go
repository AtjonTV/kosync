//
// File:        internal/epub/description.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub

import (
	"encoding/xml"
	"regexp"
	"strings"
	"unicode/utf8"
)

// maxDescriptionInput caps how much of a dc:description is looked at, and
// maxDescriptionRunes caps what is kept from it.
//
// The element is prose meant for a reader, and prose that runs past four
// thousand characters is not a blurb any more: publishers put whole review
// sections, translator's notes and back catalogues in here. What the library
// wants is the paragraph that answers "what is this one about", so a long one is
// cut at a word boundary rather than stored in full and hidden behind a scroll
// bar on every book page.
//
// The input cap is far looser, because it is not a judgement about prose but a
// bound on the parsing: a description that arrives with sixty kilobytes of
// markup in it is a file with something else in mind.
const (
	maxDescriptionInput = 64 << 10
	maxDescriptionRunes = 4000
)

// markup matches a tag a description would be written with.
//
// This is the test for the second parse, and it is deliberately narrow. It names
// the tags rather than matching "< followed by a letter", and it wants the tag
// spelled the way a tag is spelled — no space after the bracket, and a
// terminator after the name — because a description is prose: one that says "for
// all values a < b" must not have the rest of its sentence parsed away as an
// element.
var markup = regexp.MustCompile(`(?i)</?(p|br|div|span|i|b|em|strong|h[1-6]|ul|ol|li|a|blockquote|hr|img)[\s/>]`)

// breakElements are the elements a line ends at. Everything else is inline and
// runs into the text around it, which is why <em>emphasis</em> does not become a
// paragraph of its own.
var breakElements = map[string]bool{
	"p": true, "br": true, "div": true, "li": true, "tr": true, "blockquote": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "hr": true, "section": true, "table": true,
}

// description returns the publisher's summary of the book as plain text.
//
// Plain text, and not the markup the file holds, because of where this ends up:
// a paragraph on the book page and a line in the catalog feed. Storing HTML
// would mean every reader of the column has to be trusted to render it safely,
// for a field whose whole content is one blurb — so the markup is thrown away
// here, once, and what is stored is what it said.
func (r *Reader) description() string {
	for _, entry := range r.pkg.Metadata.Descriptions {
		if text := plainText(entry.Inner); text != "" {
			return text
		}
	}

	return ""
}

// plainText turns the inside of a dc:description into paragraphs of text.
//
// It parses twice because publishers write this element two ways. Most put
// markup in it, which XML escapes on the way in: what the parser hands back is
// one run of character data that still says "&lt;p&gt;". Calibre and a few
// others put the elements in unescaped, which parses as real markup. A second
// pass over the result of the first covers the escaped case and is a no-op for
// everything else, since text with no tags left in it is not parsed again.
func plainText(fragment string) string {
	if len(fragment) > maxDescriptionInput {
		fragment = strings.ToValidUTF8(fragment[:maxDescriptionInput], "")
	}

	text := strings.Join(paragraphs(fragment), "\n\n")
	if markup.MatchString(text) {
		text = strings.Join(paragraphs(text), "\n\n")
	}

	return shorten(text)
}

// paragraphs pulls the text out of a fragment of markup, one string per line the
// markup asks for.
func paragraphs(fragment string) []string {
	// Wrapped, because a description holds a run of elements and text rather
	// than one document with a root, and given to the decoder the way the rest
	// of this package gives it a document: leniently. Publishers' markup here is
	// hand-written HTML about as often as it is XML — unclosed <br>, bare
	// ampersands, entities XML never defined — and none of that is a reason to
	// drop the blurb.
	decoder := xml.NewDecoder(strings.NewReader("<description>" + fragment + "</description>"))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity
	decoder.AutoClose = xml.HTMLAutoClose

	var lines []string
	var line strings.Builder

	flush := func() {
		if text := normalize(line.String()); text != "" {
			lines = append(lines, text)
		}
		line.Reset()
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch element := token.(type) {
		case xml.StartElement:
			if breakElements[strings.ToLower(element.Name.Local)] {
				flush()
			}
		case xml.EndElement:
			if breakElements[strings.ToLower(element.Name.Local)] {
				flush()
			}
		case xml.CharData:
			// A space, because the tag that separated these two runs of text may
			// have been an inline one that this is about to join across: "one
			// <em>two</em>" is two words and must not become one.
			if line.Len() > 0 {
				line.WriteByte(' ')
			}
			line.Write(element)
		}
	}
	flush()

	return lines
}

// shorten cuts an over-long description at a word boundary.
//
// The ellipsis is there so that the last sentence does not look like the whole
// of what the publisher wrote; a blurb that stops mid-word with no mark is read
// as a broken record rather than a shortened one.
func shorten(text string) string {
	if utf8.RuneCountInString(text) <= maxDescriptionRunes {
		return text
	}

	// Counted in runes and then stepped back to the last space, so that the text
	// neither cuts a character in half nor ends in the middle of a word.
	end := 0
	for count := 0; count < maxDescriptionRunes; count++ {
		_, size := utf8.DecodeRuneInString(text[end:])
		end += size
	}
	if space := strings.LastIndexAny(text[:end], " \n"); space > 0 {
		end = space
	}

	return strings.TrimRight(text[:end], " \n") + "…"
}
