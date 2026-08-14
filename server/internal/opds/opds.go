//
// File:        internal/opds/opds.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package opds serves the library as an OPDS 2.0 catalog.
//
// OPDS is how a reading device browses and downloads from a library it does not
// hold, and KOReader speaks it. Serving one turns the library from something to
// look at in a browser into somewhere a device gets its books — and, because a
// book downloaded from here is the very file the server holds, into the way to
// make a device's progress pushes recognisable before the first one arrives.
//
// The route group is "/opds" rather than "/koreader/opds": the "/koreader"
// prefix exists to isolate that reader's own header protocol, while this is a
// standard other readers speak too, and the path should not claim otherwise.
package opds

import (
	"net/http"
	"strconv"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// RoutePrefix is where the catalog lives.
const RoutePrefix = "/opds"

// Query parameters the catalog understands.
const (
	pageParam  = "page"
	queryParam = "query"
)

// CatalogTitle is the name of the whole catalog.
const CatalogTitle = "KOsync library"

// Handler serves the catalog.
type Handler struct {
	app      core.App
	conf     *config.Config
	auth     Authenticator
	renderer Renderer
}

// NewHandler creates the catalog handler.
func NewHandler(app core.App, conf *config.Config, auth Authenticator) *Handler {
	return &Handler{
		app:      app,
		conf:     conf,
		auth:     auth,
		renderer: JSONRenderer{},
	}
}

// Register mounts the catalog, unless it is turned off.
func Register(app core.App, conf *config.Config, auth Authenticator) *Handler {
	if !conf.EnableOpds {
		return nil
	}

	handler := NewHandler(app, conf, auth)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		handler.Mount(se)
		return se.Next()
	})

	return handler
}

// Mount registers the routes on the given serve event.
func (h *Handler) Mount(se *core.ServeEvent) {
	group := se.Router.Group(RoutePrefix)
	group.BindFunc(h.requireDevice)

	group.GET("", h.root)
	// A catalog address is typed into a device by hand or pasted from
	// somewhere, and it arrives with a trailing slash about as often as not.
	group.GET("/", h.rootOrNotFound)
	group.GET("/search", h.search)
	group.GET("/{shelf}", h.shelf)
	group.GET("/books/{id}/download/{filename}", h.download)
	group.GET("/books/{id}/cover", h.cover)
	group.GET("/books/{id}/thumbnail", h.thumbnail)
}

// root is the catalog's entry point: the shelves, and how to search them.
func (h *Handler) root(e *core.RequestEvent) error {
	at := urls{base: baseURL(e)}

	feed := &Feed{
		Id:    at.root(),
		Title: CatalogTitle,
		Links: []Link{
			{Rel: RelSelf, Href: at.root(), Type: MediaFeed},
			{Rel: RelStart, Href: at.root(), Type: MediaFeed},
			{Rel: RelSearch, Href: at.searchTemplate(), Type: MediaFeed, Templated: true},
		},
	}

	for _, entry := range shelves {
		feed.Navigation = append(feed.Navigation, Link{
			Rel:   RelSubsection,
			Href:  at.shelf(entry.Slug, 1),
			Type:  MediaFeed,
			Title: entry.Title,
		})
	}

	return h.write(e, feed)
}

// rootOrNotFound answers the trailing slash form of the catalog address.
//
// The router treats "/opds/" as a subtree, so anything deeper that matched no
// more specific route lands here as well and is refused rather than being
// quietly answered with the front page.
func (h *Handler) rootOrNotFound(e *core.RequestEvent) error {
	if strings.TrimSuffix(e.Request.URL.Path, "/") != RoutePrefix {
		return e.NotFoundError("No such part of the catalog.", nil)
	}

	return h.root(e)
}

// shelf serves one page of one list of books.
func (h *Handler) shelf(e *core.RequestEvent) error {
	entry, found := findShelf(e.Request.PathValue("shelf"))
	if !found {
		return e.NotFoundError("No such part of the catalog.", nil)
	}

	at := urls{base: baseURL(e)}
	page := pageNumber(e)
	size := h.conf.OpdsPageSize

	records, total, err := entry.list(h.app, ownerFrom(e), (page-1)*size, size)
	if err != nil {
		return e.InternalServerError("Failed to read the library.", err)
	}

	feed := h.feed(at, entry.Title, records, &Page{Number: page, Size: size, Total: total}, ownerFrom(e))
	feed.Id = at.shelf(entry.Slug, 1)
	feed.Links = append(feed.Links, pagination(func(number int) string {
		return at.shelf(entry.Slug, number)
	}, feed.Page)...)

	return h.write(e, feed)
}

// search serves the result of a title and author search.
func (h *Handler) search(e *core.RequestEvent) error {
	query := strings.TrimSpace(e.Request.URL.Query().Get(queryParam))
	at := urls{base: baseURL(e)}

	if query == "" {
		// An empty search is not an error: a client that follows the template
		// without filling it in gets the catalog's entry point back.
		return e.Redirect(http.StatusFound, at.root())
	}

	page := pageNumber(e)
	size := h.conf.OpdsPageSize

	records, total, err := listSearch(h.app, ownerFrom(e), query, (page-1)*size, size)
	if err != nil {
		return e.InternalServerError("Failed to search the library.", err)
	}

	feed := h.feed(at, "Search: "+query, records, &Page{Number: page, Size: size, Total: total}, ownerFrom(e))
	feed.Id = at.search(query, 1)
	feed.Links = append(feed.Links, pagination(func(number int) string {
		return at.search(query, number)
	}, feed.Page)...)

	return h.write(e, feed)
}

// feed builds the common part of every list of books.
func (h *Handler) feed(at urls, title string, records []*core.Record, page *Page, owner string) *Feed {
	feed := &Feed{
		Title: title,
		Page:  page,
		Links: []Link{
			{Rel: RelStart, Href: at.root(), Type: MediaFeed},
			{Rel: RelSearch, Href: at.searchTemplate(), Type: MediaFeed, Templated: true},
		},
	}

	if len(records) == 0 {
		return feed
	}

	with := loadDetails(h.app, owner, at)
	for _, record := range records {
		feed.Publications = append(feed.Publications, publicationOf(record, with))
	}

	return feed
}

// pagination returns the self and neighbour links of one page.
//
// The first and last links are always there, and next and previous only where
// there is somewhere to go, which is how a client knows it has reached an end
// without counting.
func pagination(address func(page int) string, page *Page) []Link {
	count := page.Count()

	links := []Link{{Rel: RelSelf, Href: address(page.Number), Type: MediaFeed}}
	if count < 2 {
		return links
	}

	links = append(links,
		Link{Rel: RelFirst, Href: address(1), Type: MediaFeed},
		Link{Rel: RelLast, Href: address(count), Type: MediaFeed},
	)
	if page.Number > 1 {
		links = append(links, Link{Rel: RelPrevious, Href: address(page.Number - 1), Type: MediaFeed})
	}
	if page.Number < count {
		links = append(links, Link{Rel: RelNext, Href: address(page.Number + 1), Type: MediaFeed})
	}

	return links
}

// pageNumber reads the requested page, which is one based and defaults to the
// first. Anything unreadable is the first page rather than an error: a catalog
// should not refuse to open over a mistyped query string.
func pageNumber(e *core.RequestEvent) int {
	page, err := strconv.Atoi(e.Request.URL.Query().Get(pageParam))
	if err != nil || page < 1 {
		return 1
	}

	return page
}

// write renders a feed with the configured renderer.
func (h *Handler) write(e *core.RequestEvent, feed *Feed) error {
	body, err := h.renderer.Render(feed)
	if err != nil {
		return e.InternalServerError("Failed to build the catalog feed.", err)
	}

	return e.Blob(http.StatusOK, h.renderer.ContentType(), body)
}

// findBook loads a book of the authenticated account.
//
// A book belonging to somebody else answers 404, the same as one that does not
// exist, so the catalog cannot be used to find out what other people own.
func (h *Handler) findBook(e *core.RequestEvent) (*core.Record, error) {
	id := e.Request.PathValue("id")
	if id == "" {
		return nil, e.NotFoundError("No book requested.", nil)
	}

	record, err := h.app.FindRecordById(schema.CollectionBooks, id)
	if err != nil || record.GetString(schema.FieldOwner) != ownerFrom(e) {
		return nil, e.NotFoundError("No such book.", nil)
	}

	return record, nil
}
