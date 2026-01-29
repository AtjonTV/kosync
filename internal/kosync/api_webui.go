//
// File:        internal/kosync/api_webui.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"encoding/base64"
	"encoding/json"

	"git.obth.eu/atjontv/kosync/internal/legacy"
	"github.com/gofiber/fiber/v2"
)

type UiDocumentData struct {
	Id string `json:"id"`
	legacy.FileData
	History []legacy.FileData `json:"history"`
}

func (app *Kosync) ApiGetDocumentsAll(c *fiber.Ctx) error {
	data, found := app.LegacyDb.FindUser(c.Locals("current_user").(string))
	if !found {
		return fiber.ErrNotFound
	}

	result := make([]UiDocumentData, 0, len(data.Documents))
	for id, doc := range data.Documents {
		history, found := data.History[doc.DocumentId]
		if !found {
			result = append(result, UiDocumentData{id, doc, make([]legacy.FileData, 0)})
		} else {
			result = append(result, UiDocumentData{id, doc, history.DocumentHistory})
		}
	}

	c.Set("Access-Control-Allow-Origin", "*")
	return c.JSON(result)
}

func (app *Kosync) ApiPutDocument(c *fiber.Ctx) error {
	var document UiDocumentData
	if err := c.BodyParser(&document); err != nil {
		return err
	}

	if err := app.LegacyDb.UpdateDocumentPrettyName(c.Locals("current_user").(string), document.DocumentId, document.PrettyName); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (app *Kosync) ApiAuthBasic(c *fiber.Ctx) error {
	user, _ := app.LegacyDb.FindUser(c.Locals("current_user").(string))
	type UserData struct {
		Username string `json:"username"`
		Key      string `json:"key"`
	}
	bytes, _ := json.Marshal(UserData{user.Username, user.Password})
	userObj := base64.StdEncoding.EncodeToString(bytes)
	return c.Redirect("/web?user="+userObj, fiber.StatusTemporaryRedirect)
}
