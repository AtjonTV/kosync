//
// File:        internal/documents/merge.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package documents

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/analytics"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// ErrNothingToMerge is returned when the merge was given no document other than
// the one it would merge into.
var ErrNothingToMerge = errors.New("no other document to merge")

// Merge folds several documents into one.
//
// The same book read on two devices can arrive as two documents: KOReader
// identifies a file by its contents, so two copies of one title are two
// different documents to it, and the reading ends up split between them — two
// entries, two positions, and per-book statistics that only ever see half of it.
// This joins them back together.
//
// What happens to the reading is the point:
//
//   - The furthest-forward state wins, by which is meant the most recently read.
//     A person who reads on two devices is somewhere in one book, and it is
//     wherever they last were.
//   - Every state that loses is written to the history first. The documents that
//     are folded in are deleted outright — there is no soft deletion here — so
//     archiving them is the only thing that keeps the reading they hold, and it
//     is what makes an unwanted merge recoverable: the state is one restore away.
//   - The hash of each folded document becomes an alias for the survivor, so the
//     device that reported it keeps syncing, now against the joined document.
//     Without that, the next push would recreate what was just merged away.
//
// The target keeps its own hash, and takes on a book or a title only where it
// had none: merging must not quietly relabel the document the caller chose to
// keep.
//
// It returns how many documents were folded in, which is not always as many as
// were named: the target itself and any repeat are ignored rather than refused.
func Merge(app core.App, ownerId, targetId string, sourceIds []string) (int, error) {
	merged := 0

	err := app.RunInTransaction(func(txApp core.App) error {
		target, err := ownedDocument(txApp, ownerId, targetId)
		if err != nil {
			return err
		}

		sources := []*core.Record{}
		seen := map[string]bool{targetId: true}
		for _, id := range sourceIds {
			if seen[id] {
				continue
			}
			seen[id] = true

			source, err := ownedDocument(txApp, ownerId, id)
			if err != nil {
				return err
			}
			sources = append(sources, source)
		}
		if len(sources) == 0 {
			return ErrNothingToMerge
		}

		// Read before anything moves: afterwards the source documents are gone
		// and their history hangs off the target, so there is no way back to the
		// days the merge touched.
		days, err := affectedDays(txApp, ownerId, append(ids(sources), target.Id))
		if err != nil {
			return err
		}

		winner := newest(target, sources)
		if winner.Id != target.Id {
			// The target is about to take on a newer position, so the one it is
			// leaving behind is superseded, exactly as a push would supersede it.
			if err := Archive(txApp, target); err != nil {
				return err
			}
			adoptState(target, winner)
		}

		for _, source := range sources {
			// The winner's state is not archived: it has just become the
			// target's current state, and a copy of it in the history as well
			// would show the same position twice.
			if source.Id != winner.Id {
				if err := Archive(txApp, source); err != nil {
					return err
				}
			}

			// Both of these run before the delete, because both cascade with it.
			if err := reparent(txApp, schema.CollectionDocumentHistory, source.Id, target.Id); err != nil {
				return err
			}
			if err := reparent(txApp, schema.CollectionDocumentAliases, source.Id, target.Id); err != nil {
				return err
			}
			if err := setAlias(txApp, ownerId, source.GetString(schema.FieldDocument), target.Id); err != nil {
				return err
			}

			inherit(target, source)

			if err := txApp.Delete(source); err != nil {
				return err
			}
		}

		if err := txApp.Save(target); err != nil {
			return err
		}

		// Every day of the joined reading, not only the days that changed hands.
		// The target may have picked up a book here, and if it did then every day
		// it was ever read is attributed differently now.
		for _, day := range days {
			if err := analytics.Enqueue(txApp, ownerId, day); err != nil {
				return err
			}
		}

		merged = len(sources)

		return nil
	})

	return merged, err
}

// ownedDocument loads one document of one owner.
//
// Somebody else's document is reported as missing rather than as forbidden, so
// the merge cannot be used to find out which document ids exist.
func ownedDocument(app core.App, ownerId, id string) (*core.Record, error) {
	record, err := app.FindRecordById(schema.CollectionDocuments, id)
	if err != nil {
		return nil, err
	}
	if record.GetString(schema.FieldOwner) != ownerId {
		return nil, fmt.Errorf("document %q: %w", id, sql.ErrNoRows)
	}

	return record, nil
}

// newest returns the document with the most recent reading.
//
// The target wins a tie, because it is the one the caller chose to keep and a
// tie says nothing about which position is further along.
func newest(target *core.Record, sources []*core.Record) *core.Record {
	winner := target
	for _, source := range sources {
		if source.GetDateTime(schema.FieldLastReadAt).Time().
			After(winner.GetDateTime(schema.FieldLastReadAt).Time()) {
			winner = source
		}
	}

	return winner
}

// adoptState copies a reading position from one document onto another. The
// identity of the document — its hash, its title, its book — is not part of it.
func adoptState(target, from *core.Record) {
	for _, field := range []string{
		schema.FieldCurrentLocation,
		schema.FieldProgress,
		schema.FieldLastDevice,
		schema.FieldLastDeviceId,
		schema.FieldLastReadAt,
		schema.FieldSourceAccount,
	} {
		target.Set(field, from.Get(field))
	}
}

// inherit fills in what the target is missing from a document being folded into
// it. A document that was never matched to a book can be merged into one that
// was, and either of the two can be the one worth keeping.
func inherit(target, source *core.Record) {
	if target.GetString(schema.FieldBook) == "" {
		target.Set(schema.FieldBook, source.GetString(schema.FieldBook))
	}
	if strings.TrimSpace(target.GetString(schema.FieldTitle)) == "" {
		target.Set(schema.FieldTitle, source.GetString(schema.FieldTitle))
	}
}

// reparent moves everything that hangs off one document onto another.
//
// This is a plain UPDATE rather than a record-by-record save: a document can
// carry thousands of history entries, and every one of them would otherwise be
// loaded, revalidated and announced over realtime to move a single column. The
// days the move affects are queued for recomputation by the caller, which is the
// one thing the skipped hooks would have done.
func reparent(app core.App, collection, from, to string) error {
	_, err := app.DB().
		Update(collection, dbx.Params{schema.FieldDocumentRef: to}, dbx.HashExp{schema.FieldDocumentRef: from}).
		Execute()

	return err
}

// setAlias points a retired document hash at the document it now belongs to.
func setAlias(app core.App, ownerId, documentHash, documentId string) error {
	if documentHash == "" {
		return nil
	}

	record, err := app.FindFirstRecordByFilter(
		schema.CollectionDocumentAliases,
		"owner = {:owner} && document = {:document}",
		dbx.Params{"owner": ownerId, "document": documentHash},
	)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		collection, err := app.FindCollectionByNameOrId(schema.CollectionDocumentAliases)
		if err != nil {
			return err
		}

		record = core.NewRecord(collection)
		record.Set(schema.FieldOwner, ownerId)
		record.Set(schema.FieldDocument, documentHash)
	}

	record.Set(schema.FieldDocumentRef, documentId)

	return app.Save(record)
}

// affectedDays lists the days on which any of the given documents were read,
// current states and history together, in the owner's timezone.
//
// The instants come out of the database and the days are worked out here,
// because a stored timestamp is UTC and a reading day is not — see
// internal/timezone.
func affectedDays(app core.App, ownerId string, documentIds []string) ([]string, error) {
	values := make([]any, len(documentIds))
	for index, id := range documentIds {
		values[index] = id
	}

	sources := []struct {
		collection string
		column     string
	}{
		{schema.CollectionDocuments, "id"},
		{schema.CollectionDocumentHistory, schema.FieldDocumentRef},
	}

	moments := []types.DateTime{}

	for _, source := range sources {
		rows := []struct {
			LastReadAt types.DateTime `db:"last_read_at"`
		}{}

		err := app.DB().
			Select("DISTINCT [[" + schema.FieldLastReadAt + "]] AS last_read_at").
			From(source.collection).
			Where(dbx.In(source.column, values...)).
			All(&rows)
		if err != nil {
			return nil, err
		}

		for _, row := range rows {
			moments = append(moments, row.LastReadAt)
		}
	}

	return analytics.LocalDays(analytics.OwnerLocation(app, ownerId), moments), nil
}

// ids returns the ids of the given records.
func ids(records []*core.Record) []string {
	list := make([]string, len(records))
	for index, record := range records {
		list[index] = record.Id
	}

	return list
}
