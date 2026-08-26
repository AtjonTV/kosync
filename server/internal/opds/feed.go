//
// File:        internal/opds/feed.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds

import "time"

// Media types the catalog speaks.
const (
	// MediaFeed is an OPDS 2.0 feed, which is a Readium manifest in JSON.
	MediaFeed = "application/opds+json"

	// MediaAuthentication is the document a 401 carries so that a client can
	// discover how to authenticate instead of guessing.
	MediaAuthentication = "application/opds-authentication+json"

	// MediaEpub is the only publication format the library holds.
	MediaEpub = "application/epub+zip"
)

// Link relations used in the feeds.
const (
	RelSelf       = "self"
	RelStart      = "start"
	RelNext       = "next"
	RelPrevious   = "previous"
	RelFirst      = "first"
	RelLast       = "last"
	RelSearch     = "search"
	RelSubsection = "subsection"

	// RelAcquisition is the OPDS relation for a publication that can simply be
	// downloaded. Nothing here is lent, sold or expiring, so the open access
	// relation is the honest one.
	RelAcquisition = "http://opds-spec.org/acquisition/open-access"

	// RelImage and RelThumbnail separate the full cover from the small one. A
	// reader that cannot tell them apart falls back to the first image in the
	// list, which means fetching a cover several hundred kilobytes larger than
	// the grid it is being drawn into.
	RelImage     = "http://opds-spec.org/image"
	RelThumbnail = "http://opds-spec.org/image/thumbnail"
)

// Feed is one page of the catalog, in the shape the catalog thinks in rather
// than the shape any particular version of OPDS serialises.
//
// The separation exists because the client baseline speaks OPDS 2.0 while older
// readers speak Atom, and a second renderer over this tree is a small addition
// where a second feed builder would not be. Only the JSON renderer is built.
type Feed struct {
	// Id identifies the feed. OPDS 2.0 has no such field, which is why it lives
	// here rather than in the renderer: Atom requires one.
	Id string

	Title string

	// Links are the feed's own relations — self, start, search, and the
	// pagination links when there are more publications than fit on one page.
	Links []Link

	// Navigation is how a person moves between shelves. A feed carries either
	// navigation or publications; nothing in the catalog mixes them.
	Navigation []Link

	Publications []Publication

	// Page is nil for a feed that is not a page of a longer list.
	Page *Page
}

// Page describes where a feed sits in a longer list.
type Page struct {
	// Number is one based, because that is what OPDS puts on the wire.
	Number int
	Size   int
	Total  int
}

// Count returns how many pages the whole list occupies.
func (p *Page) Count() int {
	if p == nil || p.Size < 1 {
		return 0
	}

	return (p.Total + p.Size - 1) / p.Size
}

// Link is a relation to somewhere else.
type Link struct {
	Rel   string
	Href  string
	Type  string
	Title string

	// Templated marks an href that carries URI template variables, which is how
	// OPDS 2.0 describes a search: there is no separate description document to
	// fetch, the template is the whole of it.
	Templated bool

	// Width and Height describe an image, and are what lets a reader pick the
	// thumbnail over the full cover without downloading both.
	Width  int
	Height int
}

// Publication is one book in the catalog.
type Publication struct {
	Id       string
	Title    string
	Authors  []string
	Language string

	// Identifier is the book's own identity as the publisher gave it, as a URI.
	Identifier string

	// Description is the prose a reader shows under "book information". A
	// publication without one has that button greyed out, so it is filled with
	// what this server actually knows — where the reading stands — rather than
	// left to a publisher's blurb that most EPUBs do not carry.
	Description string

	// Pages is the effective page count, measured where a measurement exists and
	// derived from the word count otherwise.
	Pages int

	Updated time.Time

	// Links holds the acquisition, and Images the cover in its two sizes.
	Links  []Link
	Images []Link
}

// Renderer turns a feed into a response body.
type Renderer interface {
	// ContentType is what to send the body as.
	ContentType() string

	// Render serialises the feed.
	Render(feed *Feed) ([]byte, error)
}
