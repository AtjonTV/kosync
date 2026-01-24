//
// File:        internal/kosync/kosync.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"flag"
	"fmt"
	"net/http"
	"sync"

	"git.obth.eu/atjontv/kosync/internal/webui"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

const Version = "2026.03.1"

type Kosync struct {
	Db     Database
	DbLock sync.Mutex
	DbFile string
}

func (app *Kosync) PrintDebug(marker, requestId, s string) {
	// Only print debugs when enabled
	if app.Db.Config.DebugLog {
		log.Debugf("RequestId=%s, Module=%s: %s\n", requestId, marker, s)
	}
}

func (app *Kosync) Print(marker, requestId, s string) {
	log.Infof("RequestId=%s, Module=%s: %s\n", requestId, marker, s)
}

func (app *Kosync) PrintError(marker, requestId, s string) {
	log.Errorf("RequestId=%s, Module=%s: %s\n", requestId, marker, s)
}

func Run() {
	log.Infof("KOsync Server v%s by Thomas Obernosterer (https://obth.eu)", Version)
	log.Info("Copyright 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later.")
	log.Info("Obtain the Source Code at https://git.obth.eu/atjontv/kosync")

	restoreFile := flag.String("restore", "", "Specify a .bak file to restore")
	makeBackup := flag.Bool("backup", false, "Create a .bak file before startup")
	enableWeb := flag.Bool("webui", false, "Enable the web interface at /web")
	flag.Parse()

	if restoreFile != nil && len(*restoreFile) > 0 {
		if err := RestoreDatabase(*restoreFile); err != nil {
			panic(err)
		}
	}

	// Try to find the database or create a new one
	foundDbFile, db, err := LoadOrInitDatabase()
	if err != nil {
		panic(err)
	}

	koapp := Kosync{
		Db:     db,
		DbFile: foundDbFile,
		DbLock: sync.Mutex{},
	}
	defer func(koapp *Kosync) {
		_ = koapp.PersistDatabase()
	}(&koapp)

	if err := koapp.MigrateSchema(); err != nil {
		panic(err)
	}

	if makeBackup != nil && *makeBackup {
		if err := koapp.BackupDatabase(); err != nil {
			koapp.PrintError("CLI", "backup", fmt.Sprintf("Failed to create backup, continuing startup: %v", err))
		}
	}

	if koapp.Db.Config.BackupOnStartup {
		if err := koapp.BackupDatabase(); err != nil {
			koapp.PrintError("Backup", "-", fmt.Sprintf("Failed to create backup, continuing startup: %v", err))
		}
	}

	app := fiber.New(fiber.Config{
		AppName:      fmt.Sprintf("KOsync v%s", Version),
		ServerHeader: "KOsync (https://git.obth.eu/atjontv/kosync)",
	})
	defer func(app *fiber.App) {
		err := app.Shutdown()
		if err != nil {
			panic(err)
		}
	}(app)
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${locals:requestid} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${error}\n",
	}))
	app.Use(koapp.NewAuthMiddleware())

	// TODO: Allow enabling web with Config option
	if enableWeb != nil && *enableWeb {
		app.Use("/web", filesystem.New(filesystem.Config{
			Root:       http.FS(webui.WebUi),
			PathPrefix: "public",
		}))

		app.Get("/", func(c *fiber.Ctx) error {
			return c.Redirect("/web")
		})
	} else {
		app.Get("/", func(c *fiber.Ctx) error {
			return c.SendString("WebUI is not enabled. If you want to use the web interface, restart KOsync with the --webui flag.")
		})
	}

	app.Get("/users/auth", koapp.UsersAuth)
	app.Post("/users/create", koapp.UsersCreate)

	app.Post("/syncs/progress", koapp.SyncsPostProgress)
	app.Get("/syncs/progress/:document", koapp.SyncsGetProgress)

	if err = app.Listen(koapp.Db.Config.ListenAddress); err != nil {
		panic(err)
	}
}
