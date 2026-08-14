//
// File:        internal/koreader/auth.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package koreader

import (
	"errors"
	"time"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// KOReader sends its credentials in these headers. The key is the MD5 hex digest
// of the password, which is all the KOReader plugin is able to produce.
const (
	HeaderAuthUser = "x-auth-user"
	HeaderAuthKey  = "x-auth-key"
)

// contextKeyAccount is where the authenticated device credential is stored on
// the request event.
const contextKeyAccount = "kosync_koreader_account"

// lastUsedThrottle is the minimum age of the stored "last_used" value before it
// is written again, so that a device pushing every two pages does not turn every
// request into a write.
const lastUsedThrottle = 5 * time.Minute

// errInvalidCredentials is returned for every authentication failure, so that
// the response cannot be used to tell an unknown username from a wrong password.
var errInvalidCredentials = errors.New("invalid koreader credentials")

// Account is the authenticated device credential of a request.
type Account struct {
	Id      string
	OwnerId string
}

// AccountFrom returns the credential the request authenticated with.
func AccountFrom(e *core.RequestEvent) *Account {
	account, _ := e.Get(contextKeyAccount).(*Account)
	return account
}

// requireAccount is the middleware in front of every device facing route.
//
// PocketBase auth tokens are deliberately not accepted here: these routes exist
// only for KOReader devices, which authenticate with the two headers above.
func (h *Handler) requireAccount(e *core.RequestEvent) error {
	username := e.Request.Header.Get(HeaderAuthUser)
	key := e.Request.Header.Get(HeaderAuthKey)

	if username == "" || key == "" {
		return e.UnauthorizedError("Missing KOReader credentials.", nil)
	}

	account, err := h.authenticate(username, key)
	if err != nil {
		return e.UnauthorizedError("Invalid KOReader credentials.", nil)
	}

	e.Set(contextKeyAccount, account)

	return e.Next()
}

// AuthenticateDevice verifies a device credential on behalf of another route
// group, and is how the OPDS catalog authenticates without a second credential
// store or a second cache.
//
// It takes the MD5 digest rather than the password because that is what is
// stored — the caller decides how it came by one. A caller holding a plain
// password (HTTP Basic sends one) hashes it first.
func (h *Handler) AuthenticateDevice(username, md5hex string) (accountId, ownerId string, err error) {
	account, err := h.authenticate(username, md5hex)
	if err != nil {
		return "", "", err
	}

	return account.Id, account.OwnerId, nil
}

// authenticate verifies the credentials against the cache first and the
// database second.
func (h *Handler) authenticate(username, md5hex string) (*Account, error) {
	if accountId, ownerId, found := h.cache.get(username, md5hex); found {
		return &Account{Id: accountId, OwnerId: ownerId}, nil
	}

	record, err := h.app.FindFirstRecordByData(schema.CollectionKoreaderAccounts, schema.FieldUsername, username)
	if err != nil {
		return nil, errInvalidCredentials
	}
	if record.GetBool(schema.FieldDisabled) {
		return nil, errInvalidCredentials
	}
	if !record.ValidatePassword(md5hex) {
		return nil, errInvalidCredentials
	}

	ownerId := record.GetString(schema.FieldOwner)
	if ownerId == "" {
		return nil, errInvalidCredentials
	}

	h.cache.put(username, md5hex, record.Id, ownerId)
	h.touchLastUsed(record)

	return &Account{Id: record.Id, OwnerId: ownerId}, nil
}

// touchLastUsed records that a device was seen.
//
// It writes with a plain query on purpose: going through app.Save would fire the
// record hooks, and the credential cache invalidation listening on those would
// undo the caching this whole path exists for.
func (h *Handler) touchLastUsed(record *core.Record) {
	now := time.Now().UTC()

	previous := record.GetDateTime(schema.FieldLastUsed)
	if !previous.IsZero() && now.Sub(previous.Time()) < lastUsedThrottle {
		return
	}

	_, err := h.app.DB().
		NewQuery("UPDATE {{" + schema.CollectionKoreaderAccounts + "}} SET [[last_used]] = {:now} WHERE [[id]] = {:id}").
		Bind(dbx.Params{
			"now": now.Format("2006-01-02 15:04:05.000Z"),
			"id":  record.Id,
		}).
		Execute()
	if err != nil {
		h.app.Logger().Warn("failed to record the last use of a KOReader credential",
			"account", record.Id, "error", err)
	}
}
