//
// File:        kosync.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package main

import "git.obth.eu/atjontv/kosync/internal/kosync"

//go:generate go generate internal/webui/webui.go

func main() {
    //db, err := kosync.NewDatabase()
    //if err != nil {
    //	panic(err)
    //}
    //defer func(db *kosync.Database) {
    //	err := db.Close()
    //	if err != nil {
    //		panic(err)
    //	}
    //}(db)

    kosync.Run()
}
