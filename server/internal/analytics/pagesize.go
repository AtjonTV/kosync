//
// File:        internal/analytics/pagesize.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/pages"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// bookProgressQuery reads every progress value ever recorded for one book,
// grouped into series by the file it was read from and the device that read it.
//
// Both tables are needed: documents holds the latest position of each document
// and document_history everything before it, so the series is only complete
// across the two. The grouping is what makes the estimate possible at all — a
// page belongs to one file on one screen, and interleaving two of them destroys
// the quantisation the measurement depends on.
const bookProgressQuery = `
	SELECT document, device, progress, moment FROM (
		SELECT
			[[id]] AS document,
			[[last_device_id]] AS device,
			[[progress]] AS progress,
			[[last_read_at]] AS moment
		FROM {{` + schema.CollectionDocuments + `}}
		WHERE [[book]] = {:book}
		UNION ALL
		SELECT
			h.[[document_ref]] AS document,
			h.[[last_device_id]] AS device,
			h.[[progress]] AS progress,
			h.[[last_read_at]] AS moment
		FROM {{` + schema.CollectionDocumentHistory + `}} h
		JOIN {{` + schema.CollectionDocuments + `}} d ON d.[[id]] = h.[[document_ref]]
		WHERE d.[[book]] = {:book}
	)
	ORDER BY document ASC, device ASC, moment ASC
`

// recentWindow is how many of a series' most recent pushes are tried before the
// whole of it.
//
// This is what makes the measurement self-healing. Changing the font changes the
// page count, and a series that spans the change fits neither pagination, so the
// estimator refuses it and the book would keep its old number forever. Looking
// at the recent end first means the new pagination is measured as soon as enough
// of it exists, and 40 pushes is a few evenings of reading — comfortably above
// the twelve the estimator needs, and short enough to leave an old setting
// behind quickly.
const recentWindow = 40

// bookProgressRow is one recorded position.
type bookProgressRow struct {
	Document string  `db:"document"`
	Device   string  `db:"device"`
	Progress float64 `db:"progress"`
	Moment   string  `db:"moment"`
}

// MeasurePageSize recovers a book's page count from the progress its devices
// pushed and stores it on the record.
//
// It reports whether anything was written. A book nobody has read since the last
// measurement is left alone, so recomputing forty days of one book's reading
// does not measure it forty times.
func MeasurePageSize(app core.App, book *core.Record) (bool, error) {
	rows := []bookProgressRow{}

	err := app.DB().
		NewQuery(bookProgressQuery).
		Bind(dbx.Params{"book": book.Id}).
		All(&rows)
	if err != nil {
		return false, fmt.Errorf("read the progress of book %s: %w", book.Id, err)
	}

	if len(rows) < pages.MinSamples {
		return false, nil
	}

	// How far into the reading the stored measurement already looked. This is
	// deliberately the newest push it saw rather than the moment it ran: reading
	// timestamps come from the device and are always in the past, so a wall clock
	// here would mean no book is ever measured twice.
	latest := newest(rows)
	if measured := book.GetDateTime(schema.FieldMeasuredThrough); !measured.IsZero() && latest <= measured.String() {
		return false, nil
	}

	device, estimate, found := bestEstimate(rows)
	if !found {
		return false, nil
	}

	moment, err := types.ParseDateTime(latest)
	if err != nil {
		return false, fmt.Errorf("read the latest push of book %s: %w", book.Id, err)
	}

	book.Set(schema.FieldMeasuredPages, estimate.Pages)
	book.Set(schema.FieldMeasuredDevice, device)
	book.Set(schema.FieldMeasuredThrough, moment)

	if err := app.Save(book); err != nil {
		return false, fmt.Errorf("store the measured page count of book %s: %w", book.Id, err)
	}

	return true, nil
}

// newest returns the latest moment in the rows, which are ordered by series
// rather than by time.
func newest(rows []bookProgressRow) string {
	latest := ""
	for _, row := range rows {
		if row.Moment > latest {
			latest = row.Moment
		}
	}

	return latest
}

// bestEstimate measures every series separately and keeps the one with the most
// evidence behind it.
//
// A book read on two devices has two page counts, both of them right for their
// own screen. The statistics need one unit, and the series with the most pushes
// behind it is the one whose pages the reader actually spent the time turning.
func bestEstimate(rows []bookProgressRow) (string, pages.Estimate, bool) {
	var (
		bestDevice string
		best       pages.Estimate
		found      bool
		series     string
		device     string
		progress   []float64
		started    bool
	)

	consider := func() {
		estimate, ok := estimateSeries(progress)
		if !ok {
			return
		}
		if !found || estimate.Samples > best.Samples {
			bestDevice, best, found = device, estimate, true
		}
	}

	for _, row := range rows {
		key := row.Document + "\x00" + row.Device
		if !started || key != series {
			consider()
			series, device = key, row.Device
			progress = progress[:0]
			started = true
		}
		progress = append(progress, row.Progress)
	}
	consider()

	return bestDevice, best, found
}

// estimateSeries measures one file on one device, most recent pushes first.
//
// Widening to the whole series when the recent window has nothing to say is what
// keeps a book that is read rarely measurable: the window is about following a
// change, not about ignoring the past.
func estimateSeries(progress []float64) (pages.Estimate, bool) {
	if len(progress) < pages.MinSamples {
		return pages.Estimate{}, false
	}

	if len(progress) > recentWindow {
		if estimate, ok := pages.FromProgress(progress[len(progress)-recentWindow:]); ok {
			return estimate, true
		}
	}

	return pages.FromProgress(progress)
}
