//
// File:        internal/opds/catalog.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds

import (
	"encoding/json"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Thumbnail dimensions, which have to agree with the cover field's Thumbs in the
// books migration or the reader is promised a size that cannot be produced.
const (
	thumbnailSize   = "200x300"
	thumbnailWidth  = 200
	thumbnailHeight = 300
)

// shelf is one of the lists the catalog offers.
//
// Three of them, and each answers a different question: what do I have, what did
// I just add, and what am I in the middle of. The last is the one this server is
// in a position to answer that a plain file share is not.
type shelf struct {
	Slug    string
	Title   string
	Summary string

	// list returns one page of the shelf and how many books it holds in total.
	list func(app core.App, owner string, offset, limit int) ([]*core.Record, int, error)
}

// shelves are the lists on the catalog's front page, in the order they appear.
var shelves = []shelf{
	{
		Slug:    "reading",
		Title:   "Currently reading",
		Summary: "Books this account has started and not finished.",
		list:    listReading,
	},
	{
		Slug:    "recent",
		Title:   "Recently added",
		Summary: "Newest uploads first.",
		list:    listRecent,
	},
	{
		Slug:    "books",
		Title:   "All books",
		Summary: "The whole library, by title.",
		list:    listAll,
	},
}

// findShelf returns the shelf with the given slug.
func findShelf(slug string) (shelf, bool) {
	for _, candidate := range shelves {
		if candidate.Slug == slug {
			return candidate, true
		}
	}

	return shelf{}, false
}

// ownedBooks starts a query over one account's library.
func ownedBooks(app core.App, owner string) *dbx.SelectQuery {
	return app.RecordQuery(schema.CollectionBooks).
		AndWhere(dbx.HashExp{schema.CollectionBooks + "." + schema.FieldOwner: owner})
}

// listAll returns the library by title.
func listAll(app core.App, owner string, offset, limit int) ([]*core.Record, int, error) {
	total, err := app.CountRecords(schema.CollectionBooks, dbx.HashExp{schema.FieldOwner: owner})
	if err != nil {
		return nil, 0, err
	}

	records := []*core.Record{}
	err = ownedBooks(app, owner).
		// NOCASE only folds ASCII, so an umlaut still sorts after "z". That is
		// worth knowing and not worth a collation of our own.
		OrderBy("[[books.title]] COLLATE NOCASE ASC", "[[books.created]] ASC").
		Limit(int64(limit)).
		Offset(int64(offset)).
		All(&records)

	return records, int(total), err
}

// listRecent returns the library newest first.
func listRecent(app core.App, owner string, offset, limit int) ([]*core.Record, int, error) {
	total, err := app.CountRecords(schema.CollectionBooks, dbx.HashExp{schema.FieldOwner: owner})
	if err != nil {
		return nil, 0, err
	}

	records := []*core.Record{}
	err = ownedBooks(app, owner).
		OrderBy("[[books.created]] DESC").
		Limit(int64(limit)).
		Offset(int64(offset)).
		All(&records)

	return records, int(total), err
}

// startedCondition is what "currently reading" means: a device has pushed
// progress through this book, and has not reached the end of it.
//
// The upper bound is short of 1 rather than at it because a reader that finishes
// a book reports 1.0, and a finished book does not belong on this shelf.
var startedCondition = dbx.NewExp("documents.progress > 0 AND documents.progress < 1")

// listReading returns the books with reading in progress, most recently read
// first.
//
// The join can produce a book twice, because two document hashes can point at
// one book — the same file identified by name on one device and by content on
// another. Grouping folds those back into one entry, and the book's position is
// the more recent of the two.
func listReading(app core.App, owner string, offset, limit int) ([]*core.Record, int, error) {
	var total int
	err := app.ConcurrentDB().
		Select("COUNT(DISTINCT books.id)").
		From(schema.CollectionBooks).
		InnerJoin(schema.CollectionDocuments, dbx.NewExp("documents.book = books.id")).
		AndWhere(dbx.HashExp{"books.owner": owner}).
		AndWhere(startedCondition).
		Row(&total)
	if err != nil {
		return nil, 0, err
	}

	records := []*core.Record{}
	err = ownedBooks(app, owner).
		InnerJoin(schema.CollectionDocuments, dbx.NewExp("documents.book = books.id")).
		AndWhere(startedCondition).
		GroupBy("books.id").
		OrderBy("MAX([[documents.last_read_at]]) DESC").
		Limit(int64(limit)).
		Offset(int64(offset)).
		All(&records)

	return records, total, err
}

// listSearch returns the books whose title or author matches.
//
// Title and author are what a person searching a catalog of their own books has
// in mind; the full text of the books is not indexed and searching it is not
// what this is for.
func listSearch(app core.App, owner, query string, offset, limit int) ([]*core.Record, int, error) {
	pattern := "%" + strings.ReplaceAll(strings.ReplaceAll(query, "\\", "\\\\"), "%", "\\%") + "%"
	condition := dbx.NewExp(
		"(books.title LIKE {:pattern} ESCAPE '\\' OR books.authors LIKE {:pattern} ESCAPE '\\')",
		dbx.Params{"pattern": pattern},
	)

	var total int
	err := app.ConcurrentDB().
		Select("COUNT(*)").
		From(schema.CollectionBooks).
		AndWhere(dbx.HashExp{"books.owner": owner}).
		AndWhere(condition).
		Row(&total)
	if err != nil {
		return nil, 0, err
	}

	records := []*core.Record{}
	err = ownedBooks(app, owner).
		AndWhere(condition).
		OrderBy("[[books.title]] COLLATE NOCASE ASC", "[[books.created]] ASC").
		Limit(int64(limit)).
		Offset(int64(offset)).
		All(&records)

	return records, total, err
}

// publicationOf turns a book record into a catalog entry.
func publicationOf(book *core.Record, with details) Publication {
	pages, _ := books.EffectivePages(book)
	at := with.at

	publication := Publication{
		Id:          at.download(book),
		Title:       book.GetString(schema.FieldTitle),
		Authors:     book.GetStringSlice(schema.FieldAuthors),
		Language:    book.GetString(schema.FieldLanguage),
		Identifier:  identifierOf(book),
		Description: with.describe(book),
		Pages:       pages,
		Updated:     book.GetDateTime(schema.FieldUpdated).Time(),
		// No title on the acquisition: a reader labels the download button with
		// the link's title if there is one and with the format if there is not,
		// so naming it "Download" replaces "EPUB" with a word the button next
		// to it already says.
		Links: []Link{{
			Rel:  RelAcquisition,
			Href: at.download(book),
			Type: MediaEpub,
		}},
	}

	// A book without a cover simply has no images, which is a case every OPDS
	// client handles; inventing a placeholder would only cost a round trip.
	if book.GetString(schema.FieldCover) != "" {
		publication.Images = []Link{
			{Rel: RelImage, Href: at.cover(book.Id)},
			{
				Rel:    RelThumbnail,
				Href:   at.thumbnail(book.Id),
				Width:  thumbnailWidth,
				Height: thumbnailHeight,
			},
		}
	}

	return publication
}

// identifierSchemes are the identifier kinds worth publishing, in the order they
// are preferred, mapped to the URN namespace they belong in.
var identifierSchemes = []struct {
	Key    string
	Prefix string
}{
	{Key: "ISBN", Prefix: "urn:isbn:"},
	{Key: "UUID", Prefix: "urn:uuid:"},
}

// identifierOf returns the book's own identity as a URI, or an empty string when
// the publisher gave it none worth repeating.
func identifierOf(book *core.Record) string {
	raw := book.GetString(schema.FieldIdentifiers)
	if strings.TrimSpace(raw) == "" {
		return ""
	}

	identifiers := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &identifiers); err != nil {
		return ""
	}

	for _, scheme := range identifierSchemes {
		value := strings.TrimSpace(identifiers[scheme.Key])
		if value == "" {
			continue
		}
		// An EPUB that already stored a full URI keeps it, rather than being
		// given a second prefix.
		if strings.Contains(value, ":") {
			return value
		}

		return scheme.Prefix + value
	}

	return ""
}
