//
// File:        internal/kosyncapi/storage.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi

import (
	"net/http"

	"git.obth.eu/atjontv/kosync/internal/books"
	"github.com/pocketbase/pocketbase/core"
)

// storage reports how much room the signed in account's library takes.
//
// An endpoint rather than a sum the browser works out for itself, because half
// of the answer is not in the data at all: the limit is an operator's setting,
// and the interface has no other way to learn it. Sending both from one place
// also means the bar and the refusal always agree about what full is.
func (h *Handler) storage(e *core.RequestEvent) error {
	usage, err := books.UsageOf(e.App, h.conf.QuotaBytes(), e.Auth.Id)
	if err != nil {
		return e.InternalServerError("Failed to measure your library.", err)
	}

	return e.JSON(http.StatusOK, usage)
}
