//
// File:        internal/kosync/kosync.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	// bearer:disable go_gosec_blocklist_md5
	"crypto/md5"
	"flag"
	"fmt"
	"net/http"

	"git.obth.eu/atjontv/kosync/internal/legacy"
	"git.obth.eu/atjontv/kosync/internal/webui"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

// Version NOTE: Must be the same as "sonar.projectVersion" in ../../sonar-project.properties
const Version = "2026.05.0-dev.0"

type Kosync struct {
	LegacyDb *legacy.LegacyDb
	Config   *Config
	Db       *Database
}

func (app *Kosync) PrintDebug(marker, requestId, s string) {
	// Only print debugs when enabled
	if app.Config.PrintDebugLog {
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

	config := NewConfig(&Config{
		EnableWebUi: enableWeb != nil && *enableWeb,
	})

	db, err := NewDatabase()
	if err != nil {
		panic(err)
	}

	koapp := Kosync{
		LegacyDb: legacy.New(legacy.LegacyConfig{
			RestoreFile: restoreFile,
			MakeBackup:  makeBackup,
		}),
		Config:   config,
		Db:       db,
	}
	defer func(koapp *Kosync) {
		_ = koapp.Db.Close()
		_ = koapp.LegacyDb.PersistDatabase()
	}(&koapp)

	if koapp.LegacyDb.Db.Config.BackupOnStartup || (makeBackup != nil && *makeBackup) {
		if err := koapp.LegacyDb.BackupDatabase(); err != nil {
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
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
	}))
	app.Use(koapp.NewAuthMiddleware())

	if koapp.Config.EnableWebUi {
		app.Use("/api/auth.basic", basicauth.New(basicauth.Config{
			Realm: "KOsync",
			Authorizer: func(user string, pass string) bool {
				// NOTE: Must be MD5 because that is what the KOReader Plugin is hardcoded to use
				// bearer:disable go_gosec_crypto_weak_crypto
				// bearer:disable go_lang_weak_hash_md5
				pwHash := fmt.Sprintf("%x", md5.Sum([]byte(pass)))

				userData, found, _ := koapp.Db.FindUserByUsername(user)
				//userData, found := koapp.LegacyDb.Db.Users[user]
				if !found || userData == nil {
					return false
				}

				return userData.Password == pwHash
			},
			ContextUsername: "current_user_name",
		}))

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

	app.Put("/syncs/progress", koapp.SyncsPostProgress)
	app.Get("/syncs/progress/:document", koapp.SyncsGetProgress)

	app.Get("/api/documents.all", koapp.ApiGetDocumentsAll)
	app.Put("/api/documents.update", koapp.ApiPutDocument)
	app.Get("/api/auth.basic", koapp.ApiAuthBasic)

	if err := app.Listen(koapp.Config.ListenAddress); err != nil {
		panic(err)
	}
}
