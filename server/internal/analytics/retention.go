//
// File:        internal/analytics/retention.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"database/sql"
	"errors"
	"time"

	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/timezone"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// monthLayout is how a rollup month is written.
const monthLayout = "2006-01"

// monthTotals is the running sum of one month during a rollup.
type monthTotals struct {
	updateCount      int
	progressIncrease float64
	readingTime      int
	daysActive       int
	pagesRead        int
}

// ApplyRetention folds or drops daily statistics older than the configured
// retention window.
//
// In "aggregate" mode every aged out day is added to its month before it is
// deleted, so the long term totals survive while the per day detail does not.
// In "delete" mode the days are simply removed.
func ApplyRetention(app core.App, conf *config.Config, now time.Time) (int, error) {
	cutoff := now.UTC().AddDate(0, 0, -conf.AnalyticsRetentionDays).Format(DateLayout)

	aged, err := app.FindRecordsByFilter(
		schema.CollectionReadingDays,
		"date < {:cutoff}",
		"date",
		0,
		0,
		dbx.Params{"cutoff": cutoff},
	)
	if err != nil {
		return 0, err
	}
	if len(aged) == 0 {
		return 0, nil
	}

	if conf.AnalyticsRetentionMode == config.RetentionModeAggregate {
		if err := rollup(app, aged); err != nil {
			return 0, err
		}
	} else if err := dropBookDays(app, cutoff); err != nil {
		return 0, err
	}

	for _, record := range aged {
		if err := app.Delete(record); err != nil {
			return 0, err
		}
	}

	return len(aged), nil
}

// dropBookDays removes the per-book rows of aged out days.
//
// Only in "delete" mode. "aggregate" keeps them, and deliberately: they are not
// the per-day detail that retention exists to bound, they are the record of how
// long a book took, and folding them into a month would destroy the one thing a
// book's page is made of. There is one row per book per day rather than per day,
// which for a reader is the same order of magnitude.
func dropBookDays(app core.App, cutoff string) error {
	_, err := app.DB().
		Delete(schema.CollectionReadingBookDays, dbx.NewExp("[[date]] < {:cutoff}", dbx.Params{"cutoff": cutoff})).
		Execute()

	return err
}

// rollup adds the given days to their monthly totals.
func rollup(app core.App, days []*core.Record) error {
	totals := map[string]map[string]*monthTotals{} // owner -> month -> totals

	for _, day := range days {
		owner := day.GetString(schema.FieldOwner)
		date := day.GetString(schema.FieldDate)
		if len(date) < len(monthLayout) {
			continue
		}
		month := date[:len(monthLayout)]

		if _, ok := totals[owner]; !ok {
			totals[owner] = map[string]*monthTotals{}
		}
		if _, ok := totals[owner][month]; !ok {
			totals[owner][month] = &monthTotals{}
		}

		sum := totals[owner][month]
		sum.updateCount += day.GetInt(schema.FieldUpdateCount)
		sum.progressIncrease += day.GetFloat(schema.FieldProgressIncrease)
		sum.readingTime += day.GetInt(schema.FieldReadingTime)
		sum.pagesRead += day.GetInt(schema.FieldPagesRead)
		sum.daysActive++
	}

	collection, err := app.FindCollectionByNameOrId(schema.CollectionReadingMonths)
	if err != nil {
		return err
	}

	for owner, months := range totals {
		for month, sum := range months {
			record, err := findMonth(app, owner, month)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if record == nil {
				record = core.NewRecord(collection)
				record.Set(schema.FieldOwner, owner)
				record.Set(schema.FieldMonth, month)
			}

			// Every daily row is folded exactly once and then deleted, so
			// adding to the stored month can never double count.
			record.Set(schema.FieldUpdateCount, record.GetInt(schema.FieldUpdateCount)+sum.updateCount)
			record.Set(schema.FieldProgressIncrease, record.GetFloat(schema.FieldProgressIncrease)+sum.progressIncrease)
			record.Set(schema.FieldReadingTime, record.GetInt(schema.FieldReadingTime)+sum.readingTime)
			record.Set(schema.FieldDaysActive, record.GetInt(schema.FieldDaysActive)+sum.daysActive)
			record.Set(schema.FieldPagesRead, record.GetInt(schema.FieldPagesRead)+sum.pagesRead)

			if err := app.Save(record); err != nil {
				return err
			}
		}
	}

	return nil
}

// findMonth loads the stored rollup of a single month.
func findMonth(app core.App, ownerId, month string) (*core.Record, error) {
	return app.FindFirstRecordByFilter(
		schema.CollectionReadingMonths,
		"owner = {:owner} && month = {:month}",
		dbx.Params{"owner": ownerId, "month": month},
	)
}

// Reconcile re-queues the recent days of everyone who read lately.
//
// It exists because an enqueue can be lost (a crash between the write and the
// queue insert, a failed insert), and a lost enqueue would otherwise leave a
// stale statistics row behind forever.
func Reconcile(app core.App, conf *config.Config, now time.Time) (int, error) {
	since := now.UTC().AddDate(0, 0, -conf.AnalyticsReconcileDays).Format(dateTimeLayout)

	// Grouped by owner because the day an instant belongs to depends on whose
	// instant it is, and two accounts can be in two zones.
	rows := []struct {
		Owner      string         `db:"owner"`
		LastReadAt types.DateTime `db:"last_read_at"`
	}{}

	err := app.DB().
		NewQuery(`
			SELECT [[owner]] AS owner, [[last_read_at]] AS last_read_at
			FROM {{` + schema.CollectionDocuments + `}}
			WHERE [[last_read_at]] >= {:since}
			UNION
			SELECT [[owner]] AS owner, [[last_read_at]] AS last_read_at
			FROM {{` + schema.CollectionDocumentHistory + `}}
			WHERE [[last_read_at]] >= {:since}
		`).
		Bind(dbx.Params{"since": since}).
		All(&rows)
	if err != nil {
		return 0, err
	}

	byOwner := map[string][]types.DateTime{}
	for _, row := range rows {
		byOwner[row.Owner] = append(byOwner[row.Owner], row.LastReadAt)
	}

	queued := 0
	for owner, moments := range byOwner {
		for _, date := range timezone.LocalDays(timezone.Of(app, owner), moments) {
			if err := Enqueue(app, owner, date); err != nil {
				return 0, err
			}
			queued++
		}
	}

	return queued, nil
}
