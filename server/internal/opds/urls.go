//
// File:        internal/opds/urls.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds

import (
	"net/url"
	"strconv"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/books"
	"github.com/pocketbase/pocketbase/core"
)

// Forwarded headers a reverse proxy sets. A catalog that is only reachable
// through a proxy would otherwise fill its links with the internal address the
// proxy dialled, which is of no use to the reader that asked.
const (
	headerForwardedProto = "X-Forwarded-Proto"
	headerForwardedHost  = "X-Forwarded-Host"
)

// baseURL is the scheme and host to build the catalog's links from.
//
// The links are absolute because a reader has to be able to follow them out of
// context, and they are derived from the request rather than from a configured
// address so that whichever name the client reached the server by is the name it
// gets back. A proxy that rewrites neither the headers nor the address will
// produce links pointing at the proxy's own backend, which is the one case this
// cannot detect.
func baseURL(e *core.RequestEvent) string {
	scheme := "http"
	if e.IsTLS() {
		scheme = "https"
	}
	if forwarded := firstValue(e.Request.Header.Get(headerForwardedProto)); forwarded != "" {
		scheme = forwarded
	}

	host := e.Request.Host
	if forwarded := firstValue(e.Request.Header.Get(headerForwardedHost)); forwarded != "" {
		host = forwarded
	}

	return scheme + "://" + host
}

// firstValue takes the first entry of a comma separated header, which is what a
// chain of proxies leaves behind, and drops anything that could not be a scheme
// or a host.
func firstValue(header string) string {
	value := strings.TrimSpace(header)
	if index := strings.IndexByte(value, ','); index >= 0 {
		value = strings.TrimSpace(value[:index])
	}

	if strings.ContainsAny(value, " \t\r\n/\\\"'<>") {
		return ""
	}

	return value
}

// urls builds the catalog's links against one request's base address.
type urls struct {
	base string
}

// root is the catalog's entry point.
func (u urls) root() string {
	return u.base + RoutePrefix
}

// shelf is one of the lists of books, at the given one based page.
func (u urls) shelf(slug string, page int) string {
	address := u.base + RoutePrefix + "/" + slug

	return withPage(address, page)
}

// facet is a navigation feed — the authors, the series, the languages — at the
// given one based page.
func (u urls) facet(slug string, page int) string {
	return withPage(u.base+RoutePrefix+"/"+slug, page)
}

// group is the books under one entry of a navigation feed.
func (u urls) group(slug, value string, page int) string {
	address := u.base + RoutePrefix + "/by?" + url.Values{
		facetParam: {slug},
		valueParam: {value},
	}.Encode()
	if page > 1 {
		address += "&" + pageParam + "=" + strconv.Itoa(page)
	}

	return address
}

// search is the result page for a query.
func (u urls) search(query string, page int) string {
	address := u.base + RoutePrefix + "/search?" + url.Values{queryParam: {query}}.Encode()
	if page > 1 {
		address += "&" + pageParam + "=" + strconv.Itoa(page)
	}

	return address
}

// searchTemplate is the templated link that tells a client how to search. OPDS
// 2.0 has no separate description document: the template is the whole of it.
func (u urls) searchTemplate() string {
	return u.base + RoutePrefix + "/search{?" + queryParam + "}"
}

// download is the acquisition link.
//
// The last segment is the name the catalog serves the book under, and it is
// there rather than being left to a Content-Disposition header because a reader
// that names the downloaded file after the URL and a reader that reads the
// header then agree. That agreement is what makes the filename hash predictable.
func (u urls) download(book *core.Record) string {
	return u.base + RoutePrefix + "/books/" + book.Id + "/download/" + url.PathEscape(books.CatalogFilename(book))
}

// cover is the full sized cover image.
func (u urls) cover(bookId string) string {
	return u.base + RoutePrefix + "/books/" + bookId + "/cover"
}

// thumbnail is the small cover, which is what a grid of books wants.
func (u urls) thumbnail(bookId string) string {
	return u.base + RoutePrefix + "/books/" + bookId + "/thumbnail"
}

// withPage appends the page number, leaving the first page unmarked so that the
// canonical address of a shelf is the shelf itself.
func withPage(address string, page int) string {
	if page <= 1 {
		return address
	}

	return address + "?" + pageParam + "=" + strconv.Itoa(page)
}
