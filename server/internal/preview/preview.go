//
// File:        internal/preview/preview.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package preview turns a chapter of an uploaded book into markup the web
// interface may show.
//
// The input is a file somebody uploaded, which means it is untrusted however
// carefully it was chosen: an EPUB is a zip of XHTML, and XHTML can carry
// script, stylesheets, forms and requests to other servers. What comes out of
// here is rebuilt from an allow-list rather than scrubbed of what is known to
// be bad, so a construct nobody thought of is dropped instead of passed on.
//
// This is the second of two defences and not the only one. The interface puts
// the result in an <iframe sandbox> with no tokens at all, so script cannot run
// there even if something got through — see docs/technical/plans/epub-preview-reader.md
// §6. What the allow-list adds beyond that is a page that stays quiet: no
// requests leave for a publisher's server, and no book's stylesheet argues with
// the interface around it.
package preview

import (
	"bytes"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// maxOutputBytes caps the markup one chapter may produce. Some books are a
// single enormous XHTML file, and a preview is not obliged to render all of it
// into a tablet browser at once.
const maxOutputBytes = 512 << 10

// Clean rebuilds one document as the markup the web interface may render, and
// reports whether it had to stop early.
//
// image is asked for the data: URI to draw each picture with, and may be nil,
// which answers that nothing may be drawn.
func Clean(document []byte, image func(href string) (string, bool)) (string, bool) {
	// html.Parse implements the HTML parsing algorithm, which has no failure
	// mode short of the reader itself failing: malformed markup produces a tree
	// rather than an error, which is exactly what is wanted from a file nobody
	// checked before uploading it.
	root, err := html.Parse(bytes.NewReader(document))
	if err != nil {
		return "", false
	}

	worker := &cleaner{image: image, left: maxOutputBytes}
	worker.children(bodyOf(root))

	return strings.TrimSpace(worker.out.String()), worker.truncated
}

// bodyOf finds the body of a parsed document. The parser always builds one, so
// the walk below is over the part of the tree that is meant to be shown — and
// the head, with its stylesheets and its metadata, is never walked at all.
func bodyOf(root *html.Node) *html.Node {
	var found *html.Node

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if found != nil {
			return
		}
		if node.Type == html.ElementNode && node.Data == "body" {
			found = node

			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)

	if found == nil {
		return root
	}

	return found
}

// cleaner rebuilds a document into its allowed form.
type cleaner struct {
	out       strings.Builder
	image     func(href string) (string, bool)
	left      int
	truncated bool
}

// children rebuilds everything inside a node, and reports whether there was
// room for all of it.
func (c *cleaner) children(node *html.Node) bool {
	if node == nil {
		return true
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if !c.node(child) {
			return false
		}
	}

	return true
}

func (c *cleaner) node(node *html.Node) bool {
	switch node.Type {
	case html.TextNode:
		return c.write(html.EscapeString(node.Data))
	case html.ElementNode:
		return c.element(node)
	default:
		// Comments, doctypes and the rest carry nothing a reader would see.
		return true
	}
}

// element rebuilds one element, if it is one a book is allowed to show.
func (c *cleaner) element(node *html.Node) bool {
	// An element in a foreign namespace is an <svg> or a <math> subtree; the
	// parser keeps the names it was given there, so the tables below cannot be
	// trusted to recognise them.
	name := node.Data
	if node.Namespace != "" || discarded[name] {
		return true
	}

	if !allowed[name] {
		// An element nobody named is not necessarily hostile — books are full
		// of <font>, <center> and generator-invented wrappers — so the element
		// goes and the text inside it stays. Nothing is emitted for it either
		// way, which is what makes this safe: the output is built from the
		// table, never from the file.
		return c.children(node)
	}

	if name == "img" {
		return c.picture(node)
	}

	closing := "</" + name + ">"
	if void[name] {
		closing = ""
	}
	if !c.open("<"+name+attributes(node, name)+">", closing) {
		return false
	}

	room := c.children(node)
	// Written whether or not there was room, because opening the element paid
	// for it: stopping early has to leave markup a browser can still parse.
	c.out.WriteString(closing)

	return room
}

// picture draws an image the archive holds, or leaves what the book said it
// shows.
//
// An image that cannot be resolved is one from another server, one the archive
// does not hold, or one too large to inline. In every case the alt text is the
// book's own account of what is missing, and it is better on the page than a
// silent gap.
func (c *cleaner) picture(node *html.Node) bool {
	source, found := "", false
	if c.image != nil {
		source, found = c.image(attribute(node, "src"))
	}
	if !found {
		return c.write(html.EscapeString(attribute(node, "alt")))
	}

	drawn := html.EscapeString(source)
	tag := `<img src="` + drawn + `"`
	if alt := attribute(node, "alt"); alt != "" {
		tag += ` alt="` + html.EscapeString(alt) + `"`
	}
	for _, size := range []string{"width", "height"} {
		if value := digits(attribute(node, size)); value != "" {
			tag += ` ` + size + `="` + value + `"`
		}
	}
	tag += ">"

	// The picture itself is not charged to the markup budget. It travels inside
	// the tag as a data: URI, but it is not markup and it has a budget of its
	// own that the resolver has already spent from; counting it here as well
	// would end a chapter on its first full-page plate.
	return c.charge(tag, len(tag)-len(drawn))
}

// open writes an element's opening tag, paying for its closing tag at the same
// time. An element that is opened has to be closed even if the budget runs out
// inside it, so the room for that is taken before anything is written between
// the two.
func (c *cleaner) open(tag, closing string) bool {
	return c.charge(tag, len(tag)+len(closing))
}

// write appends to the output while there is room for it.
func (c *cleaner) write(text string) bool {
	return c.charge(text, len(text))
}

// charge writes text against the markup budget, at a price that is not always
// its length: see image, where most of what is written is not markup.
func (c *cleaner) charge(text string, cost int) bool {
	if text == "" {
		return true
	}
	if cost > c.left {
		c.truncated = true

		return false
	}

	c.left -= cost
	c.out.WriteString(text)

	return true
}

// attributes returns the attributes an element keeps, already escaped.
//
// Everything not named here goes, including style, class and id: the book's own
// stylesheet is never loaded, so they describe nothing, and every on* handler
// is in the same sentence as them.
func attributes(node *html.Node, name string) string {
	if name != "td" && name != "th" {
		return ""
	}

	kept := ""
	for _, of := range []string{"colspan", "rowspan"} {
		if value := digits(attribute(node, of)); value != "" {
			kept += ` ` + of + `="` + value + `"`
		}
	}

	return kept
}

// attribute returns the value of an attribute, ignoring which namespace it was
// written in — xlink:href and href are the same thing to a reader.
func attribute(node *html.Node, name string) string {
	for _, candidate := range node.Attr {
		if strings.EqualFold(candidate.Key, name) {
			return candidate.Val
		}
	}

	return ""
}

// digits returns a value that is a plain number, and nothing else. A width is
// either a count of pixels or it is something this has no business repeating.
func digits(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 6 {
		return ""
	}
	if _, err := strconv.Atoi(value); err != nil {
		return ""
	}

	return value
}

// allowed is the markup a book may show. It is prose, lists, tables, images and
// links, which between them are what a chapter is made of.
var allowed = map[string]bool{
	"p": true, "div": true, "span": true, "section": true, "article": true,
	"header": true, "footer": true, "figure": true, "figcaption": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"em": true, "strong": true, "i": true, "b": true, "u": true, "s": true,
	"small": true, "sub": true, "sup": true, "q": true, "cite": true,
	"blockquote": true, "br": true, "hr": true,
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	"table": true, "thead": true, "tbody": true, "tfoot": true,
	"tr": true, "td": true, "th": true,
	"img": true,
}

// <a> is deliberately not in the table above, so a link is unwrapped and its
// words are kept as text.
//
// A link inside a book points at another file in the archive, which is not
// something the browser showing one chapter has. Left as an href of any shape
// it is worse than useless: the frame's own document is a srcdoc, whose base
// address is the address of the application around it, so following one takes
// the frame to the KOsync interface — loaded in a sandbox that forbids it
// everything, which draws as a blank page. The words are what the reader wanted
// anyway; the chapter list is how a preview is navigated.

// void elements have no closing tag.
var void = map[string]bool{"br": true, "hr": true, "img": true}

// discarded elements are dropped along with everything inside them.
//
// The rest of the document survives unwrapped, but the text inside these is not
// text: a stylesheet, a script, the label on a form control. Showing it would
// be showing the reader the machinery.
var discarded = map[string]bool{
	"script": true, "style": true, "link": true, "meta": true, "base": true,
	"head": true, "title": true, "noscript": true, "template": true,
	"iframe": true, "frame": true, "frameset": true, "object": true,
	"embed": true, "applet": true, "param": true, "canvas": true,
	"audio": true, "video": true, "source": true, "track": true,
	"form": true, "input": true, "textarea": true, "select": true,
	"option": true, "button": true, "label": true, "fieldset": true,
	"svg": true, "math": true,
}
