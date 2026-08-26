//
// File:        internal/opds/auth.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds

import (
	// KOReader hashes its passwords with MD5 before sending them, and the stored
	// credential is bcrypt over that digest. HTTP Basic delivers the plain
	// password, so the same MD5 step happens here instead of on the device.
	// bearer:disable go_gosec_blocklist_md5
	"crypto/md5" // #nosec G501 -- see above
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

// realm is what a browser or reader shows when it asks for the credentials.
const realm = `Basic realm="KOsync library", charset="UTF-8"`

// contextKeyOwner is where the owning account of an authenticated request is
// stored on the request event.
const contextKeyOwner = "kosync_opds_owner"

// Authenticator verifies a device credential.
//
// It is satisfied by the KOReader handler, which is the point: the catalog and
// the sync protocol accept the same credential, verified against the same store
// through the same cache, so there is no second thing to create and bcrypt stays
// off the hot path for both.
type Authenticator interface {
	AuthenticateDevice(username, md5hex string) (accountId, ownerId string, err error)
}

// ownerFrom returns the account the request belongs to.
func ownerFrom(e *core.RequestEvent) string {
	owner, _ := e.Get(contextKeyOwner).(string)
	return owner
}

// requireDevice is the middleware in front of every catalog route.
//
// Basic is what OPDS clients speak, and it is what KOReader offers for a
// catalog. It sends the password in the clear on every request, which is a
// reason to serve this over TLS and not a reason to invent a scheme no client
// implements.
func (h *Handler) requireDevice(e *core.RequestEvent) error {
	username, password, ok := e.Request.BasicAuth()
	if !ok || username == "" || password == "" {
		return h.unauthorized(e)
	}

	_, owner, err := h.auth.AuthenticateDevice(username, md5Hex(password))
	if err != nil || owner == "" {
		return h.unauthorized(e)
	}

	e.Set(contextKeyOwner, owner)

	return e.Next()
}

// unauthorized answers with the document that tells a client how to
// authenticate.
//
// A bare 401 leaves a conformant reader guessing; the OPDS authentication
// document names the scheme and labels its two fields, so the reader can put up
// the right prompt.
func (h *Handler) unauthorized(e *core.RequestEvent) error {
	e.Response.Header().Set("WWW-Authenticate", realm)

	body, err := json.Marshal(authenticationDocument(baseURL(e)))
	if err != nil {
		return e.UnauthorizedError("A KOReader credential is required.", nil)
	}

	return e.Blob(http.StatusUnauthorized, MediaAuthentication, body)
}

// authDocument is the OPDS authentication document.
type authDocument struct {
	Id             string       `json:"id"`
	Title          string       `json:"title"`
	Description    string       `json:"description,omitempty"`
	Authentication []authScheme `json:"authentication"`
}

type authScheme struct {
	Type   string     `json:"type"`
	Labels authLabels `json:"labels,omitempty"`
}

type authLabels struct {
	Login    string `json:"login,omitempty"`
	Password string `json:"password,omitempty"`
}

func authenticationDocument(base string) authDocument {
	return authDocument{
		Id:          base + RoutePrefix,
		Title:       "KOsync library",
		Description: "Sign in with the same KOReader credential this device syncs its reading progress with.",
		Authentication: []authScheme{
			{
				Type: "http://opds-spec.org/auth/basic",
				Labels: authLabels{
					Login:    "KOReader username",
					Password: "KOReader password",
				},
			},
		},
	}
}

// md5Hex returns the digest a KOReader device would send for this password.
func md5Hex(password string) string {
	// bearer:disable go_gosec_crypto_weak_crypto, go_lang_weak_hash_md5
	return fmt.Sprintf("%x", md5.Sum([]byte(password))) // #nosec G401 -- see the import comment
}
