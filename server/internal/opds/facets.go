//
// File:        internal/opds/facets.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds

import (
	"fmt"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// group is one entry of a navigation feed: a value the library can be broken up
// by, and how many books are under it.
type group struct {
	// Value is the stored value, and is what the link carries back. It is not
	// necessarily what is shown — a language is stored as "de-DE" and shown as
	// "German".
	Value string
	Title string
	Count int
}

// facet is one way of breaking the library into shelves.
//
// A flat list stops working somewhere around a hundred books: the reference
// library is 192, which is nineteen pages of ten on a device whose only
// navigation is next and previous. A facet turns that into one page of names.
type facet struct {
	Slug    string
	Title   string
	Summary string

	// groups returns one page of the facet's entries and how many there are in
	// all. A limit of zero returns the count alone, which is how the front page
	// finds out whether the facet is worth offering.
	groups func(app core.App, owner string, offset, limit int) ([]group, int, error)

	// list returns one page of the books under one entry.
	list func(app core.App, owner, value string, offset, limit int) ([]*core.Record, int, error)

	// heading titles the feed of one entry.
	heading func(value string) string
}

// facets are the navigation feeds the catalog offers, in the order they appear.
//
// Subjects are deliberately not among them. The books carry them and the column
// stores them, but on the reference library 143 of 202 distinct subjects belong
// to exactly one book — a navigation feed of 202 entries, most of which lead to
// a single title, is a worse way to find something than the flat list it would
// be replacing. Hand-made collections are the answer to the same problem.
var facets = []facet{
	{
		Slug:    "authors",
		Title:   "By author",
		Summary: "Every author in the library.",
		groups:  authorGroups,
		list:    listByAuthor,
		heading: func(value string) string { return value },
	},
	{
		Slug:    "series",
		Title:   "By series",
		Summary: "Books that belong to a series, in reading order.",
		groups:  seriesGroups,
		list:    listBySeries,
		heading: func(value string) string { return value },
	},
	{
		Slug:    "languages",
		Title:   "By language",
		Summary: "Every language the library holds.",
		groups:  languageGroups,
		list:    listByLanguage,
		heading: languageName,
	},
}

// findFacet returns the facet with the given slug.
func findFacet(slug string) (facet, bool) {
	for _, candidate := range facets {
		if candidate.Slug == slug {
			return candidate, true
		}
	}

	return facet{}, false
}

// authorValues is the authors of one book as rows.
//
// The column is a JSON array and a book may have several authors, so a book
// appears under each of them. The guard is not paranoia about the column's
// contents so much as about its absence: json_each of NULL is an error, and a
// book nobody claimed would otherwise take the whole query down with it.
const authorValues = `json_each(IIF(json_valid(` +
	schema.CollectionBooks + `.` + schema.FieldAuthors + `), ` +
	schema.CollectionBooks + `.` + schema.FieldAuthors + `, '[]'))`

// authorGroups lists the authors, alphabetically.
func authorGroups(app core.App, owner string, offset, limit int) ([]group, int, error) {
	return countedGroups(app, groupQuery{
		owner:    owner,
		selectAs: "[[value]]",
		from:     "{{" + schema.CollectionBooks + "}}, " + authorValues,
		where:    "TRIM([[value]]) != ''",
		order:    "[[value]] COLLATE NOCASE ASC",
		offset:   offset,
		limit:    limit,
	}, nil)
}

// listByAuthor returns the books one author wrote, by title.
func listByAuthor(app core.App, owner, value string, offset, limit int) ([]*core.Record, int, error) {
	condition := dbx.NewExp(
		"EXISTS (SELECT 1 FROM "+authorValues+" WHERE [[value]] = {:value})",
		dbx.Params{"value": value},
	)

	return listWhere(app, owner, condition,
		[]string{"[[books.title]] COLLATE NOCASE ASC", "[[books.created]] ASC"}, offset, limit)
}

// seriesGroups lists the series, alphabetically.
func seriesGroups(app core.App, owner string, offset, limit int) ([]group, int, error) {
	return countedGroups(app, groupQuery{
		owner:    owner,
		selectAs: "[[" + schema.FieldSeries + "]]",
		from:     "{{" + schema.CollectionBooks + "}}",
		where:    "TRIM([[" + schema.FieldSeries + "]]) != ''",
		order:    "[[value]] COLLATE NOCASE ASC",
		offset:   offset,
		limit:    limit,
	}, nil)
}

// listBySeries returns one series in reading order.
//
// The number first and the title only to break ties, which is the whole point of
// the shelf: a series read alphabetically is a series read in the wrong order.
// A volume the publisher gave no number sorts to the front rather than being
// scattered through the numbered ones.
func listBySeries(app core.App, owner, value string, offset, limit int) ([]*core.Record, int, error) {
	condition := dbx.NewExp(
		"books."+schema.FieldSeries+" = {:value}",
		dbx.Params{"value": value},
	)

	return listWhere(app, owner, condition,
		[]string{"[[books." + schema.FieldSeriesIndex + "]] ASC", "[[books.title]] COLLATE NOCASE ASC"},
		offset, limit)
}

// languageTag folds a stored language into the one it means.
//
// The reference library stores nine distinct values for four languages: "de",
// "de-DE" and "DE" are all German, and shelving them apart would reproduce the
// splitting this whole feature exists to undo. The region is dropped along with
// the case, so a book in Austrian German sits with the rest of the German ones.
const languageTag = `LOWER(SUBSTR([[` + schema.FieldLanguage + `]], 1, ` +
	`CASE WHEN INSTR([[` + schema.FieldLanguage + `]], '-') > 0 ` +
	`THEN INSTR([[` + schema.FieldLanguage + `]], '-') - 1 ` +
	`ELSE LENGTH([[` + schema.FieldLanguage + `]]) END))`

// languageGroups lists the languages, the most common first.
//
// Not alphabetically, because there are usually two or three of them and the one
// somebody wants is the one most of their library is in.
func languageGroups(app core.App, owner string, offset, limit int) ([]group, int, error) {
	return countedGroups(app, groupQuery{
		owner:    owner,
		selectAs: languageTag,
		from:     "{{" + schema.CollectionBooks + "}}",
		where:    "TRIM([[" + schema.FieldLanguage + "]]) != ''",
		order:    "[[total]] DESC, [[value]] ASC",
		offset:   offset,
		limit:    limit,
	}, languageName)
}

// listByLanguage returns the books in one language, by title.
func listByLanguage(app core.App, owner, value string, offset, limit int) ([]*core.Record, int, error) {
	condition := dbx.NewExp(
		strings.ReplaceAll(languageTag, "[["+schema.FieldLanguage+"]]", "books."+schema.FieldLanguage)+" = {:value}",
		dbx.Params{"value": value},
	)

	return listWhere(app, owner, condition,
		[]string{"[[books.title]] COLLATE NOCASE ASC", "[[books.created]] ASC"}, offset, limit)
}

// languageNames are the display names of the language tags a personal library
// is likely to hold. Anything else is shown as the tag itself, uppercased,
// which is honest about the fact that nothing here knows what it is.
var languageNames = map[string]string{
	"cs": "Czech",
	"da": "Danish",
	"de": "German",
	"el": "Greek",
	"en": "English",
	"es": "Spanish",
	"fi": "Finnish",
	"fr": "French",
	"hu": "Hungarian",
	"it": "Italian",
	"ja": "Japanese",
	"la": "Latin",
	"nb": "Norwegian",
	"nl": "Dutch",
	"nn": "Norwegian",
	"pl": "Polish",
	"pt": "Portuguese",
	"ro": "Romanian",
	"ru": "Russian",
	"sv": "Swedish",
	"tr": "Turkish",
	"uk": "Ukrainian",
	// Not a language: it is what an EPUB says when it will not say. One book of
	// the reference library declares it, and "Unknown" is a better shelf to find
	// that book on than "UND".
	"und": "Unknown",
	"zh":  "Chinese",
}

// languageName is what to call a language tag.
func languageName(tag string) string {
	if name, known := languageNames[strings.ToLower(tag)]; known {
		return name
	}

	return strings.ToUpper(tag)
}

// groupQuery is the shape every navigation feed's query has: one value per book,
// grouped and counted, over one account's library.
type groupQuery struct {
	owner    string
	selectAs string
	from     string
	where    string
	order    string
	offset   int
	limit    int
}

// countedGroups runs a group query, naming each entry with the given function.
//
// Written out rather than assembled with the query builder because two of the
// three group by an expression — a JSON array element, a language tag with its
// region cut off — and a builder that quotes its identifiers cannot express
// either without being talked out of it.
func countedGroups(app core.App, query groupQuery, name func(value string) string) ([]group, int, error) {
	where := "{{" + schema.CollectionBooks + "}}.[[" + schema.FieldOwner + "]] = {:owner}"
	if query.where != "" {
		where += " AND " + query.where
	}

	inner := fmt.Sprintf("SELECT %s AS value FROM %s WHERE %s", query.selectAs, query.from, where)
	params := dbx.Params{"owner": query.owner}

	var total int
	err := app.ConcurrentDB().
		NewQuery("SELECT COUNT(*) FROM (SELECT [[value]] FROM (" + inner + ") GROUP BY [[value]])").
		Bind(params).
		Row(&total)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 || query.limit <= 0 {
		return nil, total, nil
	}

	rows := []struct {
		Value string `db:"value"`
		Total int    `db:"total"`
	}{}

	params["limit"] = query.limit
	params["offset"] = query.offset

	err = app.ConcurrentDB().
		NewQuery("SELECT [[value]], COUNT(*) AS total FROM (" + inner + ")" +
			" GROUP BY [[value]] ORDER BY " + query.order +
			" LIMIT {:limit} OFFSET {:offset}").
		Bind(params).
		All(&rows)
	if err != nil {
		return nil, 0, err
	}

	groups := make([]group, 0, len(rows))
	for _, row := range rows {
		title := row.Value
		if name != nil {
			title = name(row.Value)
		}
		groups = append(groups, group{Value: row.Value, Title: title, Count: row.Total})
	}

	return groups, total, nil
}

// listWhere returns one page of the books matching a condition, and how many
// there are in all.
func listWhere(
	app core.App,
	owner string,
	condition dbx.Expression,
	order []string,
	offset, limit int,
) ([]*core.Record, int, error) {
	var total int
	err := app.ConcurrentDB().
		Select("COUNT(*)").
		From(schema.CollectionBooks).
		AndWhere(dbx.HashExp{"books." + schema.FieldOwner: owner}).
		AndWhere(condition).
		Row(&total)
	if err != nil {
		return nil, 0, err
	}

	records := []*core.Record{}
	err = ownedBooks(app, owner).
		AndWhere(condition).
		OrderBy(order...).
		Limit(int64(limit)).
		Offset(int64(offset)).
		All(&records)

	return records, total, err
}
