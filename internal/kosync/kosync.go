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
	"io"
	"os"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/webui"
	"git.obth.eu/atjontv/kosync/pkg/jmp"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/fiber/v3/middleware/static"
)

// Version NOTE: Must be the same as "sonar.projectVersion" in ../../sonar-project.properties
const Version = "2026.06.1-dev.9"

const (
	CtxContextUserName = "current_user_name"
	CtxContextUserId   = "current_user_id"
)

type Kosync struct {
	Config *Config
	Db     *Database
	Crypt  *CryptState
	Jmp    *jmp.JMP
}

func setupApp(config *Config, logStream io.Writer) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      fmt.Sprintf("KOsync v%s", Version),
		ServerHeader: "KOsync (https://git.obth.eu/atjontv/kosync)",
		TrustProxy:   config.EnableTrustedProxies,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: strings.Split(config.TrustedProxies, ","),
		},
		ProxyHeader:        fiber.HeaderXForwardedFor,
		EnableIPValidation: config.ProxyIpValidation,
	})

	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${requestid} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${error}\n",
		CustomTags: map[string]logger.LogFunc{
			"requestid": func(output logger.Buffer, c fiber.Ctx, data *logger.Data, extraParam string) (int, error) {
				return output.Write([]byte(requestid.FromContext(c)))
			},
		},
		Stream: logStream,
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
	}))

	return app
}

func Run() {
	enableWeb := flag.Bool("webui", false, "Enable the web interface at /web")
	restoreDatabase := flag.String("restore", "", "Restore the database from the given backup file")
	flag.Parse()

	config := NewConfig(&Config{
		EnableWebUi: enableWeb != nil && *enableWeb,
	})
	SetDebugLogging(config.DebugLog)
	logStream, logFile := SetLogOutput(config.LogToFile, config.LogFile)
	defer func() {
		if logFile != nil {
			_ = logFile.Close()
		}
	}()

	if restoreDatabase != nil && *restoreDatabase != "" {
		restoreLog := NewKlog("db/restore")
		restoreLog.Info("Restoring database from backup file '%s'", *restoreDatabase)
		db, err := NewDatabaseWithoutMigrate(config)
		if err != nil {
			restoreLog.Error("Failed to open database for restoring: %v", err.Error())
			os.Exit(1)
		}
		err = RestoreDatabase(db.rawDb, *restoreDatabase)
		if err != nil {
			restoreLog.Error("Failed to restore database: %v", err.Error())
			os.Exit(1)
		}
		_ = db.Close()
		restoreLog.Info("Successfully restored database from backup file '%s'", *restoreDatabase)
	}

	LogInfo("")
	LogInfo("KOsync Server v%s by Thomas Obernosterer (https://obth.eu)", Version)
	LogInfo("Copyright 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later.")
	LogInfo("Obtain the Source Code at https://git.obth.eu/atjontv/kosync")
	LogInfo("")

	db, err := NewDatabase(config)
	if err != nil {
		panic(err)
	}

	koapp := Kosync{
		Config: config,
		Db:     db,
		Crypt: NewCryptState(CryptConfig{
			StaticKeySeed:      config.CryptoKeysSeed,
			JwtDurationSeconds: config.JwtDuration,
		}),
		Jmp: jmp.New(),
	}
	defer func(koapp *Kosync) {
		_ = koapp.Db.Close()
	}(&koapp)

	if koapp.Config.PrintCryptoKeys {
		pub, pri, err := koapp.Crypt.KeysAsPem()
		if err != nil {
			LogError("Could not dump temporary crypt keys: %v", err.Error())
		} else {
			LogInfo("Temporary Crypt Keys:\n%s\n%s", pub, pri)
		}
	}

	app := setupApp(config, logStream)
	defer func(app *fiber.App) {
		err := app.Shutdown()
		if err != nil {
			panic(err)
		}
	}(app)

	app.Use(koapp.NewAuthMiddleware())

	if koapp.Config.EnableWebUi {
		LogDebug("Starting KOsync with WebUI.")
		authLog := NewKlog("api/auth")
		app.Use("/api/auth.basic", basicauth.New(basicauth.Config{
			Realm: "KOsync",
			Authorizer: func(user string, pass string, ctx fiber.Ctx) bool {
				// NOTE: Must be MD5 because that is what the KOReader Plugin is hardcoded to use
				// bearer:disable go_gosec_crypto_weak_crypto
				// bearer:disable go_lang_weak_hash_md5
				pwHash := fmt.Sprintf("%x", md5.Sum([]byte(pass)))

				userData, found, _ := koapp.Db.FindUserByUsername(user)
				if !found || userData == nil {
					authLog.Debug("Could not find user '%s'", user)
					return false
				}

				ok := userData.Password == pwHash
				if ok {
					authLog.Debug("Successful login for user '%s'", user)
					ctx.Locals(CtxContextUserId, userData.Id)
					ctx.Locals(CtxContextUserName, userData.Username)
				} else {
					authLog.Debug("Failed login for user '%s'", user)
				}

				return ok
			},
		}))

		app.Use("/web", static.New("", static.Config{
			FS: webui.WebUiPublic(),
		}))

		app.Get("/", func(c fiber.Ctx) error {
			return c.Redirect().To("/web")
		})
	} else {
		LogDebug("Starting KOsync without WebUI.")
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("WebUI is not enabled. If you want to use the web interface, restart KOsync with the --webui flag.")
		})
	}

	app.Get("/users/auth", koapp.UsersAuth)
	app.Post("/users/create", koapp.UsersCreate)

	app.Put("/syncs/progress", koapp.SyncsPostProgress)
	app.Get("/syncs/progress/:document", koapp.SyncsGetProgress)

	app.Get("/api/documents.all", koapp.ApiGetDocumentsAll)
	app.Put("/api/documents.update", koapp.ApiPutDocument)
	app.Delete("/api/documents.delete", koapp.ApiDeleteDocument)
	app.Delete("/api/documents.history.delete", koapp.ApiDeleteDocumentHistory)
	app.Post("/api/documents.history.restore", koapp.ApiRestoreDocumentHistory)
	app.Get("/api/auth.basic", koapp.ApiAuthBasic)
	app.Get("/api/auth.jwt", koapp.ApiAuthForToken)

	if !koapp.Config.DisableWebSocketApi {
		koapp.ConfigureJmp()

		app.Get("/api/ws", koapp.HandleOpenWebsocket)
		app.Get("/api/ws/:id", websocket.New(koapp.HandleWebsocket, websocket.Config{
			Subprotocols: []string{"jmp"},
		}))
	}

	if err := app.Listen(koapp.Config.ListenAddress); err != nil {
		panic(err)
	}
}
