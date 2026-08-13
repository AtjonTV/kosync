//
// File:        internal/kosyncapi/koreader_accounts.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosyncapi

import (
	"net/http"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// createAccountRequest creates a device credential.
//
// The password is sent in plain text and hashed here, so the browser never has
// to know that KOReader speaks MD5 and cannot store a value the device would
// fail to reproduce.
type createAccountRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Label    string `json:"label"`
}

// createAccountResponse describes the created credential.
type createAccountResponse struct {
	Id       string `json:"id"`
	Username string `json:"username"`
	Label    string `json:"label"`
}

// rotatePasswordRequest replaces the password of a device credential.
type rotatePasswordRequest struct {
	Password string `json:"password"`
}

// createKoreaderAccount adds a device credential to the signed in account.
func (h *Handler) createKoreaderAccount(e *core.RequestEvent) error {
	request := createAccountRequest{}
	if err := e.BindBody(&request); err != nil {
		return e.BadRequestError("Failed to read the request.", err)
	}

	username := strings.TrimSpace(request.Username)
	if username == "" {
		return e.BadRequestError("A username is required.", nil)
	}
	if err := validateKoreaderPassword(request.Password); err != nil {
		return e.BadRequestError(err.Error(), nil)
	}

	collection, err := e.App.FindCollectionByNameOrId(schema.CollectionKoreaderAccounts)
	if err != nil {
		return e.InternalServerError("Failed to load the credentials collection.", err)
	}

	record := core.NewRecord(collection)
	record.Set(schema.FieldUsername, username)
	record.Set(schema.FieldOwner, e.Auth.Id)
	record.Set(schema.FieldLabel, strings.TrimSpace(request.Label))
	record.SetPassword(md5Hex(request.Password))

	if err := e.App.Save(record); err != nil {
		// The username is unique across the whole server, because KOReader
		// identifies a device by that name alone.
		return e.BadRequestError("This KOReader username is already taken.", err)
	}

	return e.JSON(http.StatusCreated, createAccountResponse{
		Id:       record.Id,
		Username: record.GetString(schema.FieldUsername),
		Label:    record.GetString(schema.FieldLabel),
	})
}

// rotateKoreaderPassword sets a new password on one of the caller's credentials.
func (h *Handler) rotateKoreaderPassword(e *core.RequestEvent) error {
	request := rotatePasswordRequest{}
	if err := e.BindBody(&request); err != nil {
		return e.BadRequestError("Failed to read the request.", err)
	}
	if err := validateKoreaderPassword(request.Password); err != nil {
		return e.BadRequestError(err.Error(), nil)
	}

	record, err := e.App.FindRecordById(schema.CollectionKoreaderAccounts, e.Request.PathValue("id"))
	if err != nil {
		return notFoundOrError(e, err, "KOReader credential")
	}
	if record.GetString(schema.FieldOwner) != e.Auth.Id {
		// Deliberately the same answer as for a credential that does not exist.
		return e.NotFoundError("The requested KOReader credential was not found.", nil)
	}

	record.SetPassword(md5Hex(request.Password))

	if err := e.App.Save(record); err != nil {
		return e.InternalServerError("Failed to store the new password.", err)
	}

	return ok(e, "The KOReader password was changed. Sign in again on every device that used it.")
}
