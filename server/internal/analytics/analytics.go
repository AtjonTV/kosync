//
// File:        internal/analytics/analytics.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package analytics keeps the reading statistics up to date.
//
// The legacy server computed the dashboard numbers with a recursive query over
// the full progress history on every request. Here they are computed once, in
// the background, and stored in the reading_days collection, which the WebUI
// reads and subscribes to like any other collection.
package analytics

import (
	"time"

	"git.obth.eu/atjontv/kosync/internal/config"
	"github.com/pocketbase/pocketbase/core"
)

// Cron job ids and schedules.
const (
	JobRetention      = "kosync.analytics.retention"
	JobReconcile      = "kosync.analytics.reconcile"
	scheduleRetention = "15 3 * * *" // daily, shortly after midnight UTC
	scheduleReconcile = "45 3 * * 1" // weekly, on Monday
)

// Register wires the statistics pipeline into the application lifecycle.
func Register(app core.App, conf *config.Config) *Worker {
	registerEnqueueHooks(app)
	registerBookHooks(app)
	registerTimezoneHooks(app)

	worker := NewWorker(app, conf)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		worker.Start()
		return se.Next()
	})
	app.OnTerminate().BindFunc(func(te *core.TerminateEvent) error {
		worker.Stop()
		return te.Next()
	})

	app.Cron().MustAdd(JobRetention, scheduleRetention, func() {
		removed, err := ApplyRetention(app, conf, time.Now())
		if err != nil {
			app.Logger().Error("failed to apply the statistics retention", "error", err)
			return
		}
		if removed > 0 {
			app.Logger().Info("aged out daily statistics",
				"days", removed, "mode", conf.AnalyticsRetentionMode)
		}
	})

	app.Cron().MustAdd(JobReconcile, scheduleReconcile, func() {
		queued, err := Reconcile(app, conf, time.Now())
		if err != nil {
			app.Logger().Error("failed to reconcile the statistics", "error", err)
			return
		}
		app.Logger().Info("queued statistics days for reconciliation", "days", queued)
	})

	return worker
}
