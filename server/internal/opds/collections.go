//
// File:        internal/opds/collections.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds

import (
	"slices"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// collectionSize is how many books a shelf holds.
//
// The column is a relation with more than one place in it, which PocketBase
// stores as a JSON array and keeps as one: there is no null and no other shape
// to defend against here.
const collectionSize = `json_array_length([[` + schema.FieldBooks + `]])`

// collectionGroups lists the account's own shelves, alphabetically.
//
// Empty ones are left out. A shelf with nothing on it is a plan rather than a
// place, and following a link to it on a device costs a page turn to find that
// out — the same reason the front page leaves out a facet nothing is under.
func collectionGroups(app core.App, owner string, offset, limit int) ([]group, int, error) {
	where := "[[" + schema.FieldOwner + "]] = {:owner} AND " + collectionSize + " > 0"
	params := dbx.Params{"owner": owner}

	var total int
	err := app.ConcurrentDB().
		NewQuery("SELECT COUNT(*) FROM {{" + schema.CollectionBookCollections + "}} WHERE " + where).
		Bind(params).
		Row(&total)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 || limit <= 0 {
		return nil, total, nil
	}

	rows := []struct {
		Value string `db:"value"`
		Title string `db:"title"`
		Total int    `db:"total"`
	}{}

	params["limit"] = limit
	params["offset"] = offset

	err = app.ConcurrentDB().
		NewQuery("SELECT [[id]] AS value, [[" + schema.FieldName + "]] AS title, " +
			collectionSize + " AS total" +
			" FROM {{" + schema.CollectionBookCollections + "}} WHERE " + where +
			" ORDER BY [[" + schema.FieldName + "]] COLLATE NOCASE ASC, [[id]] ASC" +
			" LIMIT {:limit} OFFSET {:offset}").
		Bind(params).
		All(&rows)
	if err != nil {
		return nil, 0, err
	}

	groups := make([]group, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, group{Value: row.Value, Title: row.Title, Count: row.Total})
	}

	return groups, total, nil
}

// findCollection loads one of the account's shelves.
//
// Somebody else's answers the same as one that does not exist, so a catalog
// address cannot be used to find out what other people have made.
func findCollection(app core.App, owner, id string) (*core.Record, bool) {
	if id == "" {
		return nil, false
	}

	record, err := app.FindRecordById(schema.CollectionBookCollections, id)
	if err != nil || record.GetString(schema.FieldOwner) != owner {
		return nil, false
	}

	return record, true
}

// listCollection returns one page of a shelf, in the order it was built.
//
// The order is the whole reason a hand-made shelf is worth having, and it is not
// something the database can sort by: it is the order of the ids in the record,
// so the books are fetched and then put back into it. A shelf is a few dozen
// books — the ceiling is far above anything anyone will build — so reading all
// of them to serve ten is cheaper than the round trip that would avoid it.
func listCollection(app core.App, owner, value string, offset, limit int) ([]*core.Record, int, error) {
	shelf, found := findCollection(app, owner, value)
	if !found {
		return nil, 0, nil
	}

	ids := shelf.GetStringSlice(schema.FieldBooks)
	if len(ids) == 0 {
		return nil, 0, nil
	}

	wanted := make([]any, 0, len(ids))
	for _, id := range ids {
		wanted = append(wanted, id)
	}

	records := []*core.Record{}
	err := ownedBooks(app, owner).
		AndWhere(dbx.In(schema.CollectionBooks+".id", wanted...)).
		All(&records)
	if err != nil {
		return nil, 0, err
	}

	place := make(map[string]int, len(ids))
	for at, id := range ids {
		place[id] = at
	}
	slices.SortFunc(records, func(a, b *core.Record) int {
		return place[a.Id] - place[b.Id]
	})

	total := len(records)
	if offset >= total {
		return nil, total, nil
	}

	return records[offset:min(offset+limit, total)], total, nil
}

// collectionHeading is the name of the shelf being read.
func collectionHeading(app core.App, owner, value string) string {
	if shelf, found := findCollection(app, owner, value); found {
		return shelf.GetString(schema.FieldName)
	}

	return "Collection"
}
