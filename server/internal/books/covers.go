//
// File:        internal/books/covers.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// Cron job id and schedule for the cover backfill. An hour after the rest of
// the maintenance, because this is the one that reads files rather than rows
// and has no business competing with them for the disk.
const (
	JobCovers      = "kosync.books.covers"
	scheduleCovers = "0 4 * * *"
)

// coverStartupDelay is how long after the server starts serving the first pass
// runs. Far enough back to be out of the way of everything else a start does,
// near enough that an operator who upgraded for this does not wait a night to
// see whether it worked.
const coverStartupDelay = time.Minute

// maxCoverBytes caps how much of one book is read to look for its cover. The
// field itself allows less, but a stored file that has somehow grown past this
// is not something a maintenance pass should size the server's memory for.
const maxCoverBytes = 256 << 20

// registerCovers wires the backfill into the schedule and into the server start.
func registerCovers(app core.App) {
	pass := &coverPass{app: app}
	stop := make(chan struct{})
	var once sync.Once

	app.Cron().MustAdd(JobCovers, scheduleCovers, pass.run)

	// The wait is what the stop channel is for: a server that is asked to shut
	// down inside the first minute should not sit there holding a timer. A pass
	// already under way is left to finish on its own, the same way a cron job
	// caught by a shutdown is.
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		go func() {
			select {
			case <-stop:
			case <-time.After(coverStartupDelay):
				pass.run()
			}
		}()

		return se.Next()
	})

	app.OnTerminate().BindFunc(func(te *core.TerminateEvent) error {
		once.Do(func() { close(stop) })

		return te.Next()
	})
}

// coverPass runs the backfill, and never two at once.
//
// The schedule and the pass at start-up can both come due together on a server
// that is restarted at four in the morning. A second pass over the same books
// would find the same answers with the first one's reads still in flight, so it
// is dropped rather than queued: whatever it would have done, the next run does.
type coverPass struct {
	app core.App
	mu  sync.Mutex
}

func (p *coverPass) run() {
	if !p.mu.TryLock() {
		return
	}
	defer p.mu.Unlock()

	filled, err := BackfillCovers(p.app)
	if err != nil {
		p.app.Logger().Error("failed to look for the missing book covers", "error", err)

		return
	}
	if filled > 0 {
		p.app.Logger().Info("found covers for books that had none", "books", filled)
	}
}

// BackfillCovers reads the books that have no cover and fills in the ones a
// cover can be found for now. It returns how many it filled in.
//
// A cover is extracted when the book is uploaded, and until now a book that
// gave nothing then gave nothing forever, because the file was never read
// again. Two quite different things are behind an empty cover. One is a file
// that declares its cover in a shape the reader could not follow, which every
// improvement to the reader turns into a book that would extract fine today and
// is never asked; the operator's only way out was to delete the book and upload
// it again, which throws away the reading progress linked to it. The other is a
// failure that has nothing to do with the file at all — storage that was
// briefly unreachable when the upload landed.
//
// The first case is why this exists and the second is why it runs on a schedule
// rather than once. Both are cheap to ask about: the pass reads only the books
// that have no cover, which in a healthy library is a handful of books that
// genuinely have none, at four in the morning.
//
// Nothing here is fatal. A book whose file has gone missing, or which is no
// longer readable as an EPUB, keeps its empty cover and is logged; the rest of
// the shelf must still be looked at.
func BackfillCovers(app core.App) (int, error) {
	records, err := app.FindAllRecords(schema.CollectionBooks, dbx.NewExp(
		"[["+schema.FieldCover+"]] = '' AND [["+schema.FieldFile+"]] != ''"))
	if err != nil {
		return 0, fmt.Errorf("find the books without a cover: %w", err)
	}
	if len(records) == 0 {
		return 0, nil
	}

	system, err := app.NewFilesystem()
	if err != nil {
		return 0, fmt.Errorf("open the file storage: %w", err)
	}
	defer system.Close()

	filled := 0
	for _, record := range records {
		found, err := fillCover(app, system, record)
		if err != nil {
			app.Logger().Warn("could not look for the cover of a stored book",
				"book", record.Id, "file", record.GetString(schema.FieldFile), "error", err)

			continue
		}
		if found {
			filled++
		}
	}

	return filled, nil
}

// fillCover re-reads one stored book and attaches its cover, if it has one.
func fillCover(app core.App, system *filesystem.System, record *core.Record) (bool, error) {
	book, err := openStoredBook(system, record.BaseFilesPath()+"/"+record.GetString(schema.FieldFile))
	if err != nil {
		return false, err
	}

	if err := attachCover(record, book); err != nil {
		return false, err
	}
	// attachCover is silent about everything, including having found nothing,
	// which is what an unchanged field means here.
	if len(record.GetUnsavedFiles(schema.FieldCover)) == 0 {
		return false, nil
	}

	// Saved as a record rather than written into the column: a cover is a file,
	// so the bytes have to reach the storage backend, and the shelf picks the
	// picture up from the realtime event without anybody reloading it. The
	// record really did change, so it is right that it counts as edited.
	if err := app.Save(record); err != nil {
		return false, fmt.Errorf("store the cover: %w", err)
	}

	return true, nil
}

// openStoredBook reads one stored EPUB into memory and opens it.
//
// The whole file goes through memory, one book at a time. The storage backend
// may be an object store on the other side of a network, where the seeking that
// archive/zip does would be a request per read; buying that back with one
// sequential read is the better trade.
func openStoredBook(system *filesystem.System, key string) (*epub.Reader, error) {
	reader, err := system.GetReader(key)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	raw, err := io.ReadAll(io.LimitReader(reader, maxCoverBytes))
	if err != nil {
		return nil, err
	}

	return epub.Open(bytes.NewReader(raw), int64(len(raw)))
}
