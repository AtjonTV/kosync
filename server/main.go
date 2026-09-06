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
	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/collections"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/devices"
	"git.obth.eu/atjontv/kosync/internal/importer"
	"git.obth.eu/atjontv/kosync/internal/koreader"
	"git.obth.eu/atjontv/kosync/internal/kosyncapi"
	"git.obth.eu/atjontv/kosync/internal/mail"
	"git.obth.eu/atjontv/kosync/internal/opds"
	"git.obth.eu/atjontv/kosync/internal/ownership"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/webdav"
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
const Version = "2.1.0"

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

	// The catalog authenticates with the same device credentials as the sync
	// protocol, through the same verified-credential cache, so it is handed the
	// KOReader handler rather than reaching for the credentials itself.
	sync := koreader.Register(app, conf)
	kosyncapi.Register(app, conf)
	analytics.Register(app, conf)
	books.Register(app, conf)
	collections.Register(app)
	devices.Register(app)
	// The owner rule decides whether a record may be written, not whose it is
	// once it has been. These three collections say nothing else about it; the
	// two above state it themselves, for the reasons given in the package.
	ownership.Freeze(app,
		schema.CollectionKoreaderAccounts,
		schema.CollectionDocuments,
		schema.CollectionBooks,
	)
	opds.Register(app, conf, sync)
	// The same credential again: a device that can push progress can leave its
	// statistics here, without anybody typing anything into an e-ink browser.
	// What arrives is imported into the reading days, where it beats the times
	// this server can only infer from when pushes happened to land.
	analytics.RegisterMeasurements(app, webdav.Register(app, conf, sync))
	mail.RegisterSummaries(app, conf)

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
			app.Logger().Warn("the web interface is enabled but was not built into this executable, see docs/technical/build.md")
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
