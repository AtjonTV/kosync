//
// File:        internal/analytics/worker.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"sync"
	"time"

	"git.obth.eu/atjontv/kosync/internal/achievements"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/mail"
	"github.com/pocketbase/pocketbase/core"
)

// Worker drains the recomputation queue in the background.
//
// Statistics are deliberately not computed while a progress push is being
// served: a push happens every few pages and must stay cheap, while a
// recomputation reads the whole history of one document.
type Worker struct {
	app  core.App
	conf *config.Config

	mu      sync.Mutex
	stop    chan struct{}
	stopped chan struct{}
	running bool

	// mails counts the notices still being sent, so a shutdown can wait for them
	// rather than pulling the database out from under one.
	mails sync.WaitGroup
}

// NewWorker creates the queue worker.
func NewWorker(app core.App, conf *config.Config) *Worker {
	return &Worker{app: app, conf: conf}
}

// Start begins draining the queue until Stop is called.
func (w *Worker) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return
	}

	w.stop = make(chan struct{})
	w.stopped = make(chan struct{})
	w.running = true

	go w.loop(w.stop, w.stopped)
}

// Stop ends the drain loop and waits for the current pass, and for any notice
// still going out, to finish.
func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		w.mails.Wait()

		return
	}
	w.running = false
	stop, stopped := w.stop, w.stopped
	w.mu.Unlock()

	close(stop)
	<-stopped
	w.mails.Wait()
}

// loop drains the queue on every tick.
func (w *Worker) loop(stop <-chan struct{}, stopped chan<- struct{}) {
	defer close(stopped)

	ticker := time.NewTicker(w.conf.WorkerInterval())
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if _, err := w.DrainOnce(); err != nil {
				w.app.Logger().Error("failed to drain the statistics queue", "error", err)
			}
		}
	}
}

// DrainOnce processes one batch of queued days and returns how many it handled.
//
// It is exported so that the tests and the importer can run the queue to
// completion without waiting for a tick.
func (w *Worker) DrainOnce() (int, error) {
	_, handled, err := w.drainOnce()

	return handled, err
}

// drainOnce processes one batch and reports both how many items it took off the
// queue and how many of those it actually recomputed.
//
// The two numbers differ whenever something failed, and DrainAll needs the first
// one: a batch that failed in its entirety has handled nothing and yet is not the
// end of the queue, and stopping there would be the very head-of-line block the
// retry backoff exists to prevent.
func (w *Worker) drainOnce() (claimed, handled int, err error) {
	items, err := claim(w.app, w.conf.AnalyticsWorkerBatchSize)
	if err != nil {
		return 0, 0, err
	}
	if len(items) == 0 {
		return 0, 0, nil
	}

	done := make([]string, 0, len(items))
	recomputed := make([]queueItem, 0, len(items))
	for _, item := range items {
		if err := RecomputeDay(w.app, item.Owner, item.Date, w.conf.SessionGap()); err != nil {
			// Keep working on the rest of the batch, and put this one down for
			// a while so it cannot take a place in every batch from now on.
			w.app.Logger().Error("failed to recompute a statistics day",
				"owner", item.Owner, "date", item.Date, "attempts", item.Attempts+1, "error", err)

			if err := postpone(w.app, item); err != nil {
				w.app.Logger().Error("failed to postpone a statistics day",
					"owner", item.Owner, "date", item.Date, "error", err)
			}

			continue
		}
		done = append(done, item.Id)
		recomputed = append(recomputed, item)
	}

	if err := release(w.app, done); err != nil {
		return len(items), len(done), err
	}

	// Only the days that were actually recomputed: an achievement measured
	// against a day that failed is measured against numbers that have not
	// finished moving, which is the one thing this must not do.
	w.recognise(recomputed)

	return len(items), len(done), nil
}

// recognise checks what the recomputed days have earned.
//
// Once per account per batch rather than once per day: the measures are whole
// account totals, so running them per day would ask the same question eighty
// times during a bulk requeue and get the same answer.
//
// This is where it belongs because a batch is exactly the moment the numbers an
// achievement is measured from have finished moving.
func (w *Worker) recognise(items []queueItem) {
	seen := map[string]bool{}

	for _, item := range items {
		if seen[item.Owner] {
			continue
		}
		seen[item.Owner] = true

		earned, err := achievements.Evaluate(w.app, item.Owner)
		if err != nil {
			// The statistics are already stored and released. An achievement
			// that is not noticed now is noticed on the next batch.
			w.app.Logger().Warn("failed to evaluate achievements",
				"owner", item.Owner, "error", err)

			continue
		}

		for _, award := range earned {
			w.app.Logger().Info("achievement earned",
				"owner", item.Owner, "rule", award.Rule.Slug, "tier", award.Tier, "value", award.Value)
		}

		w.announce(item.Owner, earned)
	}
}

// announce mails an account what it has just earned, off the drain loop.
//
// In its own goroutine because sending is a network call to somebody else's
// server: net/smtp has no dial timeout of its own, so a mail host that accepts
// the connection and then says nothing would stall the statistics queue for as
// long as it felt like. Nothing waits on the result, and a badge is already
// stored and already on the dashboard before this is reached.
//
// Stop does wait for it, though, so a shutdown does not close the database
// underneath a message that is halfway out.
func (w *Worker) announce(ownerId string, earned []achievements.Awarded) {
	if !w.conf.EnableAchievementMail || len(earned) == 0 {
		return
	}

	w.mails.Add(1)

	go func() {
		defer w.mails.Done()

		sent, err := mail.Achievements(w.app, ownerId, earned)
		if err != nil {
			// Nothing is retried: the mail is a courtesy about something that is
			// already recorded, and a second attempt would risk sending it twice
			// rather than not at all.
			w.app.Logger().Warn("failed to mail an achievement",
				"owner", ownerId, "error", err)

			return
		}
		if sent {
			w.app.Logger().Info("achievement mail sent", "owner", ownerId, "count", len(earned))
		}
	}()
}

// DrainAll processes the queue until nothing is due, and returns how many days
// it recomputed.
//
// "Nothing is due" rather than "nothing is queued": a day that keeps failing is
// still on the queue, waiting for its backoff to run out, and waiting for it here
// would mean never returning at all.
func (w *Worker) DrainAll() (int, error) {
	total := 0

	for {
		claimed, handled, err := w.drainOnce()
		total += handled
		if err != nil {
			return total, err
		}
		if claimed == 0 {
			return total, nil
		}
	}
}
