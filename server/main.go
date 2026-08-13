//
// File:        main.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Command kosync is a progress sync server for KOReader.
//
// It is a PocketBase application: PocketBase provides the database, the
// authentication, the realtime updates and the superuser interface, while this
// package adds the KOReader sync protocol, the reading statistics and the web
// interface on top.
package main

import (
	"log"
	"net/http"

	"git.obth.eu/atjontv/kosync/internal/analytics"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/importer"
	"git.obth.eu/atjontv/kosync/internal/koreader"
	"git.obth.eu/atjontv/kosync/internal/kosyncapi"
	"git.obth.eu/atjontv/kosync/internal/webui"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/hook"

	// Registers the KOsync collections. They are applied automatically before
	// the server starts serving.
	_ "git.obth.eu/atjontv/kosync/internal/migrations"
)

// Version of the server.
const Version = "26.08.0-dev"

//go:generate go generate ./internal/webui

func main() {
	app := pocketbase.New()
	conf := config.New()

	// Schema changes made in the superuser interface are written out as Go
	// migrations here, so they can be reviewed and shipped like any other code.
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		TemplateLang: migratecmd.TemplateLangGo,
		Automigrate:  true,
		Dir:          "internal/migrations",
	})

	koreader.Register(app, conf)
	kosyncapi.Register(app, conf)
	analytics.Register(app, conf)

	app.RootCmd.AddCommand(importer.NewCommand(app, conf))

	registerWebUi(app, conf)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		app.Logger().Info("KOsync started", "version", Version, "webui", conf.EnableWebUi && webui.IsBuilt())
		return se.Next()
	})

	if err := app.Start(); err != nil {
		// Startup failures (a busy port, an unreadable data directory) have to
		// reach the operator, and at this point there is no app logger to use.
		// bearer:disable go_lang_logger_leak
		log.Fatal(err)
	}
}

// registerWebUi serves the compiled web interface from the executable.
//
// The route is registered last so that the API routes always win, and it falls
// back to index.html so the single page application can own its own paths.
func registerWebUi(app core.App, conf *config.Config) {
	if !conf.EnableWebUi {
		return
	}

	if !webui.IsBuilt() {
		app.OnServe().BindFunc(func(se *core.ServeEvent) error {
			app.Logger().Warn("the web interface is enabled but was not built into this executable, see docs/build.md")
			return se.Next()
		})

		return
	}

	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(se *core.ServeEvent) error {
			if !se.Router.HasRoute(http.MethodGet, "/{path...}") {
				se.Router.GET("/{path...}", apis.Static(webui.FS(), true))
			}

			return se.Next()
		},
		Priority: 999,
	})
}
