//
// File:        internal/app/kosync/kosync.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
)

type Kosync struct {
	DatabaseFile string
	Db           Database
	DbMutex      sync.Mutex
}

func (app *Kosync) DebugPrint(s string) {
	// Only print debugs when enabled
	if app.Db.Config.DebugLog {
		log.Println(s)
	}
}

func Run() {
	log.Println("[KOsync] KOsync Server v2026.03.0 by Thomas Obernosterer (https://obth.eu)")
	log.Println("[KOsync] Copyright 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later.")
	log.Println("[KOsync] Obtain the Source Code at https://git.obth.eu/atjontv/kosync")
	log.Println("[KOsync] ")

	var restoreFile string
	flag.StringVar(&restoreFile, "restore", "", "Specify a .bak file to restore")
	flag.Parse()

	if len(restoreFile) > 0 {
		if err := RestoreDatabase(restoreFile); err != nil {
			panic(err)
		}
	}

	// Try to find the database or create a new one
	foundDbFile, db, err := LoadOrInitDatabase()
	if err != nil {
		panic(err)
	}

	// New app instance
	kosync := Kosync{
		DatabaseFile: foundDbFile,
		Db:           db,
		DbMutex:      sync.Mutex{},
	}
	kosync.DebugPrint(fmt.Sprintf("[KOsync]: Database file '%s'", kosync.DatabaseFile))

	// Perform schema migrations
	if err := kosync.MigrateSchema(); err != nil {
		panic(err)
	}

	// Persist database
	if err := kosync.PersistDatabase(); err != nil {
		panic(err)
	}

	// Register route handlers
	kosync.RegisterApiUsers()
	kosync.RegisterApiProgress()

	// Start server
	log.Printf("[KOsync] Starting KOsync server on '%s'", db.Config.ListenAddress)
	log.Fatal(http.ListenAndServe(db.Config.ListenAddress, nil))
}
