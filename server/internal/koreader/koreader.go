//
// File:        internal/koreader/koreader.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package koreader serves the device facing sync API.
//
// The routes mirror the KOReader progress sync protocol (as implemented by the
// official KOReader sync server) below a "/koreader" prefix, so a device is
// configured with "https://host/koreader" as its custom sync server.
package koreader

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/documents"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// RoutePrefix is where the device facing API lives.
const RoutePrefix = "/koreader"

// Handler serves the KOReader routes.
type Handler struct {
	app   core.App
	conf  *config.Config
	cache *credentialCache
}

// NewHandler creates the KOReader API handler.
func NewHandler(app core.App, conf *config.Config) *Handler {
	return &Handler{
		app:   app,
		conf:  conf,
		cache: newCredentialCache(conf.AuthCacheTtl(), conf.KoreaderAuthCacheEntries),
	}
}

// Register mounts the KOReader routes and keeps the credential cache in sync
// with the stored credentials.
func Register(app core.App, conf *config.Config) *Handler {
	handler := NewHandler(app, conf)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		handler.Mount(se)
		return se.Next()
	})

	// A rotated, disabled or deleted credential must stop working immediately,
	// not after the cache lifetime.
	app.OnRecordAfterUpdateSuccess(schema.CollectionKoreaderAccounts).BindFunc(func(e *core.RecordEvent) error {
		handler.cache.invalidateAccount(e.Record.Id)
		return e.Next()
	})
	app.OnRecordAfterDeleteSuccess(schema.CollectionKoreaderAccounts).BindFunc(func(e *core.RecordEvent) error {
		handler.cache.invalidateAccount(e.Record.Id)
		return e.Next()
	})

	return handler
}

// Mount registers the routes on the given serve event.
func (h *Handler) Mount(se *core.ServeEvent) {
	group := se.Router.Group(RoutePrefix)

	group.GET("/users/auth", h.usersAuth).BindFunc(h.requireAccount)
	group.POST("/users/create", h.usersCreate)
	group.PUT("/syncs/progress", h.putProgress).BindFunc(h.requireAccount)
	group.GET("/syncs/progress/{document}", h.getProgress).BindFunc(h.requireAccount)
}

// usersAuth confirms that the supplied device credentials are valid.
func (h *Handler) usersAuth(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, AuthResponse{Authorized: "OK"})
}

// usersCreate always refuses.
//
// A KOReader credential has to belong to a WebUI account, and the device has no
// way to ask for one, so registration happens in the WebUI only. 402 is the
// status the official server uses for "registration is not available", which
// KOReader already renders as an error the user can act on.
func (h *Handler) usersCreate(e *core.RequestEvent) error {
	return e.Error(
		http.StatusPaymentRequired,
		"Registration is only possible in the KOsync web interface. Create an account there, add a KOReader credential to it and log in with that credential here.",
		nil,
	)
}

// getProgress returns the stored progress of a single document.
func (h *Handler) getProgress(e *core.RequestEvent) error {
	account := AccountFrom(e)
	if account == nil {
		return e.UnauthorizedError("Invalid KOReader credentials.", nil)
	}

	documentHash := e.Request.PathValue("document")
	if documentHash == "" {
		return e.NotFoundError("No document requested.", nil)
	}

	record, err := documents.Resolve(h.app, account.OwnerId, documentHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return e.NotFoundError("No progress stored for this document.", nil)
		}
		return e.InternalServerError("Failed to load the document progress.", err)
	}

	return e.JSON(http.StatusOK, ProgressResponse{
		// The hash that was asked about, which after a merge is not the hash the
		// document is stored under. The device asked about its own file and
		// should be answered about its own file.
		Document:   documentHash,
		Progress:   record.GetString(schema.FieldCurrentLocation),
		Percentage: record.GetFloat(schema.FieldProgress),
		Device:     record.GetString(schema.FieldLastDevice),
		DeviceId:   record.GetString(schema.FieldLastDeviceId),
		Timestamp:  record.GetDateTime(schema.FieldLastReadAt).Time().Unix(),
	})
}

// putProgress stores a progress push and archives the state it replaced.
func (h *Handler) putProgress(e *core.RequestEvent) error {
	account := AccountFrom(e)
	if account == nil {
		return e.UnauthorizedError("Invalid KOReader credentials.", nil)
	}

	request := ProgressRequest{}
	if err := e.BindBody(&request); err != nil {
		return e.BadRequestError("Failed to read the progress payload.", err)
	}
	if request.Document == "" {
		return e.BadRequestError("Field 'document' is required.", nil)
	}

	// KOReader reports a fraction of the document. Clamp instead of rejecting:
	// a rounding artefact on the device should not cost the user their push.
	percentage := min(max(request.Percentage, 0), 1)
	now := time.Now().UTC()

	err := h.app.RunInTransaction(func(txApp core.App) error {
		document, err := documents.Resolve(txApp, account.OwnerId, request.Document)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		if document == nil {
			collection, err := txApp.FindCollectionByNameOrId(schema.CollectionDocuments)
			if err != nil {
				return err
			}
			document = core.NewRecord(collection)
			document.Set(schema.FieldOwner, account.OwnerId)
			document.Set(schema.FieldDocument, request.Document)
		} else if err := documents.Archive(txApp, document); err != nil {
			return err
		}

		document.Set(schema.FieldCurrentLocation, request.Progress)
		document.Set(schema.FieldProgress, percentage)
		document.Set(schema.FieldLastDevice, request.Device)
		document.Set(schema.FieldLastDeviceId, request.DeviceId)
		document.Set(schema.FieldLastReadAt, now)
		document.Set(schema.FieldSourceAccount, account.Id)

		return txApp.Save(document)
	})
	if err != nil {
		return e.InternalServerError("Failed to store the progress.", err)
	}

	return e.JSON(http.StatusOK, PushResponse{
		Document:  request.Document,
		Timestamp: now.Unix(),
	})
}
