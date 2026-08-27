//
// File:        internal/kosyncapi/kosyncapi.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package kosyncapi serves the few WebUI operations that the generated
// PocketBase collection API cannot express on its own.
//
// Everything else the WebUI needs (listing documents, renaming one, deleting a
// history entry, reading statistics, logging in, subscribing to changes) goes
// through the regular PocketBase endpoints and their collection rules.
package kosyncapi

import (
	// KOReader hashes its passwords with MD5 before sending them, so the server
	// has to speak MD5 to talk to a device at all. The digest is never what is
	// stored: PocketBase hashes it with bcrypt on the way in.
	// bearer:disable go_gosec_blocklist_md5
	"crypto/md5" // #nosec G501 -- see above
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// RoutePrefix is where the WebUI specific endpoints live.
const RoutePrefix = "/api/kosync"

// minKoreaderPasswordLength is the shortest password accepted for a device
// credential. KOReader sends the MD5 digest of whatever the user types, so the
// stored value is always 32 characters and the server has to enforce a sensible
// length on the plain text itself.
const minKoreaderPasswordLength = 8

// Handler serves the WebUI specific endpoints.
type Handler struct {
	app  core.App
	conf *config.Config
}

// NewHandler creates the WebUI API handler.
func NewHandler(app core.App, conf *config.Config) *Handler {
	return &Handler{app: app, conf: conf}
}

// Register mounts the WebUI endpoints and the guards that keep the KOReader
// credentials consistent.
func Register(app core.App, conf *config.Config) *Handler {
	handler := NewHandler(app, conf)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		handler.Mount(se)
		return se.Next()
	})

	handler.registerGuards()

	return handler
}

// Mount registers the routes on the given serve event.
func (h *Handler) Mount(se *core.ServeEvent) {
	group := se.Router.Group(RoutePrefix)
	group.Bind(apis.RequireAuth(schema.CollectionUsers))

	group.POST("/koreader-accounts", h.createKoreaderAccount)
	group.POST("/koreader-accounts/{id}/password", h.rotateKoreaderPassword)
	group.POST("/documents/{id}/restore/{historyId}", h.restoreHistory)
	group.POST("/documents/merge", h.mergeDocuments)
	group.GET("/achievements", h.listAchievements)
	group.GET("/books/{id}/preview", h.previewOutline)
	group.GET("/books/{id}/preview/{index}", h.previewChapter)
	group.GET("/storage", h.storage)
}

// registerGuards keeps operations that must go through this package from
// sneaking in through the generated collection API.
func (h *Handler) registerGuards() {
	// The stored credential value has to stay the MD5 digest of the password,
	// otherwise the device can never authenticate again.
	h.app.OnRecordUpdateRequest(schema.CollectionKoreaderAccounts).BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil {
			return err
		}

		for _, field := range []string{schema.FieldPassword, schema.FieldUsername} {
			if _, present := info.Body[field]; present {
				return e.BadRequestError(
					fmt.Sprintf("The %q of a KOReader credential cannot be changed here, use %s.", field, RoutePrefix+"/koreader-accounts"),
					nil,
				)
			}
		}

		return e.Next()
	})

	// The bookkeeping of which summary has already gone out is the server's, not
	// the account's. Setting it by hand would either skip a report or, set back,
	// ask for the same one again on the next hourly run.
	h.app.OnRecordUpdateRequest(schema.CollectionUsers).BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil {
			return err
		}

		if _, present := info.Body[schema.FieldSummarySent]; present && !e.HasSuperuserAuth() {
			return e.BadRequestError(
				fmt.Sprintf("%q is kept by the server and cannot be set by hand.", schema.FieldSummarySent),
				nil,
			)
		}

		return e.Next()
	})

	// Registration is a WebUI concern; a private instance can turn it off.
	h.app.OnRecordCreateRequest(schema.CollectionUsers).BindFunc(func(e *core.RecordRequestEvent) error {
		if h.conf.DisableRegistration && !e.HasSuperuserAuth() {
			return e.ForbiddenError("Registration is disabled on this server.", nil)
		}

		return e.Next()
	})
}

// md5Hex returns the digest KOReader will send for the given password.
func md5Hex(password string) string {
	// bearer:disable go_gosec_crypto_weak_crypto, go_lang_weak_hash_md5
	return fmt.Sprintf("%x", md5.Sum([]byte(password))) // #nosec G401 -- see the import comment
}

// validateKoreaderPassword rejects passwords a device could not use safely.
func validateKoreaderPassword(password string) error {
	if len(strings.TrimSpace(password)) < minKoreaderPasswordLength {
		return fmt.Errorf("the password must be at least %d characters long", minKoreaderPasswordLength)
	}

	return nil
}

// notFoundOrError maps a missing record to a 404 and anything else to a 500.
func notFoundOrError(e *core.RequestEvent, err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return e.NotFoundError("The requested "+what+" was not found.", nil)
	}

	return e.InternalServerError("Failed to load the "+what+".", err)
}

// jsonMessage is the acknowledgement of a successful operation.
type jsonMessage struct {
	Message string `json:"message"`
}

// ok writes a 200 with a short message.
func ok(e *core.RequestEvent, message string) error {
	return e.JSON(http.StatusOK, jsonMessage{Message: message})
}
