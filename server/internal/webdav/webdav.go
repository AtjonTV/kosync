//
// File:        internal/webdav/webdav.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package webdav is the sync target for KOReader's reading statistics.
//
// KOReader keeps a database of every page turn — which book, which page, when,
// and for how long — and can sync it to a cloud target. The targets it offers
// are Dropbox, FTP and WebDAV, and of those only WebDAV is something this server
// can be: it is HTTP with a few more verbs, on the port that is already serving
// the catalog and the sync protocol.
//
// That matters because the alternative is a person carrying a file off an
// e-reader by hand. A statistics database that has to be fetched manually is one
// that is fetched once, in the first week, and never again.
//
// What this package does not do is read the file. It receives it, proves it is
// what it claims to be, and keeps it. What the numbers inside are worth — real
// reading time, and the days a device was offline for, which the sync protocol
// cannot know because it has no clock — is a separate piece of work, and it will
// be written against a file this endpoint has actually been given rather than
// against a guess at what one looks like.
package webdav

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"git.obth.eu/atjontv/kosync/internal/config"
	"github.com/pocketbase/pocketbase/core"
	dav "golang.org/x/net/webdav"
)

// RoutePrefix is where the sync target lives.
const RoutePrefix = "/webdav"

// storeDir is the directory under the PocketBase data directory that holds the
// uploads, one directory per account inside it.
const storeDir = "webdav"

// realm is what a client shows when it asks for the credentials.
const realm = `Basic realm="KOsync statistics", charset="UTF-8"`

// contextKeyOwner is where the owning account is put for the file system to
// find. The WebDAV handler has one file system for every request, so the only
// thing that can say whose directory a path means is the request itself.
type contextKey string

const contextKeyOwner contextKey = "kosync_webdav_owner"

// Authenticator verifies a device credential.
//
// The same interface the catalog uses, and satisfied by the same KOReader
// handler: a device that can push progress can upload its statistics, with the
// credential it already has. Anything else would mean a second thing to create,
// on a device with an on-screen keyboard.
type Authenticator interface {
	AuthenticateDevice(username, md5hex string) (accountId, ownerId string, err error)
}

// Handler serves the WebDAV endpoint.
type Handler struct {
	app  core.App
	conf *config.Config
	auth Authenticator
	dav  *dav.Handler
}

// Register mounts the endpoint unless the operator has turned it off.
func Register(app core.App, conf *config.Config, auth Authenticator) *Handler {
	if !conf.EnableWebdav {
		return nil
	}

	handler := NewHandler(app, conf, auth)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		handler.Mount(se)
		return se.Next()
	})

	return handler
}

// NewHandler creates the endpoint.
func NewHandler(app core.App, conf *config.Config, auth Authenticator) *Handler {
	handler := &Handler{app: app, conf: conf, auth: auth}

	handler.dav = &dav.Handler{
		Prefix:     RoutePrefix,
		FileSystem: store{root: filepath.Join(app.DataDir(), storeDir), limit: conf.WebdavMaxBytes(), refused: handler.logRefusal},
		LockSystem: dav.NewMemLS(),
		Logger:     handler.logRequest,
	}

	return handler
}

// methods are the verbs the WebDAV implementation answers: the ones everybody
// knows, and the ones RFC 4918 adds.
//
// They are registered one at a time rather than as a catch-all, and that is not
// a matter of taste. A route matching *every* method under "/webdav/" and the
// web interface's "GET /{path...}" are, to net/http, two patterns where neither
// is more specific than the other — one matches more methods, the other a more
// general path — and that is a conflict, which is a panic when the mux is built.
// Naming the methods makes this route strictly the more specific one for the
// verbs the two share, and disjoint from it for the rest.
var methods = []string{
	http.MethodOptions,
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodDelete,
	"PROPFIND",
	"PROPPATCH",
	"MKCOL",
	"COPY",
	"MOVE",
	"LOCK",
	"UNLOCK",
}

// Mount registers the routes.
func (h *Handler) Mount(se *core.ServeEvent) {
	group := se.Router.Group(RoutePrefix)
	group.BindFunc(h.requireDevice)

	for _, method := range methods {
		// The wildcard already matches the empty remainder, so it covers
		// "/webdav/" too; registering that separately is its own conflict. The
		// bare "/webdav" needs its own line, because a client that asks for the
		// collection without a trailing slash is asking a different pattern.
		group.Route(method, "", h.serve)
		group.Route(method, "/{path...}", h.serve)
	}
}

// requireDevice authenticates with the KOReader credential and remembers whose
// it is.
func (h *Handler) requireDevice(e *core.RequestEvent) error {
	username, password, ok := e.Request.BasicAuth()
	if !ok || username == "" || password == "" {
		return h.unauthorized(e)
	}

	_, owner, err := h.auth.AuthenticateDevice(username, md5Hex(password))
	if err != nil || owner == "" {
		return h.unauthorized(e)
	}

	e.Request = e.Request.WithContext(context.WithValue(e.Request.Context(), contextKeyOwner, owner))

	return e.Next()
}

// unauthorized asks for the credential.
func (h *Handler) unauthorized(e *core.RequestEvent) error {
	e.Response.Header().Set("WWW-Authenticate", realm)

	return e.UnauthorizedError("A KOReader credential is required.", nil)
}

// serve hands the request to the WebDAV implementation.
//
// The body is capped before it gets there. The upload itself also counts what it
// is given, and the two are not redundant: this one refuses a request that
// announces itself as too large without reading it, and that one catches a
// client that lies about the length.
func (h *Handler) serve(e *core.RequestEvent) error {
	if limit := h.conf.WebdavMaxBytes(); limit > 0 && e.Request.Body != nil {
		e.Request.Body = http.MaxBytesReader(e.Response, e.Request.Body, limit)
	}

	h.dav.ServeHTTP(e.Response, e.Request)

	return nil
}

// ownerFrom returns the account a request belongs to.
func ownerFrom(ctx context.Context) string {
	owner, _ := ctx.Value(contextKeyOwner).(string)

	return owner
}

// logRefusal records what was turned down.
//
// At info rather than at warn: on a healthy instance this is not a fault, it is
// a client asking for something this endpoint deliberately does not offer, and
// it is the only record of what KOReader's own client actually tries to do.
func (h *Handler) logRefusal(operation, name string, err error) {
	h.app.Logger().Info("refused a WebDAV request",
		"operation", operation, "name", name, "reason", err)
}

// logRequest records the requests the implementation could not answer.
//
// A device's very first sync asks for a file that is not there yet, and that
// 404 is the answer working rather than failing. Logging it would put a line
// that reads like a fault into the log of every account that ever starts using
// this.
func (h *Handler) logRequest(request *http.Request, err error) {
	if err == nil || os.IsNotExist(err) {
		return
	}

	h.app.Logger().Info("a WebDAV request failed",
		"method", request.Method, "path", request.URL.Path, "error", err)
}

// LastSync returns when an account last uploaded, for anything that wants to
// say so.
func (h *Handler) LastSync(ownerId string) (found bool) {
	if ownerId == "" {
		return false
	}

	_, ok := modTime(filepath.Join(h.app.DataDir(), storeDir, ownerId, FileName))

	return ok
}
