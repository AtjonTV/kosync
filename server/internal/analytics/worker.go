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
	items, err := claim(w.app, w.conf.AnalyticsWorkerBatchSize)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}

	done := make([]string, 0, len(items))
	for _, item := range items {
		if err := RecomputeDay(w.app, item.Owner, item.Date, w.conf.SessionGap()); err != nil {
			// Leave the item queued so the next pass retries it, but keep
			// working on the rest of the batch.
			w.app.Logger().Error("failed to recompute a statistics day",
				"owner", item.Owner, "date", item.Date, "error", err)
			continue
		}
		done = append(done, item.Id)
	}

	if err := release(w.app, done); err != nil {
		return len(done), err
	}

	w.recognise(items)

	return len(done), nil
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

// DrainAll processes the queue until it is empty.
func (w *Worker) DrainAll() (int, error) {
	total := 0

	for {
		handled, err := w.DrainOnce()
		total += handled
		if err != nil {
			return total, err
		}
		if handled == 0 {
			return total, nil
		}
	}
}
