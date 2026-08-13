//
// File:        internal/analytics/worker.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package analytics

import (
	"sync"
	"time"

	"git.obth.eu/atjontv/kosync/internal/config"
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

// Stop ends the drain loop and waits for the current pass to finish.
func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	stop, stopped := w.stop, w.stopped
	w.mu.Unlock()

	close(stop)
	<-stopped
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

	return len(done), nil
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
