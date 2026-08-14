//
// File:        internal/analytics/timezonehooks.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/timezone"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// registerTimezoneHooks keeps the stored statistics honest about which zone they
// were reckoned in.
//
// Changing a timezone moves every day boundary the account has, so the numbers
// that were computed under the old one are all wrong at once. An evening session
// that used to fall on the next day moves back, which can join two streaks into
// one or split a day's pages across two. Nothing is lost — every day is
// recomputed from the documents and their history, which this never touches —
// but the dashboard will not read exactly as it did before.
//
// Doing it here rather than at the one moment a person first picks a zone means
// the initial choice and every later change go through the same code, which is
// the only way the later ones can be trusted.
func registerTimezoneHooks(app core.App) {
	// The name has to be one the zone database knows, and the place to find that
	// out is here, where a person can be told. Everything downstream falls back
	// to UTC rather than failing a statistics run over it.
	app.OnRecordUpdateRequest(schema.CollectionUsers).BindFunc(func(e *core.RecordRequestEvent) error {
		if zone := e.Record.GetString(schema.FieldTimezone); zone != "" && !timezone.Valid(zone) {
			return e.BadRequestError(fmt.Sprintf("%q is not a known timezone name.", zone), nil)
		}

		return e.Next()
	})

	app.OnRecordUpdate(schema.CollectionUsers).BindFunc(func(e *core.RecordEvent) error {
		before := e.Record.Original().GetString(schema.FieldTimezone)
		after := e.Record.GetString(schema.FieldTimezone)

		if err := e.Next(); err != nil {
			return err
		}
		if before == after {
			return nil
		}

		if err := RequeueEverything(e.App, e.Record.Id); err != nil {
			// The zone is still saved. A missed requeue costs stale numbers until
			// the reconcile job comes round, which is worth less than refusing
			// the change a person just made.
			e.App.Logger().Warn("failed to requeue the statistics after a timezone change",
				"owner", e.Record.Id, "from", before, "to", after, "error", err)
		}

		return nil
	})
}

// RequeueEverything marks every day an account has ever read for recomputation.
//
// The days are worked out in whichever zone the account holds now, which is what
// makes this the right thing to run after a change: the old days are recomputed
// as far as they still exist, and the ones the shift created are computed for
// the first time. Days that no longer hold any reading are emptied by the
// recomputation itself, which already deletes a row it finds nothing for.
func RequeueEverything(app core.App, ownerId string) error {
	if ownerId == "" {
		return nil
	}

	rows := []struct {
		LastReadAt types.DateTime `db:"last_read_at"`
	}{}

	err := app.DB().
		NewQuery(`
			SELECT DISTINCT [[last_read_at]] AS last_read_at
			FROM {{` + schema.CollectionDocuments + `}}
			WHERE [[owner]] = {:owner}
			UNION
			SELECT DISTINCT [[last_read_at]] AS last_read_at
			FROM {{` + schema.CollectionDocumentHistory + `}}
			WHERE [[owner]] = {:owner}
		`).
		Bind(dbx.Params{"owner": ownerId}).
		All(&rows)
	if err != nil {
		return err
	}

	moments := make([]types.DateTime, len(rows))
	for index, row := range rows {
		moments[index] = row.LastReadAt
	}

	// Both zones, because a day only stops existing once something recomputes
	// it: a row written under the old boundaries would otherwise sit there
	// forever with numbers nothing produces any more.
	location := OwnerLocation(app, ownerId)
	days := append(LocalDays(location, moments), storedDays(app, ownerId)...)

	seen := map[string]bool{}
	for _, day := range days {
		if seen[day] {
			continue
		}
		seen[day] = true

		if err := Enqueue(app, ownerId, day); err != nil {
			return err
		}
	}

	return nil
}

// storedDays lists the days an account already has a statistics row for.
func storedDays(app core.App, ownerId string) []string {
	rows := []struct {
		Date string `db:"date"`
	}{}

	err := app.DB().
		Select("DISTINCT [[date]] AS date").
		From(schema.CollectionReadingDays).
		Where(dbx.HashExp{schema.FieldOwner: ownerId}).
		All(&rows)
	if err != nil {
		app.Logger().Warn("failed to list the stored statistics days",
			"owner", ownerId, "error", err)

		return nil
	}

	dates := make([]string, len(rows))
	for index, row := range rows {
		dates[index] = row.Date
	}

	return dates
}
